// P4-16c: 種族の検索欄(screens/SpeciesSearchField.tsx)の見た目の静的検査。
// P4-16b では species-search__* のクラスに対応する CSS がどこにも無く、素の <ul>(既定の箇条書き)の
// まま出ていた。ここで「どの規則が要るか」を受け入れ条件として固定する。
// 値の正は docs/design.md(トークン)と tokens.css。色・余白・角丸はトークン(CSS 変数)で書き、
// 生の値を書かない(コーディング規約 §2、styles/noColorLiterals.test.ts と同じ考え方)。
// 演出(常時動くものを置かない)は styles/motion.test.ts が全 CSS を走査するので、ここでは繰り返さない。

import { existsSync, readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { declarationMap, parseCss, type CssNode, type CssRule } from "../test/cssRules";
import { localPath } from "../test/localPath";
import { speciesSearchClass } from "../test/speciesSearchClasses";

const cssPath = localPath("../screens/SpeciesSearchField.css", import.meta.url);
const componentPath = localPath("../screens/SpeciesSearchField.tsx", import.meta.url);

function readCss(): string {
  if (!existsSync(cssPath)) {
    throw new Error("screens/SpeciesSearchField.css が無い(検索欄のスタイルが未実装)");
  }
  return readFileSync(cssPath, "utf8");
}

/** 要素に当たる規則(@media などの中も含む。@keyframes の枠は除く)。 */
function flattenRules(nodes: readonly CssNode[]): CssRule[] {
  return nodes.flatMap((node) =>
    node.kind === "rule"
      ? [node]
      : /^@(-webkit-)?keyframes\b/.test(node.prelude)
        ? []
        : flattenRules(node.children),
  );
}

function cssRules(): CssRule[] {
  return flattenRules(parseCss(readCss()));
}

/** セレクタが含むクラス名(`.species-search__option--active` → `species-search__option--active`)。 */
function classesIn(selector: string): string[] {
  return [...selector.matchAll(/\.([A-Za-z0-9_-]+)/g)].flatMap((match) =>
    match[1] === undefined ? [] : [match[1]],
  );
}

/** そのクラスを含むセレクタの規則(複数可)。 */
function rulesForClass(className: string): CssRule[] {
  return cssRules().filter((rule) => classesIn(rule.selector).includes(className));
}

/** そのクラスの規則の宣言をまとめたもの(後勝ち)。疑似クラス付きの規則も含む。 */
function declarationsForClass(className: string): Map<string, string> {
  return declarationMap(rulesForClass(className));
}

/** コメントを外した本体コード(クラス名の抽出に使う)。 */
function componentSourceWithoutComments(): string {
  return readFileSync(componentPath, "utf8")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");
}

describe("CSS ファイルがあり、コンポーネントが読み込む", () => {
  test("screens/SpeciesSearchField.css がある", () => {
    expect(existsSync(cssPath)).toBe(true);
  });

  test("SpeciesSearchField.tsx が自分の CSS を import する(CalcScreen.tsx と同じ作法)", () => {
    expect(componentSourceWithoutComments()).toContain('import "./SpeciesSearchField.css";');
  });
});

describe("本体コードが使うクラスに、すべて規則がある(素の箇条書きのまま出さない)", () => {
  test("species-search* のクラスは CSS に規則がある", () => {
    const used = new Set(componentSourceWithoutComments().match(/\bspecies-search[A-Za-z0-9_-]*/g) ?? []);
    expect(used.size, "本体コードに species-search* のクラスが無い(走査の前提の確認)").toBeGreaterThan(0);
    const styled = new Set(cssRules().flatMap((rule) => classesIn(rule.selector)));
    expect([...used].filter((className) => !styled.has(className))).toEqual([]);
  });

  test("受け入れ条件が定めるクラスがそろっている", () => {
    const styled = new Set(cssRules().flatMap((rule) => classesIn(rule.selector)));
    expect(Object.values(speciesSearchClass).filter((className) => !styled.has(className))).toEqual([]);
  });
});

describe("候補一覧は箇条書きの既定を消し、スクロールできる(上限 50 件出ることがある)", () => {
  test("list-style / margin / padding を消す", () => {
    const declarations = declarationsForClass(speciesSearchClass.list);
    expect(declarations.get("list-style")).toBe("none");
    expect(declarations.get("margin")).toBe("0");
    expect(declarations.get("padding")).toBe("0");
  });

  test("最大の高さを決めて overflow-y: auto にする", () => {
    const declarations = declarationsForClass(speciesSearchClass.list);
    expect(declarations.get("max-height") ?? "").not.toBe("");
    expect(declarations.get("overflow-y")).toBe("auto");
  });
});

describe("色・形・余白は docs/design.md のトークンで書く", () => {
  test("CSS に 16 進の色リテラルが無い(色は tokens.css の変数だけ)", () => {
    const offenders = readCss()
      .split("\n")
      .filter((line) => /#[0-9a-fA-F]{3,8}\b/.test(line));
    expect(offenders).toEqual([]);
  });

  test("色の宣言はすべて var(--*) を参照する", () => {
    const colorProperties = ["color", "background", "background-color", "border", "border-color"];
    const offenders = cssRules().flatMap((rule) =>
      rule.declarations
        .filter((declaration) => colorProperties.includes(declaration.property))
        .filter((declaration) => !declaration.value.includes("var(--"))
        .filter((declaration) => !["none", "transparent", "inherit"].includes(declaration.value))
        .map((declaration) => `${rule.selector} { ${declaration.property}: ${declaration.value} }`),
    );
    expect(offenders).toEqual([]);
  });

  test("入力欄の角丸は --radius-input(design.md「形・余白」: 入力 12)", () => {
    const inputRules = cssRules().filter((rule) => /\binput\b/.test(rule.selector));
    expect(inputRules.length, "入力欄を指す規則が無い").toBeGreaterThan(0);
    expect(declarationMap(inputRules).get("border-radius")).toBe("var(--radius-input)");
  });

  test("余白は --space-*(4 の倍数)で書く", () => {
    const spacing = ["padding", "margin", "gap", "row-gap", "column-gap"];
    const offenders = cssRules().flatMap((rule) =>
      rule.declarations
        .filter((declaration) => spacing.includes(declaration.property))
        .filter((declaration) =>
          declaration.value
            .split(/\s+/)
            .some((part) => !["0", "auto"].includes(part) && !part.startsWith("var(--space-")),
        )
        .map((declaration) => `${rule.selector} { ${declaration.property}: ${declaration.value} }`),
    );
    expect(offenders).toEqual([]);
  });

  test("補足・状態の案内は補足の文字サイズと text.secondary(CalcScreen の notice と同じ見え方)", () => {
    for (const className of [speciesSearchClass.hint, speciesSearchClass.status]) {
      const declarations = declarationsForClass(className);
      expect(declarations.get("color"), className).toBe("var(--text-secondary)");
      expect(declarations.get("font-size"), className).toBe("var(--font-size-caption)");
    }
  });
});

describe("ハイライト中とホバーは見た目で区別できる", () => {
  test("ハイライト中の候補(--active)に背景の指定がある", () => {
    const declarations = declarationsForClass(speciesSearchClass.activeOption);
    const background = declarations.get("background") ?? declarations.get("background-color") ?? "";
    expect(background).not.toBe("");
  });

  test(":hover の規則があり、ハイライト中とは別の規則になっている", () => {
    const hoverRules = rulesForClass(speciesSearchClass.option).filter((rule) =>
      rule.selector.includes(":hover"),
    );
    expect(hoverRules.length, "候補のホバーの規則が無い").toBeGreaterThan(0);
    expect(
      hoverRules.every((rule) => !classesIn(rule.selector).includes(speciesSearchClass.activeOption)),
    ).toBe(true);
  });

  test("キーボードのフォーカス位置が見える(:focus-visible の outline。WCAG 2.4.7)", () => {
    const focusRules = cssRules().filter((rule) => rule.selector.includes(":focus-visible"));
    expect(focusRules.length, ":focus-visible の規則が無い").toBeGreaterThan(0);
    const declarations = declarationMap(focusRules);
    expect(declarations.get("outline") ?? "").not.toBe("");
  });
});
