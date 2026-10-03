import { render, screen, waitFor, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi, seedSettings } from "./test/fakeApi";
import { grid, gridItem, openHome } from "./test/render";

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

  it("AC-OFF-03 取得に失敗した(401 など)ときも保存済みの一覧を使い、エラー画面にしない", async () => {
    await openHome();
    document.body.innerHTML = "";
    localStorage.setItem("wishlist.settings", JSON.stringify({ apiBaseUrl: null, token: "wrong" }));
    render(<App />);
    expect(await screen.findByRole("button", { name: "グリス" })).toBeInTheDocument();
  });

  it("AC-OFF-04 保存済みも無く取得もできないときは alert を出す(白画面にしない)。「+」と「設定」は出る", async () => {
    const api = installFakeApi();
    seedSettings();
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
