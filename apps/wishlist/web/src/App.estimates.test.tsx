import { act, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ItemEstimates, Listing, SiteEstimate } from "./api/types";
import type { FakeApi } from "./test/fakeApi";
import { installPollClock } from "./test/pollClock";
import { gridItem, openHome } from "./test/render";

// ジャンル 2(S.H.Figuarts)の site_ids は [2 Amazon, 1 メルカリ, 3 その他]、ジャンル 1(デュエマ)は [1, 2]。
const FIGUARTS_ITEM = { name: "グリス", id: 12 };

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
  item_id: FIGUARTS_ITEM.id,
  sites: [siteEst(2)],
  summary_low: 3000,
  summary_mid: 4500,
  summary_fetched_at: "2026-10-03T00:30:00Z",
  refreshing: false,
  ...over,
});
const listing = (id: number, over: Partial<Listing> = {}): Listing => ({
  id,
  site_id: 1,
  title: `出品${String(id)}`,
  price: 300,
  url: `https://shop.example/l/${String(id)}`,
  image_url: `https://img.example/l/${String(id)}.png`,
  in_stock: true,
  suspicious_reasons: [],
  fetched_at: "2026-10-03T00:00:00Z",
  ...over,
});

/** 偽の時計(ポーリング)を入れてから、商品の詳細シートを開く。時計は openHome より前に入れる。 */
async function openSheet(prepare: (api: FakeApi) => void, opts: { now?: string; item?: string } = {}) {
  const clock = installPollClock();
  if (opts.now) {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(opts.now));
  }
  // openHome が fakeApi を入れる。estimates 等はシートを開く前に差し込む必要があるため、fetch を包む。
  const r = await openHome();
  prepare(r.api);
  await r.user.click(gridItem(opts.item ?? FIGUARTS_ITEM.name));
  const dialog = screen.getByRole("dialog", { name: "詳細" });
  return { ...r, clock, dialog };
}
// 5-2(ユーザー決定 2026-10-11 / phase5-spec 5-2)で、サマリの文「だいたい ¥A〜¥B で買えそう(日付 時点)」を
// 価格の帯(price-band / band-low / band-high)と取得日(fetched-at)に置き換えた。status は読み込み中・更新中・失敗などの短い文だけを持つ。
const status = (d: HTMLElement) => within(d).getByRole("status");
const bandLow = (d: HTMLElement) => within(d).getByTestId("band-low");
const hasBand = (d: HTMLElement) => {
  expect(within(d).getByTestId("price-band")).toBeInTheDocument();
};
const siteRows = (d: HTMLElement) =>
  within(within(d).getByRole("list", { name: "サイト" })).getAllByRole("link");
const sitePrice = (d: HTMLElement, site: string) =>
  within(within(d).getByRole("link", { name: site })).queryByTestId("site-price");
const gets = (api: FakeApi, id = FIGUARTS_ITEM.id) =>
  api.callsTo("GET", `/api/items/${String(id)}/estimates`);

// AC-EST-01: 詳細を開いたら estimates を 1 回取る
describe("目安価格: 取得", () => {
  it("AC-EST-01 シートを開くと GET estimates を 1 回呼ぶ(Authorization は付けない)。サイトのリンクは出る", async () => {
    const { api, dialog } = await openSheet((a) => {
      a.estimates = () => est({ sites: [] });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("まだ価格情報はありません");
    });
    expect(gets(api)).toHaveLength(1);
    expect(gets(api)[0]?.headers.has("Authorization")).toBe(false);
    expect(siteRows(dialog)).toHaveLength(3);
  });
});

