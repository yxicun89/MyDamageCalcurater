// I-web-13g(G-05、ADR-0339・ADR-0348): お気に入り・計算履歴の見た目の作り直し(docs/design.md「画面ごとの方向」のお気に入り・計算履歴の行)。
// 確かめること:
//   - 見出しはアイコン + 名前(お気に入りは星、計算履歴は時計)
//   - お気に入りの行: 画像かエンブレム + 名前、「使う」「削除」はアイコン + 短い文字(読み上げ名は従来の長い名前のまま)
//   - 計算履歴の行: 攻撃側 → 防御側のアイコン、%は大きい数字 + 装飾のバー(100% で止まる)、日付は小さな補足、「使う」はアイコン + 短い文字
//   - 行を押せる範囲は使うボタンの範囲を行全体に広げる作り(Tab の停止点は増えない)
//   - 空の状態は大きいアイコン + 短い一言、読み込み中・失敗は ui-notice(失敗は role=alert のまま節の中)
//   - 「もっと見る」はアイコン付きのボタン
// 挙動(削除・ページング・復元)は既存のテストが守る。ここは新しい構造だけを見る。

import { act, render, screen, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { calcHistoryText, favoritesScreenText } from "../i18n/favorites";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { FavoritesScreen } from "./FavoritesScreen";

type Schemas = components["schemas"];
type Favorite = Schemas["Favorite"];
type Entry = Schemas["CalcHistoryEntry"];
type Page = Schemas["CalcHistoryPage"];

const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;

const FAV: Favorite = {
  id: "11111111-1111-4111-8111-111111111111",
  label: "HB特化",
  individual: { speciesKey: "9004-000", level: 50, natureId: "fake-nature", sp: SP },
  createdAt: "2026-10-04T01:00:00Z",
  updatedAt: "2026-10-04T01:00:00Z",
};

function entryOf(min: number, max: number): Entry {
  return {
    occurredAt: "2026-10-09T03:00:00Z",
    calc: {
      format: "single",
      attacker: { speciesKey: "9001-000", level: 50, natureId: "fake-nature", sp: SP },
      defender: { speciesKey: "9002-000", level: 50, natureId: "fake-nature", sp: SP },
      moveId: "fake-move-a",
    },
    result: { minPercent: min, maxPercent: max },
  };
}

function createRecord(
  favorites: RecordResult<Favorite[]> | "pending",
  history: RecordResult<Page> | "pending",
): RecordClient {
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  return {
    listFrequentOpponents: unused,
    deleteDeviceData: unused,
    createFavorite: unused,
    listFavorites: () =>
      favorites === "pending" ? new Promise(() => undefined) : Promise.resolve(favorites),
    deleteFavorite: unused,
    listCalcHistory: () => (history === "pending" ? new Promise(() => undefined) : Promise.resolve(history)),
  };
}

const okFavs = (value: Favorite[]): RecordResult<Favorite[]> => ({ ok: true, value });
const okPage = (items: Entry[], nextCursor: string | null): RecordResult<Page> => ({
  ok: true,
  value: { items, nextCursor },
});

async function renderScreen(
  favorites: RecordResult<Favorite[]> | "pending",
  history: RecordResult<Page> | "pending",
  onUse?: () => void,
): Promise<HTMLElement> {
  await act(async () => {
    render(<FavoritesScreen recordClient={createRecord(favorites, history)} onUse={onUse} />);
    await Promise.resolve();
  });
  return screen.getByRole("region", { name: favoritesScreenText.regionLabel });
}

describe("見出し", () => {
  test("お気に入りは星のアイコン + 名前、計算履歴は時計のアイコン + 名前(アイコンは装飾)", async () => {
    const region = await renderScreen(okFavs([]), okPage([], null));
    const h2 = within(region).getByRole("heading", { level: 2, name: favoritesScreenText.regionLabel });
    expect(h2.querySelector("svg[aria-hidden='true']")).not.toBeNull();
    const h3 = within(region).getByRole("heading", { level: 3, name: calcHistoryText.regionLabel });
    expect(h3.querySelector("svg[aria-hidden='true']")).not.toBeNull();
  });
});

describe("お気に入りの行", () => {
  test("画像かエンブレム + 名前。「使う」「削除」はアイコン + 短い文字で、読み上げ名は従来のまま", async () => {
    const onUse = (): void => undefined;
    await renderScreen(okFavs([FAV]), okPage([], null), onUse);
    const row = within(screen.getByRole("list", { name: favoritesScreenText.listLabel })).getByRole(
      "listitem",
    );
    expect(row.querySelector(".pokemon-icon")).not.toBeNull();
    const use = within(row).getByRole("button", { name: favoritesScreenText.useLabel("HB特化") });
    expect(use.querySelector("svg")).not.toBeNull();
    expect(use).toHaveTextContent(favoritesScreenText.useShort);
    expect(favoritesScreenText.useLabel("HB特化")).toContain(favoritesScreenText.useShort);
    const del = within(row).getByRole("button", { name: favoritesScreenText.deleteLabel("HB特化") });
    expect(del.querySelector("svg")).not.toBeNull();
    expect(del).toHaveTextContent(favoritesScreenText.deleteShort);
    expect(favoritesScreenText.deleteLabel("HB特化")).toContain(favoritesScreenText.deleteShort);
  });

  test("使うボタンは行全体を押せる範囲に広げる印(favorites-screen__use)を持ち、行に使うボタンは 1 つだけ", async () => {
    await renderScreen(okFavs([FAV]), okPage([], null), () => undefined);
    const row = screen.getByRole("list", { name: favoritesScreenText.listLabel }).querySelector("li");
    expect(row).toHaveClass("favorites-screen__item--pressable");
    expect(row?.querySelectorAll("button.favorites-screen__use")).toHaveLength(1);
  });

  test("onUse が無いときは押せる行にしない", async () => {
    await renderScreen(okFavs([FAV]), okPage([], null));
    const row = screen.getByRole("list", { name: favoritesScreenText.listLabel }).querySelector("li");
    expect(row).not.toHaveClass("favorites-screen__item--pressable");
  });
});

describe("計算履歴の行", () => {
  test("攻撃側 → 防御側のアイコン、%は大きい数字、バーは装飾で最大%の幅(100 で止まる)", async () => {
    await renderScreen(okFavs([]), okPage([entryOf(41.2, 48.9), entryOf(100, 118.4)], null), () => undefined);
    const rows = within(screen.getByRole("list", { name: calcHistoryText.listLabel })).getAllByRole(
      "listitem",
    );
    expect(rows[0]?.querySelectorAll(".pokemon-icon")).toHaveLength(2);
    const percent = within(rows[0] as HTMLElement).getByText("41.2〜48.9%");
    expect(percent).toHaveClass("calc-history__percent");
    const bars = rows.map((row) => row.querySelector("[data-testid='history-bar']"));
    for (const bar of bars) {
      expect(bar).toHaveAttribute("aria-hidden", "true");
    }
    expect(rows[0]?.querySelector<HTMLElement>("[data-testid='history-bar-fill']")?.style.width).toBe(
      "48.9%",
    );
    expect(rows[1]?.querySelector<HTMLElement>("[data-testid='history-bar-fill']")?.style.width).toBe("100%");
  });

  test("日付は小さな補足(time)、「使う」はアイコン + 短い文字で読み上げ名は従来のまま", async () => {
    await renderScreen(okFavs([]), okPage([entryOf(10, 12)], null), () => undefined);
    const row = within(screen.getByRole("list", { name: calcHistoryText.listLabel })).getByRole("listitem");
    expect(row.querySelector("time")).toHaveClass("calc-history__time");
    const use = within(row).getByRole("button", { name: calcHistoryText.useLabel });
    expect(use.querySelector("svg")).not.toBeNull();
    expect(use).toHaveTextContent(calcHistoryText.useShort);
    expect(calcHistoryText.useLabel).toContain(calcHistoryText.useShort);
    expect(row).toHaveClass("calc-history__item--pressable");
  });

  test("もっと見るはアイコン付きのボタン", async () => {
    await renderScreen(okFavs([]), okPage([entryOf(10, 12)], "next"));
    const more = screen.getByRole("button", { name: calcHistoryText.moreLabel });
    expect(more.querySelector("svg")).not.toBeNull();
  });
});

describe("空・読み込み中・失敗", () => {
  test("空は大きいアイコン + 短い一言(お気に入り・計算履歴とも 1 文だけ)", async () => {
    await renderScreen(okFavs([]), okPage([], null));
    for (const text of [favoritesScreenText.emptyNotice, calcHistoryText.emptyNotice]) {
      const empty = screen.getByText(text);
      expect(empty).toHaveClass("ui-notice--empty");
      const svg = empty.querySelector("svg");
      expect(svg).not.toBeNull();
      expect(svg?.getAttribute("width")).toBe("48");
      expect(empty.textContent.length).toBeLessThanOrEqual(20);
    }
  });

  test("読み込み中は静止したアイコン + 一言(ui-notice--loading)", async () => {
    await renderScreen("pending", "pending");
    for (const text of [favoritesScreenText.loadingNotice, calcHistoryText.loadingNotice]) {
      const loading = screen.getByText(text);
      expect(loading).toHaveClass("ui-notice--loading");
      expect(loading.querySelector("svg")).not.toBeNull();
    }
  });

  test("計算履歴の失敗は節の中の role=alert(お気に入りの一覧は残る)", async () => {
    const region = await renderScreen(okFavs([FAV]), {
      ok: false,
      error: { code: "internal", message: "テストの失敗" },
    });
    const history = within(region).getByRole("region", { name: calcHistoryText.regionLabel });
    const alert = within(history).getByRole("alert");
    expect(alert).toHaveClass("ui-notice--error");
    expect(within(region).getAllByRole("alert")).toHaveLength(1);
    expect(within(region).getByRole("list", { name: favoritesScreenText.listLabel })).toBeInTheDocument();
  });
});
