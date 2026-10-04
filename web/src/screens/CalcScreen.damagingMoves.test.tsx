// I-web-2(ADR-0328): 計算画面の技の選択肢はダメージを与える技(物理・特殊)だけ。変化技は出さない。
//   - オフライン(MasterData.moves)・オンライン(検索で解決した learnset)の両方で、選択肢に変化技が無い
//   - 既定の技は従来どおり最初のダメージ技(learnset の順)
//   - 変化技しか覚えない種族: 選択肢 0 件・案内を出し・計算しない(status-move の案内は出ない)
//   - 変化技だけの種族から別の種族へ変えると、技が選び直される
// 技の分類はマスタの Move.category から導く(テストも名前・ID を直書きしない)。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { learnsetMoves } from "../domain/moves";
import { calcScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

// 期待値はマスタの分類(Move.category)から導く(実装の関数には依存しない)。
const damaging = (species: MasterSpecies, moves: MasterData["moves"]) =>
  learnsetMoves(species, moves).filter((move) => move.category !== "status");

let example: MasterData;
let mixed: MasterSpecies; // 物理・特殊と変化技の両方を覚える
let statusOnly: MasterSpecies; // 変化技だけ覚える(学習セットはマスタの分類から導いた変化技 ID)
let master: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
  const found = example.species.find((species) => {
    const all = learnsetMoves(species, example.moves);
    return all.some((m) => m.category === "status") && damaging(species, example.moves).length > 0;
  });
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

const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });

function optionIds(select: HTMLElement): string[] {
  return within(select)
    .queryAllByRole("option")
    .map((option) => (option as HTMLOptionElement).value)
    .filter((value) => value !== "");
}

describe("オフライン(MasterData.moves)", () => {
  test("技の選択肢に変化技が出ず、learnset の順のダメージ技だけが並ぶ", async () => {
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    await user.selectOptions(attackerSpeciesSelect(), mixed.key);

    const expected = damaging(mixed, master.moves).map((move) => move.id);
    expect(expected.length).toBeGreaterThan(0);
    expect(optionIds(moveSelect())).toEqual(expected);
    for (const move of learnsetMoves(mixed, master.moves).filter((m) => m.category === "status")) {
      expect(within(moveSelect()).queryByRole("option", { name: move.nameJa })).toBeNull();
    }
  });

  test("既定の技は最初のダメージ技", async () => {
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    await user.selectOptions(attackerSpeciesSelect(), mixed.key);

    expect(moveSelect()).toHaveValue(damaging(mixed, master.moves)[0]?.id);
  });

  test("変化技だけの種族: 選択肢は0件で案内が出て、計算せず、変化技の案内(status-move)も出ない", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(defenderSpeciesSelect(), mixed.key);
    await user.selectOptions(attackerSpeciesSelect(), statusOnly.key);

    expect(optionIds(moveSelect())).toEqual([]);
    expect(await screen.findByText(calcScreenText.noDamagingMovesNotice)).toBeInTheDocument();
    expect(screen.queryByText(calcScreenText.statusMoveNotice)).toBeNull();
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
    expect(engine.bulkRequests).toHaveLength(0);
  });

  test("変化技だけの種族から別の種族へ変えると、最初のダメージ技が選ばれて計算される", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(defenderSpeciesSelect(), mixed.key);
    await user.selectOptions(attackerSpeciesSelect(), statusOnly.key);
    await user.selectOptions(attackerSpeciesSelect(), mixed.key);

    expect(moveSelect()).toHaveValue(damaging(mixed, master.moves)[0]?.id);
    expect(screen.queryByText(calcScreenText.noDamagingMovesNotice)).toBeNull();
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
  });
});

describe("オンライン(検索で解決した種族の learnset → 技)", () => {
  test("変化技が選択肢に出ず、既定は最初のダメージ技", async () => {
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
    await user.type(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), mixed.nameJa);
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await screen.findByRole("option", { name: mixed.nameJa }));

    const expected = damaging(mixed, master.moves).map((move) => move.id);
    await waitFor(() => {
      expect(optionIds(moveSelect())).toEqual(expected);
    });
    expect(moveSelect()).toHaveValue(expected[0]);
  });
});
