// ADR-0313: オンラインで取得したマスタを MasterCacheStore に保存し、オフラインモードはそこから読む。
// 確かめること(issue #210 の受け入れ条件):
//   - オンラインの load()/resolveSpecies() のたびにキャッシュへ書く。書き込み失敗・読み出し失敗でもオンライン動作は成功する
//   - オフラインは読み出しのみ(オンラインを呼ばない)。キャッシュ済みの持ち物・性格・解決済みの種族と技で動く
//   - キャッシュが空なら架空データを出さず、案内(appText.masterCacheEmptyError)の Error で reject する
//   - 壊れたデータ・スキーマ版違いは破棄して null 扱い(= 案内)。次のオンライン取得で作り直せる
//   - オフラインの capabilities は speciesList/moves/effects とも false。種族の検索はキャッシュ内の種族だけが対象
//   - オンラインの失敗はオフラインへ黙って落とさない(reject のまま。ADR-0301 §4)

import typeChartData from "@typechart";
import { describe, expect, test, vi } from "vitest";
import { appText } from "../../i18n/ja";
import { createMemoryMasterCacheStore } from "../../test/memoryMasterCacheStore";
import { exampleAbilities } from "../example/abilities";
import { exampleItems } from "../example/items";
import { exampleMoves } from "../example/moves";
import { exampleNatures } from "../example/natures";
import { exampleSpecies } from "../example/species";
import { ONLINE_MASTER_CAPABILITIES } from "../onlineSource";
import { typeChartFromData } from "../typeChart";
import type { MasterSpecies, MasterSpeciesResolution, SearchableMasterSource } from "../types";
import {
  createCachedMasterSources,
  createCachedOfflineMasterSource,
  createCachingMasterSource,
} from "./cachedSources";
import { MASTER_CACHE_SCHEMA_VERSION } from "./types";

function exampleSpeciesAt(index: number): MasterSpecies {
  const species = exampleSpecies[index];
  if (species === undefined) {
    throw new Error("例データに種族が2つ以上要る");
  }
  return species;
}

const fire = exampleSpeciesAt(0);
const water = exampleSpeciesAt(1);

function resolutionOf(species: MasterSpecies): MasterSpeciesResolution {
  return {
    species,
    abilities: exampleAbilities.filter((ability) => species.abilities.includes(ability.id)),
    moves: species.learnset.flatMap((id) => exampleMoves.filter((move) => move.id === id)),
  };
}

/** オンライン(公開 API)相当の fake。種族は fire・water だけ引ける。 */
function fakeOnline(): {
  source: SearchableMasterSource;
  load: ReturnType<typeof vi.fn>;
  resolveSpecies: ReturnType<typeof vi.fn>;
  searchSpecies: ReturnType<typeof vi.fn>;
} {
  const load = vi.fn(() =>
    Promise.resolve({
      species: [],
      moves: [],
      items: exampleItems,
      abilities: [],
      natures: exampleNatures,
      typeChart: typeChartFromData(typeChartData),
      capabilities: ONLINE_MASTER_CAPABILITIES,
    }),
  );
  const searchSpecies = vi.fn((query: string) =>
    Promise.resolve([fire, water].filter((s) => query !== "" && s.nameJa.startsWith(query))),
  );
  const resolveSpecies = vi.fn((key: string) => {
    const found = [fire, water].find((s) => s.key === key);
    return found === undefined
      ? Promise.reject(new Error("not found"))
      : Promise.resolve(resolutionOf(found));
  });
  return {
    source: { load, search: { searchSpecies, resolveSpecies } },
    load,
    resolveSpecies,
    searchSpecies,
  };
}