// AC-EST-02: サマリ
describe("目安価格: サマリ", () => {
  it("AC-EST-02 帯に ¥3,000 と ¥4,500、取得日 10/3(日付は summary_fetched_at の JST)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ summary_fetched_at: "2026-10-02T15:30:00Z" }); // UTC 10/2、JST 10/3
    });
    await waitFor(() => {
      expect(bandLow(dialog)).toHaveTextContent("¥3,000");
    });
    expect(within(dialog).getByTestId("band-high")).toHaveTextContent("¥4,500");
    expect(within(dialog).getByTestId("fetched-at")).toHaveTextContent("10/3");
  });

  it("AC-EST-02 summary_mid が null なら上限の金額を出さない(¥3,000 と「〜」だけ)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ summary_mid: null, sites: [siteEst(2, { mid: null, count: 1 })] });
    });
    await waitFor(() => {
      expect(bandLow(dialog)).toHaveTextContent("¥3,000");
    });
    expect(within(dialog).getByTestId("band-high")).not.toHaveTextContent("¥");
  });

  it("AC-EST-02 目安が無く estimates が空なら「まだ価格情報はありません」", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ sites: [], summary_low: null, summary_mid: null, summary_fetched_at: null });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("まだ価格情報はありません");
    });
  });

  it("AC-EST-02 目安を出せるサイトが無く、すべて no_result なら「出品ないかも」(ユーザー指示 2026-10-04)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () =>
        est({
          sites: [siteEst(2, { low: null, mid: null, count: 0, status: "no_result" })],
          summary_low: null,
          summary_mid: null,
          summary_fetched_at: null,
        });
    });
    expect(await within(dialog).findByTestId("no-listing")).toHaveTextContent("出品ないかも");
    expect(within(dialog).queryByTestId("price-band")).not.toBeInTheDocument();
    expect(within(dialog).queryByTestId("fetched-at")).not.toBeInTheDocument();
  });

  it("AC-EST-02 refreshing:true なら「更新中…」を添える(値が無くても出す)。更新中は文字だけ", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ refreshing: true });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    hasBand(dialog);
    expect(bandLow(dialog)).toHaveTextContent("¥3,000");
    expect(within(dialog).queryByRole("progressbar")).not.toBeInTheDocument();
  });
});

// AC-EST-03: サイトのタイル(旧: 行にサイト名・low〜mid・件数・在庫・状態の文。5-2 で形に置き換え。詳しくは App.detail.test.tsx の AC-5-2i-*)
describe("目安価格: サイトのタイル", () => {
  it("AC-EST-03 ジャンルの順に、low のあるサイトだけ価格(¥low)を出す。ジャンルに無いサイトは出さない。タイル全体が <a target=_blank rel=noopener>", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () =>
        est({
          sites: [
            siteEst(1, { low: 800, mid: null, count: 1, in_stock_count: 0 }),
            siteEst(2, { low: 3000, mid: 4500, count: 5, in_stock_count: 2 }),
            siteEst(99, { low: 1, mid: 2 }), // ジャンルに無いサイトは出さない
          ],
        });
    });
    await waitFor(() => {
      expect(sitePrice(dialog, "Amazon")).toHaveTextContent("¥3,000");
    });
    const rows = siteRows(dialog);
    expect(rows.map((r) => r.getAttribute("aria-label"))).toEqual(["Amazon", "メルカリ", "その他"]);
    expect(sitePrice(dialog, "メルカリ")).toHaveTextContent("¥800");
    expect(sitePrice(dialog, "メルカリ")).not.toHaveTextContent("〜");
    for (const r of rows) {
      expect(r).toHaveAttribute("target", "_blank");
      expect(r.getAttribute("rel")).toMatch(/\bnoopener\b/);
      expect(r.getAttribute("href")).toMatch(/^https:\/\//);
    }
  });

  it("AC-EST-03 目安が無いサイト(link_only など)はリンクだけ(価格・状態を出さない)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      expect(sitePrice(dialog, "Amazon")).toHaveTextContent("¥3,000");
    });
    expect(sitePrice(dialog, "その他")).not.toBeInTheDocument();
    const linkOnly = siteRows(dialog)[2];
    expect(linkOnly).not.toHaveAttribute("data-dim");
    expect(linkOnly).not.toHaveAttribute("data-failed");
  });

  it("AC-EST-03 status=failed は前回の値を出し、title に「最終取得: N日前」(JST の暦日。今日なら「今日」)", async () => {
    const { dialog } = await openSheet(
      (a) => {
        a.estimates = () =>
          est({
            sites: [
              siteEst(2, { status: "failed", fetched_at: "2026-09-30T00:00:00Z" }),
              siteEst(1, { status: "failed", fetched_at: "2026-10-02T14:59:00Z" }), // JST では前日
              siteEst(3, { status: "failed", fetched_at: "2026-10-03T01:00:00Z" }),
            ],
          });
      },
      { now: "2026-10-03T03:00:00Z" },
    );
    await waitFor(() => {
      expect(sitePrice(dialog, "Amazon")).toHaveTextContent("¥3,000");
    });
    const rows = siteRows(dialog);
    expect(rows[0]).toHaveAttribute("title", "最終取得: 3日前");
    expect(rows[1]).toHaveAttribute("title", "最終取得: 1日前");
    expect(rows[2]).toHaveAttribute("title", "最終取得: 今日");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("AC-EST-03 status=failed で前回値が無ければ価格なし(data-failed)。リンクは出る。ok のタイルに最終取得(title)は付けない", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () =>
        est({
          sites: [
            siteEst(2, { status: "failed", low: null, mid: null, count: 0, in_stock_count: 0 }),
            siteEst(1),
          ],
        });
    });
    await waitFor(() => {
      expect(siteRows(dialog)[0]).toHaveAttribute("data-failed", "true");
    });
    expect(siteRows(dialog)[0]?.getAttribute("href")).toMatch(/^https:\/\/www\.amazon\.co\.jp\//);
    expect(sitePrice(dialog, "Amazon")).not.toBeInTheDocument();
    expect(siteRows(dialog)[1]).not.toHaveAttribute("title");
  });

  it("AC-EST-03 status=no_result は薄く(data-dim)「なし」だけ(検索結果へのリンクは残す)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () =>
        est({
          sites: [siteEst(2, { status: "no_result", low: null, mid: null, count: 0, in_stock_count: 0 })],
        });
    });
    await waitFor(() => {
      expect(siteRows(dialog)[0]).toHaveAttribute("data-dim", "true");
    });
    expect(sitePrice(dialog, "Amazon")).toHaveTextContent("なし");
    expect(siteRows(dialog)[0]).toHaveAttribute("target", "_blank");
    expect(siteRows(dialog)[0]).toHaveAttribute("rel", "noopener");
  });
});

