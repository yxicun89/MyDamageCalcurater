// P4-5: 計算モード(オフライン = WASM / オンライン = API)の保存(ADR-0301 §4)。
// 既定はオフライン。選択は localStorage に覚える(端末ごとの好み)。ストレージが使えない・壊れた値でも失敗させない。

import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { CALC_MODE_STORAGE_KEY, DEFAULT_CALC_MODE, loadCalcMode, saveCalcMode } from "./calcMode";
import type { StorageLike } from "./storage";

const throwingStorage: StorageLike = {
  getItem() {
    throw new DOMException("denied", "SecurityError");
  },
  setItem() {
    throw new DOMException("denied", "QuotaExceededError");
  },
};

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  localStorage.clear();
});

describe("計算モードの保存", () => {
  test("既定はオフライン、保存先のキーは pokecalc.calcMode", () => {
    expect(DEFAULT_CALC_MODE).toBe("offline");
    expect(CALC_MODE_STORAGE_KEY).toBe("pokecalc.calcMode");
    expect(loadCalcMode()).toBe("offline");
  });

  test.each(["online", "offline"] as const)(
    "%s を保存すると localStorage に書き、次に読むと同じ値",
    (mode) => {
      saveCalcMode(mode);
      expect(localStorage.getItem(CALC_MODE_STORAGE_KEY)).toBe(mode);
      expect(loadCalcMode()).toBe(mode);
    },
  );

  test.each(["", "ONLINE", "api", "wasm", "null", '"online"'])(
    "未知の保存値 %j は既定(オフライン)として読む",
    (stored) => {
      localStorage.setItem(CALC_MODE_STORAGE_KEY, stored);
      expect(loadCalcMode()).toBe("offline");
    },
  );

  test("ストレージが例外を投げても、読むときは既定、書くときは例外にしない", () => {
    expect(loadCalcMode(throwingStorage)).toBe("offline");
    expect(() => {
      saveCalcMode("online", throwingStorage);
    }).not.toThrow();
  });

  test("ストレージを注入すると、そのストレージに読み書きする", () => {
    const saved = new Map<string, string>();
    const storage: StorageLike = {
      getItem: (key) => saved.get(key) ?? null,
      setItem: (key, value) => {
        saved.set(key, value);
      },
    };
    saveCalcMode("online", storage);
    expect(saved.get(CALC_MODE_STORAGE_KEY)).toBe("online");
    expect(loadCalcMode(storage)).toBe("online");
    expect(localStorage.getItem(CALC_MODE_STORAGE_KEY)).toBeNull();
  });
});
