// P4-1: docs/design.md のデザイントークンと web/src/styles/tokens.css の CSS 変数が一致することの検査。
// 期待値は design.md を毎回読んで作る(tokens.css の写しを持たない。コーディング規約 §2)。

import { readFileSync, readdirSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";
import { typeNameJa } from "../i18n/ja";
import { typeIds } from "../test/typeChart";
import {
  type CssNode,
  type CssRule,
  declarationMap,
  importantDeclarationMap,
  normalizeColor,
  parseCss,
  rulesInAtRules,
  splitSelectors,
  topLevelRules,
} from "../test/cssRules";
import {
  baseTokens,
  bulletLine,
  glassBlurPixels,
  labeledNumber,
  level2SectionOf,
  readDesignDoc,
  sectionOf,
  tokenToCssVariable,
  typeBadgeInkByJapaneseName,
  typeColorsByJapaneseName,
} from "../test/designDoc";

const tokensCssPath = localPath("./tokens.css", import.meta.url);

const designDoc = readDesignDoc();
const css: CssNode[] = parseCss(readFileSync(tokensCssPath, "utf8"));

function normalizeSelector(selector: string): string {
  return selector.replace(/'/g, '"').replace(/\s+/g, "");
}

const isDarkSchemeMedia = (prelude: string): boolean =>
  /^@media\b.*prefers-color-scheme\s*:\s*dark/.test(prelude);
const isReducedMotionMedia = (prelude: string): boolean =>
  /^@media\b.*prefers-reduced-motion\s*:\s*reduce/.test(prelude);

const rootRules: CssRule[] = topLevelRules(css).filter((rule) =>
  splitSelectors(rule.selector).includes(":root"),
);
const explicitDarkRules: CssRule[] = topLevelRules(css).filter((rule) =>
  splitSelectors(rule.selector).some((selector) =>
    normalizeSelector(selector).includes('[data-theme="dark"]'),
  ),
);
const osDarkRules: CssRule[] = rulesInAtRules(css, isDarkSchemeMedia);

const light = declarationMap(rootRules);
const osDark = declarationMap(osDarkRules);
const explicitDark = declarationMap(explicitDarkRules);

function expectVariable(map: Map<string, string>, name: string): string {
  const value = map.get(name);
  expect(value, `${name} が定義されていない`).toBeDefined();
  return value ?? "";
}

describe("ベースのトークン(ライト / ダーク)", () => {
  const tokens = baseTokens(designDoc);

  test("design.md のベースの表を読めている", () => {
    expect(tokens.map((token) => token.token)).toEqual([
      "bg.base",
      "bg.glass",
      "text.primary",
      "text.secondary",
      "border.hairline",
      "danger",
    ]);
  });

  test.each(tokens)("$token: :root にライトの値がある", ({ token, light: expected }) => {
    const value = expectVariable(light, tokenToCssVariable(token));
    expect(normalizeColor(value)).toBe(normalizeColor(expected.css));
  });

  test.each(tokens)(
    "$token: prefers-color-scheme: dark の中にダークの値がある",
    ({ token, dark: expected }) => {
      const value = expectVariable(osDark, tokenToCssVariable(token));
      expect(normalizeColor(value)).toBe(normalizeColor(expected.css));
    },
  );

  test.each(tokens)(
    '$token: [data-theme="dark"] の明示指定でもダークの値になる',
    ({ token, dark: expected }) => {
      const value = expectVariable(explicitDark, tokenToCssVariable(token));
      expect(normalizeColor(value)).toBe(normalizeColor(expected.css));
    },
  );

  test("ぼかし付きのトークンはぼかし量を別変数(<変数名>-blur)で持つ", () => {
    const blurred = tokens.filter((token) => token.light.blur || token.dark.blur);
    expect(blurred.map((token) => token.token)).toEqual(["bg.glass"]);
    const expectedBlurPx = glassBlurPixels(designDoc);
    for (const token of blurred) {
      const value = expectVariable(light, `${tokenToCssVariable(token.token)}-blur`);
      expect(value).toBe(`${expectedBlurPx}px`);
    }
  });

  test('[data-theme="light"] の明示指定は OS のダーク設定より優先される', () => {
    // OS がダークでも data-theme="light" ならライトのまま。
    // 方法は2通りのどちらか: ダークの @media のセレクタを :root:not([data-theme="light"]) にする、
    // または ダークの @media 内に [data-theme="light"] の規則を置いてライトの値に戻す。
    const osDarkSelectors = osDarkRules.flatMap((rule) =>
      splitSelectors(rule.selector).map(normalizeSelector),
    );
    const excludesLight =
      osDarkSelectors.length > 0 &&
      osDarkRules
        .filter((rule) => rule.declarations.some((declaration) => declaration.property.startsWith("--")))
        .every((rule) =>
          splitSelectors(rule.selector)
            .map(normalizeSelector)
            .every((selector) => selector.includes(':not([data-theme="light"])')),
        );
    if (excludesLight) {
      return;
    }
    const lightOverride = declarationMap(
      osDarkRules.filter((rule) =>
        splitSelectors(rule.selector).some((selector) =>
          normalizeSelector(selector).includes('[data-theme="light"]'),
        ),
      ),
    );
    for (const token of tokens) {
      const value = expectVariable(lightOverride, tokenToCssVariable(token.token));
      expect(normalizeColor(value)).toBe(normalizeColor(token.light.css));
    }
  });
});

describe("タイプ色", () => {
  const colorsByName = typeColorsByJapaneseName(designDoc);
  const idByName = new Map(Object.entries(typeNameJa).map(([id, name]) => [name, id]));

  test("design.md のタイプ色は18タイプ", () => {
    expect(colorsByName.size).toBe(18);
  });

  test.each([...colorsByName.entries()])(
    "%s: ja.ts の表示名から ID が引け、--type-<id> が design.md の色",
    (name, color) => {
      const id = idByName.get(name);
      expect(id, `ja.ts に表示名「${name}」が無い`).toBeDefined();
      const value = expectVariable(light, `--type-${id ?? ""}`);
      expect(normalizeColor(value)).toBe(normalizeColor(color));
    },
  );

  test("相性表の types の全 ID に --type-<id> があり、余分なタイプ色は無い", () => {
    const defined = [...light.keys()].filter((name) => name.startsWith("--type-") && !name.endsWith("-ink"));
    expect([...defined].sort()).toEqual(
      typeIds()
        .map((id) => `--type-${id}`)
        .sort(),
    );
  });

  // issue #306: タイプバッジの文字色。背景はタイプ色そのものなので、文字色だけを別変数に持つ。
  test("相性表の types の全 ID に --type-<id>-ink があり、余分な文字色は無い", () => {
    const defined = [...light.keys()].filter((name) => name.endsWith("-ink"));
    expect([...defined].sort()).toEqual(
      typeIds()
        .map((id) => `--type-${id}-ink`)
        .sort(),
    );
  });

  test.each([...typeBadgeInkByJapaneseName(designDoc).entries()])(
    "%s: --type-<id>-ink が design.md「タイプバッジ」の文字色 %s",
    (name, ink) => {
      const id = idByName.get(name);
      expect(id, `ja.ts に表示名「${name}」が無い`).toBeDefined();
      const value = expectVariable(light, `--type-${id ?? ""}-ink`);
      expect(normalizeColor(value)).toBe(normalizeColor(ink));
    },
  );

  // 文字色(--type-<id>-ink)もタイプ色と同じ扱い: テーマで切り替えない(design.md「タイプバッジ」)。
  test("タイプ色・バッジの文字色はライト / ダークで同じ(ダーク側で上書きしない)", () => {
    const overridden = [...osDark.keys(), ...explicitDark.keys()].filter((name) =>
      name.startsWith("--type-"),
    );
    expect(overridden).toEqual([]);
  });
});

describe("文字", () => {
  const section = sectionOf(designDoc, "文字");

  test("--font-family は design.md の Web の指定と同じ並び", () => {
    const webLine = bulletLine(section, "Web:");
    const expected = (webLine.replace(/^Web:\s*/, "").split("。")[0] ?? "")
      .split(",")
      .map((family) => family.trim().replace(/"/g, ""));
    expect(expected).toEqual(["SF Pro Rounded", "M PLUS Rounded 1c", "system-ui"]);
    const actual = expectVariable(light, "--font-family")
      .split(",")
      .map((family) => family.trim().replace(/["']/g, ""));
    expect(actual.slice(0, expected.length)).toEqual(expected);
  });

  test.each([
    ["結果の%表示", "--font-size-result"],
    ["見出し", "--font-size-heading"],
    ["本文", "--font-size-body"],
    ["補足", "--font-size-caption"],
  ])("%s → %s", (label, variable) => {
    const expected = labeledNumber(bulletLine(section, "サイズ:"), label);
    expect(expectVariable(light, variable)).toBe(`${expected}px`);
  });
});

describe("形・余白", () => {
  const section = sectionOf(designDoc, "形・余白");

  test.each([
    ["カード", "--radius-card"],
    ["ボタン・チップ", "--radius-pill"],
    ["入力", "--radius-input"],
  ])("角丸 %s → %s", (label, variable) => {
    const expected = labeledNumber(bulletLine(section, "角丸:"), label);
    expect(expectVariable(light, variable)).toBe(`${expected}px`);
  });

  test("余白は --space-1 から順に design.md の値", () => {
    const line = bulletLine(section, "余白:");
    const list = /[((]([\d,\s、]+)[))]/.exec(line)?.[1] ?? "";
    const expected = list.split(/[,、]/).map((part) => parseInt(part.trim(), 10));
    expect(expected).toEqual([4, 8, 12, 16, 24]);
    expected.forEach((pixels, index) => {
      expect(expectVariable(light, `--space-${index + 1}`)).toBe(`${pixels}px`);
    });
    expect(light.has(`--space-${expected.length + 1}`)).toBe(false);
  });
});

describe("動き", () => {
  const motion = level2SectionOf(designDoc, "動き");

  test("--duration-swap は攻守入れ替えの秒数", () => {
    const line = bulletLine(motion, "攻守入れ替え:");
    const seconds = /(\d+(?:\.\d+)?)\s*秒/.exec(line)?.[1];
    expect(seconds).toBe("0.35");
    expect(expectVariable(light, "--duration-swap")).toBe(`${seconds ?? ""}s`);
  });

  test("prefers-reduced-motion: reduce で遷移・アニメーションを無効化する", () => {
    const reduced = rulesInAtRules(css, isReducedMotionMedia);
    expect(reduced.length, "@media (prefers-reduced-motion: reduce) が無い").toBeGreaterThan(0);

    const universal = reduced.filter((rule) =>
      splitSelectors(rule.selector).some((selector) => selector.includes("*")),
    );
    const declarations = declarationMap(universal);
    const important = importantDeclarationMap(universal);
    const isOff = (value: string | undefined): boolean =>
      value !== undefined && /^(none|0s|0ms|0\.01ms|1ms)$/.test(value.trim());
    // 他の場所の transition/animation 宣言に確実に勝つよう、無効化する宣言は !important を必須にする
    // (animationend を発火させ続けるため animation: none ではなく継続時間を 0 にする形を使う)。
    const isOffAndImportant = (property: string): boolean =>
      isOff(declarations.get(property)) && important.get(property) === true;
    expect(
      isOffAndImportant("transition-duration") ||
        (isOff(declarations.get("transition")) && important.get("transition") === true),
      "全要素(*)の transition を !important で無効化していない",
    ).toBe(true);
    expect(
      isOffAndImportant("animation-duration") ||
        (isOff(declarations.get("animation")) && important.get("animation") === true),
      "全要素(*)の animation を !important で無効化していない",
    ).toBe(true);

    // CSS 変数で時間を参照する箇所(JS からの読み取りを含む)も止まるよう、変数も 0 にする。
    const rootInReduced = declarationMap(
      reduced.filter((rule) => splitSelectors(rule.selector).some((s) => s.startsWith(":root"))),
    );
    expect(isOff(rootInReduced.get("--duration-swap")), "--duration-swap を 0 にしていない").toBe(true);
  });
});

describe("body の既定", () => {
  const body = declarationMap(
    topLevelRules(css).filter((rule) => splitSelectors(rule.selector).includes("body")),
  );

  test("背景・文字色・書体・等幅数字がトークン経由", () => {
    expect(body.get("background-color") ?? body.get("background")).toBe("var(--bg-base)");
    expect(body.get("color")).toBe("var(--text-primary)");
    expect(body.get("font-family")).toBe("var(--font-family)");
    expect(body.get("font-variant-numeric")).toBe("tabular-nums");
  });
});

describe("web/src の CSS は design.md 由来の色だけ", () => {
  const srcDir = localPath("../", import.meta.url);

  function cssFiles(): string[] {
    return readdirSync(srcDir, { recursive: true, encoding: "utf8" })
      .map((path) => path.replace(/\\/g, "/"))
      .filter((path) => path.endsWith(".css"));
  }

  test("走査対象に styles/tokens.css が含まれる(走査の前提の確認)", () => {
    expect(cssFiles()).toContain("styles/tokens.css");
  });

  test("トークン定義(--*)以外の宣言に色の直書きが無い(tokens.css に限らず全 CSS)", () => {
    const colorLiteral = /#[0-9a-fA-F]{3,8}\b|rgba?\(/;
    const offenders: string[] = [];
    const visit = (nodes: readonly CssNode[], file: string): void => {
      for (const node of nodes) {
        if (node.kind === "at-rule") {
          visit(node.children, file);
          continue;
        }
        for (const declaration of node.declarations) {
          if (!declaration.property.startsWith("--") && colorLiteral.test(declaration.value)) {
            offenders.push(`${file}: ${node.selector} { ${declaration.property}: ${declaration.value} }`);
          }
        }
      }
    };
    for (const file of cssFiles()) {
      visit(parseCss(readFileSync(`${srcDir}${file}`, "utf8")), file);
    }
    expect(offenders).toEqual([]);
  });
});
