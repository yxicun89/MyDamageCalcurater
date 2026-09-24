// issue #99(アクセシビリティ): danger トークンが WCAG 2.2 SC 1.4.3(通常文字 4.5:1)を、
// ライト・ダークどちらの背景(bg.base 単体・bg.glass を bg.base に重ねた見た目)に対しても満たすことを検査する。
// 期待値は design.md を毎回読んで作る(tokens.css 側の固定値を書き写さない。コーディング規約 §2)。

import { describe, expect, test } from "vitest";
import { WCAG_NORMAL_TEXT_MIN_CONTRAST, compositeOver, contrastRatio } from "../test/colorContrast";
import { baseTokens, readDesignDoc } from "../test/designDoc";

const designDoc = readDesignDoc();
const tokens = baseTokens(designDoc);

function tokenCss(token: string): { readonly light: string; readonly dark: string } {
  const row = tokens.find((candidate) => candidate.token === token);
  if (row === undefined) {
    throw new Error(`design.md の「ベース」表に ${token} が無い`);
  }
  return { light: row.light.css, dark: row.dark.css };
}

describe.each([
  ["ライト", "light"],
  ["ダーク", "dark"],
] as const)(
  "%s テーマの danger は通常文字のコントラスト基準を満たす(WCAG 2.2 SC 1.4.3)",
  (_label, scheme) => {
    const danger = tokenCss("danger")[scheme];
    const bgBase = tokenCss("bg.base")[scheme];
    // bg.glass は半透明(白/黒 + ぼかし)なので、実際に文字が乗る見た目の背景は bg.base に重ねた合成色になる。
    const bgGlass = compositeOver(tokenCss("bg.glass")[scheme], bgBase);

    test("bg.base に対して4.5:1以上", () => {
      expect(contrastRatio(danger, bgBase)).toBeGreaterThanOrEqual(WCAG_NORMAL_TEXT_MIN_CONTRAST);
    });

    test("bg.glass を bg.base に重ねた背景に対して4.5:1以上", () => {
      expect(contrastRatio(danger, bgGlass)).toBeGreaterThanOrEqual(WCAG_NORMAL_TEXT_MIN_CONTRAST);
    });
  },
);
