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
const status = (d: HTMLElement) => within(d).getByRole("status");
const siteRows = (d: HTMLElement) =>
  within(within(d).getByRole("list", { name: "サイト" })).getAllByRole("link");
const gets = (api: FakeApi, id = FIGUARTS_ITEM.id) =>
  api.callsTo("GET", `/api/items/${String(id)}/estimates`);

// AC-EST-01: 詳細を開いたら estimates を 1 回取る
describe("目安価格: 取得", () => {
  it("AC-EST-01 シートを開くと GET estimates を 1 回呼ぶ(Bearer 付き)。サイト行のリンクは出る", async () => {
    const { api, dialog } = await openSheet((a) => {
      a.estimates = () => est({ sites: [] });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("まだ価格情報はありません");
    });
    expect(gets(api)).toHaveLength(1);
    expect(gets(api)[0]?.headers.get("Authorization")).toBe("Bearer test-token");
    expect(siteRows(dialog)).toHaveLength(3);
  });
});

// AC-EST-02: サマリ
describe("目安価格: サマリ", () => {
  it("AC-EST-02 「だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)」(日付は summary_fetched_at の JST)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ summary_fetched_at: "2026-10-02T15:30:00Z" }); // UTC 10/2、JST 10/3
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)");
    });
  });

  it("AC-EST-02 summary_mid が null なら「¥3,000〜 で買えそう」", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ summary_mid: null, sites: [siteEst(2, { mid: null, count: 1 })] });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("だいたい ¥3,000〜 で買えそう(10/3 時点)");
    });
    expect(status(dialog)).not.toHaveTextContent("〜¥");
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
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("出品ないかも");
    });
    expect(status(dialog)).not.toHaveTextContent("時点");
  });

  it("AC-EST-02 refreshing:true なら「更新中…」を添える(値が無くても出す)。更新中は文字だけ", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est({ refreshing: true });
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("更新中…");
    });
    expect(status(dialog)).toHaveTextContent("だいたい ¥3,000〜¥4,500 で買えそう");
    expect(within(dialog).queryByRole("progressbar")).not.toBeInTheDocument();
  });
});

