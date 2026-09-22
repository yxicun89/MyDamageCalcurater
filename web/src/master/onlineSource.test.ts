// P4-16: pokedex-svc の公開 API から読む MasterSource(createOnlineMasterSource。ADR-0304)。
// 応答は openapi-typescript の生成型(api/openapi.gen.ts)で書き、契約とのずれを typecheck で検出する
// (ADR-0301 §6。手で型を複製しない)。fetch は fake。
// 確かめること:
//   - load() は「1回で全件取れるもの」だけを読む = 持ち物(limit=200)と性格(listNatures)の2本だけ。
//     種族・技は読まない(公開 API では一括取得できない。ADR-0304 §1)
//   - 全リクエストに X-Device-Id・X-Session-Id を付ける(CLAUDE.md 技術規約)
//   - 持ち物・特性は公開 API に効果データが無いので effect は null(capabilities.effects が false)
//   - 持ち物が limit ちょうど返ってきたら、打ち切られた可能性を黙って無視せず失敗する
//   - 種族は searchSpecies(前方一致・limit=50)で都度引き、空クエリでは fetch しない
//   - resolveSpecies は getSpecies を引き、learnset の ID を順序どおり保つ(技の実体化は API レーン待ち)
//   - 通信・応答の失敗は reject する(空のマスタで握りつぶさない。ADR-0301 §4 と同じ考え方)

import typeChartData from "@typechart";
import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import {
  ITEMS_FETCH_LIMIT,
  ONLINE_MASTER_CAPABILITIES,
  SPECIES_SEARCH_LIMIT,
  createOnlineMasterSource,
} from "./onlineSource";
import { typeChartFromData } from "./typeChart";
import type { SearchableMasterSource } from "./types";

type Schemas = components["schemas"];

const BASE_URL = "http://pokedex.test/";

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

/** 公開 API のパス(api/openapi.yaml の paths。基点 URL からの相対)。 */
const PATHS = {
  species: "/api/pokedex/species",
  items: "/api/pokedex/items",
  natures: "/api/pokedex/natures",
} as const;

const itemsResponse: Schemas["Item"][] = [
  { id: "example-item-def", nameJa: "テストぼうぎょだま" },
  { id: "example-item-power", nameJa: "テストちからのたま" },
];

const naturesResponse: Schemas["Nature"][] = [
  { id: "adamant", nameJa: "いじっぱり", plus: "atk", minus: "spa" },
  // 無補正の性格は plus・minus を省く(省略と null のどちらも来うる。両方とも null に写す)。
  { id: "hardy", nameJa: "がんばりや" },
  { id: "docile", nameJa: "すなお", plus: null, minus: null },
];

const speciesSummaries: Schemas["SpeciesSummary"][] = [
  { key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストポケモンA", types: ["fire"] },
  { key: "9002-000", dexNo: 9002, form: 0, nameJa: "テストポケモンB", types: ["water", "flying"] },
];

const speciesDetail: Schemas["SpeciesDetail"] = {
  key: "9001-000",
  dexNo: 9001,
  form: 0,
  nameJa: "テストポケモンA",
  types: ["fire"],
  baseStats: { hp: 100, atk: 110, def: 90, spa: 80, spd: 85, spe: 95 },
  abilities: [
    { id: "example-ability-blaze", nameJa: "テストもうか" },
    { id: "example-ability-none", nameJa: "テストなし" },
  ],
  // 並びは API の返す順(ID 昇順)をそのまま保つ。マスタの並べ替えをクライアントでしない。
  learnset: ["example-move-firepunch", "example-move-tackle", "example-move-growl"],
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/** fetch の第1引数(文字列・URL・Request のいずれか)から URL を作る。 */
function urlOf(input: RequestInfo | URL): URL {
  if (input instanceof URL) {
    return input;
  }
  return new URL(typeof input === "string" ? input : input.url);
}

/** パスごとに応答を決める fake fetch(応答を用意していないパスを引いたらテストを落とす)。 */
function routedFetch(routes: Partial<Record<string, () => Response>>) {
  return vi.fn<typeof fetch>((input) => {
    const url = urlOf(input);
    const route =
      routes[url.pathname] ?? (url.pathname.startsWith(`${PATHS.species}/`) ? routes.detail : undefined);
    if (route === undefined) {
      throw new Error(`テストが用意していないパスを引いた: ${url.pathname}`);
    }
    return Promise.resolve(route());
  });
}

/** 既定(すべて成功)の fake fetch。 */
function okFetch(overrides: Partial<Record<string, () => Response>> = {}) {
  return routedFetch({
    [PATHS.items]: () => jsonResponse(200, itemsResponse),
    [PATHS.natures]: () => jsonResponse(200, naturesResponse),
    [PATHS.species]: () => jsonResponse(200, speciesSummaries),
    detail: () => jsonResponse(200, speciesDetail),
    ...overrides,
  });
}

function createSource(fetchImpl: typeof fetch): SearchableMasterSource {
  return createOnlineMasterSource({ baseUrl: BASE_URL, fetch: fetchImpl, ids });
}

/** fetch の呼び出しのうち、pathname が一致する最初のものを返す。 */
function callTo(fetchMock: ReturnType<typeof okFetch>, pathname: string): { url: URL; init: RequestInit } {
  for (const [input, init] of fetchMock.mock.calls) {
    const url = urlOf(input);
    if (url.pathname === pathname) {
      return { url, init: init ?? {} };
    }
  }
  throw new Error(`${pathname} への fetch が無い`);
}

/** リクエストヘッダーから1つ読む(Headers でも素のオブジェクトでも読めるようにする)。 */
function headerOf(init: RequestInit, name: string): string | null {
  return new Headers(init.headers).get(name);
}

test("load は持ち物と性格だけを読む(種族・技は読まない)", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).load();

  const paths = fetchMock.mock.calls.map(([input]) => urlOf(input).pathname);
  expect(paths.toSorted()).toEqual([PATHS.items, PATHS.natures].toSorted());
});

