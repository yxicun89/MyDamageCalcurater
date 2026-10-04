// オンライン相当のマスタ(speciesList=false・abilities: []・moves: [])でも、未対応の印の特性名・技名が
// 日本語(解決済みの一覧から引いた名前)で出る。ID のまま出さない(フォールバックは引けないときだけ)。
// マスタは架空の例データ。engine は fake。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { bulkRow, createFakeEngine, ok } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

describe("オンライン相当のマスタでの未対応の印の名前", () => {
  test("攻撃側の特性・技の印が、解決済みの一覧から日本語名で出る", async () => {
    const attacker: MasterSpecies | undefined = example.species.find(
      (species) => species.abilities.length > 0,
    );
    const defender = example.species[1];
    if (attacker === undefined || defender === undefined) {
      throw new Error("例データに種族が足りない");
    }
    const abilityId = attacker.abilities[0] as string;
    const ability = example.abilities.find((candidate) => candidate.id === abilityId);
    const moveId = attacker.learnset.find((id) => example.moves.some((move) => move.id === id));
    const move = example.moves.find((candidate) => candidate.id === moveId);
    if (ability === undefined || move === undefined) {
      throw new Error("例データに特性・技が足りない");
    }

    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const engine = createFakeEngine((request) =>
      ok({
        defenderSpeciesKey: request.defenderSpecies.key,
        rows: [
          bulkRow({
            unsupported: [
              { target: "attacker_ability", reason: "unsupported_effect", id: ability.id },
              { target: "move", reason: "unsupported_effect", id: move.id },
            ],
          }),
        ],
      }),
    );
    const master = limitedMaster(example, { speciesList: false, moves: true, effects: true });
    const search = createFakeSpeciesSearch({
      species: example.species,
      abilities: example.abilities,
      moves: example.moves,
    });
    render(<CalcScreen engine={engine} master={master} masterSearch={search} />);

    for (const [label, card, species] of [
      ["攻撃側のポケモン", "攻撃側", attacker],
      ["防御側のポケモン", "防御側", defender],
    ] as const) {
      const region = screen.getByRole("region", { name: card });
      await user.type(within(region).getByRole("combobox", { name: label }), species.nameJa);
      act(() => {
        vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
      });
      await user.click(await within(region).findByRole("option", { name: species.nameJa }));
    }

    const notice = await screen.findByText(/この結果は正確でない可能性があります/);
    expect(notice).toHaveTextContent(ability.nameJa);
    expect(notice).toHaveTextContent(move.nameJa);
    expect(notice).not.toHaveTextContent(ability.id);
    expect(notice).not.toHaveTextContent(move.id);
    vi.useRealTimers();
  });
});
