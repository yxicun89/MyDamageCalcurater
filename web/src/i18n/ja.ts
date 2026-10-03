// 画面の文言資源(日本語)。タイプの表示名はここに ID → 表示名で持つ(ADR-0300 §4)。
// タイプの一覧(どの ID が存在するか)は相性表のデータ(testdata/golden/typechart.json の types)が正であり、
// ここでは全 ID を過不足なく覆う対応表だけを持つ(コーディング規約 §2: マスタをコードに埋め込まない)。
// TypeId のユニオンは手書きの複製だが、相性表と過不足なく一致することを ja.test.ts が検査して同期を保つ
// (コーディング規約 §2 の「独立した検証」)。

import type { ObservationUnit } from "../domain/observations";
import type {
  ReverseSide,
  StatKey,
  UnsupportedMark,
  UnsupportedReason,
  UnsupportedTarget,
} from "../engine/types";

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
 * 領域(カード・枠)の見える見出しの語と、入力欄の見えるラベルの語(issue 304、docs/design.md
 * 「入力のラベル」)。欄の accessible name は「<領域の見出しの語>の<ラベルの語>」の形に組み立て、
 * 画面(CalcScreen.tsx・ReverseScreen.tsx)はこの組み立て済みの語だけを使う(同じ日本語を2回書かない)。
 */
const attackerRegionLabel = "攻撃側";
const defenderRegionLabel = "防御側";
/** 入力欄の見えるラベルの語(短くする。どちら側かは領域の見出しが担う)。 */
const pokemonFieldLabel = "ポケモン";
const itemFieldLabel = "持ち物";
const abilityFieldLabel = "特性";

/**
 * 計算画面(P4-2)の文言。コーディング規約 §2「UI の文言は文言資源に置く」に従い、
 * 画面・書式のコードはここの語だけを組み合わせ、日本語の文字列リテラルを直接持たない。
 */
