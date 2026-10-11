import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeItem } from "./test/factories";
import { gridItem, openHome, queryGridItem } from "./test/render";
import { scenario } from "./test/scenario";

async function openMenu(name: string, over: Parameters<typeof openHome>[0] = {}) {
  const r = await openHome(over);
  fireEvent.pointerDown(gridItem(name), { pointerId: 1, button: 0 });
  const menu = await screen.findByRole("menu", {}, { timeout: 1500 });
  fireEvent.pointerUp(gridItem(name), { pointerId: 1 });
  return { ...r, menu };
}

// AC-EDIT-*: 編集・削除
describe("編集・削除", () => {
  it("AC-EDIT-01 編集は名前・option・検索ワード上書き・ジャンル・最低価格を変えて PATCH する(変えた項目だけ)", async () => {
    const { user, api, menu } = await openMenu("ボルシャック");
    await user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    expect(within(dlg).getByRole("textbox", { name: "名前" })).toHaveValue("ボルシャック");
    expect(within(dlg).getByRole("textbox", { name: "オプション" })).toHaveValue("銀トレジャー");

    await user.clear(within(dlg).getByRole("textbox", { name: "名前" }));
    await user.type(within(dlg).getByRole("textbox", { name: "名前" }), "ボルシャック・ドラゴン");
    await user.type(within(dlg).getByRole("textbox", { name: "検索ワード上書き" }), "ボルシャック 銀");
    // 旧: select(combobox)で選んでいた。5-2 はジャンルを絵柄のボタン(aria-label が名前)で選ぶ。ユーザー決定 2026-10-11 / phase5-spec 5-2
    await user.click(
      within(within(dlg).getByRole("group", { name: "ジャンル" })).getByRole("button", {
        name: "S.H.Figuarts",
      }),
    );
    await user.type(within(dlg).getByRole("spinbutton", { name: "最低価格(円)" }), "1500");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));

    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/11")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/11")[0]?.json).toEqual({
      name: "ボルシャック・ドラゴン",
      query_override: "ボルシャック 銀",
      genre_id: 2,
      min_price: 1500,
    });
    expect(await screen.findByRole("button", { name: "ボルシャック・ドラゴン" })).toBeInTheDocument();
  });

  it("AC-EDIT-02 option・検索ワード上書き・最低価格を空にして保存すると null で消す", async () => {
    const { user, api, menu } = await openMenu("ボルシャック");
    await user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    await user.clear(within(dlg).getByRole("textbox", { name: "オプション" }));
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/11")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/11")[0]?.json).toEqual({ option_text: null });
  });

  it("AC-EDIT-03 画像の差し替えは PUT /api/items/:id/image(multipart)で、グリッドの画像も変わる", async () => {
    const { user, api, menu } = await openMenu("グリス");
    await user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    await user.upload(
      within(dlg).getByLabelText("画像を差し替え"),
      new File(["x"], "new.png", { type: "image/png" }),
    );
    await waitFor(() => {
      expect(api.callsTo("PUT", "/api/items/12/image")).toHaveLength(1);
    });
    expect((api.callsTo("PUT", "/api/items/12/image")[0]?.form?.get("image") as File).name).toBe("new.png");
    await waitFor(() => {
      expect(gridItem("グリス").querySelector("img")?.getAttribute("src")).toContain(
        "11111111-1111-4111-8111-111111111111.png",
      );
    });
  });

  it("AC-EDIT-04 削除は確認(alertdialog)のあとだけ DELETE。キャンセルなら何もしない", async () => {
    const { user, api, menu } = await openMenu("ビルド");
    await user.click(within(menu).getByRole("menuitem", { name: "削除" }));
    const confirm = screen.getByRole("alertdialog");
    expect(confirm).toHaveTextContent("ビルド");
    await user.click(within(confirm).getByRole("button", { name: "キャンセル" }));
    expect(api.callsTo("DELETE", "/api/items")).toHaveLength(0);
    expect(queryGridItem("ビルド")).toBeInTheDocument();

    fireEvent.pointerDown(gridItem("ビルド"), { pointerId: 1, button: 0 });
    const menu2 = await screen.findByRole("menu", {}, { timeout: 1500 });
    fireEvent.pointerUp(gridItem("ビルド"), { pointerId: 1 });
    await user.click(within(menu2).getByRole("menuitem", { name: "削除" }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "削除する" }));
    await waitFor(() => {
      expect(api.callsTo("DELETE", "/api/items/13")).toHaveLength(1);
    });
    await waitFor(() => {
      expect(queryGridItem("ビルド")).not.toBeInTheDocument();
    });
  });

  it("AC-EDIT-05 保存・削除に失敗したら alert を出し、グリッドは変えない", async () => {
    const { user, api, menu } = await openMenu("ビルド");
    await user.click(within(menu).getByRole("menuitem", { name: "削除" }));
    api.offline = true;
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "削除する" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(queryGridItem("ビルド")).toBeInTheDocument();
  });
});

