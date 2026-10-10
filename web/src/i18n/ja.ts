// 画面の文言資源(日本語)。タイプの表示名はここに ID → 表示名で持つ(ADR-0300 §4)。
// タイプの一覧(どの ID が存在するか)は相性表のデータ(testdata/golden/typechart.json の types)が正であり、
// ここでは全 ID を過不足なく覆う対応表だけを持つ(コーディング規約 §2: マスタをコードに埋め込まない)。
// TypeId のユニオンは手書きの複製だが、相性表と過不足なく一致することを ja.test.ts が検査して同期を保つ
// (コーディング規約 §2 の「独立した検証」)。

import type { ObservationUnit } from "../domain/observations";
import type { ReverseSide, UnsupportedMark, UnsupportedReason, UnsupportedTarget } from "../engine/types";
import { abilityFieldLabel, itemFieldLabel, pokemonFieldLabel, speciesPlaceholderOption } from "./common";

// ADR-0323: レーン固有の文言はレーン別のファイルに置き、既存の import 先(i18n/ja)を変えないためにここで再エクスポートする。
// 新しいレーンは i18n/<レーン>.ts を作って画面から直接 import する(この一覧に行を足さない)。
export { statLetterJa } from "./common";
export * from "./balance";
export * from "./speed";
export * from "./judge";
export * from "./team";
export * from "./adjust";
export * from "./about";

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
// 入力欄の見えるラベルの語(pokemonFieldLabel・itemFieldLabel・abilityFieldLabel)は、レーン別の文言ファイルも使うので
// i18n/common.ts に置く(ADR-0323)。

/**
 * メガシンカの持ち物固定の文言(issue 515、ADR-0320、docs/mega-evolution-spec.md §4-3)。iOS は同じ語にそろえる。
 * 計算・逆算(構築の編集・判定も)で共通に使う。
 */
export const megaItemText = {
  /** メガ種族の持ち物欄が固定されている理由。 */
  lockedReason: "メガシンカするので、持ち物はメガストーンに決まっています",
  /** メガ種族だが、必要なメガストーンをマスタの持ち物から引けないときの理由。 */
  missingReason: "メガシンカに使うメガストーンが、データに見つかりません",
  /** 防御側がメガ種族のとき「持ち物の候補も比較」を使えない理由。 */
  compareDisabledReason:
    "防御側はメガシンカするので持ち物がメガストーンに決まっています。持ち物の候補は比べません",
  /** 持ち物欄を持たない相手のカードに出す、固定のメガストーンの表示。 */
  fixedItemName: (name: string): string => `持ち物: ${name}`,
  /** 構築の古い保存データ(メガ種族に別の持ち物)を読み込み時にストーンへ直したときの通知(PR-B。保存で永続化)。 */
  correctedNotice: (itemName: string): string =>
    `メガシンカのため持ち物を${itemName}に直しました。保存すると反映されます`,
  /** 同、ストーンがマスタに無いため持ち物を空にしたときの通知(黙って別の持ち物にしない)。 */
  clearedNotice:
    "メガシンカに使うメガストーンがデータに無いため、持ち物を空にしました。保存すると反映されます",
};

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
  anyAbilityOption: "おまかせ(すべての特性で計算)",
  /** 結果の行・候補に、まとめた特性の名前を並べるときの区切り。 */
  abilityNameSeparator: "・",
  moveLabel: "技",
  /** 種族 select が未選択のとき、hidden の先頭 option に出す文言(空文字にしない。issue 304)。 */
  speciesPlaceholderOption,
  noItemOption: "なし",
  noItemRowLabel: "持ち物なし",
  resultsListLabel: "計算結果",
  compareItemCandidatesLabel: "持ち物の候補も比較",
  swapButtonLabel: "攻守入れ替え",
  statusMoveNotice: "変化技はダメージを計算しません",
  noDamagingMovesNotice: "このポケモンはダメージを与える技を覚えないため、計算できません",
  /** 技セレクタの各行の区切り(「技名・分類・威力n」)。 */
  moveOptionSeparator: "・",
  /** 技セレクタの威力の前置き(「威力80」)。 */
  movePowerLabel: "威力",
  /** 入力が揃い calcBulk の応答待ちのときに出す文言(古い行を出さず、これに差し替える)。 */
  loadingNotice: "計算中",
} as const;

