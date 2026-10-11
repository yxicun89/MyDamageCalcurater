// I-web-13i(G-05、ADR-0339・ADR-0349): このアプリについての見た目の作り直し(docs/design.md「画面ごとの方向」のこのアプリの行)。
// 確かめること:
//   - 節の見出し(非公式表示・データの出典・データの扱い)はアイコン + 名前(アイコンは装飾)
//   - 説明の文は例外として残すが、1 文ずつの箇条(リスト)にして目で追える
//   - 削除ボタンは危険ボタン + ごみ箱アイコン(名前は従来のまま)。確認ダイアログの見出し・確認ボタンもアイコン付き
//   - 確認ダイアログの作法(初期フォーカス = キャンセル・Esc・フォーカスを戻す)は従来どおり
// 挙動は既存のテスト(AboutScreen.deviceData など)が守る。ここは新しい構造だけを見る。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { AboutScreen } from "./AboutScreen";
import type { DeviceDataDeleter } from "./deviceData/deleteDeviceData";
import { aboutText, deviceDataText } from "./i18n/ja";

const pending: DeviceDataDeleter = {
  deleteDeviceData: vi.fn(() => new Promise<never>(() => undefined)),
};

function renderAbout() {
  const user = userEvent.setup();
  render(
    <AboutScreen
      backHref="/calc"
      onBack={() => undefined}
      focusOnMount={false}
      recordClient={pending}
      teamClient={pending}
      onTeamDataDeleted={() => undefined}
    />,
  );
  return user;
}

describe("見出し", () => {
  test.each([aboutText.unofficialHeading, aboutText.dataSourcesHeading, deviceDataText.sectionHeading])(
    "h3「%s」はアイコン(装飾)+ 名前",
    (name) => {
      renderAbout();
      const h3 = screen.getByRole("heading", { level: 3, name });
      expect(h3.querySelector("svg[aria-hidden='true']")).not.toBeNull();
    },
  );
});

describe("文の見やすさ", () => {
  test("データの扱いの説明は 1 文ずつの箇条(文は全て残る)", () => {
    renderAbout();
    const list = screen.getByRole("list", { name: deviceDataText.sectionHeading });
    const items = within(list).getAllByRole("listitem");
    expect(items.map((item) => item.textContent)).toEqual([...deviceDataText.explanation]);
  });

  test("非公式の注記は全文が 1 つの要素に残る", () => {
    renderAbout();
    expect(screen.getByText(aboutText.unofficialNotice)).toBeInTheDocument();
  });
});

describe("削除ボタンと確認ダイアログ", () => {
  test("削除ボタンは危険ボタン + ごみ箱アイコン(名前は従来のまま)", () => {
    renderAbout();
    const button = screen.getByRole("button", { name: deviceDataText.deleteButton });
    expect(button).toHaveClass("ui-button--danger");
    expect(button.querySelector("svg[aria-hidden='true']")).not.toBeNull();
  });

  test("ダイアログ: 見出し・確認ボタンにアイコン、初期フォーカスはキャンセル、Esc で閉じて削除ボタンへ戻る", async () => {
    const user = renderAbout();
    const open = screen.getByRole("button", { name: deviceDataText.deleteButton });
    await user.click(open);
    const dialog = screen.getByRole("alertdialog");
    expect(within(dialog).getByRole("heading", { level: 4 }).querySelector("svg")).not.toBeNull();
    const confirm = within(dialog).getByRole("button", { name: deviceDataText.confirmAction });
    expect(confirm).toHaveClass("ui-button--danger");
    expect(confirm.querySelector("svg[aria-hidden='true']")).not.toBeNull();
    expect(within(dialog).getByText(deviceDataText.confirmMessage)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: deviceDataText.cancelAction })).toHaveFocus();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(open).toHaveFocus();
  });
});
