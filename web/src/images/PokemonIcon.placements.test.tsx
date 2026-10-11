// G-06(ADR-0343): ポケモンを見せる各所の画像。manifest にキーがあれば <img>(thumb・lazy・alt 空)、
// 無ければタイプ色のエンブレムのまま成立する(AC-X)。対象: 種族検索の候補・お気に入りの行・計算履歴の行・
// 逆算のカード・構築の編集枠・タイプバランスの枠。(計算・素早さ・構築の一覧は P8-1c で済み。判定は非表示、調整は対象外)

import type { ComponentProps } from "react";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { FavoritesScreen } from "../favorites/FavoritesScreen";
import { CalcHistorySection } from "../favorites/CalcHistorySection";
import { teamMemberText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData } from "../master/types";
import type { RecordClient } from "../record/recordClient";
import { BalanceScreen } from "../screens/BalanceScreen";
import { ReverseScreen } from "../screens/ReverseScreen";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import { createFakeEngine } from "../test/fakeEngine";
import { createFakeTeamClient, flush, lastCall } from "../test/fakeTeamClient";
import { createFakeSpeciesSearch } from "../test/onlineMaster";
import { TeamScreen } from "../team/TeamScreen";
import { PokemonImagesProvider } from "./PokemonImagesContext";
import { parsePokemonImageManifest } from "./pokemonImages";

type Schemas = components["schemas"];

let master: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
});
afterEach(() => {
  vi.useRealTimers();
});

function manifestOf(keys: readonly string[]) {
  return parsePokemonImageManifest({
    version: 1,
    images: Object.fromEntries(
      keys.map((key) => [
        key,
        { thumb: `thumb/${key}.aaaaaaaa.webp`, detail: `detail/${key}.bbbbbbbb.webp` },
      ]),
    ),
  });
}

const srcOf = (root: HTMLElement) => root.querySelector("img")?.getAttribute("src") ?? null;
const emblemsIn = (root: HTMLElement) => root.querySelectorAll("[data-testid='pokemon-icon-emblem']");

describe("種族検索の候補(SpeciesSearchField)", () => {
  async function openOptions(keys: readonly string[]) {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const search = createFakeSpeciesSearch({ species: master.species, abilities: master.abilities });
    const first = master.species[0];
    if (first === undefined) {
      throw new Error("例データに種族が無い");
    }
    render(
      <PokemonImagesProvider manifest={manifestOf(keys)}>
        <SpeciesSearchField label="攻撃側のポケモン" masterSearch={search} onResolved={() => undefined} />
      </PokemonImagesProvider>,
    );
    await user.type(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), first.nameJa.slice(0, 2));
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await screen.findAllByRole("option");
    return { user, search };
  }

  test("manifest にある候補は <img>、無い候補はエンブレム。名前は文字のまま・候補の accessible name は名前だけ", async () => {
    const first = master.species[0];
    if (first === undefined) {
      throw new Error("例データに種族が無い");
    }
    await openOptions([first.key]);
    const options = screen.getAllByRole("option");
    const withImage = options.find((option) => option.textContent === first.nameJa);
    expect(withImage).toBeDefined();
    if (withImage === undefined) {
      return;
    }
    expect(srcOf(withImage)).toBe(`/images/thumb/${first.key}.aaaaaaaa.webp`);
    expect(withImage.querySelector("img")?.getAttribute("alt")).toBe("");
    expect(emblemsIn(withImage)).toHaveLength(0);
    expect(within(withImage).queryByRole("img")).toBeNull();
    for (const option of options) {
      if (option !== withImage) {
        expect(option.querySelector("img")).toBeNull();
        expect(emblemsIn(option)).toHaveLength(1);
      }
    }
  });

  test("manifest が無くてもエンブレムで成立し、キーボード操作(↓ で次の候補・Enter で確定)は変わらない", async () => {
    const { user, search } = await openOptions([]);
    const options = screen.getAllByRole("option");
    expect(options.every((option) => option.querySelector("img") === null)).toBe(true);
    expect(options.every((option) => emblemsIn(option).length === 1)).toBe(true);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{ArrowDown}");
    expect(screen.getAllByRole("option")[1]).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{Enter}");
    expect(search.resolvedKeys).toHaveLength(1);
  });
});

