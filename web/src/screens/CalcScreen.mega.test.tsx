// issue #515・ADR-0320(docs/mega-evolution-spec.md §4-3): 計算画面のメガシンカの持ち物固定。
// engine は fake。マスタは架空の例データに、架空のメガ種族・メガストーンを足したもの(test/megaMaster.ts)。
// 確かめること:
//   - メガ種族を選ぶと持ち物がメガストーンになり、欄は disabled・名前が見え・理由が見える文言と aria-describedby の両方で伝わる
//   - メガでない種族に変えると固定が外れ、持ち物は未選択に戻る(メガストーンを残さない)
//   - メガストーンは単独の持ち物の選択肢に出ない
//   - 要求(calcBulk)に固定した持ち物が載る(attacker.item / itemVariants)
//   - 「持ち物の候補も比較」はメガの防御側では探索しない(disabled・理由つき・itemVariants はメガストーン1件)。
//     メガストーンは比較の候補にも混ざらない
//   - 攻守入れ替えでも固定が整合する
//   - ストーンをマスタから引けないメガ種族は、固定せず空にして理由を出す(黙って壊さない)
//   - 種族を検索で解決するマスタ(オンライン・キャッシュ済みのオフライン)でも同じに動く
//   - メガでない種族の挙動は変わらない

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import type { BulkRequest } from "../engine/types";
import { calcScreenText, megaItemText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterCapabilities, MasterData, MasterSpecies, MasterSpeciesSearch } from "../master/types";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_ORPHAN,
  MEGA_WATER,
  MEGA_WATER_STONE,
  withMegaFixture,
} from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

let example: MasterData;
let master: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
  master = withMegaFixture(example);
});

afterEach(() => {
  vi.useRealTimers();
});

function normalSpecies(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });
const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const attackerItemSelect = () => screen.getByRole("combobox", { name: "攻撃側の持ち物" });
const defenderItemSelect = () => screen.getByRole("combobox", { name: "防御側の持ち物" });
const compareToggle = () => screen.getByRole("checkbox", { name: /持ち物の候補/ });
const swapButton = () => screen.getByRole("button", { name: calcScreenText.swapButtonLabel });

function lastRequest(engine: FakeEngine): BulkRequest {
  const request = engine.bulkRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return request;
}

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .queryAllByRole("option")
    .map((option) => option.textContent ?? "");
}

function renderScreen(data: MasterData = master): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  const engine = createFakeEngine();
  render(<CalcScreen engine={engine} master={data} />);
  return { user, engine };
}

describe("メガ種族を選ぶと持ち物が固定される", () => {
  test("攻撃側: 持ち物欄は disabled でメガストーンの名前が見え、固定の理由が見える文言と aria-describedby で伝わる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);

    const select = attackerItemSelect();
    expect(select).toBeDisabled();
    expect(select).toHaveValue(MEGA_FIRE_STONE.id);
    expect(select).toHaveDisplayValue(MEGA_FIRE_STONE.nameJa);
    expect(within(attackerCard()).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(select).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("防御側も同じ(攻撃側の持ち物欄には影響しない)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    expect(defenderItemSelect()).toBeDisabled();
    expect(defenderItemSelect()).toHaveValue(MEGA_WATER_STONE.id);
    expect(defenderItemSelect()).toHaveAccessibleDescription(megaItemText.lockedReason);
    expect(within(defenderCard()).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(attackerItemSelect()).not.toBeDisabled();
    expect(within(attackerCard()).queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("先に別の持ち物を選んでいても、メガ種族を選ぶとメガストーンに置き換わる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    const other = example.items[0];
    if (other === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    await user.selectOptions(attackerItemSelect(), other.id);

    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);

    expect(attackerItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(attackerItemSelect()).toBeDisabled();
  });

  test("要求に固定した持ち物が載る(攻撃側は attacker.item、防御側は itemVariants)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
    const request = lastRequest(engine);
    expect(request.attacker.item).toEqual(MEGA_FIRE_STONE);
    expect(request.itemVariants).toEqual([MEGA_WATER_STONE]);
    // 画面のための追加フィールドは engine に渡さない。
    expect(request.attacker.species).not.toHaveProperty("isMega");
    expect(request.defenderSpecies).not.toHaveProperty("requiredItemId");
  });
});

describe("メガでない種族に変えると固定が外れ、持ち物は未選択に戻る", () => {
  test("メガ → メガでない: 欄が操作でき、メガストーンが残らず、理由も消える", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);

    const select = attackerItemSelect();
    expect(select).not.toBeDisabled();
    expect(select).toHaveValue("");
    expect(select).toHaveDisplayValue(calcScreenText.noItemOption);
    expect(select).not.toHaveAccessibleDescription(megaItemText.lockedReason);
    expect(within(attackerCard()).queryByText(megaItemText.lockedReason)).toBeNull();
    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toBeNull();
    });
  });

  test("解除したあとは、持ち物を選び直せる", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    const other = example.items[0];
    if (other === undefined) {
      throw new Error("例データに持ち物が無い");
    }

    await user.selectOptions(attackerItemSelect(), other.id);

    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toEqual(other);
    });
  });

  test("メガ → 別のメガ: 新しいメガストーンに替わる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(attackerSpeciesSelect(), MEGA_WATER.key);

    expect(attackerItemSelect()).toHaveValue(MEGA_WATER_STONE.id);
    expect(attackerItemSelect()).toBeDisabled();
  });

  test("メガでない種族どうしの切り替えでは、選んでいる持ち物を引き継ぐ(既存の挙動)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    const other = example.items[0];
    if (other === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    await user.selectOptions(attackerItemSelect(), other.id);

    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(1).key);

    expect(attackerItemSelect()).toHaveValue(other.id);
    expect(attackerItemSelect()).not.toBeDisabled();
  });
});

