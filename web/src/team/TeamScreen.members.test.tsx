// P5-5b PR-A2(ADR-0316)・F-08(ADR-0332): 構築のメンバー編集。TeamClient は fake、マスタは架空の例データ(ADR-0300 §3)。
// F-08 で、編集は常に6枠(空の枠は種族の欄と案内だけ)の画面になり、[メンバーを追加]・構築名の変更は無い。
// 6枠・空の枠・外す・入れ替え・未保存の印・一覧に戻るは TeamScreen.rebuild.test.tsx。
// 確かめること(受け入れ条件。「メンバー数」は種族の決まった枠の数):
//   AC-1 開く: 構築のカードの [開く] で編集画面を開く(API は呼ばない)・各メンバーの現在値が出る
//   AC-2 並べ替え: 上下の入れ替え・先頭の [上へ] は無効・種族を選んだ枠の初期値
//   AC-3 保存: update は全置換(name を送らない・全メンバー。ニックネームを落とさない)・二重送信しない・成功で一覧が更新・
//        失敗は role="alert" でサーバーの message を日本語のまま出し、下書きを残して再送できる・保存せずに戻ると一覧は変わらない
//   AC-4 SP: 6欄・0/32 は通り 33/負/小数は明示エラーで保存不可・合計 66 は通り 67 は超過表示で保存不可・直すと保存できる
//   AC-5 技: 枠は4つだけ(5つ目は無い)・候補はその種族の learnset・他の枠で選んだ技は選べない(重複を作らない)
//   AC-6 種族変更: 特性は持てば保ち持たなければ選び直す・技は learnset にあるものだけ残る・空の枠は保存の妨げにならない
//   AC-7 マスタ: 種族の一覧が無いマスタ(オンライン)は検索欄で選ぶ・保存済みメンバーの種族は開いたときに解決する・
//        マスタに無い種族のメンバーも壊さずに保存し直せる
//   AC-8 a11y: メンバーは legend 付きの group・各コントロールにラベル・キーボードだけで空の枠の種族の欄へ行ける
// 架空の構築名・ID だけを使う(ADR-0002)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { selectableAbilities } from "../domain/requests";
import { statLetterJa, teamMemberText, teamScreenText, typeNameJa, type TypeId } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { createFakeTeamClient, flush, lastCall, type FakeTeamClient } from "../test/fakeTeamClient";
import { createFakeSpeciesSearch, limitedMaster, type FakeSpeciesSearch } from "../test/onlineMaster";
import { MAX_TEAM_MEMBERS, TeamScreen } from "./TeamScreen";
import { MAX_MEMBER_MOVES, SP_STATS } from "./teamMember";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

// ---- 架空の構築 ----

const MEMBER_FIRE: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  nickname: "テスト愛称",
  moveIds: ["examplemovetackle", "examplemovefirepunch"],
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: "fire",
};

const MEMBER_ELECTRIC: Schemas["TeamMember"] = {
  speciesKey: "9004-000",
  moveIds: ["examplemovethunder"],
  itemId: null,
  abilityId: "exampleabilityadapt",
  natureId: "example-nature-spa",
  sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 32 },
  teraType: null,
};

function team(id: string, name: string, members: Schemas["TeamMember"][]): Schemas["Team"] {
  return {
    id,
    name,
    members,
    createdAt: "2026-09-26T12:00:00Z",
    updatedAt: "2026-09-26T12:00:00Z",
  };
}

const TEAM_A = team("33333333-3333-4333-8333-333333333333", "テスト構築A", [MEMBER_FIRE, MEMBER_ELECTRIC]);
const TEAM_EMPTY = team("44444444-4444-4444-8444-444444444444", "テスト空構築", []);

// ---- 画面の組み立て ----

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

function memberGroup(editor: HTMLElement, position: number): HTMLElement {
  return within(editor).getByRole("group", { name: teamMemberText.memberLegend(position) });
}

/** 種族の決まった枠(技の欄が出ている枠)だけ。空の枠は種族の欄と案内だけなので数えない(ADR-0332 §2)。 */
function memberGroups(editor: HTMLElement): HTMLElement[] {
  return within(editor)
    .queryAllByRole("group", { name: /^\d+体目$/ })
    .filter((group) => within(group).queryByRole("combobox", { name: teamMemberText.moveLabel(1) }) !== null);
}

