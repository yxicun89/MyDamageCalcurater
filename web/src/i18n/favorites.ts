// お気に入り(手動ピン留め。P5-3c・ADR-0327)の文言。このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

import type { FavoriteRestoreIssue } from "../favorites/favoriteCalc";

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
  /** 各行の主ボタン(見える文字 = 名前。ADR-0333 §2)。 */
  useLabel: (title: string): string => `「${title}」を計算に使う`,
  /** calc の無い旧お気に入りの行に添える案内。 */
  attackerOnlyHint: "攻撃側だけ(防御側・技・条件は今の計算のまま)",
} as const;

/** 反映しなかった項目(FavoriteRestoreIssue の ignored.field)の日本語名。 */
const IGNORED_FIELD_NAMES: Readonly<Record<string, string>> = {
  format: "ダブル形式",
  attackerSp: "攻撃側の攻撃・特攻以外の能力ポイント",
  attackerTeraType: "攻撃側のテラスタイプ",
  attackerStatus: "攻撃側のやけど以外の状態異常",
  attackerRanks: "攻撃側の攻撃・特攻以外のランク",
  attackerScreens: "攻撃側の壁",
  defenderBuild: "防御側の育成(能力ポイント・性格・テラスタイプ・状態異常・ランク)",
};

/** お気に入りから計算画面に戻したときの案内(CalcScreen.tsx。ADR-0333 §2・§4)。 */
export const favoritesRestoreText = {
  restoredNotice: (title: string): string => `「${title}」の計算を開きました`,
  attackerOnlyNotice: (title: string): string =>
    `「${title}」は攻撃側だけを戻しました(防御側・技・条件は今の計算のままです)`,
  unresolvedHeading: "お気に入りの一部を戻せませんでした",
  megaItemNotice:
    "メガシンカの持ち物はメガストーンに固定のため、保存された持ち物ではなくメガストーンで計算しました",
  /** 技を戻せなかったときの、技の選択欄の未選択の表示。 */
  moveUnselectedOption: "技を選んでください",
  ignoredNotice: "この画面で表せない項目は反映していません",
  issueText: (issue: FavoriteRestoreIssue): string => {
    const sideName = (side: "attacker" | "defender"): string => (side === "attacker" ? "攻撃側" : "防御側");
    switch (issue.kind) {
      case "species":
        return `${sideName(issue.side)}のポケモン(${issue.id})`;
      case "move":
        return `技(${issue.id})`;
      case "item":
        return `${sideName(issue.side)}の持ち物(${issue.id})`;
      case "ability":
        return `${sideName(issue.side)}の特性(${issue.id})`;
      case "nature":
        return `攻撃側の性格(${issue.id})`;
      case "megaItem":
        return `${sideName(issue.side)}の持ち物(${issue.savedItemId === "" ? "なし" : issue.savedItemId})`;
      case "ignored":
        return `反映していない項目(${IGNORED_FIELD_NAMES[issue.field] ?? issue.field})`;
    }
  },
} as const;

/** 計算画面の「攻撃側をお気に入りに追加」(CalcScreen.tsx)の文言。 */
export const favoritesCalcText = {
  addLabel: "攻撃側をお気に入りに追加",
  addedNotice: "お気に入りに追加しました",
  alreadyNotice: "すでにお気に入りに入っています",
  addErrorHeading: "お気に入りに追加できませんでした",
} as const;
