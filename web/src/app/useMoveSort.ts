// I-web-9 = F-02(ADR-0335 §4): 技の並びを計算画面と逆算画面で共有するフック(useSyncExternalStore)。
// mount したまま隠れているタブ(ADR-0308)にも、どちらで変えても反映する。保存先は moveSortStorage。

import { useCallback, useSyncExternalStore } from "react";
import type { MoveSortOrder } from "../domain/moveSort";
import { loadMoveSort, saveMoveSort } from "./moveSortStorage";

const listeners = new Set<() => void>();
// ストレージに書けなかったときだけ、画面の中で切り替えられるよう覚えておく(購読者がいなくなったら捨てる)。
let unsaved: MoveSortOrder | null = null;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  const onStorage = (): void => {
    listener();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
    if (listeners.size === 0) {
      unsaved = null;
    }
  };
}

function getSnapshot(): MoveSortOrder {
  return unsaved ?? loadMoveSort();
}

export function useMoveSort(): readonly [MoveSortOrder, (order: MoveSortOrder) => void] {
  const order = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  const setOrder = useCallback((next: MoveSortOrder) => {
    saveMoveSort(next);
    unsaved = loadMoveSort() === next ? null : next;
    for (const listener of listeners) {
      listener();
    }
  }, []);
  return [order, setOrder];
}
