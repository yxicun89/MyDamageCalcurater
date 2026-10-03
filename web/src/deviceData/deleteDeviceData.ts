// P5-5d: 「この端末のデータを削除」の手順を担う純粋な関数(ADR-0318 §2。iOS の DeviceDataDeletionViewModel と同じ規則)。
// UI を持たない。record / team のクライアント(deleteDeviceData() だけを持つ DeviceDataDeleter)を注入して使う。

import type { components } from "../api/openapi.gen";
import { deviceDataText } from "../i18n/ja";

type Schemas = components["schemas"];

/** partial を繰り返す回数の上限(対象ごと。超えたら incomplete。ADR-0209 §5 の再実行を無限にしない)。 */
export const DEVICE_DATA_MAX_REQUESTS_PER_TARGET = 20;

/** 1回の削除要求の結果(recordClient / teamClient の Result と構造が同じ)。 */
export type DeviceDataDeleteOutcome =
  | { readonly ok: true; readonly value: Schemas["RecordDeletionResult"] | Schemas["TeamDeletionResult"] }
  | { readonly ok: false; readonly error: { readonly code: string; readonly message: string } };

/** recordClient / teamClient のうち、この手順が使う部分。 */
export interface DeviceDataDeleter {
  readonly deleteDeviceData: () => Promise<DeviceDataDeleteOutcome>;
}

export type DeviceDataTarget = "record" | "team";

/** 対象ごとの状態。pending = 未実行または中断、incomplete = 上限まで繰り返しても partial(失敗ではない)。 */
export type DeviceDataTargetState =
  | { readonly kind: "pending" }
  | { readonly kind: "completed" }
  | { readonly kind: "incomplete" }
  | { readonly kind: "failed"; readonly code: string };

export interface DeviceDataDeletionResult {
  readonly record: DeviceDataTargetState;
  readonly team: DeviceDataTargetState;
}

export interface DeviceDataProgress {
  /** 直近の応答が partial(続けて削除している最中)か。 */
  readonly lastResponsePartial: boolean;
}

export interface RunDeviceDataDeletionInput {
  readonly record: DeviceDataDeleter;
  readonly team: DeviceDataDeleter;
  /** 呼ぶ対象(省略 = 両方)。再試行では completed でない対象だけを渡す。 */
  readonly targets?: readonly DeviceDataTarget[];
  /** 再試行のとき、前回の結果(対象外の状態をここから引き継ぐ)。 */
  readonly previous?: DeviceDataDeletionResult;
  readonly maxRequestsPerTarget?: number;
  readonly signal?: AbortSignal;
  readonly onProgress?: (progress: DeviceDataProgress) => void;
}

const UNEXPECTED_ERROR_CODE = "unexpected_error";

/** 1対象を completed になるまで(上限まで)呼ぶ。通信エラーは自動で再送しない。 */
async function runTarget(
  deleter: DeviceDataDeleter,
  maxRequests: number,
  signal: AbortSignal | undefined,
  onProgress: ((progress: DeviceDataProgress) => void) | undefined,
): Promise<DeviceDataTargetState> {
  for (let count = 0; count < maxRequests; count += 1) {
    if (signal?.aborted === true) {
      return { kind: "pending" };
    }
    let outcome: DeviceDataDeleteOutcome;
    try {
      outcome = await deleter.deleteDeviceData();
    } catch {
      return { kind: "failed", code: UNEXPECTED_ERROR_CODE };
    }
    if (!outcome.ok) {
      return { kind: "failed", code: outcome.error.code };
    }
    const partial = outcome.value.status === "partial";
    onProgress?.({ lastResponsePartial: partial });
    if (!partial) {
      return { kind: "completed" };
    }
  }
  return signal?.aborted === true ? { kind: "pending" } : { kind: "incomplete" };
}

/** record → team の順に、独立して削除する(片方が失敗・未完了でももう片方は進める)。 */
export async function runDeviceDataDeletion(
  input: RunDeviceDataDeletionInput,
): Promise<DeviceDataDeletionResult> {
  const targets = input.targets ?? ["record", "team"];
  const maxRequests = input.maxRequestsPerTarget ?? DEVICE_DATA_MAX_REQUESTS_PER_TARGET;
  const next: { record: DeviceDataTargetState; team: DeviceDataTargetState } = {
    record: input.previous?.record ?? { kind: "pending" },
    team: input.previous?.team ?? { kind: "pending" },
  };
  for (const target of ["record", "team"] as const) {
    if (targets.includes(target)) {
      next[target] = await runTarget(input[target], maxRequests, input.signal, input.onProgress);
    }
  }
  return next;
}

export interface DeviceDataDeletionMessage {
  readonly tone: "success" | "partial" | "failure";
  readonly lines: readonly string[];
}

/** 結果を画面の文言(ADR-0209 §8)に直す。サーバーの message・code は出さない。 */
export function describeDeviceDataDeletion(result: DeviceDataDeletionResult): DeviceDataDeletionMessage {
  const recordFailed = result.record.kind === "failed";
  const teamFailed = result.team.kind === "failed";
  if (recordFailed || teamFailed) {
    const lines: string[] = [deviceDataText.failure];
    if (recordFailed && result.team.kind === "completed") {
      lines.push(deviceDataText.partlyDeleted(deviceDataText.teamLabel));
    }
    if (teamFailed && result.record.kind === "completed") {
      lines.push(deviceDataText.partlyDeleted(deviceDataText.recordLabel));
    }
    return { tone: "failure", lines };
  }
  if (result.record.kind === "completed" && result.team.kind === "completed") {
    return { tone: "success", lines: [deviceDataText.completed] };
  }
  return { tone: "partial", lines: [deviceDataText.partialNotice] };
}
