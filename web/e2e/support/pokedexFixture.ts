// PR2(ADR-0307): E2E 専用の pokedex フィクスチャ。Web の架空の例データ(src/master/example/)から
// pokedex-svc の公開 API(api/openapi.yaml の `/api/pokedex/*`)の応答を作る。
//
// なぜ要るか: `make e2e` の `web-e2e-online` は calc-svc だけを起動する設計(ADR-0301 §5・ADR-0306 の
// 「k3d 不要」)だが、P4-16(ADR-0304)以降「オンライン」モードのマスタは pokedex-svc の公開 API から
// 読む(createOnlineMasterSource)。calc-svc は担当外の pokedex の操作を意図的に 404 で返す
// (services/calc/internal/httpapi/server.go の registerPokedexNotFoundRoutes。ADR-0301 §5 追記の R1)ため、
// オンラインのマスタが読めない。calc-svc / pokedex-svc 本体には触らず、Web 側だけで完結させる。
//
// 責務の分け方: **応答を作るのは下の純粋関数だけ**で、HTTP の待ち受けは pokedexFixtureServer.mjs が持つ。
// 純粋関数にしてあるので、vitest の契約テスト(pokedexFixture.contract.test.ts)が本物の契約
// (api/openapi.yaml → src/api/openapi.gen.ts の生成型)とのずれを毎回検出できる。
//
// 実データは持たない(架空の例データだけ。CLAUDE.md ドメイン規約・ADR-0002)。

import type { components } from "../../src/api/openapi.gen";
import type { Move } from "../../src/engine/types";
import { MOVES_BATCH_MAX_IDS } from "../../src/master/onlineSource";
import type { MasterData } from "../../src/master/types";

type Schemas = components["schemas"];

/**
 * フィクスチャが受け取る1リクエスト(pokedexFixtureServer.mjs が node:http の IncomingMessage から作る)。
 * HTTP そのものに依存させないことで、契約テストが直接呼べるようにしてある。
 */
export interface FixtureRequest {
  /** HTTP メソッド(大文字)。 */
  readonly method: string;
  /** クエリを除いたパス(例 `/api/pokedex/species/9001-000`)。 */
  readonly path: string;
  /** クエリ。`ids` のような繰り返しパラメータを保つため URLSearchParams で受ける。 */
  readonly query: URLSearchParams;
  /** ヘッダ(名前は小文字化済み)。 */
  readonly headers: Readonly<Record<string, string | undefined>>;
}

/** フィクスチャが返す1応答(本文は JSON として書き出す値)。 */
export interface FixtureResponse {
  readonly status: number;
  readonly body: unknown;
}

/** `searchItems` / `searchSpecies` の `limit` の既定値(api/openapi.yaml の `default`)。 */
export const SEARCH_LIMIT_DEFAULT = 50;

/** `searchItems` / `searchSpecies` の `limit` の上限(api/openapi.yaml の `maximum`)。 */
export const SEARCH_LIMIT_MAX = 200;

/**
 * pokedex の公開 API の応答を作る(このフィクスチャの唯一の本体)。
 *
 * 実装する挙動(api/openapi.yaml の pokedex の公開エンドポイントに合わせる):
 *
 * | リクエスト | 応答 |
 * |---|---|
 * | `GET /healthz` | 200 `{ status: "ok" }`(Playwright の webServer の待ち受け確認用) |
 * | `GET /api/pokedex/items?q=&limit=` | 200 `Item[]`。`nameJa` の前方一致(`q` 省略・空は全件)、`limit` 件まで。並びは `nameJa` の日本語の照合順序の昇順・同順位は `id` 昇順 |
 * | `GET /api/pokedex/natures` | 200 `Nature[]`(全件・`id` 昇順)。`plus` / `minus` は無補正なら `null` |
 * | `GET /api/pokedex/species?q=&limit=` | 200 `SpeciesSummary[]`。`nameJa` の前方一致、`limit` 件まで、`key` 昇順 |
 * | `GET /api/pokedex/species/{key}` | 200 `SpeciesDetail`(`abilities` は `Ability` の配列・`learnset` は技の ID 配列) |
 * | `GET /api/pokedex/moves/batch?ids=…` | 200 `Move[]`。`ids` の順のまま、マスタに無い ID は詰めて省き、重複した ID は重複したまま返す |
 *
 * 応答の本文は**公開 API のスキーマちょうど**にする(例データが持つ `Item.effect`・`Ability.effect`・
 * `MasterSpecies.learnset` を `SpeciesSummary` に混ぜる、といった漏れを起こさない)。
 *
 * エラー(本文は `Error` = `{ code, message }`):
 *
 * | 条件 | 応答 |
 * |---|---|
 * | `X-Device-Id` / `X-Session-Id` が無い・空 | 400 `missing_header` |
 * | 同ヘッダが UUID でない | 400 `invalid_header`(UUID の検証は gateway の担当。ADR-0202) |
 * | `limit` が整数でない・1〜200 の外 | 400 `invalid_input` |
 * | `{key}` が `^[0-9]{4}-[0-9]{3}$` でない | 400 `invalid_input` |
 * | `{key}` がマスタに無い | 404 `not_found` |
 * | `ids` が無い・0件・65件以上 | 400 `invalid_input` |
 * | 上記以外のパス、および GET 以外のメソッド | 404 `not_found`(E2E 専用の簡略化。405 は作らない。ADR-0307) |
 *
 * @param master 例データ(`exampleMasterSource.load()` の結果)
 * @param request 受け取ったリクエスト
 */
