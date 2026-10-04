// I-web-4(ADR-0328 / ADR-0326 §4 の更新): メガストーンの表示名は、マスタの nameJa が日本語として使えるときはそのまま、
// 使えないとき(日本語の文字=ひらがな・カタカナ・漢字を1文字も含まない)だけ従来のフォールバックにする。
// 全角英数字だけ(例「Ｘ」)・英語名・空は「日本語として使えない」。

import { expect, test } from "vitest";
import type { MasterItem, MasterSpecies } from "../master/types";
import { itemsWithStoneLabels, megaStoneDisplayName } from "./itemRoles";

test.each([
  ["リザードナイトＸ", "リザードン", "リザードナイトＸ"], // 全角Ｘを含んでもカタカナがあれば日本語
  ["ルカリオナイト", "ルカリオ", "ルカリオナイト"],
  ["かえんのいし", null, "かえんのいし"], // ひらがな
  ["炎石", undefined, "炎石"], // 漢字
  ["Barbaracite", "ガメノデス", "ガメノデス専用のメガストーン"], // 英語名はフォールバック
  ["ＸＹ", "リザードン", "リザードン専用のメガストーン"], // 全角英数字だけは日本語としない
  ["", "リザードン", "リザードン専用のメガストーン"],
  ["  ", "リザードン", "リザードン専用のメガストーン"],
  [null, "リザードン", "リザードン専用のメガストーン"],
  [undefined, "リザードン", "リザードン専用のメガストーン"],
  ["Barbaracite", null, "メガストーン"], // 基本種名なし
  ["Barbaracite", undefined, "メガストーン"],
  ["Barbaracite", "  ", "メガストーン"],
  ["", null, "メガストーン"],
])("megaStoneDisplayName(%j, %j) = %j", (stoneNameJa, baseSpeciesNameJa, expected) => {
  expect(megaStoneDisplayName(stoneNameJa, baseSpeciesNameJa)).toBe(expected);
});

const speciesBase: MasterSpecies = {
  key: "9101-001",
  dexNo: 9101,
  form: 1,
  nameJa: "メガテスト",
  types: ["fire"],
  baseStats: { hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50 },
  abilities: [],
  learnset: [],
  isMega: true,
  requiredItemId: "examplestonea",
  baseSpeciesNameJa: "テスト",
};
const stoneA: MasterItem = { id: "examplestonea", nameJa: "テストナイトＸ", effect: null, isMegaStone: true };
const stoneB: MasterItem = { id: "examplestoneb", nameJa: "Barbaracite", effect: null, isMegaStone: true };
const speciesB: MasterSpecies = {
  ...speciesBase,
  key: "9102-001",
  requiredItemId: "examplestoneb",
  baseSpeciesNameJa: "テストB",
};

test("itemsWithStoneLabels(結果の行・調整・逆算の表示用): 日本語名はそのまま、英語名は「{基本種名}のメガストーン」", () => {
  const out = itemsWithStoneLabels([stoneA, stoneB], [speciesBase, speciesB]);
  expect(out.map((item) => item.nameJa)).toEqual(["テストナイトＸ", "テストB専用のメガストーン"]);
});

test("itemsWithStoneLabels: 引ける種族が無い英語名のストーンは「メガストーン」、日本語名はそのまま", () => {
  const out = itemsWithStoneLabels([stoneA, stoneB], []);
  expect(out.map((item) => item.nameJa)).toEqual(["テストナイトＸ", "メガストーン"]);
});