// AC-EST-03: サイト行
describe("目安価格: サイト行", () => {
  it("AC-EST-03 ジャンルの順に、サイト名・目安価格(low〜mid)・件数・在庫を出す。行全体が <a target=_blank rel=noopener>", async () => {
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
      expect(siteRows(dialog)[0]).toHaveTextContent("¥3,000〜¥4,500");
    });
    const rows = siteRows(dialog);
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining("Amazon"),
      expect.stringContaining("メルカリ"),
      expect.stringContaining("その他"),
    ]);
    expect(rows[0]).toHaveTextContent("5件");
    expect(rows[0]).toHaveTextContent("在庫あり");
    expect(rows[0]).not.toHaveTextContent("在庫なし");
    // mid が null: 「¥low〜」。in_stock_count が 0: 在庫なし
    expect(rows[1]).toHaveTextContent("¥800〜");
    expect(rows[1]).not.toHaveTextContent("〜¥");
    expect(rows[1]).toHaveTextContent("1件");
    expect(rows[1]).toHaveTextContent("在庫なし");
    for (const r of rows) {
      expect(r).toHaveAttribute("target", "_blank");
      expect(r.getAttribute("rel")).toMatch(/\bnoopener\b/);
      expect(r.getAttribute("href")).toMatch(/^https:\/\//);
    }
  });

  it("AC-EST-03 目安が無いサイト(link_only など)は今までどおりリンクだけ(価格・件数・在庫・状態を出さない)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      expect(siteRows(dialog)[0]).toHaveTextContent("¥3,000");
    });
    const linkOnly = siteRows(dialog)[2];
    expect(linkOnly?.textContent).toContain("その他");
    for (const w of ["¥", "件", "在庫", "最終取得", "出品ないかも"])
      expect(linkOnly).not.toHaveTextContent(w);
  });

  it("AC-EST-03 status=failed は前回の値と「最終取得: N日前」(JST の暦日。今日なら「今日」)", async () => {
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
      expect(siteRows(dialog)[0]).toHaveTextContent("最終取得");
    });
    const rows = siteRows(dialog);
    expect(rows[0]).toHaveTextContent("¥3,000〜¥4,500");
    expect(rows[0]).toHaveTextContent("最終取得: 3日前");
    expect(rows[1]).toHaveTextContent("最終取得: 1日前");
    expect(rows[2]).toHaveTextContent("最終取得: 今日");
    expect(rows[0]).not.toHaveTextContent("エラー");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("AC-EST-03 status=failed で前回値が無ければ「取得できませんでした」。リンクは出る。ok の行に最終取得は出さない", async () => {
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
      expect(siteRows(dialog)[0]).toHaveTextContent("取得できませんでした");
    });
    expect(siteRows(dialog)[0]?.getAttribute("href")).toMatch(/^https:\/\/www\.amazon\.co\.jp\//);
    expect(siteRows(dialog)[0]).not.toHaveTextContent("¥");
    expect(siteRows(dialog)[1]).not.toHaveTextContent("最終取得");
  });

  it("AC-EST-03 status=no_result は「出品ないかも」だけ(金額・件数・在庫は出さず、検索結果へのリンクは残す)", async () => {
    const { dialog } = await openSheet((a) => {
      a.estimates = () =>
        est({
          sites: [siteEst(2, { status: "no_result", low: null, mid: null, count: 0, in_stock_count: 0 })],
        });
    });
    await waitFor(() => {
      expect(siteRows(dialog)[0]).toHaveTextContent("出品ないかも");
    });
    for (const w of ["¥", "件", "在庫"]) expect(siteRows(dialog)[0]).not.toHaveTextContent(w);
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
      expect(status(dialog)).toHaveTextContent("だいたい ¥3,000〜¥4,500 で買えそう");
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
      expect(status(dialog)).toHaveTextContent("だいたい");
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
    expect(status(dialog)).toHaveTextContent("だいたい ¥3,000〜¥4,500 で買えそう"); // 値は残す
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
      expect(status(dialog)).toHaveTextContent("だいたい");
    });
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(0);
    await user.click(within(dialog).getByRole("button", { name: "更新" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items/12/estimates/refresh")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/items/12/estimates/refresh")[0]?.headers.get("Authorization")).toBe(
      "Bearer test-token",
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

// AC-EST-06: 参考外
describe("目安価格: 参考外", () => {
  const suspicious = [
    listing(1, {
      title: "メルカリの安い出品",
      price: 300,
      suspicious_reasons: ["title_mismatch", "too_cheap"],
    }),
    listing(2, {
      title: "下限未満の出品",
      price: 500,
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

  it("AC-EST-06 suspicious_count の合計が 0 なら「参考外」の文字を出さず、listings も取らない", async () => {
    const { api, dialog } = await openSheet((a) => {
      a.estimates = () => est();
    });
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("だいたい");
    });
    expect(within(dialog).queryByText(/参考外/)).not.toBeInTheDocument();
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(0);
  });

  it("AC-EST-06 「参考外 N件」(合計)は折りたたみ。開くまで listings を取らない", async () => {
    const { api, dialog } = await openSheet(withSuspicious);
    const toggle = await within(dialog).findByText("参考外 3件");
    expect(toggle.closest("details") ?? toggle.closest("[aria-expanded]")).not.toBeNull();
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(0);
    expect(within(dialog).queryByRole("list", { name: "参考外の出品" })).not.toBeInTheDocument();
  });

  it("AC-EST-06 開くと GET listings を呼び、理由のある出品だけを画像・タイトル・価格・理由・リンクで並べる", async () => {
    const { api, dialog, user } = await openSheet(withSuspicious);
    await user.click(await within(dialog).findByText("参考外 3件"));
    const list = await within(dialog).findByRole("list", { name: "参考外の出品" });
    expect(api.callsTo("GET", "/api/items/12/listings")).toHaveLength(1);
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(list).not.toHaveTextContent("参考にしている出品");

    const first = rows[0];
    if (!first) throw new Error("no row");
    expect(first).toHaveTextContent("メルカリの安い出品");
    expect(first).toHaveTextContent("¥300");
    expect(first).toHaveTextContent("商品名が一致しない");
    expect(first).toHaveTextContent("安すぎる");
    expect(within(first).getByRole("img", { name: "メルカリの安い出品" })).toHaveAttribute(
      "src",
      "https://img.example/l/1.png",
    );
    const a = within(first).getByRole("link");
    expect(a).toHaveAttribute("href", "https://shop.example/l/1");
    expect(a).toHaveAttribute("target", "_blank");
    expect(a.getAttribute("rel")).toMatch(/\bnoopener\b/);
  });

  it("AC-EST-06 URL・画像が http(s) でない出品はリンクにも画像にもしない(題名・価格・理由は出す)", async () => {
    const { dialog, user } = await openSheet(withSuspicious);
    await user.click(await within(dialog).findByText("参考外 3件"));
    const list = await within(dialog).findByRole("list", { name: "参考外の出品" });
    const second = within(list).getAllByRole("listitem")[1];
    if (!second) throw new Error("no row");
    expect(second).toHaveTextContent("下限未満の出品");
    expect(second).toHaveTextContent("¥500");
    expect(second).toHaveTextContent("下限価格未満");
    expect(within(second).queryByRole("link")).not.toBeInTheDocument();
    expect(within(second).queryByRole("img")).not.toBeInTheDocument();
    expect(list.querySelector("[href^='javascript'], [src^='data:']")).toBeNull();
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
      expect(status(dialog)).toHaveTextContent("だいたい");
    });
    await user.click(within(dialog).getByRole("button", { name: "更新" }));
    await waitFor(() => {
      expect(status(dialog)).toHaveTextContent("オフライン(前回の値)");
    });
    expect(status(dialog)).toHaveTextContent("だいたい");
    expect(within(dialog).getByText(/5件/)).toBeInTheDocument();
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
    expect(status(dialog)).toHaveTextContent("だいたい");
    expect(status(dialog)).not.toHaveTextContent("更新中");
    expect(within(dialog).getByText(/5件/)).toBeInTheDocument();
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
      expect(status(dialog)).toHaveTextContent("だいたい");
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
      expect(status(dialog)).toHaveTextContent("2,222");
    });
    release(); // 古い GET がここで返る
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(status(dialog)).toHaveTextContent("2,222");
    expect(status(dialog)).not.toHaveTextContent("1,111");
  });
});
