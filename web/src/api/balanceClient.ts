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

// ---- 応答の実行時検証(issue 67、ADR-0303 §6 追記) ----
//
// balance の応答はそのまま表示に使う(Web で倍率・集計を計算し直さない)ので、検査するのは「画面が
// たどる形」まで: 応答がオブジェクトであること、契約で必須の最上位フィールドが存在し配列であるべき
// ところが配列であること、配列の要素がオブジェクトで、要素の必須の配列フィールドが配列であること。
// leaf のスカラー(表示にそのまま出る文字列・数値・真偽値)と列挙の値までは検査しない(契約の
// 二重管理を避けるため)。

/** オブジェクト(配列・null を除く)かどうか。 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** 配列であること(要素の中身までは見ない。leaf のスカラーの配列に使う)。 */
function isUnknownArray(value: unknown): value is unknown[] {
  return Array.isArray(value);
}

/** 配列で、すべての要素が guard を満たすか。 */
function isArrayOf<T>(value: unknown, guard: (item: unknown) => item is T): value is T[] {
  return Array.isArray(value) && value.every(guard);
}

/** analyze の応答 members[]: defense・types が配列であること。 */
function isAnalyzeMember(value: unknown): value is Schemas["AnalyzeResponse"]["members"][number] {
  return isRecord(value) && Array.isArray(value.defense) && Array.isArray(value.types);
}

/** analyze の応答: members・teamSummary が存在し配列であること(要素はオブジェクト)。 */
function isAnalyzeResponse(value: unknown): value is Schemas["AnalyzeResponse"] {
  return (
    isRecord(value) && isArrayOf(value.members, isAnalyzeMember) && isArrayOf(value.teamSummary, isRecord)
  );
}

/** coverage の応答 members[]: coverage・moveIds・attackTypes が配列であること。 */
function isCoverageMember(value: unknown): value is Schemas["CoverageResponse"]["members"][number] {
  return (
    isRecord(value) &&
    Array.isArray(value.coverage) &&
    Array.isArray(value.moveIds) &&
    Array.isArray(value.attackTypes)
  );
}

/** coverage の応答: members・teamCoverage が存在し配列であること(要素はオブジェクト)。 */
function isCoverageResponse(value: unknown): value is Schemas["CoverageResponse"] {
  return (
    isRecord(value) && isArrayOf(value.members, isCoverageMember) && isArrayOf(value.teamCoverage, isRecord)
  );
}

/** threats の応答 threats[]: matchups・attackTypes が配列であること。 */
function isThreatResult(value: unknown): value is Schemas["ThreatsResponse"]["threats"][number] {
  return isRecord(value) && Array.isArray(value.matchups) && Array.isArray(value.attackTypes);
}

/** threats の応答: threats が存在し配列であること(要素はオブジェクト)。 */
function isThreatsResponse(value: unknown): value is Schemas["ThreatsResponse"] {
  return isRecord(value) && isArrayOf(value.threats, isThreatResult);
}

/** recommendations の応答 candidates[]: types・defenseCovered・offenseCovered・pokemon が配列であること。 */
function isTypeCandidate(value: unknown): value is Schemas["RecommendationsResponse"]["candidates"][number] {
  return (
    isRecord(value) &&
    Array.isArray(value.types) &&
    Array.isArray(value.defenseCovered) &&
    Array.isArray(value.offenseCovered) &&
    Array.isArray(value.pokemon)
  );
}

/** recommendations の応答 abilityOptions[]: pokemon が配列であること。 */
function isAbilityOption(
  value: unknown,
): value is Schemas["RecommendationsResponse"]["abilityOptions"][number] {
  return isRecord(value) && Array.isArray(value.pokemon);
}

/**
 * recommendations の応答: defenseHoles・offenseHoles(leaf のタイプ名の配列。要素は見ない)・
 * candidates・abilityOptions が存在し、それぞれ配列であること。
 */
function isRecommendationsResponse(value: unknown): value is Schemas["RecommendationsResponse"] {
  return (
    isRecord(value) &&
    isUnknownArray(value.defenseHoles) &&
    isUnknownArray(value.offenseHoles) &&
    isArrayOf(value.candidates, isTypeCandidate) &&
    isArrayOf(value.abilityOptions, isAbilityOption)
  );
}

/**
 * balance API のクライアント実装(ADR-0303)。応答をそのまま運び、Web で倍率・集計を計算し直さない。
 * 通信・応答の失敗は balance_unavailable にし、自動の切り替え先(WASM 等)へは移らない(ADR-0303 §6)。
 */
export function createBalanceClient(input: CreateBalanceClientInput): BalanceClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /**
   * JSON を POST し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。
   * guard(issue 67、ADR-0303 §6 追記): 2xx の本文を実行時に検証し、契約外なら balance_unavailable
   * にする(画面がたどる形だけを検査。ADR-0303 §6)。
   */
  async function postJson<T>(
    path: string,
    body: unknown,
    guard: (value: unknown) => value is T,
  ): Promise<BalanceResult<T>> {
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
    if (!guard(parsed)) {
      return unavailableError();
    }
    return { ok: true, value: parsed };
  }

  return {
    analyze(members) {
      const body: Schemas["AnalyzeRequest"] = { members: [...members] };
      return postJson(BALANCE_PATHS.analyze, body, isAnalyzeResponse);
    },
    coverage(members) {
      const body: Schemas["CoverageRequest"] = { members: [...members] };
      return postJson(BALANCE_PATHS.coverage, body, isCoverageResponse);
    },
    threats(members, threats) {
      const body: Schemas["ThreatsRequest"] = { members: [...members], threats: [...threats] };
      return postJson(BALANCE_PATHS.threats, body, isThreatsResponse);
    },
    recommendations(members, limit) {
      const body: RecommendationsBody =
        limit === undefined ? { members: [...members] } : { members: [...members], limit };
      return postJson(BALANCE_PATHS.recommendations, body, isRecommendationsResponse);
    },
  };
}
