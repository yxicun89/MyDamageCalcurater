// P5-5e(ADR-0321): 構築画面の Showdown 形式の取り込み・書き出し UI。TeamClient は fake、マスタは例データ+架空のメガ(test/megaMaster.ts)。
// 変換の純粋部は showdownFormat(ADR-0310)・取り込み計画は showdownImportPlan・名前引きのマスタは showdownMaster で別にテストする。
//
// F-08(ADR-0332)で、構築名を廃止し、取り込みは一覧の下の閉じた折りたたみの中、書き出しは編集画面の下の折りたたみの中へ移った。
//
// 受け入れ条件(AC):
//  取り込み(折りたたみ「Showdown 形式で取り込む」の中の領域「Showdown 形式から取り込む」。2段階: 内容を確認 → この内容で作成)
//   I-1  貼り付けて「内容を確認」を押すと、作成せず、取り込めるメンバー数(role="status")と問題の一覧を出す
//   I-2  「この内容で作成」で teamClient.create({members}) が1回呼ばれる(name キーは無い)。成功で一覧の先頭に「構築 N」で出て
//        (メンバー数つき)、完了の role="status" を出し、入力欄は空に戻り、その構築のメンバー編集を開ける
//   I-4  取り込めないメンバーがあっても、取り込める分だけで作る。問題の一覧に「何体目か」「理由(日本語)」「値」を出し、
//        error があれば role="alert"、warning だけなら role="status"。コードの生の文字列は出さない
//   I-5  空入力・全て落ちた場合は「取り込めるメンバーがいません」と問題を出し、「この内容で作成」は無効で create を呼ばない
//   I-6  7体以上は6体だけ作り、too_many_members を出す
//   I-7  メガ種族は持ち物を requiredItemId(ストーン)に直して create に渡し、補正を「取り込み時の補正」の一覧に出す。
//        ストーンがマスタに無いメガは持ち物を空にして出す
//   I-8  確認後にテキストを編集すると、作成は無効に戻り「内容を確認」し直すまで create できない(古いプレビューで作らない)
//   I-9  create が失敗したら role="alert"(見出し+サーバーの message)。テキストは残り、もう一度作成できる
//   I-10 送信中の二重クリックで create は1回だけ
//   I-11 一覧の無いマスタ(オンライン相当・speciesList=false)でも、masterSearch で種族・特性・技を引いて取り込める(メガ補正も効く)。
//        masterSearch が無いと種族は解決できず、問題を出して作らない
//   I-12 a11y: テキスト欄は label で引ける。確認中は role="status" で「名前を確認しています」を出し、確認ボタンを無効にする
//  書き出し(編集画面の下の折りたたみ「Showdown 形式で書き出す」の中に「「名前」を Showdown 形式で書き出す」)
//   E-1  押すと、読み取り専用の textarea(名前は「「名前」の書き出しテキスト」)に exportShowdownTeam の text を出し、textarea にフォーカスする。
//        API(create/update/get)は呼ばない
//   E-2  「コピー」で navigator.clipboard.writeText(text)。成功で role="status" 「コピーしました」
//   E-3  clipboard が無い・writeText が失敗したら、textarea を全選択してフォーカスし、手動コピーを案内する(role="status")
//   E-4  メンバー0体の構築は書き出しボタンが無効で、理由「メンバーがいないので書き出せません」が見える(aria-describedby)
//   E-5  書き出せなかった項目(マスタに名前が無い等)は ID を出さず、「書き出しの問題」の一覧に出す
//   E-6  一覧の無いマスタ(オンライン相当)でも masterSearch で種族名を引いて書き出せる(ID を出さない)
//   E-7  「書き出しを閉じる」で領域が消える。構築を切り替えると、書き出しの内容も切り替わる(編集は1構築ずつ)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { teamMemberText, teamScreenText, teamShowdownText } from "../i18n/team";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { createFakeTeamClient, flush, lastCall, type FakeTeamClient } from "../test/fakeTeamClient";
import { MEGA_FIRE, MEGA_FIRE_STONE, MEGA_ORPHAN, withMegaFixture } from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { exportShowdownTeam, type ShowdownIssue } from "./showdownFormat";
import { TeamScreen } from "./TeamScreen";
import { SERVER_DEFAULT_TEAM_NAME } from "./teamName";

