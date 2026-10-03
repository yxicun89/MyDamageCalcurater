// AJ6: 調整 API のクライアント createAdjustClient(ADR-0319 §3・受け入れ条件 AC1)。
// judge/judgeClient.test.ts と同じ形に、AbortSignal(api/apiEngine.ts と同じ扱い)を足す。
// api/openapi.yaml を正とし、リクエスト本文・応答は openapi-typescript の生成型(api/openapi.gen.ts)で書く
// (契約とのずれを typecheck で検出する)。fetch は fake。
// 確かめること:
//   C1 4つの POST のパスは api/calc/adjust/{indices,min-sp-to-ko,min-sp-to-survive,allocation}
//   C2 ヘッダーは X-Device-Id・X-Session-Id・Content-Type: application/json、本文は request のまま(補わない)
//   C3 listMoveLearners は GET api/pokedex/moves/{key}/learners?limit=&offset=(key は encodeURIComponent)
//   C4 成功は {ok: true, value}(応答をそのまま運ぶ)
//   C5 HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ
//   C6 通信できない・JSON でない・エラー本文の形が不正なら adjust_unavailable(message は日本語で空でない)
//   C7 signal は fetch に渡す。呼ぶ前に abort 済みなら fetch せずに request_aborted、通信中の abort も request_aborted
//   C8 例外を投げない
// 架空データだけを使う(実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { REQUEST_ABORTED_CODE } from "../engine/types";
import { adjustClientText } from "../i18n/ja";
import {
  ADJUST_PATHS,
  ADJUST_UNAVAILABLE_CODE,
  createAdjustClient,
  moveLearnersPath,
  type AdjustClient,
  type AdjustResult,
} from "./adjustClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://adjust.test/";

/** 架空の自分の個体(SP 各 0..32・合計 <= 66)。 */
const self: Schemas["Individual"] = {
  speciesKey: "9001-000",
  level: 50,
  natureId: "test-nature-neutral",
  sp: { hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};

const opponent: Schemas["Individual"] = {
  speciesKey: "9002-000",
  level: 50,
  natureId: "test-nature-neutral",
  sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0 },
};

const indicesRequest: Schemas["AdjustIndicesRequest"] = {
  individual: self,
  moveId: "test-move-fire",
  modifier: 6144,
};

const indicesResult: Schemas["AdjustIndicesResult"] = {
  stats: { hp: 159, atk: 120, def: 90, spa: 80, spd: 90, spe: 130 },
  firepowerIndex: 16200,
  physicalBulkIndex: 14310,
  specialBulkIndex: 14310,
  hpLines: {
    hp: 159,
    sp: 4,
    current: "16n-1",
    next16n: { hp: 160, sp: 5, spDelta: 1 },
    prev16n: { hp: 144, sp: 0, spDelta: -4 },
    next16nMinus1: { hp: 175, sp: 20, spDelta: 16 },
    prev16nMinus1: null,
  },
};

const searchRequest: Schemas["AdjustSearchRequest"] = {
  format: "single",
  attacker: self,
  defender: opponent,
  moveId: "test-move-fire",
  hits: 2,
};

const koResult: Schemas["AdjustKOResult"] = {
  stat: "atk",
  searchLimit: 32,
  feasible: true,
  sp: 12,
  chancePercent: 100,
  unsupported: [],
};

const surviveResult: Schemas["AdjustSurviveResult"] = {
  stat: "def",
  searchLimit: 62,
  feasible: false,
  hpSp: 32,
  statSp: 30,
  totalSp: 62,
  bulkIndex: 30000,
  chancePercent: 87.5,
  unsupported: [{ target: "move", reason: "unsupported_effect", id: "test-move-fire" }],
};

const allocationRequest: Schemas["AdjustAllocationRequest"] = {
  self,
  mode: "bulk",
  focus: "both",
  ceiling: { hp: 32, def: 32, spd: 32 },
  minSpeed: 0,
};

const plan: Schemas["AdjustAllocPlan"] = {
  sp: { hp: 32, atk: 0, def: 18, spa: 0, spd: 16, spe: 0 },
  totalSp: 66,
  stats: { hp: 187, atk: 120, def: 108, spa: 80, spd: 106, spe: 130 },
  physicalBulk: 20196,
  specialBulk: 19822,
  speedMet: true,
  goalMet: false,
  chancePercent: 0,
};

const allocationResult: Schemas["AdjustAllocationResult"] = {
  remaining: 62,
  maxIndex: plan,
  minSp: null,
  unsupported: [],
};

