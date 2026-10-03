// ADR-0326(ADR-0175 §4・§5、ADR-0313 §5): 持ち物の roles・isMegaStone と種族の baseSpeciesNameJa が、
// IndexedDB のキャッシュを通ってもオフラインで残る。キャッシュの版を上げ、roles の無い古い記録は使わない。
// 確かめること:
//   - スキーマ版が 3 以上(版 2 = roles・isMegaStone・基本種名を持たない記録)
//   - 版 2 の保存済み記録は破棄され、案内の Error(空として扱う)になる
//     (残すと、オフラインで roles の無い持ち物が「絞らない」扱いになり、意味の無い持ち物とメガストーンが選択肢に戻る)
//   - オンラインで取得した持ち物の roles・isMegaStone を、オフライン(キャッシュ)の load がそのまま返す
//   - オンラインで解決したメガ種族の baseSpeciesKey・baseSpeciesNameJa を、オフラインの resolveSpecies が返す

import { describe, expect, test, vi } from "vitest";
import { appText } from "../../i18n/ja";
import { createMemoryMasterCacheStore } from "../../test/memoryMasterCacheStore";
import { ROLE_ITEMS, withoutRoleFields } from "../../test/itemRolesMaster";
import { MEGA_FIRE } from "../../test/megaMaster";
import { exampleNatures } from "../example/natures";
import { ONLINE_MASTER_CAPABILITIES } from "../onlineSource";
import type { MasterSpeciesResolution, SearchableMasterSource } from "../types";
import { createCachedOfflineMasterSource, createCachingMasterSource } from "./cachedSources";
import { MASTER_CACHE_SCHEMA_VERSION } from "./types";

function fakeOnline(): SearchableMasterSource {
  const resolution: MasterSpeciesResolution = { species: MEGA_FIRE, abilities: [], moves: [] };
  return {
    load: vi.fn(() =>
      Promise.resolve({
        species: [],
        moves: [],
        items: ROLE_ITEMS,
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

describe("持ち物の役割と基本種名のキャッシュ", () => {
  test("スキーマ版が 3 以上(roles・isMegaStone・基本種名を持たない版 2 の記録は使わない)", () => {
    expect(MASTER_CACHE_SCHEMA_VERSION).toBeGreaterThanOrEqual(3);
  });

  test("版 2 の保存済み記録は破棄され、案内の Error(空として扱う)になる", async () => {
    const store = createMemoryMasterCacheStore();
    store.poke({
      schemaVersion: 2,
      items: ROLE_ITEMS.map(withoutRoleFields),
      natures: exampleNatures,
      species: { [MEGA_FIRE.key]: { ...MEGA_FIRE, baseSpeciesKey: undefined, baseSpeciesNameJa: undefined } },
      abilities: {},
      moves: {},
    });

    await expect(createCachedOfflineMasterSource({ store }).load()).rejects.toThrow(
      appText.masterCacheEmptyError,
    );
  });

  test("オンラインで取得した持ち物の roles・isMegaStone を、オフラインがそのまま返す", async () => {
    const store = createMemoryMasterCacheStore();
    await createCachingMasterSource({ source: fakeOnline(), store }).load();

    const master = await createCachedOfflineMasterSource({ store }).load();
    expect(master.items).toEqual(ROLE_ITEMS);
  });

  test("オンラインで解決したメガ種族の baseSpeciesKey・baseSpeciesNameJa を、オフラインが返す", async () => {
    const store = createMemoryMasterCacheStore();
    const online = createCachingMasterSource({ source: fakeOnline(), store });
    await online.load();
    await online.search.resolveSpecies(MEGA_FIRE.key);

    const resolved = await createCachedOfflineMasterSource({ store }).search.resolveSpecies(MEGA_FIRE.key);
    expect(resolved.species.baseSpeciesKey).toBe(MEGA_FIRE.baseSpeciesKey);
    expect(resolved.species.baseSpeciesNameJa).toBe(MEGA_FIRE.baseSpeciesNameJa);
  });
});
