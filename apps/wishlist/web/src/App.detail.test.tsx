import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { ItemEstimates, SiteEstimate } from "./api/types";
import type { FakeApi } from "./test/fakeApi";
import { gridItem, openHome } from "./test/render";

// AC-5-2h-*: 詳細シートの新しい見た目(phase5-spec 5-2「詳細」・「5-2 デザインの決まり」)。
// 状態は文ではなく、色・形・位置で伝える。ここでは位置(data-position)・強調(aria-current / data-best)・
// 薄く(data-dim)・価格なし(site-price が無い)で確かめる。
// グリス(id 12)のジャンルは S.H.Figuarts(色 #7c4dff)で、サイトは [2 Amazon, 1 メルカリ, 3 その他] の順。

const siteEst = (site_id: number, over: Partial<SiteEstimate> = {}): SiteEstimate => ({
  site_id,
  low: 3000,
  mid: 4500,
  count: 5,
  suspicious_count: 0,
  in_stock_count: 2,
  status: "ok",
  fetched_at: "2026-10-03T00:00:00Z",
  ...over,
});
const est = (over: Partial<ItemEstimates> = {}): ItemEstimates => ({
  item_id: 12,
  sites: [siteEst(2)],
  summary_low: 3000,
  summary_mid: 4500,
  summary_fetched_at: "2026-10-03T00:30:00Z",
  refreshing: false,
  ...over,
});
const noResult = (id: number) =>
  siteEst(id, { low: null, mid: null, count: 0, in_stock_count: 0, status: "no_result" });

async function openDetail(estimates: (() => ItemEstimates) | null) {
  const r = await openHome();
  if (estimates) r.api.estimates = estimates;
  await r.user.click(gridItem("グリス"));
  const dialog = screen.getByRole("dialog", { name: "詳細" });
  return { ...r, dialog };
}
const tiles = (d: HTMLElement) =>
  within(within(d).getByRole("list", { name: "サイト" })).getAllByRole("link");
const tile = (d: HTMLElement, site: string) => within(d).getByRole("link", { name: site });
const dots = (d: HTMLElement) => within(d).queryAllByTestId("price-dot");
const dotPositions = (d: HTMLElement): Record<string, string | null> => {
  const out: Record<string, string | null> = {};
  for (const x of dots(d)) out[x.getAttribute("aria-label") ?? ""] = x.getAttribute("data-position");
  return out;
};
const ready = (d: HTMLElement) =>
  waitFor(() => {
    expect(within(d).queryByText("読み込み中…")).not.toBeInTheDocument();
  });

describe("詳細: 頭(画像・ジャンル色の点・名前・取得日)", () => {
  it("AC-5-2h-1 大きい画像、ジャンル色の点、名前、取得日(summary_fetched_at の JST)を出す", async () => {
    const { dialog } = await openDetail(() => est({ summary_fetched_at: "2026-10-02T15:30:00Z" })); // UTC 10/2 = JST 10/3
    expect(within(dialog).getByRole("img", { name: "グリス" })).toBeInTheDocument();
    expect(within(dialog).getByTestId("genre-dot")).toHaveAttribute("data-genre-color", "#7c4dff");
    expect(within(dialog).getByText("グリス")).toBeInTheDocument();
    await waitFor(() => {
      expect(within(dialog).getByTestId("fetched-at")).toHaveTextContent("10/3");
    });
  });

  it("AC-5-2h-1 目安が一度も取れていない(summary_fetched_at なし)なら取得日は出さない", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [], summary_low: null, summary_mid: null, summary_fetched_at: null }),
    );
    await ready(dialog);
    expect(within(dialog).queryByTestId("fetched-at")).not.toBeInTheDocument();
  });
});

