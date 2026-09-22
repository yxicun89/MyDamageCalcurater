// P4-12a: タイプバランスの倍率の表示(ADR-0303 §2、docs/type-balance-design.md §10)。
// 倍率は色だけで表さず文字でも出す(×4 弱点 / ×2 弱点 / ×1 等倍 / ×1/2 耐性 / ×1/4 耐性 / ×0 無効)。
// 数値は balance-svc の応答の文字列(DefenseMultiplier / CoverageMultiplier)をそのまま使い、Web で計算しない
// (ADR-0303 §1)。語は応答の category(DefenseCategory)から選ぶ(値の範囲を Web で判定しない)。

import { describe, expect, test } from "vitest";
import type { components } from "../api/balance.gen";
import { coverageMultiplierLabel, defenseMultiplierLabel } from "./balanceLabels";

type Schemas = components["schemas"];

describe("defenseMultiplierLabel(防御相性)", () => {
  test.each([
    ["4", "quad_weak", "×4 弱点"],
    ["2", "weak", "×2 弱点"],
    ["1", "neutral", "×1 等倍"],
    ["1/2", "resist", "×1/2 耐性"],
    ["1/4", "quad_resist", "×1/4 耐性"],
    ["0", "immune", "×0 無効"],
  ] as const satisfies ReadonlyArray<
    readonly [Schemas["DefenseMultiplier"], Schemas["DefenseCategory"], string]
  >)("%s・%s は「%s」", (multiplier, category, expected) => {
    expect(defenseMultiplierLabel(multiplier, category)).toBe(expected);
  });

  test.each([
    ["3/4", "resist", "×3/4 耐性"],
    ["5/4", "weak", "×5/4 弱点"],
    ["3", "weak", "×3 弱点"],
    ["5/2", "weak", "×5/2 弱点"],
  ] as const satisfies ReadonlyArray<
    readonly [Schemas["DefenseMultiplier"], Schemas["DefenseCategory"], string]
  >)("特性の倍率(%s・%s)も応答の文字列のまま「%s」", (multiplier, category, expected) => {
    expect(defenseMultiplierLabel(multiplier, category)).toBe(expected);
  });

  test("語は倍率の値でなく category で決める(値から判定し直さない)", () => {
    // 契約上ありえない組でも、category の語をそのまま使う(Web で範囲判定をしていないことの確認)。
    expect(defenseMultiplierLabel("2", "resist")).toBe("×2 耐性");
  });
});

describe("coverageMultiplierLabel(攻撃範囲)", () => {
  test.each([
    ["2", "×2 抜群"],
    ["1", "×1 等倍"],
    ["1/2", "×1/2 いまひとつ"],
    ["0", "×0 無効"],
  ] as const satisfies ReadonlyArray<readonly [NonNullable<Schemas["CoverageMultiplier"]>, string]>)(
    "%s は「%s」",
    (multiplier, expected) => {
      expect(coverageMultiplierLabel(multiplier)).toBe(expected);
    },
  );

  test("null(攻撃技なし)は「攻撃技なし」", () => {
    expect(coverageMultiplierLabel(null)).toBe("攻撃技なし");
  });
});
