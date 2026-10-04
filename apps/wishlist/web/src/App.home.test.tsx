import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi } from "./test/fakeApi";
import { grid, gridItem, openHome, queryGridItem } from "./test/render";

// AC-HOME-*: ホーム(画像のみのグリッド・ジャンルチップ・「+」・長押しメニュー)
describe("ホーム", () => {
  it("AC-HOME-01 商品を画像だけで並べる。文字は出さない(img の alt は商品名)", async () => {
    await openHome();
    const imgs = within(grid()).getAllByRole("img");
    expect(imgs.map((i) => i.getAttribute("alt"))).toEqual(["ボルシャック", "グリス", "ビルド"]);
    expect(grid().textContent).toBe("");
    // 画像はベース URL 基準で解決する(先頭 / なしの images/<name>)
    expect(imgs[0]?.getAttribute("src")).toBe(
      new URL("images/00000000-0000-4000-8000-000000000011.png", document.baseURI).href,
    );
  });

  it("AC-HOME-02 トークンが Bearer で API に付く", async () => {
    const { api } = await openHome();
    expect(api.calls.length).toBeGreaterThan(0);
    for (const c of api.calls) expect(c.headers.get("Authorization")).toBe("Bearer test-token");
  });

  it("AC-HOME-03 ジャンルのチップは「すべて」+各ジャンルを sort_order 順に並べ、押すと絞り込む", async () => {
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    const chips = within(nav).getAllByRole("button");
    expect(chips.map((c) => c.textContent)).toEqual(["すべて", "S.H.Figuarts", "デュエマ"]);
    expect(within(nav).getByRole("button", { name: "すべて" })).toHaveAttribute("aria-pressed", "true");

    await user.click(within(nav).getByRole("button", { name: "S.H.Figuarts" }));
    expect(within(nav).getByRole("button", { name: "S.H.Figuarts" })).toHaveAttribute("aria-pressed", "true");
    expect(queryGridItem("グリス")).toBeInTheDocument();
    expect(queryGridItem("ビルド")).toBeInTheDocument();
    expect(queryGridItem("ボルシャック")).not.toBeInTheDocument();

    await user.click(within(nav).getByRole("button", { name: "すべて" }));
    expect(queryGridItem("ボルシャック")).toBeInTheDocument();
  });

  it("AC-HOME-04 右下の「+」ボタン(aria-label=追加)で登録画面が開く", async () => {
    const { user } = await openHome();
    const add = screen.getByRole("button", { name: "追加" });
    expect(add).toHaveTextContent("+");
    await user.click(add);
    expect(screen.getByRole("dialog", { name: "登録" })).toBeInTheDocument();
  });

  it("AC-HOME-05 商品が 0 件でも落ちず、「+」は出る", async () => {
    installFakeApi({ genres: [], sites: [], items: [] });
    localStorage.setItem("wishlist.settings", JSON.stringify({ apiBaseUrl: null, token: "test-token" }));
    render(<App />);
    expect(await screen.findByRole("button", { name: "追加" })).toBeInTheDocument();
    expect(screen.queryAllByRole("img")).toHaveLength(0);
  });

  it("AC-HOME-06 長押し(500ms)で編集・削除メニューが出て、そのあと離しても詳細シートは開かない", async () => {
    await openHome();
    const btn = gridItem("グリス");
    fireEvent.pointerDown(btn, { pointerId: 1, button: 0 });
    const menu = await screen.findByRole("menu", {}, { timeout: 1500 });
    expect(within(menu).getByRole("menuitem", { name: "編集" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitem", { name: "削除" })).toBeInTheDocument();
    fireEvent.pointerUp(btn, { pointerId: 1 });
    fireEvent.click(btn);
    expect(screen.queryByRole("dialog", { name: "詳細" })).not.toBeInTheDocument();
  });

  it("AC-HOME-07 短いタップではメニューを出さず、詳細シートが開く", async () => {
    const { user } = await openHome();
    await user.click(gridItem("グリス"));
    expect(screen.getByRole("dialog", { name: "詳細" })).toBeInTheDocument();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("AC-HOME-08 押している途中で離す・指が外れると長押しは成立しない", async () => {
    await openHome();
    const btn = gridItem("グリス");
    fireEvent.pointerDown(btn, { pointerId: 1, button: 0 });
    fireEvent.pointerUp(btn, { pointerId: 1 });
    fireEvent.pointerDown(btn, { pointerId: 2, button: 0 });
    fireEvent.pointerCancel(btn, { pointerId: 2 });
    await new Promise((r) => setTimeout(r, 700));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("AC-HOME-09 設定ボタン(aria-label=設定)から設定画面へ、戻るで戻れる", async () => {
    const { user } = await openHome();
    await user.click(screen.getByRole("button", { name: "設定" }));
    expect(screen.getByRole("heading", { name: "設定" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "戻る" }));
    await waitFor(() => {
      expect(grid()).toBeInTheDocument();
    });
  });
});
