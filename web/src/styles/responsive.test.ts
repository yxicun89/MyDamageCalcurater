// P4-21(issue #98): 狭い幅(モバイル)のレイアウトの静的検査。
// 正は docs/design.md「幅への対応(ブレークポイント)」。期待値は design.md を毎回読んで作り、
// CSS の写しを持たない(コーディング規約 §2 の独立な検証。tokens.test.ts と同じ作法)。
//
// 見ているのは「狭い幅では1列・広い幅では左右」「列の幅が中身の固有幅で広がらない」の2点。
// 実際に溢れないことの確認は E2E(web/e2e/mobile.spec.ts)が viewport 320/375/768 で行う。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import {
  type CssNode,
  type CssRule,
  declarationMap,
  parseCss,
  rulesInAtRules,
  splitSelectors,
  topLevelRules,
} from "../test/cssRules";
import { localPath } from "../test/localPath";
import { bulletLine, labeledNumber, level2SectionOf, readDesignDoc } from "../test/designDoc";

const designDoc = readDesignDoc();
const widthSection = level2SectionOf(designDoc, "幅への対応(ブレークポイント)");

/** design.md が定めるブレークポイント(px)。CSS 側の @media はこの値を使う。 */
const breakpointPx = labeledNumber(bulletLine(widthSection, "ブレークポイント:"), "ブレークポイント:");

const screens = {
  calc: { file: "screens/CalcScreen.css", cards: ".calc-screen__cards", card: ".calc-card" },
  reverse: { file: "screens/ReverseScreen.css", cards: ".reverse-screen__cards", card: ".reverse-card" },
} as const;

function readScreenCss(file: string): CssNode[] {
  return parseCss(readFileSync(localPath(`../${file}`, import.meta.url), "utf8"));
}

const css: Record<keyof typeof screens, CssNode[]> = {
  calc: readScreenCss(screens.calc.file),
  reverse: readScreenCss(screens.reverse.file),
};

