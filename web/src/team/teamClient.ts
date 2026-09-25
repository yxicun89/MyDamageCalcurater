// P5-5 PR-A1: 構築(team-svc)の CRUD を呼ぶクライアント(ADR-0309 §3)。
// speed/speedClient.ts・judge/judgeClient.ts と同じ設計(ADR-0303 §1・§5・§6 を踏襲):
// 例外を投げず、常に TeamResult<T>(判別 union)で返す。
//
// 型は **ルートの api/openapi.yaml から生成した api/openapi.gen.ts をそのまま使う**
// (balance/speed/judge は別サービス・別 openapi.yaml なのでレーンごとの *.gen.ts を持つが、
//  team・record はルートの契約に同居し、gateway 経由で計算 API と同じ基点 URL を使うため。ADR-0309 §3)。
// 通信できない・応答が読めない・エラー本文の形が不正なときは team_unavailable(Web 側のコード)にする。
//
// **このファイルは spec-writer 工程の骨格**(型・インターフェース・パス・エラーコードだけ)。
// 中身(fetch・応答の読み取り)は implementer が team/teamClient.test.ts を緑にする形で実装する。

import type { ClientIds } from "../api/clientIds";
import type { components } from "../api/openapi.gen";
import { teamClientText } from "../i18n/ja";

type Schemas = components["schemas"];

/** 構築 API のクライアントが返す失敗(サーバーの Error 封筒、または Web 側の team_unavailable)。 */
export interface TeamError {
  readonly code: string;
  readonly message: string;
}

/** 構築 API の呼び出しの成否(判別 union)。例外を投げず、常にこの形で返す。 */
export type TeamResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: TeamError };

/** createTeamClient の引数(speedClient.ts の CreateSpeedClientInput と同じ形)。 */
export interface CreateTeamClientInput {
  /** gateway の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() と同じ形)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  /** 計算・タイプバランス・素早さ・判定と同じ端末 ID・セッション ID(CLAUDE.md 技術規約)。 */
  readonly ids: ClientIds;
}

/**
 * 構築 API のクライアント(ADR-0309 §3)。
 * 端末 ID はパスに含めず、ヘッダー(X-Device-Id)で運ぶ(api/openapi.yaml の team-svc の契約)。
 */
export interface TeamClient {
  /** この端末の構築一覧(更新の新しい順。1件も無ければ空配列)。 */
  list(): Promise<TeamResult<Schemas["Team"][]>>;
  /** 構築を作る(201 で作られた Team を返す)。 */
  create(input: Schemas["TeamInput"]): Promise<TeamResult<Schemas["Team"]>>;
  /** 構築を1件取る(この端末が持たない ID は 404 not_found)。 */
  get(teamId: string): Promise<TeamResult<Schemas["Team"]>>;
  /** 構築を全置換する(200 で更新後の Team を返す)。 */
  update(teamId: string, input: Schemas["TeamInput"]): Promise<TeamResult<Schemas["Team"]>>;
  /**
   * 構築を消す(204。本文が無いので値は持たない)。名前が予約語にならないよう remove にする。
   * 同じ ID をもう一度消すと 404 not_found(冪等ではない。api/openapi.yaml の deleteTeam)。
   */
  remove(teamId: string): Promise<TeamResult<void>>;
}

/** 構築 API のパス(api/openapi.yaml の paths。基点 URL からの相対)。 */
export const TEAM_PATHS = {
  teams: "api/team/teams",
  team: (teamId: string): string => `api/team/teams/${encodeURIComponent(teamId)}`,
} as const;

/** 通信できない・応答が読めない・エラー本文の形が不正なときの Web 側のコード(ADR-0309 §3)。 */
export const TEAM_UNAVAILABLE_CODE = "team_unavailable";

/** team_unavailable の失敗(自動の切り替え先は持たない。構築はサーバーにしか無い)。 */
export function teamUnavailableError(): TeamError {
  return { code: TEAM_UNAVAILABLE_CODE, message: teamClientText.unavailable };
}

/** サーバーのエラー本文({code, message})の形をしているかの型ガード(speedClient.ts と同じ形)。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

/** team_unavailable の失敗(speedClient.ts の unavailableResult と同じ形)。 */
function unavailableResult<T>(): TeamResult<T> {
  return { ok: false, error: teamUnavailableError() };
}

/**
 * 構築 API のクライアント実装(ADR-0309 §3)。
 * 応答をそのまま運び、Web で並べ替え・整形をしない。通信・応答の失敗は team_unavailable にする。
 */
export function createTeamClient(input: CreateTeamClientInput): TeamClient {
  const { baseUrl, fetch: fetchImpl, ids } = input;

  function headers(withBody: boolean): Record<string, string> {
    const base: Record<string, string> = {
      "X-Device-Id": ids.deviceId,
      "X-Session-Id": ids.sessionId,
    };
    if (withBody) {
      base["Content-Type"] = "application/json";
    }
    return base;
  }

  /**
   * fetch を呼び、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。
   * `noBody` のとき(remove の 204)は本文を読まず、`{ok: true, value: undefined}` を返す
   * (204 の本文を読もうとして team_unavailable に落ちないため。ADR-0309 §3)。
   */
  async function request<T>(
    path: string,
    method: string,
    body: unknown,
    noBody: boolean,
  ): Promise<TeamResult<T>> {
    let response: Response;
    try {
      response = await fetchImpl(`${baseUrl}${path}`, {
        method,
        headers: headers(body !== undefined),
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      return unavailableResult();
    }
    if (noBody && response.status === 204) {
      return { ok: true, value: undefined as T };
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
    list() {
      return request<Schemas["Team"][]>(TEAM_PATHS.teams, "GET", undefined, false);
    },
    create(teamInput) {
      return request<Schemas["Team"]>(TEAM_PATHS.teams, "POST", teamInput, false);
    },
    get(teamId) {
      return request<Schemas["Team"]>(TEAM_PATHS.team(teamId), "GET", undefined, false);
    },
    update(teamId, teamInput) {
      return request<Schemas["Team"]>(TEAM_PATHS.team(teamId), "PUT", teamInput, false);
    },
    remove(teamId) {
      // T は remove() の宣言(Promise<TeamResult<void>>)から推論させる(void を明示の型引数にすると
      // @typescript-eslint/no-invalid-void-type に引っかかるため)。
      return request(TEAM_PATHS.team(teamId), "DELETE", undefined, true);
    },
  };
}