describe("メガストーンは単独の持ち物の選択肢に出ない", () => {
  test("種族を選ぶ前・メガでない種族のとき、持ち物の選択肢はメガストーンを除いた例データの持ち物", async () => {
    const { user } = renderScreen();
    const expected = [calcScreenText.noItemOption, ...example.items.map((item) => item.nameJa)];
    expect(optionNames(attackerItemSelect())).toEqual(expected);
    expect(optionNames(defenderItemSelect())).toEqual(expected);

    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    expect(optionNames(attackerItemSelect())).toEqual(expected);
  });

  test("メガ種族を選んだ側の欄にはそのメガストーンの名前が出る(固定の表示)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);

    expect(optionNames(attackerItemSelect())).toContain(MEGA_FIRE_STONE.nameJa);
    // もう一方の欄(メガでない)には出ない。
    expect(optionNames(defenderItemSelect())).not.toContain(MEGA_FIRE_STONE.nameJa);
  });
});

describe("「持ち物の候補も比較」はメガ種族では探索しない", () => {
  test("防御側がメガ: チェックが disabled になり理由を添え、itemVariants はメガストーン1件のまま", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.click(compareToggle());
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants?.length ?? 0).toBeGreaterThan(1);
    });

    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    expect(compareToggle()).toBeDisabled();
    expect(compareToggle()).toHaveAccessibleDescription(megaItemText.compareDisabledReason);
    expect(screen.getByText(megaItemText.compareDisabledReason)).toBeVisible();
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toEqual([MEGA_WATER_STONE]);
    });
  });

  test("防御側をメガでない種族に戻すと、チェックは操作できる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);
    expect(compareToggle()).toBeDisabled();

    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    expect(compareToggle()).not.toBeDisabled();
    expect(screen.queryByText(megaItemText.compareDisabledReason)).toBeNull();
  });

  test("攻撃側だけがメガなら比較はそのまま使える(比較は防御側の持ち物の話)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    expect(compareToggle()).not.toBeDisabled();
  });

  test("メガストーンは比較の候補(防御側の持ち物の通り)に混ざらない(効果が防御を上げるものでも)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await user.click(compareToggle());

    await waitFor(() => {
      expect(lastRequest(engine).itemVariants?.length ?? 0).toBeGreaterThan(1);
    });
    const ids = (lastRequest(engine).itemVariants ?? []).map((item) => item?.id ?? null);
    expect(ids).not.toContain(MEGA_FIRE_STONE.id);
    expect(ids).not.toContain(MEGA_WATER_STONE.id);
  });
});

describe("攻守入れ替え", () => {
  test("攻撃側がメガ・防御側が通常: 入れ替えると固定が防御側に移り、攻撃側の欄は操作できて未選択", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await user.click(swapButton());

    expect(attackerItemSelect()).not.toBeDisabled();
    expect(attackerItemSelect()).toHaveValue("");
    expect(defenderItemSelect()).toBeDisabled();
    expect(defenderItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(defenderItemSelect()).toHaveAccessibleDescription(megaItemText.lockedReason);
    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toBeNull();
      expect(lastRequest(engine).itemVariants).toEqual([MEGA_FIRE_STONE]);
    });
  });

  test("両方がメガ: 入れ替えても、それぞれが自分のメガストーンを持つ", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    await user.click(swapButton());

    expect(attackerItemSelect()).toHaveValue(MEGA_WATER_STONE.id);
    expect(defenderItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(attackerItemSelect()).toBeDisabled();
    expect(defenderItemSelect()).toBeDisabled();
  });
});

