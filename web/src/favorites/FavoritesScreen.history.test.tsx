// Web: 計算履歴の一覧(ADR-0338 §1)。お気に入りの画面(FavoritesScreen)の第2の節として履歴を出す配線の確認。
// 節そのものの振る舞いは CalcHistorySection.test.tsx。ここでは、画面の中での置き方と、お気に入り側との独立だけを見る。
//   AC-1' お気に入りの region の中に region「計算履歴」が並ぶ(お気に入りの一覧と履歴が同じ画面)
//   AC-4' 履歴の失敗はお気に入りの一覧を塞がない・お気に入りの失敗は履歴を塞がない(それぞれの alert は自分の節の中)
//   AC-4'' オフライン(recordClient なし)は従来どおり案内だけで、履歴の節も API 呼び出しも無い
//   AC-5' reloadToken が変わると、お気に入りと履歴の両方を取り直す(「この端末のデータを削除」の後は履歴も空になる)
//   AC-6' 履歴の行の「計算に使う」は FavoritesScreen の onUse に calc つきの Favorite 形で渡る(App の既存の経路をそのまま使う)
// 架空の key だけを使う(ADR-0002)。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { calcHistoryText, favoritesScreenText } from "../i18n/favorites";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { FavoritesScreen } from "./FavoritesScreen";

type Schemas = components["schemas"];

const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const HISTORY_ENTRY: Schemas["CalcHistoryEntry"] = {
  occurredAt: "2026-10-09T03:00:00Z",
  calc: {
    format: "single",
    attacker: { speciesKey: "9001-000", level: 50, natureId: "fake-nature", sp: SP },
    defender: { speciesKey: "9002-000", level: 50, natureId: "fake-nature", sp: SP },
    moveId: "fake-move-a",
  },
  result: { minPercent: 41.2, maxPercent: 48.9 },
};
const FAVORITE: Schemas["Favorite"] = {
  id: "21",
  label: "HB特化",
  individual: { speciesKey: "9003-000", level: 50, natureId: "fake-nature", sp: SP },
  createdAt: "2026-10-04T01:00:00Z",
  updatedAt: "2026-10-04T01:00:00Z",
};

interface Fake {
  readonly record: RecordClient;
  readonly listFavorites: ReturnType<typeof vi.fn<RecordClient["listFavorites"]>>;
  readonly listCalcHistory: ReturnType<typeof vi.fn<RecordClient["listCalcHistory"]>>;
}

function fake(
  favorites: RecordResult<Schemas["Favorite"][]>,
  history: RecordResult<Schemas["CalcHistoryPage"]>,
): Fake {
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  const listFavorites = vi.fn<RecordClient["listFavorites"]>(() => Promise.resolve(favorites));
  const listCalcHistory = vi.fn<RecordClient["listCalcHistory"]>(() => Promise.resolve(history));
  return {
    listFavorites,
    listCalcHistory,
    record: {
      listFrequentOpponents: unused,
      deleteDeviceData: unused,
      createFavorite: unused,
      deleteFavorite: unused,
      listFavorites,
      listCalcHistory,
    },
  };
}

const okFav: RecordResult<Schemas["Favorite"][]> = { ok: true, value: [FAVORITE] };
const okHistory: RecordResult<Schemas["CalcHistoryPage"]> = {
  ok: true,
  value: { items: [HISTORY_ENTRY], nextCursor: null },
};

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
  });
}

describe("FavoritesScreen の履歴の節", () => {
  test("お気に入りの region の中に「計算履歴」の region が並び、どちらの一覧も出る", async () => {
    const { record } = fake(okFav, okHistory);
    render(<FavoritesScreen recordClient={record} />);
    await flush();
    const outer = screen.getByRole("region", { name: favoritesScreenText.regionLabel });
    const history = within(outer).getByRole("region", { name: "計算履歴" });
    expect(within(outer).getByRole("list", { name: favoritesScreenText.listLabel })).toBeInTheDocument();
    expect(within(history).getByRole("list", { name: calcHistoryText.listLabel })).toBeInTheDocument();
  });

  test("履歴が 503 でもお気に入りの一覧は出る。alert は履歴の節の中だけ", async () => {
    const { record } = fake(okFav, {
      ok: false,
      error: { code: "store_unavailable", message: "履歴を読めません" },
    });
    render(<FavoritesScreen recordClient={record} />);
    await flush();
    expect(screen.getByRole("list", { name: favoritesScreenText.listLabel })).toBeInTheDocument();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("履歴を読めません");
    expect(within(screen.getByRole("region", { name: "計算履歴" })).getByRole("alert")).toBe(alert);
  });

  test("お気に入りが失敗しても履歴は出る", async () => {
    const { record } = fake(
      { ok: false, error: { code: "store_unavailable", message: "お気に入りを読めません" } },
      okHistory,
    );
    render(<FavoritesScreen recordClient={record} />);
    await flush();
    expect(screen.getByRole("list", { name: calcHistoryText.listLabel })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("お気に入りを読めません");
    expect(within(screen.getByRole("region", { name: "計算履歴" })).queryByRole("alert")).toBeNull();
  });

  test("オフライン(recordClient なし)は案内だけで、履歴の節は出ない", () => {
    render(<FavoritesScreen />);
    expect(screen.getByText(favoritesScreenText.offlineNotice)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "計算履歴" })).toBeNull();
  });

  test("reloadToken が変わると履歴も先頭から取り直す", async () => {
    const { record, listCalcHistory, listFavorites } = fake(okFav, okHistory);
    const view = render(<FavoritesScreen recordClient={record} reloadToken={0} />);
    await flush();
    expect(listCalcHistory).toHaveBeenCalledTimes(1);
    view.rerender(<FavoritesScreen recordClient={record} reloadToken={1} />);
    await flush();
    expect(listFavorites).toHaveBeenCalledTimes(2);
    expect(listCalcHistory).toHaveBeenCalledTimes(2);
    expect(listCalcHistory.mock.calls[1]?.[0]).toBeUndefined();
  });

  test("履歴の行の「計算に使う」は onUse に calc つきの Favorite 形で渡る", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn<(favorite: Schemas["Favorite"]) => void>();
    const { record } = fake(okFav, okHistory);
    render(<FavoritesScreen recordClient={record} onUse={onUse} />);
    await flush();
    const history = screen.getByRole("region", { name: "計算履歴" });
    await user.click(within(history).getByRole("button", { name: calcHistoryText.useLabel }));
    expect(onUse).toHaveBeenCalledTimes(1);
    expect(onUse.mock.calls[0]?.[0].calc).toEqual(HISTORY_ENTRY.calc);
  });
});