// AC-5-2m-*: 長押しメニュー(phase5-spec 5-2「長押し」。背景をぼかし、編集・元のページを開く・削除)
describe("長押しメニュー(5-2)", () => {
  const withSource = () => {
    const s = scenario();
    s.items = [
      makeItem({ id: 11, genre_id: 1, name: "ボルシャック", option_text: "銀トレジャー" }),
      makeItem({ id: 12, genre_id: 2, name: "グリス", source_url: "https://shop.example/p/12" }),
      makeItem({ id: 13, genre_id: 2, name: "ビルド", source_url: "" }),
    ];
    return s;
  };

  it("AC-5-2m-1 メニューは 編集・元のページを開く・削除 の順。元のページは source_url があるときだけ", async () => {
    const withUrl = await openMenu("グリス", withSource());
    expect(
      within(withUrl.menu)
        .getAllByRole("menuitem")
        .map((m) => m.textContent),
    ).toEqual(["編集", "元のページを開く", "削除"]);
    const link = within(withUrl.menu).getByRole("menuitem", { name: "元のページを開く" });
    expect(link).toHaveAttribute("href", "https://shop.example/p/12");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link.getAttribute("rel")).toMatch(/\bnoopener\b/);
  });

  it("AC-5-2m-1 source_url が無い(null・空)商品には「元のページを開く」を出さない", async () => {
    const none = await openMenu("ボルシャック", withSource());
    expect(
      within(none.menu)
        .getAllByRole("menuitem")
        .map((m) => m.textContent),
    ).toEqual(["編集", "削除"]);
  });

  it("AC-5-2m-1 source_url が http(s) でない商品にも出さない(javascript: をリンクにしない)", async () => {
    const s = withSource();
    s.items[1] = makeItem({ id: 12, genre_id: 2, name: "グリス", source_url: "javascript:alert(1)" });
    const r = await openMenu("グリス", s);
    expect(within(r.menu).queryByRole("menuitem", { name: "元のページを開く" })).not.toBeInTheDocument();
  });

  it("AC-5-2m-2 メニューが開いている間はホームの背景をぼかし(data-blurred)、閉じたら戻る", async () => {
    const { user, menu } = await openMenu("グリス", withSource());
    expect(screen.getByTestId("home")).toHaveAttribute("data-blurred", "true");
    await user.keyboard("{Escape}");
    expect(menu).not.toBeInTheDocument();
    expect(screen.getByTestId("home")).not.toHaveAttribute("data-blurred", "true");
  });

  it("AC-5-2m-3 メニューには押した商品の画像を大きく出す(長押ししたものが分かる)", async () => {
    await openMenu("グリス", withSource());
    expect(
      within(screen.getByTestId("menu-preview")).getByRole("img", { name: "グリス" }),
    ).toBeInTheDocument();
  });
});
