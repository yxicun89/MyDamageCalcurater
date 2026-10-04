// F-12(I-web-6、ADR-0331 §2): 共通の見た目の部品(styles/components.css の .ui-* クラス)の静的検査。
// 画面ごとの CSS に同じ見た目を何度も書かず、カード・ボタン・チップ・表・タブ・入力・案内を共通のクラスに寄せる。
// 色・角丸・影・時間は tokens.css の変数だけを参照する(色の直書きの禁止は tokens.test.ts が全 CSS で見る)。
// 部品の一覧は docs/design.md「共通の部品」にも書く(この検査と同期する)。

import { existsSync, readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { type CssNode, type CssRule, declarationMap, parseCss, splitSelectors } from "../test/cssRules";
import { readDesignDoc, sectionOf } from "../test/designDoc";
import { localPath } from "../test/localPath";

const componentsPath = localPath("./components.css", import.meta.url);

/** ADR-0331 §2 の部品のクラス(design.md「共通の部品」に全部書く)。 */
const REQUIRED_CLASSES = [
  ".ui-card",
  ".ui-card--typed",
  ".ui-card__band",
  ".ui-section",
  ".ui-heading",
  ".ui-button",
  ".ui-button--primary",
  ".ui-button--secondary",
  ".ui-button--danger",
  ".ui-chip",
  ".ui-chip--selected",
  ".ui-badge",
  ".ui-table",
  ".ui-rows",
  ".ui-tabs",
  ".ui-tab",
  ".ui-field",
  ".ui-notice",
  ".ui-notice--empty",
  ".ui-notice--error",
  ".ui-notice--loading",
  ".ui-notice--info",
  ".ui-icon",
] as const;

interface LocatedRule {
  readonly rule: CssRule;
  readonly atRules: readonly string[];
}

function flatten(nodes: readonly CssNode[], atRules: readonly string[] = []): LocatedRule[] {
  return nodes.flatMap((node) =>
    node.kind === "rule" ? [{ rule: node, atRules }] : flatten(node.children, [...atRules, node.prelude]),
  );
}

function readComponents(): LocatedRule[] {
  if (!existsSync(componentsPath)) {
    throw new Error("web/src/styles/components.css が無い(ADR-0331 §2)");
  }
  return flatten(parseCss(readFileSync(componentsPath, "utf8"))).filter(
    ({ atRules }) => !atRules.some((prelude) => /prefers-reduced-motion/.test(prelude)),
  );
}

/** セレクタ(カンマで分けた1つ)がちょうど `selector` の規則の宣言(後勝ち)。 */
function declarationsFor(selector: string): Map<string, string> {
  const normalize = (text: string): string => text.replace(/'/g, '"').replace(/\s+/g, " ").trim();
  return declarationMap(
    readComponents()
      .map(({ rule }) => rule)
      .filter((rule) => splitSelectors(rule.selector).map(normalize).includes(normalize(selector))),
  );
}

/** セレクタに `part` を含む規則の宣言の値(全部を空白でつないだもの)。 */
function valuesContaining(part: string): string {
  return readComponents()
    .filter(({ rule }) => splitSelectors(rule.selector).some((selector) => selector.includes(part)))
    .flatMap(({ rule }) =>
      rule.declarations.map((declaration) => `${declaration.property}: ${declaration.value}`),
    )
    .join("; ");
}

describe("入口", () => {
  test("main.tsx が tokens.css の次に styles/components.css を読み込む", () => {
    const main = readFileSync(localPath("../main.tsx", import.meta.url), "utf8");
    const tokens = main.search(/^import\s+["']\.\/styles\/tokens\.css["'];?\s*$/m);
    const components = main.search(/^import\s+["']\.\/styles\/components\.css["'];?\s*$/m);
    expect(components, "main.tsx が ./styles/components.css を import していない").toBeGreaterThanOrEqual(0);
    expect(components).toBeGreaterThan(tokens);
  });
});

describe("部品のクラスがそろっている", () => {
  test.each(REQUIRED_CLASSES)("%s の規則がある", (className) => {
    const found = readComponents().some(({ rule }) =>
      splitSelectors(rule.selector).some((selector) =>
        new RegExp(`${className.replace(/[.]/g, "\\.")}(?![\\w-])`).test(selector),
      ),
    );
    expect(found).toBe(true);
  });

  test.each(REQUIRED_CLASSES)("%s は design.md「共通の部品」に書いてある", (className) => {
    expect(sectionOf(readDesignDoc(), "共通の部品")).toContain(className);
  });
});

describe("カード", () => {
  test(".ui-card: カードの角丸・不透明な面・影", () => {
    const card = declarationsFor(".ui-card");
    expect(card.get("border-radius")).toBe("var(--radius-card)");
    expect(card.get("background") ?? card.get("background-color")).toBe("var(--surface-card)");
    expect(card.get("box-shadow")).toBe("var(--shadow-card)");
  });

  test(".ui-card--typed: ふちと上端の帯がタイプ色(--card-type)。未設定でもブランド色で成立する", () => {
    const typed = valuesContaining(".ui-card--typed");
    expect(typed).toContain("var(--card-type, var(--brand-primary))");
    expect(typed).toMatch(/border(-top|-color)?\s*:/);
  });

  test(".ui-card__band: 帯の塗りもタイプ色(--card-type)", () => {
    expect(valuesContaining(".ui-card__band")).toContain("var(--card-type, var(--brand-primary))");
  });
});

describe("ボタン", () => {
  test(".ui-button: 丸い(ピル)・太めの文字・押せる見た目", () => {
    const button = declarationsFor(".ui-button");
    expect(button.get("border-radius")).toBe("var(--radius-pill)");
    expect(button.get("font-weight")).toBe("var(--font-weight-strong)");
    expect(button.get("cursor")).toBe("pointer");
  });

  test.each([
    [".ui-button--primary", "var(--brand-primary)", "var(--on-primary)"],
    [".ui-button--secondary", "var(--surface-card)", "var(--brand-primary)"],
    [".ui-button--danger", "var(--danger)", "var(--on-danger)"],
  ])("%s の塗りと文字色", (selector, background, color) => {
    const declarations = declarationsFor(selector);
    expect(declarations.get("background") ?? declarations.get("background-color")).toBe(background);
    expect(declarations.get("color")).toBe(color);
  });

  test("押下の演出は :active の transform だけで、時間は var(--duration-press)", () => {
    expect(declarationsFor(".ui-button:active").get("transform")).toMatch(/^scale\(0\.\d+\)$/);
    expect(declarationsFor(".ui-button").get("transition") ?? "").toContain("var(--duration-press)");
  });

  test("押せないボタン(:disabled)は押下の演出をしない見た目になる", () => {
    const disabled = declarationsFor(".ui-button:disabled");
    expect(disabled.get("cursor")).toBe("not-allowed");
  });
});

describe("チップ・バッジ・タブ", () => {
  test(".ui-chip はピル。選択中(.ui-chip--selected)は主色の塗りに on.primary の文字", () => {
    expect(declarationsFor(".ui-chip").get("border-radius")).toBe("var(--radius-pill)");
    const selected = declarationsFor(".ui-chip--selected");
    expect(selected.get("background") ?? selected.get("background-color")).toBe("var(--brand-primary)");
    expect(selected.get("color")).toBe("var(--on-primary)");
  });

  test(".ui-badge はピルで太字", () => {
    const badge = declarationsFor(".ui-badge");
    expect(badge.get("border-radius")).toBe("var(--radius-pill)");
    expect(badge.get("font-weight")).toBe("var(--font-weight-strong)");
  });

  test('.ui-tab はピル。選択中(.ui-tab[aria-selected="true"])は主色の塗りに on.primary の文字', () => {
    expect(declarationsFor(".ui-tab").get("border-radius")).toBe("var(--radius-pill)");
    const selected = declarationsFor('.ui-tab[aria-selected="true"]');
    expect(selected.get("background") ?? selected.get("background-color")).toBe("var(--brand-primary)");
    expect(selected.get("color")).toBe("var(--on-primary)");
  });

  test(".ui-tab のアイコンと文字は横に並ぶ(inline-flex + gap)", () => {
    const tab = declarationsFor(".ui-tab");
    expect(tab.get("display")).toBe("inline-flex");
    expect(tab.get("gap")).toMatch(/^var\(--space-\d\)$/);
  });
});

describe("表", () => {
  test(".ui-table: 見出し行の色・ゼブラ・行ホバー・角丸", () => {
    expect(valuesContaining(".ui-table th")).toContain("var(--table-header)");
    expect(valuesContaining(".ui-table tbody tr:nth-child(even)")).toContain("var(--table-zebra)");
    expect(valuesContaining(".ui-table tbody tr:hover")).toContain("var(--table-hover)");
    expect(declarationsFor(".ui-table").get("border-radius")).toBe("var(--radius-input)");
  });

  test(".ui-rows(一覧を表のように見せる ul/ol): ゼブラ・行ホバー・角丸", () => {
    expect(valuesContaining(".ui-rows > li:nth-child(even)")).toContain("var(--table-zebra)");
    expect(valuesContaining(".ui-rows > li:hover")).toContain("var(--table-hover)");
    expect(declarationsFor(".ui-rows").get("border-radius")).toBe("var(--radius-input)");
  });
});

describe("フォーカス・入力", () => {
  test.each([".ui-button", ".ui-tab", ".ui-chip", ".ui-field"])(
    "%s は :focus-visible で var(--focus-ring) の輪を出す",
    (className) => {
      const focus = valuesContaining(className);
      const rules = readComponents().filter(({ rule }) =>
        splitSelectors(rule.selector).some(
          (selector) => selector.includes(className) && selector.includes(":focus-visible"),
        ),
      );
      expect(rules.length, `${className} の :focus-visible の規則が無い`).toBeGreaterThan(0);
      expect(focus).toContain("var(--focus-ring)");
    },
  );

  test(".ui-field の input・select・textarea は入力の角丸", () => {
    const field = valuesContaining(".ui-field");
    expect(field).toContain("var(--radius-input)");
    for (const element of ["input", "select", "textarea"]) {
      expect(
        readComponents().some(({ rule }) =>
          splitSelectors(rule.selector).some(
            (selector) => selector.includes(".ui-field") && selector.includes(element),
          ),
        ),
        `.ui-field ${element} の規則が無い`,
      ).toBe(true);
    }
  });
});

describe("案内(空・エラー・読み込み・情報)", () => {
  test.each([
    [".ui-notice--empty", "var(--surface-card)", "var(--text-secondary)"],
    [".ui-notice--error", "var(--danger-soft)", "var(--danger)"],
    [".ui-notice--loading", "var(--info-soft)", "var(--info)"],
    [".ui-notice--info", "var(--info-soft)", "var(--info)"],
  ])("%s の塗りと文字色(組み合わせは popPalette.test.ts の AA の表にある)", (selector, background, color) => {
    const declarations = declarationsFor(selector);
    expect(declarations.get("background") ?? declarations.get("background-color")).toBe(background);
    expect(declarations.get("color")).toBe(color);
  });

  test(".ui-notice は角丸の囲み", () => {
    expect(declarationsFor(".ui-notice").get("border-radius")).toBe("var(--radius-input)");
  });
});

describe("アイコン", () => {
  test(".ui-icon は文字色に従い(currentColor)、行の中で縮まない", () => {
    const icon = declarationsFor(".ui-icon");
    expect(icon.get("flex-shrink")).toBe("0");
    expect(icon.get("color") ?? "currentColor").toBe("currentColor");
  });
});

describe("動き(操作のときだけ)", () => {
  test("components.css に animation を置かない(演出は transition の押下・選択だけ)", () => {
    const animated = readComponents().flatMap(({ rule }) =>
      rule.declarations
        .filter((declaration) => declaration.property.startsWith("animation"))
        .map((declaration) => `${rule.selector} { ${declaration.property}: ${declaration.value} }`),
    );
    expect(animated).toEqual([]);
  });

  test("transition の時間は var(--duration-press) だけ(秒数の直書きをしない。reduced-motion で 0 になる)", () => {
    const offenders = readComponents().flatMap(({ rule }) =>
      rule.declarations
        .filter((declaration) => declaration.property.startsWith("transition"))
        .filter((declaration) => /\d+(\.\d+)?m?s\b/.test(declaration.value))
        .map((declaration) => `${rule.selector} { ${declaration.property}: ${declaration.value} }`),
    );
    expect(offenders).toEqual([]);
  });

  test(":hover の見た目は @media (hover: hover) の中にだけ置く(タッチで押したあと色が残らない)", () => {
    const offenders = readComponents()
      .filter(({ rule }) => splitSelectors(rule.selector).some((selector) => selector.includes(":hover")))
      .filter(({ atRules }) => !atRules.some((prelude) => /\(hover:\s*hover\)/.test(prelude)))
      .map(({ rule }) => rule.selector);
    expect(offenders).toEqual([]);
  });
});
