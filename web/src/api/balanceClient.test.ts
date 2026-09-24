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
import { createBalanceClient, type BalanceClient, type BalanceResult } from "./balanceClient";
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

// ---- P4-12b: 仮想敵(threats)・おすすめタイプ(recommendations)(ADR-0303 §7・§8) ----

const threatsResponse: Schemas["ThreatsResponse"] = {
  threats: [
    {
      pokemonId: "9002-000",
      abilityId: "example-ability-none",
      attackTypes: ["water"],
      matchups: [
        { pokemonId: "9001-000", incoming: "2", outgoing: "1/2", safe: false, superEffective: false },
      ],
      safeMembers: 0,
      superEffectiveMembers: 0,
    },
  ],
};

const recommendationsResponse: Schemas["RecommendationsResponse"] = {
  defenseHoles: ["water"],
  offenseHoles: ["dragon"],
  candidates: [
    {
      types: ["grass"],
      defenseCovered: ["water"],
      offenseCovered: ["dragon"],
      weaknesses: 5,
      pokemon: [{ pokemonId: "9003-000", nameJa: "テストくさ", types: ["grass"], exactMatch: true }],
    },
  ],
  abilityOptions: [
    {
      attackType: "water",
      pokemon: [
        {
          pokemonId: "9003-000",
          nameJa: "テストくさ",
          abilityId: "example-ability-none",
          multiplier: "1/2",
        },
      ],
    },
  ],
};

/**
 * recommendations に送る本文の形。契約(services/balance/api/openapi.yaml)では `limit` は必須ではないが、
 * 既定値(10)を持つため openapi-typescript が必須の項目として生成する。limit を省いたときに Web が既定値を
 * 決め打ちして送らない(サーバーの既定に任せる。ADR-0303 §7)ことを、この型で表す。
 */
type RecommendationsBody = Omit<Schemas["RecommendationsRequest"], "limit"> & {
  limit?: Schemas["RecommendationsRequest"]["limit"];
};

describe("threats(仮想敵)", () => {
  test("threats のパスに、端末 ID・セッション ID 付きで ThreatsRequest を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, threatsResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const members: Schemas["ThreatsRequest"]["members"] = [
      { pokemonId: "9001-000", moveIds: ["example-move-firepunch"], abilityId: "example-ability-none" },
    ];
    const threats: Schemas["ThreatsRequest"]["threats"] = [
      { pokemonId: "9002-000", moveIds: [] },
      { pokemonId: "9003-000", moveIds: ["example-move-tackle"], abilityId: "example-ability-none" },
    ];

    const result = await client.threats(members, threats);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://balance.test/api/balance/v1/team-balance/threats");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    const expected: Schemas["ThreatsRequest"] = {
      members: [
        { pokemonId: "9001-000", moveIds: ["example-move-firepunch"], abilityId: "example-ability-none" },
      ],
      threats: [
        { pokemonId: "9002-000", moveIds: [] },
        { pokemonId: "9003-000", moveIds: ["example-move-tackle"], abilityId: "example-ability-none" },
      ],
    };
    expect(bodyOf(init)).toEqual(expected);
    // 応答はそのまま運ぶ(safe / superEffective を倍率から判定し直さない。ADR-0303 §7)。
    expect(result).toEqual({ ok: true, value: threatsResponse });
  });

  test("HTTP エラーの {code: unknown_pokemon} をそのまま運ぶ", async () => {
    const error: Schemas["Error"] = { code: "unknown_pokemon", message: "unknown pokemonId: 9999-000" };
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(422, error)),
      ids,
    });
    const result = await client.threats(
      [{ pokemonId: "9001-000", moveIds: [] }],
      [{ pokemonId: "9999-000", moveIds: [] }],
    );
    expect(result).toEqual({ ok: false, error });
  });
});

