// issue #515・ADR-0320 PR-B(docs/mega-evolution-spec.md §4-3): 構築のメンバー編集のメガシンカの持ち物固定と、
// 古い保存データの補正。TeamClient は fake、マスタは例データ + 架空のメガ種族・メガストーン(test/megaMaster.ts)。
// 確かめること(受け入れ条件):
//   AC-1 メガ種族を選ぶと持ち物がストーンに固定される(disabled・ストーンの名前・理由の見える文言と aria-describedby)
//   AC-2 非メガに変えると解除して未選択(null)。メガストーンは単独の持ち物の選択肢に出ない
//   AC-3 保存の PUT の items にメガならストーンが載る
//   AC-4 古い保存データ: メガ種族に別の持ち物(null 含む)→ 開いたときストーンへ直し、role="status" で通知。下書きは直した状態で、
//        保存すると永続化。自動では保存しない。すでにストーンなら通知しない
//   AC-5 非メガに持たせたメガストーンは直さない(通知なし・値は保つ)
//   AC-6 ストーンがマスタに無いメガ: 黙って壊さず持ち物を空にして通知・理由を出す
//   AC-7 種族の一覧が無いマスタ(オンライン・キャッシュ済みオフライン): 種族の解決後に同じに直る。検索で選んでも固定される
//   AC-8 非メガのメンバーの挙動は変わらない

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { megaStoneItemIds } from "../domain/mega";
import { megaItemText, teamMemberText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { createFakeTeamClient, flush, lastCall, type FakeTeamClient } from "../test/fakeTeamClient";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_FIRE_STONE_LABEL,
  MEGA_ORPHAN,
  MEGA_WATER_STONE,
  UNNAMED_MEGA_STONE_LABEL,
  withMegaFixture,
} from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { TeamScreen } from "./TeamScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
});

const NORMAL_KEY = "9001-000";
const OTHER_ITEM_ID = "exampleitemdef";

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
    id: "55555555-5555-4555-8555-555555555555",
    name: "テストメガ構築",
    members,
    createdAt: "2026-10-03T12:00:00Z",
    updatedAt: "2026-10-03T12:00:00Z",
  };
}

interface Rendered {
  readonly client: FakeTeamClient;
  readonly user: UserEvent;
}

async function renderScreen(
  teams: readonly Schemas["Team"][],
  options: { master?: MasterData; masterSearch?: MasterSpeciesSearch } = {},
): Promise<Rendered> {
  const client = createFakeTeamClient();
  const user = userEvent.setup();
  render(
    <TeamScreen teamClient={client} master={options.master ?? master} masterSearch={options.masterSearch} />,
  );
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [...teams] });
  });
  return { client, user };
}

async function openEditor(user: UserEvent, name: string): Promise<HTMLElement> {
  await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(name) }));
  return screen.getByRole("region", { name: teamMemberText.editorLabel(name) });
}

function group(editor: HTMLElement, position: number): HTMLElement {
  return within(editor).getByRole("group", { name: teamMemberText.memberLegend(position) });
}

function select(element: HTMLElement, name: string): HTMLSelectElement {
  const found = within(element).getByRole("combobox", { name });
  if (!(found instanceof HTMLSelectElement)) {
    throw new Error(`「${name}」は select ではない`);
  }
  return found;
}

function itemSelect(element: HTMLElement): HTMLSelectElement {
  return select(element, teamMemberText.itemLabel);
}

function optionLabels(element: HTMLElement): string[] {
  return within(element)
    .getAllByRole("option")
    .map((option) => option.textContent.trim());
}

/** 通知(role="status")。文言がそのまま status 要素の中にあること。 */
function notice(element: HTMLElement, text: string): HTMLElement {
  const found = within(element).getByText(text);
  expect(found).toHaveAttribute("role", "status");
  return found;
}

