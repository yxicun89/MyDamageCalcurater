// P5-5d: 記録 API(record-svc)のクライアント createRecordClient の deleteDeviceData(ADR-0318 §1、ADR-0209 §5)。
// teamClient.test.ts と同じ形。ルートの api/openapi.yaml(DELETE /api/record/device-data)を正とし、応答は生成型で書く。
// このファイルは P5-5c(feat/web-record-frequent-p5-5c)の同名の recordClient.ts と統合される前提で、
// 型名・関数名(createRecordClient / RECORD_PATHS / RECORD_UNAVAILABLE_CODE / RecordResult)を揃えている。
// 確かめること:
//   - DELETE で `${baseUrl}api/record/device-data` を呼ぶ。本文もクエリも送らず、端末 ID はヘッダー(X-Device-Id・X-Session-Id)
//   - Content-Type は付けない(本文が無い)
//   - 200 は completed でも partial でも応答をそのまま運ぶ(partial の繰り返しはクライアントでなく呼び出し側の仕事)
//   - HTTP エラーで本文が {code, message} ならそのまま運ぶ(503 store_unavailable / upstream_unavailable)
//   - 通信できない・JSON でない・エラー本文の形が不正・200 なのに status が無いときは record_unavailable(Web 側のコード)
// 架空の ID だけを使う。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { RECORD_PATHS, RECORD_UNAVAILABLE_CODE, createRecordClient } from "./recordClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};
const BASE_URL = "http://record.test/";

const completed: Schemas["RecordDeletionResult"] = {
  status: "completed",
  purgedAt: "2026-10-02T01:00:00Z",
  deleted: { calcEvents: 3, aggregates: 2, favorites: 1 },
};
const partial: Schemas["RecordDeletionResult"] = { ...completed, status: "partial" };

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function fakeFetch(response: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    response instanceof Error ? Promise.reject(response) : Promise.resolve(response),
  );
}

function firstCall(fetchMock: ReturnType<typeof fakeFetch>): { url: string; init: RequestInit } {
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

function client(response: Response | Error) {
  const fetchMock = fakeFetch(response);
  return { client: createRecordClient({ baseUrl: BASE_URL, fetch: fetchMock, ids }), fetchMock };
}

describe("deleteDeviceData の要求", () => {
  test("パスは api/record/device-data(端末 ID はパスに含めない)", () => {
    expect(RECORD_PATHS.deviceData).toBe("api/record/device-data");
  });

  test("DELETE で baseUrl + パスを呼び、クエリ・本文を持たない", async () => {
    const { client: c, fetchMock } = client(jsonResponse(200, completed));
    await c.deleteDeviceData();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = firstCall(fetchMock);
    expect(url).toBe("http://record.test/api/record/device-data");
    expect(init.method).toBe("DELETE");
    expect(init.body === undefined || init.body === null).toBe(true);
  });

  test("ヘッダーは X-Device-Id・X-Session-Id だけ(Content-Type は付けない)", async () => {
    const { client: c, fetchMock } = client(jsonResponse(200, completed));
    await c.deleteDeviceData();
    const headers = Object.fromEntries(new Headers(firstCall(fetchMock).init.headers).entries());
    expect(headers).toEqual({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
  });
});

describe("deleteDeviceData の応答", () => {
  test("completed をそのまま運ぶ", async () => {
    const { client: c } = client(jsonResponse(200, completed));
    expect(await c.deleteDeviceData()).toEqual({ ok: true, value: completed });
  });

  test("partial もそのまま運ぶ(自分で繰り返さない。fetch は1回)", async () => {
    const { client: c, fetchMock } = client(jsonResponse(200, partial));
    expect(await c.deleteDeviceData()).toEqual({ ok: true, value: partial });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  test("HTTP エラーで本文が {code, message} なら、その code・message を運ぶ", async () => {
    const body = { code: "store_unavailable", message: "record DB に届きません" };
    const { client: c } = client(jsonResponse(503, body));
    expect(await c.deleteDeviceData()).toEqual({ ok: false, error: body });
  });

  test("通信できない(fetch が reject)は record_unavailable。例外を投げない", async () => {
    const { client: c } = client(new TypeError("Failed to fetch"));
    const result = await c.deleteDeviceData();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
      expect(result.error.message).not.toBe("");
    }
  });

  test.each([
    ["JSON でない本文(HTML のエラーページ等)", new Response("<html>bad gateway</html>", { status: 502 })],
    ["エラー本文の形が不正", jsonResponse(500, { error: "x" })],
    ["200 なのに status が無い", jsonResponse(200, { purgedAt: "2026-10-02T01:00:00Z" })],
    ["200 なのに status が未知の値", jsonResponse(200, { ...completed, status: "unknown" })],
  ])("%s は record_unavailable", async (_name, response) => {
    const { client: c } = client(response);
    const result = await c.deleteDeviceData();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
    }
  });
});
