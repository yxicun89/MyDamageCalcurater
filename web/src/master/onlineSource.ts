// P4-16: pokedex-svc の公開 API からマスタを読む MasterSource(ADR-0304)。
// 公開 API(api/openapi.yaml)の制約:
//   - searchSpecies / searchMoves / searchItems は limit の上限が 200 でページングが無い。
//     実データは種族349件・技515件なので一括取得できず、持ち物166件・性格25件だけが1回で全件取れる(ADR-0304 §1)。
//   - getSpecies.learnset は技の ID 配列だけで、ID から技の実体を引く公開 API が無い(ADR-0304 §3。API レーン待ち)。
// なので load() は「持ち物・性格・相性表」だけを返し、種族は MasterSpeciesSearch で都度引き、技は使えないことを
// capabilities で伝える(ADR-0304 §4 の段階的導入)。
// 応答の型は openapi-typescript の生成型(api/openapi.gen.ts)を正とし、手で複製しない(ADR-0301 §6)。

import typeChartData from "@typechart";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import type { Ability, Item } from "../engine/types";
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

  /** GET し、応答本文(パース済み)を返す。通信・応答の失敗は例外にする(空のマスタで握りつぶさない)。 */
  async function getJson(
    path: string,
    query?: Record<string, string>,
    signal?: AbortSignal,
  ): Promise<unknown> {
    // new URL(path, baseUrl) は使わない: baseUrl の既定 "/"(api/config.ts)に対して
    // new URL("api/pokedex/items", "/") は TypeError(Invalid URL)を投げる。baseUrl は
    // 末尾スラッシュ1つが保証されているので、他のクライアント(apiEngine.ts 等)と同じ文字列連結にする。
    const search = query === undefined ? "" : `?${new URLSearchParams(query).toString()}`;
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
      const resolution: MasterSpeciesResolution = {
        species: mapSpeciesDetail(detail),
        abilities: detail.abilities.map(mapAbility),
      };
      return resolution;
    },
  };

  return { load, search };
}
