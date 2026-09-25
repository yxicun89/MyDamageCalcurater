// P4-5: API の計算実装 createApiEngine(ADR-0301 §1〜§3)。画面の入力(解決済みの実体の DTO)を
// API のリクエスト(ID)に写して POST し、応答を DTO に戻す。fetch は fake(ADR-0301 §6)。
// 期待するリクエスト本文・応答は openapi-typescript の生成型(openapi.gen.ts)で書き、契約とのずれを typecheck で検出する。

import { describe, expect, test, vi } from "vitest";
import {
  REQUEST_ABORTED_CODE,
  type BulkRequest,
  type CalcEngine,
  type CalcRequest,
  type CalcResult,
  type DefenderPreset,
  type EngineResult,
  type Individual,
  type Move,
  type ReverseRequest,
  type Species,
  type TypeChart,
} from "../engine/types";
import type { MasterNature } from "../master/types";
import { createApiEngine } from "./apiEngine";
import type { ClientIds } from "./clientIds";
import type { components } from "./openapi.gen";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

/**
 * 性格の一覧。無補正の性格を2つ、ID の昇順と逆の順に置く(並びでなく ID の昇順で選ぶことの確認用。
 * ADR-0301 §2・ADR-0200 §2)。
 */
const natures: readonly MasterNature[] = [
  { id: "example-nature-neutral-b", nameJa: "テストむほせいB", plus: null, minus: null },
  { id: "example-nature-atk", nameJa: "テストつよい", plus: "atk", minus: "spa" },
  { id: "example-nature-neutral-a", nameJa: "テストむほせいA", plus: null, minus: null },
  { id: "example-nature-def", nameJa: "テストかたい", plus: "def", minus: "atk" },
];

const typeChart: TypeChart = { types: ["normal", "fire"], effectiveness: {} };

const attackerSpecies: Species = {
  key: "9001-000",
  dexNo: 9001,
  form: 0,
  nameJa: "テストほのお",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 70, spa: 90, spd: 70, spe: 95 },
  abilities: ["example-ability-adapt"],
};

const defenderSpecies: Species = {
  key: "9002-001",
  dexNo: 9002,
  form: 1,
  nameJa: "テストみず",
  types: ["water"],
  baseStats: { hp: 90, atk: 75, def: 80, spa: 95, spd: 85, spe: 70 },
  abilities: ["example-ability-none"],
};

const move: Move = {
  id: "example-move-firepunch",
  nameJa: "テストほのおパンチ",
  type: "fire",
  category: "physical",
  power: 75,
  priority: 0,
};

const zeroStats = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };
const attackerSp = { ...zeroStats, atk: 32 };

const attacker: Individual = {
  species: attackerSpecies,
  level: 50,
  nature: { plus: "atk", minus: "spa" },
  ability: { id: "example-ability-adapt", nameJa: "テストてきおう", effect: { stabMod: 8192 } },
  item: { id: "example-item-power", nameJa: "テストちからのたま", effect: { damageMod: 5324 } },
  sp: attackerSp,
  ranks: { atk: 1, def: 0, spa: 0, spd: 0, spe: 0 },
  teraType: "fire",
  status: "burn",
};

const defender: Individual = {
  species: defenderSpecies,
  level: 50,
  nature: { plus: "", minus: "" },
  // 特性の ID が "" は「特性なし」(domain/requests.ts の NO_ABILITY)。API では null。
  ability: { id: "", nameJa: "", effect: null },
  item: null,
  sp: { ...zeroStats, hp: 32 },
  teraType: "",
};

const calcRequest: CalcRequest = {
  format: "single",
  attacker,
  defender,
  move,
  field: { weather: "sun", terrain: "grassy" },
  critical: true,
  typeChart,
};

/** API の Individual(攻撃側)。実体 → ID の写し(ADR-0301 §2 の表)。 */
const apiAttacker: Schemas["Individual"] = {
  speciesKey: "9001-000",
  level: 50,
  natureId: "example-nature-atk",
  abilityId: "example-ability-adapt",
  itemId: "example-item-power",
  sp: attackerSp,
  ranks: { atk: 1, def: 0, spa: 0, spd: 0, spe: 0 },
  teraType: "fire",
  status: "burn",
};

/** API の Individual(防御側)。無補正は ID の昇順で最初の無補正性格、"" の特性・テラスタイプは null。 */
const apiDefender: Schemas["Individual"] = {
  speciesKey: "9002-001",
  level: 50,
  natureId: "example-nature-neutral-a",
  abilityId: null,
  itemId: null,
  sp: { ...zeroStats, hp: 32 },
  teraType: null,
};

const apiCalcResult: Schemas["CalcResult"] = {
  rolls: Array.from({ length: 16 }, (_, index) => 60 + index),
  minDamage: 60,
  maxDamage: 75,
  minPercent: 34.2,
  maxPercent: 42.9,
  defenderHP: 175,
  effectiveness: 0.5,
  stab: true,
  category: "physical",
  ko: { hits: 3, guaranteed: false, chancePercent: 12.34, displayChancePercent: 12.3 },
  unsupported: [],
};

