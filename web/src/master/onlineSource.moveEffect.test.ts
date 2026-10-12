// CB-6(ADR-0253): 公開 API の Move.effect(PascalCase。確率 100% のランク変化だけ)を、WASM 境界の camelCase に直して技へ載せる。
// 確かめること:
//   - {Chance,Target,Stages} → {chance,target,stages}(stages のキーは変えない)
//   - 応答に effect が無い技(古いサーバー・確率 100% 未満・効果なし)は effect のキーを作らない = ランク変化なし
//   - 形が合わない effect は載せない(落とす)
//   - 変換した技が IndexedDB のキャッシュを往復しても残り、スキーマ版 3 の記録(effect なし)は破棄される

import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { appText } from "../i18n/ja";
import { createMemoryMasterCacheStore } from "../test/memoryMasterCacheStore";
import { createCachedOfflineMasterSource, createCachingMasterSource } from "./cache/cachedSources";
import { MASTER_CACHE_SCHEMA_VERSION } from "./cache/types";
import { exampleNatures } from "./example/natures";
import { createOnlineMasterSource, fromPublicMoveEffect } from "./onlineSource";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const danceMove: Schemas["Move"] = {
  id: "exampledance",
  nameJa: "テストまい",
  type: "dragon",
  category: "status",
  power: 0,
  priority: 0,
  effect: { Chance: 100, Target: "self", Stages: { atk: 1, spe: 1 } },
};

const plainMove: Schemas["Move"] = {
  id: "exampleplain",
  nameJa: "テストふつう",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
};

const detail: Schemas["SpeciesDetail"] = {
  key: "9001-000",
  dexNo: 9001,
  form: 0,
  nameJa: "テストほのお",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 90, spa: 110, spd: 90, spe: 95 },
  abilities: [{ id: "exampleabilitynone", nameJa: "テストなし" }],
  learnset: ["exampledance", "exampleplain"],
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function onlineSource() {
  const fetchMock: typeof fetch = vi.fn((input: RequestInfo | URL) => {
    const url = new URL(
      typeof input === "string" ? input : input instanceof URL ? input.href : input.url,
      "http://localhost/",
    );
    if (url.pathname === "/api/pokedex/moves/batch") {
      return Promise.resolve(jsonResponse([danceMove, plainMove]));
    }
    if (url.pathname.startsWith("/api/pokedex/species/")) {
      return Promise.resolve(jsonResponse(detail));
    }
    return Promise.resolve(jsonResponse([]));
  });
  return createOnlineMasterSource({ baseUrl: "http://pokedex.test/", fetch: fetchMock, ids });
}

test("effect を PascalCase から camelCase に直して技に載せる", async () => {
  const resolved = await onlineSource().search.resolveSpecies("9001-000");
  const dance = resolved.moves.find((m) => m.id === "exampledance");
  expect(dance?.effect).toEqual({ chance: 100, target: "self", stages: { atk: 1, spe: 1 } });
});

test("応答に effect が無い技は effect のキーを作らない(ランク変化なし)", async () => {
  const resolved = await onlineSource().search.resolveSpecies("9001-000");
  const plain = resolved.moves.find((m) => m.id === "exampleplain");
  expect(plain).toBeDefined();
  expect(plain !== undefined && "effect" in plain).toBe(false);
});

test("fromPublicMoveEffect: 相手側の変化・省略・null・形の合わないものを扱う", () => {
  expect(fromPublicMoveEffect({ Chance: 100, Target: "target", Stages: { spe: -1 } })).toEqual({
    chance: 100,
    target: "target",
    stages: { spe: -1 },
  });
  expect(fromPublicMoveEffect(undefined)).toBeUndefined();
  expect(fromPublicMoveEffect(null)).toBeUndefined();
  expect(fromPublicMoveEffect({})).toBeUndefined();
  expect(fromPublicMoveEffect({ Chance: 100, Target: "ally", Stages: {} })).toBeUndefined();
  expect(fromPublicMoveEffect({ Chance: "100", Target: "self", Stages: {} })).toBeUndefined();
  expect(fromPublicMoveEffect({ Chance: 100, Target: "self", Stages: null })).toBeUndefined();
});

test("キャッシュを往復しても effect が残る(オフラインでも連続技の変化技が使える)", async () => {
  const store = createMemoryMasterCacheStore();
  const online = createCachingMasterSource({ source: onlineSource(), store });
  await online.load();
  await online.search.resolveSpecies("9001-000");

  const resolved = await createCachedOfflineMasterSource({ store }).search.resolveSpecies("9001-000");
  expect(resolved.moves.find((m) => m.id === "exampledance")?.effect).toEqual({
    chance: 100,
    target: "self",
    stages: { atk: 1, spe: 1 },
  });
  expect("effect" in (resolved.moves.find((m) => m.id === "exampleplain") ?? {})).toBe(false);
});

test("スキーマ版 3 の記録(技に effect が無い)は破棄される", async () => {
  expect(MASTER_CACHE_SCHEMA_VERSION).toBeGreaterThanOrEqual(4);
  const store = createMemoryMasterCacheStore();
  store.poke({
    schemaVersion: 3,
    items: [],
    natures: exampleNatures,
    species: {},
    abilities: {},
    moves: {},
  });
  await expect(createCachedOfflineMasterSource({ store }).load()).rejects.toThrow(
    appText.masterCacheEmptyError,
  );
});
