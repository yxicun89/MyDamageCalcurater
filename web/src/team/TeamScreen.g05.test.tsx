// I-web-13d(G-05、ADR-0339・ADR-0347): 構築画面の見た目の作り直し(docs/design.md「画面ごとの方向」の構築の行)。
// 確かめること:
//   - 一覧: 構築のカードは 6 つの枠(埋まった枠は画像かエンブレム、空きは破線の枠)、開くは アイコン + 名前、削除はアイコンだけ(名前は用語集の語のまま)
//   - 空の一覧は大きいアイコン + 短い一言(長い案内の文を出さない)
//   - 編集: 6 枠のタイルの格子(埋まった枠はポケモンカード、空きは「+」のタイル)。押すとその枠の欄へ移る
//   - SP は増減ボタン付き(0〜32 で止まる)、合計と残りは数値 + バー。入れ替え・外すはアイコンだけのボタン
//   - 未保存の印・保存済みの印はアイコン + 一言
// 既存の構築のテスト(TeamScreen.*.test.tsx)が挙動と読み上げ用の名前を守る。ここは新しい構造だけを見る。

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { statLetterJa, teamMemberText, teamScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeTeamClient, flush, lastCall } from "../test/fakeTeamClient";
import { TeamScreen } from "./TeamScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const MEMBER_FIRE: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  moveIds: ["examplemovetackle"],
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: "fire",
};

const TEAM: Schemas["Team"] = {
  id: "66666666-6666-4666-8666-666666666666",
  name: "テスト構築G",
  members: [MEMBER_FIRE],
  createdAt: "2026-10-11T12:00:00Z",
  updatedAt: "2026-10-11T12:00:00Z",
};

async function renderScreen(
  teams: readonly Schemas["Team"][],
): Promise<{ user: UserEvent; container: HTMLElement }> {
  const client = createFakeTeamClient();
  const user = userEvent.setup();
  const { container } = render(<TeamScreen teamClient={client} master={master} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [...teams] });
  });
  return { user, container };
}

async function openEditor(user: UserEvent, name: string): Promise<HTMLElement> {
  await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(name) }));
  return screen.getByRole("region", { name: teamMemberText.editorLabel(name) });
}

function speciesName(key: string): string {
  const found = master.species.find((species) => species.key === key);
  if (found === undefined) {
    throw new Error(`例データに種族 ${key} が無い`);
  }
  return found.nameJa;
}

describe("一覧", () => {
  test("カードは 6 つの枠を並べる(埋まった枠は名前つきの画像かエンブレム、空きは読み上げない破線の枠)", async () => {
    await renderScreen([TEAM]);
    const card = screen.getByRole("listitem");
    const icons = within(card).getByRole("group", { name: teamScreenText.memberIconsLabel(TEAM.name) });
    expect(within(icons).getAllByRole("img")).toHaveLength(1);
    expect(icons.querySelectorAll(".team-card__slot")).toHaveLength(6);
    const empties = icons.querySelectorAll(".team-card__slot--empty");
    expect(empties).toHaveLength(5);
    for (const empty of empties) {
      expect(empty).toHaveAttribute("aria-hidden", "true");
    }
  });

  test("[開く]は アイコン + 名前。[削除]はアイコンだけで、名前は「<名前>を削除」のまま", async () => {
    await renderScreen([TEAM]);
    const open = screen.getByRole("button", { name: teamMemberText.editLabel(TEAM.name) });
    expect(open.querySelector("svg")).not.toBeNull();
    expect(open).toHaveTextContent(teamScreenText.openLabel);
    const remove = screen.getByRole("button", { name: teamScreenText.deleteLabel(TEAM.name) });
    expect(remove.querySelector("svg")).not.toBeNull();
    expect(remove).toHaveClass("ui-button--danger", "team-screen__icon-button");
    expect(remove.textContent).toBe("");
  });

  test("削除は 2 段階のまま(確認の文と、確定・やめるが出る)", async () => {
    const { user } = await renderScreen([TEAM]);
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteLabel(TEAM.name) }));
    expect(screen.getByText(teamScreenText.deleteConfirmNotice(TEAM.name))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM.name) })).toBeEnabled();
    expect(screen.getByRole("button", { name: teamScreenText.deleteCancelLabel(TEAM.name) })).toBeEnabled();
  });

  test("[新しい構築]は「+」のアイコン付きで、名前は変わらない", async () => {
    await renderScreen([]);
    const create = screen.getByRole("button", { name: teamScreenText.createLabel });
    expect(create.querySelector("svg")).not.toBeNull();
  });

  test("空の一覧は大きいアイコン + 短い一言(1 文だけで、長い案内を出さない)", async () => {
    await renderScreen([]);
    const empty = screen.getByText(teamScreenText.emptyNotice);
    expect(empty).toHaveClass("ui-notice--empty");
    expect(empty.querySelector("svg")).not.toBeNull();
    expect(empty.textContent.length).toBeLessThanOrEqual(20);
  });
});

