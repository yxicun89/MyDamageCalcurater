// F-11(ADR-0331 §2・§6・受け入れ条件 AC4): 調整 API のクライアントの adjustGoals。
// 既存の4操作(adjustClient.test.ts の C1〜C8)と同じ扱いを、新しい操作にも求める。
// 確かめること:
//   CG1 パスは api/calc/adjust/goals(gateway の /api/calc/* の前方一致に乗る。ADR-0250 §1)
//   CG2 端末 ID・セッション ID・Content-Type 付きで request をそのまま POST し、応答をそのまま返す
//   CG3 HTTP エラーの {code, message} はそのまま運ぶ。通信できない・JSON でない・形が不正は adjust_unavailable
//   CG4 signal を fetch に渡す。呼ぶ前に abort 済みなら fetch せず request_aborted、通信中の abort も request_aborted
// 架空データだけを使う(ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { REQUEST_ABORTED_CODE } from "../engine/types";
import { ADJUST_GOALS_PATH, ADJUST_UNAVAILABLE_CODE, createAdjustClient } from "./adjustClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};

const BASE_URL = "http://adjust.test/";

const self: Schemas["Individual"] = {
  speciesKey: "9001-000",
  level: 50,
  natureId: "test-nature-neutral",
  sp: { hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};

const opponent: Schemas["Individual"] = {
  speciesKey: "9002-000",
  level: 50,
  natureId: "test-nature-plus-spe",
  sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 32 },
};

/** 素早さ+倒す の2つの目標(判定レーンの元の要望「抜けて倒せるか」と同じ形)。 */
const goalsRequest: Schemas["AdjustGoalsRequest"] = {
  format: "single",
  self,
  goals: [
    { kind: "outspeed", opponent, moveId: "test-move-boost" },
    { kind: "ko", opponent, moveId: "test-move-fire", hits: 1 },
  ],
};

const goalsResult: Schemas["AdjustGoalsResult"] = {
  feasible: true,
  remaining: 20,
  plan: {
    sp: { hp: 4, atk: 22, def: 0, spa: 0, spd: 0, spe: 20 },
    totalSp: 46,
    stats: { hp: 159, atk: 142, def: 90, spa: 80, spd: 90, spe: 150 },
  },
  goals: [
    {
      kind: "outspeed",
      met: true,
      chancePercent: null,
      selfSpeed: 225,
      opponentSpeed: 167,
      selfSpeedRank: 1,
    },
    { kind: "ko", met: true, chancePercent: 100, selfSpeed: null, opponentSpeed: null, selfSpeedRank: null },
  ],
  unsupported: [],
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function fakeFetch(result: Response | Error) {
  return vi.fn<typeof fetch>(() =>
    result instanceof Error ? Promise.reject(result) : Promise.resolve(result),
  );
}

function firstCall(fetchMock: ReturnType<typeof fakeFetch>): { url: string; init: RequestInit } {
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

describe("CG1・CG2 adjustGoals の呼び出し", () => {
  test("パスは api/calc/adjust/goals", () => {
    expect(ADJUST_GOALS_PATH).toBe("api/calc/adjust/goals");
  });

  test("端末 ID・セッション ID 付きで request をそのまま POST し、応答をそのまま返す", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, goalsResult));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });

    const result = await client.goals(goalsRequest);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { url, init } = firstCall(fetchMock);
    expect(url).toBe(`${BASE_URL}api/calc/adjust/goals`);
    expect(init.method).toBe("POST");
    expect(Object.fromEntries(new Headers(init.headers).entries())).toMatchObject({
      "content-type": "application/json",
      "x-device-id": ids.deviceId,
      "x-session-id": ids.sessionId,
    });
    // 省略された欄(ceiling・thresholdPercent)を補わない(既定はサーバーが持つ)。
    expect(JSON.parse(typeof init.body === "string" ? init.body : "null")).toEqual(goalsRequest);
    // 応答はそのまま運ぶ(Web で計算し直さない・確率を丸めない)。
    expect(result).toEqual({ ok: true, value: goalsResult });
  });
});

describe("CG3 失敗の写像(例外を投げない)", () => {
  test("HTTP エラーの {code, message} はそのまま運ぶ", async () => {
    const fetchMock = fakeFetch(jsonResponse(400, { code: "invalid_input", message: "goals: too many" }));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    expect(await client.goals(goalsRequest)).toEqual({
      ok: false,
      error: { code: "invalid_input", message: "goals: too many" },
    });
  });

  test.each([
    ["通信できない", () => fakeFetch(new TypeError("network"))],
    ["JSON でない", () => fakeFetch(new Response("not json", { status: 200 }))],
    ["エラー本文の形が不正", () => fakeFetch(jsonResponse(500, { error: "x" }))],
  ])("%s → adjust_unavailable", async (_label, make) => {
    const fetchMock = make();
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const result = await client.goals(goalsRequest);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(ADJUST_UNAVAILABLE_CODE);
    }
  });
});

describe("CG4 取り消し", () => {
  test("signal を fetch に渡す", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, goalsResult));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const controller = new AbortController();
    await client.goals(goalsRequest, controller.signal);
    expect(firstCall(fetchMock).init.signal).toBe(controller.signal);
  });

  test("呼ぶ前に abort 済みなら fetch せず request_aborted", async () => {
    const fetchMock = fakeFetch(jsonResponse(200, goalsResult));
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const controller = new AbortController();
    controller.abort();
    const result = await client.goals(goalsRequest, controller.signal);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(REQUEST_ABORTED_CODE);
    }
  });

  test("通信中の abort は request_aborted(adjust_unavailable にしない)", async () => {
    const controller = new AbortController();
    const fetchMock = vi.fn<typeof fetch>(() => {
      controller.abort();
      return Promise.reject(new DOMException("aborted", "AbortError"));
    });
    const client = createAdjustClient({ baseUrl: BASE_URL, fetch: fetchMock, ids });
    const result = await client.goals(goalsRequest, controller.signal);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(REQUEST_ABORTED_CODE);
    }
  });
});
