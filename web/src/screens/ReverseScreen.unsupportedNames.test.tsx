// オンライン相当のマスタ(speciesList=false・abilities: []・moves: [])でも、逆算の未対応の印の特性名・技名が
// 日本語(解決済みの一覧から引いた名前)で出る。ID のまま出さない。マスタは架空の例データ。engine は fake。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData } from "../master/types";
import { createFakeEngine, ok, reverseCandidate } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { ReverseScreen } from "./ReverseScreen";

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

describe("オンライン相当のマスタでの逆算の未対応の印の名前", () => {
  test("全候補に共通する特性・技の印が、解決済みの一覧から日本語名で出る", async () => {
    const mine = example.species.find((species) => species.abilities.length > 0);
    const theirs = example.species[1];
    if (mine === undefined || theirs === undefined) {
      throw new Error("例データに種族が足りない");
    }
    const ability = example.abilities.find((candidate) => candidate.id === mine.abilities[0]);
    const move = example.moves.find((candidate) => mine.learnset.includes(candidate.id));
    if (ability === undefined || move === undefined) {
      throw new Error("例データに特性・技が足りない");
    }
    const unsupported = [
      { target: "attacker_ability", reason: "unsupported_effect", id: ability.id },
      { target: "move", reason: "unsupported_effect", id: move.id },
    ] as const;

    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const engine = createFakeEngine(undefined, () =>
      ok({
        side: "attacker",
        stat: "atk",
        assumedHpSp: 32,
        exactCount: 1,
        candidates: [reverseCandidate({ unsupported })],
      }),
    );
    const master = limitedMaster(example, { speciesList: false, moves: true, effects: true });
    const search = createFakeSpeciesSearch({
      species: example.species,
      abilities: example.abilities,
      moves: example.moves,
    });
    render(<ReverseScreen engine={engine} master={master} masterSearch={search} />);

    for (const [label, card, species] of [
      ["自分のポケモン", "自分", mine],
      ["相手のポケモン", "相手", theirs],
    ] as const) {
      const region = screen.getByRole("region", { name: new RegExp(card) });
      await user.type(within(region).getByRole("combobox", { name: label }), species.nameJa);
      act(() => {
        vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
      });
      await user.click(await within(region).findByRole("option", { name: species.nameJa }));
    }
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");
    act(() => {
      vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
    });

    const notice = await screen.findByText(/この結果は正確でない可能性があります/);
    expect(notice).toHaveTextContent(ability.nameJa);
    expect(notice).toHaveTextContent(move.nameJa);
    expect(notice).not.toHaveTextContent(ability.id);
    expect(notice).not.toHaveTextContent(move.id);
    vi.useRealTimers();
  });
});