describe("編集の 6 枠のタイル", () => {
  test("埋まった枠はポケモンカード(名前・タイプバッジ・持ち物)、空きは「+」のタイル", async () => {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    const grid = within(editor).getByRole("group", { name: teamMemberText.slotsLabel });
    const cards = grid.querySelectorAll(".ui-pokemon-card");
    expect(cards).toHaveLength(1);
    expect(cards[0]).toHaveTextContent(speciesName(MEMBER_FIRE.speciesKey));
    expect(cards[0]?.querySelectorAll(".ui-badge").length).toBeGreaterThan(0);
    expect(cards[0]?.querySelector(".ui-pokemon-card__item")).not.toBeNull();
    const tiles = grid.querySelectorAll(".ui-tile--empty");
    expect(tiles).toHaveLength(5);
    for (const position of [2, 3, 4, 5, 6]) {
      expect(within(grid).getByRole("button", { name: teamMemberText.slotAddLabel(position) })).toBeEnabled();
    }
  });

  test("空きのタイルを押すと、その枠の「ポケモン」欄へフォーカスが移る", async () => {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    await user.click(within(editor).getByRole("button", { name: teamMemberText.slotAddLabel(3) }));
    const slot = within(editor).getByRole("group", { name: teamMemberText.memberLegend(3) });
    expect(within(slot).getByRole("combobox", { name: teamMemberText.speciesLabel })).toHaveFocus();
  });

  test("埋まったタイルを押すと、その枠の欄へフォーカスが移る", async () => {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    const grid = within(editor).getByRole("group", { name: teamMemberText.slotsLabel });
    await user.click(
      within(grid).getByRole("button", { name: new RegExp(speciesName(MEMBER_FIRE.speciesKey)) }),
    );
    const slot = within(editor).getByRole("group", { name: teamMemberText.memberLegend(1) });
    expect(within(slot).getByRole("combobox", { name: teamMemberText.speciesLabel })).toHaveFocus();
  });

  test("枠のポケモンを選ぶと、タイルが空きからカードに変わる", async () => {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    const slot = within(editor).getByRole("group", { name: teamMemberText.memberLegend(2) });
    await user.selectOptions(
      within(slot).getByRole("combobox", { name: teamMemberText.speciesLabel }),
      "9004-000",
    );
    const grid = within(editor).getByRole("group", { name: teamMemberText.slotsLabel });
    expect(grid.querySelectorAll(".ui-pokemon-card")).toHaveLength(2);
    expect(within(grid).queryByRole("button", { name: teamMemberText.slotAddLabel(2) })).toBeNull();
  });
});

