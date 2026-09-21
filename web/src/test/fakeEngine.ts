// テスト専用: 画面テストで使う fake の CalcEngine(ADR-0016 §8「単体・画面」層。engine は fake)。
// 画面が組み立てたリクエストを記録し、決め打ちの結果を返す。計算の正しさ(数値)は Web では見ないので、
// 返す数値は表示の確認に都合のよい架空の値でよい。

import type {
  BulkRequest,
  BulkResult,
  BulkRow,
  CalcEngine,
  CalcRequest,
  CalcResult,
  EngineResult,
  ReverseRequest,
  ReverseResult,
} from "../engine/types";

/** engine の既定プリセット(ADR-0009 §1。物理技の5行)。fake の行の Key と表示名に使う。 */
export const physicalPresetRows: ReadonlyArray<{ preset: string; presetLabel: string }> = [
  { preset: "none", presetLabel: "無振り" },
  { preset: "hp", presetLabel: "H振り" },
  { preset: "hb_boost", presetLabel: "H振り+B補正" },
  { preset: "hb", presetLabel: "HB振り" },
  { preset: "hb_full", presetLabel: "HB特化" },
];

export interface RowSpec {
  readonly preset?: string;
  readonly presetLabel?: string;
  readonly itemId?: string;
  readonly minPercent?: number;
  readonly maxPercent?: number;
  readonly effectiveness?: number;
  readonly ko?: CalcResult["ko"];
}

const zeroStats = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

/** 決め打ちの計算結果1件。 */
export function calcResult(spec: RowSpec = {}): CalcResult {
  return {
    rolls: Array.from({ length: 16 }, (_, index) => 60 + index),
    minDamage: 60,
    maxDamage: 75,
    minPercent: spec.minPercent ?? 72.1,
    maxPercent: spec.maxPercent ?? 85.3,
    defenderHP: 88,
    effectiveness: spec.effectiveness ?? 1,
    stab: false,
    category: "physical",
    ko: spec.ko ?? { hits: 2, guaranteed: true, chancePercent: 0, displayChancePercent: 100 },
  };
}

/** 決め打ちの一括計算の行1件。 */
export function bulkRow(spec: RowSpec = {}): BulkRow {
  return {
    preset: spec.preset ?? "none",
    presetLabel: spec.presetLabel ?? "無振り",
    itemId: spec.itemId ?? "",
    defender: {
      sp: zeroStats,
      nature: { plus: "", minus: "" },
      stats: { hp: 175, atk: 100, def: 100, spa: 100, spd: 100, spe: 100 },
    },
    result: calcResult(spec),
  };
}

/**
 * リクエストに応じた一括計算の結果(engine の形をまねる): 持ち物バリアントごとに既定5行。
 * itemVariants が無ければ5行、あれば 5 × バリアント数。null のバリアントは itemId ""。
 */
export function bulkResultFor(request: BulkRequest): BulkResult {
  const variants = request.itemVariants ?? [null];
  const rows = variants.flatMap((item) =>
    physicalPresetRows.map((preset) => bulkRow({ ...preset, itemId: item?.id ?? "" })),
  );
  return { defenderSpeciesKey: request.defenderSpecies.key, rows };
}

/** EngineResult の成功側を作る(テストの fixture 用)。 */
export function ok<T>(value: T): EngineResult<T> {
  return { ok: true, value };
}

/** EngineResult の失敗側を作る(テストの fixture 用。code・message は境界のエラー封筒と同じ形。ADR-0011 §5)。 */
export function engineError<T>(code: string, message: string): EngineResult<T> {
  return { ok: false, error: { code, message } };
}

export interface FakeEngine extends CalcEngine {
  /** calcBulk に渡されたリクエスト(呼ばれた順)。 */
  readonly bulkRequests: BulkRequest[];
  readonly calcRequests: CalcRequest[];
  readonly reverseRequests: ReverseRequest[];
}

type BulkResponder = (request: BulkRequest) => EngineResult<BulkResult>;

/** すぐに応答する fake。応答は respond で差し替えられる(既定は bulkResultFor の成功)。 */
export function createFakeEngine(
  respond: BulkResponder = (request) => ok(bulkResultFor(request)),
): FakeEngine {
  const bulkRequests: BulkRequest[] = [];
  const calcRequests: CalcRequest[] = [];
  const reverseRequests: ReverseRequest[] = [];
  return {
    bulkRequests,
    calcRequests,
    reverseRequests,
    calcBulk(request) {
      bulkRequests.push(request);
      return Promise.resolve(respond(request));
    },
    calc(request) {
      calcRequests.push(request);
      return Promise.resolve(ok(calcResult()));
    },
    calcReverse(request): Promise<EngineResult<ReverseResult>> {
      reverseRequests.push(request);
      return Promise.resolve(engineError("not_used", "画面テストでは逆算を使わない"));
    },
  };
}

export interface PendingBulk {
  readonly request: BulkRequest;
  resolve(result: EngineResult<BulkResult>): void;
}

/** 応答をテストが好きな順に返せる fake(古い応答が新しい表示を上書きしないことの確認用)。 */
export function createDeferredEngine(): { engine: FakeEngine; pending: PendingBulk[] } {
  const pending: PendingBulk[] = [];
  const base = createFakeEngine();
  const engine: FakeEngine = {
    ...base,
    calcBulk(request) {
      base.bulkRequests.push(request);
      return new Promise((resolve) => {
        pending.push({ request, resolve });
      });
    },
  };
  return { engine, pending };
}