/** key を持たない(undefined を入れるのでなく、プロパティごと無い)コピー。省略時の写し方の確認用。 */
function omit<T extends object, K extends keyof T>(value: T, key: K): Omit<T, K> {
  return Object.fromEntries(Object.entries(value).filter(([name]) => name !== key)) as Omit<T, K>;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/** 決め打ちの応答を返す fake の fetch。呼ばれた URL・init を記録する。 */
function fakeFetch(respond: () => Promise<Response>) {
  return vi.fn<typeof fetch>(() => respond());
}

interface SentRequest {
  readonly url: string;
  readonly method: string;
  readonly headers: Headers;
  readonly body: unknown;
}

/** fake の fetch の n 番目の呼び出しを、URL・メソッド・ヘッダー・JSON 本文に読み直す。 */
function sentRequest(fetchMock: ReturnType<typeof fakeFetch>, index = 0): SentRequest {
  const call = fetchMock.mock.calls[index];
  if (call === undefined) {
    throw new Error(`fetch の ${String(index + 1)} 回目の呼び出しが無い`);
  }
  const [input, init] = call;
  const url = input instanceof Request ? input.url : input instanceof URL ? input.href : input;
  const bodyText = init?.body;
  if (typeof bodyText !== "string") {
    throw new Error("本文は JSON 文字列で送る");
  }
  return {
    url,
    method: init?.method ?? "GET",
    headers: new Headers(init?.headers),
    body: JSON.parse(bodyText) as unknown,
  };
}

function engineWith(fetchMock: typeof fetch, baseUrl = "http://api.test/") {
  return createApiEngine({ baseUrl, fetch: fetchMock, master: { natures }, ids });
}

describe("リクエスト: URL・メソッド・ヘッダー(ADR-0301 §3)", () => {
  test.each([
    ["calc", "http://api.test/", "http://api.test/api/calc"],
    ["calcBulk", "http://api.test/", "http://api.test/api/calc/bulk"],
    ["calcReverse", "http://api.test/", "http://api.test/api/calc/reverse"],
    ["calcBulk", "/", "/api/calc/bulk"],
    ["calcBulk", "https://example.test/pokecalc/", "https://example.test/pokecalc/api/calc/bulk"],
  ] as const)("%s は基点 %s に対して %s へ POST する", async (method, baseUrl, expectedUrl) => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({ code: "internal", message: "x" }, 500)));
    const engine = engineWith(fetchMock, baseUrl);
    switch (method) {
      case "calc":
        await engine.calc(calcRequest);
        break;
      case "calcBulk":
        await engine.calcBulk(bulkRequest);
        break;
      case "calcReverse":
        await engine.calcReverse(reverseRequest);
        break;
    }
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const sent = sentRequest(fetchMock);
    expect(sent.url).toBe(expectedUrl);
    expect(sent.method).toBe("POST");
  });

  test("Content-Type: application/json・X-Device-Id・X-Session-Id を付ける", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc(calcRequest);
    const { headers } = sentRequest(fetchMock);
    expect(headers.get("Content-Type")).toMatch(/^application\/json/);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
  });
});

describe("calc: 実体 → ID の写像(ADR-0301 §2)", () => {
  test("本文は API の CalcRequest(speciesKey・natureId・abilityId・itemId・moveId・options.critical。typeChart は送らない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc(calcRequest);

    const expected: Schemas["CalcRequest"] = {
      format: "single",
      attacker: apiAttacker,
      defender: apiDefender,
      moveId: "example-move-firepunch",
      field: { weather: "sun", terrain: "grassy" },
      options: { critical: true },
    };
    const { body } = sentRequest(fetchMock);
    expect(body).toEqual(expected);
    expect(body).not.toHaveProperty("typeChart");
    expect(body).not.toHaveProperty("move");
  });

  test('天候・地形の "" は送らない(API の enum は "" を拒否する。WASM は "" を「なし」として受ける)', async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc({ ...calcRequest, field: { weather: "", terrain: "" } });
    const { body } = sentRequest(fetchMock);
    expect(body).toHaveProperty("field");
    expect(body).not.toHaveProperty("field.weather");
    expect(body).not.toHaveProperty("field.terrain");
  });

  test('状態異常の "" は送らない(API の既定 none に任せる)', async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc({ ...calcRequest, attacker: { ...calcRequest.attacker, status: "" } });
    expect(sentRequest(fetchMock).body).not.toHaveProperty("attacker.status");
  });

  test("critical を省いたら options も送らない(サーバーの既定 false)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc(omit(calcRequest, "critical"));
    expect(sentRequest(fetchMock).body).not.toHaveProperty("options");
  });

  test("成功の応答は CalcResult の形のまま返す", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    const result = await engineWith(fetchMock).calc(calcRequest);
    // unsupported は API のみの項目で、Web の CalcResult(engine/types)にはまだ無い(Web はまだ表示しない。
    // ADR-0123 §6・DECISIONS.md 2026-09-25)。issue 67 の前方互換どおり DTO には混ざらない。
    const expected: CalcResult = omit(
      { ...apiCalcResult, ko: { ...apiCalcResult.ko, chancePercent: 12.34 } },
      "unsupported",
    );
    expect(result).toEqual({ ok: true, value: expected });
  });

  test("ko.chancePercent が省かれた応答は 0 で埋める(API では任意、DTO では必須。確定・倒せないときの値)", async () => {
    const koWithoutChance = omit(apiCalcResult.ko, "chancePercent");
    const fetchMock = fakeFetch(() =>
      Promise.resolve(jsonResponse({ ...apiCalcResult, ko: koWithoutChance })),
    );
    const result = await engineWith(fetchMock).calc(calcRequest);
    expect(result).toMatchObject({ ok: true, value: { ko: { ...koWithoutChance, chancePercent: 0 } } });
  });
});

