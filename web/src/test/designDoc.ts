// テスト専用: docs/design.md(デザイントークンの正)を読み、CSS 変数と照合できる形にする。
// 実装(tokens.css)の写しを持たず、設計書そのものから期待値を作る(コーディング規約 §2 の独立な検証)。

import { readFileSync } from "node:fs";
import { localPath } from "./localPath";

export const designDocPath = localPath("../../../docs/design.md", import.meta.url);

export function readDesignDoc(): string {
  return readFileSync(designDocPath, "utf8");
}

/** `### 見出し` から次の見出しまでの本文。見つからなければ例外。 */
export function sectionOf(markdown: string, heading: string): string {
  const lines = markdown.split("\n");
  const start = lines.findIndex((line) => line.trim() === `### ${heading}`);
  if (start < 0) {
    throw new Error(`design.md に「### ${heading}」が無い`);
  }
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => /^#{1,3} /.test(line));
  return (end < 0 ? rest : rest.slice(0, end)).join("\n");
}

/** Markdown の表の本文行(見出し行と区切り行を除く)をセルの配列で返す。 */
export function tableRows(section: string): string[][] {
  const rows = section
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.startsWith("|"))
    .map((line) =>
      line
        .replace(/^\|/, "")
        .replace(/\|$/, "")
        .split("|")
        .map((cell) => cell.trim()),
    );
  return rows.slice(1).filter((cells) => !cells.every((cell) => /^:?-+:?$/.test(cell)));
}

/** 設計書の色の書き方を CSS の色に変換した期待値。ぼかし付きなら blur が true。 */
export interface DesignColor {
  readonly css: string;
  readonly blur: boolean;
}

const namedBaseColors: Readonly<Record<string, string>> = {
  白: "255, 255, 255",
  黒: "0, 0, 0",
};

/**
 * design.md の色表記を CSS の色に変換する。
 * - `#RRGGBB` → そのまま
 * - `白 70%` → rgba(255, 255, 255, 0.7) / `黒 8%` → rgba(0, 0, 0, 0.08)
 * - `#1A1D24 60%` → rgba(26, 29, 36, 0.6)
 * - 末尾の `+ ぼかし` は blur: true(背景のぼかしを別変数 `<変数名>-blur` で持つ)
 */
export function designColorToCss(notation: string): DesignColor {
  const blur = /\+\s*ぼかし/.test(notation);
  const text = notation.replace(/\+\s*ぼかし/, "").trim();
  const plainHex = /^#[0-9A-Fa-f]{6}$/.exec(text);
  if (plainHex) {
    return { css: text, blur };
  }
  const withAlpha = /^(\S+)\s+(\d+(?:\.\d+)?)%$/.exec(text);
  if (withAlpha?.[1] !== undefined && withAlpha[2] !== undefined) {
    const base = withAlpha[1];
    const alpha = parseFloat(withAlpha[2]) / 100;
    const named = namedBaseColors[base];
    if (named !== undefined) {
      return { css: `rgba(${named}, ${alpha})`, blur };
    }
    if (/^#[0-9A-Fa-f]{6}$/.test(base)) {
      const channels = [1, 3, 5].map((offset) => parseInt(base.slice(offset, offset + 2), 16));
      return { css: `rgba(${channels.join(", ")}, ${alpha})`, blur };
    }
  }
  throw new Error(`design.md の色表記を解釈できない: ${notation}`);
}

/** トークン名 `bg.base` → CSS 変数名 `--bg-base`。 */
export function tokenToCssVariable(token: string): string {
  return `--${token.replace(/\./g, "-")}`;
}

export interface BaseToken {
  readonly token: string;
  readonly light: DesignColor;
  readonly dark: DesignColor;
}

export function baseTokens(markdown: string): BaseToken[] {
  return tableRows(sectionOf(markdown, "ベース")).map((cells) => {
    const [token, light, dark] = cells;
    if (token === undefined || light === undefined || dark === undefined) {
      throw new Error(`ベースの表の行が3列でない: ${cells.join(" | ")}`);
    }
    return { token, light: designColorToCss(light), dark: designColorToCss(dark) };
  });
}

/** design.md「ベース」の bg.glass のぼかし量(px)。「blur(20px)」のような表記から数値を読む。 */
export function glassBlurPixels(markdown: string): number {
  const line = bulletLine(sectionOf(markdown, "ベース"), "bg.glass のぼかし:");
  const match = /blur\((\d+(?:\.\d+)?)px\)/.exec(line);
  if (match?.[1] === undefined) {
    throw new Error(`design.md の「${line}」に blur(...px) の記載が無い`);
  }
  return parseFloat(match[1]);
}

/** タイプ色の表(「タイプ | 色」の組が横に並ぶ)を 日本語名 → #RRGGBB にする。 */
export function typeColorsByJapaneseName(markdown: string): Map<string, string> {
  const colors = new Map<string, string>();
  for (const cells of tableRows(sectionOf(markdown, "タイプ色(自作パレット)"))) {
    for (let index = 0; index + 1 < cells.length; index += 2) {
      const name = cells[index];
      const color = cells[index + 1];
      if (name === undefined || color === undefined || name === "") {
        continue;
      }
      if (!/^#[0-9A-Fa-f]{6}$/.test(color)) {
        throw new Error(`タイプ色の表の色が #RRGGBB でない: ${name} ${color}`);
      }
      colors.set(name, color);
    }
  }
  return colors;
}

/** 「ラベル 数値 / ラベル 数値 …」の並びから、指定ラベルの数値を引く。 */
export function labeledNumber(line: string, label: string): number {
  const escaped = label.replace(/[.*+?^${}()|[\]\\%]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*(\\d+(?:\\.\\d+)?)`).exec(line);
  if (match?.[1] === undefined) {
    throw new Error(`design.md の「${line}」に「${label}」の数値が無い`);
  }
  return parseFloat(match[1]);
}

/** セクション本文から、指定の書き出しで始まる箇条書きの行(先頭の `- ` を除く)。 */
export function bulletLine(section: string, prefix: string): string {
  const line = section
    .split("\n")
    .map((text) => text.trim().replace(/^-\s*/, ""))
    .find((text) => text.startsWith(prefix));
  if (line === undefined) {
    throw new Error(`design.md に「${prefix}」で始まる行が無い`);
  }
  return line;
}

/** `## 見出し` 直下の箇条書きを含む本文(### ではなく ## の節。例: 「動き」)。 */
export function level2SectionOf(markdown: string, heading: string): string {
  const lines = markdown.split("\n");
  const start = lines.findIndex((line) => line.trim() === `## ${heading}`);
  if (start < 0) {
    throw new Error(`design.md に「## ${heading}」が無い`);
  }
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => /^#{1,2} /.test(line));
  return (end < 0 ? rest : rest.slice(0, end)).join("\n");
}
