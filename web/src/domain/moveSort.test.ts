// I-web-9 = F-02(ADR-0335 §1〜§3): 技の並び替えの純粋関数 sortMoves / moveTypeGroups。
//   - 習得順は入力順のまま(既定。入力を書き換えない)
//   - 五十音順: Intl.Collator("ja") で nameJa を比べる。ひらがな/カタカナ・濁点は同じ字として比べ(濁点だけの違いは後)、
//     同順位は技 ID の昇順で決める(入力の順に依らず決定的)
//   - タイプ順: マスタの相性表(types)の並びで群にし、群の中は五十音順。相性表に無いタイプは最後(タイプ ID の昇順)
//   - 空・1件・未知のタイプでも壊れない
// 技の名前・タイプはこのテストの中の架空のデータ(ADR-0002)。

import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import {
  DEFAULT_MOVE_SORT_ORDER,
  MOVE_SORT_ORDERS,
  isMoveSortOrder,
  moveTypeGroups,
  sortMoves,
} from "./moveSort";

function mv(id: string, nameJa: string, type = "normal"): Move {
  return { id, nameJa, type, category: "physical", power: 50, priority: 0 };
}

const TYPES = ["bug", "dark", "electric", "fire", "normal", "water"] as const;
const ids = (moves: readonly Move[]): string[] => moves.map((m) => m.id);

describe("並びの種類", () => {
  test("習得順・五十音順・タイプ順の3つで、既定は習得順", () => {
    expect(MOVE_SORT_ORDERS).toEqual(["learnset", "kana", "type"]);
    expect(DEFAULT_MOVE_SORT_ORDER).toBe("learnset");
  });

  test("isMoveSortOrder は3つの値だけ true", () => {
    for (const order of MOVE_SORT_ORDERS) {
      expect(isMoveSortOrder(order)).toBe(true);
    }
    for (const bad of ["", "Kana", "type ", "alphabet", null, undefined, 1]) {
      expect(isMoveSortOrder(bad)).toBe(false);
    }
  });
});

describe("習得順", () => {
  test("入力の順のまま返し、入力の配列は書き換えない(新しい配列)", () => {
    const input = [mv("c", "ウ"), mv("a", "ア"), mv("b", "イ")];
    const snapshot = ids(input);
    const result = sortMoves(input, "learnset", TYPES);
    expect(ids(result)).toEqual(["c", "a", "b"]);
    expect(result).not.toBe(input);
    expect(ids(input)).toEqual(snapshot);
  });
});

describe("五十音順", () => {
  test("あ行からわ行の順に並ぶ(コードポイント順ではない)", () => {
    const input = [
      mv("w", "わるあがき"),
      mv("t", "たいあたり"),
      mv("a", "あてみなげ"),
      mv("n", "なみのり"),
      mv("h", "はたく"),
    ];
    expect(ids(sortMoves(input, "kana", TYPES))).toEqual(["a", "t", "n", "h", "w"]);
  });

  test("ひらがなとカタカナは同じ字として比べる(カタカナがひらがなの後ろにまとまらない)", () => {
    const input = [mv("k3", "きりさく"), mv("g", "ガンマ"), mv("k2", "かみなり")];
    // コードポイント順なら かみなり・きりさく・ガンマ。同じ字として比べると か(み)< か(ん) < き。
    expect(ids(sortMoves(input, "kana", TYPES))).toEqual(["k2", "g", "k3"]);
  });

  test("濁点は同じ字として先に比べる(「が」は「か」の仲間。濁点だけの違いは同じ字の中で後)", () => {
    const input = [mv("kami", "かみなり"), mv("gama", "がまん"), mv("kama", "かまう")];
    // か(ま)う < が(ま)ん < か(み)なり ではなく、まず「かま…」「がま…」を「かみ…」より前に置く。
    expect(ids(sortMoves(input, "kana", TYPES)).indexOf("kami")).toBe(2);
    expect(
      ids(sortMoves(input, "kana", TYPES))
        .slice(0, 2)
        .sort(),
    ).toEqual(["gama", "kama"]);
  });

  test("濁点だけが違う名前は、清音が先・濁音が後(決定的)", () => {
    const input = [mv("b", "ばあと"), mv("a", "はあと")];
    expect(ids(sortMoves(input, "kana", TYPES))).toEqual(["a", "b"]);
  });

  test('長音「ー」は Intl.Collator("ja") の扱いに従う(スーパー は スパーク より前)', () => {
    const input = [mv("spark", "スパーク"), mv("super", "スーパー")];
    expect(ids(sortMoves(input, "kana", TYPES))).toEqual(["super", "spark"]);
  });

  test("ひらがな/カタカナだけが違う同じ読みは同順位になり、技 ID の昇順で決まる(入力の順に依らない)", () => {
    const forward = [mv("id-b", "ガンマ"), mv("id-a", "がんま"), mv("id-c", "ガンマ")];
    const backward = [...forward].reverse();
    expect(ids(sortMoves(forward, "kana", TYPES))).toEqual(["id-a", "id-b", "id-c"]);
    expect(ids(sortMoves(backward, "kana", TYPES))).toEqual(["id-a", "id-b", "id-c"]);
  });

  test("入力の配列を書き換えない", () => {
    const input = [mv("b", "いわ"), mv("a", "あめ")];
    sortMoves(input, "kana", TYPES);
    expect(ids(input)).toEqual(["b", "a"]);
  });
});

