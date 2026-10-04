// P5-5 PR-A1: 構築ビルダーの画面(ADR-0309 §4)。TeamClient は fake(props で注入)。
// F-08(ADR-0332)で構築名を廃止したため、新規作成は [新しい構築] だけ、名前変更は無い。
// 新規作成・6枠の編集・保存・未保存の印などの新しい画面の詳細は TeamScreen.rebuild.test.tsx。
// 確かめること(受け入れ条件):
//   AC-2 一覧: マウント時に list を1回だけ呼ぶ / 読み込み中・空・失敗・一覧の4状態
//   AC-3 新規作成: 成功したら一覧に足す(読み直さない)
//   AC-5 削除: 2段階(window.confirm は使わない)・確定まで remove を呼ばない・失敗しても行は残る
//   AC-6 立て直し: 一覧が読めなくても [新しい構築] は使える
// 架空の構築名・ID だけを使う(実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { teamMemberText, teamScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { MAX_TEAM_MEMBERS, TeamScreen } from "./TeamScreen";
import type { TeamClient, TeamResult } from "./teamClient";

type Schemas = components["schemas"];

// PR-A2 から TeamScreen は master を受け取る(一覧・作成・名前変更・削除のこのテストでは使わない)。
let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

// ---- fake の TeamClient(呼び出しを記録し、テストが応答を返す。JudgeScreen.test.tsx と同じ形) ----

interface PendingCall<A, T> {
  readonly args: A;
  resolve(result: TeamResult<T>): void;
}

interface FakeTeamClient extends TeamClient {
  readonly listCalls: PendingCall<null, Schemas["Team"][]>[];
  readonly createCalls: PendingCall<Schemas["TeamInput"], Schemas["Team"]>[];
  readonly getCalls: PendingCall<string, Schemas["Team"]>[];
  readonly updateCalls: PendingCall<
    { readonly teamId: string; readonly input: Schemas["TeamInput"] },
    Schemas["Team"]
  >[];
  readonly removeCalls: PendingCall<string, void>[];
}

function createFakeTeamClient(): FakeTeamClient {
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
    list() {
      return new Promise((resolve) => {
        listCalls.push({ args: null, resolve });
      });
    },
    create(input) {
      return new Promise((resolve) => {
        createCalls.push({ args: structuredClone(input), resolve });
      });
    },
    get(teamId) {
      return new Promise((resolve) => {
        getCalls.push({ args: teamId, resolve });
      });
    },
    update(teamId, input) {
      return new Promise((resolve) => {
        updateCalls.push({ args: { teamId, input: structuredClone(input) }, resolve });
      });
    },
    remove(teamId) {
      return new Promise((resolve) => {
        removeCalls.push({ args: teamId, resolve });
      });
    },
  };
}

/** 1回だけ解決を流す(JudgeScreen.test.tsx と同じ形)。 */
async function flush(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

function lastCall<A, T>(calls: readonly PendingCall<A, T>[], name: string): PendingCall<A, T> {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}

// ---- 架空の構築 ----

const TEAM_A: Schemas["Team"] = {
  id: "33333333-3333-4333-8333-333333333333",
  name: "テスト構築A",
  members: [],
  createdAt: "2026-09-26T12:00:00Z",
  updatedAt: "2026-09-26T12:00:00Z",
};

const TEAM_B: Schemas["Team"] = {
  id: "44444444-4444-4444-8444-444444444444",
  name: "テスト構築B",
  members: [],
  createdAt: "2026-09-25T12:00:00Z",
  updatedAt: "2026-09-25T12:00:00Z",
};

/** 画面を描き、list の応答(成功)を流す。 */
async function renderWithTeams(teams: readonly Schemas["Team"][]): Promise<FakeTeamClient> {
  const client = createFakeTeamClient();
  render(<TeamScreen teamClient={client} master={master} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [...teams] });
  });
  return client;
}

/** 画面を描き、list を失敗させる。 */
async function renderWithListError(error: { code: string; message: string }): Promise<FakeTeamClient> {
  const client = createFakeTeamClient();
  render(<TeamScreen teamClient={client} master={master} />);
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: false, error });
  });
  return client;
}

function teamList(): HTMLElement {
  return screen.getByRole("list", { name: teamScreenText.listLabel });
}

function teamItems(): HTMLElement[] {
  return within(teamList()).getAllByRole("listitem");
}

/** 名前で構築の行を引く(行の中のボタンは within で引く)。 */
function teamItem(name: string): HTMLElement {
  const item = teamItems().find((candidate) => candidate.textContent.includes(name));
  if (item === undefined) {
    throw new Error(`「${name}」の行が無い`);
  }
  return item;
}

function createButton(): HTMLElement {
  return screen.getByRole("button", { name: teamScreenText.createLabel });
}

