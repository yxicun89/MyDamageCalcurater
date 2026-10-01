// P5-5 PR-A1: 構築 API のクライアント createTeamClient(ADR-0309 §3)。speedClient.test.ts と同じ形。
// ルートの api/openapi.yaml を正とし、リクエスト本文・応答は openapi-typescript の生成型
// (api/openapi.gen.ts)で書く(契約とのずれを typecheck で検出する)。fetch は fake。
// 確かめること(受け入れ条件 AC-1):
//   - list / get は GET、create は POST、update は PUT、remove は DELETE で、
//     `${baseUrl}api/team/teams`(1件は `.../teams/{teamId}`)を呼ぶ。パスに端末 ID は含めない
//   - ヘッダーは X-Device-Id・X-Session-Id(他の API と同じ ID。CLAUDE.md 技術規約)。
//     本文を送る create / update だけ Content-Type: application/json も付ける
//   - 本文は TeamInput のまま送る(Web で members を足さない・削らない)
//   - 成功は {ok: true, value}(応答をそのまま運ぶ。並べ替え・整形をしない)
//   - remove は 204(本文なし)でも {ok: true} になる(本文を読もうとして失敗しない)
//   - HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ(404 not_found も)
//   - 通信できない・応答が JSON でない・エラー本文の形が不正なら team_unavailable(Web 側のコード)
// 架空の ID・名前だけを使う(実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { TEAM_PATHS, TEAM_UNAVAILABLE_CODE, createTeamClient, type TeamResult } from "./teamClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://team.test/";

const TEAM_ID = "33333333-3333-4333-8333-333333333333";

const team: Schemas["Team"] = {
  id: TEAM_ID,
  name: "テスト構築",
  members: [],
  createdAt: "2026-09-26T01:00:00Z",
  updatedAt: "2026-09-26T02:00:00Z",
};

const otherTeam: Schemas["Team"] = {
  id: "44444444-4444-4444-8444-444444444444",
  name: "テスト構築2",
  members: [],
  createdAt: "2026-09-25T01:00:00Z",
  updatedAt: "2026-09-25T01:00:00Z",
};

/** PR-A1 が送る TeamInput(メンバーは常に空)。 */
const input: Schemas["TeamInput"] = { name: "テスト構築", members: [] };

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function fakeFetch(response: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    response instanceof Error ? Promise.reject(response) : Promise.resolve(response),
  );
}

/** fetch の n 回目の呼び出しの URL・RequestInit を取り出す(speedClient.test.ts と同じ形)。 */
function callAt(fetchMock: ReturnType<typeof fakeFetch>, index = 0): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[index];
  if (call === undefined) {
    throw new Error(`fetch の ${String(index)} 回目の呼び出しが無い`);
  }
  const [url, init] = call;
  if (typeof url !== "string" || init === undefined) {
    throw new Error("fetch は (文字列の URL, RequestInit) で呼ぶ");
  }
  return { url, init };
}

function headersOf(init: RequestInit): Record<string, string> {
  return Object.fromEntries(new Headers(init.headers).entries());
}

function bodyOf(init: RequestInit): unknown {
  if (typeof init.body !== "string") {
    throw new Error("本文は JSON 文字列");
  }
  return JSON.parse(init.body);
}

function client(response: Response | Error): {
  client: ReturnType<typeof createTeamClient>;
  fetchMock: ReturnType<typeof fakeFetch>;
} {
  const fetchMock = fakeFetch(response);
  return { client: createTeamClient({ baseUrl: BASE_URL, fetch: fetchMock, ids }), fetchMock };
}

describe("パス(api/openapi.yaml の paths。端末 ID はパスに含めない)", () => {
  test("一覧のパスと、1件のパス(teamId は URL エンコードする)", () => {
    expect(TEAM_PATHS.teams).toBe("api/team/teams");
    expect(TEAM_PATHS.team(TEAM_ID)).toBe(`api/team/teams/${TEAM_ID}`);
    expect(TEAM_PATHS.team("a/b")).toBe("api/team/teams/a%2Fb");
  });
});