describe("性格 → natureId の選び方(ADR-0301 §2・ADR-0200 §2)", () => {
  function withAttackerNature(nature: Individual["nature"]): CalcRequest {
    return { ...calcRequest, attacker: { ...attacker, nature } };
  }

  test.each([
    ["+A/-C", { plus: "atk", minus: "spa" }, "example-nature-atk"],
    ["+B/-A", { plus: "def", minus: "atk" }, "example-nature-def"],
    [
      "無補正(一覧の並びでなく ID の昇順で最初の無補正性格)",
      { plus: "", minus: "" },
      "example-nature-neutral-a",
    ],
  ] as const)("%s は %s", async (_label, nature, expectedId) => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    await engineWith(fetchMock).calc(withAttackerNature(nature));
    const body = sentRequest(fetchMock).body as Schemas["CalcRequest"];
    expect(body.attacker.natureId).toBe(expectedId);
  });

  test("plus と minus が同じステータスの性格も無補正として扱う(calc-svc の Store.NatureID と同じ規則)", async () => {
    const withSameStat: readonly MasterNature[] = [
      { id: "example-nature-z-neutral", nameJa: "テストむほせいZ", plus: null, minus: null },
      { id: "example-nature-a-same", nameJa: "テストすなお", plus: "spe", minus: "spe" },
      { id: "example-nature-atk", nameJa: "テストつよい", plus: "atk", minus: "spa" },
    ];
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    const engine = createApiEngine({
      baseUrl: "http://api.test/",
      fetch: fetchMock,
      master: { natures: withSameStat },
      ids,
    });
    await engine.calc(withAttackerNature({ plus: "", minus: "" }));
    const body = sentRequest(fetchMock).body as Schemas["CalcRequest"];
    expect(body.attacker.natureId).toBe("example-nature-a-same");
  });

  test.each([
    ["攻撃側", { ...calcRequest, attacker: { ...attacker, nature: { plus: "spe", minus: "atk" } } }],
    ["防御側", { ...calcRequest, defender: { ...defender, nature: { plus: "spd", minus: "atk" } } }],
  ] as const)("%s の性格がマスタに無ければ fetch せず unknown_nature の not ok", async (_label, request) => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    const result = await engineWith(fetchMock).calc(request);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("unknown_nature");
      expect(result.error.message).not.toBe("");
    }
  });

  test("無補正の性格がマスタに1つも無ければ、無補正の個体は unknown_nature(fetch しない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiCalcResult)));
    const engine = createApiEngine({
      baseUrl: "http://api.test/",
      fetch: fetchMock,
      master: { natures: natures.filter((nature) => nature.plus !== null) },
      ids,
    });
    const result = await engine.calc(calcRequest);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result).toMatchObject({ ok: false, error: { code: "unknown_nature" } });
  });
});

const bulkRequest: BulkRequest = {
  format: "single",
  attacker,
  defenderSpecies,
  move,
  itemVariants: [
    null,
    { id: "example-item-def", nameJa: "テストぼうぎょだま", effect: { statMods: { def: 6144 } } },
  ],
  typeChart,
};

const apiBulkResult: Schemas["BulkCalcResult"] = {
  defenderSpeciesKey: "9002-001",
  rows: [
    {
      preset: "none",
      presetLabel: "無振り",
      itemId: null,
      defender: {
        sp: zeroStats,
        nature: { plus: null, minus: null },
        natureId: "example-nature-neutral-a",
        stats: { hp: 165, atk: 95, def: 100, spa: 115, spd: 105, spe: 90 },
      },
      result: apiCalcResult,
    },
    {
      preset: "hb_boost",
      presetLabel: "H振り+B補正",
      itemId: "example-item-def",
      defender: {
        sp: { ...zeroStats, hp: 32 },
        nature: { plus: "def", minus: "atk" },
        natureId: null,
        stats: { hp: 197, atk: 85, def: 110, spa: 115, spd: 105, spe: 90 },
      },
      result: apiCalcResult,
    },
  ],
};

