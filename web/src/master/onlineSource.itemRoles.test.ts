// ADR-0326(ADR-0175 §3・§4): 公開 API の Item.roles・Item.isMegaStone と SpeciesDetail.baseSpeciesKey・baseSpeciesNameJa を
// MasterItem・MasterSpecies に写す。
// 確かめること:
//   - roles・isMegaStone は応答の値をそのまま写す(空配列・false も保つ。並びも保つ)
//   - 項目を返さない応答(古いサーバー)は省略のまま(キーを作らない。「分からない」= 絞らない、と読むため)
//   - 種族の詳細の baseSpeciesKey・baseSpeciesNameJa をそのまま写す(null も保つ)。省略は省略のまま
//   - 効果(effect)の写像は従来どおり

import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { createOnlineMasterSource } from "./onlineSource";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const megaDetail: Schemas["SpeciesDetail"] = {
  key: "9101-001",
  dexNo: 9101,
  form: 1,
  nameJa: "メガテストほのお",
  types: ["fire"],
  baseStats: { hp: 80, atk: 130, def: 90, spa: 110, spd: 90, spe: 95 },
  abilities: [{ id: "exampleabilitynone", nameJa: "テストなし" }],
  learnset: [],
  isMega: true,
  requiredItemId: "examplemegastonefire",
  baseSpeciesKey: "9001-000",
  baseSpeciesNameJa: "テストほのお",
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function sourceFor(detail: unknown, items: Schemas["Item"][] = []) {
  const fetchImpl = vi.fn((input: RequestInfo | URL) => {
    const url = new URL(
      typeof input === "string" ? input : input instanceof URL ? input.href : input.url,
      "http://localhost/",
    );
    if (url.pathname === "/api/pokedex/items") {
      return Promise.resolve(jsonResponse(items));
    }
    if (url.pathname === "/api/pokedex/natures") {
      return Promise.resolve(jsonResponse([]));
    }
    if (url.pathname.startsWith("/api/pokedex/species/")) {
      return Promise.resolve(jsonResponse(detail));
    }
    return Promise.resolve(new Response("{}", { status: 404 }));
  });
  return createOnlineMasterSource({ baseUrl: "http://pokedex.test/", fetch: fetchImpl, ids });
}

test("持ち物の roles・isMegaStone を応答のまま写す(空配列・false・並びも保つ)", async () => {
  const master = await sourceFor(megaDetail, [
    {
      id: "exampleitemboth",
      nameJa: "テストりょうほうだま",
      roles: ["attacker", "defender"],
      isMegaStone: false,
    },
    { id: "exampleitemheal", nameJa: "テストかいふくのみ", roles: [], isMegaStone: false },
    { id: "examplemegastonefire", nameJa: "Examplite Z", roles: [], isMegaStone: true },
  ]).load();

  expect(master.items).toEqual([
    {
      id: "exampleitemboth",
      nameJa: "テストりょうほうだま",
      effect: null,
      roles: ["attacker", "defender"],
      isMegaStone: false,
    },
    { id: "exampleitemheal", nameJa: "テストかいふくのみ", effect: null, roles: [], isMegaStone: false },
    { id: "examplemegastonefire", nameJa: "Examplite Z", effect: null, roles: [], isMegaStone: true },
  ]);
});

test("roles・isMegaStone を返さない応答(古いサーバー)は、キーを作らない(役割が分からない = 絞らない)", async () => {
  const master = await sourceFor(megaDetail, [{ id: "exampleitemdef", nameJa: "テストぼうぎょだま" }]).load();
  const [item] = master.items;
  expect(item).toBeDefined();
  expect(item).not.toHaveProperty("roles");
  expect(item).not.toHaveProperty("isMegaStone");
});

test("種族の詳細の baseSpeciesKey・baseSpeciesNameJa をそのまま写す", async () => {
  const resolved = await sourceFor(megaDetail).search.resolveSpecies("9101-001");
  expect(resolved.species.baseSpeciesKey).toBe("9001-000");
  expect(resolved.species.baseSpeciesNameJa).toBe("テストほのお");
});

test("メガでない種族(null)も null のまま写す", async () => {
  const resolved = await sourceFor({
    ...megaDetail,
    key: "9001-000",
    form: 0,
    isMega: false,
    requiredItemId: null,
    baseSpeciesKey: null,
    baseSpeciesNameJa: null,
  }).search.resolveSpecies("9001-000");
  expect(resolved.species.baseSpeciesKey).toBeNull();
  expect(resolved.species.baseSpeciesNameJa).toBeNull();
});

test("項目を返さない応答(古いサーバー)は省略のまま(固定中の表示は「メガストーン」になる)", async () => {
  const withoutBase: Schemas["SpeciesDetail"] = { ...megaDetail };
  delete withoutBase.baseSpeciesKey;
  delete withoutBase.baseSpeciesNameJa;
  const resolved = await sourceFor(withoutBase).search.resolveSpecies("9101-001");
  expect(resolved.species).not.toHaveProperty("baseSpeciesKey");
  expect(resolved.species).not.toHaveProperty("baseSpeciesNameJa");
});
