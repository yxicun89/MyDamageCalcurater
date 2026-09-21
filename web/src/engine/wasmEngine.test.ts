// P4-2: WASM の計算実装 createWasmEngine(ADR-0300 §2、境界の契約は ADR-0011 §2〜§5)。
// 本物の engine.wasm は使わず、Go ランタイム(wasm_exec.js の globalThis.Go)と
// engine/cmd/wasm が登録する globalThis.pokecalc を fake に置き換えて、次を確かめる:
//   - 読み込みは初回の計算時に1回だけ(同時の初回呼び出しも1回の読み込みを共有する)
//   - リクエストは JSON 文字列で渡し、{"result"} → ok、{"error":{code,message}} → not ok に写す
//   - 読み込みの失敗・壊れた応答は例外にせず、Web 側のエラーコードの not ok にする

import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { bulkResultFor, calcResult } from "../test/fakeEngine";
import type { BulkRequest, CalcRequest, ReverseRequest } from "./types";
import { createWasmEngine, READY_TIMEOUT_MS, type WasmLoader } from "./wasmEngine";

type BoundaryFunction = "calc" | "calcBulk" | "calcReverse";

interface FakeRuntimeOptions {
  /** 境界関数の応答(JSON 文字列)。既定は成功の封筒。 */
  readonly respond?: (fn: BoundaryFunction, requestJSON: string) => string;
  /** go.run の振る舞い: ready = 登録して待機し続ける / exit = 登録せずに終了 / reject = 異常終了。 */
  readonly run?: "ready" | "exit" | "reject";
  /** pokecalcReady が立つまでの遅延(ミリ秒)。 */
  readonly readyDelayMs?: number;
  readonly failLoadRuntimeTimes?: number;
  readonly failInstantiate?: boolean;
  /** pokecalcReady を一切立てない(起動タイムアウトを試すため。readyDelayMs とは併用しない)。 */
  readonly readyNever?: boolean;
}

interface FakeRuntime {
  readonly loader: WasmLoader;
  readonly counts: { loadRuntime: number; instantiate: number; goConstructed: number; run: number };
  /** 境界関数に渡された引数(呼ばれた順)。 */
  readonly calls: Array<{ fn: BoundaryFunction; args: unknown[] }>;
  /** instantiate に渡された importObject が、生成した Go の importObject と同じだったか。 */
  readonly importObjectsMatched: boolean[];
}

const globals = globalThis as Record<string, unknown>;

function defaultRespond(fn: BoundaryFunction, requestJSON: string): string {
  if (fn === "calcBulk") {
    return JSON.stringify({ result: bulkResultFor(JSON.parse(requestJSON) as BulkRequest) });
  }
  return JSON.stringify({ result: calcResult() });
}

function createFakeRuntime(options: FakeRuntimeOptions = {}): FakeRuntime {
  const counts = { loadRuntime: 0, instantiate: 0, goConstructed: 0, run: 0 };
  const calls: FakeRuntime["calls"] = [];
  const importObjectsMatched: boolean[] = [];
  const respond = options.respond ?? defaultRespond;
  let failuresLeft = options.failLoadRuntimeTimes ?? 0;
  let lastImportObject: WebAssembly.Imports | undefined;

  const api = Object.fromEntries(
    (["calc", "calcBulk", "calcReverse"] as const).map((fn) => [
      fn,
      (...args: unknown[]) => {
        calls.push({ fn, args });
        return respond(fn, String(args[0]));
      },
    ]),
  );

  class FakeGo {
    readonly importObject = { go: {} };
    constructor() {
      counts.goConstructed += 1;
      lastImportObject = this.importObject;
    }
    run(): Promise<void> {
      counts.run += 1;
      switch (options.run ?? "ready") {
        case "exit":
          return Promise.resolve();
        case "reject":
          return Promise.reject(new Error("fake の異常終了"));
        case "ready":
          if (options.readyNever !== true) {
            setTimeout(() => {
              globals.pokecalc = api;
              globals.pokecalcReady = true;
            }, options.readyDelayMs ?? 0);
          }
          // 本物の Go プログラムは select{} で待機し続け、run の Promise は解決しない。
          return new Promise<void>(() => undefined);
      }
    }
  }

  const loader: WasmLoader = {
    loadRuntime() {
      counts.loadRuntime += 1;
      if (failuresLeft > 0) {
        failuresLeft -= 1;
        return Promise.reject(new Error("wasm_exec.js の読み込みに失敗(fake)"));
      }
      globals.Go = FakeGo;
      return Promise.resolve();
    },
    instantiate(importObject) {
      counts.instantiate += 1;
      importObjectsMatched.push(lastImportObject !== undefined && importObject === lastImportObject);
      if (options.failInstantiate === true) {
        return Promise.reject(new Error("engine.wasm の読み込みに失敗(fake)"));
      }
      return Promise.resolve({ exports: {} } as WebAssembly.Instance);
    },
  };
  return { loader, counts, calls, importObjectsMatched };
}

