// ADR-0313: MasterCacheStore のメモリ fake(テスト用。fake-indexeddb 等の新規依存は入れない)。
// 失敗の注入(読み・書きが reject する、壊れた値を直接置く)ができる。

import type { MasterCacheRecord, MasterCacheStore } from "../master/cache/types";

export interface MemoryMasterCacheStore extends MasterCacheStore {
  /** 今保存されている値(未保存は null)。検査用。 */
  peek(): unknown;
  /** 型に合わない値(壊れたデータ)を直接置く。 */
  poke(value: unknown): void;
  /** 以降の save を reject させる(書き込み失敗。IndexedDB の容量超過・private mode の再現)。 */
  failSaves(): void;
  /** 以降の load を reject させる。 */
  failLoads(): void;
  readonly saveCount: () => number;
}

export function createMemoryMasterCacheStore(): MemoryMasterCacheStore {
  let value: unknown = null;
  let saveFails = false;
  let loadFails = false;
  let saves = 0;
  return {
    load() {
      return loadFails ? Promise.reject(new Error("load failed")) : Promise.resolve(structuredClone(value));
    },
    save(record: MasterCacheRecord) {
      saves += 1;
      if (saveFails) {
        return Promise.reject(new DOMException("quota", "QuotaExceededError"));
      }
      value = structuredClone(record);
      return Promise.resolve();
    },
    clear() {
      value = null;
      return Promise.resolve();
    },
    peek: () => structuredClone(value),
    poke(next) {
      value = next;
    },
    failSaves() {
      saveFails = true;
    },
    failLoads() {
      loadFails = true;
    },
    saveCount: () => saves,
  };
}
