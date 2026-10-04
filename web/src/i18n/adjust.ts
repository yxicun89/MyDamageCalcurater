// 調整(AJ6・ADR-0319)の文言。ADR-0323 で i18n/ja.ts から移した(ja.ts が再エクスポートする)。
// このレーンの文言はこのファイルにだけ足す(ja.ts は触らない)。

import { STAT_ORDER } from "../domain/requests";
import type { StatKey } from "../engine/types";
import { statLetterJa } from "./common";

/**
 * AJ6: 調整 API のクライアント(adjust/adjustClient.ts)が作る文言(ADR-0319 §3。judgeClientText と同じ形)。
 * サーバーが返す `Error.message`(英語の内部メッセージを含みうる)は画面に出さない(ADR-0411 §3 と同じ)。
 */
export const adjustClientText = {
  unavailable: "調整の API に接続できません",
  /** 画面が新しい送信・画面を閉じたことで取り消した呼び出し(REQUEST_ABORTED_CODE)。画面には出さない。 */
  aborted: "新しい入力で調整を取り消しました",
} as const;

/**
 * AJ6: 調整の画面に出すエラーの文言(ADR-0319 §6)。キーは api/openapi.yaml の ErrorCode と、
 * Web 側の adjust_unavailable(adjustClient.ts)。サーバーの message は出さず、コードからこの文言を引く。
 * 未知のコードは fallback(adjust/adjustFormat.ts の adjustErrorMessage が引き分ける)。
 */
export const adjustErrorText = {
  invalid_json: "入力の形が正しくありません",
  unknown_field: "入力の形が正しくありません",
  invalid_enum: "選んだ項目の値が正しくありません",
  invalid_input: "入力の値が範囲の外です。能力ポイント・上限・発数・確率を確かめてください",
  unknown_species: "このポケモンはマスタにありません",
  unknown_move: "この技はマスタにありません",
  unknown_nature: "この性格はマスタにありません",
  unknown_item: "この持ち物はマスタにありません",
  unknown_ability: "この特性はマスタにありません",
  not_found: "見つかりませんでした。入力を確かめてください",
  missing_header: "端末の識別子を送れませんでした。ページを読み込み直してください",
  invalid_header: "端末の識別子を送れませんでした。ページを読み込み直してください",
  type_chart_missing: "タイプ相性表を読み込めていません。しばらくしてからお試しください",
  master_unavailable: "マスタの準備ができていません。しばらくしてからお試しください",
  upstream_unavailable: "調整に必要なサービスに接続できません",
  /** Web 側のコード(adjustClient.ts。通信できない・応答が読めない)。 */
  adjust_unavailable: "調整の API に接続できません",
  /** 上のどれにも当たらないコード。 */
  fallback: "調整に失敗しました",
} as const;

/** AJ6: 調整のモード(ADR-0319 §2)。画面の state と文言のキー。 */
export type AdjustModeKey = "goals" | "indices" | "bulk" | "offense" | "minKo" | "minSurvive";

/** F-11(ADR-0331): 目標の種類(api/openapi.yaml の AdjustGoalKind と同じ値)。 */
export type AdjustGoalKindKey = "outspeed" | "survive" | "ko";

/** F-11(ADR-0331 §5): 素早さのプリセット(adjust/adjustGoals.ts の SpeedPresetKey と同じ値)。 */
type AdjustSpeedPresetKey = "fastest" | "neutral_max" | "none";

/** 「先に使う技」で自分の素早さのランクが変わったときの文の前置き(ADR-0331 §7)。 */
function speedBoostPrefix(boost: { readonly moveName: string; readonly rank: number } | null): string {
  if (boost === null) {
    return "";
  }
  if (boost.rank > 0) {
    return `${boost.moveName}で素早さが${boost.rank}段階上がったあと、`;
  }
  if (boost.rank < 0) {
    return `${boost.moveName}で素早さが${-boost.rank}段階下がったあと、`;
  }
  return `${boost.moveName}では素早さは上がりません。`;
}

/** AJ6: HP のライン(api/openapi.yaml の HPLineKind と同じ値)。 */
type AdjustHpLineKind = "none" | "16n" | "16n-1";

/** 6能力の数値(SP・実数値)を「H 4 / A 0 / …」の1行にする。 */
function statsLine(values: Readonly<Record<StatKey, number>>): string {
  return STAT_ORDER.map((stat) => `${statLetterJa[stat]} ${values[stat]}`).join(" / ");
}

