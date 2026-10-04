// I-web-9 = F-02(ADR-0335): 計算画面の技の選択肢の並び(習得順・五十音順・タイプ順)。
// 確かめること:
//   S-1 既定は習得順(従来の並び)。「技の並び」のチップ群(role=radiogroup)に3つあり、習得順が選ばれている
//   S-2 五十音順・タイプ順に切り替えると選択肢が並び替わる(タイプ順は optgroup のタイプ名見出し)。表示は「技名・分類・威力」のまま
//   S-3 選択中の技は並びを変えても保たれる。既定の自動選択(最初のダメージ技)は並びに依らない
//   S-4 並びを変えても計算の要求・結果は変わらない(要求を増やさない)
//   S-5 並びは localStorage(pokecalc.moveSort)に覚え、次に開いたとき(別のマウント)も同じ。不正値は既定。使えなくても動く
//   S-6 技を戻せなかったときの未選択の選択肢(F-09)は、どの並びでも先頭で、disabled のまま
//   S-7 オンライン(検索で解決した learnset)でも同じ並びになる
// 架空のデータだけ(test/moveSortMaster.ts)。engine は fake。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { MOVE_SORT_STORAGE_KEY } from "../app/moveSortStorage";
import type { FavoriteRestoreRequest } from "../favorites/favoriteCalc";
import { favoritesRestoreText } from "../i18n/favorites";
import { moveSortText } from "../i18n/moveSort";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import {
  KANA_IDS,
  LEARNSET_IDS,
  SORT_MOVES,
  SORT_SPECIES_KEY,
  TYPE_GROUPS,
  TYPE_IDS,
  sortSpecies,
  withSortFixture,
} from "../test/moveSortMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

type Schemas = components["schemas"];

let master: MasterData;
beforeAll(async () => {
  master = withSortFixture(await exampleMasterSource.load());
});
beforeEach(() => {
  localStorage.clear();
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  localStorage.clear();
});

const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const sortGroup = () => screen.getByRole("radiogroup", { name: moveSortText.groupLabel });

/** 選択肢の ID(値が空の未選択の選択肢は除く)。 */
function optionIds(): string[] {
  return within(moveSelect())
    .getAllByRole("option")
    .map((option) => (option as HTMLOptionElement).value)
    .filter((value) => value !== "");
}

async function chooseSort(user: ReturnType<typeof userEvent.setup>, label: string): Promise<void> {
  await user.click(within(sortGroup()).getByRole("radio", { name: label }));
}

async function renderWithSortSpecies(user: ReturnType<typeof userEvent.setup>) {
  const engine = createFakeEngine();
  const view = render(<CalcScreen engine={engine} master={master} />);
  await user.selectOptions(attackerSelect(), SORT_SPECIES_KEY);
  return { engine, ...view };
}

describe("S-1 チップ群と既定", () => {
  test("「技の並び」の radiogroup に 習得順・五十音順・タイプ順 があり、既定は習得順で従来の並び", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    const radios = within(sortGroup()).getAllByRole("radio");
    expect(radios.map((r) => r.getAttribute("aria-label") ?? r.closest("label")?.textContent.trim())).toEqual(
      [moveSortText.options.learnset, moveSortText.options.kana, moveSortText.options.type],
    );
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.learnset })).toBeChecked();
    expect(optionIds()).toEqual(LEARNSET_IDS);
    // 既定の技は learnset の最初のダメージ技(並びを足しても変えない)。
    expect(moveSelect()).toHaveValue(LEARNSET_IDS[0]);
  });

  test("チップは F-12 の部品(ui-chip)で、文言は i18n の語(習得順・五十音順・タイプ順)", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    expect(moveSortText.groupLabel).toBe("技の並び");
    expect(moveSortText.options).toEqual({ learnset: "習得順", kana: "五十音順", type: "タイプ順" });
    const labels = within(sortGroup())
      .getAllByRole("radio")
      .map((r) => r.closest("label"));
    for (const label of labels) {
      expect(label).toHaveClass("ui-chip");
    }
  });
});

