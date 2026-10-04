// F-12 PR-2(I-web-12、ADR-0336 §2): お気に入り画面(F-09 の「計算に使う」・件数・削除確認・復元案内を含む)への共通部品の適用。
// 一覧の各行は ui-card、主ボタン「計算に使う」は ui-button--primary、削除は ui-button--secondary、削除確認の「削除する」は
// ui-button--danger、件数は ui-badge、読み込み中・空・オフライン・エラーは ui-notice。
// 「計算に使う」で計算タブへ移ったときの復元案内(CalcScreen の calc-screen__restore)も ui-notice にそろえる。
// 既存のクラス(favorites-screen__*)・アクセシブルな名前・role・テキストは変えない。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { favoritesRestoreText, favoritesScreenText } from "../i18n/favorites";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { createFakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "../screens/CalcScreen";
import { FavoritesScreen } from "./FavoritesScreen";

type Favorite = components["schemas"]["Favorite"];

const SP = { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 } as const;

function favoriteOf(id: string, label: string | null): Favorite {
  return {
    id,
    label,
    individual: { speciesKey: "9004-000", level: 50, natureId: "example-nature-spa", sp: SP },
    createdAt: "2026-10-04T01:00:00Z",
    updatedAt: "2026-10-04T01:00:00Z",
  };
}

type ListResult = RecordResult<Favorite[]>;

function createRecord(list: ListResult | "pending"): RecordClient {
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  return {
    listFrequentOpponents: unused,
    deleteDeviceData: unused,
    createFavorite: unused,
    listFavorites: () => (list === "pending" ? new Promise(() => undefined) : Promise.resolve(list)),
    deleteFavorite: () => new Promise(() => undefined),
  };
}

const region = () => screen.getByRole("region", { name: favoritesScreenText.regionLabel });

async function renderLoaded(favorites: Favorite[], onUse = () => undefined): Promise<void> {
  render(<FavoritesScreen recordClient={createRecord({ ok: true, value: favorites })} onUse={onUse} />);
  await screen.findByRole("list", { name: favoritesScreenText.listLabel });
}

describe("一覧", () => {
  test("各行は ui-card(既存の favorites-screen__item も残す)", async () => {
    await renderLoaded([favoriteOf("2", "HB特化"), favoriteOf("1", "CS振り")]);
    const rows = within(screen.getByRole("list", { name: favoritesScreenText.listLabel })).getAllByRole(
      "listitem",
    );
    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect(row).toHaveClass("ui-card", "favorites-screen__item");
    }
  });

  test("件数(n/100件)は ui-badge(既存の favorites-screen__count も残す)", async () => {
    await renderLoaded([favoriteOf("2", "HB特化")]);
    expect(within(region()).getByText(/^1\/\d+件$/)).toHaveClass("ui-badge", "favorites-screen__count");
  });

  test("「計算に使う」は ui-button--primary、削除は ui-button--secondary", async () => {
    await renderLoaded([favoriteOf("2", "HB特化")]);
    expect(screen.getByRole("button", { name: favoritesScreenText.useLabel("HB特化") })).toHaveClass(
      "ui-button",
      "ui-button--primary",
    );
    expect(screen.getByRole("button", { name: favoritesScreenText.deleteLabel("HB特化") })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
  });

  test("削除確認の「削除する」は ui-button--danger、「やめる」は ui-button--secondary、確認文は ui-notice", async () => {
    const user = userEvent.setup();
    await renderLoaded([favoriteOf("2", "HB特化")]);
    await user.click(screen.getByRole("button", { name: favoritesScreenText.deleteLabel("HB特化") }));
    expect(
      screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel("HB特化") }),
    ).toHaveClass("ui-button", "ui-button--danger");
    expect(screen.getByRole("button", { name: favoritesScreenText.deleteCancelLabel("HB特化") })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
    expect(screen.getByText(favoritesScreenText.deleteConfirmNotice("HB特化"))).toHaveClass("ui-notice");
  });

  test("calc の無い旧お気に入りの案内(attackerOnlyHint)は ui-badge", async () => {
    await renderLoaded([favoriteOf("2", "HB特化")]);
    expect(screen.getByText(favoritesScreenText.attackerOnlyHint)).toHaveClass("ui-badge");
  });
});

describe("案内", () => {
  test("読み込み中は ui-notice ui-notice--loading", () => {
    render(<FavoritesScreen recordClient={createRecord("pending")} />);
    expect(screen.getByText(favoritesScreenText.loadingNotice)).toHaveClass(
      "ui-notice",
      "ui-notice--loading",
    );
  });

  test("空の案内は ui-notice ui-notice--empty", async () => {
    render(<FavoritesScreen recordClient={createRecord({ ok: true, value: [] })} />);
    expect(await screen.findByText(favoritesScreenText.emptyNotice)).toHaveClass(
      "ui-notice",
      "ui-notice--empty",
    );
  });

  test("オフラインの案内は role=status のまま ui-notice ui-notice--info", () => {
    render(<FavoritesScreen />);
    expect(screen.getByRole("status")).toHaveClass("ui-notice", "ui-notice--info");
  });

  test("一覧の取得失敗は role=alert のまま ui-notice ui-notice--error", async () => {
    render(
      <FavoritesScreen
        recordClient={createRecord({ ok: false, error: { code: "internal", message: "テストの失敗" } })}
      />,
    );
    expect(await screen.findByRole("alert")).toHaveClass("ui-notice", "ui-notice--error");
  });
});

describe("復元案内(計算画面)", () => {
  test("お気に入りを開いた旨(role=status)は ui-notice ui-notice--info", async () => {
    const master: MasterData = await exampleMasterSource.load();
    const favorite = favoriteOf("41", "テストの構成");
    render(
      <CalcScreen engine={createFakeEngine()} master={master} restoreRequest={{ token: 1, favorite }} />,
    );
    const text = await screen.findByText(favoritesRestoreText.attackerOnlyNotice("テストの構成"));
    expect(text.closest("[role=status]")).toHaveClass("ui-notice", "ui-notice--info");
  });
});
