// P5-5c: 記録 API(record-svc)のクライアント(ADR-0317 §1)。teamClient.ts と同じ流儀:
// 例外を投げず、常に RecordResult<T>(判別 union)で返す。型はルートの api/openapi.gen.ts を使う。

import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { recordClientText } from "../i18n/ja";

type Schemas = components["schemas"];

/** 記録 API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の record_unavailable)。 */
export interface RecordError {
  readonly code: string;
  readonly message: string;
}

/** 記録 API の呼び出しの成否(判別 union)。 */
export type RecordResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: RecordError };

export interface CreateRecordClientInput {
  /** gateway の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  readonly fetch: typeof fetch;
  readonly ids: ClientIds;
}

export interface RecordClient {
  /** この端末のよく計算する相手(スコア降順。記録が無ければ空配列)。 */
  listFrequentOpponents(signal?: AbortSignal): Promise<RecordResult<Schemas["FrequentOpponent"][]>>;
}

/** 記録 API のパス(基点 URL からの相対)。 */
export const RECORD_PATHS = {
  frequentOpponents: "api/record/frequent-opponents",
} as const;

/** チップに出す上限として API へ渡す limit(openapi の 1〜50 の範囲内。ADR-0317 §1)。 */
export const FREQUENT_OPPONENTS_LIMIT = 5;

/** 通信できない・応答が読めない・エラー本文の形が不正なときの Web 側のコード。 */
export const RECORD_UNAVAILABLE_CODE = "record_unavailable";

export function recordUnavailableError(): RecordError {
  return { code: RECORD_UNAVAILABLE_CODE, message: recordClientText.unavailable };
}

function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

function unavailableResult<T>(): RecordResult<T> {
  return { ok: false, error: recordUnavailableError() };
}

/** 記録 API のクライアント実装(ADR-0317 §1)。応答はそのまま運び、通信・応答の失敗は record_unavailable にする。 */
export function createRecordClient(input: CreateRecordClientInput): RecordClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  return {
    async listFrequentOpponents(signal) {
      let response: Response;
      try {
        response = await fetchImpl(
          `${baseUrl}${RECORD_PATHS.frequentOpponents}?limit=${String(FREQUENT_OPPONENTS_LIMIT)}`,
          {
            method: "GET",
            headers: { "X-Device-Id": ids.deviceId, "X-Session-Id": ids.sessionId },
            signal,
          },
        );
      } catch {
        return unavailableResult();
      }
      let parsed: unknown;
      try {
        parsed = await response.json();
      } catch {
        return unavailableResult();
      }
      if (!response.ok) {
        return isErrorBody(parsed)
          ? { ok: false, error: { code: parsed.code, message: parsed.message } }
          : unavailableResult();
      }
      return Array.isArray(parsed)
        ? { ok: true, value: parsed as Schemas["FrequentOpponent"][] }
        : unavailableResult();
    },
  };
}