/** 計算画面の攻撃側の「攻撃」「特攻」の2ブロック(I-web-1・I-web-3、ADR-0329)の文言。iOS と同じ。 */
export const attackerStatText = {
  /** ブロックの見出し(攻撃 = atk、特攻 = spa)。 */
  statName: { atk: "攻撃", spa: "特攻" } as const,
  /** 選んだ技が使う側の見出しに足す語(色だけに頼らず、文字でも強調する)。 */
  usedSuffix: "(この技で使用)",
  /** プリセットのラジオグループの名前(「攻撃の調整」)。 */
  presetGroupLabel: (name: string) => `${name}の調整`,
  /** SP の数値入力の名前(「攻撃のSP」)。 */
  spLabel: (name: string) => `${name}のSP`,
  /** 性格補正のラジオグループの名前(「攻撃の性格補正」)。 */
  natureGroupLabel: (name: string) => `${name}の性格補正`,
  /** プリセットのどれとも一致しない値のときの印。 */
  custom: "カスタム",
  modifierLabel: { up: "上昇", neutral: "補正なし", down: "下降" } as const,
  /** SP が 0〜32 の整数でないときの明示エラー(計算しない)。 */
  spInvalid: (name: string) => `${name}のSPは0〜32の整数で入力してください`,
  /** 補正の組み合わせに当たる性格がマスタに無いときの明示エラー(計算しない)。 */
  natureUnresolved: "この性格補正の組み合わせに当たる性格が、データにありません",
  /** 攻撃と特攻を同じ向きにできない理由(選べない選択肢の説明)。 */
  sameDirectionReason: "攻撃と特攻の両方を上昇、または両方を下降にすることはできません",
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

/** 計算画面の「対戦の状態」(残り HP・多段の回数。ADR-0144 §3、I-web-13)。 */
export const battleStateText = {
  toggleLabel: "対戦の状態",
  /** 設定中の目印(閉じていても分かるように)。 */
  activeMark: "(設定中)",
  attackerHpLabel: "攻撃側の残りHP",
  attackerHpHint: "空なら満タンで計算します",
  defenderHpLabel: "防御側の残りHP",
  defenderHpHint: "割合(%)で入力します。確定数は残りHPから数えます(%表示は最大HPに対する値のまま)",
  percentUnit: "%",
  /** 横に出す「/ 最大」と割合。 */
  maxSuffix: (max: number): string => `/ ${max}`,
  hpError: (max: number): string => `1〜${max}で入力してください`,
  clamped: (max: number): string => `最大HP(${max})に合わせました`,
  hitsLabel: "回数",
  hitsDefault: (min: number, max: number): string => `既定(通常 ${min + 1} 回/スキルリンク等 ${max} 回)`,
  hitsOption: (count: number): string => `${count} 回`,
  /** 結果の近くに出す、前提にした状態。 */
  summary: (parts: readonly string[]): string => `対戦の状態: ${parts.join("・")}`,
  summaryAttacker: (hp: number, max: number): string => `攻撃側 HP ${hp}/${max}`,
  summaryDefender: (percent: number): string => `防御側 HP ${percent}%`,
  summaryHits: (count: number): string => `${count} 回`,
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
  masterLoadError: "ポケモンのデータを読み込めませんでした",
  /** issue 276: API 専用の画面(タイプバランス・判定)がオンラインのマスタを読めなかったときの案内。 */
  onlineMasterLoadError:
    "サーバーからポケモンのデータを読み込めませんでした。接続を確かめて、もう一度お試しください",
  /**
   * issue 308: マスタが読めないときの次の一手。自動でオフラインへ切り替えることはしない
   * (ADR-0301 §4 の既定方針)ので、画面から操作できるようにする。
   * 原因は握りつぶさず、受け取った Error の message をこの見出しに続けてそのまま出す
   * (fetch の失敗・HTTP エラーなど。凝った分類はしない)。
   */
  masterLoadErrorDetailLabel: "原因",
  masterLoadRetryLabel: "再試行",
  /** ADR-0313: オフラインでキャッシュが空(初回・未取得・破棄後)のときの案内。 */
  masterCacheEmptyError: "オフラインで使うには、一度オンラインで開いてポケモンのデータを取り込んでください",
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
  /** AJ6: 調整(指数・16n・SP 配分・最小 SP・技を覚えるポケモン)のタブ(ADR-0319 §1)。 */
  adjustTabLabel: "調整",
  /** 計算モード(オフライン = WASM / オンライン = API)の切り替え(P4-5、ADR-0301 §4)。 */
  calcModeGroupLabel: "計算する場所",
  calcModeOfflineLabel: "この端末(オフライン)",
  calcModeOnlineLabel: "サーバー(オンライン)",
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
 * P5-5c: 記録 API(record-svc)のクライアント(record/recordClient.ts、ADR-0317)の文言。
 * 失敗は画面に出さない(黙って非表示)ので、使うのはクライアントが返す Error.message だけ。
 */
export const recordClientText = {
  unavailable: "記録のサーバーに接続できません",
} as const;

/** P5-5c: 計算画面の「よく計算する相手」チップ(ADR-0317)。 */
export const frequentOpponentsText = {
  groupLabel: "よく計算する相手",
} as const;
/**
 * API 実装(createApiEngine、P4-5)がクライアント側(fetch する前・応答を読めないとき)で作るエラーの文言
 * (ADR-0301 §2・§4)。サーバーが返す Error.message はそのまま運ぶので、ここには含まない。
 */
export const apiEngineText = {
  /** 個体の性格(plus/minus の組)に一致するマスタの性格が無いとき。 */
  unknownNature: "この性格がデータに見つかりません",
  /** 通信できない・応答が読めない・エラー本文の形が不正なとき(ADR-0301 §4: 自動フォールバックはしない)。 */
  unavailable: "サーバーに接続できません",
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
    `ダメージは${String(max)}件まで入力できます。追加するには、どれかの行を削除してください`,
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
  sideGroupLabel: "どちらのダメージ",
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
  observationLabel: (n: number): string => `ダメージ${String(n)}`,
  observationUnitGroupLabel: (n: number): string => `ダメージ${String(n)}の単位`,
  removeObservationLabel: (n: number): string => `ダメージ${String(n)}を削除`,
  addObservationLabel: "ダメージを追加",
  percentUnitLabel: "%",
  damageUnitLabel: "HP",
  percentInvalidMessage: "1〜100 の整数で入力してください",
  damageInvalidMessage: "1 以上の整数で入力してください",
  resultsListLabel: "考えられる振り方",
  observationHintLabel,
} as const;

/** 逆算の結果の表示(domain/reverseLabels.ts)の文言(ADR-0010 §R1・§R3)。 */
export const reverseResultText = {
  natureClassNeutral: "補正なし",
  /** 「関連ステータス上昇」の接尾辞(「B上昇」「C上昇」)。 */
  natureClassPlusSuffix: "上昇",
  closeCandidateLabel: "ほぼ合う候補",
  /**
   * 観測を厳密に説明できる候補(exact)が1件も無いとき(exactCount 0 かつ候補が1件以上)に、
   * 結果の先頭へ出す案内(issue 305)。候補一覧自体は消さずに残す(要件「候補の提示を優先」)。
   */
  noExactCandidateNotice:
    "入力したダメージにぴったり合う振り方が見つかりません(技・持ち物・入力した値を確かめてください)",
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
