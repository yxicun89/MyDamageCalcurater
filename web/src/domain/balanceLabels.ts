// P4-12a: タイプバランスの倍率の表示(ADR-0303 §2、docs/type-balance-design.md §10)。
// 倍率の数値は balance-svc の応答の文字列(DefenseMultiplier / CoverageMultiplier)をそのまま使い、
// Web で計算し直さない(ADR-0303 §1)。語は応答の category(DefenseCategory)から選び、値の範囲を
// Web 側で判定し直さない(balance-svc が既に判定済み)。

import { balanceLabelText } from "../i18n/ja";
import type { components } from "../api/balance.gen";

type Schemas = components["schemas"];

/** 防御相性の倍率表示(「×2 弱点」のように文字で出す。DefenseMultiplier は生成型では string)。 */
export function defenseMultiplierLabel(multiplier: string, category: Schemas["DefenseCategory"]): string {
  return `×${multiplier} ${balanceLabelText.defenseCategoryWord[category]}`;
}

/** 攻撃範囲の倍率表示(「×2 抜群」のように文字で出す。攻撃技が無ければ null で「攻撃技なし」)。 */
export function coverageMultiplierLabel(multiplier: Schemas["CoverageMultiplier"]): string {
  if (multiplier === null) {
    return balanceLabelText.coverageNoAttackMove;
  }
  return `×${multiplier} ${balanceLabelText.coverageWord[multiplier]}`;
}

/**
 * P4-12b: 語を添えない倍率表示(「×2」)。ThreatMatchup.incoming/outgoing・AbilityOptionPokemon.multiplier は
 * category を持たないため、倍率の範囲から弱点・耐性を Web で判定し直さない(ADR-0303 §7)。
 */
export function multiplierLabel(multiplier: string): string {
  return `×${multiplier}`;
}

/** P4-12b: 仮想敵の受ける/与える倍率(MatchupMultiplier)。攻撃技が無ければ null で「攻撃技なし」。 */
export function matchupMultiplierLabel(multiplier: string | null): string {
  return multiplier === null ? balanceLabelText.coverageNoAttackMove : multiplierLabel(multiplier);
}

/** P4-12b: 仮想敵の「安全に受けられるか」(ThreatMatchup.safe)。応答の真偽値をそのまま語にする。 */
export function safeLabel(safe: boolean): string {
  return safe ? balanceLabelText.safe : balanceLabelText.unsafe;
}

/** P4-12b: 仮想敵への「抜群が取れるか」(ThreatMatchup.superEffective)。応答の真偽値をそのまま語にする。 */
export function superEffectiveLabel(superEffective: boolean): string {
  return superEffective ? balanceLabelText.superEffective : balanceLabelText.notSuperEffective;
}
