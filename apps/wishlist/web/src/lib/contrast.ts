/** 地の色の上に置く文字・線画の色を決める(phase6-spec「デザインの決め」)。WCAG の相対輝度で、対比の高いほうを選ぶ。 */
export const ON_LIGHT = "#ffffff";
export const ON_DARK = "#0e1a2b";

const channel = (v: number): number => {
  const s = v / 255;
  return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
};

/** `#rrggbb` の相対輝度(0〜1)。読めなければ null。 */
export const luminance = (hex: string): number | null => {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m?.[1]) return null;
  const n = parseInt(m[1], 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
};

/** 地が `background` のときの文字色。読めない色は白(強調色の上と同じ)。 */
export const onColor = (background: string): string => {
  const l = luminance(background);
  if (l === null) return ON_LIGHT;
  const white = 1.05 / (l + 0.05);
  const dark = (l + 0.05) / ((luminance(ON_DARK) ?? 0) + 0.05);
  return white >= dark ? ON_LIGHT : ON_DARK;
};
