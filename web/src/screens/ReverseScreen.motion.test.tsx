// P4-8: 逆算画面の演出(docs/design.md「動き」、plan.md P4-8「逆算で観測を追加したときの絞り込みの動き」)。
// jsdom は実際のアニメーションを動かさないので、きっかけ(状態クラス)だけを確かめる。
// 確かめること:
//   - 観測が1つだけの結果では、候補の一覧に is-narrowing を付けない(最初の推定は「絞り込み」ではない)
//   - 2つ目以降の観測を入れて届いた結果では、候補の一覧(role=list「推定結果」)に is-narrowing を付け、animationend で外す
//   - 観測を削除して1つに戻った結果では付けない
//   - OS の「視差効果を減らす」では付けない

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fakeMatchMedia } from "../test/matchMedia";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const observationInput = (n: number) => screen.getByRole("textbox", { name: `観測${String(n)}` });
const addObservationButton = () => screen.getByRole("button", { name: "観測を追加" });
const candidateList = () => screen.findByRole("list", { name: "推定結果" });

function renderScreen(): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  const engine = createFakeEngine();
  render(<ReverseScreen engine={engine} master={master} />);
  return { user, engine };
}

/** 直近の calcReverse に渡った観測の数が count になるまで待ち、その結果の一覧を返す。 */
async function listAfterObservations(engine: FakeEngine, count: number): Promise<HTMLElement> {
  await waitFor(() => {
    expect(engine.reverseRequests.at(-1)?.observations).toHaveLength(count);
  });
  return candidateList();
}

async function enterTwoObservations(user: UserEvent): Promise<void> {
  await user.selectOptions(mySpeciesSelect(), speciesAt(0).key);
  await user.selectOptions(theirSpeciesSelect(), speciesAt(1).key);
  await user.type(observationInput(1), "45");
  await user.click(addObservationButton());
  await user.type(observationInput(2), "50");
}

describe("観測を追加すると候補が絞られる動き", () => {
  test("観測が1つだけの結果では is-narrowing を付けない(入力し直しても付けない)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), speciesAt(0).key);
    await user.selectOptions(theirSpeciesSelect(), speciesAt(1).key);
    await user.type(observationInput(1), "45");
    expect(await listAfterObservations(engine, 1)).not.toHaveClass("is-narrowing");

    await user.clear(observationInput(1));
    await user.type(observationInput(1), "40");
    await waitFor(() => {
      expect(engine.reverseRequests.at(-1)?.observations).toEqual([{ percent: 40 }]);
    });
    expect(await candidateList()).not.toHaveClass("is-narrowing");
  });

  test("2つ目の観測を入れて届いた結果の一覧に is-narrowing を付け、animationend で外す", async () => {
    const { user, engine } = renderScreen();
    await enterTwoObservations(user);
    const list = await listAfterObservations(engine, 2);
    await waitFor(() => {
      expect(list).toHaveClass("is-narrowing");
    });

    fireEvent.animationEnd(list);
    expect(list).not.toHaveClass("is-narrowing");
  });

  test("2つ目以降の観測を入れ直すたびに、新しい結果でまた絞り込みの動きを出す", async () => {
    const { user, engine } = renderScreen();
    await enterTwoObservations(user);
    const first = await listAfterObservations(engine, 2);
    await waitFor(() => {
      expect(first).toHaveClass("is-narrowing");
    });
    fireEvent.animationEnd(first);

    await user.type(observationInput(2), "{Backspace}");
    await waitFor(() => {
      expect(engine.reverseRequests.at(-1)?.observations).toEqual([{ percent: 45 }, { percent: 5 }]);
    });
    await waitFor(async () => {
      expect(await candidateList()).toHaveClass("is-narrowing");
    });
  });

  test("観測を削除して1つに戻った結果では付けない(絞り込みではない)", async () => {
    const { user, engine } = renderScreen();
    await enterTwoObservations(user);
    await listAfterObservations(engine, 2);

    await user.click(screen.getByRole("button", { name: "観測2を削除" }));
    expect(await listAfterObservations(engine, 1)).not.toHaveClass("is-narrowing");
  });

  test("OS の「視差効果を減らす」では付けない", async () => {
    vi.stubGlobal("matchMedia", fakeMatchMedia(true));
    const { user, engine } = renderScreen();
    await enterTwoObservations(user);
    expect(await listAfterObservations(engine, 2)).not.toHaveClass("is-narrowing");
  });
});