describe("オンラインで取得したマスタをキャッシュへ保存する(createCachingMasterSource)", () => {
  test("load() の結果は変えずに返し、持ち物・性格をキャッシュへ書く", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const source = createCachingMasterSource({ source: online.source, store });

    const master = await source.load();

    expect(master.items).toEqual(exampleItems);
    expect(master.natures).toEqual(exampleNatures);
    expect(master.capabilities).toEqual(ONLINE_MASTER_CAPABILITIES);
    const saved = store.peek() as { schemaVersion: number; items: unknown; natures: unknown };
    expect(saved.schemaVersion).toBe(MASTER_CACHE_SCHEMA_VERSION);
    expect(saved.items).toEqual(exampleItems);
    expect(saved.natures).toEqual(exampleNatures);
  });

  test("resolveSpecies() の結果(種族・特性・技)を、既存のキャッシュに足して保存する", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const source = createCachingMasterSource({ source: online.source, store });
    await source.load();

    const resolved = await source.search.resolveSpecies(fire.key);
    await source.search.resolveSpecies(water.key);

    expect(resolved).toEqual(resolutionOf(fire));
    const offline = createCachedOfflineMasterSource({ store });
    expect((await offline.search.resolveSpecies(fire.key)).species).toEqual(fire);
    expect((await offline.search.resolveSpecies(water.key)).species).toEqual(water);
  });

  test("searchSpecies() は結果をそのまま返す(検索の候補だけではキャッシュしない)", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const source = createCachingMasterSource({ source: online.source, store });
    await source.load();

    expect(await source.search.searchSpecies("テスト")).toHaveLength(2);

    const offline = createCachedOfflineMasterSource({ store });
    expect(await offline.search.searchSpecies("テスト")).toEqual([]);
  });

  test("書き込みに失敗しても、オンラインの load/resolveSpecies は成功する(握りつぶす)", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    store.failSaves();
    const source = createCachingMasterSource({ source: online.source, store });

    await expect(source.load()).resolves.toMatchObject({ items: exampleItems });
    await expect(source.search.resolveSpecies(fire.key)).resolves.toEqual(resolutionOf(fire));
    expect(store.saveCount()).toBeGreaterThan(0);
  });

  test("キャッシュの読み出しに失敗しても(解決時の追記で読む場合も)オンラインは成功する", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const source = createCachingMasterSource({ source: online.source, store });
    await source.load();
    store.failLoads();

    await expect(source.search.resolveSpecies(fire.key)).resolves.toEqual(resolutionOf(fire));
  });

  test("オンラインの失敗はそのまま reject し、キャッシュで代替しない(自動フォールバック禁止)", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    await createCachingMasterSource({ source: online.source, store }).load();
    online.load.mockRejectedValueOnce(new Error("API に届かない"));

    const source = createCachingMasterSource({ source: online.source, store });
    await expect(source.load()).rejects.toThrow("API に届かない");
  });
});

