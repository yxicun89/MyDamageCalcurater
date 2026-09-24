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
//   - resolveSpecies は getSpecies を引き、learnset の ID を順序どおり保つ
//   - P4-17: resolveSpecies は learnset を getMovesByIds(GET /api/pokedex/moves/batch?ids=...)で技の実体に
//     解決する。ids は契約上1〜64件なので、64件ずつに分割して複数回引く(ADR-0304 §3 追記・A-13)
//   - 通信・応答の失敗は reject する(空のマスタで握りつぶさない。ADR-0301 §4 と同じ考え方)

import typeChartData from "@typechart";
import { readFile } from "node:fs/promises";
import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { localPath } from "../test/localPath";
import {
  ITEMS_FETCH_LIMIT,
  MOVES_BATCH_MAX_IDS,
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
  movesBatch: "/api/pokedex/moves/batch",
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

/**
 * getMovesByIds が引く技のマスタ(架空)。`example-move-growl` はわざと**含めない**:
 * 契約どおり「マスタに無い ID は黙って省く」ことを確かめるため(ADR-0304 §3 追記)。
 */
const moveMaster: Schemas["Move"][] = [
  {
    id: "example-move-firepunch",
    nameJa: "テストほのおのパンチ",
    type: "fire",
    category: "physical",
    power: 75,
    priority: 0,
  },
  {
    id: "example-move-tackle",
    nameJa: "テストたいあたり",
    type: "normal",
    category: "physical",
    power: 40,
    priority: 0,
  },
];

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/**
 * fetch の第1引数(文字列・URL・Request のいずれか)から URL を作る。
 * baseUrl: "/" のとき実装が渡すのは相対パス文字列("/api/pokedex/items?...")なので、
 * 絶対 URL には影響しないダミーの基点(new URL は絶対文字列に対して第2引数を無視する)を添える。
 */
function urlOf(input: RequestInfo | URL): URL {
  if (input instanceof URL) {
    return input;
  }
  return new URL(typeof input === "string" ? input : input.url, "http://localhost/");
}

/**
 * fetch の第1引数を、基点を足さずに URL にする(相対パスなら TypeError で落ちる)。
 * urlOf のダミーの基点は相対パスを黙って吸収してしまうので、「基点 URL を使っているか」を見るときは
 * こちらを使う(P4-16c(3)。BASE_URL の origin はダミーの基点と別のホストにしてある)。
 */
function absoluteUrlOf(input: RequestInfo | URL): URL {
  if (input instanceof URL) {
    return input;
  }
  return new URL(typeof input === "string" ? input : input.url);
}

/** 1つのパスの応答(引いた URL を見て決められる。getMovesByIds は ids に応じて返すため)。 */
type Route = (url: URL) => Response;

/** パスごとに応答を決める fake fetch(応答を用意していないパスを引いたらテストを落とす)。 */
function routedFetch(routes: Partial<Record<string, Route>>) {
  return vi.fn<typeof fetch>((input) => {
    const url = urlOf(input);
    const route =
      routes[url.pathname] ?? (url.pathname.startsWith(`${PATHS.species}/`) ? routes.detail : undefined);
    if (route === undefined) {
      throw new Error(`テストが用意していないパスを引いた: ${url.pathname}`);
    }
    return Promise.resolve(route(url));
  });
}

/**
 * getMovesByIds の応答を契約どおりに作る(api/openapi.yaml の /api/pokedex/moves/batch):
 * ids の順のまま返し、マスタに無い ID は**詰めて省く**(エラーにしない)。
 */
function movesBatchRoute(master: readonly Schemas["Move"][]): Route {
  return (url) => {
    const ids = url.searchParams.getAll("ids");
    return jsonResponse(
      200,
      ids.flatMap((id) => master.filter((move) => move.id === id)),
    );
  };
}

/** 既定(すべて成功)の fake fetch。 */
function okFetch(overrides: Partial<Record<string, Route>> = {}) {
  return routedFetch({
    [PATHS.items]: () => jsonResponse(200, itemsResponse),
    [PATHS.natures]: () => jsonResponse(200, naturesResponse),
    [PATHS.species]: () => jsonResponse(200, speciesSummaries),
    [PATHS.movesBatch]: movesBatchRoute(moveMaster),
    detail: () => jsonResponse(200, speciesDetail),
    ...overrides,
  });
}

/** moves/batch への呼び出しの ids(呼ばれた順)。 */
function batchIdCalls(fetchMock: ReturnType<typeof okFetch>): string[][] {
  return fetchMock.mock.calls
    .map(([input]) => urlOf(input))
    .filter((url) => url.pathname === PATHS.movesBatch)
    .map((url) => url.searchParams.getAll("ids"));
}

function createSource(fetchImpl: typeof fetch): SearchableMasterSource {
  return createOnlineMasterSource({ baseUrl: BASE_URL, fetch: fetchImpl, ids });
}

/** fetch の呼び出しのうち、pathname が一致する最初のもの(input は fetch に渡った生の値)。 */
function callTo(
  fetchMock: ReturnType<typeof okFetch>,
  pathname: string,
): { url: URL; init: RequestInit; input: RequestInfo | URL } {
  for (const [input, init] of fetchMock.mock.calls) {
    const url = urlOf(input);
    if (url.pathname === pathname) {
      return { url, init: init ?? {}, input };
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

test("性格は基点 URL からの相対パスをクエリなしで引く(listNatures は無条件に全件)", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).load();
  const { url, input } = callTo(fetchMock, PATHS.natures);
  expect([...url.searchParams.keys()]).toEqual([]);
  // 持ち物と同じく、基点 URL の origin から引く(P4-16c(3))。
  expect(absoluteUrlOf(input).origin).toBe(new URL(BASE_URL).origin);
});

test("基点 URL が同じオリジン(api/config.ts の既定 '/')でも load できる(new URL(path, '/') は Invalid URL になる)", async () => {
  const fetchMock = okFetch();
  const source = createOnlineMasterSource({ baseUrl: "/", fetch: fetchMock, ids });
  await expect(source.load()).resolves.toMatchObject({ capabilities: ONLINE_MASTER_CAPABILITIES });
  const { url } = callTo(fetchMock, PATHS.items);
  expect(url.pathname).toBe(PATHS.items);
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

test("load は使える機能(capabilities)を報告する(種族・技は一覧にできない・効果データなし)", async () => {
  const master = await createSource(okFetch()).load();
  expect(master.capabilities).toEqual(ONLINE_MASTER_CAPABILITIES);
  // P4-17 でも moves は false のまま。この項目の意味は「MasterData.moves が**全件**そろっているか」で、
  // searchMoves の limit 上限200 < 実データ515件という事実は技の ID 解決が入っても変わらない
  // (ADR-0304 §1・A-13)。種族ごとの技は resolveSpecies が返す(下の P4-17 の節)。
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

test("searchSpecies は基点 URL から、前方一致の q と limit=50 で引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.searchSpecies("テスト");
  const { url, input } = callTo(fetchMock, PATHS.species);
  expect(url.searchParams.get("q")).toBe("テスト");
  expect(url.searchParams.get("limit")).toBe(String(SPECIES_SEARCH_LIMIT));
  expect(absoluteUrlOf(input).origin).toBe(new URL(BASE_URL).origin);
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

test("resolveSpecies は基点 URL から種族の key のパスを引き、続けて技の一括解決を引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.resolveSpecies("9001-000");
  const paths = fetchMock.mock.calls.map(([input]) => urlOf(input).pathname);
  // P4-17: 技の ID は種族の応答(learnset)が来てから決まるので、必ず詳細 → moves/batch の順になる。
  expect(paths).toEqual([`${PATHS.species}/9001-000`, PATHS.movesBatch]);
  const { input } = callTo(fetchMock, `${PATHS.species}/9001-000`);
  expect(absoluteUrlOf(input).origin).toBe(new URL(BASE_URL).origin);
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
    // learnset は ID のまま保つ(公開 API の SpeciesDetail.learnset は P4-17 でも string[] のまま。
    // 技の実体は resolution.moves に入る)。
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

// ---- P4-16b: P4-16 の critic が残した軽微な積み残し(plan.md P4-16b の(1)〜(3)) ----

test("searchSpecies は fetch の AbortError をそのまま返す(汎用のエラーに包まない)", async () => {
  const abortError = new DOMException("中断した", "AbortError");
  const fetchMock = vi.fn<typeof fetch>(() => Promise.reject(abortError));
  const controller = new AbortController();
  controller.abort();

  await expect(createSource(fetchMock).search.searchSpecies("テスト", controller.signal)).rejects.toBe(
    abortError,
  );
});

test("応答の読み出し中の AbortError もそのまま返す(取り消しと通信失敗を取り違えない)", async () => {
  const abortError = new DOMException("中断した", "AbortError");
  // 本文を読んでいる途中で取り消されると、失敗するのは fetch ではなく response.json() の側になる。
  const abortingBody = {
    ok: true,
    status: 200,
    json: () => Promise.reject(abortError),
  } as unknown as Response;
  const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(abortingBody));
  const controller = new AbortController();
  controller.abort();

  await expect(createSource(fetchMock).search.searchSpecies("テスト", controller.signal)).rejects.toBe(
    abortError,
  );
});

test("性格・種族の取得先も基点 URL の origin を使う(相対パスの組み立てを取り違えない)", async () => {
  const fetchMock = okFetch();
  const source = createSource(fetchMock);
  await source.load();
  await source.search.searchSpecies("テスト");
  await source.search.resolveSpecies("9001-000");

  const base = new URL(BASE_URL);
  for (const [input] of fetchMock.mock.calls) {
    expect(urlOf(input).origin).toBe(base.origin);
  }
});

// ---- P4-17: learnset の ID を技の実体に解決する(ADR-0304 §3 の解消・A-13) ----

/** 学習技が count 件ある架空の種族の詳細(ID は `example-move-0` …)。 */
function detailWithLearnset(count: number): Schemas["SpeciesDetail"] {
  return {
    ...speciesDetail,
    learnset: Array.from({ length: count }, (_unused, index) => `example-move-${String(index)}`),
  };
}

/** detailWithLearnset の learnset に対応する技のマスタ(全件あるので1件も省かれない)。 */
function moveMasterFor(count: number): Schemas["Move"][] {
  return Array.from({ length: count }, (_unused, index): Schemas["Move"] => ({
    id: `example-move-${String(index)}`,
    nameJa: `テスト技${String(index)}`,
    type: "normal",
    category: "physical",
    power: 40,
    priority: 0,
  }));
}

/** learnset が count 件ある種族を解決する fake fetch(技のマスタも同じ件数そろえる)。 */
function learnsetFetch(count: number) {
  return okFetch({
    detail: () => jsonResponse(200, detailWithLearnset(count)),
    [PATHS.movesBatch]: movesBatchRoute(moveMasterFor(count)),
  });
}

test("MOVES_BATCH_MAX_IDS は契約(api/openapi.yaml の ids の maxItems)と同じ", async () => {
  // 契約と定数がずれたら 400 invalid_input を踏む。Go 側(pokedex-svc)と同じく同期をテストで固定する。
  const contract = await readFile(localPath("../../../api/openapi.yaml", import.meta.url), "utf8");
  const fromBatchPath = contract.slice(contract.indexOf("/api/pokedex/moves/batch:"));
  expect(fromBatchPath).not.toBe("");
  expect(/maxItems:\s*(\d+)/.exec(fromBatchPath)?.[1]).toBe(String(MOVES_BATCH_MAX_IDS));
});

test("resolveSpecies は learnset の ID を ids に並べて moves/batch を引く", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.resolveSpecies("9001-000");

  expect(batchIdCalls(fetchMock)).toEqual([
    ["example-move-firepunch", "example-move-tackle", "example-move-growl"],
  ]);
  // 繰り返しクエリ(?ids=a&ids=b)であって、カンマ区切りの1件ではない(契約の `in: query` の配列)。
  expect(callTo(fetchMock, PATHS.movesBatch).url.searchParams.get("ids")).toBe("example-move-firepunch");
});

test("resolveSpecies は技を実体(名前・タイプ・分類・威力・優先度)にして返す(ADR-0304 A-13)", async () => {
  const resolved = await createSource(okFetch()).search.resolveSpecies("9001-000");
  expect(resolved.moves).toEqual(moveMaster);
});

test("優先度は応答の値をそのまま写す(先制技を 0 に潰さない)", async () => {
  const quick: Schemas["Move"] = {
    id: "example-move-firepunch",
    nameJa: "テストさきどりパンチ",
    type: "fire",
    category: "physical",
    power: 40,
    priority: 1,
  };
  const fetchMock = okFetch({ [PATHS.movesBatch]: movesBatchRoute([quick]) });
  const resolved = await createSource(fetchMock).search.resolveSpecies("9001-000");
  expect(resolved.moves).toEqual([quick]);
});

test("応答に含まれない ID(マスタに無い技)は詰めて省く(エラーにしない)", async () => {
  const resolved = await createSource(okFetch()).search.resolveSpecies("9001-000");
  // learnset は3件だが moveMaster に growl が無いので2件。learnset 自体は3件のまま残す。
  expect(resolved.moves.map((move) => move.id)).toEqual(["example-move-firepunch", "example-move-tackle"]);
  expect(resolved.species.learnset).toHaveLength(3);
});

test("learnset が空なら moves/batch を引かない(ids 省略は 400 invalid_input)", async () => {
  const fetchMock = okFetch({ detail: () => jsonResponse(200, detailWithLearnset(0)) });
  const resolved = await createSource(fetchMock).search.resolveSpecies("9001-000");

  expect(batchIdCalls(fetchMock)).toEqual([]);
  expect(resolved.moves).toEqual([]);
});

describe("learnset が64件を超えても全件を解決する(1回で収まる前提を置かない。ADR-0304 §3 追記)", () => {
  // 境界値: 1件 / ちょうど上限 / 上限+1 / 上限×2 / 上限×2+1。
  test.each([
    [1, 1],
    [MOVES_BATCH_MAX_IDS, 1],
    [MOVES_BATCH_MAX_IDS + 1, 2],
    [MOVES_BATCH_MAX_IDS * 2, 2],
    [MOVES_BATCH_MAX_IDS * 2 + 1, 3],
  ])("learnset %i 件は moves/batch を %i 回に分ける", async (learnsetSize, expectedCalls) => {
    const fetchMock = learnsetFetch(learnsetSize);
    const resolved = await createSource(fetchMock).search.resolveSpecies("9001-000");
    const calls = batchIdCalls(fetchMock);

    expect(calls).toHaveLength(expectedCalls);
    // 1回あたりの ids は契約の範囲(1〜64件)に必ず収まる。
    for (const ids of calls) {
      expect(ids.length).toBeGreaterThanOrEqual(1);
      expect(ids.length).toBeLessThanOrEqual(MOVES_BATCH_MAX_IDS);
    }
    // 分割しても取りこぼさず、learnset の順のまま並ぶ(チャンクの応答を順につなぐ)。
    expect(calls.flat()).toEqual(resolved.species.learnset);
    expect(resolved.moves.map((move) => move.id)).toEqual(resolved.species.learnset);
  });
});

test("分割した moves/batch には1つの AbortSignal をそのまま渡す(種族の解決1回ぶんとして取り消せる)", async () => {
  const fetchMock = learnsetFetch(MOVES_BATCH_MAX_IDS + 1);
  const controller = new AbortController();
  await createSource(fetchMock).search.resolveSpecies("9001-000", controller.signal);

  const signals = fetchMock.mock.calls.map(([, init]) => (init ?? {}).signal);
  expect(signals).toHaveLength(3); // 詳細1回 + moves/batch 2回
  for (const signal of signals) {
    expect(signal).toBe(controller.signal);
  }
});

test("マスタ未投入で 200 [] が返っても失敗しない(技なしとして解決する)", async () => {
  // 契約: moves/batch は searchMoves と異なり 503 ではなく空配列を返す。
  const fetchMock = okFetch({ [PATHS.movesBatch]: () => jsonResponse(200, []) });
  const resolved = await createSource(fetchMock).search.resolveSpecies("9001-000");
  expect(resolved.moves).toEqual([]);
  expect(resolved.species.key).toBe("9001-000");
});

test("技の解決に失敗したら resolveSpecies 全体が失敗する(種族だけ返さない。ADR-0304 A-13)", async () => {
  const error: Schemas["Error"] = { code: "upstream_unavailable", message: "pokedex に届かない" };
  const fetchMock = okFetch({ [PATHS.movesBatch]: () => jsonResponse(503, error) });
  await expect(createSource(fetchMock).search.resolveSpecies("9001-000")).rejects.toThrow(Error);
});

test("分割した moves/batch の1回でも失敗したら resolveSpecies 全体が失敗する", async () => {
  const master = moveMasterFor(MOVES_BATCH_MAX_IDS + 1);
  let calls = 0;
  const fetchMock = okFetch({
    detail: () => jsonResponse(200, detailWithLearnset(MOVES_BATCH_MAX_IDS + 1)),
    [PATHS.movesBatch]: (url) => {
      calls += 1;
      // 2回目(最後のチャンク)だけ失敗させる。1回目の成功で握りつぶさないこと。
      return calls === 1 ? movesBatchRoute(master)(url) : jsonResponse(503, { code: "x", message: "y" });
    },
  });
  await expect(createSource(fetchMock).search.resolveSpecies("9001-000")).rejects.toThrow(Error);
});

test("moves/batch にも端末 ID・セッション ID を付ける", async () => {
  const fetchMock = okFetch();
  await createSource(fetchMock).search.resolveSpecies("9001-000");
  const { init } = callTo(fetchMock, PATHS.movesBatch);
  expect(headerOf(init, "X-Device-Id")).toBe(ids.deviceId);
  expect(headerOf(init, "X-Session-Id")).toBe(ids.sessionId);
});
