// issue #274 残り(ADR-0315 案): API 実装の防御側ランクの写像。
//   - BulkRequest.defenderOverride.ranks は BulkCalcRequest.defenderOverride.ranks にそのまま写す
//   - #272 の abilityId(defenderAbilities がちょうど1件のとき)と同じ defenderOverride オブジェクトに併存する
//   - ranks が無く、特性もおまかせ/未指定なら defenderOverride 自体を送らない(従来と同じ本文)
// 期待する本文は openapi-typescript の生成型で書く。

import { describe, expect, test, vi } from "vitest";
import type { BulkRequest, Individual, Move, Species, TypeChart } from "../engine/types";
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
  abilities: ["exampleabilityone", "exampleabilitytwo"],
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
const ranks = { atk: 0, def: 1, spa: 0, spd: -2, spe: 0 };

function engineWith() {
  const body: Schemas["BulkCalcResult"] = { defenderSpeciesKey: species.key, rows: [] };
  const fetchMock = vi.fn<typeof fetch>(() =>
    Promise.resolve(new Response(JSON.stringify(body), { status: 200 })),
  );
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

describe("calcBulk: 防御側のランクの送り方", () => {
  test("defenderOverride.ranks をそのまま defenderOverride.ranks に写す(abilityId は付けない)", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calcBulk({ ...bulkRequest, defenderOverride: { ranks } });
    const expected: Schemas["DefenderOverride"] = { ranks };
    expect(sentBody(fetchMock).defenderOverride).toEqual(expected);
  });

  test("特性が1件(個別選択)と併存: 同じ defenderOverride に abilityId と ranks が入る", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calcBulk({ ...bulkRequest, defenderAbilities: [abilityTwo], defenderOverride: { ranks } });
    const expected: Schemas["DefenderOverride"] = { abilityId: "exampleabilitytwo", ranks };
    expect(sentBody(fetchMock).defenderOverride).toEqual(expected);
    expect(sentBody(fetchMock)).not.toHaveProperty("defenderAbilities");
  });

  test("おまかせ(2件)+ ranks: abilityId は付けず ranks だけ送る", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calcBulk({
      ...bulkRequest,
      defenderAbilities: [abilityOne, abilityTwo],
      defenderOverride: { ranks },
    });
    expect(sentBody(fetchMock).defenderOverride).toEqual({ ranks });
  });

  test("ranks も特性指定も無ければ defenderOverride を送らない(従来と同じ本文)", async () => {
    const { engine, fetchMock } = engineWith();
    await engine.calcBulk(bulkRequest);
    expect(sentBody(fetchMock)).not.toHaveProperty("defenderOverride");
  });
});
