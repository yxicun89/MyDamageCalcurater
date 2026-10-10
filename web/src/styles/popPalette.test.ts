// F-12(I-web-6、ADR-0334): ポップ・カラフルな見た目の基盤のトークン。
// 値の正は docs/design.md「ポップ配色」(と「文字」「形・余白」「動き」の追記)。期待値は design.md を毎回読んで作り、
// tokens.css の写しを持たない(コーディング規約 §2。tokens.test.ts と同じ作法)。
// ここで固定するのは「どの名前のトークンが要るか」と「どの組み合わせで AA を満たすか」だけ。値は design.md が正。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { WCAG_NORMAL_TEXT_MIN_CONTRAST, compositeOver, contrastRatio } from "../test/colorContrast";
import {
  type CssNode,
  type CssRule,
  declarationMap,
  normalizeColor,
  parseCss,
  rulesInAtRules,
  splitSelectors,
  topLevelRules,
} from "../test/cssRules";
import {
  type BaseToken,
  baseTokens,
  bulletLine,
  labeledNumber,
  level2SectionOf,
  popPaletteTokens,
  readDesignDoc,
  sectionOf,
  tokenToCssVariable,
} from "../test/designDoc";
import { localPath } from "../test/localPath";

/** WCAG 2.2 SC 1.4.11(UI 部品・グラフィック)/ SC 1.4.3 の大きい文字の最低基準。 */
const WCAG_NON_TEXT_MIN_CONTRAST = 3;

const designDoc = readDesignDoc();
const css: CssNode[] = parseCss(readFileSync(localPath("./tokens.css", import.meta.url), "utf8"));

