// SP3: speed API のクライアント createSpeedClient(ADR-0604 §3)。balanceClient.test.ts と同じ形。
// services/speed/api/openapi.yaml を正とし、リクエスト本文・応答は openapi-typescript の生成型(speed.gen.ts)で
// 書く(契約とのずれを typecheck で検出する)。fetch は fake。
// 確かめること:
//   - pokemon / table は GET、position は POST で、`${baseUrl}api/speed/v1/pokemon|table|position` を呼ぶ
//   - ヘッダーは X-Device-Id・X-Session-Id(計算・タイプバランスと同じ ID。CLAUDE.md 技術規約)。
//     本文を送る position だけ Content-Type: application/json も付ける
//   - table の presets は省略なら付けず、渡したらカンマ区切り1つのクエリにする(ADR-0601 §4)
//   - position の本文は PositionRequest のまま送る(mode ごとに要る項目だけ。余分な項目は契約上 400)
//   - 成功は {ok: true, value}(応答をそのまま運ぶ。Web で素早さを計算し直さない。ADR-0604 §3)
//   - HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ
//   - 通信できない・応答が JSON でない・エラー本文の形が不正なら speed_unavailable(Web 側のコード)

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "./speed.gen";
import { createSpeedClient } from "./speedClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://speed.test/";

const pokemonListResponse: Schemas["PokemonListResponse"] = {
  regulationId: "example",
  pokemon: [
    { pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100 },
    { pokemonId: "9002-000", nameJa: "テストカソウギョ", types: ["water"], baseSpeed: 80 },
  ],
};

const tableResponse: Schemas["TableResponse"] = {
  regulationId: "example",
  presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
  tiers: [
    {
      speed: 252,
      entries: [
        {
          pokemonId: "9001-000",
          nameJa: "テストカソウドリ",
          types: ["fire", "flying"],
          baseSpeed: 100,
          preset: "max-scarf",
        },
      ],
    },
    {
      speed: 168,
      entries: [
        {
          pokemonId: "9001-000",
          nameJa: "テストカソウドリ",
          types: ["fire", "flying"],
          baseSpeed: 100,
          preset: "max",
        },
        {
          pokemonId: "9002-000",
          nameJa: "テストカソウギョ",
          types: ["water"],
          baseSpeed: 80,
          preset: "max-plus1",
        },
      ],
    },
  ],
};

const positionResponse: Schemas["PositionResponse"] = {
  speed: 168,
  pokemon: { pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100 },
  faster: 1,
  slower: 0,
  tie: [
    {
      pokemonId: "9002-000",
      nameJa: "テストカソウギョ",
      types: ["water"],
      baseSpeed: 80,
      preset: "max-plus1",
    },
  ],
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function fakeFetch(response: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    response instanceof Error ? Promise.reject(response) : Promise.resolve(response),
  );
}

/** fetch の n 回目の呼び出しの URL・RequestInit を取り出す。 */
function callAt(fetchMock: ReturnType<typeof fakeFetch>, index = 0): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[index];
  if (call === undefined) {
    throw new Error(`fetch の ${String(index)} 回目の呼び出しが無い`);
  }
  const [url, init] = call;
  if (typeof url !== "string" || init === undefined) {
    throw new Error("fetch は (文字列の URL, RequestInit) で呼ぶ");
  }
  return { url, init };
}

function headersOf(init: RequestInit): Record<string, string> {
  return Object.fromEntries(new Headers(init.headers).entries());
}

function bodyOf(init: RequestInit): unknown {
  if (typeof init.body !== "string") {
    throw new Error("本文は JSON 文字列");
  }
  return JSON.parse(init.body);
}

describe("pokemon(使用可能なポケモンの一覧)", () => {
  test("pokemon のパスに、端末 ID・セッション ID 付きで GET する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, pokemonListResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.pokemon();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/pokemon");
    expect(init.method).toBe("GET");
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    // 応答はそのまま運ぶ(並べ替え・整形をしない)。
    expect(result).toEqual({ ok: true, value: pokemonListResponse });
  });

  test("基点 URL が同じオリジン(/)なら /api/speed/... を呼ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, pokemonListResponse));
    const client = createSpeedClient({ baseUrl: "/", fetch: fetchMock, ids });
    await client.pokemon();
    expect(callAt(fetchMock).url).toBe("/api/speed/v1/pokemon");
  });

  test("GET なので本文を送らない", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, pokemonListResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    await client.pokemon();
    expect(callAt(fetchMock).init.body).toBeUndefined();
  });
});

