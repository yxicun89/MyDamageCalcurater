export interface QueryInput {
  template: string;
  name: string;
  option: string | null;
  queryOverride: string | null;
  siteQuery: string | null;
}

/** 連続する空白(全角・タブを含む)を 1 つの半角空白にして前後を落とす。 */
const squeeze = (s: string): string => s.replace(/\s+/g, " ").trim();

/** 検索ワードを作る(Go の query.Build と同じ規則。testdata/query-cases.json の build)。 */
export const buildQuery = (input: QueryInput): string => {
  const site = squeeze(input.siteQuery ?? "");
  if (site !== "") return site;
  const override = squeeze(input.queryOverride ?? "");
  if (override !== "") return override;
  // 1 回の走査で置換する(値の中の {option} などを再置換しない)。
  const filled = input.template.replace(/\{(name|option)\}/g, (_, key: string) =>
    key === "name" ? input.name : (input.option ?? ""),
  );
  return squeeze(filled);
};
