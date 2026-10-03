// ADR-0326: マスタの持ち物(MasterItem)は roles・isMegaStone を持つが、engine(WASM 境界)と calc-svc は
// 未知のフィールドを拒否する(engine/wasmapi の DisallowUnknownFields)。要求を組み立てる関数が toEngineItem で
// 形を戻すこと(画面ごとに落とし忘れないよう、組み立ての1か所で落とす)。
// 確かめること: buildIndividual の item・buildBulkRequest の itemVariants・buildReverseRequest の itemCandidates に
// roles・isMegaStone が載らない。null(持ち物なし)は null のまま。

import { expect, test } from "vitest";
import type { Ability, Move, TypeChart } from "../engine/types";
import { BOTH_ITEM, DEF_ITEM, POWER_ITEM } from "../test/itemRolesMaster";
import { MEGA_FIRE } from "../test/megaMaster";
import { NEUTRAL_NATURE, ZERO_SP, buildBulkRequest, buildIndividual, buildReverseRequest } from "./requests";

const ABILITY: Ability = { id: "exampleabilitynone", nameJa: "テストなし", effect: null };
const MOVE: Move = {
  id: "examplemovetackle",
  nameJa: "テストたいあたり",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};
const TYPE_CHART: TypeChart = { types: ["normal"], effectiveness: {} };
const ENGINE_ITEM_KEYS = ["effect", "id", "nameJa"];

function keysOf(value: object | null | undefined): string[] {
  if (value === null || value === undefined) {
    throw new Error("持ち物が載っていない");
  }
  return Object.keys(value).sort();
}

test("buildIndividual の item に roles・isMegaStone が載らない", () => {
  const individual = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: POWER_ITEM,
    ability: ABILITY,
  });
  expect(keysOf(individual.item)).toEqual(ENGINE_ITEM_KEYS);
  expect(individual.item).toEqual({
    id: POWER_ITEM.id,
    nameJa: POWER_ITEM.nameJa,
    effect: POWER_ITEM.effect,
  });
});

test("buildIndividual の item が null(持ち物なし)なら null のまま", () => {
  const individual = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: ABILITY,
  });
  expect(individual.item).toBeNull();
});

test("buildBulkRequest の itemVariants の各要素に roles・isMegaStone が載らない(null は保つ)", () => {
  const attacker = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: ABILITY,
  });
  const request = buildBulkRequest({
    attacker,
    defenderSpecies: MEGA_FIRE,
    move: MOVE,
    typeChart: TYPE_CHART,
    itemVariants: [null, DEF_ITEM, BOTH_ITEM],
  });
  expect(request.itemVariants?.[0]).toBeNull();
  expect(keysOf(request.itemVariants?.[1])).toEqual(ENGINE_ITEM_KEYS);
  expect(keysOf(request.itemVariants?.[2])).toEqual(ENGINE_ITEM_KEYS);
});

test("buildReverseRequest の itemCandidates・known.item に roles・isMegaStone が載らない", () => {
  const known = buildIndividual(MEGA_FIRE, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: BOTH_ITEM,
    ability: ABILITY,
  });
  const request = buildReverseRequest({
    side: "defender",
    known,
    unknownSpecies: MEGA_FIRE,
    move: MOVE,
    typeChart: TYPE_CHART,
    itemCandidates: [null, DEF_ITEM],
    observations: [],
  });
  expect(keysOf(request.known.item)).toEqual(ENGINE_ITEM_KEYS);
  expect(request.itemCandidates?.[0]).toBeNull();
  expect(keysOf(request.itemCandidates?.[1])).toEqual(ENGINE_ITEM_KEYS);
});