const learners: Schemas["SpeciesSummary"][] = [
  { key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストカソウドリ", types: ["fire", "flying"] },
  { key: "9003-000", dexNo: 9003, form: 0, nameJa: "テストカソウソウ", types: ["grass"] },
];

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function fakeFetch(result: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    result instanceof Error ? Promise.reject(result) : Promise.resolve(result),
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

/** 4つの POST 操作を同じ表で回すための対応(操作名 → 呼び出し・送る request・返す応答・パス)。 */
type PostCase = readonly [
  name: string,
  call: (client: AdjustClient, signal?: AbortSignal) => Promise<AdjustResult<unknown>>,
  request: unknown,
  response: unknown,
  path: string,
];

const POST_CASES: readonly PostCase[] = [
  [
    "indices",
    (client, signal) => client.indices(indicesRequest, signal),
    indicesRequest,
    indicesResult,
    "api/calc/adjust/indices",
  ],
  [
    "minSpToKo",
    (client, signal) => client.minSpToKo(searchRequest, signal),
    searchRequest,
    koResult,
    "api/calc/adjust/min-sp-to-ko",
  ],
  [
    "minSpToSurvive",
    (client, signal) => client.minSpToSurvive(searchRequest, signal),
    searchRequest,
    surviveResult,
    "api/calc/adjust/min-sp-to-survive",
  ],
  [
    "allocation",
    (client, signal) => client.allocation(allocationRequest, signal),
    allocationRequest,
    allocationResult,
    "api/calc/adjust/allocation",
  ],
];

describe("C1 パス(api/openapi.yaml のまま)", () => {
  test("4つの POST のパスは api/calc/adjust/ の下(gateway の /api/calc/* の前方一致に乗る。ADR-0250 §1)", () => {
    expect(ADJUST_PATHS).toEqual({
      indices: "api/calc/adjust/indices",
      minSpToKo: "api/calc/adjust/min-sp-to-ko",
      minSpToSurvive: "api/calc/adjust/min-sp-to-survive",
      allocation: "api/calc/adjust/allocation",
    });
  });

  test("listMoveLearners のパスは api/pokedex/moves/{key}/learners に limit・offset のクエリを付ける", () => {
    expect(moveLearnersPath("test-move-fire", { limit: 50, offset: 0 })).toBe(
      "api/pokedex/moves/test-move-fire/learners?limit=50&offset=0",
    );
    expect(moveLearnersPath("test-move-fire", { limit: 50, offset: 100 })).toBe(
      "api/pokedex/moves/test-move-fire/learners?limit=50&offset=100",
    );
  });

  test("技の ID はパスの1セグメントとして encodeURIComponent する(/ や ? で別のパスにならない)", () => {
    expect(moveLearnersPath("a/b?c", { limit: 1, offset: 0 })).toBe(
      "api/pokedex/moves/a%2Fb%3Fc/learners?limit=1&offset=0",
    );
  });
});

describe("C2・C4 POST の4操作", () => {
  test.each(POST_CASES)(
    "%s: 端末 ID・セッション ID 付きで request をそのまま POST し、応答をそのまま返す",
    async (_name, call, request, response, path) => {
      const fetchMock = fakeFetch(jsonResponse(200, response));
      const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

      const result = await call(client);

      expect(fetchMock).toHaveBeenCalledTimes(1);
      const { url, init } = callAt(fetchMock);
      expect(url).toBe(`${BASE_URL}${path}`);
      expect(init.method).toBe("POST");
      expect(headersOf(init)).toMatchObject({
        "content-type": "application/json",
        "x-device-id": ids.deviceId,
        "x-session-id": ids.sessionId,
      });
      // 省略された欄を勝手に補わない(thresholdPercent・modifier などの既定はサーバーが持つ。ADR-0250 §3)。
      expect(bodyOf(init)).toEqual(request);
      // 応答はそのまま運ぶ(Web で計算し直さない・確率を丸めない。ADR-0250 §6)。
      expect(result).toEqual({ ok: true, value: response });
    },
  );

  test("基点 URL が同じオリジン(/)なら /api/calc/adjust/... を呼ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, indicesResult));
    const client = createAdjustClient({ baseUrl: "/", fetch: fetchMock, ids });
    await client.indices(indicesRequest);
    expect(callAt(fetchMock).url).toBe("/api/calc/adjust/indices");
  });
});

describe("C3 listMoveLearners(技を覚えるポケモン)", () => {
  test("GET で limit・offset を付けて呼び、端末 ID・セッション ID を付け、本文を送らない", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, learners));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.moveLearners("test-move-fire", { limit: 50, offset: 50 });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe(`${BASE_URL}api/pokedex/moves/test-move-fire/learners?limit=50&offset=50`);
    expect(init.method ?? "GET").toBe("GET");
    expect(init.body ?? null).toBeNull();
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(result).toEqual({ ok: true, value: learners });
  });

  test("一致なしの [] はそのまま空配列で返す(エラーにしない。ADR-0251 §1)", async () => {
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fakeFetch(jsonResponse(200, [])), ids });
    expect(await client.moveLearners("test-move-fire", { limit: 50, offset: 0 })).toEqual({
      ok: true,
      value: [],
    });
  });

  test.each([
    [404, "not_found", "move not found"],
    [503, "master_unavailable", "master is not ready"],
    [400, "invalid_input", "limit out of range"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createAdjustClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    expect(await client.moveLearners("test-move-fire", { limit: 50, offset: 0 })).toEqual({
      ok: false,
      error: { code, message },
    });
  });
});