describe("calcBulk", () => {
  test("本文は API の BulkCalcRequest(defenderSpeciesKey・itemVariants の ID/null。typeChart は送らない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiBulkResult)));
    await engineWith(fetchMock).calcBulk(bulkRequest);

    const expected: Schemas["BulkCalcRequest"] = {
      format: "single",
      attacker: apiAttacker,
      defenderSpeciesKey: "9002-001",
      moveId: "example-move-firepunch",
      itemVariants: [null, "example-item-def"],
    };
    const { body } = sentRequest(fetchMock);
    expect(body).toEqual(expected);
    expect(body).not.toHaveProperty("typeChart");
    expect(body).not.toHaveProperty("defenderSpecies");
  });

  test("itemVariants を省いたら itemVariants を送らない", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiBulkResult)));
    await engineWith(fetchMock).calcBulk(omit(bulkRequest, "itemVariants"));
    expect(sentRequest(fetchMock).body).not.toHaveProperty("itemVariants");
  });

  test("カスタムのプリセット定義(presets)は API で表せないので、fetch せずに invalid_preset", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiBulkResult)));
    const custom: DefenderPreset = {
      key: "custom",
      label: "カスタム",
      sp: { hp: 32, atk: 0, def: 16, spa: 0, spd: 0, spe: 0 },
      nature: { plus: "", minus: "" },
      applies: "",
    };
    const result = await engineWith(fetchMock).calcBulk({ ...bulkRequest, presets: [custom] });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result).toMatchObject({ ok: false, error: { code: "invalid_preset" } });
  });

  test("presetKeys は presets として送る(API の DefenderPreset の文字列)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiBulkResult)));
    await engineWith(fetchMock).calcBulk({ ...bulkRequest, presetKeys: ["none", "hb"] });
    expect(sentRequest(fetchMock).body).toMatchObject({ presets: ["none", "hb"] });
  });

  test('応答の行は DTO に戻す: NatureModifier の null は ""、itemId の null は ""、natureId は捨てる', async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiBulkResult)));
    const result = await engineWith(fetchMock).calcBulk(bulkRequest);
    expect(result).toEqual({
      ok: true,
      value: {
        defenderSpeciesKey: "9002-001",
        rows: [
          {
            preset: "none",
            presetLabel: "無振り",
            itemId: "",
            defender: {
              sp: zeroStats,
              nature: { plus: "", minus: "" },
              stats: { hp: 165, atk: 95, def: 100, spa: 115, spd: 105, spe: 90 },
            },
            // unsupported は Web の CalcResult にまだ無い(DTO には混ざらない。上と同じ理由)。
            result: omit(apiCalcResult, "unsupported"),
          },
          {
            preset: "hb_boost",
            presetLabel: "H振り+B補正",
            itemId: "example-item-def",
            defender: {
              sp: { ...zeroStats, hp: 32 },
              nature: { plus: "def", minus: "atk" },
              stats: { hp: 197, atk: 85, def: 110, spa: 115, spd: 105, spe: 90 },
            },
            result: omit(apiCalcResult, "unsupported"),
          },
        ],
      },
    });
    if (result.ok) {
      for (const row of result.value.rows) {
        expect(row.defender).not.toHaveProperty("natureId");
      }
    }
  });
});

const reverseRequest: ReverseRequest = {
  format: "single",
  side: "defender",
  known: attacker,
  unknownSpecies: defenderSpecies,
  move,
  critical: false,
  itemCandidates: [{ id: "example-item-def", nameJa: "テストぼうぎょだま", effect: null }, null],
  observations: [{ percent: 45 }, { percentTenths: 453, note: "2発目" }, { damage: 80 }],
  maxCandidates: 5,
  typeChart,
};

const apiReverseResult: Schemas["ReverseResult"] = {
  side: "defender",
  stat: "def",
  assumedHpSp: 32,
  exactCount: 1,
  candidates: [
    {
      natureClass: "neutral",
      nature: { plus: null, minus: null },
      natureId: "example-nature-neutral-a",
      itemId: null,
      ranges: [{ min: 10, max: 12 }],
      spCount: 3,
      exact: true,
      mismatch: 0,
      support: 4,
      minPercent: 40.2,
      maxPercent: 47.8,
      unsupported: [],
    },
    {
      natureClass: "plus",
      nature: { plus: "def", minus: "atk" },
      natureId: "example-nature-def",
      itemId: "example-item-def",
      ranges: [{ min: 0, max: 2 }],
      spCount: 3,
      exact: false,
      mismatch: 7,
      support: 1,
      minPercent: 44.1,
      maxPercent: 52.0,
      unsupported: [],
    },
  ],
};

describe("calcReverse", () => {
  test("本文は API の ReverseRequest(known・unknownSpeciesKey・itemCandidates・observations。maxCandidates・typeChart は送らない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiReverseResult)));
    await engineWith(fetchMock).calcReverse(reverseRequest);

    // maxCandidates は API では既定 0(無制限)。画面は切らずに全候補を受け取る(ADR-0301 §2 の表に無い)。
    const expected: Omit<Schemas["ReverseRequest"], "maxCandidates"> = {
      format: "single",
      side: "defender",
      known: apiAttacker,
      unknownSpeciesKey: "9002-001",
      moveId: "example-move-firepunch",
      options: { critical: false },
      itemCandidates: ["example-item-def", null],
      observations: [{ percent: 45 }, { percentTenths: 453, note: "2発目" }, { damage: 80 }],
    };
    const { body } = sentRequest(fetchMock);
    expect(body).toEqual(expected);
    expect(body).not.toHaveProperty("maxCandidates");
    expect(body).not.toHaveProperty("typeChart");
  });

  test("format を省いた逆算リクエストは single を送る(API では format が必須)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiReverseResult)));
    await engineWith(fetchMock).calcReverse(omit(reverseRequest, "format"));
    expect(sentRequest(fetchMock).body).toMatchObject({ format: "single" });
  });

  test("itemCandidates を省いたら itemCandidates を送らない", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiReverseResult)));
    await engineWith(fetchMock).calcReverse(omit(reverseRequest, "itemCandidates"));
    expect(sentRequest(fetchMock).body).not.toHaveProperty("itemCandidates");
  });

  test('応答の候補は DTO に戻す: NatureModifier の null は ""、itemId の null は ""、natureId は捨てる', async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(apiReverseResult)));
    const result = await engineWith(fetchMock).calcReverse(reverseRequest);
    expect(result).toEqual({
      ok: true,
      value: {
        side: "defender",
        stat: "def",
        assumedHpSp: 32,
        exactCount: 1,
        candidates: [
          {
            natureClass: "neutral",
            nature: { plus: "", minus: "" },
            itemId: "",
            ranges: [{ min: 10, max: 12 }],
            spCount: 3,
            exact: true,
            mismatch: 0,
            support: 4,
            minPercent: 40.2,
            maxPercent: 47.8,
          },
          {
            natureClass: "plus",
            nature: { plus: "def", minus: "atk" },
            itemId: "example-item-def",
            ranges: [{ min: 0, max: 2 }],
            spCount: 3,
            exact: false,
            mismatch: 7,
            support: 1,
            minPercent: 44.1,
            maxPercent: 52.0,
          },
        ],
      },
    });
    if (result.ok) {
      for (const candidate of result.value.candidates) {
        expect(candidate).not.toHaveProperty("natureId");
      }
    }
  });
});

