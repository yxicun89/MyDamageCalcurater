import type { Genre, Item, Site } from "../api/types";

export interface CacheShape {
  items: Item[];
  genres: Genre[];
  sites: Site[];
}
export type CacheKind = keyof CacheShape;

/** localStorage の "wishlist.cache.<kind>" に保存する。 */
export const cacheKey: (kind: CacheKind) => string = (kind) => `wishlist.cache.${kind}`;

/** 保存できたら true。例外は投げない。 */
export const saveCache = <K extends CacheKind>(kind: K, data: CacheShape[K]): boolean => {
  try {
    localStorage.setItem(cacheKey(kind), JSON.stringify(data));
    return true;
  } catch {
    return false;
  }
};

/** 無い・壊れている・使えないときは null。 */
export const loadCache = <K extends CacheKind>(kind: K): CacheShape[K] | null => {
  try {
    const raw = localStorage.getItem(cacheKey(kind));
    if (raw === null) return null;
    const v: unknown = JSON.parse(raw);
    return Array.isArray(v) ? (v as CacheShape[K]) : null;
  } catch {
    return null;
  }
};

export interface Cached<T> {
  data: T;
  /** true なら取得に失敗して保存済みの値を返した */
  stale: boolean;
}

/**
 * fetcher が成功したら保存して { stale: false }。失敗(ApiError を含むどんな例外も)したら保存済みを { stale: true } で返す。
 * 保存済みも無いときは元の例外をそのまま投げる。
 */
export const fetchWithCache = async <K extends CacheKind>(
  kind: K,
  fetcher: () => Promise<CacheShape[K]>,
): Promise<Cached<CacheShape[K]>> => {
  try {
    const data = await fetcher();
    saveCache(kind, data);
    return { data, stale: false };
  } catch (err) {
    const saved = loadCache(kind);
    if (saved === null) throw err;
    return { data: saved, stale: true };
  }
};