/** 広い幅の目印の @media(design.md のブレークポイント以上)か。 */
function isWideMedia(prelude: string): boolean {
  const match = /^@media\b[^{]*\bmin-width\s*:\s*(\d+(?:\.\d+)?)px/.exec(prelude);
  return match?.[1] !== undefined && Number(match[1]) === breakpointPx;
}

/** @media の外(= 狭い幅の既定)の規則のうち、セレクタがちょうど `selector` を含むもの。 */
function narrowRules(nodes: readonly CssNode[], selector: string): CssRule[] {
  return topLevelRules(nodes).filter((rule) => splitSelectors(rule.selector).includes(selector));
}

/** 広い幅の @media の中の、セレクタがちょうど `selector` を含む規則。 */
function wideRules(nodes: readonly CssNode[], selector: string): CssRule[] {
  return rulesInAtRules(nodes, isWideMedia).filter((rule) =>
    splitSelectors(rule.selector).includes(selector),
  );
}

/** 規則群のうち、セレクタに `part` を含むものの宣言(後勝ち)。 */
function declaredIn(rules: readonly CssRule[], property: string): string {
  return declarationMap(rules).get(property) ?? "";
}

/** grid-template-columns の値を列ごとに分ける(minmax(0, 1fr) の中のカンマでは切らない)。 */
function splitTracks(value: string): string[] {
  const tracks: string[] = [];
  let depth = 0;
  let current = "";
  for (const char of value) {
    if (char === "(") {
      depth += 1;
    } else if (char === ")") {
      depth -= 1;
    }
    if (char === " " && depth === 0) {
      if (current.trim() !== "") {
        tracks.push(current.trim());
      }
      current = "";
      continue;
    }
    current += char;
  }
  if (current.trim() !== "") {
    tracks.push(current.trim());
  }
  return tracks;
}

/** 中身の固有幅で広がらない伸縮列(minmax(0, …))か。`1fr` は minmax(auto, 1fr) と同義なので不可。 */
function shrinkableTrack(track: string): boolean {
  return /^minmax\(\s*0(px)?\s*,/.test(track);
}

function columnsOf(rules: readonly CssRule[]): string[] {
  return splitTracks(declaredIn(rules, "grid-template-columns"));
}

describe("design.md「幅への対応(ブレークポイント)」を読めている", () => {
  test("ブレークポイントは 375 より大きく 720(カードの最大幅)以下の4の倍数", () => {
    // 実測(issue #98): 320・375 では左右に並べると溢れ、768 では溢れない。
    // 余白は4の倍数(design.md「形・余白」)なので、幅のしきい値も4の倍数にそろえる。
    expect(breakpointPx).toBeGreaterThan(375);
    expect(breakpointPx).toBeLessThanOrEqual(720);
    expect(breakpointPx % 4).toBe(0);
  });

  test("狭い幅の縦積みの順序が書いてある(計算: 攻撃側→攻守入れ替え→防御側、逆算: 自分側→相手側)", () => {
    const calcOrder = bulletLine(widthSection, "計算:");
    expect(calcOrder.indexOf("攻撃側")).toBeLessThan(calcOrder.indexOf("攻守入れ替え"));
    expect(calcOrder.indexOf("攻守入れ替え")).toBeLessThan(calcOrder.indexOf("防御側"));
    const reverseOrder = bulletLine(widthSection, "逆算:");
    expect(reverseOrder.indexOf("自分側")).toBeLessThan(reverseOrder.indexOf("相手側"));
  });
});

describe.each([["計算画面", "calc", 3] as const, ["逆算画面", "reverse", 2] as const])(
  "%s: カードの並び",
  (_label, key, wideColumnCount) => {
    const nodes = css[key];
    const { cards, card } = screens[key];

    test(`狭い幅(既定)の ${cards} は1列`, () => {
      const columns = columnsOf(narrowRules(nodes, cards));
      expect(columns.length, `${cards} の既定の列が1列でない: ${columns.join(" ")}`).toBe(1);
    });

    test(`@media (min-width: ${String(breakpointPx)}px) の中で ${cards} は${String(wideColumnCount)}列`, () => {
      const columns = columnsOf(wideRules(nodes, cards));
      expect(
        columns.length,
        `広い幅の @media(min-width: ${String(breakpointPx)}px)に ${cards} の${String(wideColumnCount)}列が無い`,
      ).toBe(wideColumnCount);
    });

    test("伸縮する列は minmax(0, …)(中身の固有幅で列を広げない)", () => {
      for (const rules of [narrowRules(nodes, cards), wideRules(nodes, cards)]) {
        for (const track of columnsOf(rules).filter((track) => track.includes("fr"))) {
          expect(
            shrinkableTrack(track),
            `列 "${track}" は 1fr = minmax(auto, 1fr) で中身より狭くならない`,
          ).toBe(true);
        }
      }
    });

    test(`${card} は min-width: 0(グリッドの列より狭くなれる)`, () => {
      expect(declaredIn(narrowRules(nodes, card), "min-width")).toBe("0");
    });

    test(`${card} の select はカードの幅まで縮む(width: 100% と min-width: 0)`, () => {
      const rules = narrowRules(nodes, `${card} select`);
      expect(declaredIn(rules, "width")).toBe("100%");
      expect(declaredIn(rules, "min-width")).toBe("0");
    });
  },
);

describe("計算画面: 攻守入れ替えボタン", () => {
  test("狭い幅では2枚のカードの間の行に中央寄せで置く(design.md)", () => {
    const rules = narrowRules(css.calc, ".calc-screen__swap");
    expect(declaredIn(rules, "justify-self")).toBe("center");
  });

  test("広い幅ではカードの間で縦中央にそろえる(従来どおり)", () => {
    const align =
      declaredIn(wideRules(css.calc, ".calc-screen__swap"), "align-self") ||
      declaredIn(narrowRules(css.calc, ".calc-screen__swap"), "align-self");
    expect(align).toBe("center");
  });
});

describe("逆算画面: 観測の行", () => {
  test(".reverse-observation の伸縮する列も minmax(0, …)", () => {
    const columns = columnsOf(narrowRules(css.reverse, ".reverse-observation"));
    expect(columns.length).toBeGreaterThan(0);
    for (const track of columns.filter((track) => track.includes("fr"))) {
      expect(shrinkableTrack(track), `列 "${track}" が中身より狭くならない`).toBe(true);
    }
  });

  test(".reverse-observation__input は列の幅まで縮む(width: 100% と min-width: 0)", () => {
    const rules = narrowRules(css.reverse, ".reverse-observation__input");
    expect(declaredIn(rules, "width")).toBe("100%");
    expect(declaredIn(rules, "min-width")).toBe("0");
  });
});

describe("視覚順を DOM 順から動かさない(WCAG 1.3.2。design.md「幅への対応」)", () => {
  const reorderingProperties = new Set(["order", "direction"]);
  const reorderingValues = /(row|column)-reverse|\bdense\b/;

  test.each([["計算画面", "calc"] as const, ["逆算画面", "reverse"] as const])(
    "%s の CSS に並び替えの指定が無い",
    (_label, key) => {
      const offenders: string[] = [];
      const visit = (nodes: readonly CssNode[]): void => {
        for (const node of nodes) {
          if (node.kind === "at-rule") {
            visit(node.children);
            continue;
          }
          for (const declaration of node.declarations) {
            if (reorderingProperties.has(declaration.property) || reorderingValues.test(declaration.value)) {
              offenders.push(`${node.selector} { ${declaration.property}: ${declaration.value} }`);
            }
          }
        }
      };
      visit(css[key]);
      expect(offenders).toEqual([]);
    },
  );
});
