// Web: 計算履歴の一覧(ADR-0230「Web・iOS レーンへの依頼」・ADR-0338)。createRecordClient の listCalcHistory。
// recordClient.favorites.test.ts と同じ流儀(fetch は fake、例外を投げない、RecordResult の判別 union)。
// 確かめること(受け入れ条件 AC-1):
//   - GET `${baseUrl}api/record/calc-history?limit=20`(RECORD_PATHS.calcHistory・CALC_HISTORY_PAGE_LIMIT)。
//     X-Device-Id・X-Session-Id を付け、本文なし。渡した AbortSignal を fetch に渡す
//   - cursor を渡すと同じ limit のまま `&cursor=` を足す。値は加工しない(クエリとして安全に符号化するだけ。復号すると元の文字列)
//   - 成功は {items, nextCursor} をそのまま運ぶ(並べ替え・件数の補正をしない)。nextCursor: null も成功
//   - エラー本文が {code, message} ならそのまま運ぶ(400 invalid_input / 503 store_unavailable・upstream_unavailable)
//   - 通信できない・JSON でない・成功本文の形が不正・エラー本文の形が不正は record_unavailable。例外を投げない
// 架空の key だけを使う(ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import {
  CALC_HISTORY_PAGE_LIMIT,
  RECORD_PATHS,
  RECORD_UNAVAILABLE_CODE,
  createRecordClient,
} from "./recordClient";

type Schemas = components["schemas"];

const ids: ClientIds = {
  deviceId: "11111111-1111-4111-8111-111111111111",
  sessionId: "22222222-2222-4222-a222-222222222222",
};
const BASE_URL = "http://record.test/";

const page: Schemas["CalcHistoryPage"] = {
  items: [
    {
      occurredAt: "2026-10-09T03:00:00Z",
      calc: {
        format: "single",
        attacker: {
          speciesKey: "9001-000",
          level: 50,
          natureId: "fake-nature",
          sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 },
        },
        defender: {
          speciesKey: "9002-000",
          level: 50,
          natureId: "fake-nature",
          sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
        },
        moveId: "fake-move",
      },
      result: { minPercent: 41.2, maxPercent: 48.9 },
    },
  ],
  nextCursor: "opaque+cursor/with=chars&more",
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function clientWith(fetchImpl: typeof fetch) {
  return createRecordClient({ baseUrl: BASE_URL, fetch: fetchImpl, ids });
}

const unavailable = {
  ok: false,
  error: expect.objectContaining({ code: RECORD_UNAVAILABLE_CODE }) as unknown,
};

describe("定数", () => {
  test("1ページは 20 件固定、パスは api/record/calc-history", () => {
    expect(CALC_HISTORY_PAGE_LIMIT).toBe(20);
    expect(RECORD_PATHS.calcHistory).toBe("api/record/calc-history");
  });
});

describe("createRecordClient.listCalcHistory", () => {
  test("最初のページ: GET ?limit=20、端末 ID・セッション ID をヘッダーで運び、本文なし。signal を渡す", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(200, page)));
    const controller = new AbortController();
    await clientWith(fetchMock).listCalcHistory(undefined, controller.signal);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`${BASE_URL}api/record/calc-history?limit=20`);
    expect(init?.method ?? "GET").toBe("GET");
    const headers = new Headers(init?.headers);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
    expect(headers.get("Content-Type")).toBeNull();
    expect(init?.body ?? undefined).toBeUndefined();
    expect(init?.signal).toBe(controller.signal);
  });

  test("続きのページ: 同じ limit に cursor を足す。cursor は復号すると元の文字列のまま(加工しない)", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(200, page)));
    const cursor = "opaque+cursor/with=chars&more";
    await clientWith(fetchMock).listCalcHistory(cursor);

    const [url] = fetchMock.mock.calls[0] ?? [];
    const parsed = new URL(url as string);
    expect(`${parsed.origin}${parsed.pathname}`).toBe(`${BASE_URL}api/record/calc-history`);
    expect(parsed.searchParams.get("limit")).toBe("20");
    expect(parsed.searchParams.get("cursor")).toBe(cursor);
    expect([...parsed.searchParams.keys()].sort()).toEqual(["cursor", "limit"]);
  });

  test("成功は items と nextCursor をそのまま運ぶ。nextCursor: null(終わり)・空の items も成功", async () => {
    expect(await clientWith(() => Promise.resolve(jsonResponse(200, page))).listCalcHistory()).toEqual({
      ok: true,
      value: page,
    });
    const empty = { items: [], nextCursor: null };
    expect(await clientWith(() => Promise.resolve(jsonResponse(200, empty))).listCalcHistory()).toEqual({
      ok: true,
      value: empty,
    });
  });

  test.each([
    [400, "invalid_input", "カーソルが不正です"],
    [503, "store_unavailable", "記録を読めません"],
    [503, "upstream_unavailable", "記録サービスに届きません"],
  ])("%i %s は code・message をそのまま運ぶ", async (status, code, message) => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(status, { code, message })),
    ).listCalcHistory();
    expect(result).toEqual({ ok: false, error: { code, message } });
  });

  test.each([
    ["通信できない", () => Promise.reject(new TypeError("network"))],
    ["応答が JSON でない", () => Promise.resolve(new Response("<html>", { status: 200 }))],
    ["成功本文が配列(items を持たない)", () => Promise.resolve(jsonResponse(200, []))],
    ["成功本文に items が無い", () => Promise.resolve(jsonResponse(200, { nextCursor: null }))],
    ["items が配列でない", () => Promise.resolve(jsonResponse(200, { items: {}, nextCursor: null }))],
    ["nextCursor が無い", () => Promise.resolve(jsonResponse(200, { items: [] }))],
    [
      "nextCursor が文字列でも null でもない",
      () => Promise.resolve(jsonResponse(200, { items: [], nextCursor: 3 })),
    ],
    [
      "行が壊れている(calc が無い)",
      () => Promise.resolve(jsonResponse(200, { items: [{ occurredAt: "x" }], nextCursor: null })),
    ],
    ["エラー本文の形が不正", () => Promise.resolve(jsonResponse(500, { oops: true }))],
  ])("%s なら record_unavailable(例外を投げない)", async (_name, respond) => {
    await expect(clientWith(respond as typeof fetch).listCalcHistory()).resolves.toMatchObject(unavailable);
  });
});
