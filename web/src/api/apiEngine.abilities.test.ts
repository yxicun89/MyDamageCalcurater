// issue 272(ADR-0126・ADR-0214・ADR-0311): API 実装の特性の写像。
//   - 個別選択(防御側/相手の特性が1件)のときだけ defenderOverride.abilityId / unknownAbilityId を送る
//   - おまかせ(2件以上)・特性なし(未指定/空)は省略する(calc-svc が種族の全特性を解決する。ADR-0214)
//   - 応答の abilityId / abilityIds は行・候補に写す
// 期待する本文は openapi-typescript の生成型で書き、契約とのずれを typecheck で検出する。

import { describe, expect, test, vi } from "vitest";
import type { BulkRequest, Individual, Move, ReverseRequest, Species, TypeChart } from "../engine/types";
import type { MasterNature } from "../master/types";
import { createApiEngine } from "./apiEngine";
import type { components } from "./openapi.gen";

type Schemas = components["schemas"];

const ids = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};
const natures: readonly MasterNature[] = [
  { id: "examplenatureneutral", nameJa: "テストむほせい", plus: null, minus: null },
];
const typeChart: TypeChart = { types: ["normal", "water"], effectiveness: {} };
const zeroStats = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

const species: Species = {
  key: "9002-000",
  dexNo: 9002,
  form: 0,
  nameJa: "テストみず",
  types: ["water"],
  baseStats: { hp: 90, atk: 75, def: 80, spa: 95, spd: 85, spe: 70 },
  abilities: ["exampleabilityone", "exampleabilitytwo", "exampleabilitythree"],
};
const abilityOne = { id: "exampleabilityone", nameJa: "テスト一", effect: null };
const abilityTwo = { id: "exampleabilitytwo", nameJa: "テスト二", effect: null };
const move: Move = {
  id: "examplemovetackle",
  nameJa: "テストたいあたり",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};
const attacker: Individual = {
  species,
  level: 50,
  nature: { plus: "", minus: "" },
  ability: abilityOne,
  item: null,
  sp: zeroStats,
};

const bulkRequest: BulkRequest = { format: "single", attacker, defenderSpecies: species, move, typeChart };
const reverseRequest: ReverseRequest = {
  format: "single",
  side: "defender",
  known: attacker,
  unknownSpecies: species,
  move,
  observations: [{ percent: 45 }],
  typeChart,
};

const apiCalcResult: Schemas["CalcResult"] = {
  rolls: Array.from({ length: 16 }, (_, index) => 60 + index),
  minDamage: 60,
  maxDamage: 75,
  minPercent: 34.2,
  maxPercent: 42.9,
  defenderHP: 175,
  effectiveness: 1,
  stab: false,
  category: "physical",
  ko: { hits: 3, guaranteed: false, chancePercent: 12.3, displayChancePercent: 12.3 },
  unsupported: [],
};

function apiBulkRow(abilityId: string, abilityIds: string[]): Schemas["BulkCalcRow"] {
  return {
    preset: "none",
    presetLabel: "無振り",
    itemId: null,
    defender: {
      sp: zeroStats,
      nature: { plus: null, minus: null },
      natureId: "examplenatureneutral",
      stats: { hp: 165, atk: 95, def: 100, spa: 115, spd: 105, spe: 90 },
    },
    result: apiCalcResult,
    abilityId,
    abilityIds,
  };
}

function apiCandidate(abilityId: string, abilityIds: string[]): Schemas["ReverseCandidate"] {
  return {
    natureClass: "neutral",
    nature: { plus: null, minus: null },
    natureId: "examplenatureneutral",
    itemId: null,
    ranges: [{ min: 10, max: 12 }],
    spCount: 3,
    exact: true,
    mismatch: 0,
    support: 4,
    minPercent: 40.2,
    maxPercent: 47.8,
    unsupported: [],
    abilityId,
    abilityIds,
  };
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function engineWith(body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(body)));
  const engine = createApiEngine({ baseUrl: "http://api.test/", fetch: fetchMock, master: { natures }, ids });
  return { engine, fetchMock };
}

function sentBody(fetchMock: ReturnType<typeof engineWith>["fetchMock"]): Record<string, unknown> {
  const text = fetchMock.mock.calls[0]?.[1]?.body;
  if (typeof text !== "string") {
    throw new Error("本文は JSON 文字列で送る");
  }
  return JSON.parse(text) as Record<string, unknown>;
}

const bulkBody: Schemas["BulkCalcResult"] = {
  defenderSpeciesKey: species.key,
  rows: [apiBulkRow("exampleabilityone", ["exampleabilityone"])],
};
const reverseBody: Schemas["ReverseResult"] = {
  side: "defender",
  stat: "def",
  assumedHpSp: 32,
  exactCount: 1,
  candidates: [apiCandidate("exampleabilityone", ["exampleabilityone"])],
};

