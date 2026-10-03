// issue #328(P6-18、Web 分。ADR-0314): 「このアプリについて」(情報ページ)。
// アプリ下部のフッターに「このアプリについて」リンクを置き、押すと /about(ADR-0300 §1 のパス連動。タブには入れない)で
// 非公式の注記とデータの出典4件を出す。文言は i18n/ja.ts の aboutText が正(src/i18n/aboutText.test.ts)。
// ここは画面の振る舞い: フッターのリンク・/about の表示・戻る導線・戻る/進む・未知パス・見出し構造・
// キーボード・他タブの入力の保持(ADR-0308)。文言はあえてリテラルで書く(aboutText の取り違えを検出するため)。
// jsdom の window.history を使う。vitest では import.meta.env.BASE_URL は "/"。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

const NOTICE =
  "このアプリは個人が私的に使うための非公式ツールです。" +
  "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。" +
  "ポケモン・Pokémon および関連する名称は各社の商標です。";

const SOURCES = [
  ["ダメージ計算の検証", "@smogon/calc(MIT License)"],
  ["ポケモン・技・習得技の照合", "Pokémon Showdown(MIT License)"],
  ["日本語名・図鑑番号", "PokeAPI"],
  ["使用可能なポケモン等の基準", "Pokémon HOME・Pokémon Champions の公式情報"],
] as const;

const pendingMasterSource: MasterSource = {
  load: () => new Promise<MasterData>(() => undefined),
};

const failingMasterSource: MasterSource = {
  load: () => Promise.reject(new Error("master down")),
};

function setPath(path: string): void {
  window.history.replaceState(null, "", path);
}

function simulatePopState(path: string): void {
  act(() => {
    window.history.replaceState(null, "", path);
    window.dispatchEvent(new PopStateEvent("popstate", { state: null }));
  });
}

function footerLink(): HTMLElement {
  return within(screen.getByRole("contentinfo")).getByRole("link", { name: "このアプリについて" });
}

function aboutHeading(): HTMLElement {
  return screen.getByRole("heading", { level: 2, name: "このアプリについて" });
}

beforeEach(() => {
  window.localStorage.clear();
  setPath("/");
});

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
  setPath("/");
});

describe("フッターの入口", () => {
  test.each(["/calc", "/reverse"])(
    "%s にも、フッター(contentinfo)に /about へのリンクがある(main の外・タブには入らない)",
    async (path) => {
      setPath(path);
      render(<App engine={createFakeEngine()} />);
      await screen.findByRole("tablist", { name: "画面の切り替え" });
      const link = footerLink();
      expect(link).toHaveAttribute("href", "/about");
      expect(screen.getByRole("main")).not.toContainElement(screen.getByRole("contentinfo"));
      expect(screen.getAllByRole("tab")).toHaveLength(7);
      expect(screen.queryByRole("tab", { name: "このアプリについて" })).toBeNull();
    },
  );

  test("マスタの読み込み中・失敗中でもフッターのリンクがある(情報ページはマスタを使わない)", async () => {
    const { unmount } = render(<App engine={createFakeEngine()} masterSource={pendingMasterSource} />);
    expect(footerLink()).toBeInTheDocument();
    unmount();
    render(<App engine={createFakeEngine()} masterSource={failingMasterSource} />);
    await screen.findByRole("alert");
    expect(footerLink()).toBeInTheDocument();
  });

  test("リンクを押すと /about を pushState し(履歴を1件積む)、情報ページが出る。見出しにフォーカスが移る", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(footerLink());

    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(window.location.pathname).toBe("/about");
    expect(aboutHeading()).toHaveFocus();
    expect(document.title).toBe("このアプリについて | pokecalc");
  });

  test("キーボードだけで開ける(リンクにフォーカスして Enter)", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    act(() => {
      footerLink().focus();
    });
    await user.keyboard("{Enter}");
    expect(window.location.pathname).toBe("/about");
    expect(aboutHeading()).toBeInTheDocument();
  });
});

