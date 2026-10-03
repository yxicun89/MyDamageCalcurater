// テスト専用(P5-5b): 呼び出しを記録し、テストが応答を流す fake の TeamClient。
// team/TeamScreen.test.tsx(PR-A1)の fake と同じ形(あちらは変更しない)。新しいテストはこちらを使う。

import { act } from "@testing-library/react";
import type { components } from "../api/openapi.gen";
import type { TeamClient, TeamResult } from "../team/teamClient";

type Schemas = components["schemas"];

export interface PendingCall<A, T> {
  readonly args: A;
  resolve(result: TeamResult<T>): void;
}

export interface FakeTeamClient extends TeamClient {
  readonly listCalls: PendingCall<null, Schemas["Team"][]>[];
  readonly createCalls: PendingCall<Schemas["TeamInput"], Schemas["Team"]>[];
  readonly getCalls: PendingCall<string, Schemas["Team"]>[];
  readonly updateCalls: PendingCall<
    { readonly teamId: string; readonly input: Schemas["TeamInput"] },
    Schemas["Team"]
  >[];
  readonly removeCalls: PendingCall<string, void>[];
}

export function createFakeTeamClient(): FakeTeamClient {
  const listCalls: FakeTeamClient["listCalls"] = [];
  const createCalls: FakeTeamClient["createCalls"] = [];
  const getCalls: FakeTeamClient["getCalls"] = [];
  const updateCalls: FakeTeamClient["updateCalls"] = [];
  const removeCalls: FakeTeamClient["removeCalls"] = [];
  return {
    listCalls,
    createCalls,
    getCalls,
    updateCalls,
    removeCalls,
    list: () => new Promise((resolve) => listCalls.push({ args: null, resolve })),
    create: (input) => new Promise((resolve) => createCalls.push({ args: structuredClone(input), resolve })),
    get: (teamId) => new Promise((resolve) => getCalls.push({ args: teamId, resolve })),
    update: (teamId, input) =>
      new Promise((resolve) =>
        updateCalls.push({ args: { teamId, input: structuredClone(input) }, resolve }),
      ),
    remove: (teamId) => new Promise((resolve) => removeCalls.push({ args: teamId, resolve })),
  };
}

/** 1回だけ解決を流す。 */
export async function flush(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

export function lastCall<A, T>(calls: readonly PendingCall<A, T>[], name: string): PendingCall<A, T> {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}