describe("calcBulk: 防御側の特性の送り方", () => {
  test("個別選択(defenderAbilities が1件)は defenderOverride.abilityId で送る", async () => {
    const { engine, fetchMock } = engineWith(bulkBody);
    await engine.calcBulk({ ...bulkRequest, defenderAbilities: [abilityTwo] });
    const expected: Schemas["DefenderOverride"] = { abilityId: "exampleabilitytwo" };
    expect(sentBody(fetchMock).defenderOverride).toEqual(expected);
    expect(sentBody(fetchMock)).not.toHaveProperty("defenderAbilities");
  });

  test("おまかせ(2件以上)は defenderOverride を省略する(calc-svc が種族の全特性を解決する)", async () => {
    const { engine, fetchMock } = engineWith(bulkBody);
    await engine.calcBulk({ ...bulkRequest, defenderAbilities: [abilityOne, abilityTwo] });
    expect(sentBody(fetchMock)).not.toHaveProperty("defenderOverride");
    expect(sentBody(fetchMock)).not.toHaveProperty("defenderAbilities");
  });

  test("未指定・空配列も defenderOverride を省略する(従来と同じ本文)", async () => {
    for (const request of [bulkRequest, { ...bulkRequest, defenderAbilities: [] }]) {
      const { engine, fetchMock } = engineWith(bulkBody);
      await engine.calcBulk(request);
      expect(sentBody(fetchMock)).not.toHaveProperty("defenderOverride");
    }
  });

  test("応答の行の abilityId / abilityIds を BulkRow に写す(結果が分かれた行は別行のまま)", async () => {
    const { engine } = engineWith({
      defenderSpeciesKey: species.key,
      rows: [
        apiBulkRow("exampleabilityone", ["exampleabilityone", "exampleabilitytwo"]),
        apiBulkRow("exampleabilitythree", ["exampleabilitythree"]),
      ],
    } satisfies Schemas["BulkCalcResult"]);
    const result = await engine.calcBulk(bulkRequest);
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    expect(result.value.rows.map((row) => [row.abilityId, row.abilityIds])).toEqual([
      ["exampleabilityone", ["exampleabilityone", "exampleabilitytwo"]],
      ["exampleabilitythree", ["exampleabilitythree"]],
    ]);
  });
});

describe("calcReverse: 相手の特性の送り方", () => {
  test("個別選択(unknownAbilities が1件)は unknownAbilityId で送る", async () => {
    const { engine, fetchMock } = engineWith(reverseBody);
    await engine.calcReverse({ ...reverseRequest, unknownAbilities: [abilityTwo] });
    const body = sentBody(fetchMock);
    const expected: Schemas["ReverseRequest"]["unknownAbilityId"] = "exampleabilitytwo";
    expect(body.unknownAbilityId).toBe(expected);
    expect(body).not.toHaveProperty("unknownAbilities");
  });

  test("おまかせ(2件以上)・未指定・空配列は unknownAbilityId を省略する", async () => {
    for (const unknownAbilities of [[abilityOne, abilityTwo], [], undefined]) {
      const { engine, fetchMock } = engineWith(reverseBody);
      await engine.calcReverse(
        unknownAbilities === undefined ? reverseRequest : { ...reverseRequest, unknownAbilities },
      );
      expect(sentBody(fetchMock)).not.toHaveProperty("unknownAbilityId");
      expect(sentBody(fetchMock)).not.toHaveProperty("unknownAbilities");
    }
  });

  test("応答の候補の abilityId / abilityIds を ReverseCandidate に写す", async () => {
    const { engine } = engineWith({
      ...reverseBody,
      candidates: [
        apiCandidate("exampleabilityone", ["exampleabilityone", "exampleabilitytwo"]),
        apiCandidate("exampleabilitythree", ["exampleabilitythree"]),
      ],
    } satisfies Schemas["ReverseResult"]);
    const result = await engine.calcReverse(reverseRequest);
    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }
    expect(result.value.candidates.map((c) => [c.abilityId, c.abilityIds])).toEqual([
      ["exampleabilityone", ["exampleabilityone", "exampleabilitytwo"]],
      ["exampleabilitythree", ["exampleabilitythree"]],
    ]);
  });
});

describe("前方互換", () => {
  test("応答に abilityId / abilityIds が無い(古いサーバー)行・候補でも成功し、フィールドは出さない", async () => {
    const row = apiBulkRow("x", ["x"]) as Partial<Schemas["BulkCalcRow"]>;
    delete row.abilityId;
    delete row.abilityIds;
    const { engine } = engineWith({ defenderSpeciesKey: species.key, rows: [row] });
    const result = await engine.calcBulk(bulkRequest);
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.rows[0]).not.toHaveProperty("abilityIds");
    }
  });
});
