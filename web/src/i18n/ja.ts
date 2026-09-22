// 画面の文言資源(日本語)。タイプの表示名はここに ID → 表示名で持つ(ADR-0300 §4)。
// タイプの一覧(どの ID が存在するか)は相性表のデータ(testdata/golden/typechart.json の types)が正であり、
// ここでは全 ID を過不足なく覆う対応表だけを持つ(コーディング規約 §2: マスタをコードに埋め込まない)。
// TypeId のユニオンは手書きの複製だが、相性表と過不足なく一致することを ja.test.ts が検査して同期を保つ
// (コーディング規約 §2 の「独立した検証」)。

import type { StatKey } from "../engine/types";

/** 相性表が持つ18タイプの ID(`testdata/golden/typechart.json` の `types` と同じ)。 */
export type TypeId =
  | "bug"
  | "dark"
  | "dragon"
  | "electric"
  | "fairy"
  | "fighting"
  | "fire"
  | "flying"
  | "ghost"
  | "grass"
  | "ground"
  | "ice"
  | "normal"
  | "poison"
  | "psychic"
  | "rock"
  | "steel"
  | "water";

/** タイプ ID → 日本語の表示名(docs/design.md 「タイプ色(自作パレット)」の表と同じ呼び名)。 */
export const typeNameJa: Record<TypeId, string> = {
  normal: "ノーマル",
  fire: "ほのお",
  water: "みず",
  electric: "でんき",
  grass: "くさ",
  ice: "こおり",
  fighting: "かくとう",
  poison: "どく",
  ground: "じめん",
  flying: "ひこう",
  psychic: "エスパー",
  bug: "むし",
  rock: "いわ",
  ghost: "ゴースト",
  dragon: "ドラゴン",
  dark: "あく",
  steel: "はがね",
  fairy: "フェアリー",
};

/**
 * 値がタイプ ID(TypeId)かどうかの型ガード。マスタ由来の文字列(種族の types など)を
 * typeNameJa に渡す前に安全性を確かめるために使い、`as TypeId` の型アサーションを避ける
 * (コーディング規約 §4 TypeScript「非 null 断言・キャストは原則禁止」と同じ精神)。
 */
export function isTypeId(value: string): value is TypeId {
  return Object.hasOwn(typeNameJa, value);
}

/**
 * 計算画面(P4-2)の文言。コーディング規約 §2「UI の文言は文言資源に置く」に従い、
 * 画面・書式のコードはここの語だけを組み合わせ、日本語の文字列リテラルを直接持たない。
 */
export const calcScreenText = {
  attackerPokemonLabel: "攻撃側のポケモン",
  defenderPokemonLabel: "防御側のポケモン",
  attackerItemLabel: "攻撃側の持ち物",
  defenderItemLabel: "防御側の持ち物",
  moveLabel: "技",
  attackerRegionLabel: "攻撃側",
  defenderRegionLabel: "防御側",
  noItemOption: "なし",
  noItemRowLabel: "持ち物なし",
  resultsListLabel: "計算結果",
  compareItemCandidatesLabel: "持ち物の候補も比較",
  swapButtonLabel: "攻守入れ替え",
  statusMoveNotice: "変化技はダメージを計算しません",
  /** 技セレクタの各行の区切り(「技名・分類・威力n」)。 */
  moveOptionSeparator: "・",
  /** 技セレクタの威力の前置き(「威力80」)。 */
  movePowerLabel: "威力",
  /** 入力が揃い calcBulk の応答待ちのときに出す文言(古い行を出さず、これに差し替える)。 */
  loadingNotice: "計算中",
  /** 攻撃側プリセットのラジオグループの名前(P4-3、ADR-0300 §5)。 */
  attackerPresetGroupLabel: "攻撃側の調整",
} as const;

/**
 * 攻撃側プリセット(domain/attackerPresets.ts、P4-3、ADR-0300 §5)の文言。
 * X は技の分類で決まる関連ステータス(物理・変化 = atk、特殊 = spa)。
 */
export const attackerPresetText = {
  /** none の表示名(分類によらず共通)。 */
  none: "無振り",
  /** X のステータスの1文字表記(物理・変化 = A、特殊 = C)。 */
  statLetter: { atk: "A", spa: "C" } as const,
  /** x_full の接尾辞(「A特化」「C特化」)。 */
  fullSuffix: "特化",
  /** x の接尾辞(「A振り(無補正)」「C振り(無補正)」)。 */
  xSuffix: "振り(無補正)",
} as const;

/** アプリ全体(App.tsx)の文言。 */
export const appText = {
  title: "ポケモン ダメージ計算",
  loading: "読み込み中…",
  masterLoadError: "マスタデータの読み込みに失敗しました",
  /** 計算・逆算の切り替えタブ(P4-4、ADR-0300 §7)。 */
  tabsLabel: "画面の切り替え",
  /** サイト名(index.html の <title> と同じ。文書タイトルの接尾辞)。 */
  siteTitle: "pokecalc",
  calcTabLabel: "計算",
  reverseTabLabel: "逆算",
  /** P4-12a: タイプバランスのタブ(ADR-0303 §2)。 */
  balanceTabLabel: "タイプバランス",
  /** 計算モード(オフライン = WASM / オンライン = API)の切り替え(P4-5、ADR-0301 §4)。 */
  calcModeGroupLabel: "計算モード",
  calcModeOfflineLabel: "オフライン(WASM)",
  calcModeOnlineLabel: "オンライン(API)",
} as const;