describe("エラーの写し(ADR-0301 §2 の表・§4)", () => {
  test.each([
    [400, "unknown_species", "speciesKey がマスタに無い"],
    [400, "invalid_input", "SP の合計が 66 を超える"],
    [503, "master_unavailable", "マスタを参照できない"],
    [500, "internal", "内部エラー"],
  ] as const)("HTTP %i の Error 本文 %s は code・message をそのまま運ぶ", async (status, code, message) => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({ code, message }, status)));
    const result = await engineWith(fetchMock).calcBulk(bulkRequest);
    expect(result).toEqual({ ok: false, error: { code, message } });
  });

  test.each([
    ["通信できない(fetch が reject)", () => Promise.reject(new TypeError("Failed to fetch"))],
    [
      "エラー本文が JSON でない(gateway の HTML など)",
      () => Promise.resolve(new Response("<html>502 Bad Gateway</html>", { status: 502 })),
    ],
    ["エラー本文の JSON に code が無い", () => Promise.resolve(jsonResponse({ message: "x" }, 500))],
    ["200 だが本文が JSON でない", () => Promise.resolve(new Response("not json", { status: 200 }))],
  ] as const)("%s は例外にせず engine_unavailable の not ok", async (_label, respond) => {
    const fetchMock = fakeFetch(respond);
    const engine = engineWith(fetchMock);
    for (const result of [
      await engine.calc(calcRequest),
      await engine.calcBulk(bulkRequest),
      await engine.calcReverse(reverseRequest),
    ]) {
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error.code).toBe("engine_unavailable");
        expect(result.error.message).not.toBe("");
      }
    }
  });
});

// issue 113(P4-18): 入力が変わったときに、画面が先行の計算を取り消せること。
// API 実装は signal を fetch にそのまま渡し、取り消しは engine_unavailable(API が使えない)と区別して
// REQUEST_ABORTED_CODE で返す(ADR-0300 §11・ADR-0301 §4 追記)。
describe("計算の取り消し(AbortSignal。issue 113)", () => {
  type CallWithSignal = (engine: CalcEngine, signal?: AbortSignal) => Promise<EngineResult<unknown>>;

  /** 3つの計算を同じ形で呼ぶ表(成功の応答本文は、その計算が読める形にする)。 */
  const callsWithSignal: ReadonlyArray<readonly [string, CallWithSignal, unknown]> = [
    ["calc", (engine, signal) => engine.calc(calcRequest, signal), apiCalcResult],
    ["calcBulk", (engine, signal) => engine.calcBulk(bulkRequest, signal), apiBulkResult],
    ["calcReverse", (engine, signal) => engine.calcReverse(reverseRequest, signal), apiReverseResult],
  ];

  /** fake の fetch の n 番目の呼び出しに渡った init.signal。 */
  function sentSignal(fetchMock: ReturnType<typeof fakeFetch>, index = 0): AbortSignal | null | undefined {
    const call = fetchMock.mock.calls[index];
    if (call === undefined) {
      throw new Error(`fetch の ${String(index + 1)} 回目の呼び出しが無い`);
    }
    return call[1]?.signal;
  }

  /** fetch / Response.json が abort されたときに投げる例外(ブラウザと同じ DOMException)。 */
  function abortException(): DOMException {
    return new DOMException("The operation was aborted.", "AbortError");
  }

  test.each(callsWithSignal)("%s: 渡した signal をそのまま fetch に渡す", async (_label, call, body) => {
    const controller = new AbortController();
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(body)));
    const result = await call(engineWith(fetchMock), controller.signal);
    expect(result.ok).toBe(true);
    expect(sentSignal(fetchMock)).toBe(controller.signal);
  });

  test.each(callsWithSignal)(
    "%s: signal を渡さなければ init に signal を付けない",
    async (_label, call, body) => {
      const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(body)));
      await call(engineWith(fetchMock));
      expect(sentSignal(fetchMock) ?? undefined).toBeUndefined();
    },
  );

  test.each(callsWithSignal)(
    "%s: すでに abort 済みの signal なら fetch せずに request_aborted を返す",
    async (_label, call, body) => {
      const controller = new AbortController();
      controller.abort();
      const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(body)));
      const result = await call(engineWith(fetchMock), controller.signal);
      expect(fetchMock).not.toHaveBeenCalled();
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error.code).toBe(REQUEST_ABORTED_CODE);
        expect(result.error.message).not.toBe("");
      }
    },
  );

  test.each(callsWithSignal)(
    "%s: 送信後に abort されたら request_aborted にする(engine_unavailable を表示させない)",
    async (_label, call) => {
      const controller = new AbortController();
      const fetchMock = fakeFetch(() => {
        controller.abort();
        return Promise.reject(abortException());
      });
      const result = await call(engineWith(fetchMock), controller.signal);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error.code).toBe(REQUEST_ABORTED_CODE);
      }
    },
  );

  test("応答の本文を読んでいる途中で abort されても request_aborted にする", async () => {
    const controller = new AbortController();
    const fetchMock = fakeFetch(() => {
      controller.abort();
      // 本文の読み取り(response.json())が abort で reject する応答。
      const aborted = {
        ok: true,
        status: 200,
        json: () => Promise.reject(abortException()),
      } as unknown as Response;
      return Promise.resolve(aborted);
    });
    const result = await engineWith(fetchMock).calcReverse(reverseRequest, controller.signal);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(REQUEST_ABORTED_CODE);
    }
  });

  test("abort していない通信の失敗は、signal を渡していても従来どおり engine_unavailable", async () => {
    const controller = new AbortController();
    const fetchMock = fakeFetch(() => Promise.reject(new TypeError("Failed to fetch")));
    const result = await engineWith(fetchMock).calcReverse(reverseRequest, controller.signal);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("engine_unavailable");
    }
  });

  test("取り消しの code は engine_unavailable と別の値(画面が見分けられる)", () => {
    expect(REQUEST_ABORTED_CODE).not.toBe("engine_unavailable");
  });
});