type Schemas = components["schemas"];

let master: MasterData;
let online: MasterData;
beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
  online = limitedMaster(master, { speciesList: false, moves: false, effects: false });
});

const block = (first: string): string =>
  [
    first,
    "Ability: テストむこう",
    "EVs: 2 HP / 32 Atk / 32 Spe",
    "テストいじっぱり Nature",
    "- テストたいあたり",
    "- テストかえんパンチ",
  ].join("\n");

const VALID = block("テストほのお @ テストぼうぎょだま");
const VALID_MEMBER = {
  speciesKey: "9001-000",
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  moveIds: ["examplemovetackle", "examplemovefirepunch"],
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
};

function team(id: string, name: string, members: Schemas["TeamMember"][] = []): Schemas["Team"] {
  return { id, name, members, createdAt: "2026-10-03T12:00:00Z", updatedAt: "2026-10-03T12:00:00Z" };
}

const TEAM_ID = "11111111-1111-4111-8111-111111111111";
const NEW_ID = "22222222-2222-4222-8222-222222222222";

function exportMember(extra: Partial<Schemas["TeamMember"]> = {}): Schemas["TeamMember"] {
  return {
    speciesKey: "9001-000",
    moveIds: ["examplemovetackle"],
    itemId: "exampleitemdef",
    abilityId: "exampleabilitynone",
    natureId: "example-nature-atk",
    sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
    teraType: null,
    ...extra,
  };
}

interface Rendered {
  readonly client: FakeTeamClient;
  readonly user: UserEvent;
}

async function renderScreen(
  teams: readonly Schemas["Team"][] = [],
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

function importRegion(): HTMLElement {
  return screen.getByRole("region", { name: teamShowdownText.importRegionLabel });
}

/** 一覧の下の取り込みの折りたたみを開く(閉じていれば)。 */
async function openImportFold(user: UserEvent): Promise<void> {
  const details = screen.getByText(teamShowdownText.importFoldLabel).closest("details");
  if (details === null) {
    throw new Error("取り込みの折りたたみ(details)が無い");
  }
  if (!details.open) {
    await user.click(screen.getByText(teamShowdownText.importFoldLabel));
  }
}

async function fillImport(user: UserEvent, text: string): Promise<void> {
  await openImportFold(user);
  const textbox = within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel });
  await user.click(textbox);
  if (text !== "") {
    await user.paste(text);
  }
}

async function preview(user: UserEvent): Promise<void> {
  await user.click(within(importRegion()).getByRole("button", { name: teamShowdownText.importPreviewLabel }));
}

function createButton(): HTMLElement {
  return within(importRegion()).getByRole("button", { name: teamShowdownText.importCreateLabel });
}

function issueItems(): string[] {
  const list = within(importRegion()).queryByRole("list", { name: teamShowdownText.issuesLabel });
  return list === null
    ? []
    : within(list)
        .getAllByRole("listitem")
        .map((li) => li.textContent);
}

const VALID_ISSUE_ANY: ShowdownIssue = {
  severity: "error",
  code: "unresolved_name",
  memberIndex: 1,
  field: "species",
  value: "ふめいなもん",
};

