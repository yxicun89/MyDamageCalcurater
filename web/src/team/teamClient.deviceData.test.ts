// P5-5d: 構築 API(team-svc)のクライアントの deleteDeviceData(ADR-0318 §1、ADR-0209 §5)。
// DELETE /api/team/device-data(api/openapi.yaml の deleteTeamDeviceData)。recordClient.test.ts と同じ観点。
//   - DELETE で `${baseUrl}api/team/device-data`。本文・クエリ・Content-Type なし。ヘッダーは X-Device-Id・X-Session-Id
//   - 200 は completed / partial をそのまま運ぶ(繰り返しは呼び出し側)
//   - HTTP エラーの {code, message} はそのまま運ぶ。通信不能・JSON でない・形が不正は team_unavailable
// 既存の teamClient.test.ts は編集しない(別ファイルで追加)。架空の ID だけを使う。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { TEAM_PATHS, TEAM_UNAVAILABLE_CODE, createTeamClient } from "./teamClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};
const BASE_URL = "http://team.test/";

const completed: Schemas["TeamDeletionResult"] = {
  status: "completed",
  purgedAt: "2026-10-02T01:00:00Z",
  deleted: { teams: 2, teamMembers: 6 },
};
const partial: Schemas["TeamDeletionResult"] = { ...completed, status: "partial" };

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function client(response: Response | Error) {
  const fetchMock = vi.fn<typeof fetch>(() =>
    response instanceof Error ? Promise.reject(response) : Promise.resolve(response),
  );
  return { client: createTeamClient({ baseUrl: BASE_URL, fetch: fetchMock, ids }), fetchMock };
}

function firstCall(fetchMock: ReturnType<typeof client>["fetchMock"]): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[0];
  if (call === undefined) {
    throw new Error("fetch が呼ばれていない");
  }
  const [url, init] = call;
  if (typeof url !== "string" || init === undefined) {
    throw new Error("fetch は (文字列の URL, RequestInit) で呼ぶ");
  }
  return { url, init };
}

describe("deleteDeviceData の要求", () => {
  test("パスは api/team/device-data(TEAM_PATHS.deviceData)", () => {
    expect(TEAM_PATHS.deviceData).toBe("api/team/device-data");
  });

  test("DELETE で baseUrl + パス。本文なし、ヘッダーは X-Device-Id・X-Session-Id だけ", async () => {
    const { client: c, fetchMock } = client(jsonResponse(200, completed));
    await c.deleteDeviceData();
    const { url, init } = firstCall(fetchMock);
    expect(url).toBe("http://team.test/api/team/device-data");
    expect(init.method).toBe("DELETE");
    expect(init.body === undefined || init.body === null).toBe(true);
    expect(Object.fromEntries(new Headers(init.headers).entries())).toEqual({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
  });
});

describe("deleteDeviceData の応答", () => {
  test("completed / partial をそのまま運ぶ(fetch は1回ずつ)", async () => {
    const a = client(jsonResponse(200, completed));
    expect(await a.client.deleteDeviceData()).toEqual({ ok: true, value: completed });
    const b = client(jsonResponse(200, partial));
    expect(await b.client.deleteDeviceData()).toEqual({ ok: true, value: partial });
    expect(b.fetchMock).toHaveBeenCalledTimes(1);
  });

  test("HTTP エラーの {code, message} をそのまま運ぶ", async () => {
    const body = { code: "upstream_unavailable", message: "team-svc に届きません" };
    const { client: c } = client(jsonResponse(503, body));
    expect(await c.deleteDeviceData()).toEqual({ ok: false, error: body });
  });

  test("通信できないときは team_unavailable。例外を投げない", async () => {
    const { client: c } = client(new TypeError("Failed to fetch"));
    const result = await c.deleteDeviceData();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(TEAM_UNAVAILABLE_CODE);
    }
  });

  test.each([
    ["JSON でない本文", new Response("<html>bad gateway</html>", { status: 502 })],
    ["エラー本文の形が不正", jsonResponse(500, { error: "x" })],
    ["200 なのに status が無い", jsonResponse(200, { purgedAt: "2026-10-02T01:00:00Z" })],
    ["200 なのに status が未知の値", jsonResponse(200, { ...completed, status: "unknown" })],
  ])("%s は team_unavailable", async (_name, response) => {
    const { client: c } = client(response);
    const result = await c.deleteDeviceData();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(TEAM_UNAVAILABLE_CODE);
    }
  });
});
