// issue #515・ADR-0320: メガ種族の isMega・requiredItemId がキャッシュ(ADR-0313)を通っても残る。
// 確かめること:
//   - オンラインで解決したメガ種族を保存し、オフライン(キャッシュ)の resolveSpecies が isMega・requiredItemId を返す
//   - 持ち物(メガストーンを含む)も同じキャッシュから読める(オフラインでも固定の名前を引ける)
//   - スキーマ版を上げた: この変更より前に保存された記録(版 1。メガの項目が無い)は破棄して空として扱う
//     (残すと、メガ種族が isMega を持たないまま再利用され、固定されない)

import { describe, expect, test, vi } from "vitest";
import { appText } from "../../i18n/ja";
import { createMemoryMasterCacheStore } from "../../test/memoryMasterCacheStore";
import { MEGA_FIRE, MEGA_FIRE_STONE } from "../../test/megaMaster";
import { exampleItems } from "../example/items";
import { exampleSpecies } from "../example/species";
import { exampleNatures } from "../example/natures";
import { ONLINE_MASTER_CAPABILITIES } from "../onlineSource";
import type { MasterSpeciesResolution, SearchableMasterSource } from "../types";
import { createCachedOfflineMasterSource, createCachingMasterSource } from "./cachedSources";
import { MASTER_CACHE_SCHEMA_VERSION } from "./types";

const items = [...exampleItems, MEGA_FIRE_STONE];

function fakeOnline(): SearchableMasterSource {
  const resolution: MasterSpeciesResolution = { species: MEGA_FIRE, abilities: [], moves: [] };
  return {
    load: vi.fn(() =>
      Promise.resolve({
        species: [],
        moves: [],
        items,
        abilities: [],
        natures: exampleNatures,
        typeChart: { effectiveness: {} },
        capabilities: ONLINE_MASTER_CAPABILITIES,
      }),
    ),
    search: {
      searchSpecies: vi.fn(() => Promise.resolve([])),
      resolveSpecies: vi.fn(() => Promise.resolve(resolution)),
    },
  } as unknown as SearchableMasterSource;
}

describe("メガ種族のキャッシュ", () => {
  test("オンラインで解決したメガ種族を、オフラインが isMega・requiredItemId つきで返す", async () => {
    const store = createMemoryMasterCacheStore();
    const online = createCachingMasterSource({ source: fakeOnline(), store });
    await online.load();
    await online.search.resolveSpecies(MEGA_FIRE.key);

    const offline = createCachedOfflineMasterSource({ store });
    const resolved = await offline.search.resolveSpecies(MEGA_FIRE.key);
    expect(resolved.species.isMega).toBe(true);
    expect(resolved.species.requiredItemId).toBe(MEGA_FIRE_STONE.id);
    const master = await offline.load();
    expect(master.items.find((item) => item.id === MEGA_FIRE_STONE.id)?.nameJa).toBe(MEGA_FIRE_STONE.nameJa);
  });

  test("スキーマ版が上がっている(メガの項目を持たない版 1 の記録は使わない)", () => {
    expect(MASTER_CACHE_SCHEMA_VERSION).toBeGreaterThanOrEqual(2);
  });

  test("版 1 の保存済み記録は破棄され、案内の Error(空として扱う)になる", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke({
      schemaVersion: 1,
      items,
      natures: exampleNatures,
      species: { [MEGA_FIRE.key]: { ...MEGA_FIRE, isMega: undefined, requiredItemId: undefined } },
      abilities: {},
      moves: {},
    });

    await expect(createCachedOfflineMasterSource({ store }).load()).rejects.toThrow(
      appText.masterCacheEmptyError,
    );
  });
});

// issue #515・ADR-0324: オフライン(キャッシュ)の検索も、オンライン(pokedex-svc)と同じ規則・並びにする。
describe("メガ種族のオフライン検索", () => {
  async function offlineWithMegaAndBase(): Promise<ReturnType<typeof createCachedOfflineMasterSource>> {
    const store = createMemoryMasterCacheStore();
    const base = exampleSpecies.find((species) => species.key === "9001-000");
    if (base === undefined) {
      throw new Error("例データに 9001-000 が無い");
    }
    const resolutions: Record<string, MasterSpeciesResolution> = {
      [MEGA_FIRE.key]: { species: MEGA_FIRE, abilities: [], moves: [] },
      [base.key]: { species: base, abilities: [], moves: [] },
    };
    const source = {
      ...fakeOnline(),
      search: {
        searchSpecies: vi.fn(() => Promise.resolve([])),
        resolveSpecies: vi.fn((key: string) => Promise.resolve(resolutions[key] as MasterSpeciesResolution)),
      },
    } as unknown as SearchableMasterSource;
    const online = createCachingMasterSource({ source, store });
    await online.load();
    await online.search.resolveSpecies(MEGA_FIRE.key);
    await online.search.resolveSpecies(base.key);
    return createCachedOfflineMasterSource({ store });
  }

  test("メガの名前・「メガ」・基本種名でメガ種族が返る(基本種名では基本種が先)", async () => {
    const offline = await offlineWithMegaAndBase();
    const keys = async (query: string): Promise<string[]> =>
      (await offline.search.searchSpecies(query)).map((species) => species.key);

    expect(await keys(MEGA_FIRE.nameJa)).toEqual([MEGA_FIRE.key]);
    expect(await keys("メガ")).toEqual([MEGA_FIRE.key]);
    expect(await keys("テストほのお")).toEqual(["9001-000", MEGA_FIRE.key]);
  });
});