describe("メガストーンをマスタの持ち物から引けないメガ種族", () => {
  test("固定せず持ち物は空・欄は disabled で、理由(引けないこと)を伝える。要求の持ち物は null", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_ORPHAN.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    expect(attackerItemSelect()).toBeDisabled();
    expect(attackerItemSelect()).toHaveValue("");
    expect(attackerItemSelect()).toHaveAccessibleDescription(megaItemText.missingReason);
    expect(within(attackerCard()).getByText(megaItemText.missingReason)).toBeVisible();
    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toBeNull();
    });
  });
});

describe("メガでない種族の挙動は変わらない", () => {
  test("メガ種族を一度も選ばなければ、固定の文言は出ず、欄は操作できる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    expect(attackerItemSelect()).not.toBeDisabled();
    expect(defenderItemSelect()).not.toBeDisabled();
    expect(screen.queryByText(megaItemText.lockedReason)).toBeNull();
    expect(screen.queryByText(megaItemText.compareDisabledReason)).toBeNull();
    expect(compareToggle()).not.toBeDisabled();
  });
});

describe("種族を検索で解決するマスタ(オンライン・キャッシュ済みのオフライン。isMega は解決後の種族から読む)", () => {
  const SEARCH_ONLY: MasterCapabilities = { speciesList: false, moves: true, effects: true };

  function fakeSearch(): MasterSpeciesSearch {
    return createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
  }

  function renderSearchScreen(): { user: UserEvent; engine: FakeEngine } {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const engine = createFakeEngine();
    render(
      <CalcScreen engine={engine} master={limitedMaster(master, SEARCH_ONLY)} masterSearch={fakeSearch()} />,
    );
    return { user, engine };
  }

  async function chooseBySearch(
    user: UserEvent,
    card: HTMLElement,
    label: string,
    species: MasterSpecies,
  ): Promise<void> {
    const input = within(card).getByRole("combobox", { name: label });
    // 選び直しのときは、いまの名前を消してから打つ(入力欄は選んだ名前に置き換わっている)。
    await user.clear(input);
    await user.type(input, species.nameJa);
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await within(card).findByRole("option", { name: species.nameJa }));
  }

  test("検索でメガ種族を選ぶと、持ち物が固定される(disabled・名前・理由)", async () => {
    const { user } = renderSearchScreen();
    await chooseBySearch(user, attackerCard(), "攻撃側のポケモン", MEGA_FIRE);

    const select = await within(attackerCard()).findByRole("combobox", { name: "攻撃側の持ち物" });
    expect(select).toBeDisabled();
    expect(select).toHaveDisplayValue(MEGA_FIRE_STONE.nameJa);
    expect(select).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("検索で選んだメガ種族の要求に、固定した持ち物が載る", async () => {
    const { user, engine } = renderSearchScreen();
    await chooseBySearch(user, attackerCard(), "攻撃側のポケモン", MEGA_FIRE);
    await chooseBySearch(user, defenderCard(), "防御側のポケモン", MEGA_WATER);

    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toEqual(MEGA_FIRE_STONE);
      expect(lastRequest(engine).itemVariants).toEqual([MEGA_WATER_STONE]);
    });
  });

  test("検索でメガでない種族に変えると固定が外れ、未選択に戻る", async () => {
    const { user } = renderSearchScreen();
    await chooseBySearch(user, attackerCard(), "攻撃側のポケモン", MEGA_FIRE);
    await chooseBySearch(user, attackerCard(), "攻撃側のポケモン", normalSpecies(0));

    const select = await within(attackerCard()).findByRole("combobox", { name: "攻撃側の持ち物" });
    expect(select).not.toBeDisabled();
    expect(select).toHaveValue("");
  });

  // 種族の全件一覧が無いマスタでは、メガストーンの集合は「いままでに解決した種族」から導く
  // (ADR-0320。公開 API に持ち物がメガストーンかを示す項目が無いため。一覧が全件そろうマスタでは最初から全件)。
  test("解決済みのメガ種族のメガストーンは、もう一方のカード(メガでない種族)の選択肢にも出ない", async () => {
    const { user } = renderSearchScreen();
    await chooseBySearch(user, attackerCard(), "攻撃側のポケモン", MEGA_FIRE);
    await chooseBySearch(user, defenderCard(), "防御側のポケモン", normalSpecies(1));

    const select = await within(defenderCard()).findByRole("combobox", { name: "防御側の持ち物" });
    expect(optionNames(select)).not.toContain(MEGA_FIRE_STONE.nameJa);
    expect(select).not.toBeDisabled();
  });
});