describe("/about の表示", () => {
  test("非公式の注記(完全一致)とデータの出典4件(同じ順)を出す", async () => {
    setPath("/about");
    render(<App engine={createFakeEngine()} />);

    expect(await screen.findByText(NOTICE)).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "データの出典" });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(4);
    SOURCES.forEach(([title, detail], index) => {
      const item = items[index];
      expect(item).toHaveTextContent(title);
      expect(item).toHaveTextContent(detail);
    });
  });

  test("直接開いても URL を書き換えず(/calc へ置き換えない)、タブは出さない(どのタブも選択中でない)", async () => {
    setPath("/about");
    const pushSpy = vi.spyOn(window.history, "pushState");
    const replaceSpy = vi.spyOn(window.history, "replaceState");
    render(<App engine={createFakeEngine()} />);
    await screen.findByText(NOTICE);
    expect(window.location.pathname).toBe("/about");
    expect(pushSpy).not.toHaveBeenCalled();
    expect(replaceSpy).not.toHaveBeenCalled();
    expect(screen.queryByRole("tab")).toBeNull();
    expect(document.title).toBe("このアプリについて | pokecalc");
  });

  test("マスタの読み込み中でも失敗中でも表示される(マスタを使わない)", async () => {
    setPath("/about");
    const { unmount } = render(<App engine={createFakeEngine()} masterSource={pendingMasterSource} />);
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    unmount();
    render(<App engine={createFakeEngine()} masterSource={failingMasterSource} />);
    expect(await screen.findByText(NOTICE)).toBeInTheDocument();
  });

  test("末尾の / 1つは同じ画面(/about/)。深いパス・大文字小文字違いは未知のパスとして /calc に置き換える", async () => {
    setPath("/about/");
    const first = render(<App engine={createFakeEngine()} />);
    expect(await screen.findByText(NOTICE)).toBeInTheDocument();
    first.unmount();

    for (const path of ["/about/extra", "/ABOUT"]) {
      setPath(path);
      const replaceSpy = vi.spyOn(window.history, "replaceState");
      const view = render(<App engine={createFakeEngine()} />);
      expect(await screen.findByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
      expect(window.location.pathname).toBe("/calc");
      expect(replaceSpy).toHaveBeenCalled();
      expect(screen.queryByText(NOTICE)).toBeNull();
      view.unmount();
      replaceSpy.mockRestore();
    }
  });

  test("見出し構造: h1(アプリ名)→ h2(このアプリについて)→ h3(非公式表示・データの出典)。h2 は1つだけ", async () => {
    setPath("/about");
    render(<App engine={createFakeEngine()} />);
    await screen.findByText(NOTICE);
    const headings = screen.getAllByRole("heading").map((heading) => [heading.tagName, heading.textContent]);
    expect(headings).toEqual([
      ["H1", expect.any(String)],
      ["H2", "このアプリについて"],
      ["H3", "非公式表示"],
      ["H3", "データの出典"],
    ]);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    // 注記は「非公式表示」の節、出典の一覧は「データの出典」の節の中にある。
    const noticeSection = screen.getByRole("heading", { level: 3, name: "非公式表示" }).parentElement;
    expect(noticeSection).not.toBeNull();
    expect(within(noticeSection as HTMLElement).getByText(NOTICE)).toBeInTheDocument();
  });

  test("ランドマーク: banner・main・contentinfo が1つずつ(情報ページの本文は main の中、フッターは外)", async () => {
    setPath("/about");
    render(<App engine={createFakeEngine()} />);
    await screen.findByText(NOTICE);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    expect(screen.getByRole("main")).toContainElement(aboutHeading());
    expect(screen.getByRole("main")).not.toContainElement(screen.getByRole("contentinfo"));
  });
});

describe("戻る導線・ブラウザの戻る/進む", () => {
  test("「計算に戻る」(リンクまたはボタン)で /calc を pushState し、計算タブの画面に戻る", async () => {
    const user = userEvent.setup();
    setPath("/about");
    render(<App engine={createFakeEngine()} />);
    await screen.findByText(NOTICE);
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("link", { name: "計算に戻る" }));

    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(window.location.pathname).toBe("/calc");
    expect(await screen.findByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByText(NOTICE)).toBeNull();
    expect(document.title).toBe("計算 | pokecalc");
  });

  test("「計算に戻る」はキーボード(Tab で届き Enter)で押せる", async () => {
    const user = userEvent.setup();
    setPath("/about");
    render(<App engine={createFakeEngine()} />);
    await screen.findByText(NOTICE);
    const back = screen.getByRole("link", { name: "計算に戻る" });
    // ヘッダーの計算モード(ラジオ)の次に Tab で届く(6回以内。タブ・フッターより前)。
    for (let i = 0; i < 6 && document.activeElement !== back; i += 1) {
      await user.tab();
    }
    expect(back).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(window.location.pathname).toBe("/calc");
  });

  test("popstate: /reverse から情報ページを開き、戻ると /reverse の逆算タブ、進むと情報ページ(履歴は積まない)", async () => {
    const user = userEvent.setup();
    setPath("/reverse");
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tab", { name: "逆算" });
    await user.click(footerLink());
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    const pushSpy = vi.spyOn(window.history, "pushState");

    simulatePopState("/reverse");
    expect(await screen.findByRole("tab", { name: "逆算" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByText(NOTICE)).toBeNull();

    simulatePopState("/about");
    await waitFor(() => {
      expect(screen.getByText(NOTICE)).toBeInTheDocument();
    });
    expect(pushSpy).not.toHaveBeenCalled();
  });
});

describe("他タブの入力状態(ADR-0308)を情報ページの往復で失わない", () => {
  test("計算の入力・逆算の入力は、情報ページを開いて戻っても残る(情報ページ中も DOM には残り、支援技術からは隠れる)", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const attacker = await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const option = within(attacker).getAllByRole("option")[1];
    if (option === undefined) {
      throw new Error("例データに種族が要る");
    }
    await user.selectOptions(attacker, option);
    const selected = (attacker as HTMLSelectElement).value;
    expect(selected).not.toBe("");

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    await user.type(await screen.findByRole("textbox", { name: "観測1" }), "45");

    await user.click(footerLink());
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    // 隠れているだけで unmount されていない。
    expect(document.querySelector('select[aria-label="攻撃側のポケモン"]')).not.toBeNull();
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();

    await user.click(screen.getByRole("link", { name: "計算に戻る" }));
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(selected);
    await user.click(screen.getByRole("tab", { name: "逆算" }));
    expect(screen.getByRole("textbox", { name: "観測1" })).toHaveValue("45");
  });

  test("/reverse → 情報ページ → ブラウザの戻る(popstate)でも逆算の入力が残る", async () => {
    const user = userEvent.setup();
    setPath("/reverse");
    render(<App engine={createFakeEngine()} />);
    await user.type(await screen.findByRole("textbox", { name: "観測1" }), "45");
    await user.click(footerLink());

    simulatePopState("/reverse");
    expect(await screen.findByRole("textbox", { name: "観測1" })).toHaveValue("45");
  });
});
