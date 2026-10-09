// ADR-0178 §6: 公開 API の Move の target・mechanisms・flags を、オフライン(WASM)計算に渡す技にそのまま写す。
// 確かめること:
//   - getMovesByIds の応答の target・mechanisms・flags が MasterSpeciesResolution.moves の技に載る(値・順のまま)
//   - 空配列(既知のフラグなし・通常の技)は空配列のまま写す(省略にしない。フラグの「不明」と区別する)
//   - 応答が返さないキー(古いサーバー・取り込み前)は作らない(WASM がキーなし = 不明として扱う)
// 背景: mapMove は id・名前・タイプ・分類・威力・優先度だけを写していたため、オフラインではダブルの全体技の補正と
// 技の機構の未対応の印が黙って落ちていた(ADR-0223 §5 の Web の追従が残っていた)。

import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { createOnlineMasterSource } from "./onlineSource";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const detail: Schemas["SpeciesDetail"] = {
  key: "9001-000",
  dexNo: 9001,
  form: 0,
  nameJa: "テストほのお",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 90, spa: 110, spd: 90, spe: 95 },
  abilities: [{ id: "exampleabilitynone", nameJa: "テストなし" }],
  learnset: ["examplefullmove", "exampleplainmove", "exampleoldmove"],
};

const fullMove: Schemas["Move"] = {
  id: "examplefullmove",
  nameJa: "テストぜんぶ",
  type: "normal",
  category: "special",
  power: 90,
  priority: 0,
  target: "spread",
  mechanisms: ["multi_hit", "variable_power"],
  flags: ["secondary", "sound"],
};

const plainMove: Schemas["Move"] = {
  id: "exampleplainmove",
  nameJa: "テストふつう",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
  target: "single",
  mechanisms: [],
  flags: [],
};

// 古いサーバー・取り込み前: target・mechanisms・flags を返さない。
const oldMove: Schemas["Move"] = {
  id: "exampleoldmove",
  nameJa: "テストむかし",
  type: "water",
  category: "special",
  power: 70,
  priority: 0,
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function fetchFor(moves: Schemas["Move"][]): typeof fetch {
  return vi.fn((input: RequestInfo | URL) => {
    const url = new URL(
      typeof input === "string" ? input : input instanceof URL ? input.href : input.url,
      "http://localhost/",
    );
    if (url.pathname === "/api/pokedex/moves/batch") {
      return Promise.resolve(jsonResponse(moves));
    }
    if (url.pathname.startsWith("/api/pokedex/species/")) {
      return Promise.resolve(jsonResponse(detail));
    }
    return Promise.resolve(jsonResponse([]));
  });
}

async function resolvedMoves() {
  const source = createOnlineMasterSource({
    baseUrl: "http://pokedex.test/",
    fetch: fetchFor([fullMove, plainMove, oldMove]),
    ids,
  });
  const resolved = await source.search.resolveSpecies("9001-000");
  return new Map(resolved.moves.map((m) => [m.id, m]));
}

test("target・mechanisms・flags を応答のまま技に写す", async () => {
  const moves = await resolvedMoves();
  const full = moves.get("examplefullmove");
  expect(full?.target).toBe("spread");
  expect(full?.mechanisms).toEqual(["multi_hit", "variable_power"]);
  expect(full?.flags).toEqual(["secondary", "sound"]);
});

test("空配列は空配列のまま写す(既知のフラグなし・通常の技。省略にしない)", async () => {
  const moves = await resolvedMoves();
  const plain = moves.get("exampleplainmove");
  expect(plain?.target).toBe("single");
  expect(plain?.mechanisms).toEqual([]);
  expect(plain?.flags).toEqual([]);
  expect(plain !== undefined && "flags" in plain).toBe(true);
});

test("応答が返さないキーは作らない(WASM が不明として扱う)", async () => {
  const moves = await resolvedMoves();
  const old = moves.get("exampleoldmove");
  expect(old).toBeDefined();
  expect(old !== undefined && "target" in old).toBe(false);
  expect(old !== undefined && "mechanisms" in old).toBe(false);
  expect(old !== undefined && "flags" in old).toBe(false);
});
