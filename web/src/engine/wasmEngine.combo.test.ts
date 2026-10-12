// CB-6: createWasmEngine.calcCombo は globalThis.pokecalc.calcCombo に ComboRequest を JSON 文字列で渡し、
// {result} / {error} の封筒を EngineResult に写す(ADR-0011 §5)。Go ランタイムと pokecalc は fake。

import { afterEach, describe, expect, test } from "vitest";
import { REQUEST_ABORTED_CODE, type ComboRequest, type ComboResult } from "./types";
import { createWasmEngine, type WasmLoader } from "./wasmEngine";

const globals = globalThis as Record<string, unknown>;

/** calcCombo の応答を差し替えられる fake の読み込み口。呼ばれた引数を calls に残す。 */
function fakeLoader(
  respond: (requestJSON: string) => string,
  hasCalcCombo = true,
): { loader: WasmLoader; calls: string[] } {
  const calls: string[] = [];
  // hasCalcCombo が false は calcCombo を登録しない古い engine.wasm。
  const api = hasCalcCombo
    ? {
        calcCombo: (requestJSON: string) => {
          calls.push(requestJSON);
          return respond(requestJSON);
        },
      }
    : {};
  class FakeGo {
    readonly importObject = { go: {} };
    run(): Promise<void> {
      globals.pokecalc = api;
      globals.pokecalcReady = true;
      return new Promise<void>(() => undefined);
    }
  }
  const loader: WasmLoader = {
    loadRuntime() {
      globals.Go = FakeGo;
      return Promise.resolve();
    },
    instantiate() {
      return Promise.resolve({ exports: {} } as WebAssembly.Instance);
    },
  };
  return { loader, calls };
}

const move = {
  id: "dance",
  nameJa: "まい",
  type: "normal",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1 } },
} as const;
const request = {
  format: "single",
  steps: [{ move }, { move, critical: true, hits: 2 }],
  opponentSpeed: "from_defender",
  typeChart: { types: [], effectiveness: {} },
} as unknown as ComboRequest;
const comboResult = { steps: [], unsupported: [] } as unknown as ComboResult;

afterEach(() => {
  for (const name of ["Go", "pokecalc", "pokecalcReady"]) {
    Reflect.deleteProperty(globalThis, name);
  }
});

describe("calcCombo", () => {
  test("リクエストをそのまま JSON にして渡し、result を ok の value にする", async () => {
    const { loader, calls } = fakeLoader(() => JSON.stringify({ result: comboResult }));
    const result = await createWasmEngine(loader).calcCombo(request);
    expect(result).toEqual({ ok: true, value: comboResult });
    expect(calls).toHaveLength(1);
    // effect は camelCase のまま、steps の critical・hits も境界の DTO のキーのまま渡る。
    expect(JSON.parse(calls[0] ?? "")).toEqual(request);
  });

  test("error の封筒は code・message のまま not ok", async () => {
    const { loader } = fakeLoader(() =>
      JSON.stringify({ error: { code: "invalid_input", message: "steps は 1..3 手でなければならない" } }),
    );
    expect(await createWasmEngine(loader).calcCombo(request)).toEqual({
      ok: false,
      error: { code: "invalid_input", message: "steps は 1..3 手でなければならない" },
    });
  });

  test("壊れた応答は invalid_response", async () => {
    const { loader } = fakeLoader(() => "not json");
    expect(await createWasmEngine(loader).calcCombo(request)).toMatchObject({
      ok: false,
      error: { code: "invalid_response" },
    });
  });

  test("calcCombo を登録していない古い engine.wasm は例外にせず engine_unavailable", async () => {
    const { loader } = fakeLoader(() => "", false);
    expect(await createWasmEngine(loader).calcCombo(request)).toMatchObject({
      ok: false,
      error: { code: "engine_unavailable" },
    });
  });

  test("abort 済みの signal は呼ばずに request_aborted", async () => {
    const { loader, calls } = fakeLoader(() => JSON.stringify({ result: comboResult }));
    const controller = new AbortController();
    controller.abort();
    expect(await createWasmEngine(loader).calcCombo(request, controller.signal)).toMatchObject({
      ok: false,
      error: { code: REQUEST_ABORTED_CODE },
    });
    expect(calls).toHaveLength(0);
  });
});