async function save(
  user: UserEvent,
  client: FakeTeamClient,
  editor: HTMLElement,
): Promise<Schemas["TeamMember"][]> {
  await user.click(within(editor).getByRole("button", { name: teamMemberText.saveLabel }));
  await waitFor(() => {
    expect(client.updateCalls.length).toBeGreaterThan(0);
  });
  const call = lastCall(client.updateCalls, "update");
  const members = call.args.input.members;
  await flush(() => {
    call.resolve({
      ok: true,
      // 名前の省略はサーバーが既定名を補う(ADR-0229)。
      value: { ...team([]), id: call.args.teamId, name: call.args.input.name ?? "名称未設定", members },
    });
  });
  return members;
}

describe("AC-1・AC-2・AC-3 メガ種族の選択と固定", () => {
  test("メガ種族を選ぶと持ち物がストーンに固定され、理由が見える文言と aria-describedby で伝わる", async () => {
    const { user } = await renderScreen([team([])]);
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);

    await user.selectOptions(select(first, teamMemberText.speciesLabel), MEGA_FIRE.key);

    const item = itemSelect(first);
    expect(item).toBeDisabled();
    expect(item).toHaveValue(MEGA_FIRE_STONE.id);
    // (なし)ではなく「{基本種名}のメガストーン」が見える(ADR-0326。ストーンの nameJa は出さない)。
    expect(item).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(within(first).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(item).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("先に別の持ち物を選んでいても、メガ種族に変えるとストーンに置き換わる。非メガに戻すと解除されて未選択", async () => {
    const { user } = await renderScreen([team([member(NORMAL_KEY, OTHER_ITEM_ID)])]);
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);
    expect(itemSelect(first)).toHaveValue(OTHER_ITEM_ID);

    await user.selectOptions(select(first, teamMemberText.speciesLabel), MEGA_FIRE.key);
    expect(itemSelect(first)).toBeDisabled();
    expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);

    await user.selectOptions(select(first, teamMemberText.speciesLabel), NORMAL_KEY);
    expect(itemSelect(first)).toBeEnabled();
    expect(itemSelect(first)).toHaveValue("");
    expect(itemSelect(first)).not.toHaveAccessibleDescription(megaItemText.lockedReason);
    expect(within(first).queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("メガストーンは単独の持ち物の選択肢に出ない(非メガの持ち物欄は、ストーンを除いたマスタの全件 + なし)", async () => {
    const { user } = await renderScreen([team([member(NORMAL_KEY, null)])]);
    const first = group(await openEditor(user, "テストメガ構築"), 1);
    const stones = megaStoneItemIds(master.species);
    expect(stones.size).toBeGreaterThan(0);

    expect(optionLabels(itemSelect(first))).toEqual([
      teamMemberText.itemNone,
      ...master.items.filter((item) => !stones.has(item.id)).map((item) => item.nameJa),
    ]);
  });

  test("保存の PUT: メガのメンバーの itemId はストーン。非メガは持ち物を保つ", async () => {
    const { client, user } = await renderScreen([team([member(NORMAL_KEY, OTHER_ITEM_ID)])]);
    const editor = await openEditor(user, "テストメガ構築");
    await user.selectOptions(select(group(editor, 2), teamMemberText.speciesLabel), MEGA_FIRE.key);

    const members = await save(user, client, editor);

    expect(members.map((m) => m.itemId)).toEqual([OTHER_ITEM_ID, MEGA_FIRE_STONE.id]);
  });
});

describe("AC-4 古い保存データの補正(別の持ち物を持つメガ種族)", () => {
  test("開くと持ち物がストーンに直り、直したことが role=status で出る。自動では保存しない", async () => {
    const { client, user } = await renderScreen([team([member(MEGA_FIRE.key, OTHER_ITEM_ID)])]);
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);

    expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    expect(itemSelect(first)).toBeDisabled();
    expect(itemSelect(first)).toHaveAccessibleDescription(megaItemText.lockedReason);
    notice(first, megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL));
    expect(client.updateCalls).toHaveLength(0);
  });

  test("持ち物が null(なし)の古いメガ種族も、ストーンに直して通知する", async () => {
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, null)])]);
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    notice(first, megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL));
  });

  test("直した状態が下書きで、保存すると PUT にストーンが載る(他の欄は変わらない)", async () => {
    const original = member(MEGA_FIRE.key, OTHER_ITEM_ID);
    const { client, user } = await renderScreen([team([original])]);
    const editor = await openEditor(user, "テストメガ構築");
    expect(within(editor).getByRole("button", { name: teamMemberText.saveLabel })).toBeEnabled();

    const members = await save(user, client, editor);

    expect(members).toEqual([{ ...original, itemId: MEGA_FIRE_STONE.id }]);
  });

  test("体ごとに直す: 直したメンバーにだけ通知が出て、すでにストーンのメンバー・非メガには出ない", async () => {
    const { user } = await renderScreen([
      team([
        member(MEGA_FIRE.key, MEGA_FIRE_STONE.id),
        member(MEGA_FIRE.key, OTHER_ITEM_ID),
        member(NORMAL_KEY, OTHER_ITEM_ID),
      ]),
    ]);
    const editor = await openEditor(user, "テストメガ構築");
    const text = megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL);

    expect(within(group(editor, 1)).queryByText(text)).toBeNull();
    notice(group(editor, 2), text);
    expect(within(group(editor, 3)).queryByText(text)).toBeNull();
    expect(itemSelect(group(editor, 3))).toHaveValue(OTHER_ITEM_ID);
  });

  test("すでにストーンを持つメガ種族は、通知なしで固定のまま開く", async () => {
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, MEGA_FIRE_STONE.id)])]);
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    expect(itemSelect(first)).toBeDisabled();
    expect(within(first).queryByText(megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL))).toBeNull();
  });

  test("[保存せずに戻る]で閉じて開き直すと、下書きは捨てられ保存済みの値から再び補正される", async () => {
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, OTHER_ITEM_ID)])]);
    let editor = await openEditor(user, "テストメガ構築");
    // 補正で下書きが保存済みと違う(未保存)ので、戻るときに確認が出る。[保存せずに戻る]で捨てる。
    await user.click(within(editor).getByRole("button", { name: teamMemberText.closeLabel }));
    await user.click(screen.getByRole("button", { name: teamMemberText.leaveDiscardLabel }));
    editor = await openEditor(user, "テストメガ構築");

    notice(group(editor, 1), megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL));
  });
});

