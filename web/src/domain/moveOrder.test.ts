// G-01(ADR-0341): 技の選択肢の並びは「タイプ順」だけ(ADR-0335 の習得順・五十音順の切り替えを廃止)。
// orderMoves(moves, types): types(マスタのタイプ表 master.typeChart.types)の並びで群にし、群の中は五十音順
// (Intl.Collator("ja")・同順位は技 ID の昇順)。タイプ表に無いタイプは最後にタイプ ID の昇順。入力は書き換えない。
// 旧 moveSort.ts の type モード(sortMoves(…, "type", types))と同じ結果。

import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import { SORT_MOVES, TYPE_IDS } from "../test/moveSortMaster";
import { orderMoves } from "./moveOrder";

const TYPES = ["electric", "fire", "normal"];

const mv = (id: string, nameJa: string, type: string): Move => ({
  id,
  nameJa,
  type,
  category: "physical",
  power: 50,
  priority: 0,
});

describe("orderMoves", () => {
  test("タイプ表の並びで群にし、群の中は五十音順(濁点・カタカナ・長音を含む)", () => {
    expect(orderMoves(SORT_MOVES, TYPES).map((m) => m.id)).toEqual(TYPE_IDS);
  });

  test("入力の順に依らず同じ結果で、入力の配列は書き換えない", () => {
    const input = [...SORT_MOVES].reverse();
    const before = input.map((m) => m.id);
    expect(orderMoves(input, TYPES).map((m) => m.id)).toEqual(TYPE_IDS);
    expect(input.map((m) => m.id)).toEqual(before);
  });

  test("タイプ表に無いタイプは最後に、タイプ ID の昇順で並ぶ(壊れない)", () => {
    const moves = [mv("z1", "あ", "zzz"), mv("a1", "い", "fire"), mv("b1", "う", "aaa")];
    expect(orderMoves(moves, TYPES).map((m) => m.id)).toEqual(["a1", "b1", "z1"]);
  });

  test("名前が同順位(ひらがな/カタカナだけの違い)なら技 ID の昇順", () => {
    const moves = [mv("m2", "ひかり", "fire"), mv("m1", "ヒカリ", "fire")];
    expect(orderMoves(moves, TYPES).map((m) => m.id)).toEqual(["m1", "m2"]);
  });

  test("空は空", () => {
    expect(orderMoves([], TYPES)).toEqual([]);
  });
});
