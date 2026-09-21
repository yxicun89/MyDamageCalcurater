// P4-3: 攻撃側(自分側)のプリセット(ADR-0016 §5、requirements.md「自分側のプリセット」)。
// 置き場は Web(ADR-0016 §5: engine への移管は DECISIONS.md に提案済みで、まだ実施していない)。
// X は技の分類で決まる関連ステータス(物理・変化 = atk、特殊 = spa)。下降補正の置き先は
// ADR-0010 §R1 の代表性格(Minus: atk、関連が atk のときだけ Minus: spa)と同じにする。

import type { MoveCategory, Nature, Stats } from "../engine/types";
import { attackerPresetText } from "../i18n/ja";
import { MAX_SP_PER_STAT, NEUTRAL_NATURE, ZERO_SP } from "./requests";

/** X になりうるステータス(物理・変化 = atk、特殊 = spa)。 */
type RelevantStat = "atk" | "spa";

/** 攻撃側プリセットの Key。none = 無振り、x_full = X特化、x = X振り(無補正)。 */
export type AttackerPresetKey = "none" | "x_full" | "x";

/** カタログの順序(ADR-0016 §5 の表の順)。 */
export const ATTACKER_PRESET_KEYS: readonly AttackerPresetKey[] = ["none", "x_full", "x"];

/** 既定は無振り(P4-2 で固定していた値と同じ)。 */
export const DEFAULT_ATTACKER_PRESET: AttackerPresetKey = "none";

/** 技の分類から関連ステータス X を決める(物理・変化 = atk、特殊 = spa。ADR-0016 §5)。 */
function relevantStat(category: MoveCategory): RelevantStat {
  return category === "special" ? "spa" : "atk";
}

/** X の反対のステータス(下降補正の置き先。ADR-0010 §R1 の代表性格と同じ)。 */
function oppositeStat(stat: RelevantStat): RelevantStat {
  return stat === "atk" ? "spa" : "atk";
}

/** resolveAttackerPreset が返す SP・性格。 */
export interface ResolvedAttackerPreset {
  readonly sp: Stats;
  readonly nature: Nature;
}

/**
 * Key と技の分類から、engine に渡す SP・性格を求める(ADR-0016 §5 の表)。
 * ZERO_SP・NEUTRAL_NATURE を直接返さず新しいオブジェクトにし、共有の定数を書き換えない。
 */
export function resolveAttackerPreset(
  key: AttackerPresetKey,
  category: MoveCategory,
): ResolvedAttackerPreset {
  if (key === "none") {
    return { sp: { ...ZERO_SP }, nature: { ...NEUTRAL_NATURE } };
  }
  const stat = relevantStat(category);
  const sp: Stats = { ...ZERO_SP, [stat]: MAX_SP_PER_STAT };
  const nature: Nature = key === "x_full" ? { plus: stat, minus: oppositeStat(stat) } : { ...NEUTRAL_NATURE };
  return { sp, nature };
}

/** プリセットの表示名(文言は i18n/ja.ts)。技の分類で A/C 表記を切り替える。 */
export function attackerPresetLabel(key: AttackerPresetKey, category: MoveCategory): string {
  if (key === "none") {
    return attackerPresetText.none;
  }
  const letter = attackerPresetText.statLetter[relevantStat(category)];
  const suffix = key === "x_full" ? attackerPresetText.fullSuffix : attackerPresetText.xSuffix;
  return `${letter}${suffix}`;
}
