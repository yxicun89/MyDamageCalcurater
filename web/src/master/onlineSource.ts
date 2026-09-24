// P4-16/P4-17: pokedex-svc の公開 API からマスタを読む MasterSource(ADR-0304)。
// 公開 API(api/openapi.yaml)の制約:
//   - searchSpecies / searchMoves / searchItems は limit の上限が 200 でページングが無い。
//     実データは種族349件・技515件なので一括取得できず、持ち物166件・性格25件だけが1回で全件取れる(ADR-0304 §1)。
//   - getSpecies.learnset は技の ID 配列だけなので、resolveSpecies が getMovesByIds
//     (GET /api/pokedex/moves/batch?ids=...)で技の実体に解決する。ids は契約上1〜64件なので、
//     MOVES_BATCH_MAX_IDS 件ずつに分割して並列に呼ぶ(ADR-0304 §3 追記・A-13)。
// なので load() は「持ち物・性格・相性表」だけを返し、種族・技は MasterSpeciesSearch.resolveSpecies で
// 種族ごとに都度引く(ADR-0304 §4 の段階的導入・A-13)。
// 応答の型は openapi-typescript の生成型(api/openapi.gen.ts)を正とし、手で複製しない(ADR-0301 §6)。

import typeChartData from "@typechart";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import type { Ability, Item, Move } from "../engine/types";
import { typeChartFromData } from "./typeChart";
import type {
  MasterCapabilities,
  MasterData,
  MasterNature,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
  MasterSpeciesSummary,
  SearchableMasterSource,
} from "./types";

type Schemas = components["schemas"];

/**
 * 持ち物を1回で全件取るときの limit。公開 API の maximum(200)そのもので、実データ166件を上回る
 * (ADR-0304 §1 の実測)。これを超えて持ち物が増えたら1回では取り切れないので、load() は黙って
 * 打ち切らずに失敗する。
 */
export const ITEMS_FETCH_LIMIT = 200;

/**
 * 種族の検索で1回に受け取る件数の上限。349件を一覧にはできないので、入力補完の候補として出せる数に絞る
 * (ADR-0304 §1 の「既定の limit(50)」)。
 */
export const SPECIES_SEARCH_LIMIT = 50;

/** 種族の検索を始める最小の文字数。1文字目から候補を出す(日本語名の前方一致なので1文字で十分絞れる)。 */
export const SPECIES_SEARCH_MIN_LENGTH = 1;

/**
 * P4-17: `GET /api/pokedex/moves/batch`(`getMovesByIds`)に1回で渡せる ID の上限。
 * 契約(api/openapi.yaml の `ids` の `maxItems`)そのもので、65件以上は 400 `invalid_input` になる。
 * **1種族の learnset がこの件数に収まる保証は無い**(API レーンも実データ未確認。ADR-0304 §3 追記)ので、
 * 呼び出し側がこの件数ずつに分割して複数回呼ぶ(ADR-0304 A-13)。
 */
export const MOVES_BATCH_MAX_IDS = 64;

/**
 * 種族の検索入力のデバウンス(ミリ秒)。入力1文字ごとに検索を投げない(ADR-0304 §1)。
 * 検索 UI(P4-16b)がこの値を使う。
 */
export const SPECIES_SEARCH_DEBOUNCE_MS = 250;

/** オンラインのマスタが使える機能(ADR-0304 §4)。種族は検索、技は未対応、効果データは公開 API に無い。 */
export const ONLINE_MASTER_CAPABILITIES: MasterCapabilities = {
  speciesList: false,
  moves: false,
  effects: false,
};

