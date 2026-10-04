// F-12 PR-2(I-web-12、ADR-0336 §2): 「このアプリについて」への共通部品の適用。
// 節は ui-card、出典リストは ui-rows(ゼブラ)、データ削除・再試行・ダイアログのボタンは ui-button
// (削除は ui-button--danger、キャンセル・再試行は ui-button--secondary)、削除確認ダイアログの本体は ui-card、
// 状態表示(role=status)は ui-notice、失敗(role=alert)は ui-notice--error、戻るリンクは ui-button--secondary。
// 既存のクラス(about__*)・アクセシブルな名前・role(alertdialog)・テキストは変えない。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { AboutScreen } from "./AboutScreen";
import type { DeviceDataDeleter } from "./deviceData/deleteDeviceData";
import { aboutText, deviceDataText } from "./i18n/ja";

type DeleteResult = Awaited<ReturnType<DeviceDataDeleter["deleteDeviceData"]>>;

function deleter(result: DeleteResult | "pending"): DeviceDataDeleter {
  return {
    deleteDeviceData: vi.fn(() =>
      result === "pending" ? new Promise<DeleteResult>(() => undefined) : Promise.resolve(result),
    ),
  };
}

function renderAbout(
  record: DeviceDataDeleter = deleter("pending"),
  team: DeviceDataDeleter = deleter("pending"),
) {
  const user = userEvent.setup();
  render(
    <AboutScreen
      backHref="/calc"
      onBack={() => undefined}
      focusOnMount={false}
      recordClient={record}
      teamClient={team}
      onTeamDataDeleted={() => undefined}
    />,
  );
  return user;
}

describe("節・出典", () => {
  test("各節(about__section)は ui-card", () => {
    renderAbout();
    const sections = document.querySelectorAll(".about__section");
    expect(sections.length).toBeGreaterThanOrEqual(3);
    for (const section of sections) {
      expect(section).toHaveClass("ui-card");
    }
  });

  test("出典リスト(list)は ui-rows(既存の about__sources も残す)", () => {
    renderAbout();
    expect(screen.getByRole("list", { name: aboutText.dataSourcesHeading })).toHaveClass(
      "ui-rows",
      "about__sources",
    );
  });

  test("計算に戻るリンクは ui-button ui-button--secondary(リンクのまま)", () => {
    renderAbout();
    const back = screen.getByRole("link", { name: aboutText.backLabel });
    expect(back).toHaveClass("ui-button", "ui-button--secondary", "about__back");
  });
});

describe("データの扱い・削除確認ダイアログ", () => {
  test("「この端末のデータを削除」は ui-button ui-button--danger(about__button も残す)", () => {
    renderAbout();
    expect(screen.getByRole("button", { name: deviceDataText.deleteButton })).toHaveClass(
      "ui-button",
      "ui-button--danger",
      "about__button",
    );
  });

  test("ダイアログ(alertdialog)は ui-card、「キャンセル」は secondary、「削除する」は danger", async () => {
    const user = renderAbout();
    await user.click(screen.getByRole("button", { name: deviceDataText.deleteButton }));
    const dialog = screen.getByRole("alertdialog", { name: deviceDataText.deleteButton });
    expect(dialog).toHaveClass("ui-card", "about__dialog");
    expect(within(dialog).getByRole("button", { name: deviceDataText.cancelAction })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
    expect(within(dialog).getByRole("button", { name: deviceDataText.confirmAction })).toHaveClass(
      "ui-button",
      "ui-button--danger",
    );
  });

  test("実行中の状態(role=status)は ui-notice ui-notice--loading", async () => {
    const user = renderAbout();
    await user.click(screen.getByRole("button", { name: deviceDataText.deleteButton }));
    await user.click(screen.getByRole("button", { name: deviceDataText.confirmAction }));
    expect(screen.getByRole("status")).toHaveClass("ui-notice", "ui-notice--loading", "about__status");
  });

  test("失敗(role=alert)は ui-notice ui-notice--error、「もう一度削除する」は ui-button--secondary", async () => {
    const failed: DeleteResult = { ok: false, error: { code: "upstream_unavailable", message: "x" } };
    const user = renderAbout(deleter(failed), deleter(failed));
    await user.click(screen.getByRole("button", { name: deviceDataText.deleteButton }));
    await user.click(screen.getByRole("button", { name: deviceDataText.confirmAction }));
    expect(await screen.findByRole("alert")).toHaveClass("ui-notice", "ui-notice--error");
    expect(screen.getByRole("button", { name: deviceDataText.retryButton })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
  });
});
