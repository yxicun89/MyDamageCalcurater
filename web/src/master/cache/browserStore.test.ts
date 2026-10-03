// ADR-0313: IndexedDB 実装(ブラウザ用の薄い MasterCacheStore)の「使えない環境」の契約。
// IndexedDB が無い(Node/jsdom)・開けない(private mode 等)でも例外を投げず、空として振る舞う
// (キャッシュが無くてもオンライン動作は続けられる)。実際の読み書きは e2e/offline.spec.ts で確かめる。

import { describe, expect, test } from "vitest";
import { MASTER_CACHE_DB_NAME, createBrowserMasterCacheStore } from "./browserStore";
import { MASTER_CACHE_SCHEMA_VERSION, type MasterCacheRecord } from "./types";

const record: MasterCacheRecord = {
  schemaVersion: MASTER_CACHE_SCHEMA_VERSION,
  items: [],
  natures: [],
  species: {},
  abilities: {},
  moves: {},
};

test("データベース名は e2e/support/calcPage.ts の MASTER_CACHE_DB_NAME と同じ", () => {
  expect(MASTER_CACHE_DB_NAME).toBe("pokecalc-master-cache");
});

describe("IndexedDB が使えない環境", () => {
  test("factory が null: load は null、save・clear は何もせず resolve する", async () => {
    const store = createBrowserMasterCacheStore(null);
    await expect(store.load()).resolves.toBeNull();
    await expect(store.save(record)).resolves.toBeUndefined();
    await expect(store.clear()).resolves.toBeUndefined();
  });

  test("open() が例外を投げても(private mode 等)load は null、save・clear は resolve する", async () => {
    const throwing = {
      open() {
        throw new DOMException("denied", "SecurityError");
      },
    } as unknown as IDBFactory;
    const store = createBrowserMasterCacheStore(throwing);
    await expect(store.load()).resolves.toBeNull();
    await expect(store.save(record)).resolves.toBeUndefined();
    await expect(store.clear()).resolves.toBeUndefined();
  });

  test("引数を省くと globalThis.indexedDB を使う(jsdom には無いので、同じく空として振る舞う)", async () => {
    await expect(createBrowserMasterCacheStore().load()).resolves.toBeNull();
  });
});