function select(group: HTMLElement, name: string): HTMLSelectElement {
  const element = within(group).getByRole("combobox", { name });
  if (!(element instanceof HTMLSelectElement)) {
    throw new Error(`「${name}」は select ではない`);
  }
  return element;
}

function spInput(group: HTMLElement, stat: (typeof SP_STATS)[number]): HTMLInputElement {
  const element = within(group).getByRole("textbox", { name: teamMemberText.spLabel(statLetterJa[stat]) });
  if (!(element instanceof HTMLInputElement)) {
    throw new Error("SP 欄は input ではない");
  }
  return element;
}

async function setSp(user: UserEvent, group: HTMLElement, stat: (typeof SP_STATS)[number], value: string) {
  const input = spInput(group, stat);
  await user.clear(input);
  if (value !== "") {
    await user.type(input, value);
  }
}

function saveButton(editor: HTMLElement): HTMLElement {
  return within(editor).getByRole("button", { name: teamMemberText.saveLabel });
}

function optionLabels(element: HTMLElement): string[] {
  return within(element)
    .getAllByRole("option")
    .map((option) => option.textContent.trim());
}

function abilityName(id: string): string {
  const found = master.abilities.find((ability) => ability.id === id);
  if (found === undefined) {
    throw new Error(`例データに特性 ${id} が無い`);
  }
  return found.nameJa;
}

function moveName(id: string): string {
  const found = master.moves.find((move) => move.id === id);
  if (found === undefined) {
    throw new Error(`例データに技 ${id} が無い`);
  }
  return found.nameJa;
}

function speciesName(key: string): string {
  const found = master.species.find((species) => species.key === key);
  if (found === undefined) {
    throw new Error(`例データに種族 ${key} が無い`);
  }
  return found.nameJa;
}

/** 保存を押し、update の応答を成功で返す(入力をそのまま Team にする)。 */
async function saveAndSucceed(user: UserEvent, client: FakeTeamClient, editor: HTMLElement) {
  await user.click(saveButton(editor));
  await waitFor(() => {
    expect(client.updateCalls.length).toBeGreaterThan(0);
  });
  const call = lastCall(client.updateCalls, "update");
  const saved: Schemas["Team"] = {
    id: call.args.teamId,
    name: call.args.input.name ?? "名称未設定", // 名前の省略はサーバーが既定名を補う(ADR-0229)
    members: call.args.input.members,
    createdAt: "2026-09-26T12:00:00Z",
    updatedAt: "2026-10-02T09:00:00Z",
  };
  await flush(() => {
    call.resolve({ ok: true, value: saved });
  });
}

describe("AC-1 開く・閉じる", () => {
  test("カードの [開く] で編集画面が開き、通信はしない(list の応答で足りる)", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    expect(screen.queryByRole("region", { name: teamMemberText.editorLabel(TEAM_A.name) })).toBeNull();

    const editor = await openEditor(user, TEAM_A.name);

    expect(memberGroups(editor)).toHaveLength(2);
    expect(client.getCalls).toHaveLength(0);
    expect(client.updateCalls).toHaveLength(0);
    expect(client.listCalls).toHaveLength(1);
  });

  test("各メンバーの現在値(種族・技・持ち物・特性・性格・テラスタイプ・SP と合計)が出る", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);

    expect(select(first, teamMemberText.speciesLabel).value).toBe("9001-000");
    expect(select(first, teamMemberText.moveLabel(1)).value).toBe("examplemovetackle");
    expect(select(first, teamMemberText.moveLabel(2)).value).toBe("examplemovefirepunch");
    expect(select(first, teamMemberText.moveLabel(3)).value).toBe("");
    expect(select(first, teamMemberText.moveLabel(4)).value).toBe("");
    expect(select(first, teamMemberText.itemLabel).value).toBe("exampleitemdef");
    expect(select(first, teamMemberText.abilityLabel).value).toBe("exampleabilitynone");
    expect(select(first, teamMemberText.natureLabel).value).toBe("example-nature-atk");
    expect(select(first, teamMemberText.teraLabel).value).toBe("fire");
    expect(spInput(first, "hp").value).toBe("2");
    expect(spInput(first, "atk").value).toBe("32");
    expect(spInput(first, "spe").value).toBe("32");
    expect(first).toHaveTextContent(teamMemberText.spSummary(66, 66, 0));

    const second = memberGroup(editor, 2);
    expect(select(second, teamMemberText.itemLabel).value).toBe("");
    expect(select(second, teamMemberText.teraLabel).value).toBe("");
  });

  test("選択肢はマスタから出す: 持ち物・性格はマスタの全件(+なし)、テラスタイプはタイプ相性表のタイプ", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);

    expect(optionLabels(select(first, teamMemberText.itemLabel))).toEqual([
      teamMemberText.itemNone,
      ...master.items.map((item) => item.nameJa),
    ]);
    expect(optionLabels(select(first, teamMemberText.natureLabel))).toEqual(
      master.natures.map((nature) => nature.nameJa),
    );
    expect(optionLabels(select(first, teamMemberText.teraLabel))).toEqual([
      teamMemberText.teraNone,
      ...master.typeChart.types.map((type) => typeNameJa[type as TypeId]),
    ]);
  });
});

