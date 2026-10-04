// F-12(I-web-6、ADR-0331 §3): 素早さ画面への共通部品の適用。
// 2つの領域はカード(ui-card)、絞り込み・入力方法の選択はチップ(ui-chip。選択中は ui-chip--selected)、
// 速い順の一覧は表のような行(ui-rows)、読み込み中・失敗は案内(ui-notice)。
// アクセシブルな名前・役割(region・group・radiogroup・alert)と既存のクラスは変えない。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { speedScreenText } from "../i18n/ja";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient, SpeedResult } from "./speedClient";

type Schemas = components["schemas"];

/** 応答をテストから流せる最小の fake(呼び出しの記録は持たない)。 */
function createClient(): {
  client: SpeedClient;
  resolveTable: (result: SpeedResult<Schemas["TableResponse"]>) => Promise<void>;
  resolvePokemon: (result: SpeedResult<Schemas["PokemonListResponse"]>) => Promise<void>;
} {
  const tableResolvers: ((result: SpeedResult<Schemas["TableResponse"]>) => void)[] = [];
  const pokemonResolvers: ((result: SpeedResult<Schemas["PokemonListResponse"]>) => void)[] = [];
  const client: SpeedClient = {
    pokemon: () => new Promise((resolve) => pokemonResolvers.push(resolve)),
    table: () => new Promise((resolve) => tableResolvers.push(resolve)),
    position: () => new Promise(() => undefined),
  };
  const flushAll = async <T,>(resolvers: ((result: T) => void)[], result: T): Promise<void> => {
    await act(async () => {
      for (const resolve of resolvers.splice(0)) {
        resolve(result);
      }
      await Promise.resolve();
    });
  };
  return {
    client,
    resolveTable: (result) => flushAll(tableResolvers, result),
    resolvePokemon: (result) => flushAll(pokemonResolvers, result),
  };
}

const FAKE: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire"],
  baseSpeed: 100,
};

const tableResponse: Schemas["TableResponse"] = {
  regulationId: "example",
  presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
  tiers: [
    { speed: 200, entries: [{ ...FAKE, preset: "max" }] },
    { speed: 120, entries: [{ ...FAKE, preset: "uninvested" }] },
  ],
};

const tableRegion = () => screen.getByRole("region", { name: speedScreenText.tableRegionLabel });
const selfRegion = () => screen.getByRole("region", { name: speedScreenText.selfRegionLabel });

describe("素早さ画面の見た目の部品", () => {
  test("2つの領域は ui-card(既存の speed-table / speed-self も残す)", () => {
    render(<SpeedScreen speedClient={createClient().client} />);
    expect(tableRegion()).toHaveClass("ui-card", "speed-table");
    expect(selfRegion()).toHaveClass("ui-card", "speed-self");
  });

  test("絞り込みの選択肢は ui-chip。チェック中は ui-chip--selected、外すと消える", async () => {
    const user = userEvent.setup();
    render(<SpeedScreen speedClient={createClient().client} />);
    const filter = within(tableRegion()).getByRole("group", { name: speedScreenText.filterGroupLabel });
    const checkboxes = within(filter).getAllByRole("checkbox");
    for (const checkbox of checkboxes) {
      const option = checkbox.closest("label");
      expect(option).toHaveClass("ui-chip", "speed-table__filter-option");
      expect(option?.classList.contains("ui-chip--selected")).toBe((checkbox as HTMLInputElement).checked);
    }
    const first = checkboxes[0];
    if (first === undefined) {
      throw new Error("絞り込みの選択肢が無い");
    }
    await user.click(first);
    expect(first.closest("label")?.classList.contains("ui-chip--selected")).toBe(
      (first as HTMLInputElement).checked,
    );
  });

  test("入力の方法(ラジオ)は ui-chip。選択中だけ ui-chip--selected", () => {
    render(<SpeedScreen speedClient={createClient().client} />);
    const group = within(selfRegion()).getByRole("radiogroup", { name: speedScreenText.modeGroupLabel });
    for (const radio of within(group).getAllByRole("radio")) {
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
  });

  test("読み込み中の案内は ui-notice ui-notice--loading", () => {
    render(<SpeedScreen speedClient={createClient().client} />);
    const loading = within(tableRegion()).getByText(speedScreenText.loadingNotice);
    expect(loading).toHaveClass("ui-notice", "ui-notice--loading");
  });

  test("表の取得に失敗したら role=alert のまま ui-notice ui-notice--error", async () => {
    const { client, resolveTable } = createClient();
    render(<SpeedScreen speedClient={client} />);
    await resolveTable({ ok: false, error: { code: "unavailable", message: "テストの失敗" } });
    const alert = within(tableRegion()).getByRole("alert");
    expect(alert).toHaveClass("ui-notice", "ui-notice--error");
  });

  test("速い順の一覧は ui-rows(既存の speed-table__tiers も残す)", async () => {
    const { client, resolveTable } = createClient();
    render(<SpeedScreen speedClient={client} />);
    await resolveTable({ ok: true, value: tableResponse });
    const tiers = tableRegion().querySelector(".speed-table__tiers");
    expect(tiers).not.toBeNull();
    expect(tiers).toHaveClass("ui-rows");
  });
});