// AC-EST-04: refreshing の間は再取得する
describe("目安価格: refreshing の再取得", () => {
  it("AC-EST-04 refreshing:true の間は 5 秒後に GET し直し、false になったら止まる", async () => {
    const { api, clock, dialog } = await openSheet((a) => {
      a.estimates = (_id, nth) =>
        nth < 2 ? est({ refreshing: true, sites: [] }) : est({ refreshing: false });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    expect(gets(api)).toHaveLength(1);
    await clock.advance(4999);
    expect(gets(api)).toHaveLength(1);
    await clock.advance(1);
    await waitFor(() => {
      expect(gets(api)).toHaveLength(2);
    });
    await waitFor(() => {
      expect(clock.pendingCount()).toBe(1); // まだ refreshing:true なので次を待つ
    });
    await clock.advance(5000);
    await waitFor(() => {
      expect(bandLow(dialog)).toHaveTextContent("¥3,000");
    });
    expect(status(dialog)).not.toHaveTextContent("更新中");
    expect(gets(api)).toHaveLength(3);
    expect(clock.pendingCount()).toBe(0);
    await clock.advance(60_000);
    expect(gets(api)).toHaveLength(3);
  });

  it("AC-EST-04 refreshing:false なら再取得の予約をしない", async () => {
    const { api, clock, dialog } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      hasBand(dialog);
    });
    expect(clock.pendingCount()).toBe(0);
    await clock.advance(60_000);
    expect(gets(api)).toHaveLength(1);
  });

  it("AC-EST-04 refreshing:true が続いても再取得は最大 6 回(最初と合わせて 7 回)。打ち切ったら「更新中…」を消す", async () => {
    const { api, clock, dialog } = await openSheet((a) => {
      a.estimates = () => est({ refreshing: true });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    for (let i = 1; i <= 6; i++) {
      await clock.advance(5000);
      await waitFor(() => {
        expect(gets(api)).toHaveLength(1 + i);
      });
    }
    await waitFor(() => {
      expect(status(dialog)).not.toHaveTextContent("更新中");
    });
    expect(bandLow(dialog)).toHaveTextContent("¥3,000"); // 値は残す
    expect(clock.pendingCount()).toBe(0);
    await clock.advance(60_000);
    expect(gets(api)).toHaveLength(7);
  });

  it("AC-EST-04 シートを閉じたら予約を取り消し、それ以後は GET しない", async () => {
    const { api, clock, dialog, user } = await openSheet((a) => {
      a.estimates = () => est({ refreshing: true });
    });
    await waitFor(() => {
      expect(clock.pendingCount()).toBe(1);
    });
    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));
    expect(clock.pendingCount()).toBe(0);
    await clock.advance(60_000);
    expect(gets(api)).toHaveLength(1);
  });
});

// AC-EST-05: 手動更新
describe("目安価格: 手動更新", () => {
  it("AC-EST-05 「更新」で POST estimates/refresh を呼び、更新中を出して 5 秒後に GET し直す", async () => {
    const { api, clock, dialog, user } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      hasBand(dialog);
    });
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(0);
    await user.click(within(dialog).getByRole("button", { name: "更新" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")[0]?.headers.has("Authorization")).toBe(
      false,
    );
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…"); // POST の本文は refreshing:true
    });
    expect(gets(api)).toHaveLength(1);
    await clock.advance(5000);
    await waitFor(() => {
      expect(gets(api)).toHaveLength(2);
    });
    await waitFor(() => {
      expect(status(dialog)).not.toHaveTextContent("更新中");
    });
    expect(clock.pendingCount()).toBe(0);
  });
});

