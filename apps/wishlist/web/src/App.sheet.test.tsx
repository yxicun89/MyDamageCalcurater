import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { gridItem, openHome } from "./test/render";
import { makeItem } from "./test/factories";
import { scenario } from "./test/scenario";

async function openSheet(name: string, over?: Parameters<typeof openHome>[0]) {
  const r = await openHome(over);
  await r.user.click(gridItem(name));
  return { ...r, dialog: screen.getByRole("dialog", { name: "詳細" }) };
}

// AC-SHEET-*: 詳細シート
describe("詳細シート", () => {
  it("AC-SHEET-01 大きい画像・検索ワード・サマリ・サイト行を出す。閉じるで戻る", async () => {
    const { dialog, user } = await openSheet("グリス");
    expect(within(dialog).getByRole("img", { name: "グリス" })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "検索ワードを編集" })).toHaveTextContent(
      "S.H.Figuarts グリス",
    );
    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));
    expect(screen.queryByRole("dialog", { name: "詳細" })).not.toBeInTheDocument();
  });

  it("AC-SHEET-02 サイト行は <a href=検索URL target=_blank rel=noopener>。ジャンルの site_ids 順", async () => {
    const { dialog } = await openSheet("グリス");
    const links = within(dialog).getAllByRole("link");
    expect(links.map((l) => l.textContent)).toEqual([
      expect.stringContaining("Amazon"),
      expect.stringContaining("メルカリ"),
      expect.stringContaining("その他"),
    ]);
    const q = encodeURIComponent("S.H.Figuarts グリス");
    expect(links[0]).toHaveAttribute("href", `https://www.amazon.co.jp/s?k=${q}&s=price-asc-rank`);
    expect(links[1]).toHaveAttribute(
      "href",
      `https://jp.mercari.com/search?keyword=${q}&status=on_sale&sort=price&order=asc`,
    );
    for (const l of links) {
      expect(l).toHaveAttribute("target", "_blank");
      expect(l.getAttribute("rel")).toMatch(/\bnoopener\b/);
    }
  });

  it("AC-SHEET-03 デュエマは name と option から検索ワードを作る(空白が詰まる)", async () => {
    const { dialog } = await openSheet("ボルシャック");
    expect(within(dialog).getByRole("button", { name: "検索ワードを編集" })).toHaveTextContent(
      "ボルシャック 銀トレジャー",
    );
    expect(within(dialog).getAllByRole("link")[0]).toHaveAttribute(
      "href",
      expect.stringContaining(encodeURIComponent("ボルシャック 銀トレジャー")),
    );
  });

  it("AC-SHEET-04 site_overrides: enabled=false は出さず、サイト別 query が最優先、query_override がその次", async () => {
    const s = scenario();
    s.items[1] = makeItem({
      id: 12,
      genre_id: 2,
      name: "グリス",
      query_override: "グリス 上書き",
      site_overrides: [
        { site_id: 3, enabled: false },
        { site_id: 1, query: "メルカリ専用", enabled: true },
      ],
    });
    const { dialog } = await openSheet("グリス", s);
    const links = within(dialog).getAllByRole("link");
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAttribute(
      "href",
      expect.stringContaining(`s?k=${encodeURIComponent("グリス 上書き")}`),
    );
    expect(links[1]).toHaveAttribute(
      "href",
      expect.stringContaining(`keyword=${encodeURIComponent("メルカリ専用")}`),
    );
  });

  it("AC-SHEET-05 サマリ: フェーズ1は estimates が空なので「まだ価格情報はありません」", async () => {
    const { dialog, api } = await openSheet("グリス");
    await waitFor(() => {
      expect(within(dialog).getByRole("status")).toHaveTextContent("まだ価格情報はありません");
    });
    expect(api.callsTo("GET", "/api/items/12/estimates")).toHaveLength(1);
  });

  it("AC-SHEET-06 参考外: フェーズ1は 0 件なので折りたたみ自体を出さない", async () => {
    const { dialog } = await openSheet("グリス");
    await waitFor(() => {
      expect(within(dialog).getByRole("status")).toBeInTheDocument();
    });
    expect(within(dialog).queryByText(/参考外/)).not.toBeInTheDocument();
  });

  it("AC-SHEET-07 検索ワードはタップでその場編集。「保存」を押すまで PATCH しない。シートを閉じれば破棄", async () => {
    const { dialog, user, api } = await openSheet("グリス");
    await user.click(within(dialog).getByRole("button", { name: "検索ワードを編集" }));
    const box = within(dialog).getByRole("textbox", { name: "検索ワード" });
    await user.clear(box);
    await user.type(box, "グリス 変身ベルト");
    // 編集中でもサイト行のリンクは入力値で変わる(一時的な値)
    expect(within(dialog).getAllByRole("link")[0]).toHaveAttribute(
      "href",
      expect.stringContaining(encodeURIComponent("グリス 変身ベルト")),
    );
    expect(api.callsTo("PATCH", "/api/items")).toHaveLength(0);

    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));
    await user.click(gridItem("グリス"));
    const again = screen.getByRole("dialog", { name: "詳細" });
    expect(within(again).getByRole("button", { name: "検索ワードを編集" })).toHaveTextContent(
      "S.H.Figuarts グリス",
    );
    expect(api.callsTo("PATCH", "/api/items")).toHaveLength(0);
  });

  it("AC-SHEET-08 「保存」で PATCH {query_override} を送り、反映される。空にして保存すると null(テンプレートに戻す)", async () => {
    const { dialog, user, api } = await openSheet("グリス");
    await user.click(within(dialog).getByRole("button", { name: "検索ワードを編集" }));
    const box = within(dialog).getByRole("textbox", { name: "検索ワード" });
    await user.clear(box);
    await user.type(box, "グリス 変身ベルト");
    await user.click(within(dialog).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/12")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/12")[0]?.json).toEqual({ query_override: "グリス 変身ベルト" });
    await waitFor(() => {
      expect(within(dialog).getByRole("button", { name: "検索ワードを編集" })).toHaveTextContent(
        "グリス 変身ベルト",
      );
    });

    await user.click(within(dialog).getByRole("button", { name: "検索ワードを編集" }));
    await user.clear(within(dialog).getByRole("textbox", { name: "検索ワード" }));
    await user.click(within(dialog).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/12")).toHaveLength(2);
    });
    expect(api.callsTo("PATCH", "/api/items/12")[1]?.json).toEqual({ query_override: null });
  });

  it("AC-SHEET-09 保存に失敗したら alert を出し、編集中の値とリンクは残る", async () => {
    const { dialog, user, api } = await openSheet("グリス");
    await user.click(within(dialog).getByRole("button", { name: "検索ワードを編集" }));
    const box = within(dialog).getByRole("textbox", { name: "検索ワード" });
    await user.type(box, "X");
    api.offline = true;
    await user.click(within(dialog).getByRole("button", { name: "保存" }));
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "検索ワード" })).toHaveValue("S.H.Figuarts グリスX");
    expect(within(dialog).getAllByRole("link").length).toBeGreaterThan(0);
  });
});
