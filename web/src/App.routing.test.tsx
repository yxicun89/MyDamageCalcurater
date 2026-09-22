// P4-10: URL で画面を切り替える(ADR-0300 §1: ルーターのライブラリは入れず、History API で足りる)。
// /calc → 計算、/reverse → 逆算。/ と未知のパスは /calc に置き換える(replaceState。履歴を増やさない)。
// タブの選択(クリック・キーボード)は pushState で履歴に積み、選択中のタブをもう一度選んでも積まない。
// ブラウザの戻る・進む(popstate)でタブが切り替わる。文書のタイトルは「<画面名> | pokecalc」。
// jsdom の window.history を使う。テストの間で URL が漏れないよう、前後で "/" に戻す。
// (vitest では import.meta.env.BASE_URL は "/"。base 付きのパスは app/routes.test.ts が確かめる。)

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

function setPath(path: string): void {
  window.history.replaceState(null, "", path);
}

/** ブラウザの戻る・進むで URL が変わったときと同じく、URL を変えてから popstate を送る。 */
function simulatePopState(path: string): void {
  act(() => {
    window.history.replaceState(null, "", path);
    window.dispatchEvent(new PopStateEvent("popstate", { state: null }));
  });
}

/** 解決しないマスタ(読み込み中のまま)。 */
const pendingMasterSource: MasterSource = {
  load: () => new Promise<MasterData>(() => undefined),
};

beforeEach(() => {
  setPath("/");
});

afterEach(() => {
  vi.restoreAllMocks();
  setPath("/");
});

describe("P4-10 URL から画面を決める", () => {
  test("/reverse を直接開くと逆算タブが選択され、逆算画面を出す", async () => {
    setPath("/reverse");
    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByRole("tab", { name: "逆算" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "false");
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/reverse");
  });

  test("/calc を直接開くと計算タブが選択される", async () => {
    setPath("/calc");
    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/calc");
  });

  test.each(["/", "/unknown", "/calc/extra"])(
    "%s を開くと /calc に置き換え(pushState でなく replaceState)、計算タブを出す",
    async (path) => {
      setPath(path);
      const lengthBefore = window.history.length;
      const pushSpy = vi.spyOn(window.history, "pushState");
      const replaceSpy = vi.spyOn(window.history, "replaceState");
      render(<App engine={createFakeEngine()} />);

      expect(await screen.findByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
      expect(window.location.pathname).toBe("/calc");
      expect(replaceSpy).toHaveBeenCalled();
      expect(pushSpy).not.toHaveBeenCalled();
      expect(window.history.length).toBe(lengthBefore);
    },
  );

  test("/ の置き換えはマスタの読み込みを待たない(読み込み中でも /calc になる)", () => {
    render(<App engine={createFakeEngine()} masterSource={pendingMasterSource} />);
    expect(screen.getByText(/読み込み中/)).toBeInTheDocument();
    expect(window.location.pathname).toBe("/calc");
  });

  test("/calc・/reverse を開いたときは URL を書き換えない", async () => {
    setPath("/reverse");
    const pushSpy = vi.spyOn(window.history, "pushState");
    const replaceSpy = vi.spyOn(window.history, "replaceState");
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tab", { name: "逆算" });
    expect(pushSpy).not.toHaveBeenCalled();
    expect(replaceSpy).not.toHaveBeenCalled();
  });
});

describe("P4-10 タブの選択で URL を変える", () => {
  test("逆算タブのクリックで /reverse を pushState し、計算タブで /calc を pushState する", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    expect(window.location.pathname).toBe("/reverse");
    expect(pushSpy).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("tab", { name: "計算" }));
    expect(window.location.pathname).toBe("/calc");
    expect(pushSpy).toHaveBeenCalledTimes(2);
  });

  test("選択中のタブをもう一度選んでも履歴を積まない", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(calcTab);
    expect(pushSpy).not.toHaveBeenCalled();
    expect(window.location.pathname).toBe("/calc");
  });

  test("キーボード(ArrowRight・Home)で選んでも pushState する", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    act(() => {
      calcTab.focus();
    });
    await user.keyboard("{ArrowRight}");
    expect(window.location.pathname).toBe("/reverse");
    await user.keyboard("{Home}");
    expect(window.location.pathname).toBe("/calc");
    expect(pushSpy).toHaveBeenCalledTimes(2);
  });
});