describe("お気に入り・計算履歴の行", () => {
  const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;
  const individual = (speciesKey: string) => ({ speciesKey, level: 50, natureId: "n", sp: SP });

  test("お気に入りの行: individual.speciesKey の画像(無ければエンブレム)。見出しの文字は変わらない", async () => {
    const favorites: Schemas["Favorite"][] = [
      {
        id: "1",
        label: "HB特化",
        individual: individual("9001-000"),
        createdAt: "2026-10-04T01:00:00Z",
        updatedAt: "2026-10-04T01:00:00Z",
      },
      {
        id: "2",
        label: null,
        individual: individual("9002-000"),
        createdAt: "2026-10-04T01:00:00Z",
        updatedAt: "2026-10-04T01:00:00Z",
      },
    ];
    const client = {
      listFavorites: () => Promise.resolve({ ok: true, value: favorites }),
      listCalcHistory: () => new Promise(() => undefined),
    } as unknown as RecordClient;
    render(
      <PokemonImagesProvider manifest={manifestOf(["9001-000"])}>
        <FavoritesScreen recordClient={client} reloadToken={0} />
      </PokemonImagesProvider>,
    );
    const rows = await screen.findAllByRole("listitem");
    expect(srcOf(rows[0] as HTMLElement)).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
    expect(emblemsIn(rows[0] as HTMLElement)).toHaveLength(0);
    expect(rows[1]?.querySelector("img")).toBeNull();
    expect(emblemsIn(rows[1] as HTMLElement)).toHaveLength(1);
    expect(within(rows[0] as HTMLElement).getByText("HB特化")).toBeInTheDocument();
  });

  test("計算履歴の行: 攻撃側と防御側の2つ(画像かエンブレム)", async () => {
    const entry: Schemas["CalcHistoryEntry"] = {
      occurredAt: "2026-10-09T03:00:00Z",
      calc: {
        format: "single",
        attacker: individual("9001-000"),
        defender: individual("9002-000"),
        moveId: "fake-move-a",
      },
      result: { minPercent: 41.2, maxPercent: 48.9 },
    };
    const client = {
      listCalcHistory: () => Promise.resolve({ ok: true, value: { items: [entry], nextCursor: null } }),
    } as unknown as RecordClient;
    render(
      <PokemonImagesProvider manifest={manifestOf(["9001-000"])}>
        <CalcHistorySection recordClient={client} reloadToken={0} />
      </PokemonImagesProvider>,
    );
    const row = (await screen.findAllByRole("listitem"))[0] as HTMLElement;
    expect(row.querySelectorAll("img")).toHaveLength(1);
    expect(srcOf(row)).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
    expect(emblemsIn(row)).toHaveLength(1);
  });
});

describe("逆算・構築の編集枠・タイプバランスの枠", () => {
  test("逆算: 種族を選ぶとそのカードに画像(無い側はエンブレム)", async () => {
    const [mine, theirs] = master.species;
    if (mine === undefined || theirs === undefined) {
      throw new Error("例データに種族が足りない");
    }
    const user = userEvent.setup();
    render(
      <PokemonImagesProvider manifest={manifestOf([mine.key])}>
        <ReverseScreen engine={createFakeEngine()} master={master} />
      </PokemonImagesProvider>,
    );
    await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), mine.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), theirs.key);
    const mineCard = screen.getByRole("region", { name: "自分のポケモン" });
    const theirCard = screen.getByRole("region", { name: "相手のポケモン" });
    expect(srcOf(mineCard)).toBe(`/images/thumb/${mine.key}.aaaaaaaa.webp`);
    expect(theirCard.querySelector("img")).toBeNull();
    expect(emblemsIn(theirCard)).toHaveLength(1);
  });

  test("タイプバランス: 種族を選んだ枠に画像かエンブレム(選ぶ前は出さない)", async () => {
    const first = master.species[0];
    if (first === undefined) {
      throw new Error("例データに種族が無い");
    }
    const user = userEvent.setup();
    const pending = () => new Promise<never>(() => undefined);
    const client = {
      analyze: pending,
      coverage: pending,
      threats: pending,
      recommendations: pending,
    } as unknown as ComponentProps<typeof BalanceScreen>["client"];
    render(
      <PokemonImagesProvider manifest={manifestOf([first.key])}>
        <BalanceScreen master={master} client={client} />
      </PokemonImagesProvider>,
    );
    const group = screen.getByRole("group", { name: "メンバー1" });
    expect(group.querySelector("img")).toBeNull();
    expect(emblemsIn(group)).toHaveLength(0);
    await user.selectOptions(within(group).getByRole("combobox", { name: "ポケモン" }), first.key);
    expect(srcOf(group)).toBe(`/images/thumb/${first.key}.aaaaaaaa.webp`);
  });

  test("構築の編集枠: 種族の決まった枠に画像かエンブレム", async () => {
    const client = createFakeTeamClient();
    const user = userEvent.setup();
    const member: Schemas["TeamMember"] = {
      speciesKey: "9001-000",
      moveIds: [],
      itemId: null,
      abilityId: "exampleabilitynone",
      natureId: "example-nature-atk",
      sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      teraType: null,
    };
    const team: Schemas["Team"] = {
      id: "33333333-3333-4333-8333-333333333333",
      name: "テスト構築A",
      members: [member],
      createdAt: "2026-09-26T12:00:00Z",
      updatedAt: "2026-09-26T12:00:00Z",
    };
    render(
      <PokemonImagesProvider manifest={manifestOf(["9001-000"])}>
        <TeamScreen teamClient={client} master={master} />
      </PokemonImagesProvider>,
    );
    await flush(() => {
      lastCall(client.listCalls, "list").resolve({ ok: true, value: [team] });
    });
    await user.click(screen.getByRole("button", { name: teamMemberText.editLabel("テスト構築A") }));
    const editor = screen.getByRole("region", { name: teamMemberText.editorLabel("テスト構築A") });
    const slot = within(editor).getByRole("group", { name: teamMemberText.memberLegend(1) });
    expect(srcOf(slot)).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
  });
});