describe("C5 HTTP エラーの封筒はそのまま運ぶ(画面が code から日本語を引く)", () => {
  test.each([
    [400, "invalid_input", "hits must be in 1..10"],
    [400, "invalid_enum", "unknown mode"],
    [400, "unknown_species", "unknown speciesKey"],
    [400, "unknown_move", "unknown moveId"],
    [400, "unknown_nature", "unknown natureId"],
    [503, "master_unavailable", "master is not ready"],
    [500, "type_chart_missing", "type chart missing"],
  ] as const)("HTTP %i の {code: %s}", async (status, code, message) => {
    for (const [, call] of POST_CASES) {
      const client = createAdjustClient({
        baseUrl: BASE_URL,
        fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
        ids,
      });
      expect(await call(client)).toEqual({ ok: false, error: { code, message } });
    }
  });
});

describe("C6 通信・応答の失敗は adjust_unavailable(自動の切り替え先は持たない)", () => {
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

  test.each(cases)("%s(5操作とも)", async (_name, make) => {
    const calls: ReadonlyArray<(client: AdjustClient) => Promise<AdjustResult<unknown>>> = [
      ...POST_CASES.map(
        ([, call]) =>
          (client: AdjustClient) =>
            call(client),
      ),
      (client) => client.moveLearners("test-move-fire", { limit: 50, offset: 0 }),
    ];
    for (const call of calls) {
      const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
      const result = await call(client);
      expect(result).toEqual({
        ok: false,
        error: { code: ADJUST_UNAVAILABLE_CODE, message: adjustClientText.unavailable },
      });
    }
  });

  test("adjust_unavailable のコードは Web 側の語で、契約の ErrorCode と衝突しない", () => {
    expect(ADJUST_UNAVAILABLE_CODE).toBe("adjust_unavailable");
  });
});

describe("C7 AbortSignal", () => {
  test.each(POST_CASES)("%s: signal を fetch の init.signal に渡す", async (_name, call, _req, response) => {
    const fetchMock = fakeFetch(jsonResponse(200, response));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const controller = new AbortController();

    await call(client, controller.signal);

    expect(callAt(fetchMock).init.signal).toBe(controller.signal);
  });

  test("moveLearners も signal を fetch に渡す", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, learners));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const controller = new AbortController();

    await client.moveLearners("test-move-fire", { limit: 50, offset: 0 }, controller.signal);

    expect(callAt(fetchMock).init.signal).toBe(controller.signal);
  });

  test("呼ぶ前に abort 済みなら fetch せずに request_aborted を返す(5操作とも)", async () => {
    const controller = new AbortController();
    controller.abort();
    const calls: ReadonlyArray<(client: AdjustClient) => Promise<AdjustResult<unknown>>> = [
      ...POST_CASES.map(
        ([, call]) =>
          (client: AdjustClient) =>
            call(client, controller.signal),
      ),
      (client) => client.moveLearners("test-move-fire", { limit: 50, offset: 0 }, controller.signal),
    ];
    for (const call of calls) {
      const fetchMock = fakeFetch(jsonResponse(200, {}));
      const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
      const result = await call(client);
      expect(fetchMock).not.toHaveBeenCalled();
      expect(result).toEqual({
        ok: false,
        error: { code: REQUEST_ABORTED_CODE, message: adjustClientText.aborted },
      });
    }
  });

  test("通信中に abort されて fetch が reject したら adjust_unavailable ではなく request_aborted", async () => {
    const controller = new AbortController();
    const fetchMock = vi.fn<typeof fetch>(() => {
      controller.abort();
      return Promise.reject(new DOMException("The operation was aborted.", "AbortError"));
    });
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.allocation(allocationRequest, controller.signal);

    expect(result).toEqual({
      ok: false,
      error: { code: REQUEST_ABORTED_CODE, message: adjustClientText.aborted },
    });
  });

  test("signal を渡さなければ init に signal を付けない(既存のクライアントと同じ)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, indicesResult));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    await client.indices(indicesRequest);
    expect("signal" in callAt(fetchMock).init).toBe(false);
  });
});

describe("C8 例外を投げない", () => {
  test("fetch が同期的に throw しても、失敗は戻り値で返す", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => {
      throw new TypeError("fetch is not available");
    });
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    await expect(client.indices(indicesRequest)).resolves.toMatchObject({
      ok: false,
      error: { code: ADJUST_UNAVAILABLE_CODE },
    });
    await expect(client.moveLearners("x", { limit: 1, offset: 0 })).resolves.toMatchObject({ ok: false });
  });
});
