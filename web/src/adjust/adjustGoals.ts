// F-11(ADR-0331 §5・§6): 調整の「相手を選んで目標を選ぶ」の定数と、要求を組み立てる純粋関数。
// 定数は「入力の作り方の型」(画面の選択肢)で、マスタではない(ポケモン・技・持ち物のリストは持たない)。
// 相手の素早さの実数値は Web で計算しない(サーバーが engine で求める。ADR-0331 §3)。
//
// スタブ: 関数の中身は段階 A の実装で書く(spec 段階。テストは adjustGoals.test.ts)。

import type { components } from "../api/openapi.gen";
import type { AttackerPresetKey } from "../domain/attackerPresets";
import type { DefenderPresetKey } from "../domain/defenderPresets";
import type { Item, MoveCategory, Nature, Stats } from "../engine/types";
import type { MasterNature, MasterSpecies } from "../master/types";

type Schemas = components["schemas"];

/** 目標の種類(api/openapi.yaml の AdjustGoalKind)。 */
export type AdjustGoalKind = Schemas["AdjustGoalKind"];

/**
 * 「目標から振り方を決める」モードを出すか(ADR-0331 §2)。calc-svc の adjustGoals(段階 B)が main に入るまで false。
 * 画面の props `goalsEnabled` で上書きできる(テスト用)。
 */
export const ADJUST_GOALS_ENABLED = false;

/** 目標の件数の上限(api/openapi.yaml の AdjustGoalsRequest.goals の maxItems)。 */
export const MAX_ADJUST_GOALS = 6;

/** 種類の選択肢の順(ADR-0331 §5)。 */
export const ADJUST_GOAL_KINDS: readonly AdjustGoalKind[] = ["outspeed", "survive", "ko"];

/** 目標を足したときの種類の既定。 */
export const DEFAULT_ADJUST_GOAL_KIND: AdjustGoalKind = "outspeed";

/** 素早さのプリセット(docs/speed-design.md §5 の語。ADR-0331 §5 の表)。 */
export type SpeedPresetKey = "fastest" | "neutral_max" | "none";

/** 素早さのプリセットの並び(速い順)。 */
export const SPEED_PRESET_KEYS: readonly SpeedPresetKey[] = ["fastest", "neutral_max", "none"];

/** 素早さの目標の相手の振り方の既定(最速)。 */
export const DEFAULT_SPEED_PRESET: SpeedPresetKey = "fastest";

/** 「耐える」の相手の振り方の既定(相手が火力に振っていても耐える側に寄せる)。 */
export const DEFAULT_SURVIVE_PRESET: AttackerPresetKey = "x_full";

/** 「倒す」の相手の振り方の既定。 */
export const DEFAULT_KO_PRESET: DefenderPresetKey = "none";

/** resolveSpeedPreset が返す SP・性格(engine の Nature。plus/minus の "" は補正なし)。 */
export interface ResolvedSpeedPreset {
  readonly sp: Stats;
  readonly nature: Nature;
}

/** 素早さのプリセットの SP・性格(ADR-0331 §5 の表)。 */
export function resolveSpeedPreset(key: SpeedPresetKey): ResolvedSpeedPreset {
  throw new Error(`未実装(ADR-0331 段階 A): resolveSpeedPreset(${key})`);
}

/** 目標の相手の振り方(種類ごとにプリセットの種類が違う)。 */
export type GoalOpponentPreset =
  | { readonly kind: "outspeed"; readonly key: SpeedPresetKey }
  /** 耐える: 相手が攻撃する。category は相手の技の分類(A/C の切り替え)。 */
  | { readonly kind: "survive"; readonly key: AttackerPresetKey; readonly category: MoveCategory }
  | { readonly kind: "ko"; readonly key: DefenderPresetKey };

export interface GoalOpponentInput {
  readonly species: MasterSpecies;
  readonly preset: GoalOpponentPreset;
  readonly natures: readonly MasterNature[];
  /** マスタの持ち物の全件(メガストーンを引く。ADR-0326 §4)。 */
  readonly items: readonly Item[];
}

/**
 * 目標の相手の Individual(`{ speciesKey, level: 50, natureId, sp }`。メガ種族はストーンの itemId。ADR-0331 §1・§6)。
 * プリセットの性格がマスタに無ければ null(別の性格で代えない)。
 */
export function goalOpponentIndividual(input: GoalOpponentInput): Schemas["Individual"] | null {
  throw new Error(`未実装(ADR-0331 段階 A): goalOpponentIndividual(${input.species.key})`);
}

export interface GoalRequestInput {
  readonly kind: AdjustGoalKind;
  readonly opponent: Schemas["Individual"];
  /** 耐える = 相手の技、倒す = 自分の技、素早さ = 先に使う技(任意。null は使わない)。 */
  readonly moveId: string | null;
  readonly hits: number;
  readonly thresholdPercent: number;
}

/** 1つの目標の要求(省略可の欄は送らない。ADR-0331 §6)。 */
export function buildGoalRequest(input: GoalRequestInput): Schemas["AdjustGoal"] {
  throw new Error(`未実装(ADR-0331 段階 A): buildGoalRequest(${input.kind})`);
}
