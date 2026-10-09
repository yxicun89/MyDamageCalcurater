// P5-3c(ADR-0327): お気に入りの画面(favorites/FavoritesScreen.tsx)。RecordClient は fake(props で注入)。
// 一覧と削除だけを持つ(追加は計算画面。CalcScreen.favorites.test.tsx)。
// 確かめること(受け入れ条件 AC-2〜AC-5):
//   AC-2 一覧: マウント時に listFavorites を AbortSignal 付きで1回だけ呼ぶ / 読み込み中・空・失敗・一覧の4状態 /
//        region「お気に入り」・list「お気に入り一覧」・件数「n/100件」/ 行の見出しは label(null なら speciesKey)/
//        サーバーの順(更新の新しい順)のまま並べる
//   AC-3 削除: 2段階(window.confirm は使わない)・確定まで deleteFavorite を呼ばない・やめると呼ばない・送信中は二重に呼ばない
//        成功で行が消える。404 not_found は「もう無い」= 成功扱い(行が消え、alert を出さない = 冪等)
//        それ以外の失敗は role=alert に見出し+サーバー message を出し、行は残る
//   AC-4 オンライン限定: recordClient が無い(オフライン)ときは API を呼ばず、role=status で案内を出す(alert にしない)
//   AC-5 取り直し・後始末: reloadToken が変わったら取り直す(古い応答で上書きしない)/
//        アンマウント後に setState しない(console.error なし)・取得は abort する
// 架空の key だけを使う(ADR-0002)。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { favoritesScreenText } from "../i18n/favorites";
import { MAX_FAVORITES_PER_DEVICE, type RecordClient, type RecordResult } from "../record/recordClient";
import { FavoritesScreen } from "./FavoritesScreen";

type Favorite = components["schemas"]["Favorite"];

const SP = { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 } as const;

function favoriteOf(id: string, label: string | null, speciesKey = "9002-000"): Favorite {
  return {
    id,
    label,
    individual: { speciesKey, level: 50, natureId: "fake-nature", sp: SP },
    createdAt: "2026-10-04T01:00:00Z",
    updatedAt: "2026-10-04T01:00:00Z",
  };
}

const FAV_A = favoriteOf("21", "HB特化");
const FAV_B = favoriteOf("20", "CS振り", "9001-000");
const FAV_C = favoriteOf("19", null, "9003-000"); // 名前なし → 見出しは speciesKey

/** 見出しの規則: label があればそれ、null なら speciesKey。 */
function titleOf(favorite: Favorite): string {
  return favorite.label ?? favorite.individual.speciesKey;
}

interface Pending<A, T> {
  readonly args: A;
  resolve(result: RecordResult<T>): void;
}

interface FakeRecord extends RecordClient {
  readonly listCalls: Pending<AbortSignal | undefined, Favorite[]>[];
  readonly deleteCalls: Pending<string, void>[];
}

function createFakeRecord(): FakeRecord {
  const listCalls: FakeRecord["listCalls"] = [];
  const deleteCalls: FakeRecord["deleteCalls"] = [];
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  return {
    listCalls,
    deleteCalls,
    listFrequentOpponents: unused,
    deleteDeviceData: unused,
    createFavorite: unused,
    listCalcHistory: () => new Promise(() => undefined),
    listFavorites(signal) {
      return new Promise((resolve) => {
        listCalls.push({ args: signal, resolve });
      });
    },
    deleteFavorite(favoriteId) {
      return new Promise((resolve) => {
        deleteCalls.push({ args: favoriteId, resolve });
      });
    },
  };
}