describe("AC-2 一覧(マウント時に1回だけ読む)", () => {
  test("マウント時に list を1回だけ呼び、応答が来るまで読み込み中を出す(書き込みの API は呼ばない)", () => {
    const client = createFakeTeamClient();
    render(<TeamScreen teamClient={client} master={master} />);

    expect(client.listCalls).toHaveLength(1);
    expect(client.createCalls).toHaveLength(0);
    expect(client.updateCalls).toHaveLength(0);
    expect(client.removeCalls).toHaveLength(0);
    expect(screen.getByText(teamScreenText.loadingNotice)).toBeInTheDocument();
    // 読み込み中はまだ一覧も空の案内も出さない(取り違えない)。
    expect(screen.queryByRole("list", { name: teamScreenText.listLabel })).toBeNull();
    expect(screen.queryByText(teamScreenText.emptyNotice)).toBeNull();
  });

  test("成功したら、構築ごとに名前・メンバー数・最終更新を応答の順のまま出す", async () => {
    await renderWithTeams([TEAM_A, TEAM_B]);

    expect(screen.queryByText(teamScreenText.loadingNotice)).toBeNull();
    const items = teamItems();
    expect(items).toHaveLength(2);
    // 応答の並び(更新の新しい順)をそのまま出す(Web で並べ替えない)。
    expect(items.map((item) => item.textContent)).toEqual([
      expect.stringContaining(TEAM_A.name),
      expect.stringContaining(TEAM_B.name),
    ]);
    // PR-A1 ではメンバーは常に空(0/6体)。最終更新は日付が分かる形で出す。
    expect(teamItem(TEAM_A.name)).toHaveTextContent(teamScreenText.memberCountLabel(0, MAX_TEAM_MEMBERS));
    expect(teamItem(TEAM_A.name)).toHaveTextContent("2026-09-26");
    expect(teamItem(TEAM_B.name)).toHaveTextContent("2026-09-25");
  });

  test("1件も無ければ空の案内を出す(一覧もエラーも出さない)", async () => {
    await renderWithTeams([]);

    expect(screen.getByText(teamScreenText.emptyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: teamScreenText.listLabel })).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("領域の名前と一覧の見出しを持つ(ARIA)", async () => {
    await renderWithTeams([TEAM_A]);

    expect(screen.getByRole("region", { name: teamScreenText.regionLabel })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: teamScreenText.listHeading })).toBeInTheDocument();
    expect(teamList().tagName).toBe("UL");
  });
});

describe("AC-6 一覧が読めないときの立て直し", () => {
  test.each([
    ["サーバーのエラー", { code: "internal_error", message: "internal error" }],
    ["Web 側の team_unavailable", { code: "team_unavailable", message: "構築の API に接続できません" }],
  ])("%s: role=alert に見出しと message を出し、[新しい構築] は使える", async (_name, error) => {
    const client = await renderWithListError(error);

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(teamScreenText.loadErrorHeading);
    expect(alert).toHaveTextContent(error.message);
    expect(screen.queryByText(teamScreenText.loadingNotice)).toBeNull();
    // 一覧が読めなくても、作ることはできる(ADR-0309 §4)。
    expect(createButton()).toBeEnabled();
    expect(client.listCalls).toHaveLength(1);
  });
});

