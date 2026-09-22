// SP3: speed API のクライアント createSpeedClient(ADR-0604 §3)。balance の api/balanceClient.ts と
// 同じ設計(ADR-0303 §1・§5・§6 を踏襲): 例外を投げず、常に SpeedResult<T> で返す。
// リクエスト・応答は openapi-typescript の生成型(speed.gen.ts)のまま運び、Web で素早さを計算し直さない
// (実数値・段・位置はすべて speed-svc が決める。docs/speed-design.md §2)。
// 通信できない・応答が読めない・エラー本文の形が不正なときは speed_unavailable(Web 側のコード)にする。
// 基点 URL(api/config.ts の apiBaseUrl)・端末 ID/セッション ID(api/clientIds.ts の ClientIds)は
// Web レーン共通のヘルパーをそのまま使う(ADR-0604 §3。あちらは変更しない)。
//
import type { ClientIds } from "../api/clientIds";
import { speedClientText } from "../i18n/ja";
import type { components } from "./speed.gen";

type Schemas = components["schemas"];

/** speed API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の speed_unavailable)。 */
export interface SpeedError {
  readonly code: string;
  readonly message: string;
}

/** speed API の呼び出しの成否(判別 union)。例外を投げず、常にこの形で返す。 */
export type SpeedResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: SpeedError };

/** createSpeedClient の引数(balanceClient.ts の CreateBalanceClientInput と同じ形)。 */
export interface CreateSpeedClientInput {
  /** speed API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 計算・タイプバランスと同じ端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/** speed API のクライアント(ADR-0604 §3)。呼び出し側の配列を書き換えないので readonly で受ける。 */
export interface SpeedClient {
  /** 既定のレギュレーションで使用可能なポケモンの一覧(ADR-0600 §5)。 */
  pokemon(): Promise<SpeedResult<Schemas["PokemonListResponse"]>>;
  /** 素早さの表。presets を省くと全6行(ADR-0601 §4)。 */
  table(presets?: readonly Schemas["PresetId"][]): Promise<SpeedResult<Schemas["TableResponse"]>>;
  /** 自分のポケモンの実数値と、表の中の位置(ADR-0602 §3)。 */
  position(request: Schemas["PositionRequest"]): Promise<SpeedResult<Schemas["PositionResponse"]>>;
}

/** speed API のパス(services/speed/api/openapi.yaml の paths。基点 URL からの相対)。 */
export const SPEED_PATHS = {
  pokemon: "api/speed/v1/pokemon",
  table: "api/speed/v1/table",
  position: "api/speed/v1/position",
} as const;

/** 通信できない・応答が読めない・エラー本文の形が不正なときの Web 側のコード(ADR-0604 §3)。 */
export const SPEED_UNAVAILABLE_CODE = "speed_unavailable";

/** speed_unavailable の失敗(自動の切り替え先は持たない。素早さはサーバーでしか計算しない)。 */
export function speedUnavailableError(): SpeedError {
  return { code: SPEED_UNAVAILABLE_CODE, message: speedClientText.unavailable };
}

/** サーバーのエラー本文({code, message})の形をしているかの型ガード(balanceClient.ts と同じ形)。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

/** speed_unavailable の失敗(通信できない・応答が読めない・エラー本文の形が不正)。 */
function unavailableResult<T>(): SpeedResult<T> {
  return { ok: false, error: speedUnavailableError() };
}

/**
 * speed API のクライアント実装(ADR-0604 §3)。応答をそのまま運び、Web で素早さを計算し直さない。
 * 通信・応答の失敗は speed_unavailable にし、自動の切り替え先へは移らない(ADR-0604 §3)。
 */
export function createSpeedClient(input: CreateSpeedClientInput): SpeedClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /** GET し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。 */
  async function getJson<T>(path: string, query?: string): Promise<SpeedResult<T>> {
    const url = `${baseUrl}${path}${query === undefined ? "" : `?${query}`}`;
    let response: Response;
    try {
      response = await fetchImpl(url, {
        method: "GET",
        headers: {
          "X-Device-Id": ids.deviceId,
          "X-Session-Id": ids.sessionId,
        },
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
    return { ok: true, value: parsed as T };
  }

  /** JSON を POST し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。 */
  async function postJson<T>(path: string, body: unknown): Promise<SpeedResult<T>> {
    let response: Response;
    try {
      response = await fetchImpl(`${baseUrl}${path}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Device-Id": ids.deviceId,
          "X-Session-Id": ids.sessionId,
        },
        body: JSON.stringify(body),
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
    return { ok: true, value: parsed as T };
  }

  return {
    pokemon() {
      return getJson(SPEED_PATHS.pokemon);
    },
    table(presets) {
      const query =
        presets === undefined ? undefined : new URLSearchParams({ presets: presets.join(",") }).toString();
      return getJson(SPEED_PATHS.table, query);
    },
    position(request) {
      return postJson(SPEED_PATHS.position, request);
    },
  };
}