describe("AC-5 非メガに持たせたメガストーンは直さない", () => {
  test("非メガの持ち物がメガストーンでも、値を保ち(「メガストーン」と表示。ADR-0326)、通知も出さず、保存でも変えない", async () => {
    const { client, user } = await renderScreen([team([member(NORMAL_KEY, MEGA_WATER_STONE.id)])]);
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);

    expect(itemSelect(first)).toBeEnabled();
    expect(itemSelect(first)).toHaveValue(MEGA_WATER_STONE.id);
    expect(itemSelect(first)).toHaveDisplayValue(UNNAMED_MEGA_STONE_LABEL);
    expect(within(first).queryByRole("status")).toBeNull();

    const members = await save(user, client, editor);
    expect(members[0]?.itemId).toBe(MEGA_WATER_STONE.id);
  });
});

describe("AC-6 ストーンがマスタに無いメガ種族", () => {
  test("固定せず、別の持ち物は空にして、空にしたことと理由を出す(黙って別の持ち物にしない)", async () => {
    const { client, user } = await renderScreen([team([member(MEGA_ORPHAN.key, OTHER_ITEM_ID)])]);
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);

    expect(itemSelect(first)).toBeDisabled();
    expect(itemSelect(first)).toHaveValue("");
    expect(within(first).getByText(megaItemText.missingReason)).toBeVisible();
    expect(itemSelect(first)).toHaveAccessibleDescription(megaItemText.missingReason);
    notice(first, megaItemText.clearedNotice);

    const members = await save(user, client, editor);
    expect(members[0]?.itemId).toBeNull();
  });

  test("もともと持ち物が空なら、理由は出すが「空にした」通知は出さない", async () => {
    const { user } = await renderScreen([team([member(MEGA_ORPHAN.key, null)])]);
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    expect(itemSelect(first)).toBeDisabled();
    expect(within(first).getByText(megaItemText.missingReason)).toBeVisible();
    expect(within(first).queryByText(megaItemText.clearedNotice)).toBeNull();
  });
});

