// AJ6: 調整のリクエストを組み立てる補助(ADR-0319 §2・§5)。純粋関数と、画面のプリセットの定数。
// 定数は「入力の作り方の型」(画面の選択肢)で、マスタではない(持ち物・技・ポケモンのリストは持たない)。

import type { Nature } from "../engine/types";
import type { MasterNature } from "../master/types";

/** 発数の選択肢の上限(api/openapi.yaml の AdjustHits.maximum = engine の MaxAdjustHits)。 */
export const MAX_ADJUST_HITS = 10;

/** 発数の選択肢(1..MAX_ADJUST_HITS)。既定は DEFAULT_ADJUST_HITS。 */
export const ADJUST_HITS_OPTIONS: readonly number[] = Array.from(
  { length: MAX_ADJUST_HITS },
  (_, i) => i + 1,
);

/** 発数の既定(確定1発)。 */
export const DEFAULT_ADJUST_HITS = 1;

/**
 * 確率のしきい値の選択肢(%)。数値を打たせずプリセットから選ぶ(requirements.md「プリセットを選ぶだけ」)。
 * 先頭の 100 が既定(確定)で、契約の既定と同じなのでリクエストでは省略する(ADR-0319 §5)。
 */
export const ADJUST_THRESHOLD_PRESETS: readonly number[] = [100, 90, 75, 50];

/** しきい値の既定(確定 = 契約の AdjustThresholdPercent の default)。 */
export const DEFAULT_ADJUST_THRESHOLD_PERCENT = 100;

/** 技を覚えるポケモンの1ページの件数(契約 listMoveLearners の limit の既定と同じ 50)。 */
export const LEARNERS_PAGE_SIZE = 50;

/** 4096 基準の等倍(domain/requests.ts の NEUTRAL_MODIFIER と同じ値。契約の AdjustModifier の default)。 */
export const ADJUST_NEUTRAL_MODIFIER = 4096;

/** タイプ一致の補正(×1.5 を 4096 基準で表したもの。ADR-0319 §5)。 */
export const STAB_MODIFIER = 6144;

/**
 * 火力指数の補正(ADR-0319 §5)。技のタイプが種族のタイプのどれかと一致すれば STAB_MODIFIER、
 * そうでなければ ADJUST_NEUTRAL_MODIFIER。持ち物・特性・テラスタルは含めない。
 */
export function firepowerModifier(speciesTypes: readonly string[], moveType: string): number {
  return speciesTypes.includes(moveType) ? STAB_MODIFIER : ADJUST_NEUTRAL_MODIFIER;
}

/**
 * プリセットの性格(engine の Nature。plus/minus は "" が「補正なし」)に一致するマスタの性格の ID。
 * 無補正(plus・minus とも "")はマスタの plus・minus が両方 null の性格のうち先頭。見つからなければ null。
 */
export function natureIdForPreset(natures: readonly MasterNature[], nature: Nature): string | null {
  const found = natures.find(
    (candidate) => (candidate.plus ?? "") === nature.plus && (candidate.minus ?? "") === nature.minus,
  );
  return found?.id ?? null;
}
