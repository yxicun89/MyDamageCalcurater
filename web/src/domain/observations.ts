// P4-4: 逆算の観測の入力(ADR-0300 §7、ADR-0010 §R2)。観測は整数%(1〜100)か、HP の実点数(1 以上の整数)。
// 小数の表示%は観測に使わない(ADR-0300 §7)ので、画面の入力は整数だけを受け付け、engine に渡す前に弾く。
// engine に渡す Observation は percent / damage のちょうど1つを持つ(percentTenths は画面から使わない)。

import type { Observation, ReverseSide } from "../engine/types";
import { MAX_OBSERVATIONS } from "./requestLimits";

/** 観測の入力単位。percent = 整数%、damage = HP の実点数。 */
export type ObservationUnit = "percent" | "damage";

/**
 * 観測の数値を打っている間、計算を始めるまでの待ち時間(ミリ秒。issue 113、ADR-0300 §11)。
 * 「45」と打つ途中の「4」も有効な観測なので、待たないと1文字ごとに計算が走る。表示と入力の検証は待たずに行い、
 * 計算だけをこの時間だけ遅らせる(trailing debounce)。選択・単位の切り替え・行の追加や削除は確定した操作なので待たない。
 * 値は SPECIES_SEARCH_DEBOUNCE_MS(250ms)と別に持つ: 逆算の観測は「打ち終わり」が短く、issue 113 の既定案が 200ms。
 */
export const OBSERVATION_INPUT_DEBOUNCE_MS = 200;

/** parseObservation の結果(判別 union)。empty は未入力(不正扱いにしない)。 */
export type ParsedObservation =
  | { readonly status: "empty" }
  | { readonly status: "invalid" }
  | { readonly status: "valid"; readonly observation: Observation };

/** 観測%の範囲(ADR-0010 §R2: `Observation.Percent` は 1..100)。 */
const MIN_OBSERVED_PERCENT = 1;
const MAX_OBSERVED_PERCENT = 100;

/** 観測(HP の実点数)の下限(ADR-0010 §R2: `Observation.Damage` は正の整数)。 */
const MIN_OBSERVED_DAMAGE = 1;

/** 整数だけを受け付ける(ASCII 数字のみ。全角数字・指数表記・16進表記・小数・符号は不正)。 */
const INTEGER_PATTERN = /^[0-9]+$/;

/** 前後の空白を落とし、ASCII の整数表記だけを数値にする。整数でなければ null。 */
function parseStrictInteger(text: string): number | null {
  const trimmed = text.trim();
  return INTEGER_PATTERN.test(trimmed) ? Number.parseInt(trimmed, 10) : null;
}

/**
 * 観測の入力文字列を検証する。空(空白のみ含む)は「未入力」として不正と区別する
 * (画面は空行を無視して送るだけで、エラー表示はしない)。
 */
export function parseObservation(unit: ObservationUnit, text: string): ParsedObservation {
  if (text.trim() === "") {
    return { status: "empty" };
  }
  const value = parseStrictInteger(text);
  if (value === null) {
    return { status: "invalid" };
  }
  if (unit === "percent") {
    if (value < MIN_OBSERVED_PERCENT || value > MAX_OBSERVED_PERCENT) {
      return { status: "invalid" };
    }
    return { status: "valid", observation: { percent: value } };
  }
  if (value < MIN_OBSERVED_DAMAGE) {
    return { status: "invalid" };
  }
  return { status: "valid", observation: { damage: value } };
}

/**
 * 観測をもう1行追加できるか(P4-19。上限は MAX_OBSERVATIONS = api/openapi.yaml の
 * `ReverseRequest.observations` の maxItems。ADR-0208)。上限に達したら画面は追加を無効にし、理由を出す。
 */
export function canAddObservation(currentCount: number): boolean {
  return currentCount < MAX_OBSERVATIONS;
}

/** 対象側の既定の観測単位: 与えたダメージ(defender)は%、受けたダメージ(attacker)は HP の実点数。 */
export function defaultObservationUnit(side: ReverseSide): ObservationUnit {
  return side === "defender" ? "percent" : "damage";
}
