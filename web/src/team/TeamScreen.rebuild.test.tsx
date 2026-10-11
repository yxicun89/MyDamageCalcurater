// ADR-0332(F-08 / I-web-7): 構築の作り直し。TeamClient は fake、マスタは架空の例データ(オフライン。種族は <select>)。
// 確かめること(受け入れ条件 AC-1〜AC-9 のうち画面で見えるもの):
//   R-1 構築名の廃止と新規作成: 名前欄・名前変更が無い / [新しい構築] は name 無しの create を1回 / 成功したらすぐ6枠の編集画面 /
//       失敗は alert / 一覧が読めなくても押せる
//   R-2 一覧のカード: 既定名は「構築 N」・名前付きはそのまま / ui-card・アイコン列・n/6体・[開く]・[削除](2段階)
//   R-3 6枠: 常に6つの group。空の枠は種族の欄と案内だけ / 種族を選ぶと欄が展開 / タイプ色のカード
//   R-4 保存: update の body に name が無い・種族の決まった枠だけを枠の順に / SP の検査 / 成功・失敗
//   R-5 未保存の印 / R-6 入れ替え・外す / R-7 一覧に戻る(未保存なら2段階)
//   R-8 メガの持ち物固定・古いデータの補正(ADR-0320 を維持)
//   R-9 Showdown 形式の入口は無い(G-03 で廃止)
//   R-10 reloadToken が変わったら編集画面を閉じて一覧を取り直す
// 架空の ID・名前だけを使う(ADR-0002)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { megaItemText, statLetterJa, teamMemberText, teamScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeTeamClient, flush, lastCall, type FakeTeamClient } from "../test/fakeTeamClient";
import { MEGA_FIRE, MEGA_FIRE_STONE, MEGA_FIRE_STONE_LABEL, withMegaFixture } from "../test/megaMaster";
import { chooseTeamMove, teamMoveRowName, teamMoveRows, teamMoveTrigger } from "../test/teamMovePicker";
import { MAX_TEAM_MEMBERS, TeamScreen } from "./TeamScreen";
import { MAX_MEMBER_MOVES, SP_STATS } from "./teamMember";

type Schemas = components["schemas"];

let master: MasterData;
let megaMaster: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
  megaMaster = withMegaFixture(master);
});

// ---- 架空のデータ ----

const DEFAULT_NAME = "名称未設定"; // サーバーの既定名(ADR-0229)

const FIRE: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  nickname: "テスト愛称",
  moveIds: ["examplemovetackle", "examplemovefirepunch"],
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: "fire",
};

const ELECTRIC: Schemas["TeamMember"] = {
  speciesKey: "9004-000",
  moveIds: ["examplemovethunder"],
  itemId: null,
  abilityId: "exampleabilityadapt",
  natureId: "example-nature-spa",
  sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 32 },
  teraType: null,
};

function team(
  id: string,
  name: string,
  members: Schemas["TeamMember"][],
  createdAt = "2026-10-01T09:00:00Z",
): Schemas["Team"] {
  return { id, name, members, createdAt, updatedAt: createdAt };
}

const NAMED = team(
  "33333333-3333-4333-8333-333333333333",
  "テスト構築A",
  [FIRE, ELECTRIC],
  "2026-09-30T09:00:00Z",
);
const UNTITLED_OLD = team(
  "44444444-4444-4444-8444-444444444444",
  DEFAULT_NAME,
  [FIRE],
  "2026-10-01T09:00:00Z",
);
const UNTITLED_NEW = team("55555555-5555-4555-8555-555555555555", DEFAULT_NAME, [], "2026-10-02T09:00:00Z");
const CREATED_ID = "66666666-6666-4666-8666-666666666666";

// ---- 画面の組み立て ----

interface Rendered {
  readonly client: FakeTeamClient;
  readonly user: UserEvent;
  readonly rerender: (reloadToken: number) => void;
}

