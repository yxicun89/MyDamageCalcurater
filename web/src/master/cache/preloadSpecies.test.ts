// ADR-0358 段 3: オフライン用の事前保存。一覧を頭文字ごとに集める・直列に解決する・保存済みを飛ばす・中断・失敗の再開。

import { describe, expect, test } from "vitest";
import { createMemoryMasterCacheStore } from "../../test/memoryMasterCacheStore";
import type {
  MasterData,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSummary,
  SearchableMasterSource,
} from "../types";
import { createCachingMasterSource, listSavedSpeciesKeys } from "./cachedSources";
import {
  PRELOAD_LIST_LIMIT,
  SPECIES_INITIALS,
  listAllSpecies,
  preloadSpecies,
  type PreloadResult,
} from "./preloadSpecies";

function summary(key: string, nameJa: string): MasterSpeciesSummary {
  return { key, dexNo: Number(key.slice(0, 4)), form: 0, nameJa, types: ["normal"] };
}

function pad(n: number): string {
  return String(n).padStart(4, "0");
}

/** 前方一致で引ける偽の検索。呼ばれた q と limit を記録し、limit 件で打ち切る。 */
function fakeSearch(names: readonly MasterSpeciesSummary[]) {
  const queries: string[] = [];
  const limits: (number | undefined)[] = [];
  return {
    queries,
    limits,
    searchSpecies: (query: string, _signal?: AbortSignal, limit?: number) => {
      queries.push(query);
      limits.push(limit);
      const hits = names.filter((entry) => entry.nameJa.startsWith(query));
      return Promise.resolve(limit === undefined ? hits : hits.slice(0, limit));
    },
  };
}

describe("listAllSpecies(頭文字ごとに引いて合算)", () => {
  test("頭文字を網羅し、key で重複排除し、limit に 200 を渡す", async () => {
    const entries = [summary("0001-000", "アブ"), summary("0002-000", "ガブ"), summary("0003-000", "ン")];
    const search = fakeSearch(entries);
    const list = await listAllSpecies(search);
    expect(list.map((entry) => entry.key).sort()).toEqual(["0001-000", "0002-000", "0003-000"]);
    expect(new Set(search.queries)).toEqual(new Set(SPECIES_INITIALS));
    expect(search.limits.every((limit) => limit === PRELOAD_LIST_LIMIT)).toBe(true);
  });

  test("同じ key が複数の頭文字で返っても 1 件にする(濁音と清音が同じに扱われるサーバー)", async () => {
    const same = summary("0001-000", "ガブ");
    const search = {
      searchSpecies: (query: string) => Promise.resolve(query === "カ" || query === "ガ" ? [same] : []),
    };
    expect(await listAllSpecies(search)).toHaveLength(1);
  });

  test("1 文字で上限に達した頭文字は 2 文字目で分割して取りこぼさない", async () => {
    const many = Array.from({ length: PRELOAD_LIST_LIMIT + 30 }, (_unused, index) =>
      // 「ア」+ 2 文字目は イ か ウ に振り分ける
      summary(`${pad(index + 1)}-000`, `ア${index % 2 === 0 ? "イ" : "ウ"}${String(index)}`),
    );
    const search = fakeSearch(many);
    const list = await listAllSpecies(search);
    expect(list).toHaveLength(many.length);
    expect(search.queries).toContain("アイ");
    expect(search.queries).toContain("アウ");
  });

  test("通信の失敗は握りつぶさず reject する", async () => {
    const search = { searchSpecies: () => Promise.reject(new Error("down")) };
    await expect(listAllSpecies(search)).rejects.toThrow("down");
  });

  test("中断されたら次の頭文字を引かない", async () => {
    const controller = new AbortController();
    let calls = 0;
    const search = {
      searchSpecies: () => {
        calls += 1;
        controller.abort();
        return Promise.resolve([]);
      },
    };
    await expect(listAllSpecies(search, controller.signal)).rejects.toThrow();
    expect(calls).toBe(1);
  });
});

// ---- preloadSpecies ----

function speciesOf(key: string): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: 0,
    nameJa: `n${key}`,
    types: ["normal"],
    baseStats: { hp: 1, atk: 1, def: 1, spa: 1, spd: 1, spe: 1 },
    abilities: [],
    learnset: [],
  };
}

interface Harness {
  readonly source: SearchableMasterSource;
  readonly store: ReturnType<typeof createMemoryMasterCacheStore>;
  readonly resolved: string[];
  readonly inFlightMax: () => number;
}

function harness(
  keys: readonly string[],
  options: { failOn?: string; onResolve?: (key: string) => void } = {},
): Harness {
  const store = createMemoryMasterCacheStore();
  const resolved: string[] = [];
  let inFlight = 0;
  let max = 0;
  const raw: SearchableMasterSource = {
    load: () =>
      Promise.resolve({
        species: [],
        moves: [],
        items: [],
        abilities: [],
        natures: [],
      } as unknown as MasterData),
    search: {
      searchSpecies: (query) =>
        Promise.resolve(query === "ア" ? keys.map((key) => summary(key, `ア${key}`)) : []),
      async resolveSpecies(key): Promise<MasterSpeciesResolution> {
        inFlight += 1;
        max = Math.max(max, inFlight);
        await Promise.resolve();
        options.onResolve?.(key);
        inFlight -= 1;
        if (key === options.failOn) {
          throw new Error("down");
        }
        resolved.push(key);
        return { species: speciesOf(key), abilities: [], moves: [] };
      },
    },
  };
  return {
    source: createCachingMasterSource({ source: raw, store }),
    store,
    resolved,
    inFlightMax: () => max,
  };
}