describe("recommendations(おすすめタイプ)", () => {
  test("recommendations のパスに、端末 ID・セッション ID 付きで {members} を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, recommendationsResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const members: Schemas["RecommendationsRequest"]["members"] = [
      { pokemonId: "9001-000", moveIds: ["example-move-firepunch"], abilityId: "example-ability-none" },
      { pokemonId: "9002-000", moveIds: [] },
    ];

    const result = await client.recommendations(members);

    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://balance.test/api/balance/v1/team-balance/recommendations");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    const expected: RecommendationsBody = {
      members: [
        { pokemonId: "9001-000", moveIds: ["example-move-firepunch"], abilityId: "example-ability-none" },
        { pokemonId: "9002-000", moveIds: [] },
      ],
    };
    expect(bodyOf(init)).toEqual(expected);
    expect(result).toEqual({ ok: true, value: recommendationsResponse });
  });

  test("limit を渡さなければ limit のキー自体を送らない(既定の件数を Web が決めない)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, recommendationsResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.recommendations([{ pokemonId: "9001-000", moveIds: [] }]);

    const body = bodyOf(callAt(fetchMock).init);
    expect(Object.keys(body as Record<string, unknown>)).toEqual(["members"]);
  });

  test("limit を渡すとそのまま送る", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, recommendationsResponse));
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.recommendations([{ pokemonId: "9001-000", moveIds: [] }], 5);

    const expected: RecommendationsBody = { members: [{ pokemonId: "9001-000", moveIds: [] }], limit: 5 };
    expect(bodyOf(callAt(fetchMock).init)).toEqual(expected);
  });

  test("HTTP エラーの {code: invalid_request} をそのまま運ぶ", async () => {
    const error: Schemas["Error"] = { code: "invalid_request", message: "limit must be 1 to 20" };
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(400, error)),
      ids,
    });
    const result = await client.recommendations([{ pokemonId: "9001-000", moveIds: [] }], 21);
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

  test.each(cases)("threats: %s", async (_name, make) => {
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.threats(
      [{ pokemonId: "9001-000", moveIds: [] }],
      [{ pokemonId: "9002-000", moveIds: [] }],
    );
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("balance_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test.each(cases)("recommendations: %s", async (_name, make) => {
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.recommendations([{ pokemonId: "9001-000", moveIds: [] }]);
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

// ---- issue 67(P4-21): 2xx の契約外 JSON は balance_unavailable にする(ADR-0303 §6 追記) ----
//
// balance の応答はそのまま表示に使う(Web で倍率・集計を計算し直さない。ADR-0303 §1)ので、契約外の 200 を
// ok:true で通すと、例外は画面の描画(`response.teamSummary.map(...)`・`threat.matchups.map(...)` など)で起きる。
// 画面は「呼び出しは reject しない」前提で `.then` しか登録していない(BalanceScreen.tsx)。そこで応答を
// クライアントで検証し、契約外なら balance_unavailable の not ok にする。
//
// 検証の範囲は「画面がたどる形」まで:
//   - 応答がオブジェクトであること
//   - 契約で必須の最上位フィールドが存在し、配列であるべきところが配列であること
//   - 配列の各要素がオブジェクトであり、要素の必須の配列フィールド(members[].defense・types、
//     members[].coverage・moveIds・attackTypes、threats[].matchups・attackTypes、
//     candidates[].types・defenseCovered・offenseCovered・pokemon、abilityOptions[].pokemon)が配列であること
// leaf のスカラー(表示にそのまま出る文字列・数値・真偽値)と列挙の値までは検査しない。応答をそのまま運ぶ
// 設計で契約の全項目を TS に書き写すと、契約(services/balance/api/openapi.yaml)の二重管理になるため。

/** 配列の先頭を取り出す(テストの素材づくり。非 null 断言を使わないため)。 */
function firstOf<T>(items: readonly T[], label: string): T {
  const [first] = items;
  if (first === undefined) {
    throw new Error(`${label} の先頭が無い`);
  }
  return first;
}

const analyzeMember = firstOf(analyzeResponse.members, "analyzeResponse.members");
const coverageMember = firstOf(coverageResponse.members, "coverageResponse.members");
const threatResult = firstOf(threatsResponse.threats, "threatsResponse.threats");
const typeCandidate = firstOf(recommendationsResponse.candidates, "recommendationsResponse.candidates");
const abilityOption = firstOf(
  recommendationsResponse.abilityOptions,
  "recommendationsResponse.abilityOptions",
);

type BalanceCall = (client: BalanceClient) => Promise<BalanceResult<unknown>>;

const analyzeCall: BalanceCall = (client) => client.analyze([{ pokemonId: "9001-000" }]);
const coverageCall: BalanceCall = (client) => client.coverage([{ pokemonId: "9001-000", moveIds: [] }]);
const threatsCall: BalanceCall = (client) =>
  client.threats([{ pokemonId: "9001-000", moveIds: [] }], [{ pokemonId: "9002-000", moveIds: [] }]);
const recommendationsCall: BalanceCall = (client) =>
  client.recommendations([{ pokemonId: "9001-000", moveIds: [] }]);

/** 200 で body を返す fake fetch に対して、呼び出しが balance_unavailable の not ok になること。 */
async function expectBalanceUnavailable(call: BalanceCall, body: unknown): Promise<void> {
  const client = createBalanceClient({
    baseUrl: BASE_URL,
    fetch: fakeFetch(jsonResponse(200, body)),
    ids,
  });
  const result = await call(client);
  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect(result.error.code).toBe("balance_unavailable");
    expect(result.error.message).not.toBe("");
  }
}

/** 4つの呼び出しに共通の「そもそも応答のオブジェクトでない」本文(境界値)。 */
const notAnObjectBodies: ReadonlyArray<readonly [string, unknown]> = [
  ["空オブジェクト {}", {}],
  ["null", null],
  ["配列", []],
  ["文字列", "ok"],
  ["数値", 42],
  ["真偽値", true],
];

describe("2xx の契約外の応答は balance_unavailable(issue 67)", () => {
  const calls: ReadonlyArray<readonly [string, BalanceCall]> = [
    ["analyze", analyzeCall],
    ["coverage", coverageCall],
    ["threats", threatsCall],
    ["recommendations", recommendationsCall],
  ];

  for (const [name, call] of calls) {
    test.each(notAnObjectBodies)(
      `${name}: 200 の本文が %s なら balance_unavailable`,
      async (_label, body) => {
        await expectBalanceUnavailable(call, body);
      },
    );
  }

  const invalidAnalyzeBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["members が無い", { teamSummary: analyzeResponse.teamSummary }],
    ["teamSummary が無い", { members: analyzeResponse.members }],
    ["members が配列でない(文字列)", { ...analyzeResponse, members: "x" }],
    ["teamSummary が配列でない(オブジェクト)", { ...analyzeResponse, teamSummary: {} }],
    ["メンバーが null", { ...analyzeResponse, members: [null] }],
    ["メンバーが文字列", { ...analyzeResponse, members: ["9001-000"] }],
    [
      "メンバーの defense が無い",
      { ...analyzeResponse, members: [{ pokemonId: "9001-000", types: ["fire"] }] },
    ],
    [
      "メンバーの defense が配列でない",
      { ...analyzeResponse, members: [{ ...analyzeMember, defense: "x" }] },
    ],
    ["メンバーの types が配列でない", { ...analyzeResponse, members: [{ ...analyzeMember, types: "fire" }] }],
    ["集計の要素が null", { ...analyzeResponse, teamSummary: [null] }],
  ];

  test.each(invalidAnalyzeBodies)("analyze: %s なら balance_unavailable", async (_label, body) => {
    await expectBalanceUnavailable(analyzeCall, body);
  });

  const invalidCoverageBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["members が無い", { teamCoverage: coverageResponse.teamCoverage }],
    ["teamCoverage が無い", { members: coverageResponse.members }],
    ["teamCoverage が配列でない(文字列)", { ...coverageResponse, teamCoverage: "x" }],
    ["メンバーが null", { ...coverageResponse, members: [null] }],
    [
      "メンバーの coverage が無い",
      { ...coverageResponse, members: [{ pokemonId: "9001-000", moveIds: [], attackTypes: [] }] },
    ],
    [
      "メンバーの coverage が配列でない",
      { ...coverageResponse, members: [{ ...coverageMember, coverage: {} }] },
    ],
    [
      "メンバーの moveIds が配列でない",
      { ...coverageResponse, members: [{ ...coverageMember, moveIds: "m" }] },
    ],
    [
      "メンバーの attackTypes が配列でない",
      { ...coverageResponse, members: [{ ...coverageMember, attackTypes: "fire" }] },
    ],
  ];

  test.each(invalidCoverageBodies)("coverage: %s なら balance_unavailable", async (_label, body) => {
    await expectBalanceUnavailable(coverageCall, body);
  });

  const invalidThreatsBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["threats が無い", { results: threatsResponse.threats }],
    ["threats が配列でない(文字列)", { threats: "x" }],
    ["threats が配列でない(オブジェクト)", { threats: {} }],
    ["仮想敵が null", { threats: [null] }],
    [
      "仮想敵の matchups が無い",
      {
        threats: [
          { pokemonId: "9002-000", attackTypes: ["water"], safeMembers: 0, superEffectiveMembers: 0 },
        ],
      },
    ],
    ["仮想敵の matchups が配列でない", { threats: [{ ...threatResult, matchups: {} }] }],
    ["仮想敵の attackTypes が配列でない", { threats: [{ ...threatResult, attackTypes: "water" }] }],
  ];

  test.each(invalidThreatsBodies)("threats: %s なら balance_unavailable", async (_label, body) => {
    await expectBalanceUnavailable(threatsCall, body);
  });

  const invalidRecommendationsBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["defenseHoles が無い", { ...recommendationsResponse, defenseHoles: undefined }],
    ["offenseHoles が配列でない(文字列)", { ...recommendationsResponse, offenseHoles: "water" }],
    ["candidates が無い", { ...recommendationsResponse, candidates: undefined }],
    ["candidates が配列でない(オブジェクト)", { ...recommendationsResponse, candidates: {} }],
    ["abilityOptions が無い", { ...recommendationsResponse, abilityOptions: undefined }],
    ["abilityOptions が配列でない", { ...recommendationsResponse, abilityOptions: "none" }],
    ["候補が null", { ...recommendationsResponse, candidates: [null] }],
    [
      "候補の pokemon が配列でない",
      { ...recommendationsResponse, candidates: [{ ...typeCandidate, pokemon: {} }] },
    ],
    [
      "候補の types が配列でない",
      { ...recommendationsResponse, candidates: [{ ...typeCandidate, types: "grass" }] },
    ],
    [
      "候補の defenseCovered が配列でない",
      { ...recommendationsResponse, candidates: [{ ...typeCandidate, defenseCovered: "water" }] },
    ],
    [
      "特性の選択肢の pokemon が配列でない",
      { ...recommendationsResponse, abilityOptions: [{ ...abilityOption, pokemon: "x" }] },
    ],
  ];

  test.each(invalidRecommendationsBodies)(
    "recommendations: %s なら balance_unavailable",
    async (_label, body) => {
      await expectBalanceUnavailable(recommendationsCall, body);
    },
  );

  test("契約外の 200 でも Promise は reject しない(画面は .then しか登録していない)", async () => {
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(jsonResponse(200, {})), ids });
    await expect(client.analyze([{ pokemonId: "9001-000" }])).resolves.toMatchObject({ ok: false });
  });
});