async function renderScreen(
  teams: readonly Schemas["Team"][] | "error",
  options: { master?: MasterData } = {},
): Promise<Rendered> {
  const client = createFakeTeamClient();
  const user = userEvent.setup();
  const view = render(<TeamScreen teamClient={client} master={options.master ?? master} reloadToken={0} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve(
      teams === "error"
        ? { ok: false, error: { code: "team_unavailable", message: "テストの接続失敗" } }
        : { ok: true, value: [...teams] },
    );
  });
  return {
    client,
    user,
    rerender: (reloadToken) => {
      view.rerender(
        <TeamScreen teamClient={client} master={options.master ?? master} reloadToken={reloadToken} />,
      );
    },
  };
}

function createButton(): HTMLElement {
  return screen.getByRole("button", { name: teamScreenText.createLabel });
}

function teamList(): HTMLElement {
  return screen.getByRole("list", { name: teamScreenText.listLabel });
}

function card(displayName: string): HTMLElement {
  const found = within(teamList())
    .getAllByRole("listitem")
    .find((item) => within(item).queryAllByText(displayName).length > 0);
  if (found === undefined) {
    throw new Error(`「${displayName}」のカードが無い`);
  }
  return found;
}

function editorRegion(displayName: string): HTMLElement {
  return screen.getByRole("region", { name: teamMemberText.editorLabel(displayName) });
}

async function openTeam(user: UserEvent, displayName: string): Promise<HTMLElement> {
  await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(displayName) }));
  return editorRegion(displayName);
}

/** [新しい構築] を押して create を成功させ、開いた編集画面を返す。 */
async function createNew(rendered: Rendered, members: Schemas["TeamMember"][] = []): Promise<HTMLElement> {
  await rendered.user.click(createButton());
  await flush(() => {
    lastCall(rendered.client.createCalls, "create").resolve({
      ok: true,
      value: team(CREATED_ID, DEFAULT_NAME, members, "2026-10-04T09:00:00Z"),
    });
  });
  return editorRegion(teamScreenText.untitledTeamName(1));
}

function slot(editor: HTMLElement, position: number): HTMLElement {
  return within(editor).getByRole("group", { name: teamMemberText.memberLegend(position) });
}

function slots(editor: HTMLElement): HTMLElement[] {
  return within(editor).getAllByRole("group", { name: /^\d体目$/ });
}

function combo(group: HTMLElement, name: string): HTMLSelectElement {
  const element = within(group).getByRole("combobox", { name });
  if (!(element instanceof HTMLSelectElement)) {
    throw new Error(`「${name}」は select ではない`);
  }
  return element;
}

function speciesSelect(group: HTMLElement): HTMLSelectElement {
  return combo(group, teamMemberText.speciesLabel);
}

function spInput(group: HTMLElement, stat: (typeof SP_STATS)[number]): HTMLElement {
  return within(group).getByRole("textbox", { name: teamMemberText.spLabel(statLetterJa[stat]) });
}

function saveButton(editor: HTMLElement): HTMLElement {
  return within(editor).getByRole("button", { name: teamMemberText.saveLabel });
}

function backButton(): HTMLElement {
  return screen.getByRole("button", { name: teamMemberText.closeLabel });
}