describe("AC-2 並べ替え・種族を選んだ枠の初期値", () => {
  test("空の枠で種族を選んだ枠は、技なし・SP 合計 0(残り 66)", async () => {
    const { user } = await renderScreen([TEAM_EMPTY]);
    const editor = await openEditor(user, TEAM_EMPTY.name);
    expect(memberGroups(editor)).toHaveLength(0);
    await user.selectOptions(select(memberGroup(editor, 1), teamMemberText.speciesLabel), "9001-000");
    const group = memberGroup(editor, 1);

    expect(memberGroups(editor)).toHaveLength(1);
    expect(group).toHaveTextContent(teamMemberText.spSummary(0, 66, 66));
    for (let slot = 1; slot <= MAX_MEMBER_MOVES; slot += 1) {
      expect(select(group, teamMemberText.moveLabel(slot)).value).toBe("");
    }
  });

  test("上へ・下へで入れ替わる。先頭の [上へ] は無効", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    expect(
      within(memberGroup(editor, 1)).getByRole("button", { name: teamMemberText.moveUpLabel(1) }),
    ).toBeDisabled();

    await user.click(
      within(memberGroup(editor, 1)).getByRole("button", { name: teamMemberText.moveDownLabel(1) }),
    );

    expect(select(memberGroup(editor, 1), teamMemberText.speciesLabel).value).toBe("9004-000");
    expect(select(memberGroup(editor, 2), teamMemberText.speciesLabel).value).toBe("9001-000");
    // 内容(技・SP)も体と一緒に動く。
    expect(select(memberGroup(editor, 2), teamMemberText.moveLabel(1)).value).toBe("examplemovetackle");
    expect(spInput(memberGroup(editor, 1), "spa").value).toBe("32");
  });

  test("並べ替えの結果が保存する members の順になる", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(
      within(memberGroup(editor, 1)).getByRole("button", { name: teamMemberText.moveDownLabel(1) }),
    );
    await user.click(saveButton(editor));

    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(
      lastCall(client.updateCalls, "update").args.input.members.map((member) => member.speciesKey),
    ).toEqual(["9004-000", "9001-000"]);
  });
});

