// AJ6: 調整のリクエストの補助(ADR-0319 §2・§5・受け入れ条件 AC3)。
//   R1 火力指数の補正はタイプ一致だけ(一致 6144・不一致 4096)。持ち物・特性・テラスタルは含めない
//   R2 プリセットの性格(plus/minus)→ マスタの性格 ID。無補正は plus・minus とも null の先頭。無ければ null
//   R3 プリセットの定数は契約の範囲に収まる(発数 1..10、確率 (0, 100]、limit 1..200)
// 架空データだけを使う(実マスタは使わない)。

import { describe, expect, test } from "vitest";
import type { MasterNature } from "../master/types";
import {
  ADJUST_HITS_OPTIONS,
  ADJUST_NEUTRAL_MODIFIER,
  ADJUST_THRESHOLD_PRESETS,
  DEFAULT_ADJUST_HITS,
  DEFAULT_ADJUST_THRESHOLD_PERCENT,
  LEARNERS_PAGE_SIZE,
  MAX_ADJUST_HITS,
  STAB_MODIFIER,
  firepowerModifier,
  natureIdForPreset,
} from "./adjustRequest";

describe("R1 firepowerModifier", () => {
  test("技のタイプが種族のタイプのどれかと一致すれば ×1.5(6144)", () => {
    expect(firepowerModifier(["fire", "flying"], "fire")).toBe(STAB_MODIFIER);
    expect(firepowerModifier(["fire", "flying"], "flying")).toBe(STAB_MODIFIER);
    expect(STAB_MODIFIER).toBe(6144);
  });

  test("一致しなければ等倍(4096)", () => {
    expect(firepowerModifier(["fire", "flying"], "water")).toBe(ADJUST_NEUTRAL_MODIFIER);
    expect(firepowerModifier([], "water")).toBe(ADJUST_NEUTRAL_MODIFIER);
    expect(ADJUST_NEUTRAL_MODIFIER).toBe(4096);
  });
});

describe("R2 natureIdForPreset", () => {
  const natures: readonly MasterNature[] = [
    { id: "test-nature-plus-atk", nameJa: "テストいじっぱり", plus: "atk", minus: "spa" },
    { id: "test-nature-neutral-a", nameJa: "テストまじめ", plus: null, minus: null },
    { id: "test-nature-neutral-b", nameJa: "テストすなお", plus: null, minus: null },
    { id: "test-nature-plus-spa", nameJa: "テストひかえめ", plus: "spa", minus: "atk" },
    { id: "test-nature-plus-def", nameJa: "テストわんぱく", plus: "def", minus: "atk" },
  ];

  test("plus・minus が一致する性格の ID", () => {
    expect(natureIdForPreset(natures, { plus: "atk", minus: "spa" })).toBe("test-nature-plus-atk");
    expect(natureIdForPreset(natures, { plus: "spa", minus: "atk" })).toBe("test-nature-plus-spa");
    expect(natureIdForPreset(natures, { plus: "def", minus: "atk" })).toBe("test-nature-plus-def");
  });

  test('無補正(plus・minus とも "")は、マスタの無補正の性格のうち先頭', () => {
    expect(natureIdForPreset(natures, { plus: "", minus: "" })).toBe("test-nature-neutral-a");
  });

  test("一致する性格が無ければ null(勝手に別の性格で代えない)", () => {
    expect(natureIdForPreset(natures, { plus: "spd", minus: "atk" })).toBeNull();
    expect(natureIdForPreset([], { plus: "", minus: "" })).toBeNull();
  });
});

describe("R3 プリセットの定数", () => {
  test("発数は 1..10(契約 AdjustHits)で、既定は 1", () => {
    expect(MAX_ADJUST_HITS).toBe(10);
    expect(ADJUST_HITS_OPTIONS).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
    expect(DEFAULT_ADJUST_HITS).toBe(1);
  });

  test("確率の選択肢は (0, 100] で、先頭が既定の 100(確定)", () => {
    expect(ADJUST_THRESHOLD_PRESETS[0]).toBe(DEFAULT_ADJUST_THRESHOLD_PERCENT);
    expect(DEFAULT_ADJUST_THRESHOLD_PERCENT).toBe(100);
    for (const percent of ADJUST_THRESHOLD_PRESETS) {
      expect(percent).toBeGreaterThan(0);
      expect(percent).toBeLessThanOrEqual(100);
    }
  });

  test("技を覚えるポケモンの1ページは契約の limit の範囲(1..200)", () => {
    expect(LEARNERS_PAGE_SIZE).toBeGreaterThanOrEqual(1);
    expect(LEARNERS_PAGE_SIZE).toBeLessThanOrEqual(200);
  });
});
