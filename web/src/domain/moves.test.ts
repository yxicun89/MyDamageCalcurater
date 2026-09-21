// P4-2: 技セレクタの候補(種族の learnset → 技)と、既定で選ぶ技(最初のダメージ技)。
// 技の一覧はマスタのデータが正で、画面は learnset の順をそのまま使う(並べ替えない)。

import { expect, test } from "vitest";
import type { Move } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { firstDamagingMove, isDamagingMove, learnsetMoves } from "./moves";

const base: Move = { id: "", nameJa: "", type: "normal", category: "physical", power: 50, priority: 0 };
const status: Move = { ...base, id: "example-status", nameJa: "テスト変化", category: "status", power: 0 };
const special: Move = { ...base, id: "example-special", nameJa: "テスト特殊", category: "special" };
const physical: Move = { ...base, id: "example-physical", nameJa: "テスト物理" };
const moves = [physical, special, status];

function speciesWith(learnset: string[]): MasterSpecies {
  return {
    key: "example-species",
    dexNo: 9001,
    form: 0,
    nameJa: "テスト種族",
    types: ["normal"],
    baseStats: { hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50 },
    abilities: [],
    learnset,
  };
}

test("learnsetMoves は learnset の順に技を解決する(マスタの順ではない)", () => {
  expect(
    learnsetMoves(speciesWith(["example-status", "example-special", "example-physical"]), moves),
  ).toEqual([status, special, physical]);
});

test("learnsetMoves は一覧に無い ID を飛ばす", () => {
  expect(learnsetMoves(speciesWith(["example-missing", "example-physical"]), moves)).toEqual([physical]);
});

test("isDamagingMove は変化技以外", () => {
  expect([physical, special, status].map(isDamagingMove)).toEqual([true, true, false]);
});

test("firstDamagingMove は learnset の順で最初のダメージ技、無ければ undefined", () => {
  expect(
    firstDamagingMove(speciesWith(["example-status", "example-special", "example-physical"]), moves),
  ).toEqual(special);
  expect(firstDamagingMove(speciesWith(["example-status"]), moves)).toBeUndefined();
});
