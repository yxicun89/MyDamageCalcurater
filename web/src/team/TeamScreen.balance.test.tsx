// ADR-0421 段階 2: 構築編集の中の「タイプバランス」の節。BalanceClient は fake、TeamClient も fake、マスタは架空の例データ。
// 確かめること:
//   B-1 保存済みの 1 体で analyze を呼ぶ({pokemonId, abilityId} だけ。空の枠は送らない)=1 体からでも出る
//   B-2 ポケモンが 0 体なら balance を呼ばず、案内の一言を出す
//   B-3 balance が失敗しても(エラーは節の中の alert)構築の編集・保存は止まらない
//   B-4 balanceClient を渡さない画面では節を出さない(既存の構築画面は変わらない)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { teamBalanceText, teamMemberText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeTeamClient, flush, lastCall } from "../test/fakeTeamClient";
import { MAX_TEAM_MEMBERS, TeamScreen } from "./TeamScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const FIRE: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  moveIds: ["examplemovetackle"],
  itemId: null,
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: null,
};

function teamOf(id: string, members: Schemas["TeamMember"][]): Schemas["Team"] {
  return {
    id,
    name: "名称未設定",
    members,
    createdAt: "2026-10-01T09:00:00Z",
    updatedAt: "2026-10-01T09:00:00Z",
  };
}

interface RecordedBalanceClient extends BalanceClient {
  readonly analyzeCalls: unknown[];
  readonly allCalls: string[];
}

function createRecordingBalanceClient(failWith: string | null): RecordedBalanceClient {
  const analyzeCalls: unknown[] = [];
  const allCalls: string[] = [];
  const failure: BalanceResult<never> = {
    ok: false,
    error: { code: failWith ?? "", message: "テストの失敗" },
  };
  return {
    analyzeCalls,
    allCalls,
    analyze(members) {
      allCalls.push("analyze");
      analyzeCalls.push(structuredClone(members));
      return Promise.resolve(failure);
    },
    coverage() {
      allCalls.push("coverage");
      return Promise.resolve(failure);
    },
    threats() {
      allCalls.push("threats");
      return Promise.resolve(failure);
    },
    recommendations() {
      allCalls.push("recommendations");
      return Promise.resolve(failure);
    },
  };
}

async function openEditor(members: Schemas["TeamMember"][], balanceClient: BalanceClient | undefined) {
  const client = createFakeTeamClient();
  const user = userEvent.setup();
  const target = teamOf("33333333-3333-4333-8333-333333333333", members);
  render(<TeamScreen teamClient={client} balanceClient={balanceClient} master={master} reloadToken={0} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [target] });
  });
  await user.click(await screen.findByRole("button", { name: teamMemberText.editLabel("構築 1") }));
  return { client, user, target };
}

describe("構築編集の中のタイプバランス(ADR-0421)", () => {
  test("B-1: 1 体だけでも analyze を呼ぶ(空の枠は送らない)", async () => {
    const balance = createRecordingBalanceClient("balance_unavailable");
    await openEditor([FIRE], balance);
    expect(screen.getByRole("heading", { name: teamBalanceText.heading })).toBeInTheDocument();
    await waitFor(() => {
      expect(balance.analyzeCalls).toHaveLength(1);
    });
    expect(balance.analyzeCalls[0]).toEqual([{ pokemonId: FIRE.speciesKey, abilityId: FIRE.abilityId }]);
    expect(MAX_TEAM_MEMBERS).toBeGreaterThan(1);
  });

  test("B-2: ポケモンが 0 体なら balance を呼ばず、案内を出す", async () => {
    const balance = createRecordingBalanceClient("balance_unavailable");
    await openEditor([], balance);
    expect(screen.getByText(teamBalanceText.empty)).toBeInTheDocument();
    expect(balance.allCalls).toEqual([]);
  });

  test("B-3: balance が失敗しても節の中に alert が出るだけで、保存は押せる", async () => {
    const balance = createRecordingBalanceClient("balance_unavailable");
    await openEditor([FIRE], balance);
    const section = screen.getByRole("region", { name: teamBalanceText.heading });
    await waitFor(() => {
      expect(within(section).getAllByRole("alert").length).toBeGreaterThan(0);
    });
    expect(screen.getByRole("button", { name: teamMemberText.saveLabel })).toBeEnabled();
    // 編集画面全体の alert は保存エラー用なので、節の外には出ない。
    expect(screen.queryAllByRole("alert").filter((element) => !section.contains(element))).toHaveLength(0);
  });

  test("B-4: balanceClient が無い画面では節を出さない", async () => {
    await openEditor([FIRE], undefined);
    expect(screen.queryByRole("heading", { name: teamBalanceText.heading })).not.toBeInTheDocument();
  });
});