/**
 * P4-12a: balance API のクライアント(api/balanceClient.ts、ADR-0303 §1・§6)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動でオフラインへは切り替えない)。
 */
export const balanceClientText = {
  unavailable: "タイプバランスの API に接続できません",
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
  speciesLabel: "ポケモン",
  abilityLabel: "特性",
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

/**
 * API 実装(createApiEngine、P4-5)がクライアント側(fetch する前・応答を読めないとき)で作るエラーの文言
 * (ADR-0301 §2・§4)。サーバーが返す Error.message はそのまま運ぶので、ここには含まない。
 */
export const apiEngineText = {
  /** 個体の性格(plus/minus の組)に一致するマスタの性格が無いとき。 */
  unknownNature: "この性格に対応するマスタの性格が見つかりません",
  /** 通信できない・応答が読めない・エラー本文の形が不正なとき(ADR-0301 §4: 自動フォールバックはしない)。 */
  unavailable: "API に接続できません",
  /** BulkRequest.presets(engine のカスタムプリセット定義)は API に送れないとき(ADR-0301 §2)。 */
  invalidPreset: "カスタムの防御側プリセット定義は API に送れません",
} as const;

/** ステータスの1文字表記(H・A・B・C・D・S)。逆算の SP 範囲・目安の名前の表示に使う(P4-4)。 */
export const statLetterJa: Record<StatKey, string> = {
  hp: "H",
  atk: "A",
  def: "B",
  spa: "C",
  spd: "D",
  spe: "S",
};

/**
 * 逆算画面(P4-4、ADR-0300 §7、ADR-0010 §R)の入力まわりの文言。
 * n を含む語は行番号(1始まり)から作る関数にする(観測は複数行あるため)。
 */
export const reverseScreenText = {
  sideGroupLabel: "観測したダメージ",
  sideDefenderLabel: "与えたダメージ",
  sideAttackerLabel: "受けたダメージ",
  mySpeciesLabel: "自分のポケモン",
  theirSpeciesLabel: "相手のポケモン",
  myItemLabel: "自分の持ち物",
  myPresetGroupLabel: "自分の調整",
  observationLabel: (n: number): string => `観測${String(n)}`,
  observationUnitGroupLabel: (n: number): string => `観測${String(n)}の単位`,
  removeObservationLabel: (n: number): string => `観測${String(n)}を削除`,
  addObservationLabel: "観測を追加",
  percentUnitLabel: "%",
  damageUnitLabel: "HP",
  percentInvalidMessage: "1〜100 の整数で入力してください",
  damageInvalidMessage: "1 以上の整数で入力してください",
  resultsListLabel: "推定結果",
} as const;

/** 逆算の結果の表示(domain/reverseLabels.ts)の文言(ADR-0010 §R1・§R3)。 */
export const reverseResultText = {
  natureClassNeutral: "補正なし",
  /** 「関連ステータス上昇」の接尾辞(「B上昇」「C上昇」)。 */
  natureClassPlusSuffix: "上昇",
  closeCandidateLabel: "近い候補",
  /** 防御側の結果に添える、H の仮定の注記(ADR-0010 §R1: 防御側は H32 前提)。 */
  assumedHpNote: (assumedHpSp: number): string => `H${String(assumedHpSp)} を仮定した結果です`,
  /** 目安の名前(ADR-0010 §R3)の部品。範囲が SP 0 / 32 を含むとき、性格クラスと組んで併記する。 */
  guide: {
    defenderZeroNeutral: "H振り",
    defenderZeroPlusPrefix: "H振り+",
    defenderZeroPlusSuffix: "補正",
    defenderFullNeutralPrefix: "H",
    defenderFullNeutralSuffix: "振り",
    defenderFullPlusPrefix: "H",
    defenderFullPlusSuffix: "特化",
    attackerZeroNeutral: "無振り",
    attackerZeroPlusSuffix: "補正のみ",
    attackerFullNeutralSuffix: "振り",
    attackerFullPlusSuffix: "特化",
  },
} as const;

/** 計算結果の書式(domain/format.ts)で使う語。 */
export const resultText = {
  determinedPrefix: "確定",
  randomPrefix: "乱数",
  hitsSuffix: "発",
  cannotKO: "倒せない",
  effectivenessNone: "効果なし",
  effectivenessNotVery: "効果はいまひとつ",
  effectivenessNeutral: "等倍",
  effectivenessSuper: "効果はばつぐん",
  moveCategory: { physical: "物理", special: "特殊", status: "変化" },
} as const;