describe("詳細: 価格の帯と各サイトの点", () => {
  it("AC-5-2h-2 帯に summary の low と mid を数字で出す(「だいたい…買えそう」の文は出さない)", async () => {
    const { dialog } = await openDetail(() => est());
    const band = await within(dialog).findByTestId("price-band");
    expect(within(band).getByTestId("band-low")).toHaveTextContent("¥3,000");
    expect(within(band).getByTestId("band-high")).toHaveTextContent("¥4,500");
    expect(dialog).not.toHaveTextContent("買えそう");
  });

  it("AC-5-2h-2 mid が null なら「〜」だけ(上限の金額は出さない)", async () => {
    const { dialog } = await openDetail(() => est({ summary_mid: null, sites: [siteEst(2, { mid: null })] }));
    const band = await within(dialog).findByTestId("price-band");
    expect(within(band).getByTestId("band-low")).toHaveTextContent("¥3,000");
    expect(within(band).getByTestId("band-high")).toHaveTextContent("〜");
    expect(within(band).getByTestId("band-high")).not.toHaveTextContent("¥");
  });

  it("AC-5-2h-3 low のあるサイトごとに点を 1 つ。位置は安い順に 0〜100(最安 0・最高 100・その間は価格に比例して四捨五入)", async () => {
    const { dialog } = await openDetail(() =>
      est({
        sites: [
          siteEst(1, { low: 6800 }), // メルカリ
          siteEst(2, { low: 7200 }), // Amazon
          siteEst(3, { low: 7900 }), // その他
          siteEst(99, { low: 1 }), // このジャンルに無いサイトは点にしない
        ],
        summary_low: 6800,
        summary_mid: 8200,
      }),
    );
    await within(dialog).findByTestId("price-band");
    expect(dots(dialog)).toHaveLength(3);
    // (7200-6800)/(7900-6800)=0.3636 → 36
    expect(dotPositions(dialog)).toEqual({ メルカリ: "0", Amazon: "36", その他: "100" });
    // 点は帯の中にある
    for (const d of dots(dialog)) expect(within(dialog).getByTestId("price-band")).toContainElement(d);
  });

  it("AC-5-2h-3 点が 1 つだけ(最安と最高が同じ)なら位置は 0。価格の無いサイト(no_result・失敗で前回値なし・目安なし)は点にしない", async () => {
    const { dialog } = await openDetail(() =>
      est({
        sites: [
          siteEst(2, { low: 3000 }),
          noResult(1),
          siteEst(3, { status: "failed", low: null, mid: null, count: 0, in_stock_count: 0 }),
        ],
      }),
    );
    await within(dialog).findByTestId("price-band");
    expect(dotPositions(dialog)).toEqual({ Amazon: "0" });
  });

  it("AC-5-2h-3 取得失敗でも前回値(low)があるサイトは点にする", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [siteEst(2, { low: 3000 }), siteEst(1, { low: 5000, status: "failed" })] }),
    );
    await within(dialog).findByTestId("price-band");
    expect(dotPositions(dialog)).toEqual({ Amazon: "0", メルカリ: "100" });
  });
});