function speciesName(key: string): string {
  const found = master.species.find((species) => species.key === key);
  if (found === undefined) {
    throw new Error(`例データに種族 ${key} が無い`);
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

/** 保存を押し、update の応答を成功で返す(サーバーと同じく name の省略には既定名を補う)。 */
async function saveAndSucceed(user: UserEvent, client: FakeTeamClient, editor: HTMLElement): Promise<void> {
  await user.click(saveButton(editor));
  await waitFor(() => {
    expect(client.updateCalls.length).toBeGreaterThan(0);
  });
  const call = lastCall(client.updateCalls, "update");
  await flush(() => {
    call.resolve({
      ok: true,
      value: {
        id: call.args.teamId,
        name: call.args.input.name ?? DEFAULT_NAME,
        members: call.args.input.members,
        createdAt: "2026-10-01T09:00:00Z",
        updatedAt: "2026-10-04T10:00:00Z",
      },
    });
  });
}

// =====================================================================

describe("R-1 構築名の廃止と新規作成", () => {
  test("構築名の入力欄・名前変更のボタンは無い", async () => {
    await renderScreen([NAMED]);
    expect(screen.queryByRole("textbox", { name: /構築名/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /名前を変更/ })).toBeNull();
  });

  test("[新しい構築] は name を持たない body で create を1回だけ呼ぶ(二重押下でも1回)", async () => {
    const { client, user } = await renderScreen([]);
    await user.click(createButton());
    await user.click(createButton());

    expect(client.createCalls).toHaveLength(1);
    const args = lastCall(client.createCalls, "create").args;
    expect(args).toStrictEqual({ members: [] });
    expect("name" in args).toBe(false);
    expect(createButton()).toBeDisabled();
  });

  test("成功したら一覧に足し、すぐその構築の6枠の編集画面を開く(一覧は隠す)", async () => {
    const rendered = await renderScreen([]);
    const editor = await createNew(rendered);

    expect(slots(editor)).toHaveLength(MAX_TEAM_MEMBERS);
    expect(screen.queryByRole("list", { name: teamScreenText.listLabel })).toBeNull();
    expect(screen.queryByRole("button", { name: teamScreenText.createLabel })).toBeNull();
    // 「名称未設定」は画面に出さない。
    expect(document.body).not.toHaveTextContent(DEFAULT_NAME);
    expect(rendered.client.updateCalls).toHaveLength(0);
  });

  test("失敗したら role=alert に見出しと message を出し、編集画面は開かず一覧は残る。もう一度押せる", async () => {
    const { client, user } = await renderScreen([NAMED]);
    await user.click(createButton());
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({
        ok: false,
        error: { code: "internal_error", message: "テストのサーバーエラー" },
      });
    });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(teamScreenText.createErrorHeading);
    expect(alert).toHaveTextContent("テストのサーバーエラー");
    expect(screen.queryByRole("region", { name: /のメンバー編集$/ })).toBeNull();
    expect(within(teamList()).getAllByRole("listitem")).toHaveLength(1);
    expect(createButton()).toBeEnabled();
    await user.click(createButton());
    expect(client.createCalls).toHaveLength(2);
  });

  test("一覧が読めなくても [新しい構築] は押せる(ADR-0309 §4)", async () => {
    const { client, user } = await renderScreen("error");
    expect(screen.getByRole("alert")).toHaveTextContent(teamScreenText.loadErrorHeading);
    expect(createButton()).toBeEnabled();
    await user.click(createButton());
    expect(client.createCalls).toHaveLength(1);
  });
});

