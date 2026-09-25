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
  attackerPokemonLabel: `${attackerRegionLabel}の${pokemonFieldLabel}`,
  defenderPokemonLabel: `${defenderRegionLabel}の${pokemonFieldLabel}`,
  attackerItemLabel: `${attackerRegionLabel}の${itemFieldLabel}`,
  defenderItemLabel: `${defenderRegionLabel}の${itemFieldLabel}`,
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
  /**
   * issue 308: マスタが読めないときの次の一手。自動でオフラインへ切り替えることはしない
   * (ADR-0301 §4 の既定方針)ので、画面から操作できるようにする。
   * 原因は握りつぶさず、受け取った Error の message をこの見出しに続けてそのまま出す
   * (fetch の失敗・HTTP エラーなど。凝った分類はしない)。
   */
  masterLoadErrorDetailLabel: "原因",
  masterLoadRetryLabel: "再試行",
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
  /** 計算モード(オフライン = WASM / オンライン = API)の切り替え(P4-5、ADR-0301 §4)。 */
  calcModeGroupLabel: "計算モード",
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
  // ---- 右(自分のポケモン)の結果(ADR-0604 §4) ----
  positionLoadingNotice: "位置を計算中",
  selfSpeedLabel: (speed: number): string => `実数値 ${String(speed)}`,
  fasterLabel: (rows: number): string => `自分より速い ${String(rows)}行`,
  slowerLabel: (rows: number): string => `自分より遅い ${String(rows)}行`,
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
  moveIdLabel: "技の ID",
  /** 技を一覧から選べない理由(ADR-0304 §3 の既知の欠落。ADR-0705 §5)。 */
  moveIdHint: "技は ID で入力します(ID から技を引く API がまだありません)",
  unselectedOption: "未選択",
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
  requiredMessage: "ポケモン・性格・技の ID をすべて入力してください",
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
 * 色だけに頼らない(design.md「画面: ダメージ計算」): 印は必ず文字(badgeLabel と markLabel)で出し、
 * アイコン・色は補助にする(アイコンは aria-hidden)。
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
 * 正は ADR-0123 §2 の表と api/openapi.yaml の UnsupportedMark.reason。
 */
const unsupportedReasonLabel: Record<UnsupportedReason, string> = {
  alt_defense_stat: "ふだんと違う能力値で受ける",
  alt_offense_stat: "ふだんと違う能力値で攻撃する",
  always_crit: "必ず急所に当たる",
  effectiveness_change: "相性の決まり方が変わる",
  field_specific: "天候・フィールドで効果が変わる",
  fixed_damage: "ダメージが固定",
  ignore_defense_ranks: "相手の能力ランクを無視する",
  move_specific: "この技だけの特別な処理がある",
  multi_hit: "1回で何度も当たる",
  ohko: "一撃必殺",
  priority_change: "優先度が変わる",
  type_change: "タイプが変わる",
  variable_power: "威力が状況で変わる",
  zero_power: "威力が技の処理で決まる",
  unsupported_effect: "ダメージへの影響が未対応",
};

export const unsupportedText = {
  /** 印そのものの文字(バッジ)。色・アイコンだけにしない。 */
  badgeLabel: "未対応",
  /**
   * 印が1件でも出ている結果の先頭に置く案内(issue 305 の noExactCandidateNotice と同じ作法)。
   * 数値は消さずに残すので、「目安」であることを言う。
   */
  notice: "「未対応」の印が付いた結果は、正しく計算できていない可能性があります(数値は目安です)",
  /** 印をまとめた並びの accessible name(行・候補の中で何の並びかが分かるように)。 */
  listLabel: "未対応の内容",
  target: unsupportedTargetLabel,
  reason: unsupportedReasonLabel,
  /**
   * 印 1 件の文言。name は ID を解決した表示名(マスタに無ければ空文字を渡す。そのとき ID をそのまま出す)。
   * 例: 技「テストれんぞくパンチ」: 1回で何度も当たる
   */
  markLabel: (mark: UnsupportedMark, name: string): string =>
    `${unsupportedTargetLabel[mark.target]}「${name === "" ? mark.id : name}」: ${unsupportedReasonLabel[mark.reason]}`,
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