describe("AC-3 保存(update は全置換。応答で一覧を書き換える)", () => {
  test("何も変えずに保存すると、name キーを持たずに全メンバー(ニックネーム含む)をそのまま送る", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(saveButton(editor));

    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    const { teamId, input } = lastCall(client.updateCalls, "update").args;
    expect(teamId).toBe(TEAM_A.id);
    expect("name" in input).toBe(false);
    expect(input.members).toEqual(TEAM_A.members);
  });

  test("編集した内容が members に入る(性格・持ち物・テラスタイプ・技・SP)。技の空き枠は詰める", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);
    await user.selectOptions(select(first, teamMemberText.natureLabel), "example-nature-neutral-hardy");
    await user.selectOptions(select(first, teamMemberText.itemLabel), "");
    await user.selectOptions(select(first, teamMemberText.teraLabel), "water");
    await user.selectOptions(select(first, teamMemberText.moveLabel(1)), "");
    await setSp(user, first, "hp", "0");
    await setSp(user, first, "def", "2");
    await user.click(saveButton(editor));

    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    const [saved] = lastCall(client.updateCalls, "update").args.input.members;
    expect(saved).toMatchObject({
      speciesKey: "9001-000",
      nickname: "テスト愛称",
      moveIds: ["examplemovefirepunch"],
      natureId: "example-nature-neutral-hardy",
      teraType: "water",
      sp: { hp: 0, atk: 32, def: 2, spa: 0, spd: 0, spe: 32 },
    });
    expect(saved?.itemId ?? null).toBeNull();
  });

  test("送信中は [保存] を無効にし、二重に送らない", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(saveButton(editor));

    expect(saveButton(editor)).toBeDisabled();
    await user.click(saveButton(editor));
    expect(client.updateCalls).toHaveLength(1);
  });

  test("成功すると編集画面は開いたまま保存済みを知らせ、一覧に戻ると行のメンバー数が書き換わっている(list は読み直さない)", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(
      within(memberGroup(editor, 2)).getByRole("button", { name: teamMemberText.removeLabel(2) }),
    );
    await saveAndSucceed(user, client, editor);

    // name を送らないので、保存した構築はサーバーの既定名になり、画面では「構築 N」と出る(ADR-0332 §1)。
    expect(
      screen.getByRole("region", { name: teamMemberText.editorLabel(teamScreenText.untitledTeamName(1)) }),
    ).toBeInTheDocument();
    expect(within(editor).getByRole("status")).toHaveTextContent(teamMemberText.savedNotice);
    expect(saveButton(editor)).toBeEnabled();

    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));
    expect(screen.getByRole("list", { name: teamScreenText.listLabel })).toHaveTextContent(
      teamScreenText.memberCountLabel(1, MAX_TEAM_MEMBERS),
    );
    expect(client.listCalls).toHaveLength(1);
  });

  test("失敗は role=alert にサーバーの message(日本語)をそのまま出し、下書きを残す。直して再送できる", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);
    await user.selectOptions(select(first, teamMemberText.natureLabel), "example-nature-spa");
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({
        ok: false,
        error: { code: "invalid_input", message: "能力ポイントの合計は66以下にしてください" },
      });
    });

    const alert = within(editor).getByRole("alert");
    expect(alert).toHaveTextContent(teamMemberText.saveErrorHeading);
    expect(alert).toHaveTextContent("能力ポイントの合計は66以下にしてください");
    // 下書きは残る。
    expect(select(memberGroup(editor, 1), teamMemberText.natureLabel).value).toBe("example-nature-spa");
    // 再送できる(ボタンは有効に戻る)。
    expect(saveButton(editor)).toBeEnabled();
    await saveAndSucceed(user, client, editor);
    expect(client.updateCalls).toHaveLength(2);
    expect(within(editor).queryByText(teamMemberText.saveErrorHeading)).toBeNull();
  });

  test("通信できない失敗(team_unavailable)も同じく alert に出る", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({
        ok: false,
        error: { code: "team_unavailable", message: "構築サーバーに接続できません" },
      });
    });
    expect(within(editor).getByRole("alert")).toHaveTextContent("構築サーバーに接続できません");
  });

  test("保存に失敗したあと、保存せずに戻ると、一覧のメンバー数は保存前のまま", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.click(
      within(memberGroup(editor, 2)).getByRole("button", { name: teamMemberText.removeLabel(2) }),
    );
    await user.click(saveButton(editor));
    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({
        ok: false,
        error: { code: "invalid_input", message: "テストの入力エラー" },
      });
    });

    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));
    await user.click(screen.getByRole("button", { name: teamMemberText.leaveDiscardLabel }));
    expect(screen.getByRole("list", { name: teamScreenText.listLabel })).toHaveTextContent(
      teamScreenText.memberCountLabel(2, MAX_TEAM_MEMBERS),
    );
  });
});

