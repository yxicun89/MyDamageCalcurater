// P4-8: 演出の CSS の静的検査(docs/design.md「動き」、CLAUDE.md「常時動くアニメーションを入れない。演出は操作時のみ」)。
// 動きは JS が操作のときだけ付ける状態クラス(.is-*)にだけ置き、ずっと繰り返すアニメーションは置かない。
// ダメージバーは spring(行き過ぎて戻る cubic-bezier)で伸び縮みする。イージングは tokens.css の --ease-spring に置く。

import { readFileSync, readdirSync } from "node:fs";
import { describe, expect, test } from "vitest";
import {
  type CssNode,
  type CssRule,
  declarationMap,
  parseCss,
  splitSelectors,
  topLevelRules,
} from "../test/cssRules";
import { localPath } from "../test/localPath";

const srcDir = localPath("../", import.meta.url);

function cssFiles(): string[] {
  return readdirSync(srcDir, { recursive: true, encoding: "utf8" })
    .map((path) => path.replace(/\\/g, "/"))
    .filter((path) => path.endsWith(".css"));
}

function productionScripts(): string[] {
  return readdirSync(srcDir, { recursive: true, encoding: "utf8" })
    .map((path) => path.replace(/\\/g, "/"))
    .filter((path) => /\.(ts|tsx)$/.test(path))
    .filter((path) => !/\.test\.(ts|tsx)$/.test(path))
    .filter((path) => !path.startsWith("test/"));
}

function readSource(path: string): string {
  return readFileSync(`${srcDir}${path}`, "utf8");
}

interface LocatedRule {
  readonly file: string;
  readonly rule: CssRule;
  /** 外側の @規則の prelude(外から順)。 */
  readonly atRules: readonly string[];
}

function flattenRules(
  file: string,
  nodes: readonly CssNode[],
  atRules: readonly string[] = [],
): LocatedRule[] {
  return nodes.flatMap((node) =>
    node.kind === "rule"
      ? [{ file, rule: node, atRules }]
      : flattenRules(file, node.children, [...atRules, node.prelude]),
  );
}

const allRules: LocatedRule[] = cssFiles().flatMap((file) => flattenRules(file, parseCss(readSource(file))));

const isKeyframes = (prelude: string): boolean => /^@(-webkit-)?keyframes\b/.test(prelude);
const isReducedMotion = (prelude: string): boolean => /prefers-reduced-motion\s*:\s*reduce/.test(prelude);

/** @keyframes の中の枠(from/to/%)と、reduced-motion の無効化規則を除いた、要素に当たる規則。 */
const elementRules: LocatedRule[] = allRules.filter(
  ({ atRules }) => !atRules.some((prelude) => isKeyframes(prelude) || isReducedMotion(prelude)),
);

function rulesFor(file: string, selectorPart: string): CssRule[] {
  return elementRules
    .filter((located) => located.file === file && located.rule.selector.includes(selectorPart))
    .map((located) => located.rule);
}

function declared(rules: readonly CssRule[], property: string): string {
  return declarationMap(rules).get(property) ?? "";
}

