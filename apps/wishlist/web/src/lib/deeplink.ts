/** 検索 URL テンプレートの `{q}` を encodeURIComponent(query) で置き換える(testdata/query-cases.json の deeplink)。 */
export const buildDeeplink = (template: string, query: string): string => {
  const encoded = encodeURIComponent(query);
  // 関数で置換する(値の中の `$&` などを特別扱いさせない。再置換もしない)。
  return template.replace(/\{q\}/g, () => encoded);
};

/** テンプレートが http(s) の URL で `{q}` を含むか(サイト設定の入力検証)。 */
export const isValidSearchTemplate = (template: string): boolean => {
  if (!template.includes("{q}")) return false;
  try {
    const { protocol } = new URL(template);
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
};

/** http(s) の絶対 URL なら true。href に入れる前の最後の確認(javascript: などを出さない)。 */
export const isHttpUrl = (s: string): boolean => {
  try {
    const u = new URL(s);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
};