describe("AC-3 新規作成([新しい構築]。成功したら一覧に足す)", () => {
  test("成功したら応答の Team を一覧の先頭に足す(編集画面から戻ると見える。list を呼び直さない)", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A]);

    await user.click(createButton());
    const created: Schemas["Team"] = {
      id: "55555555-5555-4555-8555-555555555555",
      name: "テスト構築C",
      members: [],
      createdAt: "2026-09-27T12:00:00Z",
      updatedAt: "2026-09-27T12:00:00Z",
    };
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({ ok: true, value: created });
    });
    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));

    expect(teamItems().map((item) => item.textContent)).toEqual([
      expect.stringContaining(created.name),
      expect.stringContaining(TEAM_A.name),
    ]);
    expect(client.listCalls).toHaveLength(1);
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("list() 応答と書き込みの競合(critic 指摘: 消えて見えるレースコンディション)", () => {
  const CREATED: Schemas["Team"] = {
    id: "66666666-6666-4666-8666-666666666666",
    name: "テスト構築C",
    members: [],
    createdAt: "2026-09-27T12:00:00Z",
    updatedAt: "2026-09-27T12:00:00Z",
  };

  test("list() が届く前に create() が成功すると、後から届いた古い list() の一覧で上書きしない", async () => {
    const user = userEvent.setup();
    const client = createFakeTeamClient();
    render(<TeamScreen teamClient={client} master={master} />);

    // list() はまだ保留のまま(応答を流さない)。この間に新規作成が成功し、編集画面が開く。
    await user.click(createButton());
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({ ok: true, value: CREATED });
    });
    expect(
      screen.getByRole("region", { name: teamMemberText.editorLabel(CREATED.name) }),
    ).toBeInTheDocument();

    // list() の応答が、作成より前に取得された古いスナップショット(空)としてあとから届く。
    await flush(() => {
      lastCall(client.listCalls, "list").resolve({ ok: true, value: [] });
    });

    // 古い list() で上書きされず、作成した構築は消えない(編集画面も開いたまま、戻ると一覧にある)。
    expect(
      screen.getByRole("region", { name: teamMemberText.editorLabel(CREATED.name) }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));
    expect(teamItems().map((item) => item.textContent)).toEqual([expect.stringContaining(CREATED.name)]);
    expect(screen.queryByText(teamScreenText.emptyNotice)).toBeNull();
  });

  test("list() が届く前に update() が成功しても、後から届いた古い list() の一覧で上書きしない", async () => {
    const user = userEvent.setup();
    const client = createFakeTeamClient();
    render(<TeamScreen teamClient={client} master={master} />);

    // list() が解決する前に、作成 → 1体目に種族を選んで保存まで進める。
    await user.click(createButton());
    await flush(() => {
      lastCall(client.createCalls, "create").resolve({ ok: true, value: CREATED });
    });
    const editor = screen.getByRole("region", { name: teamMemberText.editorLabel(CREATED.name) });
    const first = within(editor).getByRole("group", { name: teamMemberText.memberLegend(1) });
    await user.selectOptions(
      within(first).getByRole("combobox", { name: teamMemberText.speciesLabel }),
      "9001-000",
    );
    await user.click(within(editor).getByRole("button", { name: teamMemberText.saveLabel }));
    const saved: Schemas["Team"] = {
      ...CREATED,
      members: lastCall(client.updateCalls, "update").args.input.members,
      updatedAt: "2026-09-27T13:00:00Z",
    };
    await flush(() => {
      lastCall(client.updateCalls, "update").resolve({ ok: true, value: saved });
    });

    // list() の応答が、これらの書き込みより前に取得された古いスナップショット(空)としてあとから届く。
    await flush(() => {
      lastCall(client.listCalls, "list").resolve({ ok: true, value: [] });
    });

    await user.click(screen.getByRole("button", { name: teamMemberText.closeLabel }));
    expect(teamItems().map((item) => item.textContent)).toEqual([expect.stringContaining(CREATED.name)]);
    expect(teamItem(CREATED.name)).toHaveTextContent(teamScreenText.memberCountLabel(1, MAX_TEAM_MEMBERS));
  });
});

describe("AC-5 削除(2段階。window.confirm は使わない)", () => {
  function deleteButton(name: string): HTMLElement {
    return within(teamItem(name)).getByRole("button", { name: teamScreenText.deleteLabel(name) });
  }

  test("「削除」を押しただけでは remove を呼ばず、確認と2つの選択肢を出す", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A]);

    await user.click(deleteButton(TEAM_A.name));

    expect(client.removeCalls).toHaveLength(0);
    expect(screen.getByText(teamScreenText.deleteConfirmNotice(TEAM_A.name))).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM_A.name) }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: teamScreenText.deleteCancelLabel(TEAM_A.name) }),
    ).toBeInTheDocument();
  });

  test("確定すると remove をその id で1回だけ呼び、成功したら行が消える(他の行は残る)", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A, TEAM_B]);

    await user.click(deleteButton(TEAM_A.name));
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM_A.name) }));

    expect(client.removeCalls).toHaveLength(1);
    expect(lastCall(client.removeCalls, "remove").args).toBe(TEAM_A.id);

    await flush(() => {
      lastCall(client.removeCalls, "remove").resolve({ ok: true, value: undefined });
    });

    await waitFor(() => {
      expect(teamItems()).toHaveLength(1);
    });
    expect(teamItems()[0]?.textContent).toContain(TEAM_B.name);
    expect(client.listCalls).toHaveLength(1);
  });

  test("やめると remove を呼ばず、確認が消えて行は残る", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A]);

    await user.click(deleteButton(TEAM_A.name));
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteCancelLabel(TEAM_A.name) }));

    expect(client.removeCalls).toHaveLength(0);
    expect(screen.queryByText(teamScreenText.deleteConfirmNotice(TEAM_A.name))).toBeNull();
    expect(teamItems()).toHaveLength(1);
  });

  test("最後の1件を消すと、空の案内に戻る", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A]);

    await user.click(deleteButton(TEAM_A.name));
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM_A.name) }));
    await flush(() => {
      lastCall(client.removeCalls, "remove").resolve({ ok: true, value: undefined });
    });

    expect(await screen.findByText(teamScreenText.emptyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: teamScreenText.listLabel })).toBeNull();
  });

  test("失敗したら role=alert に見出しと message を出し、行は残る", async () => {
    const user = userEvent.setup();
    const client = await renderWithTeams([TEAM_A]);

    await user.click(deleteButton(TEAM_A.name));
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM_A.name) }));
    await flush(() => {
      lastCall(client.removeCalls, "remove").resolve({
        ok: false,
        error: { code: "not_found", message: "team not found" },
      });
    });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(teamScreenText.deleteErrorHeading);
    expect(alert).toHaveTextContent("team not found");
    expect(teamItems()).toHaveLength(1);
  });
});