async function settle(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

function last<A, T>(calls: readonly Pending<A, T>[], name: string): Pending<A, T> {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}

async function renderLoaded(favorites: Favorite[]) {
  const record = createFakeRecord();
  const view = render(<FavoritesScreen recordClient={record} />);
  await settle(() => {
    last(record.listCalls, "listFavorites").resolve({ ok: true, value: favorites });
  });
  return { record, view };
}

const items = () =>
  within(screen.getByRole("list", { name: favoritesScreenText.listLabel })).getAllByRole("listitem");
const itemOf = (favorite: Favorite) => {
  const found = items().find((item) => item.textContent.includes(titleOf(favorite)));
  if (found === undefined) {
    throw new Error(`${titleOf(favorite)} の行が無い`);
  }
  return found;
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe("AC-2 一覧", () => {
  test("マウント時に listFavorites を AbortSignal 付きで1回だけ呼び、読み込み中を出す", () => {
    const record = createFakeRecord();
    render(<FavoritesScreen recordClient={record} />);
    expect(record.listCalls).toHaveLength(1);
    expect(record.listCalls[0]?.args).toBeInstanceOf(AbortSignal);
    expect(screen.getByRole("region", { name: favoritesScreenText.regionLabel })).toBeInTheDocument();
    expect(screen.getByText(favoritesScreenText.loadingNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
  });

  test("サーバーの順に並べ、見出しは label(null なら speciesKey)。件数は「n/100件」", async () => {
    await renderLoaded([FAV_A, FAV_B, FAV_C]);
    expect(MAX_FAVORITES_PER_DEVICE).toBe(100);
    expect(items().map((item) => item.textContent)).toEqual([
      expect.stringContaining("HB特化"),
      expect.stringContaining("CS振り"),
      expect.stringContaining("9003-000"),
    ]);
    expect(screen.getByText(favoritesScreenText.countLabel(3, MAX_FAVORITES_PER_DEVICE))).toBeInTheDocument();
    expect(screen.queryByText(favoritesScreenText.loadingNotice)).toBeNull();
  });

  test("0件なら空の案内(list を出さない・alert にしない)と 0/100件", async () => {
    await renderLoaded([]);
    expect(screen.getByText(favoritesScreenText.emptyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText(favoritesScreenText.countLabel(0, MAX_FAVORITES_PER_DEVICE))).toBeInTheDocument();
  });

  test("失敗は role=alert に見出し+サーバー message(list も空の案内も出さない)", async () => {
    const record = createFakeRecord();
    render(<FavoritesScreen recordClient={record} />);
    await settle(() => {
      last(record.listCalls, "listFavorites").resolve({
        ok: false,
        error: { code: "store_unavailable", message: "記録を読めません" },
      });
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(favoritesScreenText.listErrorHeading);
    expect(alert).toHaveTextContent("記録を読めません");
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
    expect(screen.queryByText(favoritesScreenText.emptyNotice)).toBeNull();
  });
});

describe("AC-3 削除(2段階。window.confirm は使わない)", () => {
  const deleteButton = (favorite: Favorite) =>
    within(itemOf(favorite)).getByRole("button", {
      name: favoritesScreenText.deleteLabel(titleOf(favorite)),
    });

  test("「削除」を押しただけでは呼ばず、確認と2つの選択肢を出す", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm");
    const { record } = await renderLoaded([FAV_A, FAV_B]);
    await user.click(deleteButton(FAV_A));

    expect(record.deleteCalls).toHaveLength(0);
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(screen.getByText(favoritesScreenText.deleteConfirmNotice(titleOf(FAV_A)))).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)) }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: favoritesScreenText.deleteCancelLabel(titleOf(FAV_A)) }),
    ).toBeInTheDocument();
  });

  test("やめると呼ばず、確認が消えて行は残る", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([FAV_A]);
    await user.click(deleteButton(FAV_A));
    await user.click(
      screen.getByRole("button", { name: favoritesScreenText.deleteCancelLabel(titleOf(FAV_A)) }),
    );
    expect(record.deleteCalls).toHaveLength(0);
    expect(screen.queryByText(favoritesScreenText.deleteConfirmNotice(titleOf(FAV_A)))).toBeNull();
    expect(items()).toHaveLength(1);
  });

  test("確定すると id で1回だけ呼ぶ(送信中の再押下は呼ばない)。成功で行が消え、件数が減り、list は取り直さない", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([FAV_A, FAV_B]);
    await user.click(deleteButton(FAV_A));
    const confirm = screen.getByRole("button", {
      name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)),
    });
    await user.click(confirm);
    await user.click(confirm);
    expect(record.deleteCalls).toHaveLength(1);
    expect(record.deleteCalls[0]?.args).toBe(FAV_A.id);

    await settle(() => {
      last(record.deleteCalls, "deleteFavorite").resolve({ ok: true, value: undefined });
    });
    expect(items()).toHaveLength(1);
    expect(items()[0]?.textContent).toContain(titleOf(FAV_B));
    expect(screen.getByText(favoritesScreenText.countLabel(1, MAX_FAVORITES_PER_DEVICE))).toBeInTheDocument();
    expect(record.listCalls).toHaveLength(1);
  });

  test("最後の1件を消すと空の案内に戻る", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([FAV_A]);
    await user.click(deleteButton(FAV_A));
    await user.click(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)) }),
    );
    await settle(() => {
      last(record.deleteCalls, "deleteFavorite").resolve({ ok: true, value: undefined });
    });
    expect(screen.getByText(favoritesScreenText.emptyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
  });

  test("404 not_found は「もう無い」=成功扱い(行が消え、alert を出さない。冪等)", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([FAV_A, FAV_B]);
    await user.click(deleteButton(FAV_A));
    await user.click(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)) }),
    );
    await settle(() => {
      last(record.deleteCalls, "deleteFavorite").resolve({
        ok: false,
        error: { code: "not_found", message: "お気に入りが見つかりません" },
      });
    });
    expect(items()).toHaveLength(1);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("それ以外の失敗は role=alert に見出し+サーバー message を出し、行は残る(再度削除できる)", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([FAV_A]);
    await user.click(deleteButton(FAV_A));
    await user.click(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)) }),
    );
    await settle(() => {
      last(record.deleteCalls, "deleteFavorite").resolve({
        ok: false,
        error: { code: "store_unavailable", message: "記録を書けません" },
      });
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(favoritesScreenText.deleteErrorHeading);
    expect(alert).toHaveTextContent("記録を書けません");
    expect(items()).toHaveLength(1);
    expect(deleteButton(FAV_A)).toBeEnabled();
  });
});

