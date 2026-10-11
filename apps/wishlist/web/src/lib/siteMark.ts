/** サイトのマークの地の色。sites に色の項目がまだ無いので、名前から引く固定の表(無い名前は --muted)。 */
const SITE_COLORS: Record<string, string> = {
  メルカリ: "#ff4d6d",
  駿河屋: "#e17055",
  "Yahoo!フリマ": "#ef233c",
  "Yahoo!ショッピング": "#2f9e44",
  あみあみ: "#00a884",
  Amazon: "#ff9f1c",
  魂ウェブ: "#5b4bd6",
};

export const siteColor = (name: string): string => SITE_COLORS[name] ?? "var(--muted)";

const YAHOO_PREFIX = "Yahoo!";

/**
 * マークの 1 文字。通常はサイト名の先頭 1 文字。`Yahoo!` で始まる名前は、その後ろの最初の 1 文字
 * (Yahoo!フリマ → 「フ」・Yahoo!ショッピング → 「シ」。同じ「Y」が並ばないため。iOS の SiteMark と同じ)。
 */
export const siteMark = (name: string): string => {
  const rest = name.startsWith(YAHOO_PREFIX) ? name.slice(YAHOO_PREFIX.length) : "";
  return Array.from(rest || name)[0] ?? "";
};
