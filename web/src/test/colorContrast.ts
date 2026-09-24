// issue #99(アクセシビリティ): WCAG 2.2 の相対輝度・コントラスト比の計算(SC 1.4.3)。
// design.md の DesignColor(#RRGGBB または rgba(...))から色を読み、通常文字の基準 4.5:1 を検査するのに使う。

/** #RRGGBB または rgb(a)(r, g, b[, a]) を 0〜255 の RGBA に変換する。 */
export function parseColor(css: string): { r: number; g: number; b: number; a: number } {
  const hex = /^#([0-9A-Fa-f]{6})$/.exec(css);
  if (hex?.[1] !== undefined) {
    const value = hex[1];
    return {
      r: parseInt(value.slice(0, 2), 16),
      g: parseInt(value.slice(2, 4), 16),
      b: parseInt(value.slice(4, 6), 16),
      a: 1,
    };
  }
  const rgba = /^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)$/.exec(css);
  if (rgba?.[1] !== undefined && rgba[2] !== undefined && rgba[3] !== undefined) {
    return {
      r: parseInt(rgba[1], 10),
      g: parseInt(rgba[2], 10),
      b: parseInt(rgba[3], 10),
      a: rgba[4] === undefined ? 1 : parseFloat(rgba[4]),
    };
  }
  throw new Error(`色として解釈できない: ${css}`);
}

/** 半透明の前景色を不透明な背景色の上に重ねた、見た目上の不透明色(アルファブレンド)。 */
export function compositeOver(foreground: string, background: string): string {
  const fg = parseColor(foreground);
  const bg = parseColor(background);
  const blend = (fgChannel: number, bgChannel: number): number => fgChannel * fg.a + bgChannel * (1 - fg.a);
  const toHex = (channel: number): string => Math.round(channel).toString(16).padStart(2, "0");
  return `#${toHex(blend(fg.r, bg.r))}${toHex(blend(fg.g, bg.g))}${toHex(blend(fg.b, bg.b))}`;
}

/** sRGB の 1 チャンネル(0〜255)を WCAG の相対輝度計算用の線形値にする。 */
function linearizeChannel(channel255: number): number {
  const channel = channel255 / 255;
  return channel <= 0.03928 ? channel / 12.92 : Math.pow((channel + 0.055) / 1.055, 2.4);
}

/**
 * WCAG 2.2 の相対輝度(0〜1)。半透明色は先に compositeOver で不透明にしてから渡すこと
 * (半透明のまま渡すと、実際より明るい/暗い誤った値になりうるため、ここで弾く)。
 */
export function relativeLuminance(css: string): number {
  const { r, g, b, a } = parseColor(css);
  if (a < 1) {
    throw new Error(`半透明の色はそのまま輝度計算できない(compositeOver で不透明にしてから渡す): ${css}`);
  }
  return 0.2126 * linearizeChannel(r) + 0.7152 * linearizeChannel(g) + 0.0722 * linearizeChannel(b);
}

/** WCAG 2.2 のコントラスト比(1〜21)。順序に依存しない(常に明るい方 ÷ 暗い方)。 */
export function contrastRatio(colorA: string, colorB: string): number {
  const luminanceA = relativeLuminance(colorA);
  const luminanceB = relativeLuminance(colorB);
  const lighter = Math.max(luminanceA, luminanceB);
  const darker = Math.min(luminanceA, luminanceB);
  return (lighter + 0.05) / (darker + 0.05);
}

/** WCAG 2.2 SC 1.4.3(通常文字)の最低基準。 */
export const WCAG_NORMAL_TEXT_MIN_CONTRAST = 4.5;
