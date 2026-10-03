// ADR-0313: オンラインで取得したマスタの保存先の型。実装は IndexedDB(browserStore.ts)、テストはメモリ fake。

import type { Ability, Item, Move } from "../../engine/types";
import type { MasterNature, MasterSpecies } from "../types";

/**
 * 保存するデータの形の版。形を変えたら上げる。版違いの保存済みデータは破棄して空として扱う
 * (次のオンライン取得で作り直す。ADR-0313 §5)。
 */
export const MASTER_CACHE_SCHEMA_VERSION = 1;

/** 保存するマスタ。種族・特性・技は resolveSpecies で解決したものだけを、キー(種族 key・特性 ID・技 ID)で持つ。 */
export interface MasterCacheRecord {
  readonly schemaVersion: number;
  readonly items: readonly Item[];
  readonly natures: readonly MasterNature[];
  readonly species: Readonly<Record<string, MasterSpecies>>;
  readonly abilities: Readonly<Record<string, Ability>>;
  readonly moves: Readonly<Record<string, Move>>;
}

/** マスタの保存先。load は未保存なら null を返す。 */
export interface MasterCacheStore {
  load(): Promise<unknown>;
  save(record: MasterCacheRecord): Promise<void>;
  clear(): Promise<void>;
}