describe("AC-4 SP のグリッド(各 0〜32・合計 66 以下)", () => {
  test("6欄(H・A・B・C・D・S)がラベル付きの数値入力で並ぶ。fieldset の legend は SP", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);

    expect(within(first).getByRole("group", { name: teamMemberText.spLegend })).toBeInTheDocument();
    expect(within(first).getAllByRole("textbox")).toHaveLength(SP_STATS.length);
    for (const stat of SP_STATS) {
      expect(spInput(first, stat)).toBeInTheDocument();
    }
  });

  test("入力すると合計と残りが更新される", async () => {
    const { user } = await renderScreen([TEAM_EMPTY]);
    const editor = await openEditor(user, TEAM_EMPTY.name);
    await user.selectOptions(select(memberGroup(editor, 1), teamMemberText.speciesLabel), "9001-000");
    const group = memberGroup(editor, 1);

    await setSp(user, group, "hp", "10");
    await setSp(user, group, "atk", "20");

    expect(group).toHaveTextContent(teamMemberText.spSummary(30, 66, 36));
  });

  test.each(["0", "32"])("1欄 %s は境界として通る(エラーなし・保存できる)", async (value) => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);
    await setSp(user, first, "atk", value);

    expect(within(first).queryByRole("alert")).toBeNull();
    expect(spInput(first, "atk")).not.toHaveAttribute("aria-invalid", "true");
    expect(saveButton(editor)).toBeEnabled();
  });

  test.each(["33", "-1", "1.5"])(
    "1欄 %s は明示エラー(その欄が aria-invalid・alert)で、保存は送れない",
    async (bad) => {
      const { client, user } = await renderScreen([TEAM_A]);
      const editor = await openEditor(user, TEAM_A.name);
      const first = memberGroup(editor, 1);
      await setSp(user, first, "atk", bad);

      expect(spInput(first, "atk")).toHaveAttribute("aria-invalid", "true");
      expect(within(first).getByRole("alert")).toHaveTextContent(
        teamMemberText.spStatError(statLetterJa.atk, 32),
      );
      expect(saveButton(editor)).toBeDisabled();
      await user.click(saveButton(editor));
      expect(client.updateCalls).toHaveLength(0);
    },
  );

  test("合計 66 ちょうどは通り、67 は超過を示して保存できない。1つ減らすと保存できる", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1); // 2 + 32 + 32 = 66
    expect(first).toHaveTextContent(teamMemberText.spSummary(66, 66, 0));
    expect(saveButton(editor)).toBeEnabled();

    await setSp(user, first, "hp", "3");
    expect(first).toHaveTextContent(teamMemberText.spSummary(67, 66, -1));
    expect(within(first).getByRole("alert")).toHaveTextContent(teamMemberText.spTotalError(1, 66));
    expect(saveButton(editor)).toBeDisabled();
    await user.click(saveButton(editor));
    expect(client.updateCalls).toHaveLength(0);

    await setSp(user, first, "hp", "2");
    expect(within(first).queryByRole("alert")).toBeNull();
    expect(saveButton(editor)).toBeEnabled();
  });

  test("空欄は 0 として合計に数える(エラーにしない)", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);
    await setSp(user, first, "spe", "");

    expect(within(first).queryByRole("alert")).toBeNull();
    expect(first).toHaveTextContent(teamMemberText.spSummary(34, 66, 32));
  });

  test("別のメンバーの SP の問題でも保存できない(体ごとに独立して検査する)", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await setSp(user, memberGroup(editor, 2), "hp", "33");

    expect(within(memberGroup(editor, 1)).queryByRole("alert")).toBeNull();
    expect(within(memberGroup(editor, 2)).getByRole("alert")).toBeInTheDocument();
    expect(saveButton(editor)).toBeDisabled();
  });
});

describe("AC-5 技(最大4・その種族の learnset から)", () => {
  test("技の枠は4つだけで、5つ目は無い", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);

    for (let slot = 1; slot <= MAX_MEMBER_MOVES; slot += 1) {
      expect(select(first, teamMemberText.moveLabel(slot))).toBeInTheDocument();
    }
    expect(
      within(first).queryByRole("combobox", { name: teamMemberText.moveLabel(MAX_MEMBER_MOVES + 1) }),
    ).toBeNull();
  });

  test("候補は(なし)+その種族の learnset の技(マスタで名前を解決)", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);
    const species = master.species.find((candidate) => candidate.key === "9001-000");

    expect(optionLabels(select(first, teamMemberText.moveLabel(3)))).toEqual([
      teamMemberText.moveNone,
      ...(species?.learnset.map((id) => moveName(id)) ?? []),
    ]);
  });

  test("他の枠で選んだ技は、この枠では選べない(無効)。自分の枠の選択は無効にしない", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);
    const slot3 = select(first, teamMemberText.moveLabel(3));

    expect(within(slot3).getByRole("option", { name: moveName("examplemovetackle") })).toBeDisabled();
    expect(within(slot3).getByRole("option", { name: moveName("examplemovefirepunch") })).toBeDisabled();
    expect(within(slot3).getByRole("option", { name: moveName("examplemovegrowl") })).toBeEnabled();
    const slot1 = select(first, teamMemberText.moveLabel(1));
    expect(within(slot1).getByRole("option", { name: moveName("examplemovetackle") })).toBeEnabled();
  });

  test("枠を埋めて保存すると4技が順に送られる", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await user.selectOptions(select(memberGroup(editor, 1), teamMemberText.moveLabel(3)), "examplemovegrowl");
    await user.click(saveButton(editor));

    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members[0]?.moveIds).toEqual([
      "examplemovetackle",
      "examplemovefirepunch",
      "examplemovegrowl",
    ]);
  });
});