describe("I-1・I-2 取り込み(2段階)", () => {
  test("確認では作成せず、取り込めるメンバー数を role=status で出す。作成で create が1回(name 無し)、成功すると一覧に出て入力が空に戻る", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, VALID);
    await preview(user);

    expect(await within(importRegion()).findByText(teamShowdownText.previewSummary(1))).toBeInTheDocument();
    expect(
      within(importRegion()).getByText(teamShowdownText.previewSummary(1)).closest("[role=status]"),
    ).not.toBeNull();
    expect(client.createCalls).toHaveLength(0);

    await user.click(createButton());
    expect(client.createCalls).toHaveLength(1);
    expect(lastCall(client.createCalls, "create").args).toStrictEqual({
      members: [expect.objectContaining(VALID_MEMBER)],
    });
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({
        ok: true,
        value: team(NEW_ID, SERVER_DEFAULT_TEAM_NAME, [exportMember()]),
      });
    });

    const list = screen.getByRole("list", { name: teamScreenText.listLabel });
    expect(within(list).getByText(teamScreenText.untitledTeamName(1))).toBeInTheDocument();
    expect(within(list).getByText(teamScreenText.memberCountLabel(1, 6))).toBeInTheDocument();
    expect(within(importRegion()).getByRole("status")).toHaveTextContent(teamShowdownText.importCreated(1));
    expect(
      within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel }),
    ).toHaveValue("");

    // 作成後はメンバー編集を開ける
    const display = teamScreenText.untitledTeamName(1);
    await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(display) }));
    expect(screen.getByRole("region", { name: teamMemberText.editorLabel(display) })).toBeInTheDocument();
  });
});

describe("I-4・I-5・I-6 問題の一覧と、取り込める分だけの作成", () => {
  test("2体目が未解決: 1体だけ作る。一覧に何体目・理由・値を日本語で出し、error なので role=alert", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, `${VALID}\n\n${block("ふめいなもん")}`);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));

    const items = issueItems();
    expect(items.length).toBeGreaterThan(0);
    const joined = items.join("\n");
    expect(joined).toContain("2体目");
    expect(joined).toContain("ふめいなもん");
    expect(joined).toContain(teamShowdownText.issueReason.unresolved_name);
    expect(joined).not.toContain("unresolved_name");
    expect(
      within(importRegion())
        .getByRole("list", { name: teamShowdownText.issuesLabel })
        .closest("[role=alert]"),
    ).not.toBeNull();
    expect(items).toContain(teamShowdownText.issueText({ ...VALID_ISSUE_ANY, value: "ふめいなもん" }));

    await user.click(createButton());
    expect(lastCall(client.createCalls, "create").args.members).toHaveLength(1);
  });

  test("warning だけ(Level: 100)は取り込め、一覧は role=status", async () => {
    const { user } = await renderScreen();
    await fillImport(user, `${VALID}\nLevel: 100`);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    const list = within(importRegion()).getByRole("list", { name: teamShowdownText.issuesLabel });
    expect(list.closest("[role=status]")).not.toBeNull();
    expect(list.closest("[role=alert]")).toBeNull();
    expect(createButton()).toBeEnabled();
  });

  test("空入力・全て落ちた: 取り込めるメンバーがいません。作成は無効で create を呼ばない", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, "");
    await preview(user);
    expect(await within(importRegion()).findByText(teamShowdownText.previewNone)).toBeInTheDocument();
    expect(issueItems().join("\n")).toContain(teamShowdownText.issueReason.empty_input);
    expect(createButton()).toBeDisabled();

    await user.click(within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel }));
    await user.paste(block("ふめいなもん"));
    await preview(user);
    expect(await within(importRegion()).findByText(teamShowdownText.previewNone)).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
    expect(client.createCalls).toHaveLength(0);
  });

  test("7体は6体だけ作り too_many_members を出す", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, Array.from({ length: 7 }, () => VALID).join("\n\n"));
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(6));
    expect(issueItems().join("\n")).toContain(teamShowdownText.issueReason.too_many_members);
    await user.click(createButton());
    expect(lastCall(client.createCalls, "create").args.members).toHaveLength(6);
  });
});