describe("オフラインはキャッシュから読む(createCachedOfflineMasterSource)", () => {
  async function warmedStore() {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const source = createCachingMasterSource({ source: online.source, store });
    await source.load();
    await source.search.resolveSpecies(fire.key);
    return store;
  }

  test("キャッシュ済みの持ち物・性格・相性表を返し、種族・技の一覧は持たない(capabilities は全て false)", async () => {
    const offline = createCachedOfflineMasterSource({ store: await warmedStore() });

    const master = await offline.load();

    expect(master.items).toEqual(exampleItems);
    expect(master.natures).toEqual(exampleNatures);
    expect(Object.keys(master.typeChart).length).toBeGreaterThan(0);
    expect(master.species).toEqual([]);
    expect(master.moves).toEqual([]);
    expect(master.capabilities).toEqual({ speciesList: false, moves: false, effects: false });
  });

  test("検索はキャッシュ済みの種族だけが対象(日本語名の前方一致・空クエリは空配列)", async () => {
    const offline = createCachedOfflineMasterSource({ store: await warmedStore() });

    const hits = await offline.search.searchSpecies("テスト");
    expect(hits.map((hit) => hit.key)).toEqual([fire.key]);
    expect(hits[0]).toMatchObject({
      key: fire.key,
      dexNo: fire.dexNo,
      nameJa: fire.nameJa,
      types: fire.types,
    });
    expect(await offline.search.searchSpecies("   ")).toEqual([]);
    expect(await offline.search.searchSpecies("ない名前")).toEqual([]);
  });

  test("resolveSpecies はキャッシュ済みの種族・特性・技を返し、無い種族は reject する", async () => {
    const offline = createCachedOfflineMasterSource({ store: await warmedStore() });

    expect(await offline.search.resolveSpecies(fire.key)).toEqual(resolutionOf(fire));
    await expect(offline.search.resolveSpecies(water.key)).rejects.toThrow();
  });

  test("キャッシュが空(初回・未取得)なら架空データを出さず、案内の Error で reject する", async () => {
    const offline = createCachedOfflineMasterSource({ store: createMemoryMasterCacheStore() });

    await expect(offline.load()).rejects.toThrow(appText.masterCacheEmptyError);
    expect(appText.masterCacheEmptyError).toContain("オンライン");
  });

  test("キャッシュの読み出し自体が失敗(IndexedDB 不可など)でも、案内の Error で reject する", async () => {
    const store = createMemoryMasterCacheStore();
    store.failLoads();
    const offline = createCachedOfflineMasterSource({ store });

    await expect(offline.load()).rejects.toThrow(appText.masterCacheEmptyError);
  });

  test.each([
    ["壊れたデータ(オブジェクトでない)", "garbage"],
    ["壊れたデータ(必須の項目が無い)", { schemaVersion: MASTER_CACHE_SCHEMA_VERSION }],
    [
      "壊れたデータ(型が違う)",
      {
        schemaVersion: MASTER_CACHE_SCHEMA_VERSION,
        items: "x",
        natures: 1,
        species: [],
        abilities: 0,
        moves: null,
      },
    ],
  ])("%s は破棄して案内になる", async (_name, broken) => {
    const store = await warmedStore();
    store.poke(broken);
    const offline = createCachedOfflineMasterSource({ store });

    await expect(offline.load()).rejects.toThrow(appText.masterCacheEmptyError);
    expect(store.peek()).toBeNull();
  });

  test("スキーマ版が違うデータは破棄して案内になり、次のオンライン取得で作り直せる", async () => {
    const online = fakeOnline();
    const store = await warmedStore();
    const record = store.peek() as { schemaVersion: number };
    store.poke({ ...record, schemaVersion: MASTER_CACHE_SCHEMA_VERSION + 1 });

    await expect(createCachedOfflineMasterSource({ store }).load()).rejects.toThrow(
      appText.masterCacheEmptyError,
    );
    expect(store.peek()).toBeNull();

    await createCachingMasterSource({ source: online.source, store }).load();
    await expect(createCachedOfflineMasterSource({ store }).load()).resolves.toMatchObject({
      items: exampleItems,
    });
  });

  test("オフラインの読み出しはオンラインの取得口を呼ばない", async () => {
    const online = fakeOnline();
    const store = await warmedStore();
    const sources = createCachedMasterSources({ online: online.source, store });
    online.load.mockClear();

    await sources.offline.load();

    expect(online.load).not.toHaveBeenCalled();
    expect(online.resolveSpecies).not.toHaveBeenCalled();
  });
});

describe("モード別の取得口(createCachedMasterSources)", () => {
  test("online で取得 → offline で同じ内容が読める(API 遮断後を再現)", async () => {
    const online = fakeOnline();
    const store = createMemoryMasterCacheStore();
    const sources = createCachedMasterSources({ online: online.source, store });

    await sources.online.load();
    await (sources.online as SearchableMasterSource).search.resolveSpecies(fire.key);
    online.load.mockRejectedValue(new Error("API 遮断"));
    online.resolveSpecies.mockRejectedValue(new Error("API 遮断"));

    const master = await sources.offline.load();
    expect(master.items).toEqual(exampleItems);
    expect(
      (await (sources.offline as SearchableMasterSource).search.resolveSpecies(fire.key)).species,
    ).toEqual(fire);
  });

  test("キャッシュ空で online も失敗: online は reject、offline は案内(どちらも架空データを返さない)", async () => {
    const online = fakeOnline();
    online.load.mockRejectedValue(new Error("API 遮断"));
    const sources = createCachedMasterSources({
      online: online.source,
      store: createMemoryMasterCacheStore(),
    });

    await expect(sources.online.load()).rejects.toThrow("API 遮断");
    await expect(sources.offline.load()).rejects.toThrow(appText.masterCacheEmptyError);
  });
});