describe("契約どおりの 2xx は従来どおり成功(検証で落とさない。issue 67)", () => {
  test("余分なフィールドがある応答もそのまま運ぶ(前方互換)", async () => {
    const body = { ...analyzeResponse, futureField: { note: "x" } };
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(jsonResponse(200, body)), ids });
    const result = await client.analyze([{ pokemonId: "9001-000" }]);
    expect(result).toEqual({ ok: true, value: body });
  });

  test("配列が空の応答も成功(件数は検査しない)", async () => {
    const empty: Schemas["AnalyzeResponse"] = { members: [], teamSummary: [] };
    const client = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(200, empty)),
      ids,
    });
    const result = await client.analyze([{ pokemonId: "9001-000" }]);
    expect(result).toEqual({ ok: true, value: empty });
  });

  test("threats・recommendations の空の応答も成功", async () => {
    const emptyThreats: Schemas["ThreatsResponse"] = { threats: [] };
    const threatsClient = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(200, emptyThreats)),
      ids,
    });
    await expect(
      threatsClient.threats(
        [{ pokemonId: "9001-000", moveIds: [] }],
        [{ pokemonId: "9002-000", moveIds: [] }],
      ),
    ).resolves.toEqual({ ok: true, value: emptyThreats });

    const emptyRecommendations: Schemas["RecommendationsResponse"] = {
      defenseHoles: [],
      offenseHoles: [],
      candidates: [],
      abilityOptions: [],
    };
    const recommendationsClient = createBalanceClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(200, emptyRecommendations)),
      ids,
    });
    await expect(
      recommendationsClient.recommendations([{ pokemonId: "9001-000", moveIds: [] }]),
    ).resolves.toEqual({ ok: true, value: emptyRecommendations });
  });

  test("任意のフィールド(abilityId)が省かれた応答も成功", async () => {
    const body = {
      ...analyzeResponse,
      members: [
        { pokemonId: analyzeMember.pokemonId, types: analyzeMember.types, defense: analyzeMember.defense },
      ],
    };
    const client = createBalanceClient({ baseUrl: BASE_URL, fetch: fakeFetch(jsonResponse(200, body)), ids });
    await expect(client.analyze([{ pokemonId: "9001-000" }])).resolves.toEqual({ ok: true, value: body });
  });
});
