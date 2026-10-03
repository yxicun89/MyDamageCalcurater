// ADR-0313: IndexedDB に1件のレコードとしてマスタを保存する MasterCacheStore。
// IndexedDB が無い・開けない・読み書きに失敗する環境(private mode 等)では例外を投げず、
// 「常に空」として振る舞う(キャッシュが無くてもオンライン動作は続ける。CLAUDE.md 絶対ルール5)。

import type { MasterCacheRecord, MasterCacheStore } from "./types";

/** IndexedDB のデータベース名(e2e/support/calcPage.ts の MASTER_CACHE_DB_NAME と同じ値)。 */
export const MASTER_CACHE_DB_NAME = "pokecalc-master-cache";

const OBJECT_STORE_NAME = "master";
const RECORD_KEY = "record";

function openDatabase(factory: IDBFactory): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = factory.open(MASTER_CACHE_DB_NAME, 1);
    request.onupgradeneeded = () => {
      request.result.createObjectStore(OBJECT_STORE_NAME);
    };
    request.onsuccess = () => {
      resolve(request.result);
    };
    request.onerror = () => {
      reject(request.error ?? new Error("IndexedDB を開けない"));
    };
    request.onblocked = () => {
      reject(new Error("IndexedDB が他のタブに塞がれている"));
    };
  });
}

/** 1つのトランザクションで操作を行い、完了まで待つ。 */
async function withStore<T>(
  factory: IDBFactory,
  mode: IDBTransactionMode,
  operation: (store: IDBObjectStore) => IDBRequest<T>,
): Promise<T> {
  const db = await openDatabase(factory);
  try {
    return await new Promise<T>((resolve, reject) => {
      const tx = db.transaction(OBJECT_STORE_NAME, mode);
      const request = operation(tx.objectStore(OBJECT_STORE_NAME));
      tx.oncomplete = () => {
        resolve(request.result);
      };
      tx.onerror = () => {
        reject(tx.error ?? new Error("IndexedDB の操作に失敗"));
      };
      tx.onabort = () => {
        reject(tx.error ?? new Error("IndexedDB の操作が中断された"));
      };
    });
  } finally {
    db.close();
  }
}

/**
 * ブラウザの IndexedDB を使う MasterCacheStore。factory を省くと globalThis.indexedDB を使い、
 * 無ければ(null)「常に空」の保存先になる。
 */
export function createBrowserMasterCacheStore(
  factory: IDBFactory | null = typeof indexedDB === "undefined" ? null : indexedDB,
): MasterCacheStore {
  if (factory === null) {
    return {
      load: () => Promise.resolve(null),
      save: () => Promise.resolve(),
      clear: () => Promise.resolve(),
    };
  }
  const idb = factory;
  /** 失敗は呼び出し側に伝えず fallback にする。同期の例外(open が投げる)も含める。 */
  async function safely<T>(run: () => Promise<T>, fallback: T): Promise<T> {
    try {
      return await run();
    } catch {
      return fallback;
    }
  }
  return {
    load: () =>
      safely(async () => {
        const value = await withStore<unknown>(idb, "readonly", (store) => store.get(RECORD_KEY));
        return value === undefined ? null : value;
      }, null),
    save: (record: MasterCacheRecord) =>
      safely(async () => {
        await withStore(idb, "readwrite", (store) => store.put(record, RECORD_KEY));
      }, undefined),
    clear: () =>
      safely(async () => {
        await withStore(idb, "readwrite", (store) => store.delete(RECORD_KEY));
      }, undefined),
  };
}