test("持ち物は基点 URL からの相対パスを limit=200(公開 API の上限)で引き、q は付けない", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).load();
  const { url } = callTo(fetchMock, PATHS.items);
  expect(url.origin).toBe(new URL(BASE_URL).origin);
  expect(url.searchParams.get("limit")).toBe(String(ITEMS_FETCH_LIMIT));
  expect(url.searchParams.has("q")).toBe(false);
});

test("性格はクエリなしで引く(listNatures は無条件に全件)", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).load();
  const { url } = callTo(fetchMock, PATHS.natures);
  expect([...url.searchParams.keys()]).toEqual([]);
});

test("すべてのリクエストに端末 ID・セッション ID を付ける", async () => {
  const fetchMock = okFetch();
  const source = createSource(fetchMock);
  await source.load();
  await source.search.searchSpecies("テ");
  await source.search.resolveSpecies("9001-000");

  expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(4);
  for (const [, init] of fetchMock.mock.calls) {
    expect(headerOf(init ?? {}, "X-Device-Id")).toBe(ids.deviceId);
    expect(headerOf(init ?? {}, "X-Session-Id")).toBe(ids.sessionId);
  }
});

test("load は持ち物を応答の順のまま写す(効果データは公開 API に無いので null)", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.items).toEqual([
    { id: "example-item-def", nameJa: "テストぼうぎょだま", effect: null },
    { id: "example-item-power", nameJa: "テストちからのたま", effect: null },
  ]);
});

test("load は性格を写し、plus・minus の省略と null をどちらも null にする", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.natures).toEqual([
    { id: "adamant", nameJa: "いじっぱり", plus: "atk", minus: "spa" },
    { id: "hardy", nameJa: "がんばりや", plus: null, minus: null },
    { id: "docile", nameJa: "すなお", plus: null, minus: null },
  ]);
});

test("load の種族・技・特性は空(公開 API では一括取得できない。都度引く)", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.species).toEqual([]);
  expect(master.moves).toEqual([]);
  expect(master.abilities).toEqual([]);
});

test("load の相性表は P1-13 のデータ(オフラインと同じ単一の正)", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.typeChart).toEqual(typeChartFromData(typeChartData));
});

test("load は使える機能(capabilities)を報告する(種族は検索・技は未対応・効果データなし)", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.capabilities).toEqual(ONLINE_MASTER_CAPABILITIES);
  expect(ONLINE_MASTER_CAPABILITIES).toEqual({ speciesList: false, moves: false, effects: false });
});

test("持ち物が limit ちょうど返ったら、打ち切りを黙って受け入れずに失敗する", async () => {
  const truncated: Schemas["Item"][] = Array.from({ length: ITEMS_FETCH_LIMIT }, (_unused, index) => ({
    id: `example-item-${String(index)}`,
    nameJa: `テスト持ち物${String(index)}`,
  }));
  const fetchMock = okFetch({ [PATHS.items]: () => jsonResponse(200, truncated) });
  await expect(createSource(fetchMock).load()).rejects.toThrow(Error);
});

test("load は HTTP エラーで失敗する(空のマスタを返さない)", async () => {
  const error: Schemas["Error"] = { code: "master_unavailable", message: "マスタが未投入" };
  const fetchMock = okFetch({ [PATHS.items]: () => jsonResponse(503, error) });
  await expect(createSource(fetchMock).load()).rejects.toThrow(Error);
});

test("load は通信できないときに失敗する", async () => {
  const fetchMock = vi.fn<typeof fetch>(() => Promise.reject(new TypeError("ネットワークに届かない")));
  await expect(createSource(fetchMock).load()).rejects.toThrow(Error);
});

