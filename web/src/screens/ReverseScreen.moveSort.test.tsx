// I-web-9 = F-02(ADR-0335): 逆算画面の技の選択肢の並び。計算画面と同じ「技の並び」のチップ群・同じ並び・同じ保存先。
//   V-1 既定は習得順・チップ群あり   V-2 五十音順・タイプ順(optgroup)に切り替わる
//   V-3 選択中の技は並びを変えても保たれ、既定の自動選択は並びに依らない
//   V-4 並びの切り替えで逆算の要求は増えない・変わらない
//   V-5 localStorage(pokecalc.moveSort)に覚える/不正値は既定/使えなくても動く。計算画面で選んだ並びを引き継ぐ
// 架空のデータだけ(test/moveSortMaster.ts)。engine は fake。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import { MOVE_SORT_STORAGE_KEY } from "../app/moveSortStorage";
import { moveSortText } from "../i18n/moveSort";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import {
  KANA_IDS,
  LEARNSET_IDS,
  SORT_SPECIES_KEY,
  TYPE_GROUPS,
  TYPE_IDS,
  withSortFixture,
} from "../test/moveSortMaster";
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

const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const sortGroup = () => screen.getByRole("radiogroup", { name: moveSortText.groupLabel });

function optionIds(): string[] {
  return within(moveSelect())
    .getAllByRole("option")
    .map((option) => (option as HTMLOptionElement).value)
    .filter((value) => value !== "");
}

async function setup() {
  const user = userEvent.setup();
  const engine = createFakeEngine();
  const view = render(<ReverseScreen engine={engine} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), SORT_SPECIES_KEY);
  await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), SORT_SPECIES_KEY);
  return { user, engine, ...view };
}

const choose = (user: ReturnType<typeof userEvent.setup>, label: string) =>
  user.click(within(sortGroup()).getByRole("radio", { name: label }));

describe("V-1 既定", () => {
  test("チップ群は3つで、既定は習得順(従来の並び)・既定の技は最初のダメージ技", async () => {
    await setup();
    expect(within(sortGroup()).getAllByRole("radio")).toHaveLength(3);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.learnset })).toBeChecked();
    expect(optionIds()).toEqual(LEARNSET_IDS);
    expect(moveSelect()).toHaveValue(LEARNSET_IDS[0]);
  });
});

describe("V-2 切り替え", () => {
  test("五十音順", async () => {
    const { user } = await setup();
    await choose(user, moveSortText.options.kana);
    expect(optionIds()).toEqual(KANA_IDS);
  });

  test("タイプ順は optgroup(タイプ名の見出し)つき、群の中は五十音順", async () => {
    const { user } = await setup();
    await choose(user, moveSortText.options.type);
    expect(optionIds()).toEqual(TYPE_IDS);
    const groups = within(moveSelect()).getAllByRole("group");
    expect(groups.map((g) => g.getAttribute("label"))).toEqual(TYPE_GROUPS.map((g) => g.label));
  });
});

describe("V-3・V-4 選択中の技と要求", () => {
  test("技を選んでから並びを変えても選択中の技は同じ", async () => {
    const { user } = await setup();
    await user.selectOptions(moveSelect(), "examplesortspark");
    await choose(user, moveSortText.options.kana);
    expect(moveSelect()).toHaveValue("examplesortspark");
    await choose(user, moveSortText.options.type);
    expect(moveSelect()).toHaveValue("examplesortspark");
  });

  test("並びを変えても逆算の要求は増えず・変わらない", async () => {
    const { user, engine } = await setup();
    const before = engine.reverseRequests.length;
    const last = JSON.stringify(engine.reverseRequests[before - 1]);
    await choose(user, moveSortText.options.kana);
    await choose(user, moveSortText.options.type);
    expect(engine.reverseRequests).toHaveLength(before);
    expect(JSON.stringify(engine.reverseRequests[before - 1])).toBe(last);
  });

  test("先に五十音順を覚えていても、攻撃側の既定の技は learnset の最初のダメージ技", async () => {
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, "kana");
    await setup();
    expect(optionIds()).toEqual(KANA_IDS);
    expect(moveSelect()).toHaveValue(LEARNSET_IDS[0]);
  });
});

describe("V-5 並びを覚える", () => {
  test("選んだ並びを pokecalc.moveSort に書き、別のマウントで復元する", async () => {
    const { user, unmount } = await setup();
    await choose(user, moveSortText.options.type);
    expect(localStorage.getItem(MOVE_SORT_STORAGE_KEY)).toBe("type");
    unmount();
    render(<ReverseScreen engine={createFakeEngine()} master={master} />);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.type })).toBeChecked();
  });

  test("計算画面と同じキーを読む(計算画面で選んだ並びを引き継ぐ)", () => {
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, "kana");
    render(<ReverseScreen engine={createFakeEngine()} master={master} />);
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.kana })).toBeChecked();
  });

  test("不正な値は既定(習得順)", async () => {
    localStorage.setItem(MOVE_SORT_STORAGE_KEY, "zzz");
    await setup();
    expect(within(sortGroup()).getByRole("radio", { name: moveSortText.options.learnset })).toBeChecked();
    expect(optionIds()).toEqual(LEARNSET_IDS);
  });

  test("localStorage が使えない環境でも動く", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("denied", "QuotaExceededError");
    });
    const { user } = await setup();
    expect(optionIds()).toEqual(LEARNSET_IDS);
    await choose(user, moveSortText.options.kana);
    expect(optionIds()).toEqual(KANA_IDS);
  });
});