const normalizeSelector = (selector: string): string => selector.replace(/'/g, '"').replace(/\s+/g, "");
const isDarkSchemeMedia = (prelude: string): boolean =>
  /^@media\b.*prefers-color-scheme\s*:\s*dark/.test(prelude);
const isReducedMotionMedia = (prelude: string): boolean =>
  /^@media\b.*prefers-reduced-motion\s*:\s*reduce/.test(prelude);

const rootRules: CssRule[] = topLevelRules(css).filter((rule) =>
  splitSelectors(rule.selector).includes(":root"),
);
const light = declarationMap(rootRules);
const osDark = declarationMap(rulesInAtRules(css, isDarkSchemeMedia));
const explicitDark = declarationMap(
  topLevelRules(css).filter((rule) =>
    splitSelectors(rule.selector).some((selector) =>
      normalizeSelector(selector).includes('[data-theme="dark"]'),
    ),
  ),
);

/**
 * ADR-0334 §1 で決めたトークン名(design.md「ポップ配色」の表に、この順で並べる)。
 * 名前は iOS と共有する(ADR-0300 §4)。`-ink` で終わる名前は使わない(タイプバッジの文字色専用。tokens.test.ts)。
 */
const REQUIRED_POP_TOKENS = [
  "brand.primary",
  "on.primary",
  "brand.accent",
  "on.accent",
  "success",
  "success.soft",
  "warning",
  "warning.soft",
  "info",
  "info.soft",
  "danger.soft",
  "on.danger",
  "surface.card",
  "bg.gradient-start",
  "bg.gradient-end",
  "table.header",
  "table.zebra",
  "table.hover",
  "focus.ring",
  "shadow.color",
] as const;

function popTokens(): BaseToken[] {
  return popPaletteTokens(designDoc);
}

/** ベース + ポップ配色の両方からトークンの色を引く(組み合わせの検査に使う)。 */
function colorOf(token: string, scheme: "light" | "dark"): string {
  const row = [...baseTokens(designDoc), ...popTokens()].find((candidate) => candidate.token === token);
  if (row === undefined) {
    throw new Error(`design.md の「ベース」「ポップ配色」の表に ${token} が無い`);
  }
  const color = row[scheme].css;
  // 半透明(bg.glass など)は bg.base に重ねた見た目の色で比べる(contrast.test.ts と同じ考え方)。
  if (color.startsWith("rgba")) {
    return compositeOver(color, colorOf("bg.base", scheme));
  }
  return color;
}

function expectVariable(map: Map<string, string>, name: string): string {
  const value = map.get(name);
  expect(value, `${name} が定義されていない`).toBeDefined();
  return value ?? "";
}

describe("design.md「ポップ配色」の表", () => {
  test("ADR-0334 の名前がこの順で並ぶ(ライト・ダークの列を持つ)", () => {
    expect(popTokens().map((token) => token.token)).toEqual([...REQUIRED_POP_TOKENS]);
  });

  test("名前は -ink で終わらない(タイプバッジの文字色の命名と衝突しない)", () => {
    expect(popTokens().filter((token) => tokenToCssVariable(token.token).endsWith("-ink"))).toEqual([]);
  });

  test("名前は --type- で始まらない(タイプ色の命名と衝突しない)", () => {
    expect(popTokens().filter((token) => tokenToCssVariable(token.token).startsWith("--type-"))).toEqual([]);
  });
});

describe("tokens.css はポップ配色を持つ(ライト / ダーク)", () => {
  test.each(popTokens())("$token: :root にライトの値がある", ({ token, light: expected }) => {
    const value = expectVariable(light, tokenToCssVariable(token));
    expect(normalizeColor(value)).toBe(normalizeColor(expected.css));
  });

  test.each(popTokens())("$token: prefers-color-scheme: dark の中にダークの値がある", ({ token, dark }) => {
    const value = expectVariable(osDark, tokenToCssVariable(token));
    expect(normalizeColor(value)).toBe(normalizeColor(dark.css));
  });

  test.each(popTokens())('$token: [data-theme="dark"] でもダークの値になる', ({ token, dark }) => {
    const value = expectVariable(explicitDark, tokenToCssVariable(token));
    expect(normalizeColor(value)).toBe(normalizeColor(dark.css));
  });
});

/**
 * ADR-0334 §1 の「組み合わせ表」。[前景, 背景, 最低比, 用途]。
 * 4.5 は通常文字(SC 1.4.3)、3 は UI 部品の境界・フォーカスの印(SC 1.4.11)。
 * brand.accent は塗りの装飾にだけ使い(文字色・境界線には使わない)、その上の文字は on.accent にする。
 */
const CONTRAST_PAIRS: readonly (readonly [string, string, number, string])[] = [
  ["on.primary", "brand.primary", WCAG_NORMAL_TEXT_MIN_CONTRAST, "主ボタン・選択中のタブの文字"],
  ["brand.primary", "bg.base", WCAG_NORMAL_TEXT_MIN_CONTRAST, "リンク・強調の文字(ページの背景)"],
  ["brand.primary", "bg.glass", WCAG_NORMAL_TEXT_MIN_CONTRAST, "リンク・強調の文字(ガラスの上)"],
  ["brand.primary", "surface.card", WCAG_NORMAL_TEXT_MIN_CONTRAST, "リンク・強調の文字(カードの上)"],
  ["on.accent", "brand.accent", WCAG_NORMAL_TEXT_MIN_CONTRAST, "アクセントの塗りの上の文字"],
  ["success", "bg.base", WCAG_NORMAL_TEXT_MIN_CONTRAST, "成功の文字"],
  ["success", "surface.card", WCAG_NORMAL_TEXT_MIN_CONTRAST, "成功の文字(カードの上)"],
  ["success", "success.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "成功の案内(淡い塗りの上)"],
  ["warning", "bg.base", WCAG_NORMAL_TEXT_MIN_CONTRAST, "注意の文字"],
  ["warning", "surface.card", WCAG_NORMAL_TEXT_MIN_CONTRAST, "注意の文字(カードの上)"],
  ["warning", "warning.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "注意の案内(淡い塗りの上)"],
  ["info", "bg.base", WCAG_NORMAL_TEXT_MIN_CONTRAST, "情報の文字"],
  ["info", "surface.card", WCAG_NORMAL_TEXT_MIN_CONTRAST, "情報の文字(カードの上)"],
  ["info", "info.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "情報・読み込み中の案内(淡い塗りの上)"],
  ["danger", "surface.card", WCAG_NORMAL_TEXT_MIN_CONTRAST, "エラーの文字(カードの上)"],
  ["danger", "danger.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "エラーの案内(淡い塗りの上)"],
  ["on.danger", "danger", WCAG_NORMAL_TEXT_MIN_CONTRAST, "危険ボタンの文字"],
  ...(
    [
      "bg.gradient-start",
      "bg.gradient-end",
      "surface.card",
      "table.header",
      "table.zebra",
      "table.hover",
    ] as const
  ).flatMap((background) => [
    ["text.primary", background, WCAG_NORMAL_TEXT_MIN_CONTRAST, `本文(${background} の上)`] as const,
    ["text.secondary", background, WCAG_NORMAL_TEXT_MIN_CONTRAST, `補足(${background} の上)`] as const,
  ]),
  ["text.primary", "success.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "淡い塗りの上の本文"],
  ["text.primary", "warning.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "淡い塗りの上の本文"],
  ["text.primary", "info.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "淡い塗りの上の本文"],
  ["text.primary", "danger.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "淡い塗りの上の本文"],
  ["focus.ring", "bg.base", WCAG_NON_TEXT_MIN_CONTRAST, "フォーカスの輪(ページの背景)"],
  ["focus.ring", "surface.card", WCAG_NON_TEXT_MIN_CONTRAST, "フォーカスの輪(カードの上)"],
  ["focus.ring", "bg.gradient-start", WCAG_NON_TEXT_MIN_CONTRAST, "フォーカスの輪(背景のグラデーション)"],
  ["focus.ring", "bg.gradient-end", WCAG_NON_TEXT_MIN_CONTRAST, "フォーカスの輪(背景のグラデーション)"],
  ["brand.primary", "bg.gradient-start", WCAG_NON_TEXT_MIN_CONTRAST, "選択中のタブの塗りの境界(背景)"],
  ["brand.primary", "bg.gradient-end", WCAG_NON_TEXT_MIN_CONTRAST, "選択中のタブの塗りの境界(背景)"],
  // G-05(ADR-0339)の部品: 区切りボタン・増減ボタン・タイル・数値欄の枠は text.secondary、選択中のタイルの枠は brand.primary。
  [
    "text.secondary",
    "surface.card",
    WCAG_NON_TEXT_MIN_CONTRAST,
    "区切りボタン・増減ボタン・タイル・数値欄の枠(カードの上)",
  ],
  ["text.secondary", "bg.gradient-start", WCAG_NON_TEXT_MIN_CONTRAST, "同上(背景のグラデーション)"],
  ["text.secondary", "bg.gradient-end", WCAG_NON_TEXT_MIN_CONTRAST, "同上(背景のグラデーション)"],
  [
    "brand.primary",
    "surface.card",
    WCAG_NON_TEXT_MIN_CONTRAST,
    "選択中のタイルの枠・チェックの印・増減ボタンの記号",
  ],
  ["text.primary", "info.soft", WCAG_NORMAL_TEXT_MIN_CONTRAST, "説明ボタンの本文"],
];

describe.each([
  ["ライト", "light"],
  ["ダーク", "dark"],
] as const)("%s テーマの組み合わせは WCAG AA を満たす", (_label, scheme) => {
  test.each(CONTRAST_PAIRS)("%s / %s ≥ %s(%s)", (foreground, background, minimum) => {
    expect(contrastRatio(colorOf(foreground, scheme), colorOf(background, scheme))).toBeGreaterThanOrEqual(
      minimum,
    );
  });
});

describe("文字の段階(design.md「文字」)", () => {
  const section = sectionOf(designDoc, "文字");

  test("タイトル(アプリ名の h1)のサイズ → --font-size-title", () => {
    const expected = labeledNumber(bulletLine(section, "サイズ:"), "タイトル");
    expect(expectVariable(light, "--font-size-title")).toBe(`${expected}px`);
  });

  test.each([
    ["タイトル", "--font-weight-title"],
    ["見出し", "--font-weight-heading"],
    ["強調", "--font-weight-strong"],
    ["本文", "--font-weight-body"],
  ])("太さ %s → %s", (label, variable) => {
    const expected = labeledNumber(bulletLine(section, "太さ:"), label);
    expect(expectVariable(light, variable)).toBe(String(expected));
  });

  test("見出し・タイトルは本文より太い(機械的に見えないよう段階をつける)", () => {
    const body = Number(expectVariable(light, "--font-weight-body"));
    expect(Number(expectVariable(light, "--font-weight-heading"))).toBeGreaterThan(body);
    expect(Number(expectVariable(light, "--font-weight-title"))).toBeGreaterThanOrEqual(
      Number(expectVariable(light, "--font-weight-heading")),
    );
  });
});

describe("影(design.md「形・余白」)", () => {
  const section = sectionOf(designDoc, "形・余白");

  test.each([
    ["カード", "--shadow-card"],
    ["浮き上がり", "--shadow-raised"],
  ])("影 %s → %s は design.md の寸法で、色は var(--shadow-color)", (label, variable) => {
    const line = bulletLine(section, "影:");
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const geometry = new RegExp(`${escaped}\\s*((?:-?\\d+(?:px)?\\s*){3,4})`).exec(line)?.[1]?.trim();
    expect(geometry, `design.md の「影:」に「${label} <x> <y> <ぼかし>」が無い`).toBeDefined();
    expect(expectVariable(light, variable)).toBe(`${geometry ?? ""} var(--shadow-color)`);
  });
});

describe("動き(design.md「動き」)", () => {
  const motion = level2SectionOf(designDoc, "動き");

  test("--duration-press は押下の演出の秒数", () => {
    const line = bulletLine(motion, "押下:");
    const seconds = /(\d+(?:\.\d+)?)\s*秒/.exec(line)?.[1];
    expect(seconds, "design.md の「押下:」に秒数が無い").toBeDefined();
    expect(expectVariable(light, "--duration-press")).toBe(`${seconds ?? ""}s`);
  });

  test("--duration-sheet はシートの出入りの秒数(0.25 秒以内。G-05)", () => {
    const line = bulletLine(motion, "シートの出入り:");
    const seconds = /(\d+(?:\.\d+)?)\s*秒/.exec(line)?.[1];
    expect(seconds, "design.md の「シートの出入り:」に秒数が無い").toBeDefined();
    expect(Number(seconds)).toBeLessThanOrEqual(0.25);
    expect(expectVariable(light, "--duration-sheet")).toBe(`${seconds ?? ""}s`);
  });

  test("prefers-reduced-motion: reduce で --duration-sheet も 0 にする", () => {
    const rootInReduced = declarationMap(
      rulesInAtRules(css, isReducedMotionMedia).filter((rule) =>
        splitSelectors(rule.selector).some((selector) => selector.startsWith(":root")),
      ),
    );
    expect(rootInReduced.get("--duration-sheet")).toMatch(/^(0s|0ms)$/);
  });

  test("prefers-reduced-motion: reduce で --duration-press も 0 にする", () => {
    const rootInReduced = declarationMap(
      rulesInAtRules(css, isReducedMotionMedia).filter((rule) =>
        splitSelectors(rule.selector).some((selector) => selector.startsWith(":root")),
      ),
    );
    expect(rootInReduced.get("--duration-press")).toMatch(/^(0s|0ms)$/);
  });
});

describe("背景のやさしいグラデーション", () => {
  const body = declarationMap(
    topLevelRules(css).filter((rule) => splitSelectors(rule.selector).includes("body")),
  );

  test("body の background-image は bg.gradient-start → bg.gradient-end の linear-gradient(トークン経由)", () => {
    const image = body.get("background-image") ?? "";
    expect(image).toMatch(/^linear-gradient\(/);
    expect(image).toContain("var(--bg-gradient-start)");
    expect(image).toContain("var(--bg-gradient-end)");
  });

  test("グラデーションは画面に固定し、長いページでも縞にならない(background-attachment: fixed)", () => {
    expect(body.get("background-attachment")).toBe("fixed");
  });

  test("下地の色(background-color: var(--bg-base))は残す(グラデーションが描けないときの既定)", () => {
    expect(body.get("background-color")).toBe("var(--bg-base)");
  });
});