describe("タイプ順", () => {
  test("相性表(types)の並びで群にし、群の中は五十音順", () => {
    const input = [
      mv("n1", "たいあたり", "normal"),
      mv("f1", "ひのこ", "fire"),
      mv("e1", "でんきショック", "electric"),
      mv("n2", "かみつく", "normal"),
      mv("e2", "かみなり", "electric"),
      mv("b1", "むしのさざめき", "bug"),
    ];
    expect(ids(sortMoves(input, "type", TYPES))).toEqual(["b1", "e2", "e1", "f1", "n2", "n1"]);
  });

  test("群の並びは入力の種族の技の順ではなく、渡した相性表の並びで決まる", () => {
    const input = [mv("n", "あ", "normal"), mv("b", "あ", "bug")];
    expect(ids(sortMoves(input, "type", ["normal", "bug"]))).toEqual(["n", "b"]);
    expect(ids(sortMoves(input, "type", ["bug", "normal"]))).toEqual(["b", "n"]);
  });

  test("相性表に無いタイプの技は最後に、タイプ ID の昇順で群にまとめる(壊れない)", () => {
    const input = [
      mv("z", "あ", "zeta"),
      mv("n", "い", "normal"),
      mv("y", "う", "alpha"),
      mv("y2", "あ", "alpha"),
    ];
    expect(ids(sortMoves(input, "type", TYPES))).toEqual(["n", "y2", "y", "z"]);
  });

  test("相性表が空でも、全部の技が1群ずつ(タイプ ID の昇順)で残る", () => {
    const input = [mv("w", "あ", "water"), mv("f", "あ", "fire")];
    expect(ids(sortMoves(input, "type", []))).toEqual(["f", "w"]);
  });

  test("moveTypeGroups: タイプごとの群(見出しのタイプ ID と、群の中の技)を並びどおりに返す", () => {
    const input = [
      mv("n", "たいあたり", "normal"),
      mv("e", "かみなり", "electric"),
      mv("n2", "かみつく", "normal"),
    ];
    const groups = moveTypeGroups(input, TYPES);
    expect(groups.map((g) => g.type)).toEqual(["electric", "normal"]);
    expect(groups.map((g) => ids(g.moves))).toEqual([["e"], ["n2", "n"]]);
  });

  test("moveTypeGroups: 技の無いタイプの群は作らない・相性表に無いタイプは最後", () => {
    const groups = moveTypeGroups([mv("x", "あ", "mystery"), mv("f", "あ", "fire")], TYPES);
    expect(groups.map((g) => g.type)).toEqual(["fire", "mystery"]);
  });

  test("moveTypeGroups を平らにすると sortMoves(type) と同じ", () => {
    const input = [mv("n", "たいあたり", "normal"), mv("e", "かみなり", "electric"), mv("q", "あ", "other")];
    expect(ids(moveTypeGroups(input, TYPES).flatMap((g) => g.moves))).toEqual(
      ids(sortMoves(input, "type", TYPES)),
    );
  });
});

describe("空・1件", () => {
  test.each(MOVE_SORT_ORDERS)("%s: 空は空、1件はそのまま", (order) => {
    expect(sortMoves([], order, TYPES)).toEqual([]);
    const one = [mv("only", "ひとつ")];
    expect(ids(sortMoves(one, order, TYPES))).toEqual(["only"]);
  });

  test("moveTypeGroups: 空は群なし", () => {
    expect(moveTypeGroups([], TYPES)).toEqual([]);
  });
});
