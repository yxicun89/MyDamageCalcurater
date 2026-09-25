// issue #304: 入力(select・数値入力)とボタンの形が、ブラウザ既定のままになっていないことの静的検査。
// 正は docs/design.md「形・余白」(角丸: カード 20 / ボタン・チップ 999(ピル)/ 入力 12)と
// 「入力のラベル」(select・数値入力は入力の角丸、ボタンはピル)。
// 期待値は design.md と tokens.css から毎回作り、画面 CSS の写しを持たない
// (コーディング規約 §2 の独立な検証。tokens.test.ts・responsive.test.ts と同じ作法)。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { type CssRule, parseCss, splitSelectors, topLevelRules } from "../test/cssRules";
import { bulletLine, labeledNumber, readDesignDoc, sectionOf } from "../test/designDoc";
import { localPath } from "../test/localPath";

const designDoc = readDesignDoc();
const shapeSection = sectionOf(designDoc, "形・余白");
const radiusLine = bulletLine(shapeSection, "角丸:");

/** :root の CSS 変数(値 → 変数名)。design.md の「角丸 入力 12」から変数名 `--radius-input` を引くため。 */
function rootVariablesByValue(): Map<string, string> {
  const nodes = parseCss(readFileSync(localPath("./tokens.css", import.meta.url), "utf8"));
  const map = new Map<string, string>();
  for (const rule of topLevelRules(nodes)) {
    if (!splitSelectors(rule.selector).includes(":root")) {
      continue;
    }
    for (const declaration of rule.declarations) {
      if (declaration.property.startsWith("--") && !map.has(declaration.value)) {
        map.set(declaration.value, declaration.property);
      }
    }
  }
  return map;
}

const variablesByValue = rootVariablesByValue();

/** design.md の角丸のラベル(「入力」「ボタン・チップ」)から、使うべき `var(--…)` を作る。 */
function radiusVar(label: string): string {
  const pixels = labeledNumber(radiusLine, label);
  const variable = variablesByValue.get(`${String(pixels)}px`);
  if (variable === undefined) {
    throw new Error(`tokens.css に ${String(pixels)}px の変数(design.md の角丸「${label}」)が無い`);
  }
  return `var(${variable})`;
}

const screenCss: Record<string, CssRule[]> = {
  calc: topLevelRules(
    parseCss(readFileSync(localPath("../screens/CalcScreen.css", import.meta.url), "utf8")),
  ),
  reverse: topLevelRules(
    parseCss(readFileSync(localPath("../screens/ReverseScreen.css", import.meta.url), "utf8")),
  ),
  balance: topLevelRules(
    parseCss(readFileSync(localPath("../screens/BalanceScreen.css", import.meta.url), "utf8")),
  ),
};

function ruleFor(screen: keyof typeof screenCss, selector: string): CssRule {
  const rules = screenCss[screen] ?? [];
  const rule = rules.find((candidate) => splitSelectors(candidate.selector).includes(selector));
  if (rule === undefined) {
    throw new Error(`${screen} の CSS に ${selector} の規則が無い`);
  }
  return rule;
}

function declaration(rule: CssRule, property: string): string {
  const value = rule.declarations.findLast((candidate) => candidate.property === property)?.value;
  if (value === undefined) {
    throw new Error(`${rule.selector} に ${property} が無い`);
  }
  return value;
}

describe("技の select と「観測を追加」ボタンに形のトークンを当てる(issue #304)", () => {
  // クラス名はこの仕様で決めた受け口(画面の JSX がこの名前を付ける)。
  test.each([
    ["calc", ".calc-screen__move", "入力"],
    ["reverse", ".reverse-screen__move", "入力"],
    ["reverse", ".reverse-observations__add", "ボタン・チップ"],
  ] as const)("%s の %s は角丸「%s」のトークン", (screen, selector, label) => {
    expect(declaration(ruleFor(screen, selector), "border-radius")).toBe(radiusVar(label));
  });

  test.each([
    ["calc", ".calc-screen__move"],
    ["reverse", ".reverse-screen__move"],
    ["reverse", ".reverse-observations__add"],
  ] as const)("%s の %s は色・文字もトークンで指定する(ブラウザ既定のままにしない)", (screen, selector) => {
    const rule = ruleFor(screen, selector);
    for (const property of ["font-family", "font-size"]) {
      expect(declaration(rule, property)).toMatch(/^var\(--/);
    }
  });
});

describe("見えるラベルを CSS で隠さない(issue #304)", () => {
  // 「見えるラベルを足す」を、読み上げ専用(視覚的に隠す)で済ませていないことの歯止め。
  // 見えるラベルの規則はセレクタに `label` を含む名前にする(例: `.calc-card__label`)。
  // ネイティブのラジオを隠す既存の規則(.calc-preset__input など)は、ラベルではないので対象外。
  const hidingPatterns = [/^clip$/, /^clip-path$/];

  test.each(["calc", "reverse", "balance"] as const)("%s の見えるラベルを隠していない", (screen) => {
    const labelRules = (screenCss[screen] ?? []).filter((rule) =>
      splitSelectors(rule.selector).some((selector) => selector.includes("label")),
    );
    expect(labelRules.length, `${screen} の CSS に見えるラベルの規則が無い`).toBeGreaterThan(0);
    for (const rule of labelRules) {
      for (const { property, value } of rule.declarations) {
        expect(hidingPatterns.some((pattern) => pattern.test(property))).toBe(false);
        expect(`${property}: ${value}`).not.toMatch(/^width:\s*1px$/);
        expect(`${property}: ${value}`).not.toMatch(/^display:\s*none$/);
      }
    }
  });
});
