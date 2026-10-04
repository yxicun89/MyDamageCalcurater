// F-12(I-web-6、ADR-0331 §3): 構築画面への共通部品の適用。
// 新規作成・Showdown 取り込みの囲みはカード(ui-card)、主な操作は主ボタン、取り消し・名前の変更などは副ボタン、
// 削除は危険ボタン。一覧は表のような行(ui-rows)、読み込み中・空・失敗は案内(ui-notice)。
// ボタンの名前(「作成」「「<名前>」を削除」など)・役割・既存のクラスは変えない。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { teamScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeTeamClient, flush, lastCall, type FakeTeamClient } from "../test/fakeTeamClient";
import { TeamScreen } from "./TeamScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const TEAM: Schemas["Team"] = {
  id: "55555555-5555-4555-8555-555555555555",
  name: "テスト構築V",
  members: [],
  createdAt: "2026-10-01T12:00:00Z",
  updatedAt: "2026-10-01T12:00:00Z",
};

function renderScreen(): FakeTeamClient {
  const client = createFakeTeamClient();
  render(<TeamScreen teamClient={client} master={master} />);
  return client;
}

async function renderWithTeams(teams: readonly Schemas["Team"][]): Promise<FakeTeamClient> {
  const client = renderScreen();
  await flush(() => {
    lastCall(client.listCalls, "list").resolve({ ok: true, value: [...teams] });
  });
  return client;
}

describe("構築画面の見た目の部品", () => {
  test("新規作成の囲みは ui-card、「作成」は主ボタン(名前は変えない)", () => {
    renderScreen();
    const create = screen.getByRole("button", { name: teamScreenText.createLabel });
    expect(create).toHaveClass("ui-button", "ui-button--primary");
    expect(create.closest(".team-screen__create")).toHaveClass("ui-card");
  });

  test("構築名の欄は ui-field の中にある(フォーカスリング・入力の角丸)", () => {
    renderScreen();
    const input = screen.getByRole("textbox", { name: teamScreenText.nameLabel });
    expect(input.closest(".ui-field")).not.toBeNull();
  });

  test("読み込み中の案内は ui-notice ui-notice--loading", () => {
    renderScreen();
    expect(screen.getByText(teamScreenText.loadingNotice)).toHaveClass("ui-notice", "ui-notice--loading");
  });

  test("1件も無いときの案内は ui-notice ui-notice--empty", async () => {
    await renderWithTeams([]);
    expect(screen.getByText(teamScreenText.emptyNotice)).toHaveClass("ui-notice", "ui-notice--empty");
  });

  test("一覧の取得に失敗したら role=alert のまま ui-notice ui-notice--error", async () => {
    const client = renderScreen();
    await flush(() => {
      lastCall(client.listCalls, "list").resolve({
        ok: false,
        error: { code: "team_unavailable", message: "テストの失敗" },
      });
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveClass("ui-notice", "ui-notice--error", "team-screen__error");
  });

  test("一覧は ui-rows。行の「名前を変更」は副ボタン、「削除」は危険ボタン", async () => {
    await renderWithTeams([TEAM]);
    const list = screen.getByRole("list", { name: teamScreenText.listLabel });
    expect(list).toHaveClass("ui-rows", "team-screen__list");
    expect(screen.getByRole("button", { name: teamScreenText.renameLabel(TEAM.name) })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
    expect(screen.getByRole("button", { name: teamScreenText.deleteLabel(TEAM.name) })).toHaveClass(
      "ui-button",
      "ui-button--danger",
    );
  });

  test("削除の確認: 「削除を確定」は危険ボタン、「削除をやめる」は副ボタン", async () => {
    const user = userEvent.setup();
    await renderWithTeams([TEAM]);
    await user.click(screen.getByRole("button", { name: teamScreenText.deleteLabel(TEAM.name) }));
    const row = screen.getByRole("listitem");
    expect(
      within(row).getByRole("button", { name: teamScreenText.deleteConfirmLabel(TEAM.name) }),
    ).toHaveClass("ui-button", "ui-button--danger");
    expect(
      within(row).getByRole("button", { name: teamScreenText.deleteCancelLabel(TEAM.name) }),
    ).toHaveClass("ui-button", "ui-button--secondary");
  });
});
