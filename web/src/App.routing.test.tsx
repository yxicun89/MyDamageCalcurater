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
import { adjustScreenText, teamScreenText } from "./i18n/ja";
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
/** ポケモン画像の manifest の取得(P8-1c)か。 */
function requestUrlOf(input: RequestInfo | URL): string {
  return typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
}

function isImagesRequest(input: RequestInfo | URL): boolean {
  return requestUrlOf(input).includes("/images/");
}

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
    // 画像の manifest(/images/manifest.json。P8-1c)は起動時に1回取る。それ以外の fetch が無いことを確かめる。
    expect(fetchSpy.mock.calls.filter(([input]) => !isImagesRequest(input))).toEqual([]);
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

// ADR-0330(F-07): 判定は Web で非表示(登録ファイルの hidden: true)。タブに出ず、URL /judge は未知のパスと同じく
// 既定の画面(計算)へ置き換わり、judge API を呼ばない。判定のコードと単体テスト(web/src/judge/)は残してある。
describe("ADR-0330 判定の非表示", () => {
  test("タブ「判定」は出ない", async () => {
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.queryByRole("tab", { name: "判定" })).toBeNull();
  });

  test.each(["/judge", "/judge/"])(
    "%s を直接開くと /calc に置き換わり(replaceState)、計算タブが選ばれて judge を呼ばない",
    async (path) => {
      const fetchSpy = vi.spyOn(globalThis, "fetch");
      setPath(path);
      const pushSpy = vi.spyOn(window.history, "pushState");
      render(<App engine={createFakeEngine()} />);

      expect(await screen.findByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
      expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
      expect(window.location.pathname).toBe("/calc");
      expect(document.title).toBe("計算 | pokecalc");
      expect(pushSpy).not.toHaveBeenCalled();
      expect(screen.queryByRole("region", { name: "自分のポケモン" })).toBeNull();
      // 判定の API は呼ばれない(置き換え先の計算画面の /api/record・画像 manifest の取得は別。判定サービスへの通信が無いことだけを見る)。
      expect(fetchSpy.mock.calls.filter(([input]) => requestUrlOf(input).includes("/api/judge"))).toEqual([]);
    },
  );

  test("popstate で /judge に戻ったときも計算を出し、/calc に置き換える", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(screen.getByRole("tab", { name: "逆算" }));

    simulatePopState("/judge");

    await waitFor(() => {
      expect(window.location.pathname).toBe("/calc");
    });
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "true");
  });
});

// P5-5 PR-A1(ADR-0309 §1): 構築ビルダー。ルート表に1件足し、タブ「構築」と /team で開く。
// 画面はマウント時に構築の一覧(GET /api/team/teams)を1回だけ呼ぶ(ADR-0309 §4)。
describe("P5-5 構築のタブ", () => {
  test("タブ「構築」があり、/team を直接開くと選択され、構築の領域と一覧の呼び出しが出る", async () => {
    // team-svc は居ないので通信は失敗させる(画面はそれでも壊れない。ADR-0309 §4)。
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    setPath("/team");
    render(<App engine={createFakeEngine()} />);

    expect(await screen.findByRole("tab", { name: "構築" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "false");
    expect(await screen.findByRole("region", { name: teamScreenText.regionLabel })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/team");
    expect(document.title).toBe("構築 | pokecalc");

    await waitFor(() => {
      // クライアントは文字列の URL で呼ぶ(team/teamClient.ts)。端末 ID はパスに入れない。
      const requested = fetchSpy.mock.calls.flatMap(([url]) => (typeof url === "string" ? [url] : []));
      expect(requested).toContain("/api/team/teams");
    });
  });

  test("構築のタブのクリックで /team を pushState し、画面を切り替える", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("tab", { name: "構築" }));
    expect(window.location.pathname).toBe("/team");
    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("region", { name: teamScreenText.regionLabel })).toBeInTheDocument();
  });
});

// AJ6(ADR-0319 §1): 調整の画面。ルート表に1件足し、タブ「調整」と /adjust で開く(末尾)。
// 画面は API 専用(ADR-0411。登録ファイル adjust.screen.tsx が OnlineMasterGate で包む。ADR-0323)で、「調整する」を押すまで調整 API を呼ばない(ADR-0319 §4)。
describe("AJ6 調整のタブ", () => {
  /** fetch の呼び出しのうち調整・技の逆引きの API のもの。 */
  function adjustRequests(fetchSpy: { mock: { calls: unknown[][] } }): string[] {
    return fetchSpy.mock.calls
      .map(([url]) => (typeof url === "string" ? url : url instanceof URL ? url.href : ""))
      .filter((url) => url.includes("api/calc/adjust") || url.includes("/learners"));
  }

  test("タブ「調整」があり、/adjust を直接開くと選択され、自分の領域を出す(調整 API はまだ呼ばない)", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    setPath("/adjust");
    render(<App engine={createFakeEngine()} />);

    expect(await screen.findByRole("tab", { name: "調整" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "計算" })).toHaveAttribute("aria-selected", "false");
    expect(await screen.findByRole("region", { name: adjustScreenText.selfRegionLabel })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/adjust");
    expect(document.title).toBe("調整 | pokecalc");
    expect(adjustRequests(fetchSpy)).toEqual([]);
  });

  test("調整のタブのクリックで /adjust を pushState し、画面を切り替える", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    const pushSpy = vi.spyOn(window.history, "pushState");

    await user.click(screen.getByRole("tab", { name: "調整" }));
    expect(window.location.pathname).toBe("/adjust");
    expect(pushSpy).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("region", { name: adjustScreenText.selfRegionLabel })).toBeInTheDocument();
  });
});