describe("R-2 一覧のカード", () => {
  test("既定名の構築は作成の古い順に「構築 N」、名前付きはそのまま。並びは応答のまま", async () => {
    await renderScreen([UNTITLED_NEW, NAMED, UNTITLED_OLD]);
    const items = within(teamList()).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent(teamScreenText.untitledTeamName(2));
    expect(items[1]).toHaveTextContent(NAMED.name);
    expect(items[2]).toHaveTextContent(teamScreenText.untitledTeamName(1));
    expect(document.body).not.toHaveTextContent(DEFAULT_NAME);
  });

  test("カードは ui-card。アイコン列(メンバーごとに名前付きの img)・n/6体・[開く](主)・[削除](危険)。書き出しは一覧に無い", async () => {
    await renderScreen([NAMED]);
    const item = card(NAMED.name);
    expect(item).toHaveClass("ui-card");

    const icons = within(item).getByRole("group", { name: teamScreenText.memberIconsLabel(NAMED.name) });
    expect(within(icons).getAllByRole("img")).toHaveLength(2);
    expect(within(icons).getByRole("img", { name: speciesName(FIRE.speciesKey) })).toBeInTheDocument();
    expect(within(icons).getByRole("img", { name: speciesName(ELECTRIC.speciesKey) })).toBeInTheDocument();
    // 画像の manifest が無いので、どれもタイプ色のエンブレム(ADR-0325)。
    expect(within(icons).getAllByTestId("type-emblem")).toHaveLength(2);

    expect(item).toHaveTextContent(teamScreenText.memberCountLabel(2, MAX_TEAM_MEMBERS));
    expect(within(item).getByRole("button", { name: teamMemberText.editLabel(NAMED.name) })).toHaveClass(
      "ui-button",
      "ui-button--primary",
    );
    expect(within(item).getByRole("button", { name: teamScreenText.deleteLabel(NAMED.name) })).toHaveClass(
      "ui-button",
      "ui-button--danger",
    );
  });

  test("アイコンの名前: マスタに無い種族は「N体目」", async () => {
    const unknown = team("77777777-7777-4777-8777-777777777777", "テスト不明構築", [
      { ...FIRE, speciesKey: "9999-999" },
    ]);
    await renderScreen([unknown]);
    const icons = within(card(unknown.name)).getByRole("group", {
      name: teamScreenText.memberIconsLabel(unknown.name),
    });
    expect(within(icons).getByRole("img", { name: teamScreenText.unknownMemberIcon(1) })).toBeInTheDocument();
  });

  test("削除は2段階(表示名で案内)。確定まで remove を呼ばず、確定で消える", async () => {
    const { client, user } = await renderScreen([UNTITLED_OLD, NAMED]);
    const display = teamScreenText.untitledTeamName(1);
    await user.click(
      within(card(display)).getByRole("button", { name: teamScreenText.deleteLabel(display) }),
    );
    expect(client.removeCalls).toHaveLength(0);
    expect(screen.getByText(teamScreenText.deleteConfirmNotice(display))).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(display) }));
    expect(lastCall(client.removeCalls, "remove").args).toBe(UNTITLED_OLD.id);
    await flush(() => {
      lastCall(client.removeCalls, "remove").resolve({ ok: true, value: undefined });
    });
    expect(within(teamList()).getAllByRole("listitem")).toHaveLength(1);
    expect(screen.queryByText(display)).toBeNull();
  });

  test("1件も無いときは、次にすることが分かる案内", async () => {
    await renderScreen([]);
    expect(screen.getByText(teamScreenText.emptyNotice)).toBeInTheDocument();
  });
});

