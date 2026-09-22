// テスト専用(P4-16b、ADR-0304 A-5・A-9・A-10): 機能の欠けたマスタ(オンライン相当)と、
// 種族の検索口(MasterSpeciesSearch)の fake。
// 画面は「オンラインかどうか」ではなく capabilities の各項目で分岐する(ADR-0304 A-2)ので、
// ここでも項目ごとに欠けさせられるようにし、「種族だけ検索・技はある」(P4-17 の形)も作れるようにする。

import type { Ability } from "../engine/types";
import type {
  MasterCapabilities,
  MasterData,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
  MasterSpeciesSummary,
} from "../master/types";

/**
 * capabilities に合わせて「公開 API では取れないもの」を落としたマスタを作る(ADR-0304 A-1)。
 *   speciesList: false → species は空(検索で都度引く)。特性の全件一覧も公開 API に無いので abilities も空
 *   moves: false       → moves は空(learnset の ID を技の実体に解決できない)
 *   effects: false     → 持ち物は残るが効果データが無い(effect は null)
 */
export function limitedMaster(base: MasterData, capabilities: MasterCapabilities): MasterData {
  return {
    ...base,
    species: capabilities.speciesList ? base.species : [],
    abilities: capabilities.speciesList ? base.abilities : [],
    moves: capabilities.moves ? base.moves : [],
    items: capabilities.effects ? base.items : base.items.map((item) => ({ ...item, effect: null })),
    capabilities,
  };
}

/** 検索1回の記録(デバウンス・取り消しの検査に使う)。 */
export interface SpeciesSearchCall {
  readonly query: string;
  readonly signal: AbortSignal | undefined;
}

export interface FakeSpeciesSearch extends MasterSpeciesSearch {
  /** searchSpecies の呼び出し(古い順)。 */
  readonly searchCalls: readonly SpeciesSearchCall[];
  /** resolveSpecies に渡された種族の key(古い順)。 */
  readonly resolvedKeys: readonly string[];
}

export interface FakeSpeciesSearchOptions {
  /** 検索の母集団(日本語名の前方一致で絞る)。 */
  readonly species: readonly MasterSpecies[];
  /** resolveSpecies が種族と一緒に返す特性(種族の abilities の ID で引く)。 */
  readonly abilities: readonly Ability[];
  /** 1回に返す件数の上限(既定は母集団の全件。上限に達した表示の検査で使う)。 */
  readonly limit?: number;
}

function toSummary(species: MasterSpecies): MasterSpeciesSummary {
  const { key, dexNo, form, nameJa, types } = species;
  return { key, dexNo, form, nameJa, types };
}

/** 前方一致で絞って候補を返し、選ばれた種族を特性ごと解決する fake(すぐ解決する)。 */
export function createFakeSpeciesSearch(options: FakeSpeciesSearchOptions): FakeSpeciesSearch {
  const searchCalls: SpeciesSearchCall[] = [];
  const resolvedKeys: string[] = [];
  const limit = options.limit ?? options.species.length;
  return {
    searchCalls,
    resolvedKeys,
    searchSpecies(query, signal) {
      searchCalls.push({ query, signal });
      const found = options.species
        .filter((species) => species.nameJa.startsWith(query.trim()))
        .slice(0, limit)
        .map(toSummary);
      return Promise.resolve(found);
    },
    resolveSpecies(key) {
      resolvedKeys.push(key);
      const species = options.species.find((candidate) => candidate.key === key);
      if (species === undefined) {
        return Promise.reject(new Error(`テストの母集団に ${key} が無い`));
      }
      const abilities = species.abilities.flatMap((id) => {
        const ability = options.abilities.find((candidate) => candidate.id === id);
        return ability === undefined ? [] : [ability];
      });
      const resolution: MasterSpeciesResolution = { species, abilities };
      return Promise.resolve(resolution);
    },
  };
}

/** 保留中の searchSpecies 1回(テストがあとから解決・棄却する)。 */
export interface PendingSpeciesSearch extends SpeciesSearchCall {
  resolve(found: readonly MasterSpeciesSummary[]): void;
  reject(error: Error): void;
}

export interface DeferredSpeciesSearch extends MasterSpeciesSearch {
  /** 保留中・解決済みを含む searchSpecies の呼び出し(古い順)。 */
  readonly searchCalls: readonly PendingSpeciesSearch[];
  /** 保留中・解決済みを含む resolveSpecies の呼び出し(古い順)。 */
  readonly resolveCalls: readonly PendingSpeciesResolve[];
}

/** 保留中の resolveSpecies 1回。 */
export interface PendingSpeciesResolve {
  readonly key: string;
  resolve(resolution: MasterSpeciesResolution): void;
  reject(error: Error): void;
}

/**
 * 応答をテストが握る fake(古い応答が新しい候補を上書きしないこと・失敗の表示を確かめる)。
 * 返した promise は必ずテストが解決・棄却する(unhandled rejection を出さないよう空の購読を付ける)。
 */
export function createDeferredSpeciesSearch(): DeferredSpeciesSearch {
  const searchCalls: PendingSpeciesSearch[] = [];
  const resolveCalls: PendingSpeciesResolve[] = [];
  return {
    searchCalls,
    resolveCalls,
    searchSpecies(query, signal) {
      let resolve: (found: readonly MasterSpeciesSummary[]) => void = () => undefined;
      let reject: (error: Error) => void = () => undefined;
      const promise = new Promise<readonly MasterSpeciesSummary[]>((onResolve, onReject) => {
        resolve = onResolve;
        reject = onReject;
      });
      promise.catch(() => undefined);
      searchCalls.push({ query, signal, resolve, reject });
      return promise;
    },
    resolveSpecies(key) {
      let resolve: (resolution: MasterSpeciesResolution) => void = () => undefined;
      let reject: (error: Error) => void = () => undefined;
      const promise = new Promise<MasterSpeciesResolution>((onResolve, onReject) => {
        resolve = onResolve;
        reject = onReject;
      });
      promise.catch(() => undefined);
      resolveCalls.push({ key, resolve, reject });
      return promise;
    },
  };
}
