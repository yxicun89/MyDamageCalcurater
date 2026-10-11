import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi } from "./test/fakeApi";
import { makeItem } from "./test/factories";
import { grid, gridItem, openHome, queryGridItem } from "./test/render";
import { scenario } from "./test/scenario";

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

  // 旧: 「トークンが Bearer で API に付く」。ユーザー決定 2026-10-11 / phase5-spec 5-1a・5-2 で認証を外したので、付かないことに置き換える。
  it("AC-HOME-02 / AC-5-2a-1 API には Authorization を付けない", async () => {
    const { api } = await openHome();
    expect(api.calls.length).toBeGreaterThan(0);
    for (const c of api.calls) expect(c.headers.has("Authorization")).toBe(false);
  });

  // 旧: チップの textContent が名前。5-2 で絵柄のボタン(文字なし・aria-label が名前)にしたので aria-label で見る。ユーザー決定 2026-10-11 / phase5-spec 5-2。
  it("AC-HOME-03 ジャンルのボタンは「すべて」+各ジャンルを sort_order 順に並べ、押すと絞り込む", async () => {
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    const chips = within(nav).getAllByRole("button");
    expect(chips.map((c) => c.getAttribute("aria-label"))).toEqual(["すべて", "S.H.Figuarts", "デュエマ"]);
    expect(within(nav).getByRole("button", { name: "すべて" })).toHaveAttribute("aria-pressed", "true");

    await user.click(within(nav).getByRole("button", { name: "S.H.Figuarts" }));
    expect(within(nav).getByRole("button", { name: "S.H.Figuarts" })).toHaveAttribute("aria-pressed", "true");
    expect(queryGridItem("グリス")).toBeInTheDocument();
    expect(queryGridItem("ビルド")).toBeInTheDocument();
    expect(queryGridItem("ボルシャック")).not.toBeInTheDocument();

    await user.click(within(nav).getByRole("button", { name: "すべて" }));
    expect(queryGridItem("ボルシャック")).toBeInTheDocument();
  });

  // 旧: 「+」の文字を持つ。5-2 は svg の「+」(mockup)なので文字は見ない。登録画面の名前は「登録」→「追加」。
  it("AC-HOME-04 右下の「+」ボタン(aria-label=追加)で追加のシートが開く", async () => {
    const { user } = await openHome();
    await user.click(screen.getByRole("button", { name: "追加" }));
    expect(screen.getByRole("dialog", { name: "追加" })).toBeInTheDocument();
  });

  it("AC-HOME-05 商品が 0 件でも落ちず、「+」は出る", async () => {
    installFakeApi({ genres: [], sites: [], items: [] });
    render(<App />);
    expect(await screen.findByRole("button", { name: "追加" })).toBeInTheDocument();
    expect(screen.queryAllByRole("img")).toHaveLength(0);
  });

  it("AC-HOME-06 長押し(500ms)でメニューが出て、そのあと離しても詳細シートは開かない", async () => {
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

// AC-5-2c-*: ホームの見た目の契約(phase5-spec 5-2 / 「5-2 デザインの決まり」)
const withSummary = (id: number, genre_id: number, name: string, summary: unknown) =>
  makeItem({ id, genre_id, name, summary } as Parameters<typeof makeItem>[0]);

describe("ホーム(5-2)", () => {
  it("AC-5-2c-1 タイルの枠はジャンルの色(data-genre-color と --genre-color)。色はジャンルごとに違う", async () => {
    await openHome();
    const dm = gridItem("ボルシャック"); // ジャンル 1 デュエマ #e0393e
    const fig = gridItem("グリス"); // ジャンル 2 S.H.Figuarts #7c4dff
    expect(dm).toHaveAttribute("data-genre-color", "#e0393e");
    expect(fig).toHaveAttribute("data-genre-color", "#7c4dff");
    expect(dm.style.getPropertyValue("--genre-color")).toBe("#e0393e");
    expect(fig.style.getPropertyValue("--genre-color")).toBe("#7c4dff");
  });

  it("AC-5-2c-2 価格の札: ok は ¥low〜、no_result は「出品なし」、none・failed・summary なしは札を出さない", async () => {
    const s = scenario();
    s.items = [
      withSummary(11, 1, "ボルシャック", { low: 880, status: "ok", fetched_at: "2026-10-03T00:00:00Z" }),
      withSummary(12, 2, "グリス", { low: null, status: "no_result", fetched_at: null }),
      withSummary(13, 2, "ビルド", { low: null, status: "none", fetched_at: null }),
      withSummary(14, 2, "ローグ", { low: null, status: "failed", fetched_at: null }),
      withSummary(15, 2, "クローズ", null),
      makeItem({ id: 16, genre_id: 2, name: "カズミン" }), // summary キー自体が無い(古い応答)
    ];
    await openHome(s);
    const tag = (name: string) => within(gridItem(name)).queryByTestId("price-tag");
    expect(tag("ボルシャック")).toHaveTextContent("¥880〜");
    expect(tag("グリス")).toHaveTextContent("出品なし");
    for (const n of ["ビルド", "ローグ", "クローズ", "カズミン"]) expect(tag(n)).not.toBeInTheDocument();
    // 札の意味は data で持つ(色分けは CSS)
    expect(tag("ボルシャック")).toHaveAttribute("data-kind", "price");
    expect(tag("グリス")).toHaveAttribute("data-kind", "none");
    // 札はボタンの名前を変えない(名前は商品名のまま)
    expect(gridItem("ボルシャック")).toHaveAccessibleName("ボルシャック");
  });

  it("AC-5-2c-3 ジャンルのボタンは絵柄(data-icon)とジャンル色を持ち、選んだものだけ aria-pressed", async () => {
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    const all = within(nav).getByRole("button", { name: "すべて" });
    const fig = within(nav).getByRole("button", { name: "S.H.Figuarts" });
    const dm = within(nav).getByRole("button", { name: "デュエマ" });
    expect(all.querySelector("[data-icon='all']")).not.toBeNull();
    expect(fig.querySelector("[data-icon='figure']")).not.toBeNull();
    expect(dm.querySelector("[data-icon='card']")).not.toBeNull();
    expect(fig).toHaveAttribute("data-genre-color", "#7c4dff");
    expect(dm).toHaveAttribute("data-genre-color", "#e0393e");
    // 文字は出さない(aria-label が名前)
    expect(fig.textContent).toBe("");
    expect([all, fig, dm].map((b) => b.getAttribute("aria-pressed"))).toEqual(["true", "false", "false"]);
    await user.click(dm);
    expect([all, fig, dm].map((b) => b.getAttribute("aria-pressed"))).toEqual(["false", "false", "true"]);
  });

  it("AC-5-2c-4 ホームの見出しは「欲しいもの」と件数", async () => {
    await openHome();
    expect(screen.getByRole("heading", { name: "欲しいもの" })).toBeInTheDocument();
    expect(screen.getByTestId("item-count")).toHaveTextContent("3");
  });
});
