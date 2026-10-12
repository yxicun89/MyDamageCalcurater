// I-speed-4b(ADR-0610 §2・§4): speed API のクライアントの matchup(上を取る最低ライン)と reach(これ以上)。
// speedClient.test.ts と同じ形(fetch は fake。本文・応答は生成型 speed.gen.ts で書く)。
// 確かめること:
//   - matchup は MatchupRequest を POST、reach は PositionRequest(position と同じ本文)を POST し、
//     `${baseUrl}api/speed/v1/matchup|reach` を呼ぶ。ヘッダーは X-Device-Id・X-Session-Id・Content-Type
//   - 本文は渡されたまま送る(省略した欄を足さない)。応答は整形せずそのまま運ぶ(Web で素早さを計算し直さない)
//   - HTTP エラーで本文が {code, message} ならそのまま運ぶ。通信できない・読めない・本文の形が不正は speed_unavailable
//   - 既存の table・position・pokemon のパスは変えない(データ・API は残す)

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "./speed.gen";
import { createSpeedClient, SPEED_PATHS } from "./speedClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://speed.test/";

const matchupRequest: Schemas["MatchupRequest"] = {
  self: { pokemonId: "9001-000", rank: 1, scarf: true, current: { sp: 20, nature: "plus" } },
  opponent: {
    pokemonId: "9002-000",
    assumptions: [{ preset: "max" }, { custom: { sp: 10, nature: "neutral", rank: 0, scarf: false } }],
    tailwind: true,
  },
  trickRoom: true,
};

const matchupResponse: Schemas["MatchupResponse"] = {
  self: { pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100 },
  opponent: { pokemonId: "9002-000", nameJa: "テストカソウギョ", types: ["water"], baseSpeed: 80 },
  trickRoom: true,
  selfSpeed: 167,
  lines: [
    {
      assumption: { preset: "max" },
      opponentSpeed: 200,
      current: "opponent_first",
      natures: [
        { nature: "minus" },
        { nature: "neutral", movesFirst: { min: 0, max: 5 }, tie: { min: 6, max: 6 } },
        { nature: "plus", movesFirst: { min: 0, max: 2 } },
      ],
    },
  ],
};

const reachRequest: Schemas["PositionRequest"] = {
  mode: "custom",
  pokemonId: "9001-000",
  sp: 20,
  nature: "plus",
  rank: 0,
  scarf: false,
  tableTailwind: true,
};

const reachResponse: Schemas["ReachResponse"] = {
  speed: 170,
  lines: [
    {
      preset: "neutral-max",
      slower: { min: 1, max: 117 },
      tie: { min: 118, max: 118 },
      faster: { min: 119, max: 255 },
      slowerCount: 5,
      tieCount: 1,
      fasterCount: 2,
    },
  ],
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function fakeFetch(response: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    response instanceof Error ? Promise.reject(response) : Promise.resolve(response),
  );
}

function callAt(fetchMock: ReturnType<typeof fakeFetch>): { url: string; init: RequestInit } {
  const call = fetchMock.mock.calls[0];
  if (call === undefined) {
    throw new Error("fetch が呼ばれていない");
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

describe("matchup(上を取る最低ライン)", () => {
  test("matchup のパスに、端末 ID・セッション ID 付きで MatchupRequest を POST し、応答をそのまま運ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, matchupResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.matchup(matchupRequest);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/matchup");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(bodyOf(init)).toEqual(matchupRequest);
    // 区間・性格の並び・欠けている欄(movesFirst なし)を整形せずに運ぶ。
    expect(result).toEqual({ ok: true, value: matchupResponse });
  });

  test("省略した欄(rank・scarf・assumptions など)を足さない", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, matchupResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const minimal: Schemas["MatchupRequest"] = {
      self: { pokemonId: "9001-000" },
      opponent: { pokemonId: "9002-000" },
    };

    await client.matchup(minimal);

    expect(bodyOf(callAt(fetchMock).init)).toEqual(minimal);
  });

  test.each([
    [400, "invalid_request", "assumption needs exactly one of preset or custom"],
    [422, "unknown_pokemon", "unknown pokemonId: 9999-000"],
    [413, "request_too_large", "request body exceeds 4 KiB"],
    [503, "master_unavailable", "pokemon read model is not configured"],
    [503, "overloaded", "too many requests"],
    [500, "internal_error", "internal error"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    const result = await client.matchup(matchupRequest);
    expect(result).toEqual({ ok: false, error: { code, message } });
  });

  test("通信できない・エラー本文の形が不正は speed_unavailable(例外を投げない)", async () => {
    const networkDown = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(new TypeError("Failed to fetch")),
      ids,
    });
    await expect(networkDown.matchup(matchupRequest)).resolves.toMatchObject({
      ok: false,
      error: { code: "speed_unavailable" },
    });
    const badBody = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(502, { unexpected: true })),
      ids,
    });
    await expect(badBody.matchup(matchupRequest)).resolves.toMatchObject({
      ok: false,
      error: { code: "speed_unavailable" },
    });
  });
});

describe("reach(これ以上)", () => {
  test("reach のパスに PositionRequest(position と同じ本文)を POST し、応答をそのまま運ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, reachResponse));
    const client = createSpeedClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.reach(reachRequest);

    const { url, init } = callAt(fetchMock);
    expect(url).toBe("http://speed.test/api/speed/v1/reach");
    expect(init.method).toBe("POST");
    expect(headersOf(init)).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    expect(bodyOf(init)).toEqual(reachRequest);
    expect(result).toEqual({ ok: true, value: reachResponse });
  });

  test.each([
    [400, "invalid_request", "mode preset needs pokemonId"],
    [422, "unknown_pokemon", "unknown pokemonId: 9999-000"],
    [503, "master_unavailable", "pokemon read model is not configured"],
  ] as const)("HTTP %i の {code: %s} をそのまま運ぶ", async (status, code, message) => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(jsonResponse(status, { code, message } satisfies Schemas["Error"])),
      ids,
    });
    const result = await client.reach({ mode: "raw", value: 170 });
    expect(result).toEqual({ ok: false, error: { code, message } });
  });

  test("通信できないときは speed_unavailable(例外を投げない)", async () => {
    const client = createSpeedClient({
      baseUrl: BASE_URL,
      fetch: fakeFetch(new TypeError("Failed to fetch")),
      ids,
    });
    await expect(client.reach({ mode: "raw", value: 170 })).resolves.toMatchObject({
      ok: false,
      error: { code: "speed_unavailable" },
    });
  });
});

describe("残す API(表・位置・一覧)は変えない(ADR-0610 §2)", () => {
  test("SPEED_PATHS は pokemon・table・position を残し、matchup・reach を足す", () => {
    expect(SPEED_PATHS).toEqual({
      pokemon: "api/speed/v1/pokemon",
      table: "api/speed/v1/table",
      position: "api/speed/v1/position",
      matchup: "api/speed/v1/matchup",
      reach: "api/speed/v1/reach",
    });
  });
});
