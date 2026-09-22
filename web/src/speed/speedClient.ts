// SP3: speed API のクライアント createSpeedClient(ADR-0604 §3)。balance の api/balanceClient.ts と
// 同じ設計(ADR-0303 §1・§5・§6 を踏襲): 例外を投げず、常に SpeedResult<T> で返す。
// リクエスト・応答は openapi-typescript の生成型(speed.gen.ts)のまま運び、Web で素早さを計算し直さない
// (実数値・段・位置はすべて speed-svc が決める。docs/speed-design.md §2)。
// 通信できない・応答が読めない・エラー本文の形が不正なときは speed_unavailable(Web 側のコード)にする。
// 基点 URL(api/config.ts の apiBaseUrl)・端末 ID/セッション ID(api/clientIds.ts の ClientIds)は
// Web レーン共通のヘルパーをそのまま使う(ADR-0604 §3。あちらは変更しない)。
//
// ここは SP3 のスタブで、まだ通信しない。契約(パス・ヘッダー・本文・エラーの解釈)は speedClient.test.ts が定める。

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

/**
 * speed API のクライアント(ADR-0604 §3)。
 *
 * SP3 のスタブ: まだ通信せず、どの呼び出しも「未実装」で落ちる。実装(implementer)は speedClient.test.ts が
 * 定める形(パス・ヘッダー・presets のクエリ・エラー本文の解釈)を満たすように、ここを埋める。
 */
export function createSpeedClient(input: CreateSpeedClientInput): SpeedClient {
  function notImplemented(): never {
    throw new Error(`not implemented: createSpeedClient(${input.baseUrl})`);
  }
  return {
    pokemon: notImplemented,
    table: notImplemented,
    position: notImplemented,
  };
}
