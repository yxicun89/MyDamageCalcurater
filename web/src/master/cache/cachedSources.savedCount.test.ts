// ADR-0358 段 1: オフラインで使える「保存済みのポケモン」の数(MasterCacheStore の record.species のキー数)。
// 読めない・空・壊れた・版違いは 0(失敗させない)。

import { describe, expect, test } from "vitest";
import { createMemoryMasterCacheStore } from "../../test/memoryMasterCacheStore";
import { countSavedSpecies } from "./cachedSources";
import { MASTER_CACHE_SCHEMA_VERSION } from "./types";

function record(speciesKeys: readonly string[]): unknown {
  return {
    schemaVersion: MASTER_CACHE_SCHEMA_VERSION,
    items: [],
    natures: [],
    species: Object.fromEntries(speciesKeys.map((key) => [key, { key }])),
    abilities: {},
    moves: {},
  };
}

describe("保存済みのポケモンの数(countSavedSpecies)", () => {
  test("species のキーの数を返す", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke(record(["0001-000", "0002-000", "0003-001"]));
    expect(await countSavedSpecies(store)).toBe(3);
  });

  test("未保存は 0", async () => {
    expect(await countSavedSpecies(createMemoryMasterCacheStore())).toBe(0);
  });

  test("species が空は 0", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke(record([]));
    expect(await countSavedSpecies(store)).toBe(0);
  });

  test("読み出しに失敗しても 0(例外にしない)", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke(record(["0001-000"]));
    store.failLoads();
    expect(await countSavedSpecies(store)).toBe(0);
  });

  test("壊れた値・版違いは 0", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke({ schemaVersion: 1, species: { a: {} } });
    expect(await countSavedSpecies(store)).toBe(0);
    store.poke("broken");
    expect(await countSavedSpecies(store)).toBe(0);
  });
});