describe("I-7 メガの持ち物補正", () => {
  test("メガ種族の持ち物はストーンに直して create に渡し、補正を一覧に出す。ストーン無しのメガは空にして出す", async () => {
    const { client, user } = await renderScreen();
    const text = `${block(`${MEGA_FIRE.nameJa} @ テストぼうぎょだま`)}\n\n${block(MEGA_ORPHAN.nameJa)}`;
    await fillImport(user, text);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(2));

    const notes = within(importRegion()).getByRole("list", { name: teamShowdownText.notesLabel });
    const noteTexts = within(notes)
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(noteTexts).toEqual([
      teamShowdownText.megaNoteText(
        { kind: "mega_item_fixed", memberIndex: 0, speciesKey: MEGA_FIRE.key },
        MEGA_FIRE.nameJa,
      ),
      teamShowdownText.megaNoteText(
        { kind: "mega_item_unavailable", memberIndex: 1, speciesKey: MEGA_ORPHAN.key },
        MEGA_ORPHAN.nameJa,
      ),
    ]);

    await user.click(createButton());
    const members = lastCall(client.createCalls, "create").args.members;
    expect(members[0]?.itemId).toBe(MEGA_FIRE_STONE.id);
    expect(members[1]?.itemId).toBeNull();
  });
});

describe("I-8 古いプレビューで作らない", () => {
  test("確認後にテキストを編集すると作成が無効に戻り、確認し直すと有効になる", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, VALID);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    expect(createButton()).toBeEnabled();

    await user.type(
      within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel }),
      "\n",
    );
    expect(createButton()).toBeDisabled();
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    expect(createButton()).toBeEnabled();

    await user.type(
      within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel }),
      "x",
    );
    expect(createButton()).toBeDisabled();
    expect(client.createCalls).toHaveLength(0);
  });
});

describe("I-9・I-10 作成の失敗と二重送信", () => {
  test("失敗は role=alert(見出し+message)。テキストは残り、再度作成できる", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, VALID);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    await user.click(createButton());
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({
        ok: false,
        error: { code: "team_unavailable", message: "つながりません" },
      });
    });
    const alert = within(importRegion()).getByRole("alert");
    expect(alert).toHaveTextContent(teamShowdownText.importErrorHeading);
    expect(alert).toHaveTextContent("つながりません");
    expect(
      within(importRegion()).getByRole("textbox", { name: teamShowdownText.importTextLabel }),
    ).toHaveValue(VALID);
    expect(createButton()).toBeEnabled();
    await user.click(createButton());
    expect(client.createCalls).toHaveLength(2);
  });

  test("送信中の二重クリックで create は1回", async () => {
    const { client, user } = await renderScreen();
    await fillImport(user, VALID);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    await user.dblClick(createButton());
    expect(client.createCalls).toHaveLength(1);
  });
});

describe("I-11 一覧の無いマスタ(オンライン相当)", () => {
  test("masterSearch で種族・特性・技を引いて取り込める。メガ補正も効く", async () => {
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const { client, user } = await renderScreen([], { master: online, masterSearch: search });
    await fillImport(user, `${VALID}\n\n${block(MEGA_FIRE.nameJa)}`);
    await preview(user);
    await within(importRegion()).findByText(teamShowdownText.previewSummary(2));
    await user.click(createButton());
    const members = lastCall(client.createCalls, "create").args.members;
    expect(members[0]).toMatchObject(VALID_MEMBER);
    expect(members[1]).toMatchObject({ speciesKey: MEGA_FIRE.key, itemId: MEGA_FIRE_STONE.id });
  });

  test("masterSearch が無いと種族を解決できず、問題を出して作らない", async () => {
    const { client, user } = await renderScreen([], { master: online });
    await fillImport(user, VALID);
    await preview(user);
    expect(await within(importRegion()).findByText(teamShowdownText.previewNone)).toBeInTheDocument();
    expect(issueItems().join("\n")).toContain(teamShowdownText.issueReason.unresolved_name);
    expect(createButton()).toBeDisabled();
    expect(client.createCalls).toHaveLength(0);
  });
});