describe("S-2 並びの切り替え", () => {
  test("五十音順にすると選択肢が五十音順(濁点・カタカナ・長音を含む)に並ぶ", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    await chooseSort(user, moveSortText.options.kana);
    expect(optionIds()).toEqual(KANA_IDS);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.kana })).toBeChecked();
  });

  test("タイプ順にすると、タイプ名の見出し(optgroup)ごとに、相性表の並びで・群の中は五十音順に並ぶ", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    await chooseSort(user, moveSortText.options.type);
    expect(optionIds()).toEqual(TYPE_IDS);
    const groups = within(moveSelect()).getAllByRole("group");
    expect(groups.map((g) => g.getAttribute("label"))).toEqual(TYPE_GROUPS.map((g) => g.label));
    groups.forEach((group, index) => {
      const ids = within(group)
        .getAllByRole("option")
        .map((o) => (o as HTMLOptionElement).value);
      expect(ids).toEqual(TYPE_GROUPS[index]?.ids);
    });
  });

  test("習得順・五十音順では optgroup を出さない。習得順に戻すと元の並び", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    await chooseSort(user, moveSortText.options.type);
    await chooseSort(user, moveSortText.options.learnset);
    expect(optionIds()).toEqual(LEARNSET_IDS);
    expect(within(moveSelect()).queryAllByRole("group")).toHaveLength(0);
    await chooseSort(user, moveSortText.options.kana);
    expect(within(moveSelect()).queryAllByRole("group")).toHaveLength(0);
  });

  test("選択肢の表示は どの並びでも「技名・分類・威力」(従来の書式)", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    for (const label of Object.values(moveSortText.options)) {
      await chooseSort(user, label);
      for (const move of SORT_MOVES) {
        const option = within(moveSelect()).getByRole("option", { name: new RegExp(`^${move.nameJa}`) });
        expect(option.textContent).toContain(move.nameJa);
        expect(option.textContent).toContain(String(move.power));
        expect(option.textContent).toMatch(/・/);
      }
    }
  });
});

describe("S-3 選択中の技と既定の自動選択", () => {
  test("技を選んでから並びを変えても、選択中の技は変わらない", async () => {
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    await user.selectOptions(moveSelect(), "examplesortspark");
    for (const label of [
      moveSortText.options.kana,
      moveSortText.options.type,
      moveSortText.options.learnset,
    ]) {
      await chooseSort(user, label);
      expect(moveSelect()).toHaveValue("examplesortspark");
    }
  });

  test("先に五十音順にしてから攻撃側を選んでも、既定の技は learnset の最初のダメージ技(並びの先頭ではない)", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, "kana");
    render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(attackerSelect(), SORT_SPECIES_KEY);
    expect(optionIds()).toEqual(KANA_IDS);
    expect(moveSelect()).toHaveValue(LEARNSET_IDS[0]);
    expect(KANA_IDS[0]).not.toBe(LEARNSET_IDS[0]);
  });
});

describe("S-4 結果と要求は変わらない", () => {
  test("並びを変えても、計算の要求は増えず・変わらない", async () => {
    const user = userEvent.setup();
    const { engine } = await renderWithSortSpecies(user);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), "9002-000");
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
    const before = engine.bulkRequests.length;
    const last = JSON.stringify(engine.bulkRequests[before - 1]);
    for (const label of [
      moveSortText.options.kana,
      moveSortText.options.type,
      moveSortText.options.learnset,
    ]) {
      await chooseSort(user, label);
    }
    // 並びの切り替えだけでは再計算しない・要求の中身も同じ。
    expect(engine.bulkRequests).toHaveLength(before);
    expect(JSON.stringify(engine.bulkRequests[before - 1])).toBe(last);
  });
});

