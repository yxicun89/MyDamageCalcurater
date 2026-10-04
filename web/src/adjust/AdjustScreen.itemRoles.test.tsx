// ADR-0326(ADR-0175 §4): 調整画面の自分の持ち物欄を役割で絞る。
// 調整は火力指数と耐久指数を両方出し、モードを切り替えても入力を消さない(ADR-0319 §2)ので、モードに依らず
// 「攻撃・防御のどちらかの役割を持つ持ち物」(either)を出す。
// 確かめること:
//   - 自分の持ち物欄は「未選択」+ either の持ち物(マスタの順)。役割の無い持ち物・メガストーンは出ない
//   - モードを切り替えても選択肢は変わらず、選んだ持ち物は保つ
//   - roles の無いマスタ(古いサーバー)は絞らない(メガストーンは isMegaStone が無ければ種族から判別して外す)

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { Ability, Move } from "../engine/types";
import { adjustScreenText } from "../i18n/ja";
import type { MasterData, MasterItem, MasterNature, MasterSpecies } from "../master/types";
import {
  BOTH_ITEM,
  EXPECTED_EITHER_ITEMS,
  NO_ROLE_ITEM,
  ROLE_ITEMS,
  ROLE_MEGA_FIRE_STONE,
  UNRESOLVED_MEGA_STONE,
  withoutRoleFields,
} from "../test/itemRolesMaster";
import { AdjustScreen } from "./AdjustScreen";
import type { AdjustClient } from "./adjustClient";

const T = adjustScreenText;

function pendingClient(): AdjustClient {
  const never = () => new Promise<never>(() => undefined);
  return {
    indices: never,
    minSpToKo: never,
    minSpToSurvive: never,
    allocation: never,
    moveLearners: never,
  };
}

const MOVE: Move = {
  id: "test-move-fire",
  nameJa: "テストほのおわざ",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
};
const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };
const BIRD: MasterSpecies = {
  key: "9001-000",
  dexNo: 9001,
  form: 0,
  nameJa: "テストカソウドリ",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
  abilities: [ABILITY.id],
  learnset: [MOVE.id],
};
/** ROLE_MEGA_FIRE_STONE を requiredItemId に持つメガ種族(roles の無いマスタでストーンを判別するため)。 */
const MEGA_BIRD: MasterSpecies = {
  ...BIRD,
  key: "9101-001",
  dexNo: 9101,
  form: 1,
  nameJa: "メガテストカソウドリ",
  isMega: true,
  requiredItemId: ROLE_MEGA_FIRE_STONE.id,
};

function masterWith(items: readonly MasterItem[]): MasterData {
  return {
    species: [BIRD, MEGA_BIRD],
    moves: [MOVE],
    items,
    abilities: [ABILITY],
    natures: [NATURE],
    typeChart: { types: ["fire"], effectiveness: {} },
    capabilities: { speciesList: true, moves: true, effects: true },
  };
}

const itemSelect = () => screen.getByRole("combobox", { name: T.selfItemLabel });

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);
}

describe("自分の持ち物欄は攻撃・防御のどちらかの役割を持つ持ち物だけ", () => {
  test("「未選択」+ either の持ち物(マスタの順)", () => {
    render(<AdjustScreen adjustClient={pendingClient()} master={masterWith(ROLE_ITEMS)} />);
    const names = optionNames(itemSelect());
    expect(names).toEqual([T.unselectedOption, ...EXPECTED_EITHER_ITEMS.map((item) => item.nameJa)]);
    expect(names).not.toContain(NO_ROLE_ITEM.nameJa);
    expect(names).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
    expect(names).not.toContain(UNRESOLVED_MEGA_STONE.nameJa);
  });

  test("モードを切り替えても選択肢は変わらず、選んだ持ち物は保つ", async () => {
    const user = userEvent.setup();
    render(<AdjustScreen adjustClient={pendingClient()} master={masterWith(ROLE_ITEMS)} />);
    await user.selectOptions(itemSelect(), BOTH_ITEM.id);
    const before = optionNames(itemSelect());

    const modes = within(screen.getByRole("radiogroup", { name: T.modeGroupLabel })).getAllByRole("radio");
    for (const mode of modes) {
      await user.click(mode);
      expect(optionNames(itemSelect())).toEqual(before);
      expect(itemSelect()).toHaveValue(BOTH_ITEM.id);
    }
  });

  test("roles の無いマスタ(古いサーバー)は絞らない(メガストーンは種族から判別して外す)", () => {
    const legacy = ROLE_ITEMS.map(withoutRoleFields);
    render(<AdjustScreen adjustClient={pendingClient()} master={masterWith(legacy)} />);
    const names = optionNames(itemSelect());
    expect(names).toContain(NO_ROLE_ITEM.nameJa);
    expect(names).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
  });
});
