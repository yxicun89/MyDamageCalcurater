// 素早さ比較(SP3・ADR-0604。I-speed-4b で ADR-0610 の画面に作り直し)の文言。ADR-0323 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。やさしい言葉で書く(F-13。「族」「準速」などの語は画面の中で説明する)。

/**
 * SP3: speed API のクライアント(speed/speedClient.ts、ADR-0604 §3)の文言。
 * 通信できない・応答が読めない・エラー本文の形が不正なとき(自動の切り替え先は持たない)。
 */
export const speedClientText = {
  unavailable:
    "素早さのサーバーに接続できません。起動中か、通信が切れている可能性があります。しばらくしてからもう一度お試しください",
} as const;

/**
 * SP3: 素早さの 6 つの振り方(PresetId。ADR-0601 §2、docs/speed-design.md §5)の表示名。
 * 表示名は契約に含めず、クライアントの文言資源が持つ(services/speed/api/openapi.yaml の PresetId の説明)。
 * 画面では相手の想定のチップと、「これ以上」の行の名前に使う。
 */
export const speedPresetText = {
  uninvested: "無振り",
  "neutral-max": "準速",
  max: "最速",
  "max-scarf": "最速スカーフ",
  "max-plus1": "最速+1",
  "max-plus2": "最速+2",
} as const;

/** 想定の振り方(PresetId)の型。 */
type PresetKey = keyof typeof speedPresetText;
/** 性格補正(NatureId)の型。 */
type NatureKey = "minus" | "neutral" | "plus";
/** 素早さだけで比べた行動順(TurnOrder)の型。 */
type TurnOrderKey = "self_first" | "opponent_first" | "speed_tie";

/** 性格補正の表示名。 */
const natureLabels = { minus: "下降", neutral: "補正なし", plus: "上昇" } as const satisfies Record<
  NatureKey,
  string
>;

/** SP の区間の文字。1 点なら「S20」、幅があれば「S20〜24」。 */
function spRangeText(min: number, max: number): string {
  return min === max ? `S${String(min)}` : `S${String(min)}〜${String(max)}`;
}

/** 族の区間の文字。1 つなら「118 族」、幅があれば「118〜120 族」。 */
function baseRangeText(min: number, max: number): string {
  return min === max ? `${String(min)} 族` : `${String(min)}〜${String(max)} 族`;
}

/**
 * 素早さ比較の画面(speed/SpeedScreen.tsx、ADR-0604 §4 → ADR-0610 §4)の文言。
 * 画面の構成: 入力(自分 / 相手 / 場)→ 出力 1「上を取る最低ライン」→ 出力 2「これ以上」。表は出さない。
 */