describe("AC-6 種族を変えたときの特性・技", () => {
  test("特性: 新しい種族が持たない特性は先頭に選び直し、選択肢は新しい種族の特性(スロット順)になる", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const second = memberGroup(await openEditor(user, TEAM_A.name), 2); // 9004: [てきおう, むこう]、選択は てきおう
    const dualSpecies = master.species.find((species) => species.key === "9004-000");
    if (dualSpecies === undefined) {
      throw new Error("例データに 9004-000 が無い");
    }
    expect(optionLabels(select(second, teamMemberText.abilityLabel))).toEqual(
      selectableAbilities(dualSpecies, master.abilities).map((ability) => ability.nameJa),
    );

    await user.selectOptions(select(second, teamMemberText.speciesLabel), "9001-000"); // 特性は むこう だけ

    expect(optionLabels(select(second, teamMemberText.abilityLabel))).toEqual([
      abilityName("exampleabilitynone"),
    ]);
    expect(select(second, teamMemberText.abilityLabel).value).toBe("exampleabilitynone");
  });

  test("特性: 新しい種族も持っている特性は保つ", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1); // 9001: むこう
    await user.selectOptions(select(first, teamMemberText.speciesLabel), "9004-000"); // [てきおう, むこう]

    expect(select(first, teamMemberText.abilityLabel).value).toBe("exampleabilitynone");
  });

  test("技: 新しい種族の learnset に無い技は外れ、あるものだけが前に詰まって残る。持ち物・SP は保つ", async () => {
    const { client, user } = await renderScreen([
      team(TEAM_A.id, TEAM_A.name, [
        { ...MEMBER_FIRE, moveIds: ["examplemovetackle", "examplemovefirepunch", "examplemovegrowl"] },
      ]),
    ]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);

    await user.selectOptions(select(first, teamMemberText.speciesLabel), "9002-000"); // learnset: みずでっぽう, なきごえ

    expect(select(first, teamMemberText.moveLabel(1)).value).toBe("examplemovegrowl");
    expect(select(first, teamMemberText.moveLabel(2)).value).toBe("");
    expect(select(first, teamMemberText.moveLabel(3)).value).toBe("");
    expect(select(first, teamMemberText.itemLabel).value).toBe("exampleitemdef");
    expect(spInput(first, "atk").value).toBe("32");

    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members[0]).toMatchObject({
      speciesKey: "9002-000",
      moveIds: ["examplemovegrowl"],
      nickname: "テスト愛称",
    });
  });

  test("空の枠は保存の妨げにならない(理由の alert を出さない)。種族を選ぶと特性が先頭になり、そのまま保存できる", async () => {
    const { client, user } = await renderScreen([TEAM_EMPTY]);
    const editor = await openEditor(user, TEAM_EMPTY.name);
    const group = memberGroup(editor, 1);

    expect(within(group).queryByRole("alert")).toBeNull();
    expect(saveButton(editor)).toBeEnabled();

    await user.selectOptions(select(group, teamMemberText.speciesLabel), "9004-000");
    expect(within(group).queryByRole("alert")).toBeNull();
    expect(select(group, teamMemberText.abilityLabel).value).toBe("exampleabilityadapt");
    expect(saveButton(editor)).toBeEnabled();
    await user.click(saveButton(editor));
    expect(client.updateCalls).toHaveLength(1);
    expect(lastCall(client.updateCalls, "update").args.input.members).toHaveLength(1);
  });
});