export function handlePokedexRequest(master: MasterData, request: FixtureRequest): FixtureResponse {
  if (request.path === "/healthz") {
    return { status: 200, body: { status: "ok" } };
  }
  if (request.method !== "GET") {
    return notFound();
  }
  const headerError = validateHeaders(request.headers);
  if (headerError !== null) {
    return headerError;
  }

  if (request.path === "/api/pokedex/items") {
    return handleItems(master, request.query);
  }
  if (request.path === "/api/pokedex/natures") {
    return handleNatures(master);
  }
  if (request.path === "/api/pokedex/species") {
    return handleSpeciesSearch(master, request.query);
  }
  if (request.path === "/api/pokedex/moves/batch") {
    return handleMovesBatch(master, request.query);
  }
  const speciesDetailMatch = /^\/api\/pokedex\/species\/([^/]+)$/.exec(request.path);
  if (speciesDetailMatch !== null) {
    const key = speciesDetailMatch[1];
    if (key !== undefined) {
      return handleSpeciesDetail(master, key);
    }
  }
  return notFound();
}

// --- ヘッダ検証(ADR-0202 の gateway の検証を簡略に再現) --------------------------------------

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function validateHeaders(headers: Readonly<Record<string, string | undefined>>): FixtureResponse | null {
  const deviceId = headers["x-device-id"];
  const sessionId = headers["x-session-id"];
  if (deviceId === undefined || deviceId === "" || sessionId === undefined || sessionId === "") {
    return errorResponse(400, "missing_header", "X-Device-Id / X-Session-Id が無い");
  }
  if (!UUID_PATTERN.test(deviceId) || !UUID_PATTERN.test(sessionId)) {
    return errorResponse(400, "invalid_header", "X-Device-Id / X-Session-Id が UUID でない");
  }
  return null;
}

// --- 各エンドポイント -----------------------------------------------------------------------

const SPECIES_KEY_PATTERN = /^[0-9]{4}-[0-9]{3}$/;

function handleItems(master: MasterData, query: URLSearchParams): FixtureResponse {
  const limit = parseLimit(query);
  if (limit === null) {
    return errorResponse(400, "invalid_input", "limit が不正");
  }
  const filtered = prefixFilter(master.items, query.get("q"), (item) => item.nameJa);
  const sorted = sortByNameThenId(filtered);
  const body: Schemas["Item"][] = sorted
    .slice(0, limit)
    .map((item) => ({ id: item.id, nameJa: item.nameJa }));
  return { status: 200, body };
}

function handleNatures(master: MasterData): FixtureResponse {
  const sorted = [...master.natures].sort((a, b) => compareStrings(a.id, b.id));
  const body: Schemas["Nature"][] = sorted.map((nature) => ({
    id: nature.id,
    nameJa: nature.nameJa,
    plus: nature.plus ?? null,
    minus: nature.minus ?? null,
  }));
  return { status: 200, body };
}

