// 構築ビルダー(P5-5・ADR-0309)の文言。ADR-0173 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

/**
 * P5-5 PR-A1: 構築 API のクライアント(team/teamClient.ts)が、通信できない・応答が読めない・
 * エラー本文の形が不正なときに作る文言(ADR-0309 §3。speedClientText・judgeClientText と同じ形)。
 * サーバーが返す `Error.message` はそのまま運ぶので、ここには含まない。
 */
export const teamClientText = {
  unavailable: "構築の API に接続できません",
} as const;

/**
 * P5-5 PR-A1: 構築ビルダーの画面(team/TeamScreen.tsx、ADR-0309)の文言。
 * この段階(PR-A1)で扱うのは一覧・新規作成(名前だけ)・名前変更・削除まで。
 * メンバー(種族・技・持ち物・特性・性格・SP・テラスタイプ)の編集は PR-A2 で足す。
 */
export const teamScreenText = {
  /** 画面全体の領域(role="region" の名前)。 */
  regionLabel: "構築",
  /** 一覧(`<ul>`)の名前と、その上の見出し。 */
  listLabel: "保存した構築",
  listHeading: "保存した構築",
  /** 一覧を読み込んでいる間(新規作成のフォームは先に使える。ADR-0309 §4)。 */
  loadingNotice: "読み込み中",
  /** 1件も無いとき(エラーと取り違えない案内。ADR-0309 §4)。 */
  emptyNotice: "保存した構築はまだありません。名前を付けて作成してください",
  /** 構築1件の要約(メンバー数・最終更新。PR-A1 ではメンバーは常に0体)。 */
  memberCountLabel: (count: number, max: number): string => `${count}/${max}体`,
  updatedAtLabel: (date: string): string => `最終更新 ${date}`,
  // ---- 新規作成 ----
  createHeading: "新しい構築",
  nameLabel: "構築名",
  createLabel: "作成",
  /** 送信前の検査(契約の TeamInput.name と同じ範囲。前後の空白を除いて1〜50文字)。 */
  nameRequiredNotice: "構築名を入力してください",
  nameTooLongNotice: (max: number): string => `構築名は${max}文字までです`,
  // ---- 名前変更 ----
  renameLabel: (name: string): string => `「${name}」の名前を変更`,
  renameFieldLabel: (name: string): string => `「${name}」の新しい構築名`,
  renameSaveLabel: "名前を保存",
  renameCancelLabel: "名前の変更をやめる",
  // ---- 削除(2段階。window.confirm は使わない。ADR-0309 §5)----
  deleteLabel: (name: string): string => `「${name}」を削除`,
  deleteConfirmLabel: (name: string): string => `「${name}」の削除を確定`,
  deleteCancelLabel: (name: string): string => `「${name}」の削除をやめる`,
  deleteConfirmNotice: (name: string): string => `「${name}」を削除します。取り消せません`,
  // ---- 失敗(role="alert"。サーバーの message はこの見出しに続けてそのまま出す)----
  loadErrorHeading: "構築の一覧を読み込めませんでした",
  createErrorHeading: "構築を作成できませんでした",
  renameErrorHeading: "構築の名前を変えられませんでした",
  deleteErrorHeading: "構築を削除できませんでした",
} as const;

/**
 * P5-5b PR-A2(ADR-0316): 構築のメンバー編集の文言。画面は web/src/team/ にある。
 * 種族・持ち物などのコントロールの accessible name は「メンバーの group(legend = 「1体目」)」の中で引くので、
 * 体の番号は名前に含めない。SP の6欄だけは statLetterJa の1文字表記を使う。
 */
export const teamMemberText = {
  /** 構築の行から編集領域を開く / 領域の名前 / 閉じる(未保存の編集は捨てる。API は呼ばない)。 */
  editLabel: (name: string): string => `「${name}」のメンバーを編集`,
  editorLabel: (name: string): string => `「${name}」のメンバー編集`,
  closeLabel: "編集を閉じる",
  /** パーティ全体を保存する(update は全置換。ADR-0309 §4)。 */
  saveLabel: "メンバーを保存",
  savedNotice: "保存しました",
  saveErrorHeading: "メンバーを保存できませんでした",
  /** メンバーの追加・削除・並べ替え。 */
  addLabel: "メンバーを追加",
  addDisabledNotice: (max: number): string => `メンバーは${max}体までです`,
  memberLegend: (position: number): string => `${position}体目`,
  removeLabel: (position: number): string => `${position}体目を削除`,
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
  speciesRequiredError: "ポケモンを選んでください",
  moveDuplicateError: "同じ技は1体に1つだけ選べます",
} as const;
