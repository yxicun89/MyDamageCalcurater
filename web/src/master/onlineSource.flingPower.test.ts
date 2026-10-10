// ADR-0144 §6: 公開 API の Item.flingPower(なげつけるの威力)を、オフライン(WASM)計算に渡す持ち物にそのまま写す(表示は変えない)。
// 確かめること:
//   - searchItems の応答の flingPower が MasterItem に載る(値のまま)
//   - 応答が返さないキー(投げられない持ち物・古いサーバー・取り込み前)は作らない(WASM がキーなし = 不明として印を残す)
// 背景: WASM の持ち物 DTO は未知のキーを拒否するので、この写しは WASM の境界(flingPower)と同じ変更で入れる。

import { expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { createOnlineMasterSource } from "./onlineSource";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function sourceFor(items: Schemas["Item"][]) {
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
    return Promise.resolve(new Response("{}", { status: 404 }));
  });
  return createOnlineMasterSource({ baseUrl: "http://pokedex.test/", fetch: fetchImpl, ids });
}

test("持ち物の flingPower を応答のまま写す", async () => {
  const master = await sourceFor([
    { id: "exampleitemheavy", nameJa: "テストおもいたま", roles: [], isMegaStone: false, flingPower: 130 },
  ]).load();
  expect(master.items).toEqual([
    {
      id: "exampleitemheavy",
      nameJa: "テストおもいたま",
      effect: null,
      roles: [],
      isMegaStone: false,
      flingPower: 130,
    },
  ]);
});

test("flingPower を返さない応答(投げられない・古いサーバー)は、キーを作らない", async () => {
  const master = await sourceFor([{ id: "exampleitemplain", nameJa: "テストふつうだま" }]).load();
  const [item] = master.items;
  expect(item).toBeDefined();
  expect(item).not.toHaveProperty("flingPower");
});