// ---- issue 67(P4-21): 2xx の契約外 JSON は engine_unavailable にする(ADR-0301 §4 追記) ----
//
// HTTP 200 で JSON として読めても、本文が契約(api/openapi.yaml)の応答の形でなければ、写像関数が
// `result.rolls` や `result.rows.map(...)` で例外になる。画面は「計算は reject しない」前提で `.then` しか
// 登録していない(CalcScreen.tsx)ので、例外は画面まで伝わって壊れる。そこで応答を実行時に検証し、
// 契約外なら engine_unavailable(通信・応答の失敗。ADR-0301 §4)の not ok として返す。
//
// 検証の範囲は「写像関数(mapCalcResult / mapBulkResult / mapReverseResult)が読むフィールド」:
//   - 読むフィールドは、存在すること・JS 上の種類(数値 / 真偽値 / 文字列 / 配列 / オブジェクト)が合うこと
//   - 列挙の値そのもの・数値の範囲・配列の件数は見ない(サーバーが語彙を増やしても Web を壊さないため)
//   - 写像が `?? ` で既定値を補うフィールド(ko.chancePercent・itemId・nature.plus/minus)と、写像が捨てる
//     フィールド(natureId)は、欠落・null を許す(Go の omitempty で省かれた応答を落とさないため)

/** 配列の先頭を取り出す(テストの素材づくり。非 null 断言を使わないため)。 */
function firstOf<T>(items: readonly T[], label: string): T {
  const [first] = items;
  if (first === undefined) {
    throw new Error(`${label} の先頭が無い`);
  }
  return first;
}

const bulkRow = firstOf(apiBulkResult.rows, "apiBulkResult.rows");
const reverseCandidate = firstOf(apiReverseResult.candidates, "apiReverseResult.candidates");

type EngineCall = (engine: CalcEngine) => Promise<EngineResult<unknown>>;

const calcCall: EngineCall = (engine) => engine.calc(calcRequest);
const bulkCall: EngineCall = (engine) => engine.calcBulk(bulkRequest);
const reverseCall: EngineCall = (engine) => engine.calcReverse(reverseRequest);

/** 200 で body を返す fake fetch に対して、呼び出しが engine_unavailable の not ok になること。 */
async function expectUnavailableFor(call: EngineCall, body: unknown): Promise<void> {
  const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse(body)));
  const result = await call(engineWith(fetchMock));
  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect(result.error.code).toBe("engine_unavailable");
    expect(result.error.message).not.toBe("");
  }
}

/** 3つの計算に共通の「そもそも応答のオブジェクトでない」本文(境界値)。 */
const notAnObjectBodies: ReadonlyArray<readonly [string, unknown]> = [
  ["空オブジェクト {}", {}],
  ["null", null],
  ["配列", []],
  ["文字列", "ok"],
  ["数値", 42],
  ["真偽値", true],
];

