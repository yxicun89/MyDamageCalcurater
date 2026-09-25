// テスト専用: 画面テストで使う fake の CalcEngine(ADR-0300 §8「単体・画面」層。engine は fake)。
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
  ReverseCandidate,
  ReverseRequest,
  ReverseResult,
  StatKey,
  UnsupportedMark,
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
  /** 「未対応」の印(ADR-0123)。既定は印なし(空配列)で、既存のテストの見た目は変わらない。 */
  readonly unsupported?: readonly UnsupportedMark[];
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
    unsupported: spec.unsupported ?? [],
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

/** 決め打ちの逆算候補1件(P4-4)。指定しないフィールドは表示の確認に都合のよい架空の値。 */
export function reverseCandidate(spec: Partial<ReverseCandidate> = {}): ReverseCandidate {
  return {
    natureClass: "neutral",
    nature: { plus: "", minus: "" },
    itemId: "",
    ranges: [{ min: 10, max: 12 }],
    spCount: 3,
    exact: true,
    mismatch: 0,
    support: 4,
    minPercent: 40.2,
    maxPercent: 47.8,
    unsupported: [],
    ...spec,
  };
}

/** 逆算の対象側と技の分類から、engine が逆算する関連ステータス(ADR-0010 §2 の写し。fake 用)。 */
function reverseStatFor(request: ReverseRequest): StatKey {
  const special = request.move.category === "special";
  if (request.side === "attacker") {
    return special ? "spa" : "atk";
  }
  return special ? "spd" : "def";
}

/**
 * リクエストに応じた逆算の結果(engine の形をまねる): 性格クラス(neutral → plus)× 持ち物候補の順に候補を返す。
 * itemCandidates が無ければ持ち物なしの1通り。防御側は H32 前提(assumedHpSp 32)、攻撃側は 0。
 */
export function reverseResultFor(request: ReverseRequest): ReverseResult {
  const items = request.itemCandidates ?? [null];
  const candidates = (["neutral", "plus"] as const).flatMap((natureClass) =>
    items.map((item) => reverseCandidate({ natureClass, itemId: item?.id ?? "" })),
  );
  return {
    side: request.side,
    stat: reverseStatFor(request),
    assumedHpSp: request.side === "defender" ? 32 : 0,
    exactCount: candidates.length,
    candidates,
  };
}

export interface FakeEngine extends CalcEngine {
  /** calcBulk に渡されたリクエスト(呼ばれた順)。 */
  readonly bulkRequests: BulkRequest[];
  readonly calcRequests: CalcRequest[];
  readonly reverseRequests: ReverseRequest[];
  /** calcReverse に渡された signal(reverseRequests と同じ順。渡されなければ undefined。issue 113)。 */
  readonly reverseSignals: Array<AbortSignal | undefined>;
  /** calcBulk に渡された signal(bulkRequests と同じ順。issue 113)。 */
  readonly bulkSignals: Array<AbortSignal | undefined>;
}

type BulkResponder = (request: BulkRequest) => EngineResult<BulkResult>;
type ReverseResponder = (request: ReverseRequest) => EngineResult<ReverseResult>;

/**
 * すぐに応答する fake。応答は respond で差し替えられる(既定は bulkResultFor の成功)。
 * 逆算(P4-4)の応答は respondReverse で差し替えられる(既定は reverseResultFor の成功)。
 */
export function createFakeEngine(
  respond: BulkResponder = (request) => ok(bulkResultFor(request)),
  respondReverse: ReverseResponder = (request) => ok(reverseResultFor(request)),
): FakeEngine {
  const bulkRequests: BulkRequest[] = [];
  const calcRequests: CalcRequest[] = [];
  const reverseRequests: ReverseRequest[] = [];
  const reverseSignals: Array<AbortSignal | undefined> = [];
  const bulkSignals: Array<AbortSignal | undefined> = [];
  return {
    bulkRequests,
    calcRequests,
    reverseRequests,
    reverseSignals,
    bulkSignals,
    calcBulk(request, signal) {
      bulkRequests.push(request);
      bulkSignals.push(signal);
      return Promise.resolve(respond(request));
    },
    calc(request) {
      calcRequests.push(request);
      return Promise.resolve(ok(calcResult()));
    },
    calcReverse(request, signal): Promise<EngineResult<ReverseResult>> {
      reverseRequests.push(request);
      reverseSignals.push(signal);
      return Promise.resolve(respondReverse(request));
    },
  };
}

export interface PendingBulk {
  readonly request: BulkRequest;
  /** 画面が渡した取り消しの signal(issue 248。渡されなければ undefined)。 */
  readonly signal: AbortSignal | undefined;
  resolve(result: EngineResult<BulkResult>): void;
}

/** 応答をテストが好きな順に返せる fake(古い応答が新しい表示を上書きしないこと・signal の確認用)。 */
export function createDeferredEngine(): { engine: FakeEngine; pending: PendingBulk[] } {
  const pending: PendingBulk[] = [];
  const base = createFakeEngine();
  const engine: FakeEngine = {
    ...base,
    calcBulk(request, signal) {
      base.bulkRequests.push(request);
      base.bulkSignals.push(signal);
      return new Promise((resolve) => {
        pending.push({ request, signal, resolve });
      });
    },
  };
  return { engine, pending };
}

export interface PendingReverse {
  readonly request: ReverseRequest;
  /** 画面が渡した取り消しの signal(issue 113。渡されなければ undefined)。 */
  readonly signal: AbortSignal | undefined;
  resolve(result: EngineResult<ReverseResult>): void;
}

/** 逆算の応答をテストが好きな順に返せる fake(P4-4。古い応答が新しい表示を上書きしないことの確認用)。 */
export function createDeferredReverseEngine(): { engine: FakeEngine; pending: PendingReverse[] } {
  const pending: PendingReverse[] = [];
  const base = createFakeEngine();
  const engine: FakeEngine = {
    ...base,
    calcReverse(request, signal) {
      base.reverseRequests.push(request);
      base.reverseSignals.push(signal);
      return new Promise((resolve) => {
        pending.push({ request, signal, resolve });
      });
    },
  };
  return { engine, pending };
}
