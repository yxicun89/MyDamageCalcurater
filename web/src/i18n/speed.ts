// 素早さ比較(SP3・ADR-0604)の文言。ADR-0173 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

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
