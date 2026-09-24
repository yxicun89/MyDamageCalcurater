// P4-16b: 逆算画面の、機能が欠けたマスタ(オンライン相当)での見え方(ADR-0304 A-5・A-10・A-11)。
// engine は fake、マスタは架空の例データから項目を落としたもの(test/onlineMaster.ts)。
// 確かめること:
//   - 技が無い(moves: false): 技のセレクトは残すが disabled・選択肢は空・案内を出す・逆算しない
//   - 効果データが無い(effects: false): 持ち物候補を探索していないことを案内する。逆算自体は実行する(A-11)
//   - 種族の一覧が無い(speciesList: false): 自分・相手とも検索欄になり、解決した種族で逆算する
//   - capabilities を省いた既存のマスタでは、今までどおり

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { masterOnlineText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterCapabilities, MasterData, MasterSpecies, MasterSpeciesSearch } from "../master/types";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster, type FakeSpeciesSearch } from "../test/onlineMaster";
import { ReverseScreen } from "./ReverseScreen";

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

const SEARCH_ONLY: MasterCapabilities = { speciesList: false, moves: true, effects: true };
const NO_MOVES: MasterCapabilities = { speciesList: true, moves: false, effects: true };
const NO_EFFECTS: MasterCapabilities = { speciesList: true, moves: true, effects: false };

