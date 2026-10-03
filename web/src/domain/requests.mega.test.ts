// issue 505・ADR-0321(ADR-0320 の見直し): メガ種族だけ isMega・requiredItemId を engine(境界)へ渡す。
// 境界(engine/wasmapi)がオンライン(calc-svc)と同じ持ち物の検証をするため。learnset は引き続き落とす。
// メガでない種族は従来どおりキーを足さない(リクエストは不変)。
// メガ種族の持ち物は呼び出し側が固定したもの(requiredItemId の持ち物)を buildIndividual の item に渡す。

import { expect, test } from "vitest";
import { MEGA_FIRE, MEGA_FIRE_STONE } from "../test/megaMaster";
import { NEUTRAL_NATURE, ZERO_SP, buildIndividual, toEngineSpecies } from "./requests";

test("toEngineSpecies はメガ種族に isMega・requiredItemId を残し、learnset は落とす", () => {
  const species = toEngineSpecies(MEGA_FIRE);
  expect(Object.keys(species).sort()).toEqual(
    ["abilities", "baseStats", "dexNo", "form", "isMega", "key", "nameJa", "requiredItemId", "types"].sort(),
  );
  expect(species.isMega).toBe(true);
  expect(species.requiredItemId).toBe(MEGA_FIRE_STONE.id);
});

test("toEngineSpecies はメガでない種族にメガ用のキーを足さない", () => {
  const species = toEngineSpecies({ ...MEGA_FIRE, isMega: false, requiredItemId: null });
  expect(Object.keys(species).sort()).toEqual(
    ["abilities", "baseStats", "dexNo", "form", "key", "nameJa", "types"].sort(),
  );
  const omitted = toEngineSpecies({ ...MEGA_FIRE, isMega: undefined, requiredItemId: undefined });
  expect(omitted).not.toHaveProperty("isMega");
  expect(omitted).not.toHaveProperty("requiredItemId");
});

test("requiredItemId が無いメガ種族は null を渡す(どの持ち物も持てない扱い)", () => {
  expect(toEngineSpecies({ ...MEGA_FIRE, requiredItemId: undefined }).requiredItemId).toBeNull();
});

test("buildIndividual は渡されたメガストーンを持ち物にし、種族に isMega・requiredItemId を載せる", () => {
  const individual = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: MEGA_FIRE_STONE,
    ability: { id: "exampleabilitynone", nameJa: "テストなし", effect: null },
  });
  expect(individual.item).toBe(MEGA_FIRE_STONE);
  expect(individual.species.isMega).toBe(true);
  expect(individual.species.requiredItemId).toBe(MEGA_FIRE_STONE.id);
});
