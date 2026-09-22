// P4-12a: balance API のクライアント createBalanceClient(ADR-0303 §1・§5・§6)。
// services/balance/api/openapi.yaml を正とし、リクエスト本文・応答は openapi-typescript の生成型(balance.gen.ts)で書く
// (契約とのずれを typecheck で検出する)。fetch は fake。
// 確かめること:
//   - analyze / coverage は `${baseUrl}api/balance/v1/team-balance/analyze|coverage` に POST する
//   - ヘッダーは Content-Type(JSON)・X-Device-Id・X-Session-Id(計算と同じ ID。ADR-0303 §5)
//   - 本文は生成型の AnalyzeRequest / CoverageRequest のまま({members})
//   - 成功は {ok: true, value}(応答をそのまま運ぶ。Web で倍率を計算し直さない。ADR-0303 §1)
//   - HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ
//   - 通信できない・応答が JSON でない・エラー本文の形が不正なら balance_unavailable(Web 側のコード)
//     にし、WASM へは切り替えない(ADR-0303 §6)

import { describe, expect, test, vi } from "vitest";
import type { components } from "./balance.gen";
import { createBalanceClient } from "./balanceClient";
import type { ClientIds } from "./clientIds";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://balance.test/";

/** 18タイプの正準順(応答の並び。テストの応答を作るためだけに使う)。 */
const TYPES: readonly Schemas["TypeId"][] = [
  "normal",
  "fire",
  "water",
  "electric",
  "grass",
  "ice",
  "fighting",
  "poison",
  "ground",
  "flying",
  "psychic",
  "bug",
  "rock",
  "ghost",
  "dragon",
  "dark",
  "steel",
  "fairy",
];

const analyzeResponse: Schemas["AnalyzeResponse"] = {
  members: [
    {
      pokemonId: "9001-000",
      abilityId: "example-ability-none",
      types: ["fire"],
      defense: TYPES.map((attackType) => ({
        attackType,
        multiplier: "1",
        category: "neutral",
        source: "type",
        effect: "none",
      })),
    },
  ],
  teamSummary: TYPES.map((attackType) => ({
    attackType,
    weak: 0,
    quadWeak: 0,
    resist: 0,
    immune: 0,
    neutral: 1,
  })),
};

const coverageResponse: Schemas["CoverageResponse"] = {
  members: [
    {
      pokemonId: "9001-000",
      moveIds: ["example-move-firepunch"],
      attackTypes: ["fire"],
      coverage: TYPES.map((defenseType) => ({
        defenseType,
        bestMultiplier: "1",
        effective: true,
        superEffective: false,
      })),
    },
  ],
  teamCoverage: TYPES.map((defenseType) => ({
    defenseType,
    bestMultiplier: "1",
    effectiveMembers: 1,
    superEffectiveMembers: 0,
  })),
};

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

/** fetch の n 回目の呼び出しの URL・RequestInit を取り出す。 */
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

describe("analyze", () => {
  test("analyze のパスに、端末 ID・セッション ID 付きで AnalyzeRequest を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, analyzeResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const members: Schemas["AnalyzeRequest"]["members"] = [
      { pokemonId: "9001-000", abilityId: "example-ability-none" },
      { pokemonId: "9002-000" },
    ];

    await client.analyze(members);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://balance.test/api/balance/v1/team-balance/analyze");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    const expected: Schemas["AnalyzeRequest"] = {
      members: [{ pokemonId: "9001-000", abilityId: "example-ability-none" }, { pokemonId: "9002-000" }],
    };
    expect(bodyOf(init)).toEqual(expected);
  });

  test("基点 URL が同じオリジン(/)なら /api/balance/... に POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, analyzeResponse));
    const client = createBalanceClient({ baseUrl: "/", fetch: fetchMock, ids });
    await client.analyze([{ pokemonId: "9001-000" }]);
    expect(callAt(fetchMock).url).toBe("/api/balance/v1/team-balance/analyze");
  });

  test("200 の応答をそのまま value にする(倍率・集計を計算し直さない)", async () => {
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(200, analyzeResponse)),
      ids,
    });
    const result = await client.analyze([{ pokemonId: "9001-000" }]);
    expect(result).toEqual({ ok: true, value: analyzeResponse });
  });

  test.each([
    [422, "unknown_pokemon", "unknown pokemonId: 9999-000"],
    [422, "unknown_ability", "unknown abilityId: example-ability-x"],
    [503, "master_unavailable", "pokemon type read model is not configured"],
    [400, "invalid_request", "members must have 1 to 6 entries"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    const result = await client.analyze([{ pokemonId: "9001-000" }]);
    expect(result).toEqual({ ok: false, error: { code, message } });
  });
});

describe("coverage", () => {
  test("coverage のパスに、端末 ID・セッション ID 付きで CoverageRequest を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, coverageResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const members: Schemas["CoverageRequest"]["members"] = [
      { pokemonId: "9001-000", moveIds: ["example-move-firepunch", "example-move-tackle"] },
      { pokemonId: "9002-000", moveIds: [] },
    ];

    const result = await client.coverage(members);

    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://balance.test/api/balance/v1/team-balance/coverage");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    const expected: Schemas["CoverageRequest"] = {
      members: [
        { pokemonId: "9001-000", moveIds: ["example-move-firepunch", "example-move-tackle"] },
        { pokemonId: "9002-000", moveIds: [] },
      ],
    };
    expect(bodyOf(init)).toEqual(expected);
    expect(result).toEqual({ ok: true, value: coverageResponse });
  });

  test("HTTP エラーの {code: unknown_move} をそのまま運ぶ", async () => {
    const error: Schemas["Error"] = { code: "unknown_move", message: "unknown moveId: example-move-x" };
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(422, error)),
      ids,
    });
    const result = await client.coverage([{ pokemonId: "9001-000", moveIds: ["example-move-x"] }]);
    expect(result).toEqual({ ok: false, error });
  });
});

describe("通信・応答の失敗は balance_unavailable(自動の切り替えはしない。ADR-0303 §6)", () => {
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

  test.each(cases)("analyze: %s", async (_name, make) => {
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.analyze([{ pokemonId: "9001-000" }]);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("balance_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test.each(cases)("coverage: %s", async (_name, make) => {
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.coverage([{ pokemonId: "9001-000", moveIds: [] }]);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("balance_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test("例外を投げない(失敗はすべて戻り値で返す)", async () => {
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(new TypeError("Failed to fetch")),
      ids,
    });
    await expect(client.analyze([{ pokemonId: "9001-000" }])).resolves.toMatchObject({ ok: false });
  });
});