describe("AC-7 種族の一覧が無いマスタ(オンライン・キャッシュ済みオフライン)", () => {
  const ONLINE = { speciesList: false, moves: false, effects: true } as const;

  function onlineFixture(): { master: MasterData; search: ReturnType<typeof createFakeSpeciesSearch> } {
    return {
      master: limitedMaster(master, ONLINE),
      search: createFakeSpeciesSearch({
        species: master.species,
        abilities: master.abilities,
        moves: master.moves,
      }),
    };
  }

  test("保存済みメンバーの種族の解決後に、持ち物がストーンに直り通知が出る", async () => {
    const fixture = onlineFixture();
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, OTHER_ITEM_ID)])], {
      master: fixture.master,
      masterSearch: fixture.search,
    });
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    await waitFor(() => {
      expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    });
    expect(itemSelect(first)).toBeDisabled();
    notice(first, megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL));
  });

  test("検索欄でメガ種族を選ぶと持ち物がストーンに固定される", async () => {
    const fixture = onlineFixture();
    const { user } = await renderScreen([team([])], { master: fixture.master, masterSearch: fixture.search });
    const editor = await openEditor(user, "テストメガ構築");
    const first = group(editor, 1);

    await user.type(
      within(first).getByRole("combobox", { name: teamMemberText.speciesLabel }),
      MEGA_FIRE.nameJa,
    );
    await user.click(await within(first).findByRole("option", { name: MEGA_FIRE.nameJa }, { timeout: 3000 }));

    await waitFor(() => {
      expect(itemSelect(first)).toHaveValue(MEGA_FIRE_STONE.id);
    });
    expect(itemSelect(first)).toBeDisabled();
    expect(itemSelect(first)).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("解決に失敗したメンバーは持ち物を書き換えない(ADR-0316 §7。alert だけ)", async () => {
    const fixture = onlineFixture();
    const failing: MasterSpeciesSearch = {
      searchSpecies: fixture.search.searchSpecies.bind(fixture.search),
      resolveSpecies: () => Promise.reject(new Error("fail")),
    };
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, OTHER_ITEM_ID)])], {
      master: fixture.master,
      masterSearch: failing,
    });
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    await within(first).findByRole("alert");
    expect(itemSelect(first)).toHaveValue(OTHER_ITEM_ID);
    expect(within(first).queryByRole("status")).toBeNull();
  });
});

describe("AC-8 非メガのメンバーは変わらない", () => {
  test("非メガのメンバーを開いても通知は出ず、持ち物の選択は自由のまま", async () => {
    const { user } = await renderScreen([team([member(NORMAL_KEY, OTHER_ITEM_ID)])]);
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    expect(within(first).queryByRole("status")).toBeNull();
    expect(itemSelect(first)).toBeEnabled();
    await user.selectOptions(itemSelect(first), "");
    expect(itemSelect(first)).toHaveValue("");
  });
});

// ADR-0326 §4(ADR-0328 でも保つ): ストーンの nameJa が英語なら、持ち物欄・補正の通知のどちらにも英語名を出さない。
describe("英語名のメガストーンを画面に出さない", () => {
  test("古いデータを直した通知と持ち物欄は「{基本種名}のメガストーン」で、英語名はどこにも出ない", async () => {
    const english: MasterData = {
      ...master,
      items: master.items.map((item) =>
        item.id === MEGA_FIRE_STONE.id ? { ...item, nameJa: "Barbaracite" } : item,
      ),
    };
    const { user } = await renderScreen([team([member(MEGA_FIRE.key, OTHER_ITEM_ID)])], { master: english });
    const first = group(await openEditor(user, "テストメガ構築"), 1);

    const fallback = "テストほのお専用のメガストーン";
    expect(itemSelect(first)).toHaveDisplayValue(fallback);
    notice(first, megaItemText.correctedNotice(fallback));
    expect(document.body).not.toHaveTextContent("Barbaracite");
  });
});
