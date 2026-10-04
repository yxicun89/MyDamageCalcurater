// テスト専用(ADR-0331): 調整画面のテストが使う fake の AdjustClient。呼び出しを記録し、テストがあとから応答を返す。
// adjust/AdjustScreen.test.tsx の同名の関数と同じ振る舞い(既存のテストは変えずに、新しいテストからはここを使う)。

import { act } from "@testing-library/react";
import type { AdjustClient, AdjustResult, LearnersPage } from "../adjust/adjustClient";

export type AdjustMethod = keyof AdjustClient;

export interface RecordedAdjustCall {
  readonly method: AdjustMethod;
  /** 呼び出しの引数(signal を除く。structuredClone 済み)。 */
  readonly args: readonly unknown[];
  readonly signal: AbortSignal | undefined;
  resolve(result: AdjustResult<unknown>): void;
}

export interface FakeAdjustClient extends AdjustClient {
  readonly calls: RecordedAdjustCall[];
}

export function createFakeAdjustClient(): FakeAdjustClient {
  const calls: RecordedAdjustCall[] = [];
  // 応答の型は操作ごとに違うが、テストが操作に合う応答を返す(AdjustResult<never> はどの AdjustResult<T> にも代入できる)。
  function record(
    method: AdjustMethod,
    args: readonly unknown[],
    signal: AbortSignal | undefined,
  ): Promise<AdjustResult<never>> {
    return new Promise((resolve) => {
      calls.push({
        method,
        args: structuredClone(args),
        signal,
        resolve: (result) => {
          resolve(result as AdjustResult<never>);
        },
      });
    });
  }
  return {
    calls,
    indices: (request, signal) => record("indices", [request], signal),
    minSpToKo: (request, signal) => record("minSpToKo", [request], signal),
    minSpToSurvive: (request, signal) => record("minSpToSurvive", [request], signal),
    allocation: (request, signal) => record("allocation", [request], signal),
    goals: (request, signal) => record("goals", [request], signal),
    moveLearners: (moveId: string, page: LearnersPage, signal?: AbortSignal) =>
      record("moveLearners", [moveId, page], signal),
  };
}

export function callsOf(client: FakeAdjustClient, method: AdjustMethod): RecordedAdjustCall[] {
  return client.calls.filter((call) => call.method === method);
}

export function lastCallOf(client: FakeAdjustClient, method: AdjustMethod): RecordedAdjustCall {
  const call = callsOf(client, method).at(-1);
  if (call === undefined) {
    throw new Error(`${method} が呼ばれていない`);
  }
  return call;
}

/** 最後の呼び出しの request(第1引数)。 */
export function lastRequestOf(client: FakeAdjustClient, method: AdjustMethod): unknown {
  return lastCallOf(client, method).args[0];
}

export async function respond(call: RecordedAdjustCall, result: AdjustResult<unknown>): Promise<void> {
  await act(async () => {
    call.resolve(result);
    await Promise.resolve();
  });
}