describe("table(素早さの表)", () => {
  test("presets を省くとクエリを付けない(全6行はサーバーの既定に任せる)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, tableResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.table();

    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/table");
    expect(init.method).toBe("GET");
    expect(headersOf(init)).toMatchObject({
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    // 段・並び順・同速のまとめは応答のまま運ぶ(ADR-0601 §3)。
    expect(result).toEqual({ ok: true, value: tableResponse });
  });

  test("presets を渡すと、カンマ区切りのクエリ1つ(style: form・explode: false)にする", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, tableResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.table(["max", "max-scarf"]);

    const { url } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/table?presets=max%2Cmax-scarf");
  });

  test("presets の並びは渡されたまま送る(Web で並べ替えない。順序は結果に影響しない)", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, tableResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.table(["max-scarf", "uninvested"]);

    expect(callAt(fetchMock).url).toBe("http://speed.test/api/speed/v1/table?presets=max-scarf%2Cuninvested");
  });

  test.each([
    [400, "invalid_request", "unknown preset: max-plus3"],
    [503, "master_unavailable", "pokemon read model is not configured"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    const result = await client.table();
    expect(result).toEqual({ ok: false, error: { code, message } });
  });
});

describe("position(自分のポケモンの位置)", () => {
  test("position のパスに、端末 ID・セッション ID 付きで PositionRequest を POST する", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, positionResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const request: Schemas["PositionRequest"] = {
      mode: "preset",
      pokemonId: "9001-000",
      preset: "max",
      scarf: false,
    };

    const result = await client.position(request);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/position");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(bodyOf(init)).toEqual(request);
    // 実数値・速い/遅い件数・同速の一覧は応答のまま運ぶ(ADR-0602 §3)。
    expect(result).toEqual({ ok: true, value: positionResponse });
  });

  test.each([
    [
      "custom",
      { mode: "custom", pokemonId: "9001-000", sp: 20, nature: "plus", rank: 1, scarf: true },
    ] as const,
    ["raw(ポケモンなし)", { mode: "raw", value: 300 }] as const,
    ["raw(表示用のポケモンあり)", { mode: "raw", value: 300, pokemonId: "9001-000" }] as const,
  ])("%s の本文をそのまま送る(mode に要らない項目を足さない)", async (_name, request) => {
    const fetchMock = fakeFetch(jsonResponse(200, positionResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    await client.position(request);

    expect(bodyOf(callAt(fetchMock).init)).toEqual(request);
  });

  test.each([
    [400, "invalid_request", "mode preset needs pokemonId"],
    [422, "unknown_pokemon", "unknown pokemonId: 9999-000"],
    [413, "request_too_large", "request body exceeds 4 KiB"],
    [503, "master_unavailable", "pokemon read model is not configured"],
    [500, "internal_error", "internal error"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    const result = await client.position({ mode: "raw", value: 300 });
    expect(result).toEqual({ ok: false, error: { code, message } });
  });
});

describe("通信・応答の失敗は speed_unavailable(自動の切り替えはしない)", () => {
  const cases: ReadonlyArray<[string, () => Response | Error]> = [
    ["fetch が reject する(通信できない)", () => new TypeError("Failed to fetch")],
    [
      "200 だが本文が JSON でない",
      () => new Response("<html>gateway</html>", { status: 200, headers: { "Content-Type": "text/html" } }),
    ],
    [
      "502 で本文が JSON でない",
      () => new Response("Bad Gateway", { status: 502, headers: { "Content-Type": "text/plain" } }),
    ],
    ["500 で本文が {code, message} の形でない", () => jsonResponse(500, { error: "boom" })],
  ];

  test.each(cases)("pokemon: %s", async (_name, make) => {
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.pokemon();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("speed_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test.each(cases)("table: %s", async (_name, make) => {
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.table();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("speed_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test.each(cases)("position: %s", async (_name, make) => {
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fakeFetch(make()), ids });
    const result = await client.position({ mode: "raw", value: 300 });
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe("speed_unavailable");
      expect(result.error.message).not.toBe("");
    }
  });

  test("例外を投げない(失敗はすべて戻り値で返す)", async () => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(new TypeError("Failed to fetch")),
      ids,
    });
    await expect(client.pokemon()).resolves.toMatchObject({ ok: false });
  });
});