describe("list(この端末の構築一覧)", () => {
  test("teams のパスに、端末 ID・セッション ID 付きで GET する", async () => {
    const { client: teamClient, fetchMock } = client(jsonResponse(200, [team, otherTeam]));

    const result = await teamClient.list();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://team.test/api/team/teams");
    expect(init.method).toBe("GET");
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    // GET なので本文は送らない。
    expect(init.body).toBeUndefined();
    // 応答はそのまま運ぶ(サーバーの並び〈更新の新しい順〉を Web で並べ替えない)。
    expect(result).toEqual({ ok: true, value: [team, otherTeam] });
  });

  test("1件も無ければ空配列を成功として返す(エラーにしない)", async () => {
    const { client: teamClient } = client(jsonResponse(200, []));
    expect(await teamClient.list()).toEqual({ ok: true, value: [] });
  });

  test("基点 URL が同じオリジン(/)なら /api/team/teams を呼ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, []));
    await createTeamClient({ baseUrl: "/", fetch: fetchMock, ids }).list();
    expect(callAt(fetchMock).url).toBe("/api/team/teams");
  });
});

describe("create(構築を作る)", () => {
  test("teams のパスに TeamInput を POST し、201 の Team を返す", async () => {
    const { client: teamClient, fetchMock } = client(jsonResponse(201, team));

    const result = await teamClient.create(input);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://team.test/api/team/teams");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    // 本文は渡されたまま送る(Web で id・createdAt を足さない。送ると 400 unknown_field)。
    expect(bodyOf(init)).toEqual(input);
    expect(result).toEqual({ ok: true, value: team });
  });

  test("メンバー付きの TeamInput も中身を変えずに送る(PR-A2 のメンバー編集の備え)", async () => {
    // 架空のメンバー1体。クライアントは中身を解釈しない(ID のまま運ぶ。ADR-0213 §3)。
    const member: Schemas["TeamMember"] = {
      speciesKey: "9001-000",
      nickname: null,
      moveIds: ["test-move"],
      itemId: null,
      abilityId: null,
      natureId: "test-nature",
      sp: { hp: 32, atk: 0, def: 0, spa: 0, spd: 0, spe: 32 },
      teraType: null,
    };
    const { client: teamClient, fetchMock } = client(jsonResponse(201, { ...team, members: [member] }));

    await teamClient.create({ name: "テスト構築", members: [member] });

    expect(bodyOf(callAt(fetchMock).init)).toEqual({ name: "テスト構築", members: [member] });
  });

  test.each([
    [400, "invalid_input", "name must be 1..50 characters"],
    [413, "request_too_large", "request body is too large"],
    [500, "internal_error", "internal error"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const { client: teamClient } = client(jsonResponse(status, { code, message }));
    expect(await teamClient.create(input)).toEqual({ ok: false, error: { code, message } });
  });
});

describe("get(構築を1件取る)", () => {
  test("teams/{teamId} のパスに、端末 ID・セッション ID 付きで GET する", async () => {
    const { client: teamClient, fetchMock } = client(jsonResponse(200, team));

    const result = await teamClient.get(TEAM_ID);

    const { url, init } = callAt(fetchMock);
    expect(url).toBe(`http://team.test/api/team/teams/${TEAM_ID}`);
    expect(init.method).toBe("GET");
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(result).toEqual({ ok: true, value: team });
  });

  test("この端末が持たない ID の 404 not_found は、そのまま運ぶ(分離は ADR-0209 §6)", async () => {
    const { client: teamClient } = client(
      jsonResponse(404, { code: "not_found", message: "team not found" }),
    );
    expect(await teamClient.get(TEAM_ID)).toEqual({
      ok: false,
      error: { code: "not_found", message: "team not found" },
    });
  });
});

describe("update(構築を全置換する)", () => {
  test("teams/{teamId} のパスに TeamInput を PUT し、200 の Team を返す", async () => {
    const renamed: Schemas["Team"] = { ...team, name: "テスト構築(改名)" };
    const { client: teamClient, fetchMock } = client(jsonResponse(200, renamed));

    const result = await teamClient.update(TEAM_ID, { name: "テスト構築(改名)", members: [] });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe(`http://team.test/api/team/teams/${TEAM_ID}`);
    expect(init.method).toBe("PUT");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(bodyOf(init)).toEqual({ name: "テスト構築(改名)", members: [] });
    expect(result).toEqual({ ok: true, value: renamed });
  });

  test("404 not_found をそのまま運ぶ", async () => {
    const { client: teamClient } = client(
      jsonResponse(404, { code: "not_found", message: "team not found" }),
    );
    expect(await teamClient.update(TEAM_ID, input)).toEqual({
      ok: false,
      error: { code: "not_found", message: "team not found" },
    });
  });
});

describe("remove(構築を消す)", () => {
  test("teams/{teamId} に DELETE し、204(本文なし)を成功として返す", async () => {
    const { client: teamClient, fetchMock } = client(new Response(null, { status: 204 }));

    const result = await teamClient.remove(TEAM_ID);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe(`http://team.test/api/team/teams/${TEAM_ID}`);
    expect(init.method).toBe("DELETE");
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(init.body).toBeUndefined();
    // 本文が無いので、本文を読もうとして team_unavailable に落ちてはいけない。
    expect(result.ok).toBe(true);
  });

  test("同じ構築をもう一度消したときの 404 not_found をそのまま運ぶ(冪等にしない)", async () => {
    const { client: teamClient } = client(
      jsonResponse(404, { code: "not_found", message: "team not found" }),
    );
    expect(await teamClient.remove(TEAM_ID)).toEqual({
      ok: false,
      error: { code: "not_found", message: "team not found" },
    });
  });
});

describe("通信・応答の失敗は team_unavailable(自動の切り替えはしない)", () => {
  const cases: ReadonlyArray<[string, () => Response | Error]> = [
    ["fetch が reject する(通信できない)", () => new TypeError("Failed to fetch")],
    [
      "200 だが本文が JSON でない",
      () => new Response("<html>gateway</html>", { status: 200, headers: { "Content-Type": "text/html" } }),
    ],
    [
      "502 で本文が JSON でない",
      () => new Response("Bad Gateway", { status: 502, headers: { "Content-Type": "text/plain" } }),
    ],
    ["500 で本文が {code, message} の形でない", () => jsonResponse(500, { error: "boom" })],
  ];

  /** 5つの操作それぞれを1回呼ぶ(失敗の扱いが操作ごとにぶれないこと)。 */
  const operations: ReadonlyArray<
    [string, (teamClient: ReturnType<typeof createTeamClient>) => Promise<TeamResult<unknown>>]
  > = [
    ["list", (teamClient) => teamClient.list()],
    ["create", (teamClient) => teamClient.create(input)],
    ["get", (teamClient) => teamClient.get(TEAM_ID)],
    ["update", (teamClient) => teamClient.update(TEAM_ID, input)],
    ["remove", (teamClient) => teamClient.remove(TEAM_ID)],
  ];

  for (const [operationName, call] of operations) {
    test.each(cases)(`${operationName}: %s`, async (_name, make) => {
      const { client: teamClient } = client(make());
      const result = await call(teamClient);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error.code).toBe(TEAM_UNAVAILABLE_CODE);
        expect(result.error.message).not.toBe("");
      }
    });
  }

  test("remove の 502(本文が JSON でない)も team_unavailable(204 の扱いと取り違えない)", async () => {
    const { client: teamClient } = client(new Response("Bad Gateway", { status: 502 }));
    const result = await teamClient.remove(TEAM_ID);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(TEAM_UNAVAILABLE_CODE);
      expect(result.error.message).not.toBe("");
    }
  });

  test("例外を投げない(失敗はすべて戻り値で返す)", async () => {
    const { client: teamClient } = client(new TypeError("Failed to fetch"));
    await expect(teamClient.list()).resolves.toMatchObject({ ok: false });
    await expect(teamClient.remove(TEAM_ID)).resolves.toMatchObject({ ok: false });
  });
});
