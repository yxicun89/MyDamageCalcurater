// I-web-9 = F-02(ADR-0335 §4): 技の並びの保存。計算モード(calcMode.ts)と同じ作法で localStorage の1キーに覚える。
// ストレージが使えない・壊れた値でも失敗させず既定(習得順)に戻す。

import { DEFAULT_MOVE_SORT_ORDER, isMoveSortOrder, type MoveSortOrder } from "../domain/moveSort";
import { defaultStorage, type StorageLike } from "./storage";

export const MOVE_SORT_STORAGE_KEY = "pokecalc.moveSort";

export function loadMoveSort(storage: StorageLike | null = defaultStorage()): MoveSortOrder {
  if (storage === null) {
    return DEFAULT_MOVE_SORT_ORDER;
  }
  try {
    const stored = storage.getItem(MOVE_SORT_STORAGE_KEY);
    return isMoveSortOrder(stored) ? stored : DEFAULT_MOVE_SORT_ORDER;
  } catch {
    return DEFAULT_MOVE_SORT_ORDER;
  }
}

export function saveMoveSort(order: MoveSortOrder, storage: StorageLike | null = defaultStorage()): void {
  if (storage === null) {
    return;
  }
  try {
    storage.setItem(MOVE_SORT_STORAGE_KEY, order);
  } catch {
    // 保存できなくても失敗させない(プライベートブラウズ・容量超過など)。
  }
}