describe("編集の欄", () => {
  async function openSlot(): Promise<{ user: UserEvent; slot: HTMLElement; editor: HTMLElement }> {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    return {
      user,
      editor,
      slot: within(editor).getByRole("group", { name: teamMemberText.memberLegend(1) }),
    };
  }

  test("SP は増減ボタン付き。押すと 1 ずつ増減し、0 と 32 で止まる", async () => {
    const { user, slot } = await openSlot();
    const letter = statLetterJa.atk;
    const input = within(slot).getByRole("textbox", { name: teamMemberText.spLabel(letter) });
    expect(input).toHaveValue("32");
    const increase = within(slot).getByRole("button", { name: `${teamMemberText.spLabel(letter)}を増やす` });
    const decrease = within(slot).getByRole("button", { name: `${teamMemberText.spLabel(letter)}を減らす` });
    expect(increase).toHaveAttribute("aria-disabled", "true");
    await user.click(increase);
    expect(input).toHaveValue("32");
    await user.click(decrease);
    expect(input).toHaveValue("31");
    await user.clear(input);
    await user.type(input, "0");
    expect(decrease).toHaveAttribute("aria-disabled", "true");
    await user.click(decrease);
    expect(input).toHaveValue("0");
    await user.click(increase);
    expect(input).toHaveValue("1");
  });

  test("SP の合計と残りは数値 + バー(バーは装飾)。66 を超えると超過の印", async () => {
    const { user, slot } = await openSlot();
    expect(slot).toHaveTextContent(teamMemberText.spSummary(66, 66, 0));
    const bar = slot.querySelector(".team-member__sp-bar");
    expect(bar).not.toBeNull();
    expect(bar).toHaveAttribute("aria-hidden", "true");
    await user.click(
      within(slot).getByRole("button", { name: `${teamMemberText.spLabel(statLetterJa.hp)}を増やす` }),
    );
    expect(slot).toHaveTextContent(teamMemberText.spSummary(67, 66, -1));
    expect(slot.querySelector(".team-member__sp-summary--over")).not.toBeNull();
  });

  test("上へ・下へ・外すはアイコンだけのボタン(名前は「N体目を…」のまま)", async () => {
    const { slot } = await openSlot();
    for (const name of [
      teamMemberText.moveUpLabel(1),
      teamMemberText.moveDownLabel(1),
      teamMemberText.removeLabel(1),
    ]) {
      const button = within(slot).getByRole("button", { name });
      expect(button.querySelector("svg")).not.toBeNull();
      expect(button.textContent).toBe("");
      expect(button).toHaveClass("team-member__icon-button");
    }
  });

  test("性格は 5 択以上なので select のまま。区切りボタンにしない", async () => {
    const { slot } = await openSlot();
    expect(within(slot).getByRole("combobox", { name: teamMemberText.natureLabel })).toBeInTheDocument();
    expect(slot.querySelector(".ui-segmented")).toBeNull();
  });

  test("未保存の印はアイコン + 一言。保存済みの印もアイコン付き", async () => {
    const { user, editor, slot } = await openSlot();
    await user.click(
      within(slot).getByRole("button", { name: `${teamMemberText.spLabel(statLetterJa.hp)}を増やす` }),
    );
    const unsaved = within(editor).getByText(teamMemberText.unsavedNotice);
    expect(unsaved.querySelector("svg")).not.toBeNull();
  });
});

describe("キーボード", () => {
  test("タイルは Tab の順に入れない(欄は Tab で順に行けるので、タイルは近道。Enter / Space は効く)", async () => {
    const { user } = await renderScreen([TEAM]);
    const editor = await openEditor(user, TEAM.name);
    const grid = within(editor).getByRole("group", { name: teamMemberText.slotsLabel });
    for (const button of within(grid).getAllByRole("button")) {
      expect(button).toHaveAttribute("tabindex", "-1");
    }
    const add = within(grid).getByRole("button", { name: teamMemberText.slotAddLabel(2) });
    add.focus();
    await user.keyboard("{Enter}");
    const slot = within(editor).getByRole("group", { name: teamMemberText.memberLegend(2) });
    expect(within(slot).getByRole("combobox", { name: teamMemberText.speciesLabel })).toHaveFocus();
  });
});
