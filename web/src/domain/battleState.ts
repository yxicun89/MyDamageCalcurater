// ADR-0144 §3(I-web-13): 計算画面の「対戦の状態」(攻撃側・防御側の残り HP と多段の回数)の入力と、要求の battleState にする純粋関数。
// 要求の形はここ1か所で決める。空・満タン・既定の回数は battleState に入れない(従来の要求とバイト単位で同じ)。
// 範囲外の入力は黙って丸めず、誤りとして返す(呼び出し側は計算を送らない)。

import type { BattleState, Move } from "../engine/types";

/** 画面の入力(残り HP は入力途中の文字列のまま持つ。回数は null が「既定」)。 */
export interface BattleStateInputs {
  readonly attackerHp: string;
  readonly defenderHp: string;
  readonly hits: number | null;
}

export const DEFAULT_BATTLE_STATE_INPUTS: BattleStateInputs = { attackerHp: "", defenderHp: "", hits: null };

/** 残り HP の入力の判定。empty = 満タン(送らない)。 */
export type HpInput =
  { readonly kind: "empty" } | { readonly kind: "ok"; readonly value: number } | { readonly kind: "error" };

/** 残り HP の文字列を 1..max の整数として読む。空白だけは空。整数でない・範囲外は error。 */
export function parseHpInput(text: string, max: number): HpInput {
  const trimmed = text.trim();
  if (trimmed === "") {
    return { kind: "empty" };
  }
  if (!/^\d+$/.test(trimmed)) {
    return { kind: "error" };
  }
  const value = Number(trimmed);
  return value >= 1 && value <= max ? { kind: "ok", value } : { kind: "error" };
}

/** 文字列が整数として最大を超えているか(最大の変更で丸める判定。整数でない入力は丸めない)。 */
export function exceedsMax(text: string, max: number): boolean {
  const trimmed = text.trim();
  return /^\d+$/.test(trimmed) && Number(trimmed) > max;
}

/** 範囲の多段技(mechanismParams.multiHit の min < max)の回数の範囲。それ以外は null。 */
export function multiHitRangeOf(move: Move | null): { readonly min: number; readonly max: number } | null {
  const multiHit = move?.mechanismParams?.["multiHit"];
  if (typeof multiHit !== "object" || multiHit === null) {
    return null;
  }
  const { min, max } = multiHit as { min?: unknown; max?: unknown };
  if (
    typeof min !== "number" ||
    typeof max !== "number" ||
    !Number.isInteger(min) ||
    !Number.isInteger(max)
  ) {
    return null;
  }
  return min < max ? { min, max } : null;
}

/** 回数の選択肢(min..max)。 */
export function hitsOptions(range: { readonly min: number; readonly max: number }): number[] {
  return Array.from({ length: range.max - range.min + 1 }, (_, index) => range.min + index);
}

/** 割合(小数第1位・切り捨て)。「41.6%」の形。 */
export function percentText(hp: number, max: number): string {
  return `${(Math.floor((hp * 1000) / max) / 10).toFixed(1)}%`;
}

export interface BattleStateLimits {
  /** 攻撃側の最大 HP(実数値)。種族が決まっていない・入力が不正なときは null(欄を出さず、送らない)。 */
  readonly attackerMaxHp: number | null;
  /** 防御側の最大 HP(実数値)。 */
  readonly defenderMaxHp: number | null;
}

export interface BattleStateResolved {
  /** 要求に入れる値。送るものが無ければ undefined(キーごと省く)。 */
  readonly state: BattleState | undefined;
  readonly attackerError: boolean;
  readonly defenderError: boolean;
}

/** 入力を要求の battleState にする。満タン(最大と同じ値)・空・既定の回数・範囲の多段でない技の回数は入れない。 */
export function resolveBattleState(
  inputs: BattleStateInputs,
  limits: BattleStateLimits,
  move: Move | null,
): BattleStateResolved {
  const attacker =
    limits.attackerMaxHp === null ? null : parseHpInput(inputs.attackerHp, limits.attackerMaxHp);
  const defender =
    limits.defenderMaxHp === null ? null : parseHpInput(inputs.defenderHp, limits.defenderMaxHp);
  const range = multiHitRangeOf(move);
  const hits =
    range !== null && inputs.hits !== null && inputs.hits >= range.min && inputs.hits <= range.max
      ? inputs.hits
      : null;
  const state: BattleState = {
    ...(attacker?.kind === "ok" && attacker.value < (limits.attackerMaxHp ?? 0)
      ? { attackerCurrentHp: attacker.value }
      : {}),
    ...(defender?.kind === "ok" && defender.value < (limits.defenderMaxHp ?? 0)
      ? { defenderCurrentHp: defender.value }
      : {}),
    ...(hits === null ? {} : { hits }),
  };
  return {
    state: Object.keys(state).length === 0 ? undefined : state,
    attackerError: attacker?.kind === "error",
    defenderError: defender?.kind === "error",
  };
}

/** 保存された battleState(お気に入り・履歴)を画面の入力に戻す。 */
export function inputsOfBattleState(state: BattleState | undefined | null): BattleStateInputs {
  return {
    attackerHp: state?.attackerCurrentHp === undefined ? "" : String(state.attackerCurrentHp),
    defenderHp: state?.defenderCurrentHp === undefined ? "" : String(state.defenderCurrentHp),
    hits: state?.hits ?? null,
  };
}
