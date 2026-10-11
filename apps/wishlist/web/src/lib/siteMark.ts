/** サイトのマークの地の色。sites に色の項目がまだ無いので、名前から引く固定の表(無い名前は --muted)。 */
const SITE_COLORS: Record<string, string> = {
  メルカリ: "#ff4d6d",
  駿河屋: "#e17055",
  "Yahoo!フリマ": "#ef233c",
  あみあみ: "#00a884",
  Amazon: "#ff9f1c",
  魂ウェブ: "#5b4bd6",
};

export const siteColor = (name: string): string => SITE_COLORS[name] ?? "var(--muted)";

/** サイト名の先頭 1 文字 */
export const siteMark = (name: string): string => Array.from(name)[0] ?? "";
