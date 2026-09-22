// P4-2: WASM の計算実装 createWasmEngine(ADR-0300 §2、境界の契約は ADR-0011 §2〜§5)。
// wasm_exec.js(Go ランタイム)を起動し、engine/cmd/wasm が登録する globalThis.pokecalc の3関数を
// JSON 文字列1つで呼ぶ(ADR-0011 §2)。読み込みは初回の計算時に1回だけ行い、同時の初回呼び出しも
// 1回の読み込みを共有する。読み込みに失敗しても例外にせず、次の計算で読み込みをやり直せるようにする
// (一時的な通信失敗からの回復。CLAUDE.md 絶対ルール5「計算はイベント保存に依存しない」と同じ精神)。

import type {
  BulkRequest,
  BulkResult,
  CalcEngine,
  CalcRequest,
  CalcResult,
  EngineResult,
  ReverseRequest,
  ReverseResult,
} from "./types";

/** ブラウザ/Node の違いを注入する読み込み口(ADR-0300 §2)。 */
export interface WasmLoader {
  /** wasm_exec.js(Go ランタイム)を読み込み、globalThis.Go を使えるようにする。 */
  loadRuntime(): Promise<void>;
  /** engine.wasm を実体化する。importObject は Go インスタンスが要求する輸入オブジェクト。 */
  instantiate(importObject: WebAssembly.Imports): Promise<WebAssembly.Instance>;
}

type BoundaryFunction = "calc" | "calcBulk" | "calcReverse";

/** globalThis.pokecalcReady のポーリング間隔(ミリ秒)。Go の起動は数十〜数百msなので十分細かい。 */
const READY_POLL_INTERVAL_MS = 5;

/**
 * globalThis.pokecalcReady の起動待ちの上限(ミリ秒)。実機の起動は数十〜数百msだが、
 * 遅い環境(CI・低速端末)でも起動失敗(pokecalcReady が永久に立たない・run が pending のまま)を
 * 無限待ちにせず確実に engine_unavailable として扱えるよう、通常の起動時間の一桁以上の余裕を持たせる。
 */
export const READY_TIMEOUT_MS = 5_000;

/**
 * Go プログラムを起動し、pokecalc の登録(pokecalcReady)を待つ。
 * 失敗(読み込み・実体化・起動・タイムアウト)は例外にする。呼び出し側(ensureLoaded)が engine_unavailable にする。
 * ready・exited・timeout のどれか1つで決着したら、残りのタイマー(ポーリング・タイムアウト)を必ず止める
 * (決着後もポーリングが回り続けるとタイマーリークになる)。
 */
async function startRuntime(loader: WasmLoader): Promise<void> {
  await loader.loadRuntime();
  const GoCtor = globalThis.Go;
  if (GoCtor === undefined) {
    throw new Error("wasm_exec.js を読み込んだが globalThis.Go が無い");
  }
  const go = new GoCtor();
  const instance = await loader.instantiate(go.importObject);

  // race が決着したら true にし、ポーリングの再スケジュールを止める。
  let decided = false;
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  const ready = new Promise<void>((resolve) => {
    const check = (): void => {
      if (decided) {
        return;
      }
      if (globalThis.pokecalcReady === true) {
        resolve();
        return;
      }
      pollTimer = setTimeout(check, READY_POLL_INTERVAL_MS);
    };
    check();
  });

  // 本物の Go プログラムは select{} で待機し続け、run の Promise は解決しない。
  // 登録前に run が解決/失敗したら、起動に失敗したとみなす。
  const exited = go.run(instance).then(() => {
    throw new Error("Go プログラムが pokecalc を登録する前に終了した");
  });
  // race の他の枝(ready・timeout)が先に決着した後で exited が失敗しても、
  // unhandled rejection として報告されないようにする(race 自体はどの枝が勝っても正しく伝える)。
  exited.catch(() => undefined);

  let timeoutTimer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_resolve, reject) => {
    timeoutTimer = setTimeout(() => {
      reject(new Error(`engine.wasm が ${String(READY_TIMEOUT_MS)}ms 以内に起動しなかった`));
    }, READY_TIMEOUT_MS);
  });

  try {
    await Promise.race([ready, exited, timeout]);
  } finally {
    decided = true;
    if (pollTimer !== undefined) {
      clearTimeout(pollTimer);
    }
    if (timeoutTimer !== undefined) {
      clearTimeout(timeoutTimer);
    }
  }
}

function unavailable<T>(cause: unknown): EngineResult<T> {
  const message = cause instanceof Error ? cause.message : String(cause);
  return {
    ok: false,
    error: { code: "engine_unavailable", message: `engine.wasm を利用できない: ${message}` },
  };
}

function invalidResponse<T>(message: string): EngineResult<T> {
  return { ok: false, error: { code: "invalid_response", message } };
}

/** {result} / {error:{code,message}} の封筒を EngineResult に写す(ADR-0011 §5)。壊れた応答は例外にしない。 */
function parseEnvelope<T>(responseJSON: string): EngineResult<T> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(responseJSON) as unknown;
  } catch {
    return invalidResponse("engine の応答が JSON として壊れている");
  }
  if (typeof parsed !== "object" || parsed === null) {
    return invalidResponse("engine の応答がオブジェクトでない");
  }
  if ("error" in parsed) {
    const err = parsed.error;
    if (typeof err === "object" && err !== null && "code" in err && "message" in err) {
      const { code, message } = err;
      if (typeof code === "string" && typeof message === "string") {
        return { ok: false, error: { code, message } };
      }
    }
    return invalidResponse("engine のエラー応答の形が不正(code / message が無い)");
  }
  if ("result" in parsed) {
    return { ok: true, value: (parsed as { result: T }).result };
  }
  return invalidResponse("engine の応答に result も error も無い");
}

/**
 * WASM 実装の CalcEngine(ADR-0300 §2)。loader はブラウザ(browserWasmLoader)と
 * Node の結合テスト(ファイルから読む loader)で差し替える。
 */
export function createWasmEngine(loader: WasmLoader): CalcEngine {
  // 読み込みは1つの createWasmEngine の呼び出しごとに独立して持つ(モジュール全体で共有しない)。
  let loading: Promise<void> | null = null;

  function ensureLoaded(): Promise<void> {
    if (loading === null) {
      loading = startRuntime(loader).catch((error: unknown) => {
        // 失敗はキャッシュしない。次の計算で読み込みをやり直せるようにする。
        loading = null;
        throw error;
      });
    }
    return loading;
  }

  async function callBoundary<T>(fn: BoundaryFunction, request: unknown): Promise<EngineResult<T>> {
    try {
      await ensureLoaded();
    } catch (error) {
      return unavailable(error);
    }
    const api = globalThis.pokecalc;
    if (api === undefined) {
      return unavailable(new Error("pokecalc が登録されていない"));
    }
    let responseJSON: string;
    try {
      responseJSON = api[fn](JSON.stringify(request));
    } catch (error) {
      return unavailable(error);
    }
    return parseEnvelope<T>(responseJSON);
  }

  return {
    calc: (request: CalcRequest) => callBoundary<CalcResult>("calc", request),
    calcBulk: (request: BulkRequest) => callBoundary<BulkResult>("calcBulk", request),
    calcReverse: (request: ReverseRequest) => callBoundary<ReverseResult>("calcReverse", request),
  };
}