describe("AC-7 マスタの状態", () => {
  const ONLINE = { speciesList: false, moves: false, effects: true } as const;

  function onlineFixture(): { master: MasterData; search: FakeSpeciesSearch } {
    return {
      master: limitedMaster(master, ONLINE),
      search: createFakeSpeciesSearch({
        species: master.species,
        abilities: master.abilities,
        moves: master.moves,
      }),
    };
  }

  test("種族の一覧が無いマスタ: 開いたとき、保存済みメンバーの種族を1体ずつ解決して名前・技・特性を出す", async () => {
    const { master: onlineMaster, search } = onlineFixture();
    const { user } = await renderScreen([TEAM_A], { master: onlineMaster, masterSearch: search });
    const editor = await openEditor(user, TEAM_A.name);

    await waitFor(() => {
      expect(search.resolvedKeys).toEqual(expect.arrayContaining(["9001-000", "9004-000"]));
    });
    const first = memberGroup(editor, 1);
    await waitFor(() => {
      expect(within(first).getByRole("combobox", { name: teamMemberText.speciesLabel })).toHaveValue(
        speciesName("9001-000"),
      );
    });
    // 技は resolveSpecies が運んだ learnset から(MasterData.moves は空)。
    expect(select(first, teamMemberText.moveLabel(1)).value).toBe("examplemovetackle");
    expect(select(first, teamMemberText.abilityLabel).value).toBe("exampleabilitynone");
  });

  test("種族の一覧が無いマスタ: 検索欄で選ぶと、その種族の特性・技が選択肢になる", async () => {
    const { master: onlineMaster, search } = onlineFixture();
    const { user } = await renderScreen([TEAM_EMPTY], { master: onlineMaster, masterSearch: search });
    const editor = await openEditor(user, TEAM_EMPTY.name);
    const group = memberGroup(editor, 1);

    const input = within(group).getByRole("combobox", { name: teamMemberText.speciesLabel });
    await user.type(input, speciesName("9004-000"));
    await user.click(
      await within(group).findByRole("option", { name: speciesName("9004-000") }, { timeout: 3000 }),
    );

    await waitFor(() => {
      expect(select(memberGroup(editor, 1), teamMemberText.abilityLabel).value).toBe("exampleabilityadapt");
    });
    expect(optionLabels(select(memberGroup(editor, 1), teamMemberText.moveLabel(1)))).toContain(
      moveName("examplemovethunder"),
    );
  });

  test("種族の解決に失敗したメンバーは、その旨を alert で出し、保存済みの内容は壊さない", async () => {
    const { master: onlineMaster } = onlineFixture();
    const failing: MasterSpeciesSearch = {
      searchSpecies: () => Promise.resolve([]),
      resolveSpecies: () => Promise.reject(new Error("offline")),
    };
    const { client, user } = await renderScreen([TEAM_A], { master: onlineMaster, masterSearch: failing });
    const editor = await openEditor(user, TEAM_A.name);

    await waitFor(() => {
      expect(within(memberGroup(editor, 1)).getByRole("alert")).toHaveTextContent(
        teamMemberText.speciesResolveError,
      );
    });
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members).toEqual(TEAM_A.members);
  });

  test("マスタに無い種族のメンバーも一覧どおり開け、手を付けずに保存すれば内容が変わらない", async () => {
    const stranger: Schemas["TeamMember"] = {
      ...MEMBER_FIRE,
      speciesKey: "9999-000",
      moveIds: ["unknownmove"],
    };
    const { client, user } = await renderScreen([team(TEAM_A.id, TEAM_A.name, [stranger])]);
    const editor = await openEditor(user, TEAM_A.name);

    expect(optionLabels(select(memberGroup(editor, 1), teamMemberText.speciesLabel))).toContain(
      teamMemberText.unknownSpeciesOption("9999-000"),
    );
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members).toEqual([stranger]);
  });
});

