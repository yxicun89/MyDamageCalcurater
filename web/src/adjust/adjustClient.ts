// AJ6: 調整 API のクライアント createAdjustClient(ADR-0319 §3)。judge/judgeClient.ts と同じ設計
// (例外を投げず、常に AdjustResult<T> で返す)に、api/apiEngine.ts と同じ AbortSignal の扱いを足したもの。
// リクエスト・応答は openapi-typescript の生成型(api/openapi.gen.ts)のまま運び、Web で計算し直さない
// (指数・16n・最小 SP・配分はすべて calc-svc の engine が決める。ADR-0150・ADR-0250)。
//
// 呼ぶ endpoint(api/openapi.yaml が正):
//   POST api/calc/adjust/indices            adjustIndices
//   POST api/calc/adjust/min-sp-to-ko       adjustMinSpToKo
//   POST api/calc/adjust/min-sp-to-survive  adjustMinSpToSurvive
//   POST api/calc/adjust/allocation         adjustAllocation
//   POST api/calc/adjust/goals              adjustGoals(ADR-0331。段階 A はスタブ)
//   GET  api/pokedex/moves/{key}/learners   listMoveLearners(?limit=&offset=)
//
// 失敗の写像(ADR-0319 §3):
//   - HTTP エラーで本文が {code, message} → その code・message をそのまま運ぶ(画面は message を出さない)
//   - 通信できない・応答が JSON でない・エラー本文の形が不正 → adjust_unavailable(Web 側のコード)
//   - signal が abort 済み(呼ぶ前・通信中・本文の読み取り中) → request_aborted(engine/types.ts の REQUEST_ABORTED_CODE)

import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { REQUEST_ABORTED_CODE } from "../engine/types";
import { adjustClientText } from "../i18n/ja";

type Schemas = components["schemas"];

/** 調整 API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の adjust_unavailable / request_aborted)。 */
export interface AdjustError {
  readonly code: string;
  readonly message: string;
}

/** 調整 API の呼び出しの成否(判別 union)。例外を投げず、常にこの形で返す。 */
export type AdjustResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: AdjustError };

