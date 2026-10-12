// テスト専用(CB-6 段階 B): 連続技(calcCombo)を持つ fake の CalcEngine。
// 数値の正しさは見ない(engine / API の側で検証済み)。画面が組み立てた要求の記録と、表示の確認に都合のよい決め打ちの応答だけ。

import type { CalcEngine, ComboRequest, ComboResult, EngineResult, UnsupportedMark } from "../engine/types";
import { calcResult, createFakeEngine, ok, type FakeEngine } from "./fakeEngine";

const zeroRanks = { atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

/** 生成型(openapi.gen.ts)は配列が可変なので、readonly の CalcResult を写し直す。 */
function genCalc(
  spec: Parameters<typeof calcResult>[0],
): NonNullable<ComboResult["steps"][number]["damage"]> {
  const result = calcResult(spec);
  return {
    ...result,
    rolls: [...result.rolls],
    unsupported: result.unsupported.map((mark) => ({ ...mark })),
  };
}

function koChance(display: number, guaranteed = false): ComboResult["total"]["ko"] {
  return { chancePercent: display, guaranteed, displayChancePercent: display };
}

export interface ComboResponseSpec {
  readonly ko?: ComboResult["total"]["ko"];
  readonly firstSelfSpeed?: number;
  readonly lastSelfSpeed?: number;
  readonly opponentSpeedKnown?: boolean;
  readonly unsupported?: readonly UnsupportedMark[];
  readonly surviveChancePercent?: number;
}

/** 要求に応じた連続技の応答(engine の形をまねる)。変化技の手は damage が null。 */
export function comboResultFor(request: ComboRequest, spec: ComboResponseSpec = {}): ComboResult {
  const first = spec.firstSelfSpeed ?? 132;
  const last = spec.lastSelfSpeed ?? 198;
  const steps = request.steps.map((step, index) => {
    const selfSpeed = index === 0 ? first : last;
    return {
      moveId: step.move.id,
      selfStartRanks: zeroRanks,
      selfEndRanks: zeroRanks,
      opponentStartRanks: zeroRanks,
      opponentEndRanks: zeroRanks,
      damage: step.move.category === "status" ? null : genCalc({ minPercent: 30.1, maxPercent: 35.4 }),
      selfSpeed,
      opponentSpeed: 0,
      selfPriority: step.move.priority,
      opponentPriority: 0,
      order: "depends_on_opponent_speed" as const,
      line: {
        selfSpeed,
        selfFirst: { min: 0, max: selfSpeed - 1 },
        opponentFirst: { min: selfSpeed + 1, max: 999 },
      },
    };
  });
  const ko = spec.ko ?? koChance(51.6);
  const scenario = {
    actions: [
      {
        step: 0,
        timing: "before_self" as const,
        damage: genCalc({ minPercent: 40.2, maxPercent: 47.8 }),
        surviveChancePercent: spec.surviveChancePercent ?? 100,
      },
    ],
    surviveChancePercent: spec.surviveChancePercent ?? 100,
  };
  return {
    steps,
    total: {
      minDamage: 169,
      maxDamage: 201,
      minPercent: 91.4,
      maxPercent: 108.6,
      defenderHP: 185,
      ko,
      koByStep: request.steps.map((_, index) => (index === request.steps.length - 1 ? ko : koChance(0))),
    },
    speed: {
      firstSelfSpeed: first,
      lastSelfSpeed: last,
      opponentSpeed: spec.opponentSpeedKnown === true ? 169 : 0,
      opponentSpeedKnown: spec.opponentSpeedKnown === true,
    },
    opponentAttack:
      request.opponentMove === undefined
        ? null
        : { selfHp: 132, minActions: 1, maxActions: 2, fast: scenario, slow: scenario },
    unsupported: (spec.unsupported ?? []).map((mark) => ({ ...mark })),
  };
}

export interface ComboFakeEngine extends FakeEngine {
  readonly comboRequests: ComboRequest[];
  readonly comboSignals: Array<AbortSignal | undefined>;
}

/** calcCombo を持つ fake。respond で応答を差し替えられる(既定は comboResultFor)。 */
export function createComboEngine(
  respond: (request: ComboRequest) => EngineResult<ComboResult> = (request) => ok(comboResultFor(request)),
  base: FakeEngine = createFakeEngine(),
): ComboFakeEngine {
  const comboRequests: ComboRequest[] = [];
  const comboSignals: Array<AbortSignal | undefined> = [];
  const engine: CalcEngine = {
    ...base,
    calcCombo(request, signal) {
      comboRequests.push(request);
      comboSignals.push(signal);
      return Promise.resolve(respond(request));
    },
  };
  return { ...base, ...engine, comboRequests, comboSignals };
}

export interface PendingCombo {
  readonly request: ComboRequest;
  readonly signal: AbortSignal | undefined;
  resolve(result: EngineResult<ComboResult>): void;
}

/** 応答を好きな順に返せる fake(古い応答の無視・abort の確認用)。 */
export function createDeferredComboEngine(): { engine: ComboFakeEngine; pending: PendingCombo[] } {
  const pending: PendingCombo[] = [];
  const engine = createComboEngine();
  const deferred: ComboFakeEngine = {
    ...engine,
    calcCombo(request, signal) {
      engine.comboRequests.push(request);
      engine.comboSignals.push(signal);
      return new Promise((resolve) => {
        pending.push({ request, signal, resolve });
      });
    },
  };
  return { engine: deferred, pending };
}
