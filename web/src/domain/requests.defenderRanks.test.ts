// issue #274 残り(ADR-0315 案): buildBulkRequest が defenderOverride(ranks)を通す。
// 渡さなければキーを作らない(従来とバイト単位で同じ)。BulkRequest.defenderOverride は WASM の境界 JSON の形
// ({ranks})そのまま。API 実装(apiEngine)はこれに defenderAbilities 由来の abilityId を足して送る。

import { describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import { NEUTRAL_NATURE, ZERO_SP, buildBulkRequest, buildIndividual } from "./requests";

const master = await exampleMasterSource.load();
const [attackerSpecies, defenderSpecies] = master.species;
const tackle = master.moves.find((move) => move.id === "examplemovetackle");
if (attackerSpecies === undefined || defenderSpecies === undefined || tackle === undefined) {
  throw new Error("例データが足りない");
}
const attacker = buildIndividual(attackerSpecies, {
  sp: ZERO_SP,
  nature: NEUTRAL_NATURE,
  item: null,
  ability: { id: "", nameJa: "", effect: null },
});
const common = { attacker, defenderSpecies, move: tackle, typeChart: master.typeChart };

describe("buildBulkRequest: defenderOverride", () => {
  test("渡さなければキーが無い(従来の形)", () => {
    expect(buildBulkRequest(common)).not.toHaveProperty("defenderOverride");
  });

  test("渡すとそのまま要求に入る", () => {
    const defenderOverride = { ranks: { atk: 0, def: 1, spa: 0, spd: 0, spe: 0 } };
    expect(buildBulkRequest({ ...common, defenderOverride }).defenderOverride).toEqual(defenderOverride);
  });

  test("defenderAbilities と併存できる(同じ要求に両方入る)", () => {
    const ability = { id: "exampleabilityone", nameJa: "テスト", effect: null };
    const defenderOverride = { ranks: { atk: 0, def: 0, spa: 0, spd: -2, spe: 0 } };
    const request = buildBulkRequest({ ...common, defenderAbilities: [ability], defenderOverride });
    expect(request.defenderAbilities).toEqual([ability]);
    expect(request.defenderOverride).toEqual(defenderOverride);
  });
});