describe("R-3 6枠の編集画面", () => {
  test("開くと常に6枠。種族の決まった枠は全項目、空の枠は種族の欄と案内だけ(alert も [外す] も無い)", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    expect(slots(editor)).toHaveLength(MAX_TEAM_MEMBERS);

    for (const position of [1, 2]) {
      const group = slot(editor, position);
      for (let move = 1; move <= MAX_MEMBER_MOVES; move += 1) {
        expect(teamMoveTrigger(group, move)).toBeInTheDocument();
      }
      expect(combo(group, teamMemberText.itemLabel)).toBeInTheDocument();
      expect(combo(group, teamMemberText.natureLabel)).toBeInTheDocument();
      expect(combo(group, teamMemberText.teraLabel)).toBeInTheDocument();
      for (const stat of SP_STATS) {
        expect(spInput(group, stat)).toBeInTheDocument();
      }
      expect(
        within(group).getByRole("button", { name: teamMemberText.removeLabel(position) }),
      ).toBeInTheDocument();
    }
    expect(speciesSelect(slot(editor, 1))).toHaveValue(FIRE.speciesKey);
    expect(speciesSelect(slot(editor, 2))).toHaveValue(ELECTRIC.speciesKey);

    for (const position of [3, 4, 5, 6]) {
      const group = slot(editor, position);
      expect(speciesSelect(group)).toHaveValue("");
      expect(within(group).getByText(teamMemberText.emptySlotHint)).toBeInTheDocument();
      expect(within(group).queryByRole("combobox", { name: teamMemberText.moveLabel(1) })).toBeNull();
      expect(within(group).queryByRole("combobox", { name: teamMemberText.itemLabel })).toBeNull();
      expect(within(group).queryByRole("textbox")).toBeNull();
      expect(within(group).queryByRole("button", { name: teamMemberText.removeLabel(position) })).toBeNull();
      expect(within(group).queryByRole("alert")).toBeNull();
    }
  });

  test("枠はカード(ui-card)。種族の決まった枠だけタイプ色(--card-type)", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    const filled = slot(editor, 1).closest<HTMLElement>(".ui-card");
    const empty = slot(editor, 3).closest<HTMLElement>(".ui-card");
    expect(filled).not.toBeNull();
    expect(empty).not.toBeNull();
    expect(filled?.style.getPropertyValue("--card-type")).toContain("--type-fire");
    expect(empty?.style.getPropertyValue("--card-type")).toBe("");
  });

  test("空の枠で種族を選ぶと欄が展開し、技の候補はその種族の覚える技すべて(変化技も含む)", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.selectOptions(speciesSelect(slot(editor, 3)), "9002-000");

    const group = slot(editor, 3);
    expect(within(group).queryByText(teamMemberText.emptySlotHint)).toBeNull();
    // 並びはタイプ順(ADR-0341・ADR-0350)。変化技(なきごえ)も候補に含まれ、先頭は「(なし)」の行。
    const rows = await teamMoveRows(user, group, 1);
    expect(rows.map(teamMoveRowName)).toEqual([
      teamMemberText.moveNone,
      moveName("examplemovegrowl"),
      moveName("examplemovewaterblast"),
    ]);
    expect(rows[0]?.getAttribute("data-move-id")).toBe("");
    expect(rows.map((row) => row.getAttribute("data-move-id"))).toContain("examplemovegrowl");
    expect(combo(group, teamMemberText.itemLabel)).toBeEnabled();
    expect(within(group).getByRole("button", { name: teamMemberText.removeLabel(3) })).toBeInTheDocument();
  });

  test("[メンバーを追加] は無い", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    expect(within(editor).queryByRole("button", { name: /メンバーを追加/ })).toBeNull();
  });
});

