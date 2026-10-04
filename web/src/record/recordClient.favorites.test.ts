// P5-3c(ADR-0227・ADR-0327): お気に入り API のクライアント(createRecordClient の listFavorites / createFavorite / deleteFavorite)。
// recordClient.test.ts と同じ流儀(fetch は fake、例外を投げない、RecordResult の判別 union)。
// 確かめること(受け入れ条件 AC-1):
//   - listFavorites: GET `${baseUrl}api/record/favorites`(RECORD_PATHS.favorites)。X-Device-Id・X-Session-Id を付け、本文なし。
//     成功は配列をそのまま(並べ替えず)運ぶ。空配列も成功。渡した AbortSignal を fetch に渡す
//   - createFavorite: POST 同パス。Content-Type: application/json、本文は入力の JSON そのまま。
//     201 は {created: true}、200(同じ内容の再ピン留め)は {created: false}。どちらも favorite は応答の本文
//   - deleteFavorite: DELETE `${baseUrl}api/record/favorites/${id}`。id は encodeURIComponent する。204(本文なし)は {ok: true}
//   - エラー: 本文が {code, message} ならそのまま運ぶ(400 invalid_input〈上限〉/ 404 not_found / 503 store_unavailable)。
//     404 も ok: false として返す(「もう無い」を成功とみなす判断は画面の仕事)
//   - 通信できない・応答が JSON でない(204 以外)・エラー本文の形が不正・成功本文の形が不正は record_unavailable。例外を投げない
//   - 上限件数は MAX_FAVORITES_PER_DEVICE(= 100。契約の maxItems と同じ)
// 架空の key だけを使う(ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import {
  MAX_FAVORITES_PER_DEVICE,
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

const input: Schemas["FavoriteInput"] = {
  label: "HB特化",
  individual: {
    speciesKey: "9002-000",
    level: 50,
    natureId: "fake-nature",
    sp: { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 },
  },
};

const favorite: Schemas["Favorite"] = {
  id: "12",
  label: "HB特化",
  individual: input.individual,
  createdAt: "2026-10-04T01:00:00Z",
  updatedAt: "2026-10-04T01:00:00Z",
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
  test("上限は契約(maxItems)と同じ 100、パスは api/record/favorites", () => {
    expect(MAX_FAVORITES_PER_DEVICE).toBe(100);
    expect(RECORD_PATHS.favorites).toBe("api/record/favorites");
  });
});

describe("createRecordClient.listFavorites", () => {
  test("GET で端末 ID・セッション ID をヘッダーで運び、本文を付けない。signal を渡す", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(200, [favorite])));
    const controller = new AbortController();
    await clientWith(fetchMock).listFavorites(controller.signal);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`${BASE_URL}api/record/favorites`);
    expect(init?.method ?? "GET").toBe("GET");
    const headers = new Headers(init?.headers);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
    expect(headers.get("Content-Type")).toBeNull();
    expect(init?.body ?? undefined).toBeUndefined();
    expect(init?.signal).toBe(controller.signal);
  });

  test("成功は配列をそのまま運ぶ(空配列も成功)", async () => {
    const second = { ...favorite, id: "11" };
    expect(
      await clientWith(() => Promise.resolve(jsonResponse(200, [favorite, second]))).listFavorites(),
    ).toEqual({ ok: true, value: [favorite, second] });
    expect(await clientWith(() => Promise.resolve(jsonResponse(200, []))).listFavorites()).toEqual({
      ok: true,
      value: [],
    });
  });

  test("503 は code・message をそのまま運ぶ", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(503, { code: "store_unavailable", message: "記録を読めません" })),
    ).listFavorites();
    expect(result).toEqual({ ok: false, error: { code: "store_unavailable", message: "記録を読めません" } });
  });

  test.each([
    ["通信できない", () => Promise.reject(new TypeError("network"))],
    ["応答が JSON でない", () => Promise.resolve(new Response("<html>", { status: 200 }))],
    ["成功本文が配列でない(オブジェクト)", () => Promise.resolve(jsonResponse(200, { id: "1" }))],
    ["成功本文が配列でない(null)", () => Promise.resolve(jsonResponse(200, null))],
    ["エラー本文の形が不正", () => Promise.resolve(jsonResponse(500, { oops: true }))],
  ])("%s なら record_unavailable(例外を投げない)", async (_name, respond) => {
    await expect(clientWith(respond as typeof fetch).listFavorites()).resolves.toMatchObject(unavailable);
  });
});

