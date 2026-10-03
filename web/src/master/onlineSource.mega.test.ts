// issue #515・ADR-0320: 公開 API の SpeciesDetail の isMega・requiredItemId を MasterSpecies に写す。
// 確かめること:
//   - メガ種族: isMega=true・requiredItemId=メガストーンの ID がそのまま MasterSpecies に載る
//   - メガでない種族の応答(isMega=false・requiredItemId=null)はそのまま
//   - 項目を返さない応答(pokedex-svc が対応する前・古い応答)は省略のまま(isMegaSpecies が false と読む)で壊れない
//   - 持ち物(load)にメガストーンも含まれる(オフラインのキャッシュに入る。画面が固定の名前を引く)
//   - 持ち物の写像は id・nameJa だけ(効果なし)のまま

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
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function fetchFor(detail: unknown, items: Schemas["Item"][] = []): typeof fetch {
  return vi.fn((input: RequestInfo | URL) => {
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
  }) as unknown as typeof fetch;
}

function sourceFor(detail: unknown, items?: Schemas["Item"][]) {
  return createOnlineMasterSource({ baseUrl: "http://pokedex.test/", fetch: fetchFor(detail, items), ids });
}

test("メガ種族の応答は isMega・requiredItemId を MasterSpecies に写す", async () => {
  const resolved = await sourceFor(megaDetail).search.resolveSpecies("9101-001");
  expect(resolved.species.isMega).toBe(true);
  expect(resolved.species.requiredItemId).toBe("examplemegastonefire");
});

test("メガでない種族の応答(isMega=false・requiredItemId=null)はそのまま写す", async () => {
  const resolved = await sourceFor({
    ...megaDetail,
    key: "9001-000",
    form: 0,
    isMega: false,
    requiredItemId: null,
  }).search.resolveSpecies("9001-000");
  expect(resolved.species.isMega).toBe(false);
  expect(resolved.species.requiredItemId).toBeNull();
});

test("項目を返さない応答でも解決でき、メガとしては扱われない(省略のまま)", async () => {
  const { isMega: _isMega, requiredItemId: _requiredItemId, ...withoutMega } = megaDetail;
  const resolved = await sourceFor(withoutMega).search.resolveSpecies("9101-001");
  expect(resolved.species.isMega).toBeUndefined();
  expect(resolved.species.requiredItemId).toBeUndefined();
});

test("持ち物の読み込みにメガストーンも入る(id・nameJa だけ写し、効果は null)", async () => {
  const master = await sourceFor(megaDetail, [
    { id: "exampleitemdef", nameJa: "テストぼうぎょだま" },
    { id: "examplemegastonefire", nameJa: "テストほのおナイト" },
  ]).load();
  expect(master.items).toEqual([
    { id: "exampleitemdef", nameJa: "テストぼうぎょだま", effect: null },
    { id: "examplemegastonefire", nameJa: "テストほのおナイト", effect: null },
  ]);
});
