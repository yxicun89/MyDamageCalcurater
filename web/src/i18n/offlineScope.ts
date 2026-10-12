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

/** ADR-0358 段 3: 「オフライン用にポケモンのデータを保存」(このアプリについて)の文言。iOS は同じ言い方にそろえる。 */
export const offlinePreloadText = {
  sectionHeading: "オフライン用のデータ",
  explanation:
    "オンラインのうちに、全ポケモンのデータをこの端末に保存します。保存すると、オフラインでも好きなポケモンを選んで計算・逆算できます。データはこの端末の中だけに置かれ、サーバーには送りません。",
  startButton: "オフライン用にポケモンのデータを保存",
  resumeButton: "続きから保存する",
  stopButton: "中断する",
  /** オフライン(計算する場所がこの端末)のときにボタンを押せない理由。 */
  offlineReason: "計算する場所を「サーバー(オンライン)」にすると保存できます。",
  progress: (done: number, total: number): string => `保存中 ${String(done)} / ${String(total)}`,
  preparing: "ポケモンの一覧を取得しています",
  completed: (count: number): string => `保存済み: ${String(count)}匹。オフラインで使えます`,
  aborted: (count: number): string =>
    `中断しました。保存済み: ${String(count)}匹。もう一度押すと続きから保存します`,
  /** サーバーに接続できない・応答がおかしいとき。 */
  failedServer: (count: number): string =>
    `サーバーに接続できません。保存済み: ${String(count)}匹。もう一度押すと続きから保存します`,
  /** 端末に書けないとき(容量不足・プライベートブラウズなど)。 */
  failedStorage: (count: number): string =>
    `この端末に保存できませんでした(空き容量が足りない可能性があります)。保存済み: ${String(count)}匹。空き容量を確保してから、もう一度押すと続きから保存します`,
} as const;
