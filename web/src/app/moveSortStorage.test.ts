// I-web-9 = F-02(ADR-0335 §4): 技の並びの保存。計算モード(calcMode.ts)と同じ作法で、
// localStorage の1キー(pokecalc.moveSort)に覚える。ストレージが使えない・壊れた値でも失敗させず既定(習得順)に戻す。

import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { DEFAULT_MOVE_SORT_ORDER, MOVE_SORT_ORDERS } from "../domain/moveSort";
import { MOVE_SORT_STORAGE_KEY, loadMoveSort, saveMoveSort } from "./moveSortStorage";
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

describe("技の並びの保存", () => {
  test("保存先のキーは pokecalc.moveSort、何も無ければ既定(習得順)", () => {
    expect(MOVE_SORT_STORAGE_KEY).toBe("pokecalc.moveSort");
    expect(loadMoveSort()).toBe(DEFAULT_MOVE_SORT_ORDER);
  });

  test.each(MOVE_SORT_ORDERS)("%s を保存すると localStorage に書き、次に読むと同じ値", (order) => {
    saveMoveSort(order);
    expect(localStorage.getItem(MOVE_SORT_STORAGE_KEY)).toBe(order);
    expect(loadMoveSort()).toBe(order);
  });

  test.each(["", "KANA", "alphabet", "kana ", "{}", "null"])("不正な値(%j)は既定に戻す", (bad) => {
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, bad);
    expect(loadMoveSort()).toBe(DEFAULT_MOVE_SORT_ORDER);
  });

  test("ストレージが null(使えない環境)でも、読むと既定・書いても例外にならない", () => {
    expect(loadMoveSort(null)).toBe(DEFAULT_MOVE_SORT_ORDER);
    expect(() => {
      saveMoveSort("kana", null);
    }).not.toThrow();
  });

  test("読み書きで例外を投げるストレージでも、読むと既定・書いても例外にならない", () => {
    expect(loadMoveSort(throwingStorage)).toBe(DEFAULT_MOVE_SORT_ORDER);
    expect(() => {
      saveMoveSort("type", throwingStorage);
    }).not.toThrow();
  });
});
