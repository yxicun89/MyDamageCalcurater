// issue #515・ADR-0320: E2E 専用の pokedex フィクスチャが、架空のメガ種族・メガストーン(src/test/megaMaster.ts の
// withMegaFixture)を公開 API の形で返す。契約の「ちょうど」は pokedexFixture.contract.test.ts が見る。
// ここでは、メガの項目の値と、検索・持ち物にメガ種族とメガストーンが出ることだけを確かめる。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../../src/api/openapi.gen";
import { exampleMasterSource } from "../../src/master/exampleSource";
import type { MasterData } from "../../src/master/types";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../../src/test/megaMaster";
import { handlePokedexRequest, type FixtureRequest } from "./pokedexFixture";

type Schemas = components["schemas"];

const HEADERS = {
  "x-device-id": "11111111-1111-4111-8111-111111111111",
  "x-session-id": "22222222-2222-4222-a222-222222222222",
} as const;

let master: MasterData;

beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
});

function get(path: string, query = ""): { status: number; body: unknown } {
  const request: FixtureRequest = {
    method: "GET",
    path,
    query: new URLSearchParams(query),
    headers: HEADERS,
  };
  return handlePokedexRequest(master, request);
}

describe("メガ種族のフィクスチャ", () => {
  test("メガ種族の詳細は isMega=true・requiredItemId=メガストーンの ID", () => {
    const response = get(`/api/pokedex/species/${MEGA_FIRE.key}`);
    expect(response.status).toBe(200);
    const body = response.body as Schemas["SpeciesDetail"];
    expect(body.isMega).toBe(true);
    expect(body.requiredItemId).toBe(MEGA_FIRE_STONE.id);
  });

  test("基本種の詳細は isMega=false・requiredItemId=null", () => {
    const body = get("/api/pokedex/species/9001-000").body as Schemas["SpeciesDetail"];
    expect(body.isMega).toBe(false);
    expect(body.requiredItemId).toBeNull();
  });

  test("名前の検索でメガ種族が引ける。検索結果(SpeciesSummary)にメガの項目は混ざらない", () => {
    const found = get("/api/pokedex/species", "q=メガテストほのお").body as Schemas["SpeciesSummary"][];
    expect(found.map((species) => species.key)).toEqual([MEGA_FIRE.key]);
    expect(Object.keys(found[0] ?? {}).sort()).toEqual(["dexNo", "form", "key", "nameJa", "types"]);
  });

  test("持ち物の一覧にメガストーンが含まれる", () => {
    const items = get("/api/pokedex/items").body as Schemas["Item"][];
    expect(items.find((item) => item.id === MEGA_FIRE_STONE.id)?.nameJa).toBe(MEGA_FIRE_STONE.nameJa);
  });
});
