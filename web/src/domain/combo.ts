// CB-6: 連続技(docs/plan/combo-calc.md)のリクエスト組み立てと、技がランクを変えるかの判定(純粋関数)。
// 組み立ては domain/requests.ts の buildCalcRequest と同じ流儀(省略できる項目はキーを作らない)。

import type {
  ComboOpponentSpeed,
  ComboRequest,
  ComboSpeedField,
  ComboStepRequest,
  Field,
  Individual,
  Move,
  MoveEffect,
  TypeChart,
} from "../engine/types";
import { limitToMax } from "./requestLimits";

/**
 * 連続技の手の数の上限(engine の MaxComboSteps。API は 400 invalid_input、WASM は invalid_input で拒否する)。
 * 値の正は engine/combo.go。
 */
export const MAX_COMBO_STEPS = 3;

/** 連続技の手の数の下限(手が 0 件の連続技は無い。1 手目 = 計算画面の技)。 */
export const MIN_COMBO_STEPS = 1;

/**
 * 追加効果が 1 つ以上のランクを実際に変えるか(stages に 0 でない値がある)。
 * effect が無い・null・stages が空は「変わらない」(古いサーバーも確率 100% 未満の効果も同じ扱い。Move.effect の説明)。
 */
export function changesRanks(move: Move): boolean {
  return rankEffectOf(move) !== null;
}

/** ランクを変える追加効果。変えないなら null。 */
export function rankEffectOf(move: Move): MoveEffect | null {
  const effect = move.effect;
  if (effect === undefined || effect === null) {
    return null;
  }
  return Object.values(effect.stages).some((stage) => stage !== 0) ? effect : null;
}

/**
 * 連続技の手に置ける技か。攻撃技はいつでも置ける。変化技はランクを変えるもの(りゅうのまい等)だけ
 * (ランク変化を持たない変化技は API が 400、WASM が invalid_input で拒否する)。
 */
export function canBeComboStep(move: Move): boolean {
  return move.category !== "status" || changesRanks(move);
}

/** 相手の技に置けるか。攻撃技だけ(変化技は API が 400、WASM が invalid_input)。 */
export function canBeOpponentMove(move: Move): boolean {
  return move.category !== "status";
}

export interface BuildComboRequestInput {
  readonly attacker: Individual;
  readonly defender: Individual;
  readonly steps: readonly ComboStepRequest[];
  readonly typeChart: TypeChart;
  /** 場(省略は送らない)。全手で同じ。 */
  readonly field?: Field;
  readonly opponentMove?: Move;
  readonly opponentSpeed?: ComboOpponentSpeed;
  readonly speedField?: ComboSpeedField;
}

/** 1 手(急所は true のときだけ、多段の回数は指定があるときだけ載せる)。 */
function normalizeStep(step: ComboStepRequest): ComboStepRequest {
  return {
    move: step.move,
    ...(step.critical === true ? { critical: true } : {}),
    ...(step.hits === undefined ? {} : { hits: step.hits }),
  };
}

/** 行動順に効く場が 1 つでも true か(全部 false のオブジェクトは送らない)。 */
function hasSpeedField(speedField: ComboSpeedField | undefined): speedField is ComboSpeedField {
  return (
    speedField !== undefined &&
    (speedField.trickRoom === true ||
      speedField.selfTailwind === true ||
      speedField.opponentTailwind === true)
  );
}

/**
 * 連続技リクエスト。手は先頭から MAX_COMBO_STEPS 件までに絞る(画面は上限で「次の技」を出さないので通常は絞られない。
 * 超えて渡しても境界の 400 にせず、決定的に先頭から取る。limitToMax の規約)。
 */
export function buildComboRequest(input: BuildComboRequestInput): ComboRequest {
  const { attacker, defender, steps, typeChart, field, opponentMove, opponentSpeed, speedField } = input;
  return {
    format: "single",
    attacker,
    defender,
    steps: limitToMax(steps, MAX_COMBO_STEPS).values.map(normalizeStep),
    typeChart,
    ...(field === undefined ? {} : { field }),
    ...(opponentMove === undefined ? {} : { opponentMove }),
    ...(opponentSpeed === undefined ? {} : { opponentSpeed }),
    ...(hasSpeedField(speedField) ? { speedField } : {}),
  };
}