describe("P4-10 ブラウザの戻る・進む(popstate)", () => {
  test("popstate で URL に合わせてタブと画面が切り替わり、そのとき履歴を積まない", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(screen.getByRole("tab", { name: "逆算" }));
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toBeInTheDocument();

    const pushSpy = vi.spyOn(window.history, "pushState");
    simulatePopState("/calc");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();

    simulatePopState("/reverse");
    expect(screen.getByRole("tab", { name: "逆算" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toBeInTheDocument();
    expect(pushSpy).not.toHaveBeenCalled();
  });

  test("popstate で未知のパスに戻ったときは計算を出し、/calc に置き換える", async () => {
    setPath("/reverse");
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "自分のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    simulatePopState("/");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
    expect(window.location.pathname).toBe("/calc");
    expect(pushSpy).not.toHaveBeenCalled();
  });

  test("アンマウントすると popstate を聞かなくなる", async () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tab", { name: "計算" });
    const added = addSpy.mock.calls.filter(([type]) => type === "popstate").map(([, listener]) => listener);
    expect(added.length).toBeGreaterThan(0);

    unmount();
    const removed = removeSpy.mock.calls
      .filter(([type]) => type === "popstate")
      .map(([, listener]) => listener);
    for (const listener of added) {
      expect(removed).toContain(listener);
    }
  });
});

describe("P4-10 文書のタイトル", () => {
  test("画面ごとに「<画面名> | pokecalc」にし、タブの切り替え・popstate に追従する", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tab", { name: "計算" });
    expect(document.title).toBe("計算 | pokecalc");

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    expect(document.title).toBe("逆算 | pokecalc");

    simulatePopState("/calc");
    expect(document.title).toBe("計算 | pokecalc");
  });

  test("/reverse を直接開くと、マスタの読み込み中から「逆算 | pokecalc」", () => {
    setPath("/reverse");
    render(<App engine={createFakeEngine()} masterSource={pendingMasterSource} />);
    expect(document.title).toBe("逆算 | pokecalc");
  });
});

// P4-12a: タイプバランスの画面(ADR-0303 §2)。ルート表に1件足し、タブ「タイプバランス」と /balance で開く。
// 画面はメンバーを選ぶまで balance API を呼ばない(ここでは fetch が呼ばれないことも確かめる)。
describe("P4-12a タイプバランスのタブ", () => {
  test("タブ「タイプバランス」があり、/balance を直接開くと選択され、メンバーの枠を出す(balance はまだ呼ばない)", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    setPath("/balance");
    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByRole("tab", { name: "タイプバランス" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "false");
    expect(await screen.findByRole("group", { name: "メンバー1" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/balance");
    expect(document.title).toBe("タイプバランス | pokecalc");
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  test("タイプバランスのタブのクリックで /balance を pushState し、画面を切り替える", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("tab", { name: "タイプバランス" }));
    expect(window.location.pathname).toBe("/balance");
    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("group", { name: "メンバー1" })).toBeInTheDocument();

    simulatePopState("/calc");
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
  });
});

// SP3: 素早さ比較の画面(ADR-0604 §2)。ルート表に1件足し、タブ「素早さ」と /speed で開く。
// 画面はマウント時に speed API(使用可能なポケモンの一覧・素早さの表)を呼ぶ(ADR-0604 §4)。
describe("SP3 素早さのタブ", () => {
  test("タブ「素早さ」があり、/speed を直接開くと選択され、自分のポケモンの入力と speed API の呼び出しが出る", async () => {
    // speed-svc は居ないので通信は失敗させる(画面はそれでも壊れない。ADR-0604 §4)。
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    setPath("/speed");
    render(<App engine={createFakeEngine()} />);

    expect(await screen.findByRole("tab", { name: "素早さ" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "false");
    expect(await screen.findByRole("region", { name: "自分のポケモン" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/speed");
    expect(document.title).toBe("素早さ | pokecalc");

    await waitFor(() => {
      // クライアントは文字列の URL で呼ぶ(speed/speedClient.ts)。
      const requested = fetchSpy.mock.calls.flatMap(([url]) => (typeof url === "string" ? [url] : []));
      expect(requested).toEqual(expect.arrayContaining(["/api/speed/v1/pokemon", "/api/speed/v1/table"]));
    });
  });

  test("素早さのタブのクリックで /speed を pushState し、画面を切り替える", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("tab", { name: "素早さ" }));
    expect(window.location.pathname).toBe("/speed");
    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("region", { name: "自分のポケモン" })).toBeInTheDocument();
  });
});