test("load は応答が JSON でないときに失敗する", async () => {
  const fetchMock = okFetch({
    [PATHS.natures]: () => new Response("<html>proxy error</html>", { status: 200 }),
  });
  await expect(createSource(fetchMock).load()).rejects.toThrow(Error);
});

test("searchSpecies は空のクエリでは fetch せず空を返す(空 = 全件にしない)", async () => {
  const fetchMock = okFetch();
  const source = createSource(fetchMock);
  expect(await source.search.searchSpecies("")).toEqual([]);
  expect(await source.search.searchSpecies("   ")).toEqual([]);
  expect(fetchMock).not.toHaveBeenCalled();
});

test("searchSpecies は前方一致の q と limit=50 で引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.searchSpecies("テスト");
  const { url } = callTo(fetchMock, PATHS.species);
  expect(url.searchParams.get("q")).toBe("テスト");
  expect(url.searchParams.get("limit")).toBe(String(SPECIES_SEARCH_LIMIT));
});

test("searchSpecies は前後の空白を落としてから引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.searchSpecies("  テスト  ");
  expect(callTo(fetchMock, PATHS.species).url.searchParams.get("q")).toBe("テスト");
});

test("searchSpecies は応答を候補の形にそのままの順で写す", async () => {
  const found = await createSource(okFetch()).search.searchSpecies("テスト");
  expect(found).toEqual([
    { key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストポケモンA", types: ["fire"] },
    { key: "9002-000", dexNo: 9002, form: 0, nameJa: "テストポケモンB", types: ["water", "flying"] },
  ]);
});

test("searchSpecies は AbortSignal を fetch に渡す(入力ごとの取り消しに使う)", async () => {
  const fetchMock = okFetch();
  const controller = new AbortController();
  await createSource(fetchMock).search.searchSpecies("テスト", controller.signal);
  expect(callTo(fetchMock, PATHS.species).init.signal).toBe(controller.signal);
});

test("searchSpecies は失敗を握りつぶさない(空配列にしない)", async () => {
  const fetchMock = okFetch({ [PATHS.species]: () => jsonResponse(503, { code: "x", message: "y" }) });
  await expect(createSource(fetchMock).search.searchSpecies("テスト")).rejects.toThrow(Error);
});

test("resolveSpecies は種族の key のパスを引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.resolveSpecies("9001-000");
  const paths = fetchMock.mock.calls.map(([input]) => urlOf(input).pathname);
  expect(paths).toEqual([`${PATHS.species}/9001-000`]);
});

test("resolveSpecies は種族を MasterSpecies に写す(特性は ID の配列・learnset は応答の順のまま)", async () => {
  const resolved = await createSource(okFetch()).search.resolveSpecies("9001-000");
  expect(resolved.species).toEqual({
    key: "9001-000",
    dexNo: 9001,
    form: 0,
    nameJa: "テストポケモンA",
    types: ["fire"],
    baseStats: { hp: 100, atk: 110, def: 90, spa: 80, spd: 85, spe: 95 },
    abilities: ["example-ability-blaze", "example-ability-none"],
    // 技の実体化は公開 API に無い(ADR-0304 §3)。ID はそのまま保ち、API レーンの対応後に解決できるようにする。
    learnset: ["example-move-firepunch", "example-move-tackle", "example-move-growl"],
  });
});

test("resolveSpecies は種族の特性の実体も返す(効果データは公開 API に無いので null)", async () => {
  const resolved = await createSource(okFetch()).search.resolveSpecies("9001-000");
  expect(resolved.abilities).toEqual([
    { id: "example-ability-blaze", nameJa: "テストもうか", effect: null },
    { id: "example-ability-none", nameJa: "テストなし", effect: null },
  ]);
});

test("resolveSpecies は learnset の無い応答を空の learnset にする", async () => {
  const withoutLearnset: Schemas["SpeciesDetail"] = { ...speciesDetail };
  delete withoutLearnset.learnset;
  const fetchMock = okFetch({ detail: () => jsonResponse(200, withoutLearnset) });
  const resolved = await createSource(fetchMock).search.resolveSpecies("9001-000");
  expect(resolved.species.learnset).toEqual([]);
});

test("resolveSpecies は見つからない種族で失敗する", async () => {
  const error: Schemas["Error"] = { code: "not_found", message: "該当する種族が無い" };
  const fetchMock = okFetch({ detail: () => jsonResponse(404, error) });
  await expect(createSource(fetchMock).search.resolveSpecies("9999-000")).rejects.toThrow(Error);
});

test("resolveSpecies は AbortSignal を fetch に渡す", async () => {
  const fetchMock = okFetch();
  const controller = new AbortController();
  await createSource(fetchMock).search.resolveSpecies("9001-000", controller.signal);
  const [, init] = fetchMock.mock.calls[0] ?? [];
  expect((init ?? {}).signal).toBe(controller.signal);
});