// AC-EST-06: あやしい出品(旧「参考外 N件」の折りたたみ。5-2 で「あやしい出品 N」のボタン→一覧に置き換え。ユーザー決定 2026-10-11 / phase5-spec 5-2)
describe("目安価格: あやしい出品", () => {
  const suspicious = [
    listing(1, {
      title: "メルカリの安い出品",
      price: 300,
      suspicious_reasons: ["title_mismatch", "too_cheap"],
    }),
    listing(2, {
      title: "下限未満の出品",
      price: 500,
      site_id: 2,
      suspicious_reasons: ["below_min"],
      url: "javascript:alert(1)",
      image_url: "data:image/png;base64,AAAA",
    }),
    listing(3, { title: "参考にしている出品", price: 4800 }),
  ];
  const withSuspicious = (a: FakeApi) => {
    a.estimates = () =>
      est({ sites: [siteEst(2, { suspicious_count: 1 }), siteEst(1, { suspicious_count: 2 })] });
    a.listings = suspicious;
  };

  it("AC-EST-06 suspicious_count の合計が 0 なら「あやしい出品」を出さず、listings も取らない", async () => {
    const { api, dialog } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      hasBand(dialog);
    });
    expect(within(dialog).queryByText(/あやしい出品/)).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/参考外/)).not.toBeInTheDocument();
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(0);
  });

  it("AC-EST-06 「あやしい出品 N」(合計)のボタン。押すまで listings を取らない", async () => {
    const { api, dialog } = await openSheet(withSuspicious);
    const button = await within(dialog).findByRole("button", { name: "あやしい出品 3" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(0);
    expect(within(dialog).queryByRole("list", { name: "あやしい出品" })).not.toBeInTheDocument();
  });

  it("AC-EST-06 押すと GET listings を呼び、理由のある出品だけを画像・価格・理由のタグ・サイト・リンクで並べる", async () => {
    const { api, dialog, user } = await openSheet(withSuspicious);
    await user.click(await within(dialog).findByRole("button", { name: "あやしい出品 3" }));
    const list = await within(dialog).findByRole("list", { name: "あやしい出品" });
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(1);
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(list).not.toHaveTextContent("参考にしている出品");

    const first = rows[0];
    if (!first) throw new Error("no row");
    expect(first).toHaveTextContent("¥300");
    // 理由は小さなタグ(data-reason が種類、文字は短く)
    const tags = within(first).getAllByTestId("reason-tag");
    expect(tags.map((t) => t.getAttribute("data-reason"))).toEqual(["title_mismatch", "too_cheap"]);
    expect(tags.map((t) => t.textContent)).toEqual(["名前が違う", "安すぎ"]);
    // サイトのマーク(site_id 1 = メルカリ)
    expect(within(first).getByTestId("listing-site")).toHaveAttribute("aria-label", "メルカリ");
    expect(within(first).getByRole("img", { name: "メルカリの安い出品" })).toHaveAttribute(
      "src",
      "https://img.example/l/1.png",
    );
    const a = within(first).getByRole("link");
    expect(a).toHaveAttribute("href", "https://shop.example/l/1");
    expect(a).toHaveAttribute("target", "_blank");
    expect(a.getAttribute("rel")).toMatch(/\bnoopener\b/);
  });

  it("AC-EST-06 below_min のタグは「下限より下」。URL・画像が http(s) でない出品はリンクにも画像にもしない(価格・タグ・サイトは出す)", async () => {
    const { dialog, user } = await openSheet(withSuspicious);
    await user.click(await within(dialog).findByRole("button", { name: "あやしい出品 3" }));
    const list = await within(dialog).findByRole("list", { name: "あやしい出品" });
    const second = within(list).getAllByRole("listitem")[1];
    if (!second) throw new Error("no row");
    expect(second).toHaveTextContent("¥500");
    expect(within(second).getByTestId("reason-tag")).toHaveTextContent("下限より下");
    expect(within(second).getByTestId("listing-site")).toHaveAttribute("aria-label", "Amazon");
    expect(within(second).queryByRole("link")).not.toBeInTheDocument();
    expect(within(second).queryByRole("img")).not.toBeInTheDocument();
    expect(list.querySelector("[href^='javascript'], [src^='data:']")).toBeNull();
  });

  it("AC-EST-06 一覧の「戻る」で一覧を閉じ、シートの本体(帯・タイル)に戻る", async () => {
    const { dialog, user } = await openSheet(withSuspicious);
    await user.click(await within(dialog).findByRole("button", { name: "あやしい出品 3" }));
    await within(dialog).findByRole("list", { name: "あやしい出品" });
    await user.click(within(dialog).getByRole("button", { name: "戻る" }));
    expect(within(dialog).queryByRole("list", { name: "あやしい出品" })).not.toBeInTheDocument();
    expect(within(dialog).getByTestId("price-band")).toBeInTheDocument();
  });
});

