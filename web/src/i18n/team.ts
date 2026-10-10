// 構築ビルダー(P5-5・ADR-0309)の文言。ADR-0323 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

/**
 * P5-5 PR-A1: 構築 API のクライアント(team/teamClient.ts)が、通信できない・応答が読めない・
 * エラー本文の形が不正なときに作る文言(ADR-0309 §3。speedClientText・judgeClientText と同じ形)。
 * サーバーが返す `Error.message` はそのまま運ぶので、ここには含まない。
 */
export const teamClientText = {
  unavailable: "構築のサーバーに接続できません",
} as const;

/**
 * 構築ビルダーの画面(team/TeamScreen.tsx、ADR-0309・ADR-0332)の文言。
 * 構築名は廃止した(ADR-0332 §1)。一覧の表示名はサーバーの既定名のとき「構築 N」にする(team/teamName.ts)。
 */
export const teamScreenText = {
  /** 画面全体の領域(role="region" の名前)。 */
  regionLabel: "構築",
  /** 一覧(`<ul>`)の名前と、その上の見出し。 */
  listLabel: "保存した構築",
  listHeading: "保存した構築",
  /** 一覧を読み込んでいる間([新しい構築]は先に使える。ADR-0309 §4)。 */
  loadingNotice: "読み込み中",
  /** 1件も無いとき(エラーと取り違えない案内。次にすることを書く)。 */
  emptyNotice: "まだ構築がありません。「新しい構築」を押すと、ポケモンを6体まで選んで構築を作れます",
  /** 構築1件の要約(メンバー数・最終更新)。 */
  memberCountLabel: (count: number, max: number): string => `${count}/${max}体`,
  updatedAtLabel: (date: string): string => `最終更新 ${date}`,
  /** 既定名の構築の表示名(N は既定名の構築を作成の古い順に数えた番号)。 */
  untitledTeamName: (n: number): string => `構築 ${n}`,
  /** 一覧のカードのアイコン列(role="group")の名前と、種族を引けないメンバーのアイコンの名前。 */
  memberIconsLabel: (name: string): string => `「${name}」のポケモン`,
  unknownMemberIcon: (n: number): string => `${n}体目`,
  createLabel: "新しい構築",
  /** カードの見える文字(ボタンの名前は editLabel / deleteLabel の「<名前>」を含む文。WCAG 2.5.3)。 */
  openLabel: "開く",
  deleteShortLabel: "削除",
  // ---- 削除(2段階。window.confirm は使わない。ADR-0309 §5)----
  deleteLabel: (name: string): string => `「${name}」を削除`,
  deleteConfirmLabel: (name: string): string => `「${name}」の削除を確定`,
  deleteCancelLabel: (name: string): string => `「${name}」の削除をやめる`,
  deleteConfirmNotice: (name: string): string => `「${name}」を削除します。取り消せません`,
  // ---- 失敗(role="alert"。サーバーの message はこの見出しに続けてそのまま出す)----
  loadErrorHeading: "構築の一覧を読み込めませんでした",
  createErrorHeading: "構築を作成できませんでした",
  deleteErrorHeading: "構築を削除できませんでした",
} as const;

/**
 * P5-5b PR-A2(ADR-0316): 構築のメンバー編集の文言。画面は web/src/team/ にある。
 * 種族・持ち物などのコントロールの accessible name は「メンバーの group(legend = 「1体目」)」の中で引くので、
 * 体の番号は名前に含めない。SP の6欄だけは statLetterJa の1文字表記を使う。
 */
export const teamMemberText = {
  /** 一覧のカードから編集画面を開く / 編集画面の名前 / 一覧に戻る(未保存なら2段階。API は呼ばない)。 */
  editLabel: (name: string): string => `「${name}」を開く`,
  editorLabel: (name: string): string => `「${name}」のメンバー編集`,
  closeLabel: "一覧に戻る",
  /** 構築を保存する(update は全置換。ADR-0309 §4。種族の決まった枠だけを枠の順に送る)。 */
  saveLabel: "保存",
  savedNotice: "保存しました",
  saveErrorHeading: "メンバーを保存できませんでした",
  /** 保存していない変更の印と、戻るときの確認(ADR-0332 §2)。 */
  unsavedNotice: "保存していない変更があります",
  leaveConfirmNotice: "保存していない変更があります。保存せずに一覧に戻りますか",
  leaveDiscardLabel: "保存せずに戻る",
  leaveCancelLabel: "編集を続ける",
  /** 空の枠の案内。 */
  emptySlotHint: "ポケモンを選ぶと、技・持ち物・特性などを決められます",
  memberLegend: (position: number): string => `${position}体目`,
  removeLabel: (position: number): string => `${position}体目を外す`,
  moveUpLabel: (position: number): string => `${position}体目を上へ`,
  moveDownLabel: (position: number): string => `${position}体目を下へ`,
  /** 各項目のラベル(メンバーの group の中で引く)。 */
  speciesLabel: "ポケモン",
  speciesPlaceholder: "選んでください",
  unknownSpeciesOption: (key: string): string => `不明なポケモン(${key})`,
  speciesResolveError: "ポケモンの情報を読み込めませんでした",
  moveLabel: (slot: number): string => `技${slot}`,
  moveNone: "(なし)",
  itemLabel: "持ち物",
  itemNone: "(なし)",
  abilityLabel: "特性",
  /** 特性が未設定(null)のメンバー用。開いた時点で先頭の特性で黙って埋めない(ADR-0316 §12)。 */
  abilityUnset: "(未選択)",
  natureLabel: "性格",
  teraLabel: "テラスタイプ",
  teraNone: "(なし)",
  /** SP のグリッド。 */
  spLegend: "能力ポイント(SP)",
  spLabel: (letter: string): string => `SP ${letter}`,
  spSummary: (total: number, max: number, remaining: number): string =>
    `合計 ${total}/${max}(残り ${remaining})`,
  /** 入力の検査(role="alert")。 */
  spStatError: (letter: string, max: number): string => `${letter}は0〜${max}の整数で入力してください`,
  spTotalError: (over: number, max: number): string => `合計が${max}を${over}超えています`,
  /** 種族が未選択のメンバーを保存できない理由。6枠の画面では空の枠を保存の対象にしないので届かない(team/teamMember.ts の検査として残す)。 */
  speciesRequiredError: "ポケモンを選んでください",
  moveDuplicateError: "同じ技は1体に1つだけ選べます",
} as const;
