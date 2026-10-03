// issue #515・ADR-0320: メガ種族の画面用フィールド(isMega・requiredItemId)は engine に渡さない。
// 境界(engine/wasmapi)は未知のフィールドを unknown_field で拒否するので、learnset と同じく toEngineSpecies が落とす。
// メガ種族の持ち物は呼び出し側が固定したもの(requiredItemId の持ち物)を buildIndividual の item に渡す。

import { expect, test } from "vitest";
import { MEGA_FIRE, MEGA_FIRE_STONE } from "../test/megaMaster";
import { NEUTRAL_NATURE, ZERO_SP, buildIndividual, toEngineSpecies } from "./requests";

test("toEngineSpecies は isMega・requiredItemId・learnset を落とす", () => {
  const species = toEngineSpecies(MEGA_FIRE);
  expect(Object.keys(species).sort()).toEqual(
    ["abilities", "baseStats", "dexNo", "form", "key", "nameJa", "types"].sort(),
  );
});

test("buildIndividual は渡されたメガストーンを持ち物にする(種族は engine の形)", () => {
  const individual = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: MEGA_FIRE_STONE,
    ability: { id: "exampleabilitynone", nameJa: "テストなし", effect: null },
  });
  expect(individual.item).toBe(MEGA_FIRE_STONE);
  expect(individual.species).not.toHaveProperty("isMega");
});
