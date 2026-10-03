// タイプバランス(P4-12a・ADR-0303、ADR-0411)の文言。ADR-0323 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

import { pokemonFieldLabel, speciesPlaceholderOption } from "./common";

/**
 * P4-12a: balance API のクライアント(api/balanceClient.ts、ADR-0303 §1・§6)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動でオフラインへは切り替えない)。
 */
export const balanceClientText = {
  unavailable: "タイプバランスの API に接続できません",
} as const;

/**
 * issue 276(ADR-0411): balance API のエラーコード(ErrorCode)→日本語の文言。画面は応答の message(英語の
 * 内部メッセージ)を出さず、コードからここを引く。Web 側の balance_unavailable も同じ表で引く。
 */
export const balanceErrorText = {
  missing_header: "端末の情報を送れませんでした。ページを開き直してください",
  invalid_header: "端末の情報が正しくありません。ページを開き直してください",
  invalid_request: "リクエストが正しくありません。入力を見直してください",
  request_too_large: "入力が大きすぎます。メンバーや技を減らしてください",
  unknown_pokemon: "選んだポケモンがサーバーのマスタにありません。選び直してください",
  unknown_move: "選んだ技がサーバーのマスタにありません。選び直してください",
  unknown_ability: "選んだ特性がサーバーのマスタにありません。選び直してください",
  master_unavailable: "サーバーのマスタを読み込めません。しばらくしてからもう一度お試しください",
  overloaded: "サーバーが混み合っています。しばらくしてからもう一度お試しください",
  internal_error: "サーバーでエラーが起きました。しばらくしてからもう一度お試しください",
  balance_unavailable: "タイプバランスの API に接続できません",
  fallback: "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください",
} as const;

/**
 * P4-12a: タイプバランスの倍率の表示(domain/balanceLabels.ts、ADR-0303 §2、docs/type-balance-design.md §10)。
 * 倍率は色だけで表さず、語も文字で出す。
 */
export const balanceLabelText = {
  /** DefenseCategory(balance.gen.ts)→ 語。値の範囲は balance-svc が既に判定済みなので Web では判定し直さない。 */
  defenseCategoryWord: {
    quad_weak: "弱点",
    weak: "弱点",
    neutral: "等倍",
    resist: "耐性",
    quad_resist: "耐性",
    immune: "無効",
  } as const,
  /** CoverageMultiplier(null を除く)→ 語。 */
  coverageWord: {
    "0": "無効",
    "1/2": "いまひとつ",
    "1": "等倍",
    "2": "抜群",
  } as const,
  /** CoverageMultiplier が null(攻撃技なし)のときの表示。 */
  coverageNoAttackMove: "攻撃技なし",
  /** P4-12b: ThreatMatchup.safe(応答の真偽値のまま。ADR-0303 §7)。 */
  safe: "安全",
  unsafe: "注意",
  /** P4-12b: ThreatMatchup.superEffective(応答の真偽値のまま。ADR-0303 §7)。 */
  superEffective: "抜群",
  notSuperEffective: "ふつう",
} as const;

/** P4-12a: タイプバランスの画面(screens/BalanceScreen.tsx、ADR-0303 §2)の文言。 */
export const balanceScreenText = {
  memberGroupLabel: (n: number): string => `メンバー${String(n)}`,
  addMemberLabel: "メンバーを追加",
  removeMemberLabel: (n: number): string => `メンバー${String(n)}を削除`,
  /** 同じ物を指すラベルの語は画面をまたいで同じにする(issue 304)。 */
  speciesLabel: pokemonFieldLabel,
  /** critic指摘(issue 304): 未選択の種族optionの文言も、画面をまたいで同じ語を参照する形にする。 */
  speciesPlaceholderOption: speciesPlaceholderOption,
  abilityLabel: "特性",
  /** ポケモンを選ぶまで特性の候補が1件も無いとき、未選択の option に出す文言(issue 304)。 */
  abilityPlaceholderOption: `${pokemonFieldLabel}を選ぶと選べます`,
  moveLabel: (slot: number): string => `技${String(slot)}`,
  noMoveOption: "なし",
  loadingNotice: "計算中",
  defenseTableLabel: "防御相性",
  teamSummaryTableLabel: "チームの集計",
  coverageTableLabel: "攻撃範囲",
  memberColumnLabel: "メンバー",
  attackTypeColumnLabel: "攻撃タイプ",
  weakColumnLabel: "弱点",
  quadWeakColumnLabel: "うち×4",
  resistColumnLabel: "耐性",
  immuneColumnLabel: "無効",
  neutralColumnLabel: "等倍",
  defenseTypeColumnLabel: "防御タイプ",
  bestMultiplierColumnLabel: "最大倍率",
  effectiveColumnLabel: "有効",
  superEffectiveColumnLabel: "抜群",
  // ---- P4-12b: 仮想敵(threats)・おすすめタイプ(recommendations)(ADR-0303 §7、ADR-0400、ADR-0401) ----
  threatGroupLabel: (n: number): string => `仮想敵${String(n)}`,
  addThreatLabel: "仮想敵を追加",
  removeThreatLabel: (n: number): string => `仮想敵${String(n)}を削除`,
  /** 仮想敵ごとの結果のかたまり(region)の名前。 */
  threatRegionLabel: (n: number, nameJa: string): string => `仮想敵${String(n)}(${nameJa})`,
  threatMatchupTableLabel: "相性",
  incomingColumnLabel: "受ける倍率",
  outgoingColumnLabel: "与える倍率",
  safeColumnLabel: "安全",
  safeMembersLabel: (n: number): string => `安全に受けられる ${String(n)}人`,
  superEffectiveMembersLabel: (n: number): string => `抜群を取れる ${String(n)}人`,
  threatsLoadingNotice: "仮想敵を計算中",
  recommendationsRegionLabel: "おすすめタイプ",
  recommendationsLoadingNotice: "おすすめタイプを計算中",
  defenseHolesLabel: (list: string): string => `防御の穴: ${list}`,
  offenseHolesLabel: (list: string): string => `攻撃範囲の穴: ${list}`,
  candidatesTableLabel: "おすすめタイプの候補",
  typesColumnLabel: "タイプ",
  defenseCoveredColumnLabel: "ふさぐ防御の穴",
  offenseCoveredColumnLabel: "ふさぐ攻撃範囲の穴",
  pokemonColumnLabel: "ポケモン",
  abilityOptionsTableLabel: "特性で補えるポケモン",
  /** 一覧が空のときの表示(防御・攻撃範囲の穴、候補・特性の該当ポケモン)。 */
  noneLabel: "なし",
  /** タイプ・ポケモンの一覧を並べるときの区切り。 */
  listSeparator: "・",
  /** 特性で補えるポケモンの1件(「名前(特性名 ×倍率)」)。 */
  abilityOptionEntryLabel: (name: string, ability: string, multiplier: string): string =>
    `${name}(${ability} ${multiplier})`,
} as const;
