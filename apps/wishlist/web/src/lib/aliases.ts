// 表記揺れの辞書(フェーズ4-1。docs/phase4-spec.md AC-A14)の、設定画面での 1 行 = 1 グループの読み書き。

// 区切りは半角カンマ・全角カンマ・読点。
const SEPARATORS = /[,，、]/;

/** 1 行(カンマ区切り)を語の配列にする。各語の前後の空白(全角を含む)を除き、空の語は捨てる。 */
export const parseAliasLine = (line: string): string[] =>
  line
    .split(SEPARATORS)
    .map((w) => w.trim())
    .filter((w) => w !== "");

/** グループを 1 行の表示にする。 */
export const formatAliasGroup = (group: string[]): string => group.join(", ");

/** 行の配列をグループの配列にする。空の行は除く。invalidRow は 1 語だけの最初の行(1 始まり)。 */
export const parseAliasGroups = (lines: string[]): { groups: string[][]; invalidRow: number | null } => {
  const groups: string[][] = [];
  let invalidRow: number | null = null;
  lines.forEach((line, i) => {
    const words = parseAliasLine(line);
    if (words.length === 0) return;
    if (words.length === 1 && invalidRow === null) invalidRow = i + 1;
    groups.push(words);
  });
  return { groups, invalidRow };
};