// AC-EST-07: 取得できないとき
describe("目安価格: 取得失敗", () => {
  it("AC-EST-07 estimates が通信失敗ならサマリは「オフライン」。リンクは出て、alert は出さず、更新は押せず、再取得も予約しない", async () => {
    const { api, clock, dialog } = await openSheet((a) => {
      a.failEstimates = true;
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("オフライン");
    });
    expect(gets(api)).toHaveLength(1);
    expect(siteRows(dialog)).toHaveLength(3);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "更新" })).toBeDisabled();
    expect(clock.pendingCount()).toBe(0);
  });

  it("AC-EST-10 表示中に更新(POST)が失敗しても、サイト別の目安と前回のサマリが残り、短い文が添えられる", async () => {
    const { api, clock, dialog, user } = await openSheet((a) => {
      a.estimates = () => est();
      a.failRefresh = true;
    });
    await waitFor(() => {
      hasBand(dialog);
    });
    await user.click(within(dialog).getByRole("button", { name: "更新" }));
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("オフライン(前回の値)");
    });
    hasBand(dialog);
    expect(sitePrice(dialog, "Amazon")).toHaveTextContent("¥3,000");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(clock.pendingCount()).toBe(0);
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(1);
  });

  it("AC-EST-10 ポーリング中の GET が失敗しても、前回の目安が残る", async () => {
    const { api, clock, dialog } = await openSheet((a) => {
      a.estimates = () => est({ refreshing: true });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    api.failEstimates = true;
    await clock.advance(5000);
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("オフライン(前回の値)");
    });
    hasBand(dialog);
    expect(status(dialog)).not.toHaveTextContent("更新中");
    expect(sitePrice(dialog, "Amazon")).toHaveTextContent("¥3,000");
    expect(clock.pendingCount()).toBe(0);
  });
});

describe("目安価格: 更新中の操作と応答の順序", () => {
  it("AC-EST-11 更新中(POST の応答待ち・refreshing の間)は「更新」が押せず、POST は 1 回だけ", async () => {
    let release: () => void = () => undefined;
    const { api, clock, dialog, user } = await openSheet((a) => {
      a.estimates = () => est();
      a.delay = (c) => (c.method === "POST" ? new Promise<void>((r) => (release = r)) : undefined);
    });
    await waitFor(() => {
      hasBand(dialog);
    });
    const button = within(dialog).getByRole("button", { name: "更新" });
    await user.click(button);
    await waitFor(() => {
      expect(button).toBeDisabled(); // POST の応答待ち
    });
    release();
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    expect(button).toBeDisabled(); // refreshing の間
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(1);
    await clock.advance(5000);
    await waitFor(() => {
      expect(button).toBeEnabled();
    });
  });

  it("AC-EST-12 遅れて返った古い GET は、新しい POST の結果を上書きしない", async () => {
    let release: () => void = () => undefined;
    const { api, dialog, user } = await openSheet((a) => {
      a.estimates = (_id, nth) =>
        nth === 0
          ? est({ summary_low: 1111, summary_mid: 1222 })
          : est({ summary_low: 2222, summary_mid: 2333 });
      a.delay = (c) =>
        c.method === "GET" && c.path === "/api/items/12/estimates" && gets(a).length === 1
          ? new Promise<void>((r) => (release = r))
          : undefined;
    });
    await waitFor(() => {
      expect(gets(api)).toHaveLength(1); // 最初の GET は保留中
    });
    await user.click(within(dialog).getByRole("button", { name: "更新" }));
    await waitFor(() => {
      expect(bandLow(dialog)).toHaveTextContent("¥2,222");
    });
    release(); // 古い GET がここで返る
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(bandLow(dialog)).toHaveTextContent("¥2,222");
    expect(bandLow(dialog)).not.toHaveTextContent("¥1,111");
  });
});
