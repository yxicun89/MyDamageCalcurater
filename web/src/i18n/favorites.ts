// お気に入り(手動ピン留め。P5-3c・ADR-0327)の文言。このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

/** お気に入りの画面(favorites/FavoritesScreen.tsx)の文言。サーバーの message は見出しの後ろにそのまま出す。 */
export const favoritesScreenText = {
  /** タブの表示名。 */
  tabLabel: "お気に入り",
  /** 画面全体の領域(role="region" の名前)。 */
  regionLabel: "お気に入り",
  /** 一覧(`<ul>`)の名前。 */
  listLabel: "お気に入り一覧",
  loadingNotice: "読み込み中…",
  emptyNotice: "お気に入りはまだありません。計算画面で攻撃側を選んで追加できます。",
  /** オフライン(計算モード)のとき。API には触れない。 */
  offlineNotice: "お気に入りはオンラインモードで使えます。ヘッダーの計算モードをオンラインにしてください。",
  countLabel: (count: number, max: number): string => `${String(count)}/${String(max)}件`,
  listErrorHeading: "お気に入りを読み込めませんでした",
  deleteLabel: (title: string): string => `「${title}」を削除`,
  deleteConfirmNotice: (title: string): string => `「${title}」をお気に入りから外します。よろしいですか?`,
  deleteConfirmLabel: (title: string): string => `「${title}」を削除する`,
  deleteCancelLabel: (title: string): string => `「${title}」の削除をやめる`,
  deleteErrorHeading: "お気に入りを削除できませんでした",
} as const;

/** 計算画面の「攻撃側をお気に入りに追加」(CalcScreen.tsx)の文言。 */
export const favoritesCalcText = {
  addLabel: "攻撃側をお気に入りに追加",
  addedNotice: "お気に入りに追加しました",
  alreadyNotice: "すでにお気に入りに入っています",
  addErrorHeading: "お気に入りに追加できませんでした",
} as const;