describe("R-4 保存", () => {
  test("新しい構築の2枠目に種族・技・SP を入れて保存: update は name 無しで、種族の決まった枠だけを送る", async () => {
    const rendered = await renderScreen([]);
    const { client, user } = rendered;
    const editor = await createNew(rendered);

    await user.selectOptions(speciesSelect(slot(editor, 2)), FIRE.speciesKey);
    await chooseTeamMove(user, slot(editor, 2), 1, "examplemovetackle");
    await user.clear(spInput(slot(editor, 2), "atk"));
    await user.type(spInput(slot(editor, 2), "atk"), "32");
    await user.click(saveButton(editor));

    expect(client.updateCalls).toHaveLength(1);
    const { teamId, input } = lastCall(client.updateCalls, "update").args;
    expect(teamId).toBe(CREATED_ID);
    expect(Object.keys(input)).toEqual(["members"]);
    expect(input.members).toHaveLength(1);
    expect(input.members[0]).toMatchObject({
      speciesKey: FIRE.speciesKey,
      moveIds: ["examplemovetackle"],
      sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 },
    });
  });

  test("何も変えずに保存すると、name 無しで全メンバーをそのまま送る(ニックネームも落とさない)", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.click(saveButton(editor));
    const { input } = lastCall(client.updateCalls, "update").args;
    expect("name" in input).toBe(false);
    expect(input.members).toEqual([FIRE, ELECTRIC]);
  });

  test("SP の合計 67 は理由を出して保存できず、66 に戻すと保存できる", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    const group = slot(editor, 1);
    await user.clear(spInput(group, "hp"));
    await user.type(spInput(group, "hp"), "3");

    expect(within(group).getByRole("alert")).toHaveTextContent(teamMemberText.spTotalError(1, 66));
    expect(saveButton(editor)).toBeDisabled();

    await user.clear(spInput(group, "hp"));
    await user.type(spInput(group, "hp"), "2");
    expect(within(group).queryByRole("alert")).toBeNull();
    expect(saveButton(editor)).toBeEnabled();
  });

  test("送信中は [保存] と [一覧に戻る] を押せない。成功すると「保存しました」(role=status)", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.click(saveButton(editor));
    expect(saveButton(editor)).toBeDisabled();
    expect(backButton()).toBeDisabled();
    await user.click(saveButton(editor));
    expect(client.updateCalls).toHaveLength(1);

    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({ ok: true, value: { ...NAMED, name: DEFAULT_NAME } });
    });
    expect(within(editor).getByRole("status")).toHaveTextContent(teamMemberText.savedNotice);
    expect(backButton()).toBeEnabled();
  });

  test("失敗は role=alert にサーバーの message。下書きは残る", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.selectOptions(speciesSelect(slot(editor, 3)), "9002-000");
    await user.click(saveButton(editor));
    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({
        ok: false,
        error: { code: "invalid_input", message: "テストの入力エラー" },
      });
    });
    const alert = within(editor).getByText("テストの入力エラー").closest("[role=alert]");
    expect(alert).not.toBeNull();
    expect(alert).toHaveTextContent(teamMemberText.saveErrorHeading);
    expect(speciesSelect(slot(editor, 3))).toHaveValue("9002-000");
  });
});

describe("R-5 未保存の印", () => {
  test("開いた直後は出ない。変えると出て、保存に成功すると消える", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    expect(within(editor).queryByText(teamMemberText.unsavedNotice)).toBeNull();

    await user.selectOptions(combo(slot(editor, 1), teamMemberText.natureLabel), "example-nature-def");
    expect(within(editor).getByText(teamMemberText.unsavedNotice)).toBeInTheDocument();

    await saveAndSucceed(user, client, editor);
    expect(within(editor).queryByText(teamMemberText.unsavedNotice)).toBeNull();
  });
});

describe("R-6 入れ替え・外す", () => {
  test("1体目の [上へ] は無効。下へで隣の枠と入れ替わり、保存の順になる", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    expect(
      within(slot(editor, 1)).getByRole("button", { name: teamMemberText.moveUpLabel(1) }),
    ).toBeDisabled();

    await user.click(within(slot(editor, 1)).getByRole("button", { name: teamMemberText.moveDownLabel(1) }));
    expect(speciesSelect(slot(editor, 1))).toHaveValue(ELECTRIC.speciesKey);
    expect(speciesSelect(slot(editor, 2))).toHaveValue(FIRE.speciesKey);

    await user.click(saveButton(editor));
    expect(lastCall(client.updateCalls, "update").args.input.members.map((m) => m.speciesKey)).toEqual([
      ELECTRIC.speciesKey,
      FIRE.speciesKey,
    ]);
  });

  test("空の枠とも入れ替わる。6体目の [下へ] は無効", async () => {
    const { user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.click(within(slot(editor, 2)).getByRole("button", { name: teamMemberText.moveDownLabel(2) }));
    expect(speciesSelect(slot(editor, 2))).toHaveValue("");
    expect(speciesSelect(slot(editor, 3))).toHaveValue(ELECTRIC.speciesKey);

    await user.selectOptions(speciesSelect(slot(editor, 6)), "9003-000");
    expect(
      within(slot(editor, 6)).getByRole("button", { name: teamMemberText.moveDownLabel(6) }),
    ).toBeDisabled();
  });

  test("外すとその枠だけ空に戻り(他の枠は動かない)、フォーカスはその枠の「ポケモン」欄へ", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.click(within(slot(editor, 1)).getByRole("button", { name: teamMemberText.removeLabel(1) }));

    expect(speciesSelect(slot(editor, 1))).toHaveValue("");
    expect(within(slot(editor, 1)).getByText(teamMemberText.emptySlotHint)).toBeInTheDocument();
    expect(speciesSelect(slot(editor, 2))).toHaveValue(ELECTRIC.speciesKey);
    expect(speciesSelect(slot(editor, 1))).toHaveFocus();

    await user.click(saveButton(editor));
    expect(lastCall(client.updateCalls, "update").args.input.members).toEqual([ELECTRIC]);
  });
});

