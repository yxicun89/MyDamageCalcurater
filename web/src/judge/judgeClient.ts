// JD5: judge API のクライアント createJudgeClient(ADR-0705 §3)。speed の speed/speedClient.ts と
// 同じ設計(ADR-0604 §3・ADR-0303 §1 を踏襲): 例外を投げず、常に JudgeResult<T> で返す。
// リクエスト・応答は openapi-typescript の生成型(judge.gen.ts)のまま運び、Web で判定し直さない
// (素早さ・行動順・確定数はすべて judge-svc が決める。ADR-0705 §3・§8)。
// 通信できない・応答が読めない・エラー本文の形が不正なときは judge_unavailable(Web 側のコード)にする。
// 基点 URL(api/config.ts の apiBaseUrl)・端末 ID/セッション ID(api/clientIds.ts の ClientIds)は
// Web レーン共通のヘルパーをそのまま使う(ADR-0705 §1。あちらは変更しない)。
//
// **この時点では未実装のスタブ**(JD5 は spec-writer が受け入れ条件とテストを先に書く段階)。
// 実装は judgeClient.test.ts を通す形で implementer が入れる。型・パス・コードは契約(ADR-0705 §3)の正。

import type { ClientIds } from "../api/clientIds";
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

/**
 * judge API のクライアント実装(ADR-0705 §3)。
 * **未実装**: judgeClient.test.ts の受け入れ条件を満たす実装を implementer が入れる。
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- 未実装のスタブ(implementer が input を使う)
export function createJudgeClient(input: CreateJudgeClientInput): JudgeClient {
  throw new Error("createJudgeClient is not implemented yet (JD5)");
}
