import { readFileSync } from "node:fs";
import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { PriceHistory } from "./api/types";
import type { FakeApi } from "./test/fakeApi";
import { webPath } from "./test/paths";
import { gridItem, openHome } from "./test/render";

// フェーズ4-2 価格の推移(詳細シート。docs/phase4-spec.md AC-HIS-01〜06・10)。
// 商品 12(グリス)はジャンル 2(S.H.Figuarts)。サイトは 1 メルカリ・2 Amazon・3 その他。

const history = (over: Partial<PriceHistory> = {}): PriceHistory => ({
  item_id: 12,
  days: 90,
  sites: [
    {
      site_id: 1,
      points: [
        { day: "2026-10-01", low: 3000, mid: 3400 },
        { day: "2026-10-03", low: 3500, mid: null },
      ],
    },
    { site_id: 2, points: [{ day: "2026-10-02", low: 3300, mid: 3600 }] },
    { site_id: 9, points: [{ day: "2026-10-03", low: 3600, mid: null }] },
  ],
  overall: [
    { day: "2026-10-01", low: 3000 },
    { day: "2026-10-02", low: 3300 },
    { day: "2026-10-03", low: 3500 },
  ],
  ...over,
});

async function openSheet(prepare: (api: FakeApi) => void = () => undefined) {
  const r = await openHome();
  prepare(r.api);
  await r.user.click(gridItem("グリス"));
  const dialog = screen.getByRole("dialog", { name: "詳細" });
  return { ...r, dialog };
}
const historyCalls = (api: FakeApi) => api.callsTo("GET", "/api/items/12/price-history");
const toggle = (d: HTMLElement) => within(d).findByText("価格の推移");
const chart = (d: HTMLElement) => within(d).findByRole("img", { name: /^価格の推移 / });
const seriesPaths = (svg: Element, key: string) => svg.querySelectorAll(`path[data-series="${key}"]`);

// AC-HIS-01: 折りたたみ。開いたときに取る
describe("価格の推移: 取得", () => {
  it("AC-HIS-01 「価格の推移」は折りたたみで、開くまで GET price-history を呼ばない", async () => {
    const { api, dialog } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    const summary = await toggle(dialog);
    expect(summary.closest("details") ?? summary.closest("[aria-expanded]")).not.toBeNull();
    expect(historyCalls(api)).toHaveLength(0);
    expect(within(dialog).queryByRole("img", { name: /^価格の推移/ })).not.toBeInTheDocument();
  });

  it("AC-HIS-01 開くと GET price-history を 1 回(Bearer 付き・days を付けない)。閉じて開き直しても取り直さない", async () => {
    const { api, dialog, user } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    await user.click(await toggle(dialog));
    await chart(dialog);
    expect(historyCalls(api)).toHaveLength(1);
    expect(historyCalls(api)[0]?.search).toBe("");
    expect(historyCalls(api)[0]?.headers.get("Authorization")).toBe("Bearer test-token");
    await user.click(await toggle(dialog));
    await user.click(await toggle(dialog));
    await chart(dialog);
    expect(historyCalls(api)).toHaveLength(1);
  });
});

