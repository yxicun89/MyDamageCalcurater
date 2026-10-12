// CB-6: calcComboVia は calcCombo を持つ engine に委譲し、持たない engine(既存の fake 等)では例外にせず
// engine_unavailable の not ok を返す。

import { expect, test, vi } from "vitest";
import { createFakeEngine } from "../test/fakeEngine";
import { calcComboVia, type CalcEngine, type ComboRequest, type ComboResult } from "./types";

const request = { format: "single", steps: [] } as unknown as ComboRequest;

test("calcCombo を持たない engine は engine_unavailable(reject しない)", async () => {
  const engine: CalcEngine = createFakeEngine();
  expect(engine.calcCombo).toBeUndefined();
  expect(await calcComboVia(engine, request)).toMatchObject({
    ok: false,
    error: { code: "engine_unavailable" },
  });
});

test("calcCombo を持つ engine へ request と signal をそのまま渡す", async () => {
  const value = { steps: [] } as unknown as ComboResult;
  const calcCombo = vi.fn(() => Promise.resolve({ ok: true as const, value }));
  const engine: CalcEngine = { ...createFakeEngine(), calcCombo };
  const signal = new AbortController().signal;
  expect(await calcComboVia(engine, request, signal)).toEqual({ ok: true, value });
  expect(calcCombo).toHaveBeenCalledWith(request, signal);
});
