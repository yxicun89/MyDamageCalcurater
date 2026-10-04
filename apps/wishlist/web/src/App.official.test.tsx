import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Item, OfficialStatus } from "./api/types";
import { makeItem } from "./test/factories";
import { grid, gridItem, openHome } from "./test/render";
import { scenario } from "./test/scenario";

// フェーズ4-3 公式サイトの販売状況(詳細シート・編集・グリッド。docs/phase4-spec.md AC-OFF-05〜09)。
// 商品 12(グリス)を監視の対象にする。

const status = (over: Partial<OfficialStatus> = {}): OfficialStatus => ({
  status: "preorder",
  evidence: ["予約受付中", "予約する"],
  checked_at: "2026-10-03T18:00:00Z",
  changed_at: null,
  previous_status: null,
  last_result: "preorder",
  last_attempt_at: "2026-10-03T18:00:00Z",
  ...over,
});

function withItem12(over: Partial<Item>) {
  const s = scenario();
  s.items = s.items.map((it) =>
    it.id === 12
      ? makeItem({ ...it, source_url: "https://tamashii.example/item/1/", watch_official: true, ...over })
      : it,
  );
  return s;
}

async function openSheet(over: Partial<Item>) {
  const r = await openHome(withItem12(over));
  await r.user.click(gridItem("グリス"));
  const dialog = screen.getByRole("dialog", { name: "詳細" });
  return { ...r, dialog };
}

const officialRegion = (d: HTMLElement) => within(d).queryByRole("region", { name: "公式の販売状況" });

afterEach(() => {
  vi.useRealTimers();
});

describe("詳細シート: 公式の販売状況", () => {
  it("AC-OFF-05 監視中なら「公式: 予約受付中(10/4 確認)」と根拠を出す", async () => {
    const { dialog } = await openSheet({ official_status: status() });
    const region = officialRegion(dialog);
    expect(region).not.toBeNull();
    expect(region).toHaveTextContent("公式: 予約受付中(10/4 確認)");
    expect(region).toHaveTextContent("根拠: 予約受付中・予約する");
    expect(within(region as HTMLElement).queryByRole("alert")).not.toBeInTheDocument();
  });

  it("AC-OFF-05 changed_at が 7 日以内なら「10/3 に 販売中 → 販売終了」を添える。8 日目は出さない", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-04T00:00:00Z"));
    const changed = status({
      status: "ended",
      evidence: ["販売終了"],
      changed_at: "2026-10-02T18:00:00Z",
      previous_status: "available",
      last_result: "ended",
    });
    const { dialog, user } = await openSheet({ official_status: changed });
    expect(officialRegion(dialog)).toHaveTextContent("公式: 販売終了(10/4 確認)");
    expect(officialRegion(dialog)).toHaveTextContent("10/3 に 販売中 → 販売終了");

    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));
    vi.setSystemTime(new Date("2026-10-10T18:00:00Z"));
    await user.click(gridItem("グリス"));
    const again = screen.getByRole("dialog", { name: "詳細" });
    expect(officialRegion(again)).toHaveTextContent("公式: 販売終了(10/4 確認)");
    expect(officialRegion(again)).not.toHaveTextContent("→");
  });

  it.each<[string, Partial<OfficialStatus>, string]>([
    [
      "unknown",
      { status: "unknown", evidence: [], last_result: "unknown" },
      "公式: 判定できません(10/4 確認)",
    ],
    [
      "ambiguous",
      { status: "ambiguous", evidence: ["カートに入れる", "在庫切れ"], last_result: "ambiguous" },
      "公式: 判定できません(複数の表示)(10/4 確認)",
    ],
    [
      "blocked",
      { status: "blocked", evidence: [], last_result: "blocked" },
      "公式: 取得しません(robots.txt)(10/4)",
    ],
    ["failed", { status: "failed", evidence: [], last_result: "failed" }, "公式: 取得できませんでした(10/4)"],
  ])("AC-OFF-06 %s の文言", async (_name, over, want) => {
    const { dialog } = await openSheet({ official_status: status(over) });
    expect(officialRegion(dialog)).toHaveTextContent(want);
  });

  it("AC-OFF-06 判定済みのまま最後の試行が失敗したら「最新の確認(10/5): 取得できませんでした」を添える", async () => {
    const { dialog } = await openSheet({
      official_status: status({ last_result: "failed", last_attempt_at: "2026-10-04T18:00:00Z" }),
    });
    expect(officialRegion(dialog)).toHaveTextContent("公式: 予約受付中(10/4 確認)");
    expect(officialRegion(dialog)).toHaveTextContent("最新の確認(10/5): 取得できませんでした");
  });

  it("AC-OFF-07 監視中でまだ確かめていなければ「公式: まだ確認していません」", async () => {
    const { dialog } = await openSheet({ official_status: null });
    expect(officialRegion(dialog)).toHaveTextContent("公式: まだ確認していません");
  });

  it("AC-OFF-07 監視していなければ、保存済みの状態があっても何も出さない", async () => {
    const { dialog } = await openSheet({ watch_official: false, official_status: status() });
    expect(officialRegion(dialog)).toBeNull();
    expect(within(dialog).queryByText(/^公式:/)).not.toBeInTheDocument();
  });

  it("AC-OFF-07 古い応答(watch_official・official_status が無い)でも開ける", async () => {
    const s = scenario();
    const r = await openHome(s);
    await r.user.click(gridItem("グリス"));
    const dialog = screen.getByRole("dialog", { name: "詳細" });
    expect(officialRegion(dialog)).toBeNull();
    expect(within(dialog).getByRole("status")).toBeInTheDocument();
  });
});

