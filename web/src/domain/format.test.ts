// P4-2: 結果の表示の書式(ADR-0300 §8: Web は返ってきた値を加工せずに表示する。ここは書式だけ)。
// 表示%は engine が小数第1位の数値で返す(ADR-0011 §3、tenthPercent)。Web は丸め直さず、常に1桁で書く。
// 確定数の文言は design.md の結果行(「確定2発」「乱数2発」)に合わせる。文言は i18n/ja.ts に置く。

import { describe, expect, test } from "vitest";
import type { CalcResult } from "../engine/types";
import {
  formatEffectiveness,
  formatKO,
  formatMoveCategory,
  formatPercent,
  formatPercentRange,
} from "./format";

describe("formatPercent", () => {
  test.each([
    [73.4, "73.4"],
    [100, "100.0"],
    [0, "0.0"],
    [5, "5.0"],
    [120.5, "120.5"],
  ])("%s → %s(小数第1位を常に1桁)", (value, expected) => {
    expect(formatPercent(value)).toBe(expected);
  });
});

test("formatPercentRange は「最小〜最大%」", () => {
  expect(formatPercentRange({ minPercent: 72.1, maxPercent: 85.3 })).toBe("72.1〜85.3%");
  expect(formatPercentRange({ minPercent: 100, maxPercent: 118 })).toBe("100.0〜118.0%");
});

describe("formatKO", () => {
  test.each<[string, CalcResult["ko"], string]>([
    ["確定1発", { hits: 1, guaranteed: true, chancePercent: 0, displayChancePercent: 100 }, "確定1発"],
    ["確定2発", { hits: 2, guaranteed: true, chancePercent: 0, displayChancePercent: 100 }, "確定2発"],
    // 表示する確率は engine の displayChancePercent(生値 chancePercent ではない。ADR-0006)。
    // chancePercent を1桁に丸めると "49.9" になり displayChancePercent の "50.0" と食い違う値にして、
    // 実装が誤って chancePercent を使ったら検出できるようにする。
    [
      "乱数2発",
      { hits: 2, guaranteed: false, chancePercent: 49.94, displayChancePercent: 50.0 },
      "乱数2発(50.0%)",
    ],
    [
      "乱数3発・整数%",
      { hits: 3, guaranteed: false, chancePercent: 50, displayChancePercent: 50 },
      "乱数3発(50.0%)",
    ],
    ["倒せない", { hits: 0, guaranteed: false, chancePercent: 0, displayChancePercent: 0 }, "倒せない"],
  ])("%s", (_label, ko, expected) => {
    expect(formatKO(ko)).toBe(expected);
  });
});

describe("formatEffectiveness(engine の結果の effectiveness をそのまま言葉にする。TS で相性を計算しない)", () => {
  test.each([
    [0, "効果なし"],
    [0.25, "効果はいまひとつ"],
    [0.5, "効果はいまひとつ"],
    [1, "等倍"],
    [2, "効果はばつぐん"],
    [4, "効果はばつぐん"],
  ])("%s → %s", (value, expected) => {
    expect(formatEffectiveness(value)).toBe(expected);
  });
});

test("formatMoveCategory は技の分類の表示名", () => {
  expect(formatMoveCategory("physical")).toBe("物理");
  expect(formatMoveCategory("special")).toBe("特殊");
  expect(formatMoveCategory("status")).toBe("変化");
});
