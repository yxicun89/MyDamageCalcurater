// P4-12a: balance API のクライアント createBalanceClient(ADR-0303 §1・§5・§6)。
// リクエスト本文・応答は openapi-typescript の生成型(balance.gen.ts)のまま運び、倍率・集計を Web で
// 計算し直さない(ADR-0303 §1)。通信できない・応答が読めない・エラー本文の形が不正なときは balance_unavailable
// (Web 側のコード)にし、WASM のような自動の切り替え先を持たない(balance はサーバーでしか計算しない。ADR-0303 §6)。
// createApiEngine(api/apiEngine.ts)と同じ、例外を投げない設計。

import { balanceClientText } from "../i18n/ja";
import type { ClientIds } from "./clientIds";
import type { components } from "./balance.gen";

type Schemas = components["schemas"];

/** balance API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の balance_unavailable)。 */
export interface BalanceError {
  readonly code: string;
  readonly message: string;
}

/** balance API の呼び出しの成否(判別 union)。例外を投げず、常にこの形で返す。 */
export type BalanceResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: BalanceError };

/** createBalanceClient の引数。 */
export interface CreateBalanceClientInput {
  /** balance API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 計算と同じ端末 ID・セッション ID(ADR-0303 §5)。 */
  readonly ids: ClientIds;
}

/**
 * recommendations に送る本文の形(P4-12b)。契約(services/balance/api/openapi.yaml)では `limit` は必須では
 * ないが、既定値(10)を持つため openapi-typescript が必須の項目として生成する。limit を省いたときは
 * Web が既定値を決め打ちせず、`limit` キー自体を送らない(サーバーの既定に任せる。ADR-0303 §7)。
 */
type RecommendationsBody = Omit<Schemas["RecommendationsRequest"], "limit"> & {
  limit?: Schemas["RecommendationsRequest"]["limit"];
};

/** balance API のクライアント(ADR-0303 §1)。呼び出し側の配列を書き換えないので readonly で受ける。 */
export interface BalanceClient {
  analyze(
    members: readonly Schemas["AnalyzeRequestMember"][],
  ): Promise<BalanceResult<Schemas["AnalyzeResponse"]>>;
  coverage(
    members: readonly Schemas["CoverageRequestMember"][],
  ): Promise<BalanceResult<Schemas["CoverageResponse"]>>;
  /** P4-12b(ADR-0400): 仮想敵(threats)ごとの、自分のパーティとの相性診断。 */
  threats(
    members: readonly Schemas["ThreatsRequestPokemon"][],
    threats: readonly Schemas["ThreatsRequestPokemon"][],
  ): Promise<BalanceResult<Schemas["ThreatsResponse"]>>;
  /** P4-12b(ADR-0401): おすすめタイプ。limit を省くとサーバーの既定(10)になる。 */
  recommendations(
    members: readonly Schemas["RecommendationsRequestMember"][],
    limit?: number,
  ): Promise<BalanceResult<Schemas["RecommendationsResponse"]>>;
}

/** balance API のパス(services/balance/api/openapi.yaml の paths。基点 URL からの相対)。 */
const BALANCE_PATHS = {
  analyze: "api/balance/v1/team-balance/analyze",
  coverage: "api/balance/v1/team-balance/coverage",
  threats: "api/balance/v1/team-balance/threats",
  recommendations: "api/balance/v1/team-balance/recommendations",
} as const;

/** サーバーのエラー本文({code, message})の形をしているかの型ガード。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

/** balance_unavailable の失敗(通信できない・応答が読めない・エラー本文の形が不正。ADR-0303 §6)。 */
function unavailableError<T>(): BalanceResult<T> {
  return { ok: false, error: { code: "balance_unavailable", message: balanceClientText.unavailable } };
}

/**
 * balance API のクライアント実装(ADR-0303)。応答をそのまま運び、Web で倍率・集計を計算し直さない。
 * 通信・応答の失敗は balance_unavailable にし、自動の切り替え先(WASM 等)へは移らない(ADR-0303 §6)。
 */
export function createBalanceClient(input: CreateBalanceClientInput): BalanceClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /** JSON を POST し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。 */
  async function postJson<T>(path: string, body: unknown): Promise<BalanceResult<T>> {
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
      return unavailableError();
    }
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      return unavailableError();
    }
    if (!response.ok) {
      return isErrorBody(parsed)
        ? { ok: false, error: { code: parsed.code, message: parsed.message } }
        : unavailableError();
    }
    return { ok: true, value: parsed as T };
  }

  return {
    analyze(members) {
      const body: Schemas["AnalyzeRequest"] = { members: [...members] };
      return postJson(BALANCE_PATHS.analyze, body);
    },
    coverage(members) {
      const body: Schemas["CoverageRequest"] = { members: [...members] };
      return postJson(BALANCE_PATHS.coverage, body);
    },
    threats(members, threats) {
      const body: Schemas["ThreatsRequest"] = { members: [...members], threats: [...threats] };
      return postJson(BALANCE_PATHS.threats, body);
    },
    recommendations(members, limit) {
      const body: RecommendationsBody =
        limit === undefined ? { members: [...members] } : { members: [...members], limit };
      return postJson(BALANCE_PATHS.recommendations, body);
    },
  };
}
