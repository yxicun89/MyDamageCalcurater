// JD5: judge API のクライアント createJudgeClient(ADR-0705 §3)。speed の speed/speedClient.ts と
// 同じ設計(ADR-0604 §3・ADR-0303 §1 を踏襲): 例外を投げず、常に JudgeResult<T> で返す。
// リクエスト・応答は openapi-typescript の生成型(judge.gen.ts)のまま運び、Web で判定し直さない
// (素早さ・行動順・確定数はすべて judge-svc が決める。ADR-0705 §3・§8)。
// 通信できない・応答が読めない・エラー本文の形が不正なときは judge_unavailable(Web 側のコード)にする。
// 基点 URL(api/config.ts の apiBaseUrl)・端末 ID/セッション ID(api/clientIds.ts の ClientIds)は
// Web レーン共通のヘルパーをそのまま使う(ADR-0705 §1。あちらは変更しない)。

import type { ClientIds } from "../api/clientIds";
import { judgeClientText } from "../i18n/ja";
import type { components } from "./judge.gen";

type Schemas = components["schemas"];

/** judge API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の judge_unavailable)。 */
export interface JudgeError {
  readonly code: string;
  readonly message: string;
}

/** judge API の呼び出しの成否(判別 union)。例外を投げず、常にこの形で返す。 */
export type JudgeResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: JudgeError };

/** createJudgeClient の引数(speedClient.ts の CreateSpeedClientInput と同じ形)。 */
export interface CreateJudgeClientInput {
  /** judge API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 計算・タイプバランス・素早さと同じ端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/** judge API のクライアント(ADR-0705 §3)。呼ぶ endpoint は判定の1本だけ。 */
export interface JudgeClient {
  /**
   * 自分1体と相手候補1〜6体の判定(services/judge/api/openapi.yaml の outspeedAndKo)。
   * 応答の matchups は request の defenders と同じ順序・同じ件数(ADR-0703 §2)。
   */
  outspeedAndKo(
    request: Schemas["OutspeedAndKoRequest"],
  ): Promise<JudgeResult<Schemas["OutspeedAndKoResponse"]>>;
}

/** judge API のパス(services/judge/api/openapi.yaml の paths。基点 URL からの相対)。 */
export const JUDGE_PATHS = {
  outspeedAndKo: "api/judge/v1/outspeed-and-ko",
} as const;

/** 通信できない・応答が読めない・エラー本文の形が不正なときの Web 側のコード(ADR-0705 §3)。 */
export const JUDGE_UNAVAILABLE_CODE = "judge_unavailable";

/** judge_unavailable の失敗(自動の切り替え先は持たない。判定はサーバーでしか行わない)。 */
export function judgeUnavailableError(): JudgeError {
  return { code: JUDGE_UNAVAILABLE_CODE, message: judgeClientText.unavailable };
}

/** サーバーのエラー本文({code, message})の形をしているかの型ガード(speedClient.ts と同じ形)。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

/** judge_unavailable の失敗(通信できない・応答が読めない・エラー本文の形が不正)。 */
function unavailableResult<T>(): JudgeResult<T> {
  return { ok: false, error: judgeUnavailableError() };
}

/**
 * judge API のクライアント実装(ADR-0705 §3)。応答をそのまま運び、Web で判定し直さない。
 * 通信・応答の失敗は judge_unavailable にし、自動の切り替え先へは移らない。
 */
export function createJudgeClient(input: CreateJudgeClientInput): JudgeClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  /** JSON を POST し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。 */
  async function postJson<T>(path: string, body: unknown): Promise<JudgeResult<T>> {
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
    outspeedAndKo(request) {
      return postJson(JUDGE_PATHS.outspeedAndKo, request);
    },
  };
}
