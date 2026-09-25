// PR2(ADR-0307): フィクスチャ(pokedexFixture.ts)と、本番の読み取り側(src/master/onlineSource.ts の
// createOnlineMasterSource)が実際に噛み合うことを確かめる。契約テスト(pokedexFixture.contract.test.ts)は
// 「契約のスキーマどおりか」を見るが、こちらは「Web がその応答をちゃんと読めるか」(load・searchSpecies・
// resolveSpecies が成功し、期待した中身になるか)を見る。
//
// ここが緑なら、`web-e2e-online` の画面がマスタの読み込みに失敗する(= ADR-0301 §5 追記で残した不整合)
// ことは無くなる。HTTP は挟まない(挟むのは Playwright の実行そのもの)。fetch だけを
// handlePokedexRequest に差し替える。

import { beforeAll, describe, expect, test } from "vitest";
import {
  ITEMS_FETCH_LIMIT,
  ONLINE_MASTER_CAPABILITIES,
  createOnlineMasterSource,
} from "../../src/master/onlineSource";
import { exampleMasterSource } from "../../src/master/exampleSource";
import type { MasterData, MasterSpecies, SearchableMasterSource } from "../../src/master/types";
import { handlePokedexRequest, type FixtureRequest } from "./pokedexFixture";

const IDS = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-8222-222222222222",
} as const;

/** 相対 URL を解決するためだけの基点(外に出ない)。 */
const ORIGIN = "http://pokedex-fixture.invalid";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

/** handlePokedexRequest を fetch の形に包む(ネットワークは使わない)。 */
function fixtureFetch(): typeof fetch {
  return (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const href = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(href, ORIGIN);
    const rawHeaders = (init?.headers ?? {}) as Record<string, string>;
    const headers: Record<string, string | undefined> = {};
    for (const [name, value] of Object.entries(rawHeaders)) {
      headers[name.toLowerCase()] = value;
    }
    const request: FixtureRequest = {
      method: init?.method?.toUpperCase() ?? "GET",
      path: url.pathname,
      query: url.searchParams,
      headers,
    };
    const { status, body } = handlePokedexRequest(master, request);
    const response = {
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(body),
    };
    return Promise.resolve(response as unknown as Response);
  };
}

function onlineSource(): SearchableMasterSource {
  return createOnlineMasterSource({ baseUrl: "/", fetch: fixtureFetch(), ids: IDS });
}

function speciesByName(nameJa: string): MasterSpecies {
  const species = master.species.find((candidate) => candidate.nameJa === nameJa);
  if (species === undefined) {
    throw new Error(`例データに ${nameJa} が無い`);
  }
  return species;
}

describe("createOnlineMasterSource がフィクスチャから読める(ADR-0307)", () => {
  test("load() が成功し、持ち物・性格・相性表がそろう(画面が「読み込みに失敗しました」で止まらない)", async () => {
    const loaded = await onlineSource().load();
    expect(loaded.items.map((item) => item.id).sort()).toEqual(master.items.map((item) => item.id).sort());
    expect(loaded.natures.map((nature) => nature.id).sort()).toEqual(
      master.natures.map((nature) => nature.id).sort(),
    );
    expect(loaded.typeChart.types.length).toBeGreaterThan(0);
    // 公開 API に効果データは無いので、持ち物の effect は null(ADR-0304 A-1)。
    expect(loaded.items.every((item) => item.effect === null)).toBe(true);
    expect(loaded.capabilities).toEqual(ONLINE_MASTER_CAPABILITIES);
    expect(loaded.species).toEqual([]);
    expect(loaded.moves).toEqual([]);
  });

  test("持ち物が limit ちょうど返らない(load() の打ち切り検出に引っかからない)", async () => {
    const loaded = await onlineSource().load();
    expect(loaded.items.length).toBeLessThan(ITEMS_FETCH_LIMIT);
  });

  test("searchSpecies が日本語名の前方一致で候補を返す", async () => {
    const found = await onlineSource().search.searchSpecies("テストほのお");
    expect(found.map((summary) => summary.key)).toEqual([speciesByName("テストほのお").key]);
  });

  test("resolveSpecies が種族・特性・技(learnset の解決済み)を返す", async () => {
    const fire = speciesByName("テストほのお");
    const resolution = await onlineSource().search.resolveSpecies(fire.key);
    expect(resolution.species.baseStats).toEqual(fire.baseStats);
    expect(resolution.species.learnset).toEqual(fire.learnset);
    expect(resolution.abilities.map((ability) => ability.id)).toEqual(fire.abilities);
    // 公開 API に効果データは無い(ADR-0304 A-1)。
    expect(resolution.abilities.every((ability) => ability.effect === null)).toBe(true);
    // P4-17: learnset の順のまま技の実体に解決できる(ADR-0304 A-13)。
    expect(resolution.moves.map((move) => move.id)).toEqual(fire.learnset);
    const expected = fire.learnset.map((id) => master.moves.find((move) => move.id === id)?.nameJa);
    expect(resolution.moves.map((move) => move.nameJa)).toEqual(expected);
  });

  test("マスタに無い種族の resolveSpecies は 404 not_found で reject する(空で握りつぶさない)", async () => {
    // onlineSource の getJson は本文が Error({code,message})なら `code: message` を投げる。
    await expect(onlineSource().search.resolveSpecies("9999-000")).rejects.toThrow(/^not_found: /);
  });
});
