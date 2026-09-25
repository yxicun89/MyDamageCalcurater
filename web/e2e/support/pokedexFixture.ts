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

import type { MasterData } from "../../src/master/types";

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
  throw new Error(
    `未実装(PR2・ADR-0307 の実装で埋める): ${request.method} ${request.path}` +
      `(例データの種族 ${String(master.species.length)}件)`,
  );
}