/** createOnlineMasterSource の引数(createApiEngine と同じ形。ADR-0301 §3・§4)。 */
export interface CreateOnlineMasterSourceInput {
  /** API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() で作る)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 全リクエストに付ける端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/** 公開 API のパス(api/openapi.yaml の paths。基点 URL からの相対)。 */
const PATHS = {
  species: "api/pokedex/species",
  items: "api/pokedex/items",
  natures: "api/pokedex/natures",
  movesBatch: "api/pokedex/moves/batch",
} as const;

/** サーバーのエラー本文({code, message})の形をしているかの型ガード(apiEngine.ts と同じ考え方)。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

/** 持ち物(公開 API に効果データが無いので effect は常に null。ADR-0304 §追記 A-1)。 */
function mapItem(item: Schemas["Item"]): Item {
  return { id: item.id, nameJa: item.nameJa, effect: null };
}

/** 特性(公開 API に効果データが無いので effect は常に null。ADR-0304 §追記 A-1)。 */
function mapAbility(ability: Schemas["Ability"]): Ability {
  return { id: ability.id, nameJa: ability.nameJa, effect: null };
}

/** 技(getMovesByIds の応答をそのまま MasterSpeciesResolution.moves に写す。ADR-0304 A-13)。 */
function mapMove(move: Schemas["Move"]): Move {
  return {
    id: move.id,
    nameJa: move.nameJa,
    type: move.type,
    category: move.category,
    power: move.power,
    priority: move.priority,
  };
}

/** 性格(plus・minus の省略と null をどちらも null にする)。 */
function mapNature(nature: Schemas["Nature"]): MasterNature {
  return { id: nature.id, nameJa: nature.nameJa, plus: nature.plus ?? null, minus: nature.minus ?? null };
}

/** 種族の検索結果の1件。 */
function mapSpeciesSummary(summary: Schemas["SpeciesSummary"]): MasterSpeciesSummary {
  return {
    key: summary.key,
    dexNo: summary.dexNo,
    form: summary.form,
    nameJa: summary.nameJa,
    types: summary.types,
  };
}

/** 種族の詳細を MasterSpecies に写す(特性は ID のみ・learnset は応答の順のまま。技の実体化は API レーン待ち)。 */
function mapSpeciesDetail(detail: Schemas["SpeciesDetail"]): MasterSpecies {
  return {
    key: detail.key,
    dexNo: detail.dexNo,
    form: detail.form,
    nameJa: detail.nameJa,
    types: detail.types,
    baseStats: detail.baseStats,
    abilities: detail.abilities.map((ability) => ability.id),
    learnset: detail.learnset ?? [],
  };
}

/** pokedex-svc の公開 API から読むマスタの取得口(P4-16、ADR-0304)。 */
export function createOnlineMasterSource(input: CreateOnlineMasterSourceInput): SearchableMasterSource {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /**
   * クエリパラメータを組み立てる。値が配列なら同じキーの繰り返しクエリにする
   * (`?ids=a&ids=b`。api/openapi.yaml の `getMovesByIds` の `ids`(`in: query` の配列)がこの形。
   * カンマ区切りの1件にはしない)。
   */
  function searchParamsOf(query: Record<string, string | readonly string[]>): URLSearchParams {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
      // Array.isArray(value) は使わない(lib.es5 の型が `any[]` を返すため unsafe-argument に引っかかる)。
      for (const item of typeof value === "string" ? [value] : value) {
        params.append(key, item);
      }
    }
    return params;
  }

  /** GET し、応答本文(パース済み)を返す。通信・応答の失敗は例外にする(空のマスタで握りつぶさない)。 */
  async function getJson(
    path: string,
    query?: Record<string, string | readonly string[]>,
    signal?: AbortSignal,
  ): Promise<unknown> {
    // new URL(path, baseUrl) は使わない: baseUrl の既定 "/"(api/config.ts)に対して
    // new URL("api/pokedex/items", "/") は TypeError(Invalid URL)を投げる。baseUrl は
    // 末尾スラッシュ1つが保証されているので、他のクライアント(apiEngine.ts 等)と同じ文字列連結にする。
    const search = query === undefined ? "" : `?${searchParamsOf(query).toString()}`;
    const url = `${baseUrl}${path}${search}`;
    const init: RequestInit = {
      headers: { "X-Device-Id": ids.deviceId, "X-Session-Id": ids.sessionId },
    };
    if (signal !== undefined) {
      init.signal = signal;
    }
    let response: Response;
    try {
      response = await fetchImpl(url, init);
    } catch (error) {
      if (signal?.aborted === true || (error instanceof DOMException && error.name === "AbortError")) {
        throw error;
      }
      throw new Error(`マスタの取得口に届かない: ${path}`, { cause: error });
    }
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch (error) {
      if (signal?.aborted === true || (error instanceof DOMException && error.name === "AbortError")) {
        throw error;
      }
      throw new Error(`マスタの応答が読めない: ${path}`, { cause: error });
    }
    if (!response.ok) {
      throw new Error(
        isErrorBody(parsed) ? `${parsed.code}: ${parsed.message}` : `マスタの取得に失敗した: ${path}`,
      );
    }
    return parsed;
  }

