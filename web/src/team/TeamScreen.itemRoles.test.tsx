// ADR-0326(ADR-0175 §4): 構築の編集の持ち物欄。構築は実際の対戦で持たせる持ち物を記録するので、ダメージ計算の役割では
// 絞らない(any。回復のきのみのような、計算に効かない持ち物も選べる)。メガストーンだけは単独の選択肢に出さず、
// メガ種族の固定中は日本語の「{基本種名}のメガストーン」を見せる(ストーンの nameJa は画面に出さない)。
// 確かめること:
//   - 非メガの持ち物欄は「(なし)」+ メガストーンを除いた全件(役割の無い持ち物も含む。マスタの順)
//   - まだ解決していないメガ種族のストーン(種族から導く集合では判別できない)も isMegaStone で外れる
//   - メガ種族の固定中は「{基本種名}のメガストーン」、古い保存データを直した通知も同じ名前で出す
//   - 非メガの現在値がメガストーンの古いデータは、値を保ったまま「メガストーン」と表示する(英語のストーン名を出さない)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { megaItemText, teamMemberText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeTeamClient, flush, lastCall } from "../test/fakeTeamClient";
import {
  DEF_ITEM,
  EXPECTED_ANY_ITEMS,
  NO_ROLE_ITEM,
  ROLE_MEGA_FIRE_STONE,
  UNRESOLVED_MEGA_STONE,
  withItemRoles,
} from "../test/itemRolesMaster";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_FIRE_STONE_LABEL,
  UNNAMED_MEGA_STONE_LABEL,
} from "../test/megaMaster";
import { TeamScreen } from "./TeamScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = withItemRoles(await exampleMasterSource.load());
});

const NORMAL_KEY = "9001-000";
const TEAM_NAME = "テスト役割構築";

function member(speciesKey: string, itemId: string | null): Schemas["TeamMember"] {
  return {
    speciesKey,
    moveIds: ["examplemovetackle"],
    itemId,
    abilityId: "exampleabilitynone",
    natureId: "example-nature-atk",
    sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
    teraType: null,
  };
}

function team(members: Schemas["TeamMember"][]): Schemas["Team"] {
  return {
    id: "66666666-6666-4666-8666-666666666666",
    name: TEAM_NAME,
    members,
    createdAt: "2026-10-03T12:00:00Z",
    updatedAt: "2026-10-03T12:00:00Z",
  };
}

async function openFirstMember(
  members: Schemas["TeamMember"][],
): Promise<{ user: UserEvent; first: HTMLElement }> {
  const client = createFakeTeamClient();
  const user = userEvent.setup();
  render(<TeamScreen teamClient={client} master={master} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [team(members)] });
  });
  await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(TEAM_NAME) }));
  const editor = screen.getByRole("region", { name: teamMemberText.editorLabel(TEAM_NAME) });
  const first = within(editor).getByRole("group", { name: teamMemberText.memberLegend(1) });
  return { user, first };
}

const itemSelect = (element: HTMLElement) =>
  within(element).getByRole("combobox", { name: teamMemberText.itemLabel });

function optionLabels(element: HTMLElement): string[] {
  return within(element)
    .getAllByRole("option")
    .map((option) => option.textContent.trim());
}

describe("非メガの持ち物欄は役割で絞らない(メガストーンだけ外す)", () => {
  test("「(なし)」+ メガストーンを除いた全件(役割の無い持ち物も含む)", async () => {
    const { first } = await openFirstMember([member(NORMAL_KEY, null)]);

    const labels = optionLabels(itemSelect(first));
    expect(labels).toEqual([teamMemberText.itemNone, ...EXPECTED_ANY_ITEMS.map((item) => item.nameJa)]);
    expect(labels).toContain(NO_ROLE_ITEM.nameJa);
  });

  test("まだ解決していないメガ種族のストーンも出ない(isMegaStone で判別)", async () => {
    const { first } = await openFirstMember([member(NORMAL_KEY, null)]);
    expect(optionLabels(itemSelect(first))).not.toContain(UNRESOLVED_MEGA_STONE.nameJa);
    expect(optionLabels(itemSelect(first))).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
  });

  test("計算に効かない持ち物を持つメンバーも、そのまま選ばれている", async () => {
    const { first } = await openFirstMember([member(NORMAL_KEY, NO_ROLE_ITEM.id)]);
    expect(itemSelect(first)).toHaveValue(NO_ROLE_ITEM.id);
    expect(itemSelect(first)).toHaveDisplayValue(NO_ROLE_ITEM.nameJa);
  });
});

describe("メガ種族の固定中の表示", () => {
  test("「{基本種名}のメガストーン」(ストーンの nameJa ではない)、理由は aria-describedby", async () => {
    const { first } = await openFirstMember([member(MEGA_FIRE.key, MEGA_FIRE_STONE.id)]);

    const item = itemSelect(first);
    expect(item).toBeDisabled();
    expect(item).toHaveValue(MEGA_FIRE_STONE.id);
    expect(item).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(optionLabels(item)).not.toContain(MEGA_FIRE_STONE.nameJa);
    expect(item).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("古い保存データを直した通知も「{基本種名}のメガストーン」で出す", async () => {
    const { first } = await openFirstMember([member(MEGA_FIRE.key, DEF_ITEM.id)]);

    await waitFor(() => {
      expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    });
    const notice = within(first).getByText(megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL));
    expect(notice).toHaveAttribute("role", "status");
    expect(within(first).queryByText(megaItemText.correctedNotice(MEGA_FIRE_STONE.nameJa))).toBeNull();
  });

  test("非メガがメガストーンを持つ古いデータは、値を保ったまま「メガストーン」と表示する", async () => {
    const { first } = await openFirstMember([member(NORMAL_KEY, MEGA_FIRE_STONE.id)]);

    expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    expect(itemSelect(first)).toHaveDisplayValue(UNNAMED_MEGA_STONE_LABEL);
  });
});