describe("I-12 a11y", () => {
  test("確認中は role=status で案内し確認ボタンを無効にする。解決したら元に戻る", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const inner = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const slow: MasterSpeciesSearch = {
      searchSpecies: async (q, s) => {
        await gate;
        return inner.searchSpecies(q, s);
      },
      resolveSpecies: (k, s) => inner.resolveSpecies(k, s),
    };
    const { user } = await renderScreen([], { master: online, masterSearch: slow });
    await fillImport(user, VALID);
    await preview(user);
    expect(within(importRegion()).getByText(teamShowdownText.importResolving)).toBeInTheDocument();
    expect(
      within(importRegion()).getByRole("button", { name: teamShowdownText.importPreviewLabel }),
    ).toBeDisabled();
    release();
    await within(importRegion()).findByText(teamShowdownText.previewSummary(1));
    expect(
      within(importRegion()).getByRole("button", { name: teamShowdownText.importPreviewLabel }),
    ).toBeEnabled();
  });
});

// ---------------------------------------------------------------- 書き出し

function exportButton(name: string): HTMLElement {
  return screen.getByRole("button", { name: teamShowdownText.exportLabel(name) });
}

/** 構築を開き、編集画面の下の書き出しの折りたたみを開く(閉じていれば)。 */
async function openExportFold(user: UserEvent, name: string): Promise<void> {
  await user.click(screen.getByRole("button", { name: teamMemberText.editLabel(name) }));
  const details = screen.getByText(teamShowdownText.exportFoldLabel).closest("details");
  if (details === null) {
    throw new Error("書き出しの折りたたみ(details)が無い");
  }
  if (!details.open) {
    await user.click(screen.getByText(teamShowdownText.exportFoldLabel));
  }
}

async function openExport(
  user: UserEvent,
  name: string,
): Promise<{ region: HTMLElement; textarea: HTMLTextAreaElement }> {
  await openExportFold(user, name);
  return exportOpened(user, name);
}

/** 折りたたみを開いた編集画面で、書き出しボタンを押して領域を出す。 */
async function exportOpened(
  user: UserEvent,
  name: string,
): Promise<{ region: HTMLElement; textarea: HTMLTextAreaElement }> {
  await user.click(exportButton(name));
  const region = await screen.findByRole("region", { name: teamShowdownText.exportRegionLabel(name) });
  const textarea = within(region).getByRole("textbox", { name: teamShowdownText.exportTextLabel(name) });
  if (!(textarea instanceof HTMLTextAreaElement)) {
    throw new Error("書き出しのテキスト欄が textarea ではない");
  }
  return { region, textarea };
}

describe("E-1 書き出し", () => {
  test("読み取り専用の textarea に exportShowdownTeam の text を出し、フォーカスする。API は呼ばない", async () => {
    const members = [exportMember(), exportMember({ itemId: null, nickname: "ニック" })];
    const { client, user } = await renderScreen([team(TEAM_ID, "出力構築", members)]);
    const { textarea } = await openExport(user, "出力構築");
    expect(textarea).toHaveAttribute("readonly");
    expect(textarea.value).toBe(exportShowdownTeam(members, master).text);
    expect(textarea.value).toContain("テストほのお");
    expect(textarea.value).not.toContain("9001-000");
    expect(textarea).toHaveFocus();
    expect(client.createCalls).toHaveLength(0);
    expect(client.updateCalls).toHaveLength(0);
    expect(client.getCalls).toHaveLength(0);
  });
});

