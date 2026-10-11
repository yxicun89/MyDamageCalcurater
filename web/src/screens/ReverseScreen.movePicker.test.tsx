// G-01(ADR-0341): 逆算画面の技欄も技ピッカー(計算画面と同じ部品・同じ並び=タイプ順だけ)。
//   R-1 トリガー(combobox「技」)で、並びの切り替えは無い
//   R-2 開くとタイプ順。既定の技は最初のダメージ技。行を選ぶと選択中が変わる
//   R-3 廃止した並びの記憶(pokecalc.moveSort)は読まない

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { chooseMove, listedMoveIds, moveTrigger, selectedMoveId } from "../test/movePicker";
import { LEARNSET_IDS, SORT_SPECIES_KEY, TYPE_IDS, withSortFixture } from "../test/moveSortMaster";
import { ReverseScreen } from "./ReverseScreen";

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
  render(<ReverseScreen engine={createFakeEngine()} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), SORT_SPECIES_KEY);
  await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), SORT_SPECIES_KEY);
  return { user };
}

describe("逆算画面の技ピッカー", () => {
  test("R-1 トリガーで、並びの切り替えは無い", async () => {
    await setup();
    expect(moveTrigger().tagName.toLowerCase()).not.toBe("select");
    expect(screen.queryByRole("radiogroup", { name: "技の並び" })).toBeNull();
  });

  test("R-2 タイプ順・既定は最初のダメージ技・選ぶと変わる", async () => {
    const { user } = await setup();
    expect(selectedMoveId()).toBe(LEARNSET_IDS[0]);
    expect(await listedMoveIds(user)).toEqual(TYPE_IDS);
    await chooseMove(user, "examplesortspark");
    expect(selectedMoveId()).toBe("examplesortspark");
  });

  test.each(["kana", "type", "bogus"])("R-3 pokecalc.moveSort=%s が残っていてもタイプ順", async (value) => {
    localStorage.setItem("pokecalc.moveSort", value);
    const { user } = await setup();
    expect(await listedMoveIds(user)).toEqual(TYPE_IDS);
  });
});
