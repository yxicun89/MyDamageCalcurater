// ADR-0358 段 3: オフライン用にポケモンのデータを端末へ事前保存する。
// 公開 API の種族一覧は limit 上限 200・ページング無し(実データ約 349)なので、日本語名の前方一致(q)を
// 頭文字ごとに引いて key で重複排除し、各ポケモンを resolveSpecies で解決する。解決の結果は
// createCachingMasterSource が MasterCacheStore に保存する(既存の経路)。サーバー負荷のため直列(同時 1 件)。
// 新しい API は使わない。

import type { MasterSpeciesSearch, MasterSpeciesSummary, SearchableMasterSource } from "../types";

/** 一覧を引くときの limit(公開 API の上限)。 */
export const PRELOAD_LIST_LIMIT = 200;

/** 頭文字ごとに引く文字(カタカナの清音・濁音・半濁音・ヴ)。濁音と清音を同じに扱うサーバーでも key の重複排除で足りる。 */
export const SPECIES_INITIALS: readonly string[] = (
  "アイウエオカキクケコガギグゲゴサシスセソザジズゼゾタチツテトダヂヅデドナニヌネノ" +
  "ハヒフヘホバビブベボパピプペポマミムメモヤユヨラリルレロワヲンヴ"
).split("");

/** 2 文字目で再分割するときの文字(頭文字 + 長音)。 */
const SPLIT_CHARS: readonly string[] = [...SPECIES_INITIALS, "ー"];

/** 再分割する最大の深さ(クエリの文字数)。 */
const MAX_QUERY_LENGTH = 3;

/** 保存済みかどうかを確かめる間隔(件)。端末に書けない失敗を早く見つけ、残りをサーバーから引き続けない。 */
export const PRELOAD_VERIFY_EVERY = 25;

function isAborted(signal: AbortSignal | undefined): boolean {
  return signal?.aborted === true;
}

function throwIfAborted(signal: AbortSignal | undefined): void {
  if (isAborted(signal)) {
    throw new DOMException("aborted", "AbortError");
  }
}

/**
 * 全ポケモンの一覧を集める。頭文字ごとに直列に引き、上限(200)に達した頭文字は次の 1 文字で再分割する。
 * 通信の失敗は reject する(握りつぶさない)。
 */
export async function listAllSpecies(
  search: Pick<MasterSpeciesSearch, "searchSpecies">,
  signal?: AbortSignal,
): Promise<readonly MasterSpeciesSummary[]> {
  const byKey = new Map<string, MasterSpeciesSummary>();
  async function collect(query: string): Promise<void> {
    throwIfAborted(signal);
    const hits = await search.searchSpecies(query, signal, PRELOAD_LIST_LIMIT);
    for (const hit of hits) {
      byKey.set(hit.key, hit);
    }
    if (hits.length >= PRELOAD_LIST_LIMIT && query.length < MAX_QUERY_LENGTH) {
      for (const next of SPLIT_CHARS) {
        await collect(`${query}${next}`);
      }
    }
  }
  for (const initial of SPECIES_INITIALS) {
    await collect(initial);
  }
  return [...byKey.values()];
}

export type PreloadResult =
  | { readonly status: "completed"; readonly savedCount: number }
  | { readonly status: "aborted"; readonly savedCount: number }
  | {
      readonly status: "failed";
      /** server = 通信・応答の失敗 / storage = 端末に書けない(容量不足など)。 */
      readonly reason: "server" | "storage";
      readonly savedCount: number;
    };

export interface PreloadSpeciesInput {
  /** 結果を保存する取得口(createCachingMasterSource で包んだオンラインの取得口)。 */
  readonly source: SearchableMasterSource;
  /** 保存済みのポケモンの key(保存済みは飛ばす。完了後の件数にも使う)。 */
  readonly loadSavedKeys: () => Promise<ReadonlySet<string>>;
  /** 保存済み分を含む進捗(done / total)。 */
  readonly onProgress: (done: number, total: number) => void;
  readonly signal?: AbortSignal;
  /** 保存の確認の間隔(テスト用に変えられる)。 */
  readonly verifyEvery?: number;
}

async function savedCountOf(loadSavedKeys: () => Promise<ReadonlySet<string>>): Promise<number> {
  try {
    return (await loadSavedKeys()).size;
  } catch {
    return 0;
  }
}

/** 全ポケモンを直列に解決して保存する。例外は投げず、結果(完了・中断・失敗)で返す。 */
export async function preloadSpecies(input: PreloadSpeciesInput): Promise<PreloadResult> {
  const { source, loadSavedKeys, onProgress, signal, verifyEvery = PRELOAD_VERIFY_EVERY } = input;

  async function stop(status: "aborted"): Promise<PreloadResult>;
  async function stop(status: "failed", reason: "server" | "storage"): Promise<PreloadResult>;
  async function stop(status: "aborted" | "failed", reason?: "server" | "storage"): Promise<PreloadResult> {
    const savedCount = await savedCountOf(loadSavedKeys);
    return status === "aborted" ? { status, savedCount } : { status, reason: reason ?? "server", savedCount };
  }

  let list: readonly MasterSpeciesSummary[];
  let saved: ReadonlySet<string>;
  try {
    // 持ち物・性格を取得して保存の土台を作る(これが無いと resolveSpecies の結果は保存されない)。
    await source.load();
    throwIfAborted(signal);
    list = await listAllSpecies(source.search, signal);
    saved = await loadSavedKeys();
  } catch {
    return isAborted(signal) ? stop("aborted") : stop("failed", "server");
  }

  const total = list.length;
  const todo = list.filter((entry) => !saved.has(entry.key));
  let done = total - todo.length;
  onProgress(done, total);

  let sinceVerify: string[] = [];
  /** 直近に解決した分が本当に端末へ保存されたか。されていなければ false。 */
  async function verified(): Promise<boolean> {
    const keys = await loadSavedKeys();
    const ok = sinceVerify.every((key) => keys.has(key));
    sinceVerify = [];
    return ok;
  }

  for (const entry of todo) {
    if (isAborted(signal)) {
      return stop("aborted");
    }
    try {
      await source.search.resolveSpecies(entry.key, signal);
    } catch {
      return isAborted(signal) ? stop("aborted") : stop("failed", "server");
    }
    sinceVerify.push(entry.key);
    done += 1;
    onProgress(done, total);
    if (sinceVerify.length >= verifyEvery && !(await verified().catch(() => false))) {
      return stop("failed", "storage");
    }
  }
  if (sinceVerify.length > 0 && !(await verified().catch(() => false))) {
    return stop("failed", "storage");
  }
  return { status: "completed", savedCount: await savedCountOf(loadSavedKeys) };
}