function speciesAt(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const observationInput = () => screen.getByRole("textbox", { name: "観測1" });

interface RenderResult {
  readonly user: UserEvent;
  readonly engine: FakeEngine;
  advance(ms: number): void;
}

function renderScreen(master: MasterData, search?: MasterSpeciesSearch): RenderResult {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const engine = createFakeEngine();
  render(<ReverseScreen engine={engine} master={master} masterSearch={search} />);
  return {
    user,
    engine,
    advance(ms) {
      act(() => {
        vi.advanceTimersByTime(ms);
      });
    },
  };
}

function fakeSearch(): FakeSpeciesSearch {
  return createFakeSpeciesSearch({ species: example.species, abilities: example.abilities });
}

async function chooseBySearch(
  rendered: RenderResult,
  card: HTMLElement,
  label: string,
  species: MasterSpecies,
): Promise<void> {
  const input = within(card).getByRole("combobox", { name: label });
  await rendered.user.type(input, species.nameJa);
  rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
  const option = await within(card).findByRole("option", { name: species.nameJa });
  await rendered.user.click(option);
}

describe("技が使えないマスタ(capabilities.moves === false)", () => {
  test("技のセレクトは残るが disabled で、選択肢が1件も無い・案内を出す", () => {
    renderScreen(limitedMaster(example, NO_MOVES));
    expect(moveSelect()).toBeDisabled();
    expect(within(moveSelect()).queryAllByRole("option")).toHaveLength(0);
    expect(screen.getByText(masterOnlineText.movesUnavailable)).toBeInTheDocument();
  });

  test("自分・相手・観測を入れても逆算しない(壊れた結果を出さない。ADR-0304 §4)", async () => {
    const rendered = renderScreen(limitedMaster(example, NO_MOVES));
    await rendered.user.selectOptions(
      within(myCard()).getByRole("combobox", { name: "自分のポケモン" }),
      speciesAt(0).key,
    );
    await rendered.user.selectOptions(
      within(theirCard()).getByRole("combobox", { name: "相手のポケモン" }),
      speciesAt(1).key,
    );
    await rendered.user.type(observationInput(), "50");
    // P4-18(issue 113): 観測の数値入力は 200ms 待ってから計算する。
    rendered.advance(OBSERVATION_INPUT_DEBOUNCE_MS);

    expect(rendered.engine.reverseRequests).toHaveLength(0);
    expect(screen.queryByRole("list", { name: "推定結果" })).toBeNull();
  });
});

describe("効果データが無いマスタ(capabilities.effects === false。ADR-0304 A-11)", () => {
  test("持ち物の候補を探索していないことを案内する", () => {
    renderScreen(limitedMaster(example, NO_EFFECTS));
    expect(screen.getByText(masterOnlineText.itemCandidatesUnavailable)).toBeInTheDocument();
  });

  test("逆算そのものは実行する(候補は「持ち物なし」だけ = 前提の狭い正しい結果)", async () => {
    const rendered = renderScreen(limitedMaster(example, NO_EFFECTS));
    await rendered.user.selectOptions(
      within(myCard()).getByRole("combobox", { name: "自分のポケモン" }),
      speciesAt(0).key,
    );
    await rendered.user.selectOptions(
      within(theirCard()).getByRole("combobox", { name: "相手のポケモン" }),
      speciesAt(1).key,
    );
    await rendered.user.type(observationInput(), "50");
    // P4-18(issue 113): 観測の数値入力は 200ms 待ってから計算する。
    rendered.advance(OBSERVATION_INPUT_DEBOUNCE_MS);

    await waitFor(() => {
      expect(rendered.engine.reverseRequests).not.toHaveLength(0);
    });
    expect(rendered.engine.reverseRequests.at(-1)?.itemCandidates).toEqual([null]);
  });
});

describe("種族の一覧が無いマスタ(capabilities.speciesList === false)", () => {
  test("自分・相手とも検索欄になり、入力前は候補を出さない", () => {
    renderScreen(limitedMaster(example, SEARCH_ONLY), fakeSearch());
    for (const [card, label] of [
      [myCard(), "自分のポケモン"],
      [theirCard(), "相手のポケモン"],
    ] as const) {
      expect(within(card).getByRole("combobox", { name: label })).not.toBeDisabled();
      expect(within(card).queryAllByRole("option")).toHaveLength(0);
      expect(within(card).getByText(masterOnlineText.speciesSearchEmpty)).toBeInTheDocument();
    }
  });

  test("検索で選んだ種族で逆算する(技は攻撃側の learnset から選べる)", async () => {
    const search = fakeSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const mine = speciesAt(0);
    const theirs = speciesAt(1);

    await chooseBySearch(rendered, myCard(), "自分のポケモン", mine);
    await chooseBySearch(rendered, theirCard(), "相手のポケモン", theirs);
    // 「与えたダメージ」(既定)は自分が攻撃側なので、技は自分の learnset から出る。
    expect(within(moveSelect()).getAllByRole("option").length).toBeGreaterThan(0);

    await rendered.user.type(observationInput(), "50");
    // P4-18(issue 113): 観測の数値入力は 200ms 待ってから計算する。
    rendered.advance(OBSERVATION_INPUT_DEBOUNCE_MS);

    await waitFor(() => {
      expect(rendered.engine.reverseRequests).not.toHaveLength(0);
    });
    const request = rendered.engine.reverseRequests.at(-1);
    expect(request?.known.species.key).toBe(mine.key);
    expect(request?.unknownSpecies.key).toBe(theirs.key);
    // MasterData.abilities は空なので、画面が resolveSpecies の特性を覚えていないと「特性なし」に落ちる。
    expect(request?.known.ability.id).toBe(mine.abilities[0]);
    expect(search.resolvedKeys).toEqual([mine.key, theirs.key]);
  });

  test("検索口が渡されていなければ、空のドロップダウンを出さず検索欄を disabled にする", () => {
    renderScreen(limitedMaster(example, SEARCH_ONLY));
    const input = within(myCard()).getByRole("combobox", { name: "自分のポケモン" });
    expect(input).toBeDisabled();
    expect(within(myCard()).getByText(masterOnlineText.speciesSearchFailed)).toBeInTheDocument();
  });
});

describe("capabilities を省いたマスタ(オフライン相当)は今までどおり", () => {
  test("技のセレクトは使え、案内も検索欄も出さない", () => {
    renderScreen(example);
    expect(moveSelect()).not.toBeDisabled();
    expect(screen.queryByText(masterOnlineText.movesUnavailable)).toBeNull();
    expect(screen.queryByText(masterOnlineText.itemCandidatesUnavailable)).toBeNull();
    expect(screen.queryByText(masterOnlineText.speciesSearchEmpty)).toBeNull();
    const select = within(myCard()).getByRole("combobox", { name: "自分のポケモン" });
    for (const species of example.species) {
      expect(within(select).getByRole("option", { name: species.nameJa })).toHaveValue(species.key);
    }
  });
});
