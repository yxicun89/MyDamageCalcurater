// ADR-0358(案 A)段 1: オフラインのとき、使える範囲を画面の上に 1 行で出し、API 専用のタブには「オンラインのみ」の理由を添える。
//   - 保存済みのポケモンの数(MasterCacheStore の species のキー数)を「○匹」で出す(0 は専用の案内)
//   - 計算・逆算はオフラインで使える。調整・素早さ・構築・お気に入りはオンラインのみ(理由を添える。ADR-0411 により押せなくはしない)
//   - オンラインでは何も出さず、理由は出ない

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { offlineScopeText } from "./i18n/offlineScope";
import { exampleMasterSource } from "./master/exampleSource";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});
afterEach(() => {
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

function renderApp(count: () => Promise<number>) {
  return render(
    <App engine={createFakeEngine()} masterSource={exampleMasterSource} savedSpeciesCount={count} />,
  );
}

function setOffline(): void {
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
}

describe("オフラインで使える範囲の表示", () => {
  test("オフライン: 保存済みの数と使える範囲を 1 行で出す", async () => {
    setOffline();
    renderApp(() => Promise.resolve(12));
    const note = await screen.findByRole("note", { name: offlineScopeText.regionLabel });
    await waitFor(() => {
      expect(note).toHaveTextContent(offlineScopeText.summary(12));
    });
    expect(note).toHaveTextContent("12匹");
    expect(note.textContent).not.toContain("種族");
  });

  test("オフライン: 保存済みが 0 なら、オンラインで一度開くと保存される案内を出す", async () => {
    setOffline();
    renderApp(() => Promise.resolve(0));
    const note = await screen.findByRole("note", { name: offlineScopeText.regionLabel });
    await waitFor(() => {
      expect(note).toHaveTextContent(offlineScopeText.empty);
    });
  });

  test("オフライン: 数が読めない(reject)ときも 0 として案内し、失敗させない", async () => {
    setOffline();
    renderApp(() => Promise.reject(new Error("idb")));
    const note = await screen.findByRole("note", { name: offlineScopeText.regionLabel });
    await waitFor(() => {
      expect(note).toHaveTextContent(offlineScopeText.empty);
    });
  });

  test("オンライン: 1 行は出さない", async () => {
    renderApp(() => Promise.resolve(3));
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.queryByRole("note", { name: offlineScopeText.regionLabel })).toBeNull();
  });

  test("オンラインからオフラインへ切り替えると出る", async () => {
    const user = userEvent.setup();
    renderApp(() => Promise.resolve(5));
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    await user.click(screen.getByRole("radio", { name: "この端末(オフライン)" }));
    const note = await screen.findByRole("note", { name: offlineScopeText.regionLabel });
    await waitFor(() => {
      expect(note).toHaveTextContent(offlineScopeText.summary(5));
    });
  });
});

describe("API 専用のタブはオフラインで「オンラインのみ」と分かる", () => {
  test.each(["構築", "素早さ", "記録"])(
    "オフライン: 上位のタブ「%s」に理由を添える(名前は変えず aria-describedby で結ぶ)。押せなくはしない",
    async (name) => {
      setOffline();
      const user = userEvent.setup();
      renderApp(() => Promise.resolve(1));
      await screen.findByRole("tablist", { name: "画面の切り替え" });
      const tab = screen.getByRole("tab", { name });
      expect(within(tab).getByText(offlineScopeText.onlineOnlyReason)).toBeInTheDocument();
      expect(tab).toHaveAccessibleDescription(offlineScopeText.onlineOnlyReason);
      // ADR-0411: API 専用の画面は計算モードに関係なく開ける(お気に入りは開くと案内を出す)。
      await user.click(tab);
      expect(tab).toHaveAttribute("aria-selected", "true");
    },
  );

  test("オフライン: ダメージ(計算・逆算)には理由を付けない。調整には付ける", async () => {
    setOffline();
    renderApp(() => Promise.resolve(1));
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.getByRole("tab", { name: "ダメージ" })).not.toHaveAccessibleDescription(
      offlineScopeText.onlineOnlyReason,
    );
    expect(screen.getByRole("tab", { name: "計算" })).not.toHaveAccessibleDescription(
      offlineScopeText.onlineOnlyReason,
    );
    expect(screen.getByRole("tab", { name: "逆算" })).not.toHaveAccessibleDescription(
      offlineScopeText.onlineOnlyReason,
    );
    expect(screen.getByRole("tab", { name: "調整" })).toHaveAccessibleDescription(
      offlineScopeText.onlineOnlyReason,
    );
  });

  test("オンライン: 理由は出さない", async () => {
    renderApp(() => Promise.resolve(1));
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.queryByText(offlineScopeText.onlineOnlyReason)).toBeNull();
  });

  test("オフラインからオンラインへ戻すと、理由が消える", async () => {
    setOffline();
    const user = userEvent.setup();
    renderApp(() => Promise.resolve(1));
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.getAllByText(offlineScopeText.onlineOnlyReason).length).toBeGreaterThan(0);
    await user.click(screen.getByRole("radio", { name: "サーバー(オンライン)" }));
    await waitFor(() => {
      expect(screen.queryByText(offlineScopeText.onlineOnlyReason)).toBeNull();
    });
  });
});
