// I-web-2(ADR-0328): 技の選択肢の絞り込み(ダメージを与える技=物理・特殊だけ)を1か所の純粋関数に寄せる。
// 技名・ID はハードコードせず、Move.category(マスタの分類)だけから導く。

import { expect, test } from "vitest";
import type { Move } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { damagingLearnsetMoves, firstDamagingMove } from "./moves";

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

test("damagingLearnsetMoves は変化技を除き、learnset の順のまま返す", () => {
  const species = speciesWith(["example-status", "example-special", "example-physical"]);
  expect(damagingLearnsetMoves(species, moves)).toEqual([special, physical]);
});

test("damagingLearnsetMoves は一覧に無い ID を飛ばす", () => {
  expect(damagingLearnsetMoves(speciesWith(["example-missing", "example-physical"]), moves)).toEqual([
    physical,
  ]);
});

test("変化技しか覚えない種族は空配列", () => {
  expect(damagingLearnsetMoves(speciesWith(["example-status"]), moves)).toEqual([]);
});

test("firstDamagingMove は damagingLearnsetMoves の先頭と同じ(既定の技は従来どおり最初のダメージ技)", () => {
  const species = speciesWith(["example-status", "example-special", "example-physical"]);
  expect(firstDamagingMove(species, moves)).toEqual(damagingLearnsetMoves(species, moves)[0]);
  expect(firstDamagingMove(speciesWith(["example-status"]), moves)).toBeUndefined();
});
