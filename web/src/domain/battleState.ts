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

/** 防御側の残りHPの入力は割合(%)。整数の 1..100(小数は受け付けない。実数値への換算は行ごと: defenderCurrentHpOfRow)。 */
export const DEFENDER_PERCENT_MAX = 100;

/** 画面で決まった対戦の状態。防御側は割合(行の最大 HP が行ごとに違うため、実数値には行ごとに換算する)。 */
export interface ScreenBattleState {
  /** 攻撃側の残り HP(実数値)。 */
  readonly attackerCurrentHp?: number;
  /** 防御側の残り HP の割合(1..99。100 は満タンなので入れない)。 */
  readonly defenderPercent?: number;
  readonly hits?: number;
}

export interface BattleStateLimits {
  /** 攻撃側の最大 HP(実数値)。種族が決まっていない・入力が不正なときは null(欄を出さず、送らない)。 */
  readonly attackerMaxHp: number | null;
  /** 防御側の種族が決まっているか(割合の欄を出すか)。 */
  readonly defenderPresent: boolean;
}

export interface BattleStateResolved {
  /** 要求に入れる値。送るものが無ければ undefined(キーごと省く)。 */
  readonly state: ScreenBattleState | undefined;
  readonly attackerError: boolean;
  readonly defenderError: boolean;
}

/** 入力を対戦の状態にする。満タン(最大と同じ値・100%)・空・既定の回数・範囲の多段でない技の回数は入れない。 */
export function resolveBattleState(
  inputs: BattleStateInputs,
  limits: BattleStateLimits,
  move: Move | null,
): BattleStateResolved {
  const attacker =
    limits.attackerMaxHp === null ? null : parseHpInput(inputs.attackerHp, limits.attackerMaxHp);
  const defender = limits.defenderPresent ? parseHpInput(inputs.defenderHp, DEFENDER_PERCENT_MAX) : null;
  const range = multiHitRangeOf(move);
  const hits =
    range !== null && inputs.hits !== null && inputs.hits >= range.min && inputs.hits <= range.max
      ? inputs.hits
      : null;
  const state: ScreenBattleState = {
    ...(attacker?.kind === "ok" && attacker.value < (limits.attackerMaxHp ?? 0)
      ? { attackerCurrentHp: attacker.value }
      : {}),
    ...(defender?.kind === "ok" && defender.value < DEFENDER_PERCENT_MAX
      ? { defenderPercent: defender.value }
      : {}),
    ...(hits === null ? {} : { hits }),
  };
  return {
    state: Object.keys(state).length === 0 ? undefined : state,
    attackerError: attacker?.kind === "error",
    defenderError: defender?.kind === "error",
  };
}

/**
 * 割合(%)を、最大 HP の行の実数値にする: max(1, floor(最大 × % / 100))。結果が最大以上なら満タンなので null(その行には付けない)。
 * 行の最大 HP は一括の結果の defenderHP。iOS も同じ規則に揃える。
 */
export function defenderCurrentHpOfRow(percent: number, rowMaxHp: number): number | null {
  const hp = Math.max(1, Math.floor((rowMaxHp * percent) / 100));
  return hp >= rowMaxHp ? null : hp;
}

/** 行に送る battleState(その行に付けるものが無ければ undefined)。 */
export function battleStateOfRow(state: ScreenBattleState, rowMaxHp: number): BattleState | undefined {
  const defenderCurrentHp =
    state.defenderPercent === undefined ? null : defenderCurrentHpOfRow(state.defenderPercent, rowMaxHp);
  const row: BattleState = {
    ...(state.attackerCurrentHp === undefined ? {} : { attackerCurrentHp: state.attackerCurrentHp }),
    ...(defenderCurrentHp === null ? {} : { defenderCurrentHp }),
    ...(state.hits === undefined ? {} : { hits: state.hits }),
  };
  return Object.keys(row).length === 0 ? undefined : row;
}

/**
 * お気に入りに保存する battleState(実数値)。行が特定できないので、無振り(SP 0)の最大 HP の行に換算する
 * (復元は percentOfSavedHp が逆に戻す)。
 */
export function savedBattleState(
  state: ScreenBattleState,
  referenceMaxHp: number | null,
): BattleState | undefined {
  if (referenceMaxHp === null) {
    return battleStateOfRow(
      {
        ...(state.attackerCurrentHp === undefined ? {} : { attackerCurrentHp: state.attackerCurrentHp }),
        ...(state.hits === undefined ? {} : { hits: state.hits }),
      },
      1,
    );
  }
  return battleStateOfRow(state, referenceMaxHp);
}

/**
 * 保存された防御側の残り HP(実数値)を割合に戻す: その実数値に届く最小の整数 % = ceil(HP × 100 / 無振りの最大 HP)(1..99)。
 * 最大 HP が分からない・最大以上は満タン(空)。行が特定できないので近似で、換算し直すと保存値以上になる。
 */
export function percentOfSavedHp(hp: number, referenceMaxHp: number | null): string {
  if (referenceMaxHp === null || hp >= referenceMaxHp) {
    return "";
  }
  return String(Math.min(DEFENDER_PERCENT_MAX - 1, Math.max(1, Math.ceil((hp * 100) / referenceMaxHp))));
}

/** 保存された battleState(お気に入り・履歴)を画面の入力に戻す。 */
export function inputsOfBattleState(
  state: BattleState | undefined | null,
  defenderReferenceMaxHp: number | null,
): BattleStateInputs {
  return {
    attackerHp: state?.attackerCurrentHp === undefined ? "" : String(state.attackerCurrentHp),
    defenderHp:
      state?.defenderCurrentHp === undefined
        ? ""
        : percentOfSavedHp(state.defenderCurrentHp, defenderReferenceMaxHp),
    hits: state?.hits ?? null,
  };
}
