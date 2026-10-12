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
  Nature,
  Ranks,
  TypeChart,
} from "../engine/types";
import { limitToMax } from "./requestLimits";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "./requests";

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

/** 自分の素早さの性格補正の指定。keep = 計算画面の性格のまま(素早さの補正も触らない)。 */
export type SelfSpeedNature = "keep" | "plus" | "neutral" | "minus";

/**
 * 連続技の「自分の素早さ」の入力(I-web-32、ADR-0359 追記)。API・engine の連続技は素早さを攻撃側の個体
 * (SP・性格・ランク)から導くので、この入力は攻撃側の個体の素早さに関わる項目だけを上書きする。
 * こだわりスカーフは API の連続技に指定する項目が無い(持ち物は 1 つで、持ち物 ID で渡す)ので、ここには持たない。
 */
export interface SelfSpeedInput {
  readonly sp: number;
  readonly nature: SelfSpeedNature;
  readonly rank: number;
}

/** 既定。applySelfSpeed が個体をそのまま返す値(今の挙動と同一)。 */
export const DEFAULT_SELF_SPEED: SelfSpeedInput = { sp: 0, nature: "keep", rank: 0 };

/** 素早さに振れる SP の上限(1 能力 32・合計 66 から、素早さ以外の SP を引いた残り)。 */
export function maxSelfSpeedSp(attacker: Individual): number {
  const { hp, atk, def, spa, spd } = attacker.sp;
  const used = hp + atk + def + spa + spd;
  return Math.max(0, Math.min(MAX_SP_PER_STAT, MAX_SP_TOTAL - used));
}

/** 素早さを下げる性格の「上がる能力」の既定(攻撃側の性格が素早さ以外に上昇を持たないとき)。 */
const FALLBACK_PLUS_FOR_MINUS = "def";

function fallbackMinusForPlus(attackerPlus: Nature["plus"]): Nature["minus"] {
  // いじっぱり型(攻撃↑)は特攻↓、ひかえめ型(特攻↑)は攻撃↓にして、攻撃側の得意な能力を削らない。
  return attackerPlus === "spa" ? "atk" : "spa";
}

function applySpeedNature(nature: Nature, choice: Exclude<SelfSpeedNature, "keep">): Nature {
  const { plus, minus } = nature;
  switch (choice) {
    case "plus":
      return { plus: "spe", minus: minus === "" || minus === "spe" ? fallbackMinusForPlus(plus) : minus };
    case "minus":
      return { plus: plus === "" || plus === "spe" ? FALLBACK_PLUS_FOR_MINUS : plus, minus: "spe" };
    case "neutral":
      return plus === "spe" || minus === "spe" ? { plus: "", minus: "" } : nature;
  }
}

/**
 * 攻撃側の個体に、自分の素早さの入力を重ねる。既定のままなら同じ個体を返す。
 * 性格は素早さの補正だけを差し替え、攻撃・特攻の補正は(両立できる範囲で)残す。
 */
export function applySelfSpeed(attacker: Individual, input: SelfSpeedInput): Individual {
  const sp = Math.min(Math.max(0, Math.trunc(input.sp)), maxSelfSpeedSp(attacker));
  const spChanged = sp !== attacker.sp.spe;
  const rankChanged = input.rank !== (attacker.ranks?.spe ?? 0);
  if (!spChanged && !rankChanged && input.nature === "keep") {
    return attacker;
  }
  const ranks: Ranks = { atk: 0, def: 0, spa: 0, spd: 0, ...attacker.ranks, spe: input.rank };
  return {
    ...attacker,
    sp: { ...attacker.sp, spe: sp },
    ...(rankChanged || attacker.ranks !== undefined ? { ranks } : {}),
    ...(input.nature === "keep" ? {} : { nature: applySpeedNature(attacker.nature, input.nature) }),
  };
}
