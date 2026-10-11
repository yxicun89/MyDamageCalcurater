import { act, render, screen, waitFor, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi } from "./test/fakeApi";
import { makeItem } from "./test/factories";
import { grid, gridItem, openHome } from "./test/render";
import { scenario } from "./test/scenario";

// AC-OFF-*: Mac が落ちていても、ホームとディープリンクは使える(仕様 §10)
describe("オフライン", () => {
  it("AC-OFF-01 一度オンラインで開いたあとは、API が落ちていてもグリッドとジャンルチップが出る", async () => {
    const { api } = await openHome();
    screen.getByRole("navigation", { name: "ジャンル" });
    // 同じ端末で再読み込み(= 再描画)。API は落ちている
    document.body.innerHTML = "";
    api.offline = true;
    render(<App />);
    expect(await screen.findByRole("button", { name: "グリス" })).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "ジャンル" })).getAllByRole("button")).toHaveLength(
      3,
    );
  });

  it("AC-OFF-02 オフラインでも詳細シートのサイト行リンクが出て、サマリは「オフライン」", async () => {
    const { api } = await openHome();
    document.body.innerHTML = "";
    api.offline = true;
    render(<App />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "グリス" }));
    const dlg = screen.getByRole("dialog", { name: "詳細" });
    expect(within(dlg).getAllByRole("link")).toHaveLength(3);
    await waitFor(() => {
      expect(within(dlg).getByRole("status")).toHaveTextContent("オフライン");
    });
  });

  // 旧: 「取得に失敗した(401 など)」で token を "wrong" にして 401 を起こした。認証が無くなった(5-1a)ので、サーバーの 500 に置き換える。
  it("AC-OFF-03 取得に失敗した(HTTP 500 など)ときも保存済みの一覧を使い、エラー画面にしない", async () => {
    const { api } = await openHome();
    document.body.innerHTML = "";
    api.failStatus = 500;
    render(<App />);
    expect(await screen.findByRole("button", { name: "グリス" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("AC-OFF-04 保存済みも無く取得もできないときは alert を出す(白画面にしない)。「+」と「設定」は出る", async () => {
    const api = installFakeApi();
    api.offline = true;
    render(<App />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "設定" })).toBeInTheDocument();
  });

  it("AC-OFF-05 オンラインに戻った状態で開き直すと、最新の一覧に置き換わる", async () => {
    const { api } = await openHome();
    document.body.innerHTML = "";
    api.items = api.items.filter((i) => i.name !== "ビルド");
    render(<App />);
    await screen.findByRole("button", { name: "グリス" });
    await waitFor(() => {
      expect(within(grid()).queryByRole("button", { name: "ビルド" })).not.toBeInTheDocument();
    });
    expect(gridItem("グリス")).toBeInTheDocument();
  });
});

// AC-5-2g-*: オフラインの見た目(phase5-spec 5-2「オフライン」)
describe("オフライン(5-2)", () => {
  const withPrices = () => {
    const s = scenario();
    s.items = [
      makeItem({
        id: 12,
        genre_id: 2,
        name: "グリス",
        summary: { low: 6800, status: "ok", fetched_at: "2026-10-03T00:00:00Z" },
      }),
      makeItem({ id: 13, genre_id: 2, name: "ビルド" }),
    ];
    return s;
  };

  it("AC-5-2g-1 オンラインでは「オフライン」の札を出さず、価格の札を出す", async () => {
    await openHome(withPrices());
    expect(screen.queryByTestId("offline-badge")).not.toBeInTheDocument();
    expect(within(gridItem("グリス")).getByTestId("price-tag")).toHaveTextContent("¥6,800〜");
  });

  it("AC-5-2g-2 一覧を取れずに保存済みを使っているとき、上に「オフライン」の札を出し、価格の札は消す。商品の画像は出る", async () => {
    const { api } = await openHome(withPrices());
    document.body.innerHTML = "";
    api.offline = true;
    render(<App />);
    await screen.findByRole("button", { name: "グリス" });
    expect(await screen.findByTestId("offline-badge")).toHaveTextContent("オフライン");
    expect(screen.queryAllByTestId("price-tag")).toHaveLength(0);
    expect(within(grid()).getAllByRole("img")).toHaveLength(2);
  });

  it("AC-5-2g-3 ブラウザの offline イベントでも札を出し、online で戻す", async () => {
    await openHome(withPrices());
    act(() => {
      window.dispatchEvent(new Event("offline"));
    });
    expect(await screen.findByTestId("offline-badge")).toBeInTheDocument();
    expect(screen.queryAllByTestId("price-tag")).toHaveLength(0);
    act(() => {
      window.dispatchEvent(new Event("online"));
    });
    await waitFor(() => {
      expect(screen.queryByTestId("offline-badge")).not.toBeInTheDocument();
    });
    expect(within(gridItem("グリス")).getByTestId("price-tag")).toBeInTheDocument();
  });

  it("AC-5-2g-4 オフラインでも詳細のサイトのタイルはリンクとして使え、価格の帯は出ない", async () => {
    const { api } = await openHome(withPrices());
    document.body.innerHTML = "";
    api.offline = true;
    render(<App />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "グリス" }));
    const dlg = screen.getByRole("dialog", { name: "詳細" });
    for (const l of within(dlg).getAllByRole("link")) {
      expect(l).toHaveAttribute("target", "_blank");
      expect(l.getAttribute("href")).toMatch(/^https:\/\//);
    }
    expect(within(dlg).queryByTestId("price-band")).not.toBeInTheDocument();
  });
});
