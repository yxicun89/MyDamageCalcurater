// P5-5c/P5-5d: 記録 API(record-svc)のクライアント(ADR-0317 §1、ADR-0318 §1)。teamClient.ts と同じ流儀:
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
  /**
   * この端末の履歴・お気に入りをサーバーから削除する(ADR-0209 §5)。1回の呼び出しは1回の HTTP 要求。
   * partial のときの繰り返しは呼び出し側(deviceData/deleteDeviceData.ts)の仕事。
   */
  deleteDeviceData(): Promise<RecordResult<Schemas["RecordDeletionResult"]>>;
  /** この端末のお気に入り(更新の新しい順。サーバーの順のまま。ADR-0227・ADR-0327)。 */
  listFavorites(signal?: AbortSignal): Promise<RecordResult<Schemas["Favorite"][]>>;
  /** お気に入りに追加する。201 は created: true、同じ内容の再ピン留め(200)は created: false。 */
  createFavorite(input: Schemas["FavoriteInput"]): Promise<RecordResult<CreatedFavorite>>;
  /** お気に入りを外す(204 は本文なしの成功。404 も ok: false で返す。冪等とみなすのは画面の仕事)。 */
  deleteFavorite(id: string): Promise<RecordResult<void>>;
}

/** createFavorite の成功値。 */
export interface CreatedFavorite {
  readonly favorite: Schemas["Favorite"];
  readonly created: boolean;
}

/** 記録 API のパス(基点 URL からの相対)。 */
export const RECORD_PATHS = {
  frequentOpponents: "api/record/frequent-opponents",
  deviceData: "api/record/device-data",
  favorites: "api/record/favorites",
} as const;

/** 1端末のお気に入りの上限(api/openapi.yaml の /api/record/favorites の maxItems)。 */
export const MAX_FAVORITES_PER_DEVICE = 100;

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

function isDeletionResult(value: unknown): value is Schemas["RecordDeletionResult"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const status = (value as Record<string, unknown>).status;
  return status === "completed" || status === "partial";
}

function isFavorite(value: unknown): value is Schemas["Favorite"] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.id === "string" && typeof record.individual === "object" && record.individual !== null;
}

function unavailableResult<T>(): RecordResult<T> {
  return { ok: false, error: recordUnavailableError() };
}

/** 応答本文を JSON として読む(読めなければ undefined)。 */
async function readJson(response: Response): Promise<unknown> {
  try {
    return (await response.json()) as unknown;
  } catch {
    return undefined;
  }
}

/** 失敗応答の本文から RecordResult を作る(Error 封筒でなければ record_unavailable)。 */
function errorResult<T>(parsed: unknown): RecordResult<T> {
  return isErrorBody(parsed)
    ? { ok: false, error: { code: parsed.code, message: parsed.message } }
    : unavailableResult();
}

/** 記録 API のクライアント実装(ADR-0317 §1)。応答はそのまま運び、通信・応答の失敗は record_unavailable にする。 */
export function createRecordClient(input: CreateRecordClientInput): RecordClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;
  const idHeaders = { "X-Device-Id": ids.deviceId, "X-Session-Id": ids.sessionId };

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
    async deleteDeviceData() {
      let response: Response;
      try {
        response = await fetchImpl(`${baseUrl}${RECORD_PATHS.deviceData}`, {
          method: "DELETE",
          headers: { "X-Device-Id": ids.deviceId, "X-Session-Id": ids.sessionId },
        });
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
      return isDeletionResult(parsed) ? { ok: true, value: parsed } : unavailableResult();
    },
    async listFavorites(signal) {
      let response: Response;
      try {
        response = await fetchImpl(`${baseUrl}${RECORD_PATHS.favorites}`, {
          method: "GET",
          headers: idHeaders,
          signal,
        });
      } catch {
        return unavailableResult();
      }
      const parsed = await readJson(response);
      if (!response.ok) {
        return errorResult(parsed);
      }
      return Array.isArray(parsed) && parsed.every(isFavorite)
        ? { ok: true, value: parsed }
        : unavailableResult();
    },
    async createFavorite(favoriteInput) {
      let response: Response;
      try {
        response = await fetchImpl(`${baseUrl}${RECORD_PATHS.favorites}`, {
          method: "POST",
          headers: { ...idHeaders, "Content-Type": "application/json" },
          body: JSON.stringify(favoriteInput),
        });
      } catch {
        return unavailableResult();
      }
      const parsed = await readJson(response);
      if (!response.ok) {
        return errorResult(parsed);
      }
      return isFavorite(parsed)
        ? { ok: true, value: { favorite: parsed, created: response.status === 201 } }
        : unavailableResult();
    },
    async deleteFavorite(id) {
      let response: Response;
      try {
        response = await fetchImpl(`${baseUrl}${RECORD_PATHS.favorites}/${encodeURIComponent(id)}`, {
          method: "DELETE",
          headers: idHeaders,
        });
      } catch {
        return unavailableResult();
      }
      if (response.status === 204) {
        return { ok: true, value: undefined };
      }
      const parsed = await readJson(response);
      return response.ok ? unavailableResult() : errorResult(parsed);
    },
  };
}