describe("常時動くものを置かない", () => {
  test("走査対象に計算画面・逆算画面・トークンの CSS が含まれる(走査の前提の確認)", () => {
    expect(cssFiles()).toEqual(
      expect.arrayContaining(["screens/CalcScreen.css", "screens/ReverseScreen.css", "styles/tokens.css"]),
    );
  });

  test("animation / animation-iteration-count に infinite が無い", () => {
    const offenders = allRules.flatMap(({ file, rule }) =>
      rule.declarations
        .filter((declaration) => declaration.property.startsWith("animation"))
        .filter((declaration) => /\binfinite\b/i.test(declaration.value))
        .map((declaration) => `${file}: ${rule.selector} { ${declaration.property}: ${declaration.value} }`),
    );
    expect(offenders).toEqual([]);
  });

  test("animation は操作のときだけ JS が付ける状態クラス(.is-*)の規則にだけ置く", () => {
    const offenders = elementRules.flatMap(({ file, rule }) =>
      rule.declarations
        .filter(
          (declaration) => declaration.property === "animation" || declaration.property === "animation-name",
        )
        .filter((declaration) => declaration.value !== "none")
        .flatMap(() =>
          splitSelectors(rule.selector)
            .filter((selector) => !/\.is-[a-z]/.test(selector))
            .map((selector) => `${file}: ${selector}`),
        ),
    );
    expect(offenders).toEqual([]);
  });

  test("animation で参照する @keyframes はすべて定義されている", () => {
    const defined = new Set(
      cssFiles().flatMap((file) =>
        parseCss(readSource(file)).flatMap((node) =>
          node.kind === "at-rule" && isKeyframes(node.prelude) ? [node.prelude.split(/\s+/)[1] ?? ""] : [],
        ),
      ),
    );
    const referenced = elementRules.flatMap(({ rule }) =>
      rule.declarations
        .filter(
          (declaration) => declaration.property === "animation" || declaration.property === "animation-name",
        )
        .flatMap((declaration) => declaration.value.split(","))
        .flatMap((animation) => animation.trim().split(/\s+/))
        .filter((token) => /^[a-z][a-z0-9-]*$/i.test(token) && !KEYWORDS.has(token)),
    );
    expect(referenced.length, "animation を使う規則が1つも無い(P4-8 の演出が CSS に無い)").toBeGreaterThan(0);
    expect(referenced.filter((name) => !defined.has(name))).toEqual([]);
  });

  test("本体コードで requestAnimationFrame を使うなら cancelAnimationFrame で止める(回しっぱなしにしない)", () => {
    const offenders = productionScripts().filter((path) => {
      const source = readSource(path);
      return source.includes("requestAnimationFrame") && !source.includes("cancelAnimationFrame");
    });
    expect(offenders).toEqual([]);
  });

  test("本体コードに setInterval が無い(時間で動き続ける演出を置かない)", () => {
    expect(productionScripts().filter((path) => /\bsetInterval\s*\(/.test(readSource(path)))).toEqual([]);
  });
});

/** animation の短縮記法に出るキーワード(@keyframes 名と区別するため)。 */
const KEYWORDS = new Set([
  "none",
  "ease",
  "ease-in",
  "ease-out",
  "ease-in-out",
  "linear",
  "step-start",
  "step-end",
  "normal",
  "reverse",
  "alternate",
  "alternate-reverse",
  "forwards",
  "backwards",
  "both",
  "running",
  "paused",
  "var",
  "cubic-bezier",
  "steps",
]);

describe("ダメージバーは spring で伸び縮みする", () => {
  const rootRules = topLevelRules(parseCss(readSource("styles/tokens.css"))).filter((rule) =>
    splitSelectors(rule.selector).includes(":root"),
  );
  const easeSpring = declarationMap(rootRules).get("--ease-spring") ?? "";

  test("tokens.css の :root に --ease-spring があり、行き過ぎて戻る cubic-bezier(y が 0〜1 をはみ出す)", () => {
    const match = /^cubic-bezier\(\s*([-\d.]+)\s*,\s*([-\d.]+)\s*,\s*([-\d.]+)\s*,\s*([-\d.]+)\s*\)$/.exec(
      easeSpring,
    );
    expect(match, `--ease-spring が cubic-bezier() でない: "${easeSpring}"`).not.toBeNull();
    const [x1, y1, x2, y2] = (match ?? []).slice(1).map(Number);
    for (const x of [x1, x2]) {
      expect(x).toBeGreaterThanOrEqual(0);
      expect(x).toBeLessThanOrEqual(1);
    }
    expect([y1, y2].some((y) => y !== undefined && (y > 1 || y < 0))).toBe(true);
  });

  test("ダメージバー(.calc-results__bar-fill)の width の transition は var(--ease-spring) を使う", () => {
    const transition = declared(rulesFor("screens/CalcScreen.css", ".calc-results__bar-fill"), "transition");
    expect(transition).toMatch(/\bwidth\b/);
    expect(transition).toContain("var(--ease-spring)");
  });
});

describe("演出ごとの状態クラスの見た目", () => {
  test("確定数バッジの弾み: .calc-results__ko.is-pulsing に animation がある", () => {
    expect(declared(rulesFor("screens/CalcScreen.css", ".is-pulsing"), "animation")).not.toBe("");
  });

  test("攻守入れ替え: .calc-card.is-swapping の animation は --duration-swap(0.35秒)を使う", () => {
    expect(declared(rulesFor("screens/CalcScreen.css", ".is-swapping"), "animation")).toContain(
      "var(--duration-swap)",
    );
  });

  test("ホロ: .calc-card.is-holo が --holo-x / --holo-y を参照する", () => {
    const holo = rulesFor("screens/CalcScreen.css", ".is-holo")
      .flatMap((rule) => rule.declarations.map((declaration) => declaration.value))
      .join(" ");
    expect(holo).toContain("var(--holo-x");
    expect(holo).toContain("var(--holo-y");
  });

  test("逆算の絞り込み: .reverse-results__list.is-narrowing に animation がある", () => {
    expect(declared(rulesFor("screens/ReverseScreen.css", ".is-narrowing"), "animation")).not.toBe("");
  });
});