describe("グリッド", () => {
  it("AC-OFF-08 グリッドには文字を出さない(販売状況があっても)", async () => {
    await openHome(
      withItem12({
        official_status: status({
          status: "ended",
          previous_status: "available",
          changed_at: "2026-10-02T18:00:00Z",
        }),
      }),
    );
    expect(grid().textContent.trim()).toBe("");
    expect(within(grid()).queryByText(/販売|予約|公式/)).not.toBeInTheDocument();
  });
});

describe("編集: 公式ページを監視する", () => {
  async function openEdit(over: Partial<Item>) {
    const r = await openHome(withItem12(over));
    fireEvent.pointerDown(gridItem("グリス"), { pointerId: 1, button: 0 });
    const menu = await screen.findByRole("menu", {}, { timeout: 1500 });
    fireEvent.pointerUp(gridItem("グリス"), { pointerId: 1 });
    await r.user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    return { ...r, dlg, box: within(dlg).getByRole("checkbox", { name: "公式ページを監視する" }) };
  }

  it("AC-OFF-09 チェックは現在の watch_official。外して保存すると watch_official: false だけを PATCH", async () => {
    const { user, api, dlg, box } = await openEdit({});
    expect(box).toBeChecked();
    expect(box).toBeEnabled();
    await user.click(box);
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/12")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/12")[0]?.json).toEqual({ watch_official: false });
  });

  it("AC-OFF-09 source_url があり OFF の商品は、チェックして保存すると watch_official: true", async () => {
    const { user, api, dlg, box } = await openEdit({ watch_official: false });
    expect(box).not.toBeChecked();
    await user.click(box);
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/12")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/12")[0]?.json).toEqual({ watch_official: true });
  });

  it("AC-OFF-09 source_url が無ければチェックは無効で、理由を出す。変えなければ watch_official を送らない", async () => {
    const { user, api, dlg, box } = await openEdit({ source_url: null, watch_official: false });
    expect(box).toBeDisabled();
    expect(within(dlg).getByText("公式ページの URL が無いので監視できません")).toBeInTheDocument();
    await user.clear(within(dlg).getByRole("textbox", { name: "名前" }));
    await user.type(within(dlg).getByRole("textbox", { name: "名前" }), "グリス改");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/items/12")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/items/12")[0]?.json).toEqual({ name: "グリス改" });
  });

  it("AC-OFF-09 watch_official の無い古い応答は OFF として扱う(変えなければ送らない)", async () => {
    const s = scenario();
    s.items = s.items.map((it) =>
      it.id === 12 ? { ...it, source_url: "https://tamashii.example/item/1/" } : it,
    );
    const r = await openHome(s);
    fireEvent.pointerDown(gridItem("グリス"), { pointerId: 1, button: 0 });
    const menu = await screen.findByRole("menu", {}, { timeout: 1500 });
    fireEvent.pointerUp(gridItem("グリス"), { pointerId: 1 });
    await r.user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    const box = within(dlg).getByRole("checkbox", { name: "公式ページを監視する" });
    expect(box).not.toBeChecked();
    expect(box).toBeEnabled();
    await r.user.click(within(dlg).getByRole("button", { name: "保存" }));
    expect(r.api.callsTo("PATCH", "/api/items/12")).toHaveLength(0);
  });
});
