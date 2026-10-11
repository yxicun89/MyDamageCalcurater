// ADR-0332 §2・§3(F-08 / I-web-7)× ADR-0308: 構築名の欄を廃止したので、App.tabPersistence.test.tsx の
// 「構築名の入力は、計算タブへ行って戻っても残り…」の置き換え。タブを往復しても構築 API を
// 呼び直さないこと。team-svc は居ない(fetch は失敗)。(取り込みの折りたたみは G-03 で廃止。)

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { teamScreenText } from "./i18n/team";
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
  test("計算タブへ行って戻っても、構築 API を呼び直さない", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(screen.getByRole("tab", { name: "構築" }));
    await screen.findByRole("button", { name: teamScreenText.createLabel });
    await waitFor(() => {
      expect(teamCallCount(fetchMock.mock.calls)).toBeGreaterThan(0);
    });
    const callsAfterFirstVisit = teamCallCount(fetchMock.mock.calls);

    await user.click(screen.getByRole("tab", { name: "計算" }));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(screen.getByRole("tab", { name: "構築" }));

    await screen.findByRole("button", { name: teamScreenText.createLabel });
    expect(teamCallCount(fetchMock.mock.calls)).toBe(callsAfterFirstVisit);
  });
});