describe("E-2・E-3 コピー", () => {
  test("成功: clipboard.writeText(text) と「コピーしました」(role=status)", async () => {
    const { user } = await renderScreen([team(TEAM_ID, "コピー", [exportMember()])]);
    const spy = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    const { region, textarea } = await openExport(user, "コピー");
    await user.click(within(region).getByRole("button", { name: teamShowdownText.exportCopyLabel }));
    expect(spy).toHaveBeenCalledWith(textarea.value);
    await waitFor(() => {
      expect(within(region).getByRole("status")).toHaveTextContent(teamShowdownText.exportCopied);
    });
  });

  test("writeText が失敗: textarea を全選択してフォーカスし、手動コピーを案内する", async () => {
    const { user } = await renderScreen([team(TEAM_ID, "失敗コピー", [exportMember()])]);
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new Error("denied"));
    const { region, textarea } = await openExport(user, "失敗コピー");
    textarea.blur();
    await user.click(within(region).getByRole("button", { name: teamShowdownText.exportCopyLabel }));
    await waitFor(() => {
      expect(within(region).getByRole("status")).toHaveTextContent(teamShowdownText.exportCopyFailed);
    });
    expect(textarea).toHaveFocus();
    expect(textarea.selectionStart).toBe(0);
    expect(textarea.selectionEnd).toBe(textarea.value.length);
  });

  test("clipboard API が無い環境でも同じ(例外を出さない)", async () => {
    const { user } = await renderScreen([team(TEAM_ID, "無し", [exportMember()])]);
    Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    const { region, textarea } = await openExport(user, "無し");
    await user.click(within(region).getByRole("button", { name: teamShowdownText.exportCopyLabel }));
    await waitFor(() => {
      expect(within(region).getByRole("status")).toHaveTextContent(teamShowdownText.exportCopyFailed);
    });
    expect(textarea.selectionEnd).toBe(textarea.value.length);
  });
});

describe("E-4・E-5 空の構築と書き出せない項目", () => {
  test("メンバー0体は書き出しボタンが無効で、理由が見えて aria-describedby で結ばれる", async () => {
    const { user } = await renderScreen([team(TEAM_ID, "空", [])]);
    await openExportFold(user, "空");
    const button = exportButton("空");
    expect(button).toBeDisabled();
    expect(screen.getByText(teamShowdownText.exportEmptyNotice)).toBeInTheDocument();
    expect(button).toHaveAccessibleDescription(teamShowdownText.exportEmptyNotice);
  });

  test("マスタに名前が無い持ち物は ID を出さず、書き出しの問題に出す", async () => {
    const { user } = await renderScreen([
      team(TEAM_ID, "欠け", [exportMember({ itemId: "no-such-item-id" })]),
    ]);
    const { region, textarea } = await openExport(user, "欠け");
    expect(textarea.value).not.toContain("no-such-item-id");
    const list = within(region).getByRole("list", { name: teamShowdownText.exportIssuesLabel });
    const text = within(list)
      .getAllByRole("listitem")
      .map((li) => li.textContent)
      .join("\n");
    expect(text).toContain("1体目");
    expect(text).toContain(teamShowdownText.issueReason.missing_name);
  });
});

describe("E-6 一覧の無いマスタ", () => {
  test("masterSearch で種族名を引いて書き出せる(ID を出さない)", async () => {
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const { user } = await renderScreen([team(TEAM_ID, "オンライン出力", [exportMember()])], {
      master: online,
      masterSearch: search,
    });
    const { textarea } = await openExport(user, "オンライン出力");
    expect(textarea.value).toContain("テストほのお");
    expect(textarea.value).toContain("テストたいあたり");
    expect(textarea.value).not.toContain("9001-000");
    expect(search.resolvedKeys).toContain("9001-000");
  });
});

describe("E-7 閉じる・構築の切り替え", () => {
  test("閉じると領域が消える。構築を切り替えると、書き出しの内容も切り替わる(取り違えない)", async () => {
    const a = team(TEAM_ID, "甲", [exportMember()]);
    const b = team(NEW_ID, "乙", [
      exportMember({ speciesKey: "9002-000", moveIds: ["examplemovewaterblast"] }),
    ]);
    const { user } = await renderScreen([a, b]);
    const first = await openExport(user, "甲");
    expect(first.textarea.value).toContain("テストほのお");

    await user.click(within(first.region).getByRole("button", { name: teamShowdownText.exportCloseLabel }));
    expect(screen.queryByRole("region", { name: teamShowdownText.exportRegionLabel("甲") })).toBeNull();

    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));
    const second = await openExport(user, "乙");
    expect(second.textarea.value).toContain("テストみず");
    expect(second.textarea.value).not.toContain("テストほのお");
    expect(screen.queryByRole("region", { name: teamShowdownText.exportRegionLabel("甲") })).toBeNull();
  });
});