export const calcScreenText = {
  attackerRegionLabel,
  defenderRegionLabel,
  /** 入力欄の見えるラベルの語(select の label に使う。計算・逆算・タイプバランスで共通)。 */
  pokemonFieldLabel,
  itemFieldLabel,
  /** 特性欄の見えるラベルの語(issue 272、ADR-0311)。 */
  abilityFieldLabel,
  attackerPokemonLabel: `${attackerRegionLabel}の${pokemonFieldLabel}`,
  defenderPokemonLabel: `${defenderRegionLabel}の${pokemonFieldLabel}`,
  attackerItemLabel: `${attackerRegionLabel}の${itemFieldLabel}`,
  defenderItemLabel: `${defenderRegionLabel}の${itemFieldLabel}`,
  attackerAbilityLabel: `${attackerRegionLabel}の${abilityFieldLabel}`,
  defenderAbilityLabel: `${defenderRegionLabel}の${abilityFieldLabel}`,
  /** 防御側・相手の特性を決め打ちしない選択肢(種族の特性を先頭から最大3件まで全部計算する。ADR-0126・ADR-0311)。 */
  anyAbilityOption: "おまかせ(種族の全特性)",
  /** 結果の行・候補に、まとめた特性の名前を並べるときの区切り。 */
  abilityNameSeparator: "・",
  moveLabel: "技",
  /** 種族 select が未選択のとき、hidden の先頭 option に出す文言(空文字にしない。issue 304)。 */
  speciesPlaceholderOption: "ポケモンを選ぶ",
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

/** 計算画面の「詳細」(急所・やけど・天候・フィールド・防御側の壁・攻撃側と防御側のランク。issue 274、ADR-0312)の文言。iOS と同じ。 */
export const calcConditionsText = {
  toggleLabel: "詳細",
  criticalLabel: "急所",
  burnLabel: "やけど",
  weatherLabel: "天候",
  terrainLabel: "フィールド",
  screensLabel: "防御側の壁",
  ranksLabel: "攻撃側のランク",
  defenderRanksLabel: "防御側のランク",
  weather: { none: "なし", sun: "はれ", rain: "あめ", sand: "すなあらし", snow: "ゆき" },
  terrain: {
    none: "なし",
    electric: "エレキフィールド",
    grassy: "グラスフィールド",
    psychic: "サイコフィールド",
    misty: "ミストフィールド",
  },
  screens: { reflect: "リフレクター", lightScreen: "ひかりのかべ", auroraVeil: "オーロラベール" },
  rankUpLabel: "攻撃側のランクを上げる",
  rankDownLabel: "攻撃側のランクを下げる",
  defenderRankUpLabel: "防御側のランクを上げる",
  defenderRankDownLabel: "防御側のランクを下げる",
  /** ランクの増減ボタンの見た目の記号。 */
  rankUpSymbol: "+",
  rankDownSymbol: "-",
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

/**
 * issue 275: 防御側プリセット(domain/defenderPresets.ts、ADR-0009 §1 のカタログ)の表示名。
 * engine の Label(engine/bulk.go の DefenderPresetCatalog())と一字一句同じにする
 * (defenderPresets.contract.test.ts が一致を検査する)。
 */
export const defenderPresetText = {
  none: "無振り",
  hp: "H振り",
  hb_boost: "H振り+B補正",
  hb: "HB振り",
  hb_full: "HB特化",
  hd_boost: "H振り+D補正",
  hd: "HD振り",
  hd_full: "HD特化",
} as const;

/** アプリ全体(App.tsx)の文言。 */
export const appText = {
  title: "ポケモン ダメージ計算",
  loading: "読み込み中…",
  masterLoadError: "マスタデータの読み込みに失敗しました",
  /** issue 276: API 専用の画面(タイプバランス・判定)がオンラインのマスタを読めなかったときの案内。 */
  onlineMasterLoadError: "オンラインのマスタを読み込めませんでした。接続を確かめて、もう一度お試しください",
  /**
   * issue 308: マスタが読めないときの次の一手。自動でオフラインへ切り替えることはしない
   * (ADR-0301 §4 の既定方針)ので、画面から操作できるようにする。
   * 原因は握りつぶさず、受け取った Error の message をこの見出しに続けてそのまま出す
   * (fetch の失敗・HTTP エラーなど。凝った分類はしない)。
   */
  masterLoadErrorDetailLabel: "原因",
  masterLoadRetryLabel: "再試行",
  /** ADR-0313: オフラインでキャッシュが空(初回・未取得・破棄後)のときの案内。 */
  masterCacheEmptyError: "オフラインで使うには、一度オンラインで開いてマスタを取得してください",
  masterLoadSwitchToOfflineLabel: "オフラインに切り替える",
  /** 計算・逆算の切り替えタブ(P4-4、ADR-0300 §7)。 */
  tabsLabel: "画面の切り替え",
  /** サイト名(index.html の <title> と同じ。文書タイトルの接尾辞)。 */
  siteTitle: "pokecalc",
  calcTabLabel: "計算",
  reverseTabLabel: "逆算",
  /** P4-12a: タイプバランスのタブ(ADR-0303 §2)。 */
  balanceTabLabel: "タイプバランス",
  /** SP3: 素早さ比較のタブ(ADR-0604 §2)。 */
  speedTabLabel: "素早さ",
  /** JD5: 判定(抜けて倒せるか・返り討ちに遭うか)のタブ(ADR-0705 §1)。 */
  judgeTabLabel: "判定",
  /** P5-5 PR-A1: 構築ビルダーのタブ(ADR-0309 §1)。 */
  teamTabLabel: "構築",
  /** 計算モード(オフライン = WASM / オンライン = API)の切り替え(P4-5、ADR-0301 §4)。 */
  calcModeGroupLabel: "ダメージ計算の実行場所",
  calcModeOfflineLabel: "オフライン(WASM)",
  calcModeOnlineLabel: "オンライン(API)",
} as const;

/**
 * P4-16: オンラインのマスタ(pokedex-svc の公開 API。ADR-0304)で、公開 API の制約により使えない機能の案内と、
 * 種族の検索 UI の文言。画面は MasterCapabilities(master/types.ts)が false の機能について、
 * 操作を無効にしたうえでここの文言を添える(黙って空の選択肢を出さない。ADR-0304 §4)。
 */
export const masterOnlineText = {
  /**
   * 技の候補が1件も無いとき(= 技のセレクトが disabled のとき)に添える案内。
   * P4-17(ADR-0304 A-13)で表示条件が変わった: 以前は `capabilities.moves === false` で常に出していたが、
   * 技は種族の解決と一緒に届くようになったので、「攻撃側がまだ決まっていない・解決中・その種族が技を
   * 1つも覚えない」ときだけ出す。オンラインかどうかには触れない(画面はモードの名前を持たない。A-2)。
   */
  movesUnavailable: "技の候補がありません(ダメージを与える側のポケモンを選ぶと、覚える技が出ます)",
  /** capabilities.effects が false のとき、持ち物の候補比較(計算画面・逆算画面)に添える案内。 */
  itemCandidatesUnavailable: "オンラインでは持ち物の候補を比較できません(持ち物の効果データに未対応)",
  /** capabilities.speciesList が false のときの種族の検索欄のラベル。 */
  speciesSearchLabel: "ポケモンを名前で検索",
  /** 検索欄の補足(前方一致・1文字から)。 */
  speciesSearchHint: "日本語名の先頭の文字を入力すると候補が出ます",
  /** 入力前(空のクエリ)の案内。候補は出さない(空 = 全件にしない)。 */
  speciesSearchEmpty: "名前を入力してください",
  /** 前方一致で1件も無いとき。 */
  speciesSearchNoResult: "一致するポケモンがありません",
  /** 候補が上限(SPECIES_SEARCH_LIMIT)に達したとき、全件ではないことを明示する。 */
  speciesSearchTruncated: "候補が多いため一部だけ表示しています。名前をもう少し入力してください",
  /** 検索・種族の取得に失敗したとき(自動でオフラインには切り替えない。ADR-0301 §4)。 */
  speciesSearchFailed: "ポケモンの検索に失敗しました",
  /**
   * タイプバランスの画面が使えないときの案内。P4-17b(ADR-0304 A-14.1)で条件が変わった:
   * 以前は `capabilities.speciesList && capabilities.moves` が満たされないとき(= オンラインでは常に)
   * 出していたが、種族の検索欄と「種族と一緒に届く技」(A-13.3)がそろえばオンラインでも使えるので、
   * 今は**ポケモンか技を選ぶ口がそもそも無いとき**だけ出す。オンラインかどうかには触れない(A-2)。
   */
  balanceUnavailable:
    "タイプバランス診断を使えません(ポケモンか技のデータを取得できないため、オフラインで行ってください)",
} as const;

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
  speciesLabel: calcScreenText.pokemonFieldLabel,
  /** critic指摘(issue 304): 未選択の種族optionの文言も、画面をまたいで同じ語を参照する形にする。 */
  speciesPlaceholderOption: calcScreenText.speciesPlaceholderOption,
  abilityLabel: "特性",
  /** ポケモンを選ぶまで特性の候補が1件も無いとき、未選択の option に出す文言(issue 304)。 */
  abilityPlaceholderOption: `${calcScreenText.pokemonFieldLabel}を選ぶと選べます`,
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
 * SP3: speed API のクライアント(speed/speedClient.ts、ADR-0604 §3)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動の切り替え先は持たない)。
 */
export const speedClientText = {
  unavailable: "素早さの API に接続できません",
} as const;

/**
 * SP3: 素早さの表の行(PresetId。ADR-0601 §2、docs/speed-design.md §5)の表示名。
 * 表示名は契約に含めず、クライアントの文言資源が持つ(services/speed/api/openapi.yaml の PresetId の説明)。
 * MinimalPresetId(自分のポケモンで選べる3つ)も同じ語を使う。
 */
export const speedPresetText = {
  uninvested: "無振り",
  "neutral-max": "準速",
  max: "最速",
  "max-scarf": "最速スカーフ",
  "max-plus1": "最速+1",
  "max-plus2": "最速+2",
} as const;

/** SP3: 素早さ比較の画面(speed/SpeedScreen.tsx、ADR-0604 §4)の文言。 */
export const speedScreenText = {
  /** 左(速い順の全体の表)・右(自分のポケモン)の領域の名前(ADR-0604 §1)。 */
  tableRegionLabel: "素早さの表",
  selfRegionLabel: "自分のポケモン",
  /** 表がそろうまでの表示(左だけ。右の入力は先に使える。ADR-0604 §4)。 */
  loadingNotice: "読み込み中",
  /** 段の素早さの実数値。 */
  tierSpeedLabel: (speed: number): string => `素早さ ${String(speed)}`,
  /** 左の表の絞り込み(道具・ランク。ADR-0601 §4、docs/plan.md「SP: 素早さ比較」の確定仕様)。 */
  filterGroupLabel: "表の絞り込み",
  /** 絞り込みで最後の1つを外そうとしたとき(契約上、presets は1つ以上。ADR-0601 §4)。 */
  filterMinimumNotice: "少なくとも1つは選ぶ必要があります",
  /** 同じ段に2行以上あるとき(同速)のバッジ。右の結果の同速の一覧の見出しにも使う。 */
  tieLabel: "同速",
  /** 右の結果で同速の行が無いとき。 */
  noTieLabel: "同速なし",
  /** 左の表で、自分と同じ段を強調したときに読み上げる語。 */
  selfTierLabel: "自分と同速",
  /** 左の表で、自分の行が挟まる境界に引く印。 */
  selfBoundaryLabel: "ここに自分が入る",
  /** 行の中の区切り(「名前・調整」)。 */
  entrySeparator: "・",
  // ---- 右(自分のポケモン)の入力(ADR-0604 §4) ----
  modeGroupLabel: "入力の方法",
  modeLabel: { preset: "プリセット", custom: "カスタム", raw: "実数値" } as const,
  pokemonLabel: "ポケモン",
  /** ポケモンを選んでいないときの選択肢(raw では選ばなくてよい)。 */
  unselectedOption: "未選択",
  presetGroupLabel: "調整",
  scarfLabel: "こだわりスカーフ",
  spLabel: "素早さ SP",
  natureGroupLabel: "性格補正",
  natureLabel: { minus: "下降", neutral: "補正なし", plus: "上昇" } as const,
  rankLabel: "ランク",
  rawValueLabel: "実数値",
  // ---- 入力の範囲外(送信前に画面で止める。issue 307。判定画面 judgeScreenText と同じ言い回し) ----
  spRangeMessage: (max: number): string => `能力ポイントは0〜${String(max)}の整数で入力してください`,
  rankRangeMessage: (min: number, max: number): string =>
    `ランクは${String(min)}〜+${String(max)}の整数で入力してください`,
  /** 実数値の下限(契約の minimum: 1)。上限は speed サービスだけが式から導くので、画面では判定せず API の 400 を日本語にする。 */
  rawRangeMessage: "実数値は1以上の整数で入力してください",
  // ---- API エラー(サーバーの英語 message は出さず、code から日本語にする。issue 307) ----
  /** services/speed/api/openapi.yaml の ErrorCode と、Web 側の speed_unavailable に対応する。 */
  errorByCode: {
    invalid_request: "入力の形が正しくありません。値の範囲を確認してください",
    missing_header: "端末の識別情報が送られていません",
    invalid_header: "端末の識別情報の形が正しくありません",
    unknown_pokemon: "このポケモンはマスタにありません",
    request_too_large: "入力が大きすぎます",
    master_unavailable: "ポケモンのマスタを読み込めません",
    internal_error: "素早さの計算に失敗しました",
    speed_unavailable: "素早さの API に接続できません",
  } satisfies Readonly<Record<string, string>>,
  /** errorByCode に無い code のとき。 */
  errorFallback: "素早さの計算に失敗しました",
  // ---- 右(自分のポケモン)の結果(ADR-0604 §4) ----
  positionLoadingNotice: "位置を計算中",
  selfSpeedLabel: (speed: number): string => `実数値 ${String(speed)}`,
  fasterLabel: (rows: number): string => `自分より速い ${String(rows)}行`,
  slowerLabel: (rows: number): string => `自分より遅い ${String(rows)}行`,
  // ---- 場の状態・追い風・まひ・トリックルーム(ADR-0607) ----
  /** 左の表の場の状態のグループ(相手側の追い風・トリックルーム)。 */
  fieldGroupLabel: "場の状態",
  tableTailwindLabel: "追い風(相手側)",
  trickRoomLabel: "トリックルーム",
  /** 右の自分の追い風・まひ(preset / custom のみ)。 */
  selfTailwindLabel: "追い風(自分側)",
  paralysisLabel: "まひ",
  /** トリックルーム中の行動順の読み替え(速い = 後に動く、遅い = 先に動く。ADR-0607 §4)。 */
  movesBeforeLabel: (rows: number): string => `自分より先に動く ${String(rows)}行`,
  movesAfterLabel: (rows: number): string => `自分より後に動く ${String(rows)}行`,
} as const;

/**
 * JD5: judge API のクライアント(judge/judgeClient.ts、ADR-0705 §3)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動の切り替え先は持たない)。
 */
export const judgeClientText = {
  unavailable: "判定の API に接続できません",
} as const;

/**
 * JD5: 判定のエラーの見出し(ADR-0705 §8)。services/judge/api/openapi.yaml の ErrorCode と、
 * Web 側の judge_unavailable(judgeClient.ts)に 1 対 1 で対応する。
 * サーバーの message(どの候補で失敗したかが `defenders[<index>]` の形で入る。ADR-0703 §3)は
 * この見出しとは別に、補助の行として画面が出す。未知のコードは message だけを出す。
 */
export const judgeErrorText = {
  invalid_request: "入力の形が正しくありません",
  unknown_species: "このポケモンはマスタにありません",
  unknown_move: "この技の ID はマスタにありません",
  unknown_nature: "この性格はマスタにありません",
  request_too_large: "入力が大きすぎます",
  upstream_unavailable: "判定に必要なサービスに接続できません",
  internal_error: "判定に失敗しました",
  /** Web 側のコード(judgeClient.ts。通信できない・応答が読めない)。 */
  judge_unavailable: "判定の API に接続できません",
} as const;

/**
 * JD5: 判定の画面(judge/JudgeScreen.tsx、ADR-0705 §4・§6・§8)の文言。
 * 判定そのもの(素早さ・行動順・確定数)は judge-svc が返した値をそのまま出す。
 * 画面は「勝ち」「負け」に丸めた語を持たない(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
 */
export const judgeScreenText = {
  // ---- 領域(ADR-0705 §4) ----
  attackerRegionLabel: "自分のポケモン",
  defendersRegionLabel: "相手の候補",
  resultRegionLabel: "判定結果",
  // ---- 個体の入力。自分側と候補で同じ語を使う(候補は候補の group で絞り込む。ADR-0705 §4) ----
  speciesLabel: "ポケモン",
  natureLabel: "性格",
  abilityLabel: "特性",
  itemLabel: "持ち物",
  unselectedOption: "未選択",
  // ---- issue 309: 技はポケモンの覚える技から選ぶ。調整はプリセット。数値欄は「詳細」に畳む ----
  /** 技の select の名前(自分側・候補で共通。計算画面の calcScreenText.moveLabel と同じ語)。 */
  moveLabel: "技",
  /** 覚える技を1件も引けないとき(learnset が空・技の実体を解決できない)。技の select は disabled のまま。 */
  moveUnavailableNotice: "この種族の技を読み込めません",
  /** 数値の直接入力(SP6欄・ランク5欄)を畳む <details> の summary。 */
  detailsSummaryLabel: "詳細",
  /** 調整プリセットの radiogroup の名前(自分側・候補で共通。候補の group で絞り込む)。 */
  presetGroupLabel: "調整",
  /** 最速プリセット(S 全振り + 素早さ上昇の性格)の表示名。無振り・A特化は attackerPresetText、HB/HD特化は defenderPresetText から。 */
  fastestPresetLabel: "最速",
  /** 検証エラーの「どの体か」(自分側。候補は candidateGroupLabel(n) を使う)。 */
  attackerWhoLabel: "自分",
  spLabel: (stat: StatKey): string => `${statLetterJa[stat]} のポイント`,
  rankLabel: (stat: StatKey): string => `${statLetterJa[stat]} のランク`,
  formatLabel: "対戦形式",
  formatOption: { single: "シングル", double: "ダブル" } as const,
  // ---- 場の効果(speedField。ADR-0705 §6) ----
  speedFieldGroupLabel: "場の効果",
  trickRoomLabel: "トリックルーム",
  attackerTailwindLabel: "自分の側の追い風",
  defenderTailwindLabel: "相手の側の追い風",
  /** 相手側の追い風が候補ごとではない理由(ADR-0703 §5・ADR-0705 §6)。 */
  defenderTailwindNotice: "相手の側の追い風は、すべての相手候補に同じように適用されます",
  // ---- 相手候補の増減(ADR-0705 §4) ----
  candidateGroupLabel: (n: number): string => `相手候補${n}`,
  addCandidateLabel: "相手候補を追加",
  removeCandidateLabel: (n: number): string => `相手候補${n}を削除`,
  maxCandidatesNotice: (max: number): string => `相手候補は${max}件までです`,
  // ---- 送信(ADR-0705 §7) ----
  submitLabel: "判定する",
  loadingNotice: "判定中",
  emptyResultNotice: "「判定する」を押すと結果が出ます",
  // 送信前の検査(契約の範囲と同じ。違反していれば judge を呼ばずに理由を出す)。
  spRangeMessage: (max: number): string => `能力ポイントは0〜${max}の整数で入力してください`,
  spTotalMessage: (max: number): string => `能力ポイントの合計は${max}までです`,
  rankRangeMessage: "ランクは-6〜+6の整数で入力してください",
  requiredMessage: "ポケモン・性格・技をすべて選んでください",
  // ---- 結果(ADR-0705 §8)。judge の値をそのまま出す ----
  speedLabel: (attacker: number, defender: number): string => `素早さ ${attacker} 対 ${defender}`,
  priorityLabel: (attacker: number, defender: number): string => `優先度 ${attacker} 対 ${defender}`,
  outspeedsTrueLabel: "素早さで上回る",
  outspeedsFalseLabel: "素早さで下回る",
  /** 同速(outspeeds と同時に true にならない。真偽値1つに丸めない。ADR-0700 §6-1)。 */
  speedTieLabel: "同速",
  attackerMovesFirstLabel: "自分が先に動く",
  defenderMovesFirstLabel: "相手が先に動く",
  /** 優先度も素早さも同じで行動順が決まらないとき(ADR-0704 §2)。 */
  turnOrderTieLabel: "どちらが先に動くか決まらない",
  attackerKoLabel: "自分の技で相手を",
  defenderKoLabel: "相手の技で自分が",
  koGuaranteed: (hits: number): string => `確定${hits}発`,
  koRandom: (hits: number, percent: number): string => `乱数${hits}発(${percent}%)`,
  koNone: "倒せない",
} as const;

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

/**
 * 取り消した計算(REQUEST_ABORTED_CODE)の message(issue 113、ADR-0300 §11)。
 * 画面は取り消しをエラーとして出さないので利用者には見えないが、封筒の message を空にしない
 * (ログ・開発者ツールで理由が分かるようにする)。
 */
export const engineAbortText = {
  aborted: "新しい入力で計算を取り消しました",
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
 * P4-19(issue 110、ADR-0208): 候補・観測の件数上限(domain/requestLimits.ts)に当たったときの案内。
 * 計算画面と逆算画面が同じ文言を使う(件数は定数から渡し、文言に埋め込まない)。
 */
export const requestLimitText = {
  /** 持ち物候補を上限で絞り込んだとき。黙って切り捨てず、絞り込んだことを必ず出す。 */
  itemCandidatesTruncated: (max: number): string =>
    `持ち物の候補が多いため、先頭から${String(max)}通りまでで計算しています`,
  /** 観測が上限に達して「観測を追加」を無効にしたときの理由。 */
  observationLimitReached: (max: number): string =>
    `観測は${String(max)}件までです。追加するには、どれかの行を削除してください`,
} as const;

/**
 * 逆算画面(P4-4、ADR-0300 §7、ADR-0010 §R)の入力まわりの文言。
 * n を含む語は行番号(1始まり)から作る関数にする(観測は複数行あるため)。
 */
/** 逆算画面の領域(カード)の見える見出しの語(issue 304)。 */
const myRegionLabel = "自分";
const theirRegionLabel = "相手";

/**
 * 逆算の観測欄の説明(issue 304、docs/design.md「入力のラベル」)。単位(%/HP)と観測した側
 * (与えた = defender の HP が減る / 受けた = attacker の HP が減る)の組み合わせで文が変わる。
 */
function observationHintLabel(side: ReverseSide, unit: ObservationUnit): string {
  const target = side === "defender" ? theirRegionLabel : myRegionLabel;
  const amount = unit === "percent" ? "割合(%)" : "実数値(HP)";
  return `${target}の HP が減った${amount}`;
}

export const reverseScreenText = {
  sideGroupLabel: "観測したダメージ",
  sideDefenderLabel: "与えたダメージ",
  sideAttackerLabel: "受けたダメージ",
  myRegionLabel,
  theirRegionLabel,
  mySpeciesLabel: `${myRegionLabel}の${calcScreenText.pokemonFieldLabel}`,
  theirSpeciesLabel: `${theirRegionLabel}の${calcScreenText.pokemonFieldLabel}`,
  myItemLabel: `${myRegionLabel}の${calcScreenText.itemFieldLabel}`,
  myAbilityLabel: `${myRegionLabel}の${calcScreenText.abilityFieldLabel}`,
  theirAbilityLabel: `${theirRegionLabel}の${calcScreenText.abilityFieldLabel}`,
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
  observationHintLabel,
} as const;

/** 逆算の結果の表示(domain/reverseLabels.ts)の文言(ADR-0010 §R1・§R3)。 */
export const reverseResultText = {
  natureClassNeutral: "補正なし",
  /** 「関連ステータス上昇」の接尾辞(「B上昇」「C上昇」)。 */
  natureClassPlusSuffix: "上昇",
  closeCandidateLabel: "近い候補",
  /**
   * 観測を厳密に説明できる候補(exact)が1件も無いとき(exactCount 0 かつ候補が1件以上)に、
   * 結果の先頭へ出す案内(issue 305)。候補一覧自体は消さずに残す(要件「候補の提示を優先」)。
   */
  noExactCandidateNotice: "入力した観測を説明できる調整がありません(技・持ち物・入力値を確認)",
  /**
   * 全候補が観測と一致しないとき、各候補の SP 範囲に添える印(issue 305)。
   * 見た目だけでなくテキストとして出し、支援技術にも「参考値」であることが伝わるようにする。
   */
  referenceRangeLabel: "参考",
  /** %欄の意味(その候補で撃ったときの予測ダメージ%)を示すラベル(issue 305)。 */
  predictedPercentLabel: "予測",
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

/**
 * 「未対応」の印(ADR-0123、issue 271 / issue 270)の文言。engine が正しく計算できない技の機構・
 * 持ち物・特性に付く印で、数値は通常の式のまま返る(拒否しない)。画面は数値を消さず、
 * 「この結果は正しく計算できていない可能性がある」ことと、その原因(技・持ち物・特性のどれか)を示す。
 *
 * 文言・置き場所は iOS レーンの決定(docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、
 * ADR-0501「P6-17」)に揃える: 全行(全候補)に共通する印は結果の上に1回、残りはその行(候補)だけに出す。
 * 色だけに頼らない(design.md「画面: ダメージ計算」): 印は必ず文字(notice と markLabel)で出し、
 * アイコン・色は補助にする(アイコンは aria-hidden)。色は補足文と同じ text-secondary を使い、
 * danger・タイプ色は使わない(数値は通常の式の目安として出ておりエラーではないため)。
 */
const unsupportedTargetLabel: Record<UnsupportedTarget, string> = {
  move: "技",
  attacker_item: "攻撃側の持ち物",
  attacker_ability: "攻撃側の特性",
  defender_item: "防御側の持ち物",
  defender_ability: "防御側の特性",
};

/**
 * 印の理由(15 種)の説明。target のラベルに続けて読む短い語にし、技術用語(機構・スキーマ・engine)は出さない。
 * 正は ADR-0123 §2 の表と api/openapi.yaml の UnsupportedMark.reason。文言は iOS レーンの表記(DisplayLabels.swift、
 * DECISIONS.md 2026-09-25)に揃える。「特殊」はダメージ計算の特殊技分類と紛れるため、alt_offense_stat・
 * alt_defense_stat・effectiveness_change の文言には使わない(iOS critic 指摘 2026-09-25)。
 */
const unsupportedReasonLabel: Record<UnsupportedReason, string> = {
  multi_hit: "多段技",
  fixed_damage: "固定ダメージ",
  ohko: "一撃必殺",
  variable_power: "威力が変化",
  alt_offense_stat: "攻撃に使う能力値が通常と違う",
  alt_defense_stat: "防御に使う能力値が通常と違う",
  always_crit: "必ず急所",
  ignore_defense_ranks: "防御側のランク変化を無視",
  type_change: "タイプが変化",
  effectiveness_change: "相性の求め方が通常と違う",
  priority_change: "優先度が変化",
  field_specific: "天候・フィールドで変化",
  move_specific: "技固有の効果",
  zero_power: "威力が技の処理で決まる",
  unsupported_effect: "効果を計算に反映していない",
};

/** 契約に無い(古いクライアントが知らない)target・reason の汎用の語(ADR-0215)。 */
const unknownTargetLabel = "項目";
const unknownReasonLabel = "詳細は不明";

function isKnownKey<T extends string>(table: Record<T, string>, key: string): key is T {
  return Object.hasOwn(table, key);
}

export const unsupportedText = {
  unknownTarget: unknownTargetLabel,
  unknownReason: unknownReasonLabel,
  target: unsupportedTargetLabel,
  reason: unsupportedReasonLabel,
  /**
   * 印 1 件の文言(iOS レーンの書式に揃える)。`<対象>「<名前>」(<理由>)`。
   * name は ID を解決した表示名(マスタに無ければ空文字を渡す。そのとき ID をそのまま出す)。
   * reason が unsupported_effect のときは、理由の括弧を省く(「効果を計算に反映していない」は
   * 対象名だけで意味が通るため。iOS レーンの書式と同じ)。
   * 例: 技「テストれんぞくパンチ」(多段技) / 攻撃側の持ち物「テストどうぐ」
   */
  markLabel: (mark: UnsupportedMark, name: string): string => {
    // 契約は target・reason を enum にしない(ADR-0215)。未知の値は汎用の語で出し、ID は必ず出す。
    const targetLabel = isKnownKey(unsupportedTargetLabel, mark.target)
      ? unsupportedTargetLabel[mark.target]
      : unknownTargetLabel;
    const target = `${targetLabel}「${name === "" ? mark.id : name}」`;
    if (mark.reason === "unsupported_effect") return target;
    const reasonLabel = isKnownKey(unsupportedReasonLabel, mark.reason)
      ? unsupportedReasonLabel[mark.reason]
      : unknownReasonLabel;
    return `${target}(${reasonLabel})`;
  },
  /**
   * 全行(全候補)に共通する印がある結果の先頭に1回だけ置く案内(iOS レーンの書式)。
   * markLabels は markLabel で組み立て済みの印の文言(読点区切りで並べる)。
   */
  notice: (markLabels: readonly string[]): string =>
    `この結果は正確でない可能性があります(未対応: ${markLabels.join("、")})`,
  /**
   * 一部の行(候補)だけにある印を、その行・候補カードに出す文言(iOS レーンの書式)。
   * markLabels は markLabel で組み立て済みの印の文言(読点区切りで並べる)。
   */
  rowLabel: (markLabels: readonly string[]): string => `未対応: ${markLabels.join("、")}`,
} as const;

/** 計算結果の書式(domain/format.ts)で使う語。 */
export const resultText = {
  determinedPrefix: "確定",
  randomPrefix: "乱数",
  hitsSuffix: "発",
  cannotKO: "倒せない",
  // issue 334(付随項目): iOS の DisplayLabels.swift(「ばつぐん(×2)」のように倍率併記)に語を揃えた。
  // 「効果は」の接頭辞は外している(iOS 側に合わせ、Web からも申し送り済み)。
  effectivenessNone: "効果なし",
  effectivenessNotVery: (multiplier: number): string => `いまひとつ(×${multiplier})`,
  effectivenessNeutral: "等倍",
  effectivenessSuper: (multiplier: number): string => `ばつぐん(×${multiplier})`,
  moveCategory: { physical: "物理", special: "特殊", status: "変化" },
} as const;

/**
 * 「このアプリについて」(issue 328 / P6-18、ADR-0314)の文言。非公式の注記・データの出典4件は iOS の
 * PokeCalcCore.AboutText と一字一句同じ(正は docs/ai-shared/DECISIONS.md 2026-09-26「P6-18」と ADR-0002「責務の分離」表)。
 * 出典を増減するときは ADR-0002・ADR-0501「P6-18」・iOS と同時に直す。
 */
export const aboutText = {
  footerLinkLabel: "このアプリについて",
  pageHeading: "このアプリについて",
  unofficialHeading: "非公式表示",
  unofficialNotice:
    "このアプリは個人が私的に使うための非公式ツールです。" +
    "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。" +
    "ポケモン・Pokémon および関連する名称は各社の商標です。",
  dataSourcesHeading: "データの出典",
  dataSources: [
    { title: "ダメージ計算の検証", detail: "@smogon/calc(MIT License)" },
    { title: "ポケモン・技・習得技の照合", detail: "Pokémon Showdown(MIT License)" },
    { title: "日本語名・図鑑番号", detail: "PokeAPI" },
    { title: "使用可能なポケモン等の基準", detail: "Pokémon HOME・Pokémon Champions の公式情報" },
  ],
  backLabel: "計算に戻る",
} as const;