function handleSpeciesSearch(master: MasterData, query: URLSearchParams): FixtureResponse {
  const limit = parseLimit(query);
  if (limit === null) {
    return errorResponse(400, "invalid_input", "limit が不正");
  }
  const filtered = prefixFilter(master.species, query.get("q"), (species) => species.nameJa);
  const sorted = [...filtered].sort((a, b) => compareStrings(a.key, b.key));
  const body: Schemas["SpeciesSummary"][] = sorted.slice(0, limit).map((species) => ({
    key: species.key,
    dexNo: species.dexNo,
    form: species.form,
    nameJa: species.nameJa,
    types: species.types as Schemas["PokeType"][],
  }));
  return { status: 200, body };
}

function handleSpeciesDetail(master: MasterData, key: string): FixtureResponse {
  if (!SPECIES_KEY_PATTERN.test(key)) {
    return errorResponse(400, "invalid_input", `speciesKey の形式が不正: ${key}`);
  }
  const species = master.species.find((candidate) => candidate.key === key);
  if (species === undefined) {
    return errorResponse(404, "not_found", `species が見つからない: ${key}`);
  }
  const body: Schemas["SpeciesDetail"] = {
    key: species.key,
    dexNo: species.dexNo,
    form: species.form,
    nameJa: species.nameJa,
    types: species.types as Schemas["PokeType"][],
    baseStats: species.baseStats,
    abilities: species.abilities.map((id) => resolveAbility(master, id)),
    learnset: [...species.learnset],
  };
  return { status: 200, body };
}

function handleMovesBatch(master: MasterData, query: URLSearchParams): FixtureResponse {
  const ids = query.getAll("ids");
  if (ids.length === 0 || ids.length > MOVES_BATCH_MAX_IDS) {
    return errorResponse(400, "invalid_input", "ids が不正(1〜64件で指定する)");
  }
  const body: Schemas["Move"][] = ids
    .map((id) => master.moves.find((move) => move.id === id))
    .filter((move): move is Move => move !== undefined)
    .map((move) => ({
      id: move.id,
      nameJa: move.nameJa,
      type: move.type as Schemas["PokeType"],
      category: move.category,
      power: move.power,
      priority: move.priority,
    }));
  return { status: 200, body };
}

// --- 補助関数 -------------------------------------------------------------------------------

/** `limit` を検証する。範囲外・整数でなければ null(呼び出し側が 400 invalid_input にする)。 */
function parseLimit(query: URLSearchParams): number | null {
  const raw = query.get("limit");
  if (raw === null) {
    return SEARCH_LIMIT_DEFAULT;
  }
  if (!/^\d+$/.test(raw)) {
    return null;
  }
  const value = Number(raw);
  if (value < 1 || value > SEARCH_LIMIT_MAX) {
    return null;
  }
  return value;
}

/** `q` の省略・空は全件、それ以外は前方一致で絞る。 */
function prefixFilter<T>(list: readonly T[], q: string | null, nameOf: (item: T) => string): T[] {
  if (q === null || q === "") {
    return [...list];
  }
  return list.filter((item) => nameOf(item).startsWith(q));
}

function compareStrings(a: string, b: string): number {
  if (a < b) {
    return -1;
  }
  if (a > b) {
    return 1;
  }
  return 0;
}

/** `nameJa` の日本語の照合順序の昇順・同順位は `id` 昇順。 */
function sortByNameThenId<T extends { readonly id: string; readonly nameJa: string }>(
  list: readonly T[],
): T[] {
  return [...list].sort((a, b) => {
    const byName = a.nameJa.localeCompare(b.nameJa, "ja");
    return byName !== 0 ? byName : compareStrings(a.id, b.id);
  });
}

function resolveAbility(master: MasterData, id: string): Schemas["Ability"] {
  const ability = master.abilities.find((candidate) => candidate.id === id);
  if (ability === undefined) {
    throw new Error(`例データに ability ${id} が無い(species.abilities と abilities のずれ)`);
  }
  return { id: ability.id, nameJa: ability.nameJa };
}

function errorResponse(status: number, code: Schemas["ErrorCode"], message: string): FixtureResponse {
  const body: Schemas["Error"] = { code, message };
  return { status, body };
}

function notFound(): FixtureResponse {
  return errorResponse(404, "not_found", "担当外の操作、または未知のパス");
}