describe("AC-4 オンライン限定", () => {
  test("recordClient が無い(オフライン)ときは何も呼ばず、role=status の案内を出す(alert にしない)", () => {
    render(<FavoritesScreen recordClient={undefined} />);
    expect(screen.getByRole("region", { name: favoritesScreenText.regionLabel })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(favoritesScreenText.offlineNotice);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
  });

  test("オンライン → オフラインに変わったら API を呼び直さず案内に切り替わる", async () => {
    const record = createFakeRecord();
    const view = render(<FavoritesScreen recordClient={record} />);
    await settle(() => {
      last(record.listCalls, "listFavorites").resolve({ ok: true, value: [FAV_A] });
    });
    view.rerender(<FavoritesScreen recordClient={undefined} />);
    expect(screen.getByRole("status")).toHaveTextContent(favoritesScreenText.offlineNotice);
    expect(screen.queryByRole("list", { name: favoritesScreenText.listLabel })).toBeNull();
    expect(record.listCalls).toHaveLength(1);
  });
});

describe("AC-5 取り直し・後始末", () => {
  test("reloadToken が変わると取り直し(読み込み中に戻し、古い一覧を出し続けない)。同じ値の再描画では呼ばない", async () => {
    const record = createFakeRecord();
    const view = render(<FavoritesScreen recordClient={record} reloadToken={0} />);
    await settle(() => {
      last(record.listCalls, "listFavorites").resolve({ ok: true, value: [FAV_A] });
    });
    view.rerender(<FavoritesScreen recordClient={record} reloadToken={0} />);
    expect(record.listCalls).toHaveLength(1);

    view.rerender(<FavoritesScreen recordClient={record} reloadToken={1} />);
    expect(record.listCalls).toHaveLength(2);
    expect(screen.queryByText("HB特化")).toBeNull();
    expect(screen.getByText(favoritesScreenText.loadingNotice)).toBeInTheDocument();
    await settle(() => {
      last(record.listCalls, "listFavorites").resolve({ ok: true, value: [] });
    });
    expect(screen.getByText(favoritesScreenText.emptyNotice)).toBeInTheDocument();
  });

  test("古い一覧の応答が後から届いても、最新の応答を上書きしない", async () => {
    const record = createFakeRecord();
    const view = render(<FavoritesScreen recordClient={record} reloadToken={0} />);
    view.rerender(<FavoritesScreen recordClient={record} reloadToken={1} />);
    await settle(() => {
      record.listCalls[1]?.resolve({ ok: true, value: [FAV_B] });
    });
    await settle(() => {
      record.listCalls[0]?.resolve({ ok: true, value: [FAV_A] });
    });
    expect(items()).toHaveLength(1);
    expect(items()[0]?.textContent).toContain("CS振り");
  });

  test("アンマウントで取得を abort し、後から届いた応答で setState しない(console.error なし)", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const record = createFakeRecord();
    const view = render(<FavoritesScreen recordClient={record} />);
    const signal = record.listCalls[0]?.args;
    view.unmount();
    expect(signal?.aborted).toBe(true);
    await settle(() => {
      record.listCalls[0]?.resolve({ ok: true, value: [FAV_A] });
    });
    expect(errorSpy).not.toHaveBeenCalled();
  });

  test("削除の応答待ちでアンマウントしても、後から届いた応答で setState しない(console.error なし)", async () => {
    const user = userEvent.setup();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { record, view } = await renderLoaded([FAV_A]);
    await user.click(
      within(itemOf(FAV_A)).getByRole("button", { name: favoritesScreenText.deleteLabel(titleOf(FAV_A)) }),
    );
    await user.click(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(titleOf(FAV_A)) }),
    );
    view.unmount();
    await settle(() => {
      last(record.deleteCalls, "deleteFavorite").resolve({ ok: true, value: undefined });
    });
    expect(errorSpy).not.toHaveBeenCalled();
  });
});
