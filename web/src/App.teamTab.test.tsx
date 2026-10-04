// ADR-0332 §2・§3(F-08 / I-web-7)× ADR-0308: 構築名の欄を廃止したので、App.tabPersistence.test.tsx の
// 「構築名の入力は、計算タブへ行って戻っても残り…」の置き換え。取り込みの折りたたみの開閉とテキストが、
// タブを往復しても残り、構築 API を呼び直さないこと。team-svc は居ない(fetch は失敗)が、取り込みの入力は使える。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { teamShowdownText } from "./i18n/team";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
  window.sessionStorage.clear();
});

function teamCallCount(calls: readonly (readonly [string | URL | Request, ...unknown[]])[]): number {
  const urlOf = (input: string | URL | Request): string =>
    input instanceof Request ? input.url : input instanceof URL ? input.href : input;
  return calls.filter(([input]) => urlOf(input).includes("api/team/")).length;
}

describe("構築のタブ(F-08)も ADR-0308 の決まりに乗る", () => {
  test("取り込みの折りたたみの開閉とテキストは、計算タブへ行って戻っても残り、構築 API を呼び直さない", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(screen.getByRole("tab", { name: "構築" }));
    const summary = (await screen.findByText(teamShowdownText.importFoldLabel)).closest("summary");
    if (summary === null) {
      throw new Error("取り込みの折りたたみの summary が無い");
    }
    await user.click(summary);
    const region = screen.getByRole("region", { name: teamShowdownText.importRegionLabel });
    await user.click(within(region).getByRole("textbox", { name: teamShowdownText.importTextLabel }));
    await user.paste("テストほのお @ テストぼうぎょだま");
    await waitFor(() => {
      expect(teamCallCount(fetchMock.mock.calls)).toBeGreaterThan(0);
    });
    const callsAfterFirstVisit = teamCallCount(fetchMock.mock.calls);

    await user.click(screen.getByRole("tab", { name: "計算" }));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(screen.getByRole("tab", { name: "構築" }));

    const details = (await screen.findByText(teamShowdownText.importFoldLabel)).closest("details");
    expect(details?.open).toBe(true);
    expect(
      within(screen.getByRole("region", { name: teamShowdownText.importRegionLabel })).getByRole("textbox", {
        name: teamShowdownText.importTextLabel,
      }),
    ).toHaveValue("テストほのお @ テストぼうぎょだま");
    expect(teamCallCount(fetchMock.mock.calls)).toBe(callsAfterFirstVisit);
  });
});