/**
 * AJ6: 調整の画面(adjust/AdjustScreen.tsx、ADR-0319)の文言。
 * 見える見出し・ラベルの語と accessible name は同じ語から組み立てる(docs/design.md「入力のラベル」・issue 304)。
 * 欄の accessible name は「<領域の見出しの語>の<ラベルの語>」(例「自分」+「ポケモン」→「自分のポケモン」)。
 */
export const adjustScreenText = {
  // ---- 領域(h2 の見出しと region の名前。ADR-0319 §2) ----
  selfRegionLabel: "自分",
  modeRegionLabel: "調整の内容",
  opponentRegionLabel: "相手",
  goalRegionLabel: "目標",
  resultRegionLabel: "調整の結果",
  learnersRegionLabel: "この技を覚えるポケモン",
  // ---- 欄の見えるラベル(短い語)と、組み立て済みの accessible name ----
  speciesFieldLabel: "ポケモン",
  natureFieldLabel: "性格",
  abilityFieldLabel: "特性",
  itemFieldLabel: "持ち物",
  moveFieldLabel: "技",
  presetFieldLabel: "調整",
  selfSpeciesLabel: "自分のポケモン",
  selfNatureLabel: "自分の性格",
  selfAbilityLabel: "自分の特性",
  selfItemLabel: "自分の持ち物",
  selfMoveLabel: "自分の技",
  opponentSpeciesLabel: "相手のポケモン",
  opponentPresetLabel: "相手の調整",
  opponentMoveLabel: "相手の技",
  /** 未選択の select の先頭に出す文言(空の表示にしない。issue 304)。 */
  speciesPlaceholder: "ポケモンを選ぶ",
  naturePlaceholder: "性格を選ぶ",
  movePlaceholder: "技を選ぶ",
  unselectedOption: "未選択",
  // ---- 固定する SP(下限。ADR-0150 §8・ADR-0319 §2) ----
  fixedSpGroupLabel: "固定する能力ポイント",
  fixedSpLabel: (stat: StatKey): string => `${statLetterJa[stat]} の固定ポイント`,
  fixedSpTotal: (total: number, max: number): string => `合計 ${total} / ${max}`,
  fixedSpHint: "ここで決めた値より下には振りません。残りを調整に回します",
  // ---- モード(ADR-0319 §2) ----
  modeGroupLabel: "調整の内容",
  modeLabel: {
    goals: "目標から振り方を決める",
    indices: "指数と 16n を見る",
    bulk: "耐久に振る",
    offense: "攻撃と素早さに振る",
    minKo: "倒せる最小の振り方",
    minSurvive: "耐えられる最小の振り方",
  } satisfies Record<AdjustModeKey, string>,
  // 耐久側(bulk)
  focusLabel: "耐久の基準",
  focusOption: { physical: "物理(H×B)", special: "特殊(H×D)", both: "物理と特殊の両方" } as const,
  ceilingGroupLabel: "振ってよい上限",
  ceilingLabel: (stat: StatKey): string => `${statLetterJa[stat]} の上限`,
  // 攻撃側(offense)
  offenseCategoryLabel: "攻撃の分類",
  offenseCategoryOption: { physical: "物理(A)", special: "特殊(C)" } as const,
  minSpeedLabel: "素早さの目標(実数値)",
  minSpeedHint: "この実数値以上になるように S に振ります。空なら目標なし",
  /** 耐久側・攻撃側で、目標(相手と発数)を足すかどうか。 */
  useGoalLabel: "目標を指定する",
  // ---- 目標(ADR-0319 §2。数値入力をさせずプリセットから選ぶ) ----
  hitsLabel: "発数",
  hitsOption: (hits: number): string => `${hits}発`,
  thresholdLabel: "確率",
  thresholdOption: (percent: number): string => (percent === 100 ? "確定(100%)" : `${percent}% 以上`),
  // ---- 送信(ADR-0319 §4) ----
  submitLabel: "調整する",
  loadingNotice: "計算中",
  emptyResultNotice: "「調整する」を押すと結果が出ます",
  // ---- 送信前の検査(ADR-0319 §4。違反していれば API を呼ばずに理由を出す) ----
  selfRequiredMessage: "自分のポケモンと性格を選んでください",
  selfMoveRequiredMessage: "自分の技を選んでください",
  opponentRequiredMessage: "相手のポケモンを選んでください",
  opponentMoveRequiredMessage: "相手の技を選んでください",
  spRangeMessage: (max: number): string => `能力ポイントは0〜${max}の整数で入力してください`,
  spTotalMessage: (max: number): string => `能力ポイントの合計は${max}までです`,
  ceilingBelowFixedMessage: "上限は固定する能力ポイント以上にしてください",
  categoryMismatchMessage: "攻撃の分類と自分の技の分類をそろえてください",
  minSpeedMessage: "素早さの目標は0以上の整数で入力してください",
  natureNotFoundMessage: "相手の調整に合う性格がマスタにありません",
  // ---- 結果: 指数と 16n(adjustIndices) ----
  indicesHeading: "今の振り方の指数",
  statsLine: (stats: Readonly<Record<StatKey, number>>): string => `実数値 ${statsLine(stats)}`,
  firepowerIndexLabel: "火力指数",
  firepowerIndexNone: "技を選ぶと出します",
  physicalBulkLabel: "物理耐久指数",
  specialBulkLabel: "特殊耐久指数",
  indexLine: (label: string, value: number | string): string => `${label} ${value}`,
  /** 指数に含める補正の範囲(ADR-0319 §5)。 */
  indexNote: "火力指数の補正はタイプ一致だけを含めます(持ち物・特性・テラスタルは含めません)",
  hpLineHeading: "HP の 16n",
  hpLineKindLabel: {
    none: "16n でも 16n-1 でもない",
    "16n": "16n",
    "16n-1": "16n-1",
  } satisfies Record<AdjustHpLineKind, string>,
  hpCurrent: (hp: number, kindLabel: string): string => `HP ${hp}(${kindLabel})`,
  next16nLabel: "次の 16n",
  prev16nLabel: "前の 16n",
  next16nMinus1Label: "次の 16n-1",
  prev16nMinus1Label: "前の 16n-1",
  hpLinePoint: (label: string, hp: number, sp: number, spDelta: number): string =>
    `${label}: HP ${hp}(H ${sp}、${spDelta > 0 ? "+" : ""}${spDelta})`,
  hpLineNone: (label: string): string => `${label}: なし`,
  // ---- 結果: 最小 SP(adjustMinSpToKo / adjustMinSpToSurvive)。chance は formatChancePercent 済みの文字 ----
  koFeasible: (stat: StatKey, sp: number, hits: number, chance: string): string =>
    `${statLetterJa[stat]} に ${sp} 振れば ${hits}発で倒せます(確率 ${chance})`,
  koInfeasible: (stat: StatKey, sp: number, hits: number, chance: string): string =>
    `${statLetterJa[stat]} に ${sp} 振っても ${hits}発では倒せません(確率 ${chance})`,
  surviveFeasible: (stat: StatKey, hpSp: number, statSp: number, hits: number, chance: string): string =>
    `H に ${hpSp}・${statLetterJa[stat]} に ${statSp} 振れば ${hits}発耐えます(確率 ${chance})`,
  surviveInfeasible: (stat: StatKey, hpSp: number, statSp: number, hits: number, chance: string): string =>
    `H に ${hpSp}・${statLetterJa[stat]} に ${statSp} 振っても ${hits}発は耐えられません(確率 ${chance})`,
  // ---- 結果: 配分の提案(adjustAllocation) ----
  remainingLabel: (remaining: number): string => `残りの能力ポイント ${remaining}`,
  maxIndexHeading: "指数が最大になる振り方",
  minSpHeading: "目標を満たす最小の振り方",
  minSpNotRequested: "目標を指定すると、目標を満たす最小の振り方も出します",
  planSpLine: (sp: Readonly<Record<StatKey, number>>): string => `能力ポイント ${statsLine(sp)}`,
  planTotal: (total: number): string => `合計 ${total}`,
  goalMet: (chance: string): string => `目標を満たします(確率 ${chance})`,
  goalNotMet: (chance: string): string => `目標に届きません(確率 ${chance})`,
  speedMet: "素早さの目標を満たします",
  speedNotMet: "素早さの目標に届きません",
  // ---- 目標から振り方を決める(F-11・ADR-0331 §5〜§7)。領域の見出しは goalRegionLabel(「目標」) ----
  goalCardLegend: (n: number): string => `目標 ${n}`,
  /** 目標のカードの欄の accessible name(「目標 n の<見えるラベル>」。SC 2.5.3)。 */
  goalFieldName: (n: number, visibleLabel: string): string => `目標 ${n} の${visibleLabel}`,
  addGoalLabel: "目標を追加",
  goalLimitHint: (max: number): string => `目標は ${max} つまでです`,
  noGoalsNotice: "目標を追加してください",
  removeGoalLabel: "外す",
  removeGoalName: (n: number): string => `目標 ${n} を外す`,
  goalKindFieldLabel: "種類",
  goalKindOption: {
    outspeed: "素早さを上回る",
    survive: "この技を耐える",
    ko: "この技で倒す",
  } satisfies Record<AdjustGoalKindKey, string>,
  goalOpponentSpeciesFieldLabel: "相手のポケモン",
  goalPresetFieldLabel: "相手の振り方",
  goalOpponentMoveFieldLabel: "相手の技",
  goalSelfMoveFieldLabel: "自分の技",
  goalBoostMoveFieldLabel: "先に使う技",
  goalBoostMoveNone: "使わない",
  goalBoostMoveHint: "ニトロチャージのように自分の素早さが上がる技を選ぶと、上がったあとの素早さで比べます",
  speedPresetOption: {
    fastest: "最速",
    neutral_max: "準速",
    none: "無振り",
  } satisfies Record<AdjustSpeedPresetKey, string>,
  // 送信前の検査(ADR-0331 §6)
  goalsRequiredMessage: "目標を追加してください",
  goalOpponentRequiredMessage: (n: number): string => `目標 ${n}: 相手のポケモンを選んでください`,
  goalOpponentMoveRequiredMessage: (n: number): string => `目標 ${n}: 相手の技を選んでください`,
  goalSelfMoveRequiredMessage: (n: number): string => `目標 ${n}: 自分の技を選んでください`,
  goalNatureNotFoundMessage: (n: number): string => `目標 ${n}: 相手の振り方に合う性格がマスタにありません`,
  // 結果(ADR-0331 §7)。chance は formatChancePercent 済みの文字
  goalsPlanHeading: "目標をすべて満たす振り方",
  goalsNearestHeading: "目標に一番近い振り方",
  goalsInfeasibleNotice: "すべての目標は満たせませんでした",
  goalOutcomesLabel: "目標ごとの結果",
  goalOpponentName: (speciesName: string, presetLabel: string): string => `${speciesName}(${presetLabel})`,
  outspeedOutcome: (
    n: number,
    opponent: string,
    met: boolean,
    selfSpeed: number,
    opponentSpeed: number,
    boost: { readonly moveName: string; readonly rank: number } | null,
  ): string =>
    `目標 ${n}: ${speedBoostPrefix(boost)}${opponent}より先に${met ? "動けます" : "は動けません"}(自分 ${selfSpeed} / 相手 ${opponentSpeed})`,
  surviveOutcome: (
    n: number,
    opponent: string,
    moveName: string,
    hits: number,
    met: boolean,
    chance: string,
  ): string =>
    met
      ? `目標 ${n}: ${opponent}の${moveName}を${hits}発耐えます(耐える確率 ${chance})`
      : `目標 ${n}: ${opponent}の${moveName}を${hits}発は耐えられません(耐える確率 ${chance})`,
  koOutcome: (
    n: number,
    opponent: string,
    moveName: string,
    hits: number,
    met: boolean,
    chance: string,
  ): string =>
    met
      ? `目標 ${n}: ${moveName}で${opponent}を${hits}発で倒せます(倒す確率 ${chance})`
      : `目標 ${n}: ${moveName}で${opponent}を${hits}発では倒せません(倒す確率 ${chance})`,
  // ---- 技を覚えるポケモン(listMoveLearners。ADR-0319 §7) ----
  /** ボタンの見える文字(accessible name はどちらの技かを前に足す。SC 2.5.3)。 */
  learnersButtonLabel: "覚えるポケモン",
  selfLearnersButtonName: "自分の技を覚えるポケモン",
  opponentLearnersButtonName: "相手の技を覚えるポケモン",
  learnersHeading: (moveName: string): string => `${moveName}を覚えるポケモン`,
  learnersEmpty: "この技を覚えるポケモンはいません",
  learnersMore: "続きを読み込む",
  learnersLoading: "読み込み中",
} as const;
