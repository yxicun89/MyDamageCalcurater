// issue #306(アクセシビリティ): タイプ名はバッジ(背景 = タイプ色)で出し、その文字色が
// 背景のタイプ色に対して WCAG 2.2 SC 1.4.3(通常文字 4.5:1)を満たすことを検査する。
// 期待値は design.md「タイプバッジ」を毎回読んで作る(tokens.css / TS 側の固定値を書き写さない。コーディング規約 §2)。
//
// ここで確かめるのは「設計書に書いた文字色が本当に読める組み合わせか」であって、
// tokens.css に `--type-<id>-ink` が同じ値で入っているかは tokens.test.ts が見る。

import { describe, expect, test } from "vitest";
import { WCAG_NORMAL_TEXT_MIN_CONTRAST, contrastRatio } from "../test/colorContrast";
import {
  baseTokens,
  readDesignDoc,
  typeBadgeInkByJapaneseName,
  typeColorsByJapaneseName,
} from "../test/designDoc";

const designDoc = readDesignDoc();
const typeColors = typeColorsByJapaneseName(designDoc);
const badgeInks = typeBadgeInkByJapaneseName(designDoc);

/** design.md「タイプバッジ」が文字色の候補に挙げている2色(黒・白)。 */
const INK_CANDIDATES = ["#000000", "#FFFFFF"] as const;

function typeColorOf(name: string): string {
  const color = typeColors.get(name);
  if (color === undefined) {
    throw new Error(`design.md「タイプ色(自作パレット)」に ${name} が無い`);
  }
  return color;
}

function textPrimary(scheme: "light" | "dark"): string {
  const row = baseTokens(designDoc).find((candidate) => candidate.token === "text.primary");
  if (row === undefined) {
    throw new Error("design.md の「ベース」表に text.primary が無い");
  }
  return row[scheme].css;
}

const entries = [...badgeInks.entries()];

describe("タイプバッジの文字色(design.md「タイプバッジ」)", () => {
  test("タイプ色と同じ18タイプに文字色が決まっている", () => {
    expect([...badgeInks.keys()].sort()).toEqual([...typeColors.keys()].sort());
    expect(badgeInks.size).toBe(18);
  });

  test.each(entries)("%s の文字色 %s は黒か白のどちらか", (_name, ink) => {
    expect(INK_CANDIDATES).toContain(ink.toUpperCase());
  });

  test.each(entries)("%s: 文字色 %s は背景のタイプ色に対して 4.5:1 以上", (name, ink) => {
    expect(contrastRatio(ink, typeColorOf(name))).toBeGreaterThanOrEqual(WCAG_NORMAL_TEXT_MIN_CONTRAST);
  });

  test.each(entries)("%s: 文字色 %s は黒・白のうちコントラスト比が高い方", (name, ink) => {
    const typeColor = typeColorOf(name);
    const best = INK_CANDIDATES.reduce((chosen, candidate) =>
      contrastRatio(candidate, typeColor) > contrastRatio(chosen, typeColor) ? candidate : chosen,
    );
    expect(ink.toUpperCase()).toBe(best);
  });

  // design.md に書いた「text.primary を使わない」理由を固定する。
  // 黒系・白系を選び分けるだけでは足りない(= 純黒・純白が要る)ことを、実際の色で示す。
  test("text.primary(ライト/ダーク)では 4.5:1 に届かないタイプがある", () => {
    const inks = [textPrimary("light"), textPrimary("dark")];
    const failing = [...typeColors.entries()]
      .filter(
        ([, color]) =>
          Math.max(...inks.map((ink) => contrastRatio(ink, color))) < WCAG_NORMAL_TEXT_MIN_CONTRAST,
      )
      .map(([name]) => name);
    expect(failing).toEqual(["かくとう", "どく"]);
  });
});
