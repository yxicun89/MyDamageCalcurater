// G-01(ADR-0341): 計算画面の技欄は技ピッカー(MovePicker)。ADR-0335 の並びの切り替えは廃止され、並びはタイプ順だけ。
//   C-1 技欄はトリガー(combobox「技」)で、<select> ではない。並びの切り替え(radiogroup「技の並び」・習得順/五十音順チップ)は無い
//   C-2 開くとタイプ順(タイプ表の並び→五十音順)。ダメージ技だけ(ADR-0328)
//   C-3 既定の技の自動選択は従来どおり learnset の最初のダメージ技(並びがタイプ順でも変えない)
//   C-4 行を選ぶと選択中の技が変わり、計算が走る(要求の moveId が選んだ技)
//   C-5 廃止した並びの記憶(localStorage pokecalc.moveSort)は読まない。値が残っていてもタイプ順
//   C-6 技の候補が無いときはトリガーが disabled
// 架空のデータだけ(test/moveSortMaster.ts)。engine は fake。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { chooseMove, listedMoveIds, moveTrigger, selectedMoveId } from "../test/movePicker";
import { LEARNSET_IDS, SORT_SPECIES_KEY, TYPE_IDS, withSortFixture } from "../test/moveSortMaster";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;
beforeAll(async () => {
  master = withSortFixture(await exampleMasterSource.load());
});
beforeEach(() => {
  localStorage.clear();
});
afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

async function setup() {
  const user = userEvent.setup();
  const engine = createFakeEngine();
  const view = render(<CalcScreen engine={engine} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), SORT_SPECIES_KEY);
  return { user, engine, ...view };
}

describe("C-1 技ピッカー", () => {
  test("技欄は <select> ではなくトリガーで、並びの切り替えは無い", async () => {
    await setup();
    expect(moveTrigger().tagName.toLowerCase()).not.toBe("select");
    expect(screen.queryByRole("radiogroup", { name: "技の並び" })).toBeNull();
    expect(screen.queryByRole("radio", { name: /習得順|五十音順|タイプ順/ })).toBeNull();
  });
});

describe("C-2・C-3 並びと既定", () => {
  test("開くとタイプ順。既定の技は learnset の最初のダメージ技", async () => {
    const { user } = await setup();
    expect(selectedMoveId()).toBe(LEARNSET_IDS[0]);
    expect(await listedMoveIds(user)).toEqual(TYPE_IDS);
    expect(selectedMoveId()).toBe(LEARNSET_IDS[0]);
  });
});

describe("C-4 選ぶ", () => {
  test("行を選ぶと選択中の技が変わり、その技で計算を要求する", async () => {
    const { user, engine } = await setup();
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), "9002-000");
    const spy = vi.spyOn(engine, "calcBulk");
    await chooseMove(user, "examplesortspark");
    expect(selectedMoveId()).toBe("examplesortspark");
    await waitFor(() => {
      expect(JSON.stringify(spy.mock.calls.at(-1))).toContain("examplesortspark");
    });
  });
});

describe("C-5 廃止した並びの記憶", () => {
  test.each(["kana", "learnset", "type", "bogus"])(
    "pokecalc.moveSort=%s が残っていてもタイプ順で、書き換えない",
    async (value) => {
      localStorage.setItem("pokecalc.moveSort", value);
      const { user } = await setup();
      expect(await listedMoveIds(user)).toEqual(TYPE_IDS);
      expect(localStorage.getItem("pokecalc.moveSort")).toBe(value);
    },
  );
});

describe("C-6 候補が無い", () => {
  test("技を覚えない種族ならトリガーは disabled", async () => {
    const user = userEvent.setup();
    const none = {
      ...master,
      species: master.species.map((s) => (s.key === SORT_SPECIES_KEY ? { ...s, learnset: [] } : s)),
    };
    render(<CalcScreen engine={createFakeEngine()} master={none} />);
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), SORT_SPECIES_KEY);
    expect(moveTrigger()).toBeDisabled();
  });
});
