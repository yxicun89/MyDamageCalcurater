// F-12(I-web-6、ADR-0331 §2・§3): アプリのシェル(ヘッダ・タブ列・フッター)の見た目の部品。
// 見た目のためのクラスとアイコンを足すだけで、アクセシブルな名前・選択状態・DOM の役割(tab/tablist/link)は変えない。
// タブのアイコンは装飾(aria-hidden)。タブの名前は今までどおり画面の名前そのもの(exact 一致)。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { App } from "./App";
import { SCREENS } from "./app/screens";
import { appText } from "./i18n/ja";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/calc");
});

afterEach(() => {
  window.history.replaceState(null, "", "/");
});

async function renderApp(): Promise<void> {
  render(<App engine={createFakeEngine()} />);
  await screen.findByRole("tablist", { name: appText.tabsLabel });
}

describe("タブ列", () => {
  test("tablist は ui-tabs、各 tab は ui-tab(既存の app-tabs__* のクラスも残す)", async () => {
    await renderApp();
    const tablist = screen.getByRole("tablist", { name: appText.tabsLabel });
    expect(tablist).toHaveClass("ui-tabs", "app-tabs__list");
    for (const tab of within(tablist).getAllByRole("tab")) {
      expect(tab).toHaveClass("ui-tab", "app-tabs__tab");
    }
  });

  test.each(SCREENS.map((registered) => [registered.label, registered.id] as const))(
    "%s のタブは装飾のアイコン(aria-hidden の svg.ui-icon)を持ち、名前は画面名のまま",
    async (label) => {
      await renderApp();
      const tab = screen.getByRole("tab", { name: label });
      const icons = tab.querySelectorAll("svg");
      expect(icons).toHaveLength(1);
      const icon = icons[0];
      expect(icon).toHaveClass("ui-icon");
      expect(icon).toHaveAttribute("aria-hidden", "true");
      // 画面に出る文字も画面名のまま(アイコンは文字を持たない)。
      expect(tab).toHaveTextContent(new RegExp(`^${label}$`));
    },
  );

  test("タブごとにアイコンの形が違う(どの画面かが形でも分かる)", async () => {
    await renderApp();
    const shapes = screen.getAllByRole("tab").map((tab) => tab.querySelector("svg")?.innerHTML ?? "");
    expect(new Set(shapes).size).toBe(shapes.length);
  });

  test("選択の状態は aria-selected のまま(クラスで選択を表さない)。クリックで移る", async () => {
    await renderApp();
    const user = userEvent.setup();
    const calcTab = screen.getByRole("tab", { name: "計算" });
    const speedTab = screen.getByRole("tab", { name: "素早さ" });
    expect(calcTab).toHaveAttribute("aria-selected", "true");
    await user.click(speedTab);
    expect(speedTab).toHaveAttribute("aria-selected", "true");
    expect(calcTab).toHaveAttribute("aria-selected", "false");
    expect(speedTab.className).toBe(calcTab.className);
  });
});

describe("ヘッダ・計算モード・フッター", () => {
  test("h1 はアプリ名のまま(アイコンを足しても名前は変わらない)", async () => {
    await renderApp();
    expect(screen.getByRole("heading", { level: 1, name: appText.title })).toBeInTheDocument();
  });

  test("計算モードのラジオは ui-chip。選択中は ui-chip--selected", async () => {
    await renderApp();
    const group = screen.getByRole("radiogroup", { name: appText.calcModeGroupLabel });
    const labels = [appText.calcModeOfflineLabel, appText.calcModeOnlineLabel];
    for (const label of labels) {
      const radio = within(group).getByRole("radio", { name: label });
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
  });

  test("フッターのリンクは名前「このアプリについて」のまま、装飾のアイコン(about)を持つ", async () => {
    await renderApp();
    const footer = screen.getByRole("contentinfo");
    const link = within(footer).getByRole("link", { name: "このアプリについて" });
    const icon = link.querySelector("svg");
    expect(icon).toHaveClass("ui-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });
});
