// issue #274(ADR-0312): requests.ts の組み立てが計算条件(攻撃側の status / ranks、要求の critical / field)を
// 通すこと。既定(何も渡さない)の要求は従来と**バイト単位で同じ**(キーを増やさない)。

import { describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildCalcRequest,
  buildIndividual,
  toEngineSpecies,
} from "./requests";

const master = await exampleMasterSource.load();
const attackerSpecies = master.species[0];
const defenderSpecies = master.species[1];
const tackle = master.moves.find((move) => move.id === "examplemovetackle");
if (attackerSpecies === undefined || defenderSpecies === undefined || tackle === undefined) {
  throw new Error("例データが足りない");
}
const base = {
  sp: ZERO_SP,
  nature: NEUTRAL_NATURE,
  item: null,
  ability: { id: "", nameJa: "", effect: null },
};

describe("buildIndividual", () => {
  test("status・ranks を渡さなければキーが増えない(従来どおり)", () => {
    const individual = buildIndividual(attackerSpecies, base);
    expect(Object.keys(individual).sort()).toEqual(
      ["ability", "item", "level", "nature", "species", "sp"].sort(),
    );
  });

  test("status:'burn' と ranks を渡すと Individual に入る", () => {
    const ranks = { atk: 1, def: 0, spa: -2, spd: 0, spe: 0 };
    const individual = buildIndividual(attackerSpecies, { ...base, status: "burn", ranks });
    expect(individual.status).toBe("burn");
    expect(individual.ranks).toEqual(ranks);
  });
});

describe("buildBulkRequest", () => {
  const attacker = buildIndividual(attackerSpecies, base);
  const common = { attacker, defenderSpecies, move: tackle, typeChart: master.typeChart };

  test("critical・field を渡さなければ従来の形のまま(キーが無い)", () => {
    const request = buildBulkRequest(common);
    expect(Object.keys(request).sort()).toEqual(
      ["attacker", "defenderSpecies", "format", "move", "typeChart"].sort(),
    );
    expect(request.defenderSpecies).toEqual(toEngineSpecies(defenderSpecies));
  });

  test("critical・field を渡すと要求に入る", () => {
    const field = {
      weather: "sun",
      defenderScreens: { reflect: true, lightScreen: false, auroraVeil: false },
    };
    const request = buildBulkRequest({ ...common, critical: true, field });
    expect(request.critical).toBe(true);
    expect(request.field).toEqual(field);
  });

  test("critical:false は渡されても送らない(従来と同じ形を守る)", () => {
    expect(buildBulkRequest({ ...common, critical: false })).not.toHaveProperty("critical");
  });
});

describe("buildCalcRequest は変わらない(1対1。この画面は使わない)", () => {
  test("field・critical を持たない", () => {
    const attacker = buildIndividual(attackerSpecies, base);
    const defender = buildIndividual(defenderSpecies, base);
    const request = buildCalcRequest({ attacker, defender, move: tackle, typeChart: master.typeChart });
    expect(request).not.toHaveProperty("field");
    expect(request).not.toHaveProperty("critical");
  });
});