describe("createRecordClient.createFavorite", () => {
  test("POST で JSON 本文をそのまま送り、端末 ID・セッション ID・Content-Type を付ける", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(201, favorite)));
    await clientWith(fetchMock).createFavorite(input);

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`${BASE_URL}api/record/favorites`);
    expect(init?.method).toBe("POST");
    const headers = new Headers(init?.headers);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(JSON.parse(init?.body as string)).toEqual(input);
  });

  test("201 は created: true、200(同じ内容の再ピン留め)は created: false。favorite は応答の本文", async () => {
    expect(
      await clientWith(() => Promise.resolve(jsonResponse(201, favorite))).createFavorite(input),
    ).toEqual({ ok: true, value: { favorite, created: true } });
    const bumped = { ...favorite, updatedAt: "2026-10-04T02:00:00Z" };
    expect(await clientWith(() => Promise.resolve(jsonResponse(200, bumped))).createFavorite(input)).toEqual({
      ok: true,
      value: { favorite: bumped, created: false },
    });
  });

  test("上限(400 invalid_input)は code・message をそのまま運ぶ", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(400, { code: "invalid_input", message: "お気に入りは100件までです" })),
    ).createFavorite(input);
    expect(result).toEqual({
      ok: false,
      error: { code: "invalid_input", message: "お気に入りは100件までです" },
    });
  });

  test.each([
    ["通信できない", () => Promise.reject(new TypeError("network"))],
    ["応答が JSON でない", () => Promise.resolve(new Response("x", { status: 201 }))],
    ["成功本文がオブジェクトでない(配列)", () => Promise.resolve(jsonResponse(201, [favorite]))],
    ["成功本文がオブジェクトでない(null)", () => Promise.resolve(jsonResponse(200, null))],
    ["成功本文に id が無い", () => Promise.resolve(jsonResponse(201, { label: null }))],
    ["エラー本文の形が不正", () => Promise.resolve(jsonResponse(400, "bad"))],
  ])("%s なら record_unavailable(例外を投げない)", async (_name, respond) => {
    await expect(clientWith(respond as typeof fetch).createFavorite(input)).resolves.toMatchObject(
      unavailable,
    );
  });
});

describe("createRecordClient.deleteFavorite", () => {
  test("DELETE で id をパスに入れ(encodeURIComponent)、ヘッダーを付け、本文を付けない", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(new Response(null, { status: 204 })));
    await clientWith(fetchMock).deleteFavorite("12");
    await clientWith(fetchMock).deleteFavorite("a/b?c");

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`${BASE_URL}api/record/favorites/12`);
    expect(init?.method).toBe("DELETE");
    const headers = new Headers(init?.headers);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
    expect(init?.body ?? undefined).toBeUndefined();
    expect(fetchMock.mock.calls[1]?.[0]).toBe(`${BASE_URL}api/record/favorites/a%2Fb%3Fc`);
  });

  test("204(本文なし)は成功", async () => {
    const result = await clientWith(() =>
      Promise.resolve(new Response(null, { status: 204 })),
    ).deleteFavorite("12");
    expect(result.ok).toBe(true);
  });

  test("404 not_found は ok: false で code・message をそのまま運ぶ(冪等とみなすのは画面の仕事)", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(404, { code: "not_found", message: "お気に入りが見つかりません" })),
    ).deleteFavorite("12");
    expect(result).toEqual({
      ok: false,
      error: { code: "not_found", message: "お気に入りが見つかりません" },
    });
  });

  test.each([
    ["通信できない", () => Promise.reject(new TypeError("network"))],
    ["エラー応答が JSON でない", () => Promise.resolve(new Response("<html>", { status: 502 }))],
    ["エラー本文の形が不正", () => Promise.resolve(jsonResponse(500, { oops: true }))],
  ])("%s なら record_unavailable(例外を投げない)", async (_name, respond) => {
    await expect(clientWith(respond as typeof fetch).deleteFavorite("12")).resolves.toMatchObject(
      unavailable,
    );
  });
});