const KEYS = ["0001-000", "0002-000", "0003-000", "0004-000"];

function deps(h: Harness) {
  return { source: h.source, loadSavedKeys: () => listSavedSpeciesKeys(h.store) };
}

describe("preloadSpecies", () => {
  test("全件を直列(同時 1 件)で解決して保存し、進捗を知らせる", async () => {
    const h = harness(KEYS);
    const progress: [number, number][] = [];
    const result = await preloadSpecies({
      ...deps(h),
      onProgress: (done, total) => progress.push([done, total]),
    });
    expect(result).toMatchObject({ status: "completed", savedCount: 4 });
    expect(h.resolved).toEqual(KEYS);
    expect(h.inFlightMax()).toBe(1);
    expect(progress[0]).toEqual([0, 4]);
    expect(progress[progress.length - 1]).toEqual([4, 4]);
    expect([...(await listSavedSpeciesKeys(h.store))].sort()).toEqual(KEYS);
  });

  test("保存済みの種族は飛ばす(進捗は保存済み分から始まる)", async () => {
    const h = harness(KEYS);
    await preloadSpecies({ ...deps(h), onProgress: () => undefined });
    h.resolved.length = 0;
    const progress: [number, number][] = [];
    const second = await preloadSpecies({
      ...deps(h),
      onProgress: (done, total) => progress.push([done, total]),
    });
    expect(h.resolved).toEqual([]);
    expect(second).toMatchObject({ status: "completed", savedCount: 4 });
    expect(progress[0]).toEqual([4, 4]);
  });

  test("通信が失敗したら止まって failed(server)。保存済みの件数を返し、再実行で続きから", async () => {
    const h = harness(KEYS, { failOn: "0003-000" });
    const first: PreloadResult = await preloadSpecies({ ...deps(h), onProgress: () => undefined });
    expect(first).toMatchObject({ status: "failed", reason: "server", savedCount: 2 });
    const h2 = { ...h };
    // 同じ store で、今度は失敗しない取得口
    const retry = createCachingMasterSource({
      store: h.store,
      source: {
        load: () =>
          Promise.resolve({
            species: [],
            moves: [],
            items: [],
            abilities: [],
            natures: [],
          } as unknown as MasterData),
        search: {
          searchSpecies: (query) =>
            Promise.resolve(query === "ア" ? KEYS.map((key) => summary(key, `ア${key}`)) : []),
          resolveSpecies: (key) => {
            h2.resolved.push(`retry:${key}`);
            return Promise.resolve({ species: speciesOf(key), abilities: [], moves: [] });
          },
        },
      },
    });
    const second = await preloadSpecies({
      source: retry,
      loadSavedKeys: () => listSavedSpeciesKeys(h.store),
      onProgress: () => undefined,
    });
    expect(second).toMatchObject({ status: "completed", savedCount: 4 });
    expect(h.resolved.filter((entry) => entry.startsWith("retry:"))).toEqual([
      "retry:0003-000",
      "retry:0004-000",
    ]);
  });

  test("中断したら次の種族を引かず aborted", async () => {
    const controller = new AbortController();
    const h = harness(KEYS, {
      onResolve: (key) => {
        if (key === "0002-000") {
          controller.abort();
        }
      },
    });
    const result = await preloadSpecies({
      ...deps(h),
      signal: controller.signal,
      onProgress: () => undefined,
    });
    expect(result).toMatchObject({ status: "aborted" });
    expect(h.resolved).not.toContain("0003-000");
    expect(h.resolved).not.toContain("0004-000");
  });

  test("端末に書けない(容量超過)ときは failed(storage)で止まる。残りを引き続けない", async () => {
    const h = harness(KEYS);
    await h.source.load(); // 土台を作ってから書き込みを失敗させる
    h.store.failSaves();
    const result = await preloadSpecies({ ...deps(h), verifyEvery: 1, onProgress: () => undefined });
    expect(result).toMatchObject({ status: "failed", reason: "storage" });
    expect(h.resolved.length).toBeLessThan(KEYS.length);
  });

  test("一覧の取得に失敗したら failed(server)", async () => {
    const h = harness(KEYS);
    const broken: SearchableMasterSource = {
      load: () => h.source.load(),
      search: {
        searchSpecies: () => Promise.reject(new Error("down")),
        resolveSpecies: (key, signal) => h.source.search.resolveSpecies(key, signal),
      },
    };
    const result = await preloadSpecies({
      source: broken,
      loadSavedKeys: () => listSavedSpeciesKeys(h.store),
      onProgress: () => undefined,
    });
    expect(result).toMatchObject({ status: "failed", reason: "server", savedCount: 0 });
  });
});
