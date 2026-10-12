// ADR-0358(案 A)段 1・2: オフラインで使える範囲の表示の文言。iOS が同じ範囲を同じ言い方にそろえる
// (docs/ai-shared/decisions/2026-10-12-web-offline-scope.md)。UI に「種族」は使わず「ポケモン」「○匹」と言う(docs/glossary.md)。

export const offlineScopeText = {
  /** 1 行の領域の名前(role=note のアクセシブルネーム)。 */
  regionLabel: "オフラインで使える範囲",
  /** 保存済みが 1 匹以上のとき。 */
  summary: (count: number): string =>
    `保存済み: ${String(count)}匹。計算・逆算が使えます。素早さ・構築・お気に入り・調整はオンラインで`,
  /** 保存済みが 0 のとき(読めない場合も同じ)。 */
  empty:
    "保存済みのポケモンはまだありません。オンラインで一度開くと保存されます。素早さ・構築・お気に入り・調整はオンラインで",
  /** 押せないタブに出す理由(表示と aria-describedby)。 */
  onlineOnlyReason: "オンラインのみ",
} as const;