export const speedScreenText = {
  // ---- 領域の名前(region の aria-label と h2 の文字を同じにする) ----
  selfRegionLabel: "自分のポケモン",
  opponentRegionLabel: "相手のポケモン",
  fieldGroupLabel: "場の状態",
  /** 画面の使い方の一言(ポケモンを選ぶまで出す)。 */
  introNotice: "自分と相手のポケモンを選ぶと、相手より先に動くために必要な素早さの振り方を表示します",
  /** ポケモンの一覧を待っている間。 */
  loadingNotice: "読み込み中",
  pokemonListLoadingNotice: "ポケモンの一覧を読み込み中",
  unselectedOption: "未選択",
  selfPokemonLabel: "自分のポケモンを選ぶ",
  opponentPokemonLabel: "相手のポケモンを選ぶ",
  // ---- 自分の入力 ----
  rankLabel: "ランク",
  scarfLabel: "こだわりスカーフ",
  selfTailwindLabel: "追い風(自分側)",
  selfParalysisLabel: "まひ(自分)",
  /** 「今の振り」(任意。入れると先に動けるか・「これ以上」が出る)。 */
  currentToggleLabel: "今の振りも入れる",
  currentHint: "入れると、今の素早さで先に動けるかと、「これ以上」の一覧が出ます",
  spLabel: "素早さ SP",
  natureGroupLabel: "性格補正",
  natureLabel: natureLabels,
  // ---- 相手の入力 ----
  /** 想定 = 相手がどこまで素早さを振っているか(普通は分からないので並べて見せる)。 */
  assumptionGroupLabel: "相手の振り方の想定",
  assumptionHint: "相手がどこまで素早さを振っているかは分からないので、いくつか並べて見ます",
  assumptionMinimumNotice: "想定は少なくとも1つ選びます",
  opponentTailwindLabel: "追い風(相手側)",
  opponentParalysisLabel: "まひ(相手)",
  // ---- 場 ----
  trickRoomLabel: "トリックルーム",
  trickRoomHint: "トリックルーム中は、素早さが低いほうが先に動きます",
  // ---- 出力 1: 上を取る最低ライン ----
  matchupHeading: "先に動くための最低ライン",
  matchupHeadingTrickRoom: "先に動ける上限(トリックルーム中)",
  /** 想定ごとのカードの見出し。「最速 ・ 素早さ 200」。 */
  cardHeading: (preset: PresetKey, opponentSpeed: number): string =>
    `${speedPresetText[preset]} ・ 素早さ ${String(opponentSpeed)}`,
  /** カスタムの想定(API の custom)のカード見出し。 */
  customCardHeading: (opponentSpeed: number): string => `カスタム ・ 素早さ ${String(opponentSpeed)}`,
  /** カードの一言見出し(補正なしの最小 SP から導く。ADR-0610 §1)。 */
  headlineUninvested: "無振りで抜ける",
  headlineNeutralMax: "準速で抜ける",
  headlinePlusOnly: (min: number): string => `最速なら S${String(min)} 以上で抜ける`,
  /** トリックルーム中の一言見出し(上限の SP から導く)。 */
  headlineTrickRoom: (max: number): string => `S${String(max)} まで振っても先に動ける`,
  /** どの性格でも上を取れない(同速だけの場合も含む)。 */
  unreachableNotice: "どう振っても上を取れません",
  unreachableTrickRoomNotice: "どう振っても先に動けません",
  /** 性格ごとの行。 */
  natureLineLabel: (nature: NatureKey): string => `性格補正 ${natureLabels[nature]}`,
  /** 通常の場: 最小 SP(0 なら「振らなくても」)。 */
  movesFirstMin: (min: number): string => (min === 0 ? "振らなくても抜ける" : `S${String(min)} 以上で抜ける`),
  /** トリックルーム中: 上限 SP。 */
  movesFirstMax: (max: number): string => `S${String(max)} まで振っても先に動ける`,
  cannotMoveFirst: "上を取れません",
  cannotMoveFirstTrickRoom: "先に動けません",
  /** 同速は上を取るに含めず、別に出す。 */
  tieAt: (min: number, max: number): string => `${spRangeText(min, max)} で同速`,
  tieLabel: "同速",
  /** 今の振りがあるときの行動順のバッジ(色だけに頼らず文字で言う)。 */
  currentOrderLabel: {
    self_first: "今の振りなら先に動く",
    speed_tie: "今の振りなら同速",
    opponent_first: "今の振りなら後に動く",
  } as const satisfies Record<TurnOrderKey, string>,
  /** 自分の今の素早さ(実数値)。 */
  selfSpeedLabel: (speed: number): string => `自分の素早さ ${String(speed)}`,
  matchupLoadingNotice: "最低ラインを計算中",
  // ---- 出力 2: これ以上 ----
  reachHeading: "これ以上",
  reachIntro:
    "今の素早さで、素早さの基本値がどこまでのポケモンを抜けるかの一覧です(「100 族」は、素早さの基本値が 100 のポケモンのことです)",
  reachNeedsCurrent: "「今の振りも入れる」をオンにすると、ここに「これ以上」の一覧が出ます",
  reachLoadingNotice: "「これ以上」を計算中",
  /** 通常の場。「準速 117 族まで抜ける」「準速 118 族と同速」「準速 119 族以上に抜かれる」。 */
  reachSlower: (preset: PresetKey, max: number): string =>
    `${speedPresetText[preset]} ${String(max)} 族まで抜ける`,
  reachTie: (preset: PresetKey, min: number, max: number): string =>
    `${speedPresetText[preset]} ${baseRangeText(min, max)}と同速`,
  reachFaster: (preset: PresetKey, min: number): string =>
    `${speedPresetText[preset]} ${String(min)} 族以上に抜かれる`,
  /** トリックルーム中は意味が反転する(素早さの比較は変えず、読み方だけ変える。ADR-0610 §1)。 */
  reachSlowerTrickRoom: (preset: PresetKey, max: number): string =>
    `${speedPresetText[preset]} ${String(max)} 族までは後に動く`,
  reachFasterTrickRoom: (preset: PresetKey, min: number): string =>
    `${speedPresetText[preset]} ${String(min)} 族以上には先に動ける`,
  reachNoneSlower: "抜ける族はありません",
  reachNoneFaster: "抜かれる族はありません",
  reachNoneSlowerTrickRoom: "後に動く族はありません",
  reachNoneFasterTrickRoom: "先に動ける族はありません",
  /** 該当する使用可能なポケモンの数。 */
  reachCount: (n: number): string => `${String(n)} 体`,
  /** 境界の例(遅い側で最も速い 1 体 / 速い側で最も遅い 1 体)。 */
  reachExample: (nameJa: string): string => `例: ${nameJa}`,
  // ---- 連続技へのリンク(連続技の画面ができるまで出さない。ADR-0610 §4・§結果) ----
  comboLinkLabel: "この相手で連続技を計算",
  // ---- API エラー(サーバーの英語 message は出さず、code から日本語にする。issue 307) ----
  /** services/speed/api/openapi.yaml の ErrorCode と、Web 側の speed_unavailable に対応する。 */
  errorByCode: {
    invalid_request: "入力の形が正しくありません。値の範囲を確認してください",
    missing_header: "端末の情報が送られていません。ページを開き直してください",
    invalid_header: "端末の情報が正しくありません。ページを開き直してください",
    unknown_pokemon: "このポケモンはデータにありません",
    request_too_large: "入力が大きすぎます",
    master_unavailable: "ポケモンのデータを読み込めません",
    internal_error: "素早さの計算に失敗しました",
    upstream_unavailable:
      "素早さのサーバーが起動中か、一時的に使えません。しばらくしてからもう一度お試しください",
    speed_unavailable:
      "素早さのサーバーに接続できません。起動中か、通信が切れている可能性があります。しばらくしてからもう一度お試しください",
    /** gateway が上流(speed)へ届かないときの 503 は上の upstream_unavailable(契約外。ADR-0357 の次の PR で文言を原因が分かる形に)。 */
  } satisfies Readonly<Record<string, string>>,
  /** errorByCode に無い code のとき。 */
  errorFallback: "素早さの計算に失敗しました",
} as const;