describe("S-5 並びを覚える", () => {
  test("選んだ並びは localStorage の pokecalc.moveSort に書き、別のマウントで復元する", async () => {
    const user = userEvent.setup();
    const first = await renderWithSortSpecies(user);
    await chooseSort(user, moveSortText.options.kana);
    expect(localStorage.getItem(MOVE_SORT_STORAGE_KEY)).toBe("kana");
    first.unmount();

    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.kana })).toBeChecked();
    await user.selectOptions(attackerSelect(), SORT_SPECIES_KEY);
    expect(optionIds()).toEqual(KANA_IDS);
  });

  test("保存された値が不正なら既定(習得順)", async () => {
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, "bogus");
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.learnset })).toBeChecked();
    expect(optionIds()).toEqual(LEARNSET_IDS);
  });

  test("localStorage が読み書きで例外を投げる環境でも動く(画面の中では切り替わる)", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("denied", "QuotaExceededError");
    });
    const user = userEvent.setup();
    await renderWithSortSpecies(user);
    expect(optionIds()).toEqual(LEARNSET_IDS);
    await chooseSort(user, moveSortText.options.kana);
    expect(optionIds()).toEqual(KANA_IDS);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.kana })).toBeChecked();
  });
});

describe("S-6 技を戻せなかったときの未選択の選択肢(F-09)", () => {
  const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;
  const side = (speciesKey: string): Schemas["Individual"] => ({
    speciesKey,
    level: 50,
    natureId: "example-nature-neutral-docile",
    sp: ZERO,
    abilityId: "exampleabilitynone",
    itemId: "exampleitempower",
    ranks: { atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
  });
  // 戻せない技(マスタに無い ID)を持つお気に入り(CalcScreen.favoriteRestore.test.tsx の R-9 と同じ作り)。
  const goneCalc: Schemas["CalcRequest"] = {
    format: "single",
    attacker: side(SORT_SPECIES_KEY),
    defender: side("9002-000"),
    moveId: "examplemovegone",
    field: {},
    options: { critical: false },
  };
  const request: FavoriteRestoreRequest = {
    token: 1,
    favorite: {
      id: "41",
      label: "テストならびかえ→テストみず(なし)",
      individual: goneCalc.attacker,
      calc: goneCalc,
      createdAt: "2026-10-04T01:00:00Z",
      updatedAt: "2026-10-04T01:00:00Z",
    },
  };

  test.each(Object.entries(moveSortText.options))(
    "並びが %s でも、disabled の未選択の選択肢が先頭で選ばれたまま",
    async (key) => {
      localStorage.setItem(MOVE_SORT_STORAGE_KEY, key);
      render(<CalcScreen engine={createFakeEngine()} master={master} restoreRequest={request} />);
      await screen.findByRole("alert");
      const options = within(moveSelect()).getAllByRole("option");
      expect(options[0]).toHaveTextContent(favoritesRestoreText.moveUnselectedOption);
      expect(options[0]).toBeDisabled();
      expect(options[0]).toHaveProperty("selected", true);
      expect(options).toHaveLength(SORT_MOVES.length + 1);
    },
  );
});

describe("S-7 オンライン(検索で解決した learnset)", () => {
  test("検索で選んだ種族でも、並びが同じ", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    render(
      <CalcScreen
        engine={createFakeEngine()}
        master={limitedMaster(master, { speciesList: false, moves: true, effects: true })}
        masterSearch={search}
      />,
    );
    await user.type(attackerSelect(), sortSpecies().nameJa);
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await screen.findByRole("option", { name: sortSpecies().nameJa }));
    await waitFor(() => {
      expect(optionIds()).toEqual(LEARNSET_IDS);
    });
    await chooseSort(user, moveSortText.options.kana);
    expect(optionIds()).toEqual(KANA_IDS);
    await chooseSort(user, moveSortText.options.type);
    expect(optionIds()).toEqual(TYPE_IDS);
  });
});
