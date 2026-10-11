// I-web-8 = F-09(ADR-0333 §2): お気に入りの一覧から「計算に使う」。
// 確かめること:
//   - onUse を渡すと、各行に主ボタン「「{見出し}」を計算に使う」を出す(見える文字 = アクセシブルネーム。SC 2.5.3)
//   - 押すと onUse にそのお気に入り(Favorite そのもの。calc を含む)を1回渡す。一覧は取り直さない・削除しない
//   - calc の無い旧お気に入りにもボタンを出し、行に「攻撃側だけ」の案内を添える(calc のある行には出さない)
//   - onUse を渡さない(従来の呼び出し)ときはボタンを出さない
//   - 削除の確認中でも、他の行の「計算に使う」は押せる
// 架空の key だけを使う(ADR-0002)。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { favoritesScreenText } from "../i18n/favorites";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { FavoritesScreen } from "./FavoritesScreen";

type Favorite = components["schemas"]["Favorite"];

const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;

const WITH_CALC: Favorite = {
  id: "31",
  label: "テストほのお→テストみず(テストかえんパンチ)",
  individual: { speciesKey: "9001-000", level: 50, natureId: "example-nature-atk", sp: SP },
  calc: {
    format: "single",
    attacker: { speciesKey: "9001-000", level: 50, natureId: "example-nature-atk", sp: SP },
    defender: { speciesKey: "9002-000", level: 50, natureId: "example-nature-neutral-docile", sp: ZERO },
    moveId: "examplemovefirepunch",
  },
  createdAt: "2026-10-04T01:00:00Z",
  updatedAt: "2026-10-04T01:00:00Z",
};

const LEGACY: Favorite = {
  id: "30",
  label: "テストみず",
  individual: { speciesKey: "9002-000", level: 50, natureId: "example-nature-neutral-docile", sp: ZERO },
  createdAt: "2026-10-03T01:00:00Z",
  updatedAt: "2026-10-03T01:00:00Z",
};

function fakeRecord(favorites: Favorite[]) {
  const deleteFavorite = vi.fn<RecordClient["deleteFavorite"]>(() => new Promise(() => undefined));
  let listCount = 0;
  const record: RecordClient = {
    listFrequentOpponents: () => Promise.reject(new Error("unused")),
    deleteDeviceData: () => Promise.reject(new Error("unused")),
    createFavorite: () => Promise.reject(new Error("unused")),
    listFavorites: () => {
      listCount += 1;
      return Promise.resolve<RecordResult<Favorite[]>>({ ok: true, value: favorites });
    },
    deleteFavorite,
    listCalcHistory: () => new Promise(() => undefined),
  };
  return { record, deleteFavorite, listCount: () => listCount };
}

async function renderLoaded(favorites: Favorite[], onUse?: (favorite: Favorite) => void) {
  const fake = fakeRecord(favorites);
  await act(async () => {
    render(<FavoritesScreen recordClient={fake.record} onUse={onUse} />);
    await Promise.resolve();
  });
  await screen.findByRole("list", { name: favoritesScreenText.listLabel });
  return fake;
}

function rowOf(title: string): HTMLElement {
  const row = within(screen.getByRole("list", { name: favoritesScreenText.listLabel }))
    .getAllByRole("listitem")
    .find((item) => within(item).queryByText(title) !== null);
  if (row === undefined) {
    throw new Error(`${title} の行が無い`);
  }
  return row;
}

describe("「計算に使う」ボタン", () => {
  test("文言: 「{見出し}」を計算に使う", () => {
    expect(favoritesScreenText.useLabel("HB特化")).toBe("「HB特化」を計算に使う");
  });

  test("onUse を渡すと各行にボタンを出す(見える文字「使う」は名前に含まれる。WCAG 2.5.3。ADR-0348)", async () => {
    await renderLoaded([WITH_CALC, LEGACY], () => undefined);
    for (const favorite of [WITH_CALC, LEGACY]) {
      const title = favorite.label ?? favorite.individual.speciesKey;
      const button = within(rowOf(title)).getByRole("button", { name: favoritesScreenText.useLabel(title) });
      expect(button).toHaveTextContent(favoritesScreenText.useShort);
      expect(favoritesScreenText.useLabel(title)).toContain(favoritesScreenText.useShort);
    }
  });

  test("押すと onUse にそのお気に入り(calc を含む)を1回渡す。一覧の取り直し・削除はしない", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn<(favorite: Favorite) => void>();
    const fake = await renderLoaded([WITH_CALC, LEGACY], onUse);
    const title = WITH_CALC.label ?? "";
    await user.click(screen.getByRole("button", { name: favoritesScreenText.useLabel(title) }));
    expect(onUse).toHaveBeenCalledTimes(1);
    expect(onUse).toHaveBeenCalledWith(WITH_CALC);
    expect(fake.listCount()).toBe(1);
    expect(fake.deleteFavorite).not.toHaveBeenCalled();
  });

  test("calc の無い旧お気に入りもボタンを出し、押すとそのまま渡す。行には「攻撃側だけ」の案内を添える", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn<(favorite: Favorite) => void>();
    await renderLoaded([WITH_CALC, LEGACY], onUse);
    const legacyTitle = LEGACY.label ?? "";
    expect(rowOf(legacyTitle)).toHaveTextContent(favoritesScreenText.attackerOnlyHint);
    expect(rowOf(WITH_CALC.label ?? "")).not.toHaveTextContent(favoritesScreenText.attackerOnlyHint);
    await user.click(screen.getByRole("button", { name: favoritesScreenText.useLabel(legacyTitle) }));
    expect(onUse).toHaveBeenCalledWith(LEGACY);
  });

  test("onUse を渡さない(従来の呼び出し)ときはボタンを出さない", async () => {
    await renderLoaded([WITH_CALC]);
    expect(
      screen.queryByRole("button", { name: favoritesScreenText.useLabel(WITH_CALC.label ?? "") }),
    ).toBeNull();
  });

  test("ある行の削除を確認中でも、他の行の「計算に使う」は押せる", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn<(favorite: Favorite) => void>();
    await renderLoaded([WITH_CALC, LEGACY], onUse);
    const legacyTitle = LEGACY.label ?? "";
    await user.click(screen.getByRole("button", { name: favoritesScreenText.deleteLabel(legacyTitle) }));
    const useButton = screen.getByRole("button", {
      name: favoritesScreenText.useLabel(WITH_CALC.label ?? ""),
    });
    expect(useButton).toBeEnabled();
    await user.click(useButton);
    expect(onUse).toHaveBeenCalledWith(WITH_CALC);
  });
});
