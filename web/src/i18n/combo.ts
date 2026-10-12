// CB-6 段階 B(ADR-0359): 計算画面の「連続技」の文言。iOS(CB-8)は docs/ai-shared/decisions/2026-10-12-web-combo-screen.md の語に揃える。
// 用語はやさしい言葉にする(「ランク」は計算画面の既存の語。「素早さ」は表記ゆれさせない)。

/** 連続技の印(UnsupportedMark.reason)のうち、連続技だけが付けるもの。語は ADR-0123 §2 の表の書式に続ける。 */
const comboMarkReasonLabel = {
  choice_locked: "技を変えられない持ち物",
  defender_full_hp_assumed: "2手目以降も相手が満タンとして計算",
  speed_undetermined: "素早さへの影響を判定できない",
} as const;

export type ComboMarkReason = keyof typeof comboMarkReasonLabel;

export function isComboMarkReason(reason: string): reason is ComboMarkReason {
  return Object.hasOwn(comboMarkReasonLabel, reason);
}

/** 例のポケモン名を添える(「(例: ○○)」)。名前が無ければ何も足さない。 */
function exampleSuffix(example: string | null): string {
  return example === null ? "" : `(例: ${example})`;
}

export const comboScreenText = {
  sectionLabel: "連続技",
  /** 「+」のアイコン(aria-hidden)に続く、手が 1 つのときの開くボタン。 */
  addStepButton: "次の技",
  /** 追加の手を選ぶ技ピッカーの名前(計算画面の「技」とは別の名前にして取り違えない)。 */
  addPickerLabel: "追加の技",
  closeButton: "閉じる",
  maxStepsNotice: (max: number): string => `手は最大${String(max)}つまでです`,
  stepsListLabel: "連続技の手",
  resultStepsListLabel: "連続技の各手の結果",
  stepOrdinal: (index: number): string => `${String(index + 1)}手目`,
  mainStepBadge: "上の技欄の技",
  moveUp: (name: string): string => `${name}を上へ`,
  moveDown: (name: string): string => `${name}を下へ`,
  removeStep: (name: string): string => `${name}を外す`,
  critical: (index: number): string => `急所(${String(index + 1)}手目)`,
  noDamage: "ダメージなし",
  /** ランクチップの「相手の…」(相手のランクを下げる技)。 */
  opponentRank: (statName: string): string => `相手の${statName}`,
  orderLabel: {
    self_first: "先に動く",
    opponent_first: "相手が先",
    speed_tie: "同速",
  },
  defenderLabel: "相手の振り",
  opponentMoveLabel: "相手の技",
  opponentMoveNone: "選ばない",
  opponentSpeedFromDefender: "相手の素早さを防御側の振りから計算する",
  speedFieldToggle: "素早さの場",
  trickRoom: "トリックルーム",
  selfTailwind: "追い風(自分)",
  opponentTailwind: "追い風(相手)",
  loading: "連続技を計算中…",
  failed: (message: string): string => `連続技を計算できませんでした(${message})`,
  total: (min: string, max: string): string => `合計 ${min}〜${max}%`,
  koCannot: "倒せない",
  koGuaranteed: (steps: number): string => `${String(steps)} 手で倒す(確定)`,
  koChance: (steps: number, percent: string): string => `${String(steps)} 手で倒す ${percent}`,
  speedSummary: (first: number, last: number): string => `素早さ ${String(first)} → ${String(last)}`,
  opponentSpeed: (speed: number): string => `相手の素早さ ${String(speed)}`,
  /** 「N 以下を抜く」「N 以上を抜く」の N 側の語。 */
  lineAtMost: (n: number): string => `${String(n)} 以下`,
  lineAtLeast: (n: number): string => `${String(n)} 以上`,
  lineSelfFirst: "を抜く",
  lineTie: (n: number): string => `${String(n)} は同速`,
  lineOpponentFirst: "は相手が先",
  lineSeparator: " / ",
  survive: (percent: string): string => `耐える ${percent}`,
  actions: (min: number, max: number): string =>
    min === max ? `相手の行動 ${String(min)}回` : `相手の行動 ${String(min)}〜${String(max)}回`,
  perHit: (range: string): string => `1回ぶん ${range}`,
  /** 相手の振りの選択が消えて先頭に戻ったときの通知(role=status。CB-6 段階 C3)。 */
  defenderReset: "選んでいた相手の振りがなくなったので、先頭の振りに戻しました",
  /** 素早さの種族の例(CB-6 段階 C1。ADR-0610 の「族」。準速 = SP 32・補正なしの素早さ)。 */
  speedExample: {
    listLabel: "素早さの基本値の例",
    faster: (baseSpeed: number, example: string | null): string =>
      `準速 ${String(baseSpeed)} 族以上には抜かれる${exampleSuffix(example)}`,
    slower: (baseSpeed: number, example: string | null): string =>
      `準速 ${String(baseSpeed)} 族までは抜ける${exampleSuffix(example)}`,
    tie: (range: string): string => `準速 ${range} 族とは同速`,
  },
  note: "場は全手で同じ状態が続きます。命中は考えません。",
  /** 連続技だけの印 1 件。target は unsupportedText.target の語、name は ID を解決した名前。 */
  markReason: comboMarkReasonLabel,
} as const;