describe("2xx の契約外の応答は engine_unavailable(issue 67)", () => {
  test.each(notAnObjectBodies)("calc: 200 の本文が %s なら engine_unavailable", async (_label, body) => {
    await expectUnavailableFor(calcCall, body);
  });

  test.each(notAnObjectBodies)("calcBulk: 200 の本文が %s なら engine_unavailable", async (_label, body) => {
    await expectUnavailableFor(bulkCall, body);
  });

  test.each(notAnObjectBodies)(
    "calcReverse: 200 の本文が %s なら engine_unavailable",
    async (_label, body) => {
      await expectUnavailableFor(reverseCall, body);
    },
  );

  const invalidCalcBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["rolls が無い", omit(apiCalcResult, "rolls")],
    ["rolls が配列でない(文字列)", { ...apiCalcResult, rolls: "60,61" }],
    ["rolls が配列でない(オブジェクト)", { ...apiCalcResult, rolls: { 0: 60 } }],
    ["rolls の要素が数値でない", { ...apiCalcResult, rolls: [...apiCalcResult.rolls.slice(0, 15), "75"] }],
    ["minDamage が数値でない", { ...apiCalcResult, minDamage: "60" }],
    ["defenderHP が null", { ...apiCalcResult, defenderHP: null }],
    ["maxPercent が無い", omit(apiCalcResult, "maxPercent")],
    ["stab が真偽値でない", { ...apiCalcResult, stab: 1 }],
    ["category が文字列でない", { ...apiCalcResult, category: 3 }],
    ["ko が無い", omit(apiCalcResult, "ko")],
    ["ko が配列", { ...apiCalcResult, ko: [] }],
    ["ko が null", { ...apiCalcResult, ko: null }],
    ["ko.hits が無い", { ...apiCalcResult, ko: omit(apiCalcResult.ko, "hits") }],
    [
      "ko.displayChancePercent が無い",
      { ...apiCalcResult, ko: omit(apiCalcResult.ko, "displayChancePercent") },
    ],
    ["ko.guaranteed が真偽値でない", { ...apiCalcResult, ko: { ...apiCalcResult.ko, guaranteed: "true" } }],
    [
      "ko.chancePercent があるが数値でない(任意でも、あるなら型は合うこと)",
      { ...apiCalcResult, ko: { ...apiCalcResult.ko, chancePercent: "12.34" } },
    ],
  ];

  test.each(invalidCalcBodies)("calc: %s なら engine_unavailable", async (_label, body) => {
    await expectUnavailableFor(calcCall, body);
  });

  const invalidBulkBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["rows が無い", omit(apiBulkResult, "rows")],
    ["rows が配列でない(文字列)", { ...apiBulkResult, rows: "none" }],
    ["rows が配列でない(オブジェクト)", { ...apiBulkResult, rows: {} }],
    ["defenderSpeciesKey が無い", omit(apiBulkResult, "defenderSpeciesKey")],
    ["defenderSpeciesKey が文字列でない", { ...apiBulkResult, defenderSpeciesKey: 9002 }],
    ["行が空オブジェクト", { ...apiBulkResult, rows: [{}] }],
    ["行が null", { ...apiBulkResult, rows: [null] }],
    ["行の presetLabel が文字列でない", { ...apiBulkResult, rows: [{ ...bulkRow, presetLabel: 7 }] }],
    ["行の itemId が文字列でも null でもない", { ...apiBulkResult, rows: [{ ...bulkRow, itemId: 5 }] }],
    ["行の result が無い", { ...apiBulkResult, rows: [omit(bulkRow, "result")] }],
    [
      "行の result が契約外(入れ子も同じ規則で検査する)",
      { ...apiBulkResult, rows: [{ ...bulkRow, result: omit(apiCalcResult, "ko") }] },
    ],
    ["行の defender が無い", { ...apiBulkResult, rows: [omit(bulkRow, "defender")] }],
    ["行の defender が文字列", { ...apiBulkResult, rows: [{ ...bulkRow, defender: "hp32" }] }],
    [
      "行の defender.nature が無い",
      { ...apiBulkResult, rows: [{ ...bulkRow, defender: omit(bulkRow.defender, "nature") }] },
    ],
    [
      "行の defender.nature.plus が文字列でも null でもない",
      {
        ...apiBulkResult,
        rows: [{ ...bulkRow, defender: { ...bulkRow.defender, nature: { plus: 1, minus: null } } }],
      },
    ],
    [
      "行の defender.sp が無い",
      { ...apiBulkResult, rows: [{ ...bulkRow, defender: omit(bulkRow.defender, "sp") }] },
    ],
    [
      "行の defender.sp のステータスが数値でない",
      {
        ...apiBulkResult,
        rows: [{ ...bulkRow, defender: { ...bulkRow.defender, sp: { ...bulkRow.defender.sp, hp: "32" } } }],
      },
    ],
    [
      "行の defender.stats が配列",
      { ...apiBulkResult, rows: [{ ...bulkRow, defender: { ...bulkRow.defender, stats: [] } }] },
    ],
  ];

  test.each(invalidBulkBodies)("calcBulk: %s なら engine_unavailable", async (_label, body) => {
    await expectUnavailableFor(bulkCall, body);
  });

  const invalidReverseBodies: ReadonlyArray<readonly [string, unknown]> = [
    ["candidates が無い", omit(apiReverseResult, "candidates")],
    ["candidates が配列でない(オブジェクト)", { ...apiReverseResult, candidates: {} }],
    ["candidates が配列でない(文字列)", { ...apiReverseResult, candidates: "none" }],
    ["side が無い", omit(apiReverseResult, "side")],
    ["stat が文字列でない", { ...apiReverseResult, stat: 2 }],
    ["assumedHpSp が無い", omit(apiReverseResult, "assumedHpSp")],
    ["exactCount が数値でない", { ...apiReverseResult, exactCount: "1" }],
    ["候補が空オブジェクト", { ...apiReverseResult, candidates: [{}] }],
    ["候補が null", { ...apiReverseResult, candidates: [null] }],
    ["候補の nature が無い", { ...apiReverseResult, candidates: [omit(reverseCandidate, "nature")] }],
    ["候補の ranges が無い", { ...apiReverseResult, candidates: [omit(reverseCandidate, "ranges")] }],
    [
      "候補の ranges が配列でない(文字列)",
      { ...apiReverseResult, candidates: [{ ...reverseCandidate, ranges: "0-3" }] },
    ],
    [
      "候補の ranges の要素に max が無い",
      { ...apiReverseResult, candidates: [{ ...reverseCandidate, ranges: [{ min: 10 }] }] },
    ],
    [
      "候補の ranges の要素の min が数値でない",
      { ...apiReverseResult, candidates: [{ ...reverseCandidate, ranges: [{ min: "10", max: 12 }] }] },
    ],
    [
      "候補の exact が真偽値でない",
      { ...apiReverseResult, candidates: [{ ...reverseCandidate, exact: "true" }] },
    ],
    ["候補の support が null", { ...apiReverseResult, candidates: [{ ...reverseCandidate, support: null }] }],
    ["候補の minPercent が無い", { ...apiReverseResult, candidates: [omit(reverseCandidate, "minPercent")] }],
  ];

  test.each(invalidReverseBodies)("calcReverse: %s なら engine_unavailable", async (_label, body) => {
    await expectUnavailableFor(reverseCall, body);
  });

  test("契約外の 200 でも Promise は reject しない(画面は .then しか登録していない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({})));
    const engine = engineWith(fetchMock);
    await expect(engine.calc(calcRequest)).resolves.toMatchObject({ ok: false });
    await expect(engine.calcBulk(bulkRequest)).resolves.toMatchObject({ ok: false });
    await expect(engine.calcReverse(reverseRequest)).resolves.toMatchObject({ ok: false });
  });

  test("本文が読めたうえでの契約違反は、abort 済みでも engine_unavailable(通信は成立している)", async () => {
    const controller = new AbortController();
    const fetchMock = fakeFetch(() => {
      controller.abort();
      return Promise.resolve(jsonResponse({}));
    });
    const result = await engineWith(fetchMock).calc(calcRequest, controller.signal);
    expect(result).toMatchObject({ ok: false, error: { code: "engine_unavailable" } });
  });
});