// 中身は境界を素通しするだけなので、形が DTO に沿っていれば値は何でもよい。
const species = {
  key: "example-a",
  dexNo: 9001,
  form: 0,
  nameJa: "テストA",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 80, spa: 60, spd: 80, spe: 90 },
  abilities: [],
};
const individual = {
  species,
  level: 50,
  nature: { plus: "", minus: "" },
  ability: { id: "", nameJa: "", effect: null },
  item: null,
  sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};
const move = {
  id: "example-move",
  nameJa: "テスト技",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
};
const typeChart = { types: ["fire"], effectiveness: { fire: { fire: 1 } } };

const bulkRequest = {
  format: "single",
  attacker: individual,
  defenderSpecies: species,
  move,
  typeChart,
} as BulkRequest;
const calcRequest = {
  format: "single",
  attacker: individual,
  defender: individual,
  move,
  typeChart,
} as CalcRequest;
const reverseRequest = {
  format: "single",
  side: "defender",
  known: individual,
  unknownSpecies: species,
  move,
  observations: [{ percent: 50 }],
  typeChart,
} as ReverseRequest;

afterEach(() => {
  for (const name of ["Go", "pokecalc", "pokecalcReady"]) {
    Reflect.deleteProperty(globalThis, name);
  }
});

describe("読み込み", () => {
  test("作っただけでは wasm_exec.js も engine.wasm も読み込まない(初回の計算時に読む)", () => {
    const runtime = createFakeRuntime();
    createWasmEngine(runtime.loader);
    expect(runtime.counts).toEqual({ loadRuntime: 0, instantiate: 0, goConstructed: 0, run: 0 });
  });

  test("同時の初回呼び出しも含め、読み込み・起動は1回だけ", async () => {
    const runtime = createFakeRuntime();
    const engine = createWasmEngine(runtime.loader);
    const results = await Promise.all([
      engine.calcBulk(bulkRequest),
      engine.calc(calcRequest),
      engine.calcReverse(reverseRequest),
    ]);
    expect(results.map((result) => result.ok)).toEqual([true, true, true]);
    await engine.calcBulk(bulkRequest);
    expect(runtime.counts).toEqual({ loadRuntime: 1, instantiate: 1, goConstructed: 1, run: 1 });
  });

  test("engine.wasm は生成した Go の importObject で実体化する", async () => {
    const runtime = createFakeRuntime();
    await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(runtime.importObjectsMatched).toEqual([true]);
  });

  test("pokecalcReady が後から立っても、立つのを待ってから呼ぶ", async () => {
    const runtime = createFakeRuntime({ readyDelayMs: 30 });
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(result.ok).toBe(true);
    expect(runtime.calls.map((call) => call.fn)).toEqual(["calcBulk"]);
  });
});