describe("AC-8 a11y(group・ラベル・キーボード)", () => {
  test("メンバーは legend 付きの group、SP は入れ子の group。すべての入力に accessible name がある", async () => {
    const { user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);

    expect(memberGroups(editor)).toHaveLength(2);
    for (const control of [
      ...within(editor).getAllByRole("combobox"),
      ...within(editor).getAllByRole("textbox"),
    ]) {
      expect(control).toHaveAccessibleName();
    }
  });

  test("キーボードだけで空の枠の種族の欄に行ける([一覧に戻る] から Tab)。選ぶと、その枠の欄が出る", async () => {
    const { user } = await renderScreen([TEAM_EMPTY]);
    const editor = await openEditor(user, TEAM_EMPTY.name);
    within(editor).getByRole("button", { name: teamMemberText.closeLabel }).focus();

    await user.tab();

    const species = select(memberGroup(editor, 1), teamMemberText.speciesLabel);
    expect(species).toHaveFocus();
    await user.selectOptions(species, "9001-000");
    expect(memberGroups(editor)).toHaveLength(1);
  });

  test("エラーは role=alert、保存済みの知らせは role=status(分けて読み上げる)", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    await setSp(user, memberGroup(editor, 1), "atk", "33");
    expect(within(editor).getAllByRole("alert").length).toBeGreaterThan(0);
    expect(within(editor).queryByRole("status")).toBeNull();

    await setSp(user, memberGroup(editor, 1), "atk", "32");
    await saveAndSucceed(user, client, editor);
    expect(within(editor).getByRole("status")).toBeInTheDocument();
    expect(within(editor).queryByRole("alert")).toBeNull();
  });
});

describe("AC-9 critic 指摘の修正(特性 null・送信中の無効化・SP の途中入力・不明なテラスタイプ)", () => {
  const NO_ABILITY: Schemas["TeamMember"] = { ...MEMBER_FIRE, abilityId: null };

  test("特性が null のメンバーは先頭の特性で埋めず「(未選択)」で開き、無変更で保存すると null のまま送る", async () => {
    const { client, user } = await renderScreen([team(TEAM_A.id, TEAM_A.name, [NO_ABILITY])]);
    const editor = await openEditor(user, TEAM_A.name);
    const abilitySelect = select(memberGroup(editor, 1), teamMemberText.abilityLabel);

    expect(abilitySelect.value).toBe("");
    expect(optionLabels(abilitySelect)[0]).toBe(teamMemberText.abilityUnset);
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members[0]?.abilityId ?? null).toBeNull();
  });

  test("オンライン(resolveSpecies 後)でも、特性が null のメンバーは「(未選択)」のまま null で保存される", async () => {
    const onlineMaster = limitedMaster(master, { speciesList: false, moves: false, effects: true });
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const { client, user } = await renderScreen([team(TEAM_A.id, TEAM_A.name, [NO_ABILITY])], {
      master: onlineMaster,
      masterSearch: search,
    });
    const editor = await openEditor(user, TEAM_A.name);
    await waitFor(() => {
      expect(search.resolvedKeys).toContain("9001-000");
    });
    const first = memberGroup(editor, 1);
    await waitFor(() => {
      expect(within(first).getByRole("combobox", { name: teamMemberText.speciesLabel })).toHaveValue(
        speciesName("9001-000"),
      );
    });

    expect(select(first, teamMemberText.abilityLabel).value).toBe("");
    await user.click(saveButton(editor));
    await waitFor(() => {
      expect(client.updateCalls).toHaveLength(1);
    });
    expect(lastCall(client.updateCalls, "update").args.input.members[0]?.abilityId ?? null).toBeNull();
  });

  test("SP 欄の途中入力(1e)は 0 として黙って保存せず、明示エラーで保存できない", async () => {
    const { client, user } = await renderScreen([TEAM_A]);
    const editor = await openEditor(user, TEAM_A.name);
    const first = memberGroup(editor, 1);
    await setSp(user, first, "def", "1e");

    expect(spInput(first, "def")).toHaveAttribute("aria-invalid", "true");
    expect(saveButton(editor)).toBeDisabled();
    expect(client.updateCalls).toHaveLength(0);
  });

  test("マスタのタイプ一覧に無いテラスタイプも、現在値として表示して保つ", async () => {
    const trimmed: MasterData = {
      ...master,
      typeChart: { ...master.typeChart, types: master.typeChart.types.filter((type) => type !== "fire") },
    };
    const { user } = await renderScreen([TEAM_A], { master: trimmed });
    const first = memberGroup(await openEditor(user, TEAM_A.name), 1);

    expect(select(first, teamMemberText.teraLabel).value).toBe("fire");
  });
});