describe("詳細: サイトのタイル", () => {
  it("AC-5-2i-1 タイルは 3 列。ジャンルの site_ids 順に並び、押すと検索結果を新しいタブで開く(target=_blank rel=noopener、検索ワード入りの URL)", async () => {
    const { dialog } = await openDetail(() => est());
    await within(dialog).findByTestId("price-band");
    expect(within(dialog).getByRole("list", { name: "サイト" })).toHaveAttribute("data-columns", "3");
    expect(tiles(dialog).map((l) => l.getAttribute("aria-label"))).toEqual(["Amazon", "メルカリ", "その他"]);
    const q = encodeURIComponent("S.H.Figuarts グリス");
    expect(tile(dialog, "Amazon")).toHaveAttribute(
      "href",
      `https://www.amazon.co.jp/s?k=${q}&s=price-asc-rank`,
    );
    for (const l of tiles(dialog)) {
      expect(l).toHaveAttribute("target", "_blank");
      expect(l.getAttribute("rel")).toMatch(/\bnoopener\b/);
    }
  });

  it("AC-5-2i-2 タイルはサイトの頭文字(マーク)と、low のあるサイトだけ価格(¥low)を出す。件数・在庫の文は出さない", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [siteEst(2, { low: 3000 }), siteEst(1, { low: 800 })] }),
    );
    await within(dialog).findByTestId("price-band");
    expect(within(tile(dialog, "Amazon")).getByTestId("site-mark")).toHaveTextContent("A");
    expect(within(tile(dialog, "メルカリ")).getByTestId("site-mark")).toHaveTextContent("メ");
    expect(within(tile(dialog, "Amazon")).getByTestId("site-price")).toHaveTextContent("¥3,000");
    expect(within(tile(dialog, "メルカリ")).getByTestId("site-price")).toHaveTextContent("¥800");
    for (const w of ["件", "在庫"])
      expect(dialog.querySelector("[aria-label='サイト']")).not.toHaveTextContent(w);
  });

  it("AC-5-2i-3 最安のタイルだけ強調(aria-current と data-best)。同額は両方", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [siteEst(2, { low: 3000 }), siteEst(1, { low: 800 }), siteEst(3, { low: 5000 })] }),
    );
    await within(dialog).findByTestId("price-band");
    expect(tile(dialog, "メルカリ")).toHaveAttribute("aria-current", "true");
    expect(tile(dialog, "メルカリ")).toHaveAttribute("data-best", "true");
    for (const n of ["Amazon", "その他"]) {
      expect(tile(dialog, n)).not.toHaveAttribute("aria-current");
      expect(tile(dialog, n)).not.toHaveAttribute("data-best");
    }
  });

  it("AC-5-2i-3 最安が同額のタイルはどちらも強調する", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [siteEst(2, { low: 3000 }), siteEst(1, { low: 3000 }), siteEst(3, { low: 5000 })] }),
    );
    await within(dialog).findByTestId("price-band");
    expect(tile(dialog, "Amazon")).toHaveAttribute("data-best", "true");
    expect(tile(dialog, "メルカリ")).toHaveAttribute("data-best", "true");
    expect(tile(dialog, "その他")).not.toHaveAttribute("data-best");
  });

  it("AC-5-2i-4 出品なし(no_result)のタイルは薄く(data-dim)「なし」。リンクは残る。価格を持たないリンクだけのサイトは薄くせず価格も出さない", async () => {
    const { dialog } = await openDetail(() => est({ sites: [siteEst(2, { low: 3000 }), noResult(1)] }));
    await within(dialog).findByTestId("price-band");
    const none = tile(dialog, "メルカリ");
    expect(none).toHaveAttribute("data-dim", "true");
    expect(within(none).getByTestId("site-price")).toHaveTextContent("なし");
    expect(none).toHaveAttribute("target", "_blank");
    // その他 = link_only で目安なし
    const linkOnly = tile(dialog, "その他");
    expect(linkOnly).not.toHaveAttribute("data-dim");
    expect(within(linkOnly).queryByTestId("site-price")).not.toBeInTheDocument();
    expect(tile(dialog, "Amazon")).not.toHaveAttribute("data-dim");
  });

  it("AC-5-2i-5 取得に失敗したサイト: 前回値があれば価格を出して title に「最終取得: N日前」、無ければ価格なし(data-failed)。alert は出さない", async () => {
    const { dialog } = await openDetail(() =>
      est({
        sites: [
          siteEst(2, { status: "failed", fetched_at: "2026-09-30T00:00:00Z" }),
          siteEst(1, { status: "failed", low: null, mid: null, count: 0, in_stock_count: 0 }),
        ],
      }),
    );
    await within(dialog).findByTestId("price-band");
    expect(within(tile(dialog, "Amazon")).getByTestId("site-price")).toHaveTextContent("¥3,000");
    expect(tile(dialog, "Amazon").getAttribute("title")).toMatch(/^最終取得: (\d+日前|今日)$/);
    expect(within(tile(dialog, "メルカリ")).queryByTestId("site-price")).not.toBeInTheDocument();
    expect(tile(dialog, "メルカリ")).toHaveAttribute("data-failed", "true");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("詳細: 出品が無いとき", () => {
  it("AC-5-2j-1 どのサイトにも価格が無く、取得できたサイトがすべて no_result なら、帯の代わりに「出品ないかも」。点は出ない。リンクは残る", async () => {
    const { dialog } = await openDetail(() =>
      est({
        sites: [noResult(2), noResult(1)],
        summary_low: null,
        summary_mid: null,
        summary_fetched_at: null,
      }),
    );
    expect(await within(dialog).findByTestId("no-listing")).toHaveTextContent("出品ないかも");
    expect(within(dialog).queryByTestId("price-band")).not.toBeInTheDocument();
    expect(dots(dialog)).toHaveLength(0);
    expect(tiles(dialog)).toHaveLength(3);
  });

  it("AC-5-2j-1 帯があるときは「出品ないかも」を出さない", async () => {
    const { dialog } = await openDetail(() => est({ sites: [siteEst(2), noResult(1)] }));
    await within(dialog).findByTestId("price-band");
    expect(within(dialog).queryByTestId("no-listing")).not.toBeInTheDocument();
  });

  it("AC-5-2j-2 目安がまだ無い(sites が空)ときは帯も「出品ないかも」も出さず、空の枠(band-empty)", async () => {
    const { dialog } = await openDetail(() =>
      est({ sites: [], summary_low: null, summary_mid: null, summary_fetched_at: null }),
    );
    await waitFor(() => {
      expect(within(dialog).getByTestId("band-empty")).toBeInTheDocument(); // 旧: status の文。文字の多い状態は不評(ユーザー指摘 2026-10-11)
    });
    expect(within(dialog).queryByTestId("price-band")).not.toBeInTheDocument();
    expect(within(dialog).queryByTestId("no-listing")).not.toBeInTheDocument();
  });
});

describe("詳細: あやしい出品", () => {
  const withSuspicious = (a: FakeApi) => {
    a.estimates = () =>
      est({ sites: [siteEst(2, { suspicious_count: 1 }), siteEst(1, { suspicious_count: 2 })] });
  };

  it("AC-5-2k-1 件数が 0 なら「あやしい出品」のボタンを出さない。1 以上なら合計入りのボタン(押すまで listings は取らない)", async () => {
    const none = await openDetail(() => est());
    await within(none.dialog).findByTestId("price-band");
    expect(within(none.dialog).queryByRole("button", { name: /あやしい出品/ })).not.toBeInTheDocument();
  });

  it("AC-5-2k-1 合計入りのボタン「あやしい出品 3」は、押すまで listings を取らない", async () => {
    const r = await openHome();
    withSuspicious(r.api);
    await r.user.click(gridItem("グリス"));
    const dialog = screen.getByRole("dialog", { name: "詳細" });
    const button = await within(dialog).findByRole("button", { name: "あやしい出品 3" });
    expect(r.api.callsTo("GET", "/api/items/12/listings")).toHaveLength(0);
    await r.user.click(button);
    expect(await within(dialog).findByRole("list", { name: "あやしい出品" })).toBeInTheDocument();
    expect(r.api.callsTo("GET", "/api/items/12/listings")).toHaveLength(1);
  });
});