describe("封筒の写し(ADR-0011 §5)", () => {
  test("リクエストを JSON 文字列1つで渡し、{result} を ok の value にする", async () => {
    const runtime = createFakeRuntime();
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(runtime.calls).toHaveLength(1);
    const args = runtime.calls[0]?.args ?? [];
    expect(args).toHaveLength(1);
    expect(typeof args[0]).toBe("string");
    expect(JSON.parse(String(args[0]))).toEqual(bulkRequest);
    expect(result).toEqual({ ok: true, value: bulkResultFor(bulkRequest) });
  });

  test.each([
    ["calc", (engine: ReturnType<typeof createWasmEngine>) => engine.calc(calcRequest), calcRequest],
    [
      "calcReverse",
      (engine: ReturnType<typeof createWasmEngine>) => engine.calcReverse(reverseRequest),
      reverseRequest,
    ],
  ] as const)("%s は同名の境界関数を呼ぶ", async (fn, call, request) => {
    const runtime = createFakeRuntime();
    await call(createWasmEngine(runtime.loader));
    expect(runtime.calls.map((entry) => entry.fn)).toEqual([fn]);
    expect(JSON.parse(String(runtime.calls[0]?.args[0]))).toEqual(request);
  });

  test("{error:{code,message}} は code と message をそのまま運ぶ not ok にする", async () => {
    const runtime = createFakeRuntime({
      respond: () =>
        JSON.stringify({ error: { code: "type_chart_missing", message: "リクエストに typeChart が無い" } }),
    });
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(result).toEqual({
      ok: false,
      error: { code: "type_chart_missing", message: "リクエストに typeChart が無い" },
    });
  });

  test.each([
    ["JSON でない", "not json"],
    ["result も error も無い", JSON.stringify({ unexpected: true })],
  ])("応答が壊れている(%s)ときは例外にせず invalid_response", async (_label, response) => {
    const runtime = createFakeRuntime({ respond: () => response });
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("invalid_response");
      expect(result.error.message.length).toBeGreaterThan(0);
    }
  });
});

describe("読み込みの失敗(Web 側のエラーコード engine_unavailable)", () => {
  test.each([
    ["wasm_exec.js の読み込み失敗", { failLoadRuntimeTimes: 1 }],
    ["engine.wasm の実体化の失敗", { failInstantiate: true }],
    ["Go プログラムが登録前に終了", { run: "exit" }],
    ["Go プログラムが異常終了", { run: "reject" }],
  ] as const)("%s は例外にせず engine_unavailable の not ok", async (_label, options) => {
    const runtime = createFakeRuntime(options);
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("engine_unavailable");
      expect(result.error.message.length).toBeGreaterThan(0);
    }
  });

  test("読み込みに失敗したら、次の計算で読み込みをやり直す(一時的な通信失敗から回復する)", async () => {
    const runtime = createFakeRuntime({ failLoadRuntimeTimes: 1 });
    const engine = createWasmEngine(runtime.loader);
    expect((await engine.calcBulk(bulkRequest)).ok).toBe(false);
    expect((await engine.calcBulk(bulkRequest)).ok).toBe(true);
    expect(runtime.counts.loadRuntime).toBe(2);
    expect(runtime.counts.run).toBe(1);
  });
});

describe("起動待ちのタイマー(決着したらポーリング・タイムアウトのタイマーを残さない)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  test.each([
    ["Go プログラムが登録前に終了", "exit"],
    ["Go プログラムが異常終了", "reject"],
  ] as const)("%s で決着したら、タイマーを1つも残さない", async (_label, run) => {
    const runtime = createFakeRuntime({ run });
    const result = await createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    expect(result.ok).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });

  test(`pokecalcReady が ${String(READY_TIMEOUT_MS)}ms 経っても立たなければ engine_unavailable にし、タイマーを残さない`, async () => {
    const runtime = createFakeRuntime({ readyNever: true });
    const resultPromise = createWasmEngine(runtime.loader).calcBulk(bulkRequest);
    await vi.advanceTimersByTimeAsync(READY_TIMEOUT_MS);
    const result = await resultPromise;
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("engine_unavailable");
    }
    expect(vi.getTimerCount()).toBe(0);
  });
});
