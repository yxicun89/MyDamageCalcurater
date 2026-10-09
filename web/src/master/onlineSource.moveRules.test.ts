// ADR-0143 §6: 公開 API の Move の mechanismParams・rule と SpeciesDetail の weightHg を、オフライン(WASM)計算に渡す技・種族に
// そのまま写す(表示は変えない)。
// 確かめること:
//   - getMovesByIds の応答の mechanismParams・rule が MasterSpeciesResolution.moves の技に載る(値のまま。rule のキーの大文字小文字も変えない)
//   - getSpecies の応答の weightHg が MasterSpeciesResolution.species に載る
//   - 応答が返さないキー(定義の無い技・古いサーバー・取り込み前)は作らない(WASM がキーなし = 定義なし / 重さ不明として扱う)
// 背景: 段階1(ADR-0142)では公開 API が mechanismParams を返さず、オフラインでは多段・固定ダメージ等の印が残っていた。

import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { createOnlineMasterSource } from "./onlineSource";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

function detail(weightHg: number | undefined): Schemas["SpeciesDetail"] {
  return {
    key: "9001-000",
    dexNo: 9001,
    form: 0,
    nameJa: "テストほのお",
    types: ["fire"],
    baseStats: { hp: 80, atk: 100, def: 90, spa: 110, spd: 90, spe: 95 },
    abilities: [{ id: "exampleabilitynone", nameJa: "テストなし" }],
    learnset: ["exampleweightmove", "examplemultimove", "exampleplainmove"],
    ...(weightHg === undefined ? {} : { weightHg }),
  };
}

const weightMove: Schemas["Move"] = {
  id: "exampleweightmove",
  nameJa: "テストおもさ",
  type: "fighting",
  category: "physical",
  power: 0,
  priority: 0,
  mechanisms: ["move_specific", "variable_power"],
  rule: { PowerFormula: "target_weight", MoveSpecificResolved: true },
};

const multiMove: Schemas["Move"] = {
  id: "examplemultimove",
  nameJa: "テストれんぞく",
  type: "ice",
  category: "physical",
  power: 20,
  priority: 0,
  mechanisms: ["multi_hit", "variable_power"],
  mechanismParams: {
    multiHit: { min: 3, max: 3 },
    fixedDamage: null,
    ohko: null,
    offenseStat: null,
    offensePokemon: null,
    defenseStat: null,
  },
  rule: { PowerFormula: "hit_index" },
};

// 定義・中身の無い技(古いサーバーも同じ形)。
const plainMove: Schemas["Move"] = {
  id: "exampleplainmove",
  nameJa: "テストふつう",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
  mechanisms: [],
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function fetchFor(species: Schemas["SpeciesDetail"]): typeof fetch {
  return vi.fn((input: RequestInfo | URL) => {
    const url = new URL(
      typeof input === "string" ? input : input instanceof URL ? input.href : input.url,
      "http://localhost/",
    );
    if (url.pathname === "/api/pokedex/moves/batch") {
      return Promise.resolve(jsonResponse([weightMove, multiMove, plainMove]));
    }
    if (url.pathname.startsWith("/api/pokedex/species/")) {
      return Promise.resolve(jsonResponse(species));
    }
    return Promise.resolve(jsonResponse([]));
  });
}

async function resolve(weightHg: number | undefined) {
  const source = createOnlineMasterSource({
    baseUrl: "http://pokedex.test/",
    fetch: fetchFor(detail(weightHg)),
    ids,
  });
  return source.search.resolveSpecies("9001-000");
}

test("mechanismParams・rule を応答のまま技に写す", async () => {
  const resolved = await resolve(905);
  const moves = new Map(resolved.moves.map((m) => [m.id, m]));
  expect(moves.get("exampleweightmove")?.rule).toEqual({
    PowerFormula: "target_weight",
    MoveSpecificResolved: true,
  });
  expect(moves.get("examplemultimove")?.mechanismParams).toEqual(multiMove.mechanismParams);
  expect(moves.get("examplemultimove")?.rule).toEqual({ PowerFormula: "hit_index" });
});

test("応答が返さない rule・mechanismParams は作らない", async () => {
  const resolved = await resolve(905);
  const plain = resolved.moves.find((m) => m.id === "exampleplainmove");
  expect(plain).toBeDefined();
  expect(plain !== undefined && "rule" in plain).toBe(false);
  expect(plain !== undefined && "mechanismParams" in plain).toBe(false);
});

test("種族の weightHg を写す・無ければ作らない", async () => {
  expect((await resolve(905)).species.weightHg).toBe(905);
  const unknown = (await resolve(undefined)).species;
  expect("weightHg" in unknown).toBe(false);
});
