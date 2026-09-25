// issue 271 / issue 270(Web レーン): 「未対応」の印(ADR-0123)を「結果の先頭に1回」「その行・候補だけ」に
// 振り分ける splitUnsupportedMarks の単体テスト。置き場所のロジックは iOS レーンの決定
// (docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、ADR-0501「P6-17」)に揃える:
// 「全行(全候補)が持つ印は結果の上に1回、残りはその行(候補)だけ」。target で決め打ちせず、
// 印の内容(target・reason・id の組)が全行にあるかどうかで判定する(CalcScreen.tsx/ReverseScreen.tsx が
// 画面の結合テストで確かめるのは表示側の組み立てなので、ここでは振り分けそのものを直接確かめる)。

import { describe, expect, test } from "vitest";
import type { UnsupportedMark } from "../engine/types";
import { splitUnsupportedMarks } from "./unsupportedLabels";

const multiHit = (moveId: string): UnsupportedMark => ({ target: "move", reason: "multi_hit", id: moveId });
const itemMark = (itemId: string): UnsupportedMark => ({
  target: "attacker_item",
  reason: "unsupported_effect",
  id: itemId,
});

describe("splitUnsupportedMarks", () => {
  test("印が無ければ common・perRow とも空", () => {
    const result = splitUnsupportedMarks([[], []]);
    expect(result.common).toEqual([]);
    expect(result.perRow).toEqual([[], []]);
  });

  test("結果が0件(rowMarks が空配列)のときは common・perRow とも空", () => {
    const result = splitUnsupportedMarks([]);
    expect(result.common).toEqual([]);
    expect(result.perRow).toEqual([]);
  });

  test("1件の行にしかない印は perRow に残り、common には入らない", () => {
    const mark = multiHit("m1");
    const result = splitUnsupportedMarks([[mark], []]);
    expect(result.common).toEqual([]);
    expect(result.perRow).toEqual([[mark], []]);
  });

  // 技の印は全行に付くことが多い(iOS レーンの指摘: 「技の印は全行に付くので行ごとに出すと同じ文言が
  // 5〜10回並ぶ」)。全行に同じ印(target・reason・id が同じ)があるときは共通の印として1つにまとめる。
  test("全行にある同じ印(target・reason・id が同じ)は common に1つだけ入り、perRow からは消える", () => {
    const mark = multiHit("m1");
    const result = splitUnsupportedMarks([[mark], [multiHit("m1")]]);
    expect(result.common).toEqual([mark]);
    expect(result.perRow).toEqual([[], []]);
  });

  // 技由来の印(全行共通になりやすい)+ 持ち物バリアントで変わる印(一部の行だけ)が混在するケース。
  test("共通の印と行固有の印が混在するとき、共通は common に、残りはその行の perRow に入る", () => {
    const common = multiHit("m1");
    const onlyRow0 = itemMark("item-a");
    const onlyRow2 = itemMark("item-b");
    const result = splitUnsupportedMarks([[common, onlyRow0], [multiHit("m1")], [multiHit("m1"), onlyRow2]]);
    expect(result.common).toEqual([common]);
    expect(result.perRow).toEqual([[onlyRow0], [], [onlyRow2]]);
  });

  test("印がある行と無い行が混ざると、その印は全行共通にならず perRow に残る", () => {
    const mark = multiHit("m1");
    // 3行中2行にしか無い(3行目は印なし)ので「全行」を満たさない
    const result = splitUnsupportedMarks([[mark], [multiHit("m1")], []]);
    expect(result.common).toEqual([]);
    expect(result.perRow).toEqual([[mark], [multiHit("m1")], []]);
  });

  test("行が1件だけのとき、その行の印はそのまま common になる(1行 = 全行)", () => {
    const mark = multiHit("m1");
    const result = splitUnsupportedMarks([[mark]]);
    expect(result.common).toEqual([mark]);
    expect(result.perRow).toEqual([[]]);
  });

  test("common は最初に現れた行の順、perRow は engine が返した順のまま(並べ替え・重複除去をしない)", () => {
    const zeroPower: UnsupportedMark = { target: "move", reason: "zero_power", id: "m1" };
    const common = multiHit("m1");
    const onlyRow0 = itemMark("item-a");
    const result = splitUnsupportedMarks([
      [onlyRow0, common, zeroPower],
      [multiHit("m1"), zeroPower],
    ]);
    // common: multiHit と zeroPower はどちらも全行にあるので両方 common、出現順(1行目の並び)を保つ
    expect(result.common).toEqual([common, zeroPower]);
    // perRow: common に振り分けられた分を除いた残り。順序は元の並びのまま
    expect(result.perRow).toEqual([[onlyRow0], []]);
  });
});
