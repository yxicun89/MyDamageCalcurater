// AJ6: 調整の表示の書式(ADR-0319 §6・受け入れ条件 AC6)。
//   F1 確率は 0.1% 単位で切り捨て(99.99% を「100%」と出して確定に見せない)。整数は小数点なし
//   F2 エラーは code から日本語の文言を引く。サーバーの message は使わない。未知の code は fallback

import { describe, expect, test } from "vitest";
import { adjustErrorText } from "../i18n/ja";
import { adjustErrorMessage, formatChancePercent } from "./adjustFormat";

describe("F1 formatChancePercent", () => {
  test.each([
    [100, "100%"],
    [0, "0%"],
    [50, "50%"],
    [37.5, "37.5%"],
    [12.34, "12.3%"],
    [99.99, "99.9%"],
    [99.95, "99.9%"],
    [0.05, "0%"],
    [6.25, "6.2%"],
  ])("%f → %s", (percent, expected) => {
    expect(formatChancePercent(percent)).toBe(expected);
  });

  test("100 未満は決して「100%」にならない(確定と取り違えない)", () => {
    for (const percent of [99.9, 99.94, 99.999, 99.99999]) {
      expect(formatChancePercent(percent)).not.toBe("100%");
    }
  });
});

describe("F2 adjustErrorMessage", () => {
  test.each([
    "invalid_input",
    "invalid_enum",
    "unknown_species",
    "unknown_move",
    "unknown_nature",
    "unknown_item",
    "unknown_ability",
    "not_found",
    "master_unavailable",
    "upstream_unavailable",
    "type_chart_missing",
    "adjust_unavailable",
  ] as const)("%s は adjustErrorText の文言", (code) => {
    expect(adjustErrorMessage(code)).toBe(adjustErrorText[code]);
  });

  test("未知のコードは fallback(サーバーの英語の message を出さない)", () => {
    expect(adjustErrorMessage("something_new")).toBe(adjustErrorText.fallback);
    expect(adjustErrorMessage("")).toBe(adjustErrorText.fallback);
  });

  test("Object.prototype の名前(toString など)を既知のコードと取り違えない", () => {
    expect(adjustErrorMessage("toString")).toBe(adjustErrorText.fallback);
    expect(adjustErrorMessage("constructor")).toBe(adjustErrorText.fallback);
  });

  test("文言はどれも日本語で空でない(英語の内部メッセージを持たない)", () => {
    for (const text of Object.values(adjustErrorText)) {
      expect(text).not.toBe("");
      expect(text).toMatch(/[ぁ-んァ-ヶ一-龠]/);
    }
  });
});