describe("契約どおりの 2xx は従来どおり成功(検証で落とさない。issue 67)", () => {
  test("余分なフィールドがあっても成功し、DTO には混ぜない(前方互換)", async () => {
    const fetchMock = fakeFetch(() =>
      Promise.resolve(
        jsonResponse({ ...apiCalcResult, futureField: "x", ko: { ...apiCalcResult.ko, futureKo: 1 } }),
      ),
    );
    const result = await engineWith(fetchMock).calc(calcRequest);
    const expected: CalcResult = omit(
      { ...apiCalcResult, ko: { ...apiCalcResult.ko, chancePercent: 12.34 } },
      "unsupported",
    );
    expect(result).toEqual({ ok: true, value: expected });
  });

  test("列挙の値そのものは検査しない(契約に無い category でも成功)", async () => {
    const fetchMock = fakeFetch(() =>
      Promise.resolve(jsonResponse({ ...apiCalcResult, category: "future" })),
    );
    const result = await engineWith(fetchMock).calc(calcRequest);
    expect(result).toMatchObject({ ok: true, value: { category: "future" } });
  });

  test("rows が空配列でも成功(件数は検査しない)", async () => {
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({ ...apiBulkResult, rows: [] })));
    const result = await engineWith(fetchMock).calcBulk(bulkRequest);
    expect(result).toEqual({ ok: true, value: { defenderSpeciesKey: "9002-001", rows: [] } });
  });

  test("candidates が空配列でも成功", async () => {
    const fetchMock = fakeFetch(() =>
      Promise.resolve(jsonResponse({ ...apiReverseResult, candidates: [], exactCount: 0 })),
    );
    const result = await engineWith(fetchMock).calcReverse(reverseRequest);
    expect(result).toMatchObject({ ok: true, value: { candidates: [] } });
  });

  test("行の itemId・defender.natureId が省かれていても成功(既定値を補う / 捨てるフィールド)", async () => {
    const row = { ...omit(bulkRow, "itemId"), defender: omit(bulkRow.defender, "natureId") };
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({ ...apiBulkResult, rows: [row] })));
    const result = await engineWith(fetchMock).calcBulk(bulkRequest);
    expect(result).toMatchObject({ ok: true, value: { rows: [{ itemId: "" }] } });
  });

  test('defender.nature の plus・minus が省かれていても無補正("")として成功', async () => {
    const row = { ...bulkRow, defender: { ...bulkRow.defender, nature: {} } };
    const fetchMock = fakeFetch(() => Promise.resolve(jsonResponse({ ...apiBulkResult, rows: [row] })));
    const result = await engineWith(fetchMock).calcBulk(bulkRequest);
    expect(result).toMatchObject({
      ok: true,
      value: { rows: [{ defender: { nature: { plus: "", minus: "" } } }] },
    });
  });

  test("候補の natureId・itemId が省かれていても成功", async () => {
    const candidate = omit(omit(reverseCandidate, "natureId"), "itemId");
    const fetchMock = fakeFetch(() =>
      Promise.resolve(jsonResponse({ ...apiReverseResult, candidates: [candidate] })),
    );
    const result = await engineWith(fetchMock).calcReverse(reverseRequest);
    expect(result).toMatchObject({ ok: true, value: { candidates: [{ itemId: "" }] } });
  });
});
