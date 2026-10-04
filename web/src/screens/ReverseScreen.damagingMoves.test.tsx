// I-web-2(ADR-0328): 逆算画面の技の選択肢(攻撃側になるほうの learnset)はダメージを与える技だけ。
//   - オフライン・オンラインの両方で変化技が出ない。既定は最初のダメージ技
//   - 変化技だけの種族: 選択肢0件・案内・逆算しない(status-move の案内は出ない)
//   - 自分/相手のどちらの側をダメージしても同じ(技の出どころが変わる)

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { learnsetMoves } from "../domain/moves";
import { calcScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { ReverseScreen } from "./ReverseScreen";

// 期待値はマスタの分類(Move.category)から導く(実装の関数には依存しない)。
const damaging = (species: MasterSpecies, moves: MasterData["moves"]) =>
  learnsetMoves(species, moves).filter((move) => move.category !== "status");

let mixed: MasterSpecies;
let statusOnly: MasterSpecies;
let master: MasterData;

beforeAll(async () => {
  const example = await exampleMasterSource.load();
  const found = example.species.find(
    (species) =>
      learnsetMoves(species, example.moves).some((m) => m.category === "status") &&
      damaging(species, example.moves).length > 0,
  );
  const statusMove = example.moves.find((move) => move.category === "status");
  if (found === undefined || statusMove === undefined) {
    throw new Error("例データに変化技を覚える種族が無い");
  }
  mixed = found;
  statusOnly = {
    ...found,
    key: "9190-000",
    dexNo: 9190,
    nameJa: "テスト変化だけ",
    learnset: [statusMove.id],
  };
  master = { ...example, species: [...example.species, statusOnly] };
});

afterEach(() => {
  vi.useRealTimers();
});

const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const sideGroup = () => screen.getByRole("radiogroup", { name: "どちらのダメージ" });

function optionIds(select: HTMLElement): string[] {
  return within(select)
    .queryAllByRole("option")
    .map((option) => (option as HTMLOptionElement).value)
    .filter((value) => value !== "");
}

describe("オフライン", () => {
  test("技の選択肢に変化技が出ない(既定の側=自分が攻撃側でない場合の技の出どころ)・既定は最初のダメージ技", async () => {
    const user = userEvent.setup();
    render(<ReverseScreen engine={createFakeEngine()} master={master} />);
    await user.selectOptions(mySpeciesSelect(), mixed.key);
    await user.selectOptions(theirSpeciesSelect(), mixed.key);

    const expected = damaging(mixed, master.moves).map((move) => move.id);
    expect(expected.length).toBeGreaterThan(0);
    expect(optionIds(moveSelect())).toEqual(expected);
    expect(moveSelect()).toHaveValue(expected[0]);
  });

  test("ダメージした側を切り替えても、変化技は出ない", async () => {
    const user = userEvent.setup();
    render(<ReverseScreen engine={createFakeEngine()} master={master} />);
    await user.selectOptions(mySpeciesSelect(), mixed.key);
    await user.selectOptions(theirSpeciesSelect(), mixed.key);
    const radios = within(sideGroup()).getAllByRole("radio");
    for (const radio of radios) {
      await user.click(radio);
      for (const id of optionIds(moveSelect())) {
        expect(learnsetMoves(mixed, master.moves).find((m) => m.id === id)?.category).not.toBe("status");
      }
    }
  });

  test("技を出す側が変化技だけの種族: 選択肢0件・案内あり・逆算しない・status-move の案内は出ない", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    render(<ReverseScreen engine={engine} master={master} />);
    // 技の出どころはダメージの側で変わるので、両方を変化技だけの種族にして側に依らず0件にする。
    await user.selectOptions(mySpeciesSelect(), statusOnly.key);
    await user.selectOptions(theirSpeciesSelect(), statusOnly.key);

    expect(optionIds(moveSelect())).toEqual([]);
    expect(await screen.findByText(calcScreenText.noDamagingMovesNotice)).toBeInTheDocument();
    expect(screen.queryByText(calcScreenText.statusMoveNotice)).toBeNull();
    expect(engine.reverseRequests).toHaveLength(0);
  });
});

describe("オンライン(検索で解決した種族)", () => {
  test("変化技が選択肢に出ない", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    render(
      <ReverseScreen
        engine={createFakeEngine()}
        master={limitedMaster(master, { speciesList: false, moves: true, effects: true })}
        masterSearch={search}
      />,
    );
    for (const name of ["自分のポケモン", "相手のポケモン"]) {
      await user.type(screen.getByRole("combobox", { name }), mixed.nameJa);
      act(() => {
        vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
      });
      await user.click(await screen.findByRole("option", { name: mixed.nameJa }));
    }

    const expected = damaging(mixed, master.moves).map((move) => move.id);
    expect(optionIds(moveSelect())).toEqual(expected);
  });
});
