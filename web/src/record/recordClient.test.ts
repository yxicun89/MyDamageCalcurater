// P5-5c: 記録 API のクライアント createRecordClient(ADR-0317 §1)。teamClient.test.ts と同じ形。
// ルートの api/openapi.yaml を正とし、応答は生成型(api/openapi.gen.ts)で書く。fetch は fake。
// 確かめること(受け入れ条件 AC-1):
//   - GET `${baseUrl}api/record/frequent-opponents?limit=${FREQUENT_OPPONENTS_LIMIT}` を呼ぶ(端末 ID はパスに含めない)
//   - ヘッダーは X-Device-Id・X-Session-Id(他の API と同じ ID)。本文は無いので Content-Type は付けない
//   - 成功は {ok: true, value}(応答を並べ替え・加工せずそのまま運ぶ)。空配列も成功
//   - 渡した AbortSignal を fetch に渡す
//   - HTTP エラーで本文が {code, message} なら、その code・message をそのまま運ぶ(503 store_unavailable 等)
//   - 通信できない・応答が JSON でない・エラー本文の形が不正なら record_unavailable(Web 側のコード)
//   - 例外を投げない
// 架空の key だけを使う(実データは使わない。ADR-0002)。

import { describe, expect, test, vi } from "vitest";
import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import {
  FREQUENT_OPPONENTS_LIMIT,
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

const opponents: Schemas["FrequentOpponent"][] = [
  { speciesKey: "9002-000", score: 3.5, count: 4, lastCalculatedAt: "2026-10-01T01:00:00Z" },
  { speciesKey: "9001-000", score: 1.25, count: 1, lastCalculatedAt: "2026-09-30T01:00:00Z" },
];

function urlText(input: unknown): string {
  return typeof input === "string" ? input : "";
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function clientWith(fetchImpl: typeof fetch) {
  return createRecordClient({ baseUrl: BASE_URL, fetch: fetchImpl, ids });
}

describe("createRecordClient.listFrequentOpponents", () => {
  test("GET で limit 付きの URL を呼び、端末 ID・セッション ID をヘッダーで運ぶ", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(200, opponents)));
    await clientWith(fetchMock).listFrequentOpponents();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(
      `${BASE_URL}${RECORD_PATHS.frequentOpponents}?limit=${String(FREQUENT_OPPONENTS_LIMIT)}`,
    );
    expect(urlText(url)).not.toContain(ids.deviceId);
    expect(init?.method ?? "GET").toBe("GET");
    const headers = new Headers(init?.headers);
    expect(headers.get("X-Device-Id")).toBe(ids.deviceId);
    expect(headers.get("X-Session-Id")).toBe(ids.sessionId);
    expect(headers.get("Content-Type")).toBeNull();
    expect(init?.body ?? undefined).toBeUndefined();
  });

  test("limit は openapi の範囲(1〜50)に収まる", () => {
    expect(FREQUENT_OPPONENTS_LIMIT).toBeGreaterThanOrEqual(1);
    expect(FREQUENT_OPPONENTS_LIMIT).toBeLessThanOrEqual(50);
  });

  test("成功は応答をそのまま(並べ替えず)運ぶ", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(200, opponents)),
    ).listFrequentOpponents();
    expect(result).toEqual({ ok: true, value: opponents });
  });

  test("記録が無い端末の空配列も成功として運ぶ", async () => {
    const result = await clientWith(() => Promise.resolve(jsonResponse(200, []))).listFrequentOpponents();
    expect(result).toEqual({ ok: true, value: [] });
  });

  test("渡した AbortSignal を fetch に渡す", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse(200, [])));
    const controller = new AbortController();
    await clientWith(fetchMock).listFrequentOpponents(controller.signal);
    expect(fetchMock.mock.calls[0]?.[1]?.signal).toBe(controller.signal);
  });

  test("サーバーのエラー本文({code, message})はそのまま運ぶ", async () => {
    const body = { code: "store_unavailable", message: "記録を読めません" };
    const result = await clientWith(() => Promise.resolve(jsonResponse(503, body))).listFrequentOpponents();
    expect(result).toEqual({ ok: false, error: body });
  });

  test("400 invalid_input も code・message を運ぶ", async () => {
    const body = { code: "invalid_input", message: "limit が範囲外です" };
    const result = await clientWith(() => Promise.resolve(jsonResponse(400, body))).listFrequentOpponents();
    expect(result).toEqual({ ok: false, error: body });
  });

  test("通信できない(fetch が reject)は record_unavailable で、例外を投げない", async () => {
    const result = await clientWith(() =>
      Promise.reject(new TypeError("fetch failed")),
    ).listFrequentOpponents();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
      expect(result.error.message).not.toBe("");
    }
  });

  test("応答が JSON でない(例: プロキシの HTML)は record_unavailable", async () => {
    const result = await clientWith(() =>
      Promise.resolve(new Response("<html>bad gateway</html>", { status: 502 })),
    ).listFrequentOpponents();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
    }
  });

  test("エラー本文の形が不正(code・message が無い)なら record_unavailable", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(500, { error: "boom" })),
    ).listFrequentOpponents();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
    }
  });

  test("200 でも本文が配列でないなら record_unavailable(壊れた応答をチップにしない)", async () => {
    const result = await clientWith(() =>
      Promise.resolve(jsonResponse(200, { speciesKey: "9001-000" })),
    ).listFrequentOpponents();
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.code).toBe(RECORD_UNAVAILABLE_CODE);
    }
  });
});
