export interface QueryInput {
  template: string;
  name: string;
  option: string | null;
  queryOverride: string | null;
  siteQuery: string | null;
}

/** 連続する空白(全角・タブを含む)を 1 つの半角空白にして前後を落とす。 */
const squeeze = (s: string): string => s.replace(/\s+/g, " ").trim();

/** 照合用の正規化(Go の query.Normalize と同じ: NFKC → 小文字 → 空白・記号を除く)。 */
export const normalizeForMatch = (s: string): string =>
  s
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[\s\p{P}\p{S}\p{Z}]/gu, "");

/**
 * template の固定の語(空白で区切った語のうち {name}・{option} を含まないもの)のうち、
 * 正規化した name にもう含まれるものを除く(公式ページの名前はテンプレートの語を含むため。フェーズ5 5-1c)。
 */
const dropFixedWordsIn = (template: string, name: string): string => {
  const n = normalizeForMatch(name);
  return template
    .split(/\s+/)
    .filter((w) => {
      if (w === "" || w.includes("{name}") || w.includes("{option}")) return true;
      const nw = normalizeForMatch(w);
      return nw === "" || !n.includes(nw);
    })
    .join(" ");
};

/** 検索ワードを作る(Go の query.Build と同じ規則。testdata/query-cases.json の build)。 */
export const buildQuery = (input: QueryInput): string => {
  const site = squeeze(input.siteQuery ?? "");
  if (site !== "") return site;
  const override = squeeze(input.queryOverride ?? "");
  if (override !== "") return override;
  // 1 回の走査で置換する(値の中の {option} などを再置換しない)。
  const filled = dropFixedWordsIn(input.template, input.name).replace(
    /\{(name|option)\}/g,
    (_, key: string) => (key === "name" ? input.name : (input.option ?? "")),
  );
  return squeeze(filled);
};
