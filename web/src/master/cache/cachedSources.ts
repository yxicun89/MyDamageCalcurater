// ADR-0313: オンラインで取得したマスタを MasterCacheStore に保存し、オフラインモードはそこから読む取得口。
//   - createCachingMasterSource: オンラインの取得口を包み、load()/resolveSpecies() の結果を保存する(失敗は握りつぶす)
//   - createCachedOfflineMasterSource: 保存済みのマスタだけを読む(オンラインを呼ばない。空・壊れ・版違いは案内の Error)
//   - createCachedMasterSources: 上の2つを App の MasterSources にまとめる

import typeChartData from "@typechart";
import { appText } from "../../i18n/ja";
import type { Ability, Move } from "../../engine/types";
import { typeChartFromData } from "../typeChart";
import type {
  MasterData,
  MasterSources,
  MasterSpeciesResolution,
  MasterSpeciesSummary,
  SearchableMasterSource,
} from "../types";
import { MASTER_CACHE_SCHEMA_VERSION, type MasterCacheRecord, type MasterCacheStore } from "./types";

/** オフラインが使える機能(キャッシュは種族・技の一覧も効果データも持たない。ADR-0313 §3)。 */
const OFFLINE_CACHE_CAPABILITIES = { speciesList: false, moves: false, effects: false } as const;

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** 保存済みの値が MasterCacheRecord の形か(要素の中身までは見ない。版が合っていれば自分が書いた形)。 */
function isMasterCacheRecord(value: unknown): value is MasterCacheRecord {
  return (
    isPlainObject(value) &&
    value.schemaVersion === MASTER_CACHE_SCHEMA_VERSION &&
    Array.isArray(value.items) &&
    Array.isArray(value.natures) &&
    isPlainObject(value.species) &&
    isPlainObject(value.abilities) &&
    isPlainObject(value.moves)
  );
}

/** 読み出しに失敗したら null(空)として扱う。 */
async function loadRaw(store: MasterCacheStore): Promise<unknown> {
  try {
    return await store.load();
  } catch {
    return null;
  }
}

/** 保存済みのマスタを読む。無い・読めない・形か版が合わないときは null(合わないものは破棄する)。 */
async function loadRecord(store: MasterCacheStore): Promise<MasterCacheRecord | null> {
  const raw = await loadRaw(store);
  if (raw === null) {
    return null;
  }
  if (isMasterCacheRecord(raw)) {
    return raw;
  }
  try {
    await store.clear();
  } catch {
    // 破棄に失敗しても、空として扱って続ける。
  }
  return null;
}

/** 書き込みの失敗は握りつぶす(オンライン動作を妨げない)。 */
async function saveQuietly(store: MasterCacheStore, record: MasterCacheRecord): Promise<void> {
  try {
    await store.save(record);
  } catch {
    // 容量超過・private mode など。キャッシュが無くてもオンラインは動く。
  }
}

function byId<T extends { readonly id: string }>(
  entries: Readonly<Record<string, T>>,
  ids: readonly string[],
): T[] {
  return ids.flatMap((id) => {
    const found = entries[id];
    return found === undefined ? [] : [found];
  });
}

export interface CreateCachingMasterSourceInput {
  readonly source: SearchableMasterSource;
  readonly store: MasterCacheStore;
}

/** オンラインの取得口を包み、取得した持ち物・性格と解決した種族・特性・技を保存する。結果は変えない。 */
export function createCachingMasterSource(input: CreateCachingMasterSourceInput): SearchableMasterSource {
  const { source, store } = input;
  /** 読み書きを1本につなぎ(並行する解決の追記が互いを上書きしないように)、保存が済んでから返す。タスクは例外を投げない。 */
  let queue: Promise<void> = Promise.resolve();
  function enqueue(task: () => Promise<void>): Promise<void> {
    queue = queue.then(task, task);
    return queue;
  }

  return {
    async load(): Promise<MasterData> {
      const master = await source.load();
      await enqueue(async () => {
        const existing = await loadRecord(store);
        await saveQuietly(store, {
          schemaVersion: MASTER_CACHE_SCHEMA_VERSION,
          items: master.items,
          natures: master.natures,
          species: existing?.species ?? {},
          abilities: existing?.abilities ?? {},
          moves: existing?.moves ?? {},
        });
      });
      return master;
    },
    search: {
      searchSpecies: (query, signal) => source.search.searchSpecies(query, signal),
      async resolveSpecies(key, signal): Promise<MasterSpeciesResolution> {
        const resolution = await source.search.resolveSpecies(key, signal);
        await enqueue(async () => {
          // 持ち物・性格を取得していない(load が先に走っていない)間は、保存する土台が無いので何もしない。
          const existing = await loadRecord(store);
          if (existing === null) {
            return;
          }
          await saveQuietly(store, {
            ...existing,
            species: { ...existing.species, [resolution.species.key]: resolution.species },
            abilities: {
              ...existing.abilities,
              ...Object.fromEntries(resolution.abilities.map((ability) => [ability.id, ability])),
            },
            moves: {
              ...existing.moves,
              ...Object.fromEntries(resolution.moves.map((move) => [move.id, move])),
            },
          });
        });
        return resolution;
      },
    },
  };
}

export interface CreateCachedOfflineMasterSourceInput {
  readonly store: MasterCacheStore;
}

/** 保存済みのマスタだけを読むオフラインの取得口。保存が無ければ案内の Error で reject する(架空データは出さない)。 */
export function createCachedOfflineMasterSource(
  input: CreateCachedOfflineMasterSourceInput,
): SearchableMasterSource {
  const { store } = input;

  async function requireRecord(): Promise<MasterCacheRecord> {
    const record = await loadRecord(store);
    if (record === null) {
      throw new Error(appText.masterCacheEmptyError);
    }
    return record;
  }

  return {
    async load(): Promise<MasterData> {
      const record = await requireRecord();
      return {
        species: [],
        moves: [],
        items: record.items,
        abilities: [],
        natures: record.natures,
        typeChart: typeChartFromData(typeChartData),
        capabilities: OFFLINE_CACHE_CAPABILITIES,
      };
    },
    search: {
      async searchSpecies(query): Promise<readonly MasterSpeciesSummary[]> {
        const trimmed = query.trim();
        if (trimmed === "") {
          return [];
        }
        const record = await requireRecord();
        return Object.values(record.species)
          .filter((species) => species.nameJa.startsWith(trimmed))
          .map((species) => ({
            key: species.key,
            dexNo: species.dexNo,
            form: species.form,
            nameJa: species.nameJa,
            types: species.types,
          }));
      },
      async resolveSpecies(key): Promise<MasterSpeciesResolution> {
        const record = await requireRecord();
        const species = record.species[key];
        if (species === undefined) {
          throw new Error(`キャッシュに無い種族: ${key}`);
        }
        const abilities: Ability[] = byId(record.abilities, species.abilities);
        const moves: Move[] = byId(record.moves, species.learnset);
        return { species, abilities, moves };
      },
    },
  };
}

export interface CreateCachedMasterSourcesInput {
  readonly online: SearchableMasterSource;
  readonly store: MasterCacheStore;
}

/** App の MasterSources: オンラインは取得結果を保存し、オフラインは保存済みを読む。 */
export function createCachedMasterSources(input: CreateCachedMasterSourcesInput): MasterSources {
  return {
    online: createCachingMasterSource({ source: input.online, store: input.store }),
    offline: createCachedOfflineMasterSource({ store: input.store }),
  };
}
