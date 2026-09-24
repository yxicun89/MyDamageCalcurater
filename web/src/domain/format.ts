// P4-2: 結果の表示の書式(ADR-0300 §8「Web は返ってきた値を加工せずに表示する。ここは書式だけ」)。
// 表示%は engine が小数第1位の数値で返す(ADR-0011 §3、tenthPercent)。Web は丸め直さず、常に1桁で書く。
// 技の相性(effectiveness)や確定数の分類も engine の値をそのまま言葉にするだけで、TS で計算しない。

import type { CalcResult, KOChance, MoveCategory } from "../engine/types";
import { resultText } from "../i18n/ja";

/** 表示%を常に小数第1位まで書く("73.4"、"100.0"、"0.0")。 */
export function formatPercent(value: number): string {
  return value.toFixed(1);
}

/** 「最小〜最大%」の表記(design.md の結果行)。 */
export function formatPercentRange(range: { minPercent: number; maxPercent: number }): string {
  return `${formatPercent(range.minPercent)}〜${formatPercent(range.maxPercent)}%`;
}

/**
 * 確定数の表記(design.md「確定2発」「乱数2発」)。
 * 表示する確率は engine の displayChancePercent(確定は 100.0、倒せないときは 0.0。ADR-0006・ADR-0011 §3)で、
 * 生値 chancePercent は使わない。
 */
export function formatKO(ko: KOChance): string {
  if (ko.hits === 0) {
    return resultText.cannotKO;
  }
  if (ko.guaranteed) {
    return `${resultText.determinedPrefix}${ko.hits}${resultText.hitsSuffix}`;
  }
  return `${resultText.randomPrefix}${ko.hits}${resultText.hitsSuffix}(${formatPercent(ko.displayChancePercent)}%)`;
}

/** 技の相性の表記。engine の結果の effectiveness(0/0.25/0.5/1/2/4)をそのまま言葉にする。 */
export function formatEffectiveness(value: CalcResult["effectiveness"]): string {
  if (value === 0) {
    return resultText.effectivenessNone;
  }
  if (value < 1) {
    return resultText.effectivenessNotVery(value);
  }
  if (value === 1) {
    return resultText.effectivenessNeutral;
  }
  return resultText.effectivenessSuper(value);
}

/** 技の分類の表示名。 */
export function formatMoveCategory(category: MoveCategory): string {
  switch (category) {
    case "physical":
      return resultText.moveCategory.physical;
    case "special":
      return resultText.moveCategory.special;
    case "status":
      return resultText.moveCategory.status;
  }
}