// AC-HIS-02〜04: 表示
describe("価格の推移: グラフ", () => {
  it.each([
    { name: "点なし", overall: [] },
    { name: "1 点", overall: [{ day: "2026-10-03", low: 3000 }] },
  ])(
    "AC-HIS-02 全体の最安の点が 2 未満($name)なら「推移はまだありません」(グラフを出さない)",
    async ({ overall }) => {
      const { dialog, user } = await openSheet((a) => {
        a.priceHistory = () =>
          history({
            overall,
            sites: overall.map((o) => ({ site_id: 1, points: [{ ...o, mid: null }] })),
          });
      });
      await user.click(await toggle(dialog));
      expect(await within(dialog).findByText("推移はまだありません")).toBeInTheDocument();
      expect(within(dialog).queryByRole("img", { name: /^価格の推移/ })).not.toBeInTheDocument();
    },
  );

  it("AC-HIS-03 SVG(role=img)の aria-label に期間・最安・最高。既定は全体の最安の線だけで、点は実際の日だけ", async () => {
    const { dialog, user } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    await user.click(await toggle(dialog));
    const svg = await chart(dialog);
    expect(svg.tagName.toLowerCase()).toBe("svg");
    expect(svg).toHaveAccessibleName("価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,500");
    expect(seriesPaths(svg, "overall")).toHaveLength(1);
    expect(svg.querySelectorAll('circle[data-series="overall"]')).toHaveLength(3);
    expect(svg.querySelectorAll('path[data-series^="site-"]')).toHaveLength(0);
    // 軸: 日付は M/D、金額は ¥ 表記
    const text = svg.textContent;
    for (const label of ["10/1", "10/3", "¥3,000", "¥3,500"]) expect(text).toContain(label);
    // 常時動く演出を入れない
    expect(svg.querySelectorAll("animate, animateTransform, animateMotion, set")).toHaveLength(0);
  });

  it("AC-HIS-03 色は CSS 変数(ライト/ダークで切り替わる)。全体は --chart-overall、サイトは --chart-site-N", async () => {
    const { dialog, user } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    await user.click(await toggle(dialog));
    const svg = await chart(dialog);
    expect(seriesPaths(svg, "overall")[0]?.getAttribute("stroke")).toBe("var(--chart-overall)");
    const legend = within(dialog).getByRole("group", { name: "凡例" });
    await user.click(within(legend).getByRole("button", { name: "メルカリ" }));
    expect(seriesPaths(svg, "site-1")[0]?.getAttribute("stroke")).toMatch(/^var\(--chart-site-[1-4]\)$/);
  });

  it("AC-HIS-04 凡例はサイトごとのボタン(aria-pressed)。押すとそのサイトの線を足し、もう一度で消す。名前の無いサイトは「サイトN」", async () => {
    const { dialog, user } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    await user.click(await toggle(dialog));
    const svg = await chart(dialog);
    const legend = within(dialog).getByRole("group", { name: "凡例" });
    const buttons = within(legend).getAllByRole("button");
    expect(buttons.map((b) => b.textContent)).toEqual(["メルカリ", "Amazon", "サイト9"]);
    for (const b of buttons) expect(b).toHaveAttribute("aria-pressed", "false");

    const mercari = within(legend).getByRole("button", { name: "メルカリ" });
    await user.click(mercari);
    expect(mercari).toHaveAttribute("aria-pressed", "true");
    expect(seriesPaths(svg, "site-1")).toHaveLength(1);
    expect(svg.querySelectorAll('circle[data-series="site-1"]')).toHaveLength(2);
    expect(seriesPaths(svg, "overall")).toHaveLength(1);
    await user.click(mercari);
    expect(mercari).toHaveAttribute("aria-pressed", "false");
    expect(seriesPaths(svg, "site-1")).toHaveLength(0);
  });
});

// AC-HIS-05: 失敗してもシートは壊さない
describe("価格の推移: 失敗", () => {
  it.each([
    { name: "通信失敗", fail: "network" as const, text: "オフライン" },
    { name: "それ以外", fail: 500, text: "価格の推移を取得できませんでした" },
  ])(
    "AC-HIS-05 $name は折りたたみの中に「$text」。alert にせず、サマリ・サイト行は残る",
    async ({ fail, text }) => {
      const { dialog, user } = await openSheet((a) => {
        a.failPriceHistory = fail;
      });
      await user.click(await toggle(dialog));
      expect(await within(dialog).findByText(text)).toBeInTheDocument();
      expect(within(dialog).queryByRole("alert")).not.toBeInTheDocument();
      expect(within(dialog).getByRole("status")).toBeInTheDocument();
      expect(within(within(dialog).getByRole("list", { name: "サイト" })).getAllByRole("link")).toHaveLength(
        3,
      );
    },
  );
});

// AC-HIS-10: ライト/ダークの色
describe("価格の推移: 色", () => {
  it("AC-HIS-10 styles.css はグラフの色をライト(:root)とダーク(prefers-color-scheme: dark)の両方で定義する", () => {
    const css = readFileSync(webPath("src/styles.css"), "utf8");
    const dark = /@media \(prefers-color-scheme: dark\)\s*\{([\s\S]*?)\n\}/.exec(css)?.[1] ?? "";
    const light = css.slice(0, css.indexOf("@media (prefers-color-scheme: dark)"));
    for (const v of [
      "--chart-overall",
      "--chart-site-1",
      "--chart-site-2",
      "--chart-site-3",
      "--chart-site-4",
      "--chart-axis",
    ]) {
      expect(light, `ライトに ${v}`).toContain(`${v}:`);
      expect(dark, `ダークに ${v}`).toContain(`${v}:`);
    }
  });
});

// AC-HIS-06: 読み込み中の表示は短い文字だけ(スピナー・progressbar なし)。
describe("価格の推移: 読み込み中", () => {
  it("AC-HIS-06 読み込み中は「読み込み中…」の文字だけで progressbar を出さない", async () => {
    const { dialog, user, api } = await openSheet((a) => {
      a.priceHistory = () => history();
    });
    let release: () => void = () => undefined;
    api.delay = (c) =>
      c.path.endsWith("/price-history")
        ? new Promise<void>((r) => {
            release = r;
          })
        : undefined;
    await user.click(await toggle(dialog));
    const details = (await toggle(dialog)).closest("details") ?? dialog;
    expect(await within(details).findByText("読み込み中…")).toBeInTheDocument();
    expect(within(dialog).queryByRole("progressbar")).not.toBeInTheDocument();
    release();
    await waitFor(() => {
      expect(within(dialog).getByRole("img", { name: /^価格の推移 / })).toBeInTheDocument();
    });
  });
});