describe("R-7 一覧に戻る", () => {
  test("変更が無ければすぐ一覧に戻る(API は呼ばない)", async () => {
    const { client, user } = await renderScreen([NAMED]);
    await openTeam(user, NAMED.name);
    await user.click(backButton());
    expect(screen.queryByRole("region", { name: teamMemberText.editorLabel(NAMED.name) })).toBeNull();
    expect(teamList()).toBeInTheDocument();
    expect(client.updateCalls).toHaveLength(0);
    expect(client.getCalls).toHaveLength(0);
  });

  test("未保存なら2段階: [編集を続ける] で下書きが残り、[保存せずに戻る] で捨てる(開き直すと保存済みの値)", async () => {
    const { client, user } = await renderScreen([NAMED]);
    const editor = await openTeam(user, NAMED.name);
    await user.selectOptions(combo(slot(editor, 1), teamMemberText.natureLabel), "example-nature-def");

    await user.click(backButton());
    expect(screen.getByText(teamMemberText.leaveConfirmNotice)).toBeInTheDocument();
    expect(editorRegion(NAMED.name)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: teamMemberText.leaveCancelLabel }));
    expect(screen.queryByText(teamMemberText.leaveConfirmNotice)).toBeNull();
    expect(combo(slot(editorRegion(NAMED.name), 1), teamMemberText.natureLabel)).toHaveValue(
      "example-nature-def",
    );

    await user.click(backButton());
    await user.click(screen.getByRole("button", { name: teamMemberText.leaveDiscardLabel }));
    expect(screen.queryByRole("region", { name: teamMemberText.editorLabel(NAMED.name) })).toBeNull();
    expect(client.updateCalls).toHaveLength(0);

    const reopened = await openTeam(user, NAMED.name);
    expect(combo(slot(reopened, 1), teamMemberText.natureLabel)).toHaveValue(FIRE.natureId);
  });

  test("保存してから戻ると、一覧のカードのメンバー数が変わっている(list は読み直さない)", async () => {
    const rendered = await renderScreen([]);
    const { client, user } = rendered;
    const editor = await createNew(rendered);
    await user.selectOptions(speciesSelect(slot(editor, 1)), FIRE.speciesKey);
    await saveAndSucceed(user, client, editor);
    await user.click(backButton());

    expect(card(teamScreenText.untitledTeamName(1))).toHaveTextContent(
      teamScreenText.memberCountLabel(1, MAX_TEAM_MEMBERS),
    );
    expect(client.listCalls).toHaveLength(1);
  });
});