/** createAdjustClient の引数(judgeClient.ts の CreateJudgeClientInput と同じ形)。 */
export interface CreateAdjustClientInput {
  /** API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 他の API と同じ端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/** listMoveLearners の1ページの指定(契約: limit 1..200、offset 0..10000)。 */
export interface LearnersPage {
  readonly limit: number;
  readonly offset: number;
}

/** 調整タブが使う API のクライアント(ADR-0319 §3)。 */
export interface AdjustClient {
  indices(
    request: Schemas["AdjustIndicesRequest"],
    signal?: AbortSignal,
  ): Promise<AdjustResult<Schemas["AdjustIndicesResult"]>>;
  minSpToKo(
    request: Schemas["AdjustSearchRequest"],
    signal?: AbortSignal,
  ): Promise<AdjustResult<Schemas["AdjustKOResult"]>>;
  minSpToSurvive(
    request: Schemas["AdjustSearchRequest"],
    signal?: AbortSignal,
  ): Promise<AdjustResult<Schemas["AdjustSurviveResult"]>>;
  allocation(
    request: Schemas["AdjustAllocationRequest"],
    signal?: AbortSignal,
  ): Promise<AdjustResult<Schemas["AdjustAllocationResult"]>>;
  /** 相手ごとの目標をすべて満たす最小の振り方(ADR-0331)。 */
  goals(
    request: Schemas["AdjustGoalsRequest"],
    signal?: AbortSignal,
  ): Promise<AdjustResult<Schemas["AdjustGoalsResult"]>>;
  /** 技 moveId を覚えるポケモンの1ページ(図鑑番号・フォルム番号の昇順。返った件数が limit 未満なら最後)。 */
  moveLearners(
    moveId: string,
    page: LearnersPage,
    signal?: AbortSignal,
  ): Promise<AdjustResult<readonly Schemas["SpeciesSummary"][]>>;
}

/** 調整 API の POST のパス(api/openapi.yaml の paths。基点 URL からの相対)。 */
export const ADJUST_PATHS = {
  indices: "api/calc/adjust/indices",
  minSpToKo: "api/calc/adjust/min-sp-to-ko",
  minSpToSurvive: "api/calc/adjust/min-sp-to-survive",
  allocation: "api/calc/adjust/allocation",
} as const;

/** adjustGoals のパス(ADR-0331。ADJUST_PATHS とは別に置き、既存の4操作の表を変えない)。 */
export const ADJUST_GOALS_PATH = "api/calc/adjust/goals";

/** 通信できない・応答が読めない・エラー本文の形が不正なときの Web 側のコード(ADR-0319 §3)。 */
export const ADJUST_UNAVAILABLE_CODE = "adjust_unavailable";

/**
 * listMoveLearners のパス(基点 URL からの相対・クエリ込み)。技の ID は encodeURIComponent する。
 * 例: moveLearnersPath("test-move", {limit: 50, offset: 0}) → "api/pokedex/moves/test-move/learners?limit=50&offset=0"
 */
export function moveLearnersPath(moveId: string, page: LearnersPage): string {
  return `api/pokedex/moves/${encodeURIComponent(moveId)}/learners?limit=${String(page.limit)}&offset=${String(page.offset)}`;
}

/** サーバーのエラー本文({code, message})の形をしているかの型ガード。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

function isAborted(signal: AbortSignal | undefined): boolean {
  return signal?.aborted === true;
}

function failure<T>(code: string, message: string): AdjustResult<T> {
  return { ok: false, error: { code, message } };
}

function abortedResult<T>(): AdjustResult<T> {
  return failure(REQUEST_ABORTED_CODE, adjustClientText.aborted);
}

function unavailableResult<T>(): AdjustResult<T> {
  return failure(ADJUST_UNAVAILABLE_CODE, adjustClientText.unavailable);
}

/** 調整 API のクライアント実装(ADR-0319 §3)。 */
export function createAdjustClient(input: CreateAdjustClientInput): AdjustClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /** 1回の呼び出し。例外を投げず、abort は request_aborted、通信・形の失敗は adjust_unavailable にする。 */
  async function send<T>(
    path: string,
    init: RequestInit,
    signal: AbortSignal | undefined,
  ): Promise<AdjustResult<T>> {
    if (isAborted(signal)) {
      return abortedResult();
    }
    const requestInit: RequestInit = {
      ...init,
      headers: {
        ...(init.body === undefined ? {} : { "Content-Type": "application/json" }),
        "X-Device-Id": ids.deviceId,
        "X-Session-Id": ids.sessionId,
      },
    };
    if (signal !== undefined) {
      requestInit.signal = signal;
    }
    let response: Response;
    try {
      response = await fetchImpl(`${baseUrl}${path}`, requestInit);
    } catch {
      return isAborted(signal) ? abortedResult() : unavailableResult();
    }
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      return isAborted(signal) ? abortedResult() : unavailableResult();
    }
    if (!response.ok) {
      return isErrorBody(parsed) ? failure(parsed.code, parsed.message) : unavailableResult();
    }
    return { ok: true, value: parsed as T };
  }

  function post<T>(path: string, request: unknown, signal?: AbortSignal): Promise<AdjustResult<T>> {
    return send<T>(path, { method: "POST", body: JSON.stringify(request) }, signal);
  }

  return {
    indices: (request, signal) => post(ADJUST_PATHS.indices, request, signal),
    minSpToKo: (request, signal) => post(ADJUST_PATHS.minSpToKo, request, signal),
    minSpToSurvive: (request, signal) => post(ADJUST_PATHS.minSpToSurvive, request, signal),
    allocation: (request, signal) => post(ADJUST_PATHS.allocation, request, signal),
    // スタブ(ADR-0331 段階 A で post(ADJUST_GOALS_PATH, ...) にする。テストは adjustClient.goals.test.ts)。
    goals: () => Promise.resolve(unavailableResult()),
    moveLearners: (moveId, page, signal) => send(moveLearnersPath(moveId, page), { method: "GET" }, signal),
  };
}