  async function load(): Promise<MasterData> {
    const [itemsRaw, naturesRaw] = await Promise.all([
      getJson(PATHS.items, { limit: String(ITEMS_FETCH_LIMIT) }),
      getJson(PATHS.natures),
    ]);
    const items = itemsRaw as Schemas["Item"][];
    if (items.length === ITEMS_FETCH_LIMIT) {
      throw new Error("持ち物の応答が limit ちょうど返った(打ち切りの疑いがあるため中断)");
    }
    const natures = naturesRaw as Schemas["Nature"][];
    return {
      species: [],
      moves: [],
      items: items.map(mapItem),
      abilities: [],
      natures: natures.map(mapNature),
      typeChart: typeChartFromData(typeChartData),
      capabilities: ONLINE_MASTER_CAPABILITIES,
    };
  }

  /**
   * learnset(技の ID 配列)を getMovesByIds で技の実体に解決する(ADR-0304 §3 追記・A-13)。
   * ids は契約上1〜64件なので、MOVES_BATCH_MAX_IDS 件ずつに分割して並列に呼ぶ(1種族の解決の中の話なので
   * 並列でよい。issue 110 が懸念した「1リクエストでの増幅」には当たらない)。1回でも失敗したら reject する
   * (Promise.all の既定の挙動)。learnset が空なら1回も呼ばない(ids の省略は 400 invalid_input)。
   */
  async function resolveMoves(learnset: readonly string[], signal?: AbortSignal): Promise<Move[]> {
    if (learnset.length === 0) {
      return [];
    }
    const chunks: string[][] = [];
    for (let offset = 0; offset < learnset.length; offset += MOVES_BATCH_MAX_IDS) {
      chunks.push(learnset.slice(offset, offset + MOVES_BATCH_MAX_IDS));
    }
    const responses = await Promise.all(chunks.map((ids) => getJson(PATHS.movesBatch, { ids }, signal)));
    // チャンクの順のまま連結すれば learnset の順のまま(1チャンク内は ids の順で返るのが契約)。
    return responses.flatMap((raw) => (raw as Schemas["Move"][]).map(mapMove));
  }

  const search: MasterSpeciesSearch = {
    async searchSpecies(query, signal) {
      const trimmed = query.trim();
      if (trimmed.length < SPECIES_SEARCH_MIN_LENGTH) {
        return [];
      }
      const raw = await getJson(PATHS.species, { q: trimmed, limit: String(SPECIES_SEARCH_LIMIT) }, signal);
      return (raw as Schemas["SpeciesSummary"][]).map(mapSpeciesSummary);
    },
    async resolveSpecies(key, signal) {
      const raw = await getJson(`${PATHS.species}/${encodeURIComponent(key)}`, undefined, signal);
      const detail = raw as Schemas["SpeciesDetail"];
      const learnset = detail.learnset ?? [];
      const moves = await resolveMoves(learnset, signal);
      const resolution: MasterSpeciesResolution = {
        species: mapSpeciesDetail(detail),
        abilities: detail.abilities.map(mapAbility),
        moves,
      };
      return resolution;
    },
  };

  return { load, search };
}