describe("R-8 メガの持ち物固定・古いデータの補正(ADR-0320 を維持)", () => {
  test("空の枠でメガ種族を選ぶと持ち物がストーンに固定され、保存に載る", async () => {
    const rendered = await renderScreen([], { master: megaMaster });
    const { client, user } = rendered;
    const editor = await createNew(rendered);
    await user.selectOptions(speciesSelect(slot(editor, 1)), MEGA_FIRE.key);

    const item = combo(slot(editor, 1), teamMemberText.itemLabel);
    expect(item).toBeDisabled();
    expect(item).toHaveValue(MEGA_FIRE_STONE.id);

    await user.click(saveButton(editor));
    expect(lastCall(client.updateCalls, "update").args.input.members[0]?.itemId).toBe(MEGA_FIRE_STONE.id);
  });

  test("別の持ち物を持つ古いメガのデータは、開くとストーンに直り、通知と未保存の印が出る(自動では保存しない)", async () => {
    const old = team("88888888-8888-4888-8888-888888888888", "テスト古いメガ", [
      { ...FIRE, speciesKey: MEGA_FIRE.key, itemId: "exampleitemdef", moveIds: [] },
    ]);
    const { client, user } = await renderScreen([old], { master: megaMaster });
    const editor = await openTeam(user, old.name);

    expect(combo(slot(editor, 1), teamMemberText.itemLabel)).toHaveValue(MEGA_FIRE_STONE.id);
    expect(
      within(slot(editor, 1)).getByText(megaItemText.correctedNotice(MEGA_FIRE_STONE_LABEL)),
    ).toHaveAttribute("role", "status");
    expect(within(editor).getByText(teamMemberText.unsavedNotice)).toBeInTheDocument();
    expect(client.updateCalls).toHaveLength(0);
  });
});

describe("R-9 Showdown 形式の入口は無い(G-03 で廃止。ADR-0342)", () => {
  test("一覧にも編集画面にも、Showdown の取り込み・書き出しの折りたたみが無い", async () => {
    const { user } = await renderScreen([NAMED]);
    expect(document.querySelector("details")).toBeNull();
    expect(screen.queryByText(/Showdown/)).toBeNull();

    await openTeam(user, NAMED.name);
    expect(document.querySelector("details")).toBeNull();
    expect(screen.queryByText(/Showdown/)).toBeNull();
  });
});

describe("R-10 reloadToken", () => {
  test("編集中に reloadToken が変わると、編集画面を閉じて一覧を取り直す", async () => {
    const rendered = await renderScreen([NAMED]);
    await openTeam(rendered.user, NAMED.name);
    rendered.rerender(1);

    expect(screen.queryByRole("region", { name: teamMemberText.editorLabel(NAMED.name) })).toBeNull();
    expect(rendered.client.listCalls).toHaveLength(2);
    expect(screen.getByText(teamScreenText.loadingNotice)).toBeInTheDocument();
  });
});

describe("R-11 画面の切り替えでフォーカスを失わない(ADR-0332 §7)", () => {
  test("編集画面を開くと見出しへ、一覧に戻るとその構築の [開く] へ、[編集を続ける] の後は [一覧に戻る] へ", async () => {
    const { user } = await renderScreen([NAMED, UNTITLED_OLD]);
    const editor = await openTeam(user, NAMED.name);
    expect(within(editor).getByRole("heading", { name: NAMED.name })).toHaveFocus();

    await user.selectOptions(combo(slot(editor, 1), teamMemberText.natureLabel), "example-nature-def");
    await user.click(backButton());
    await user.click(screen.getByRole("button", { name: teamMemberText.leaveCancelLabel }));
    expect(backButton()).toHaveFocus();

    await user.click(backButton());
    await user.click(screen.getByRole("button", { name: teamMemberText.leaveDiscardLabel }));
    expect(screen.getByRole("button", { name: teamMemberText.editLabel(NAMED.name) })).toHaveFocus();
  });

  test("新しい構築から開いた編集画面も見出しにフォーカスし、戻ると、その構築の [開く] に戻る", async () => {
    const rendered = await renderScreen([]);
    const editor = await createNew(rendered);
    const display = teamScreenText.untitledTeamName(1);
    expect(within(editor).getByRole("heading", { name: display })).toHaveFocus();
    await rendered.user.click(backButton());
    expect(screen.getByRole("button", { name: teamMemberText.editLabel(display) })).toHaveFocus();
  });
});
