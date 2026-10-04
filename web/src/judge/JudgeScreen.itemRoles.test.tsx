// ADR-0326(ADR-0175 §4): 判定画面の持ち物欄を役割で絞り、メガストーンを日本語で表示する。
// 判定は自分と相手の候補が互いに攻撃し合う(双方向の確定数)ので、どの個体の欄も「攻撃・防御のどちらかの役割を持つ
// 持ち物」(either)を出す。
// 確かめること:
//   - 自分・相手の候補の持ち物欄は「未選択」+ either の持ち物(マスタの順)。役割の無い持ち物・メガストーンは出ない
//   - メガ種族の固定中は「{基本種名}のメガストーン」、基本種名が無ければ「メガストーン」。理由は aria-describedby
//   - roles の無いマスタは従来どおり絞らない

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { Ability, Move, TypeChart } from "../engine/types";
import { judgeScreenText, megaItemText } from "../i18n/ja";
import type { MasterData, MasterItem, MasterNature, MasterSpecies } from "../master/types";
import {
  EXPECTED_EITHER_ITEMS,
  NO_ROLE_ITEM,
  ROLE_ITEMS,
  ROLE_MEGA_FIRE_STONE,
  UNRESOLVED_MEGA_STONE,
  withoutRoleFields,
} from "../test/itemRolesMaster";
import { JudgeScreen } from "./JudgeScreen";
import type { components } from "./judge.gen";
import type { JudgeClient, JudgeResult } from "./judgeClient";

type Schemas = components["schemas"];

function createFakeJudgeClient(): JudgeClient {
  return {
    outspeedAndKo() {
      return new Promise<JudgeResult<Schemas["OutspeedAndKoResponse"]>>(() => undefined);
    },
  };
}

const typeChart: TypeChart = { types: ["fire", "water"], effectiveness: {} };
const MOVE: Move = {
  id: "test-move-phys",
  nameJa: "テストぶつりわざ",
  type: "normal",
  category: "physical",
  power: 80,
  priority: 0,
};
const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };

function species(key: string, nameJa: string, extra: Partial<MasterSpecies> = {}): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: Number(key.slice(5)),
    nameJa,
    types: ["fire"],
    baseStats: { hp: 70, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: [ABILITY.id],
    learnset: [MOVE.id],
    ...extra,
  };
}

const NORMAL = species("9001-000", "テストカソウドリ");
const MEGA = species("9101-001", "メガテストカソウドリ", {
  isMega: true,
  requiredItemId: ROLE_MEGA_FIRE_STONE.id,
  baseSpeciesKey: NORMAL.key,
  baseSpeciesNameJa: NORMAL.nameJa,
});
const MEGA_UNNAMED = species("9102-001", "メガテストカソウギョ", {
  isMega: true,
  requiredItemId: ROLE_MEGA_FIRE_STONE.id,
  baseSpeciesKey: null,
  baseSpeciesNameJa: null,
});

function masterWith(items: readonly MasterItem[]): MasterData {
  return {
    species: [NORMAL, MEGA, MEGA_UNNAMED],
    moves: [MOVE],
    items,
    abilities: [ABILITY],
    natures: [NATURE],
    typeChart,
    capabilities: { speciesList: true, moves: true, effects: true },
  };
}

function renderScreen(data: MasterData = masterWith(ROLE_ITEMS)): { user: UserEvent } {
  const user = userEvent.setup();
  render(<JudgeScreen judgeClient={createFakeJudgeClient()} master={data} />);
  return { user };
}

const attackerRegion = () => screen.getByRole("region", { name: judgeScreenText.attackerRegionLabel });
const candidate = (n: number) =>
  within(screen.getByRole("region", { name: judgeScreenText.defendersRegionLabel })).getByRole("group", {
    name: judgeScreenText.candidateGroupLabel(n),
  });
const itemSelect = (region: HTMLElement) =>
  within(region).getByRole("combobox", { name: judgeScreenText.itemLabel });
const speciesSelect = (region: HTMLElement) => within(region).getByLabelText(judgeScreenText.speciesLabel);

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);
}

describe("持ち物欄は攻撃・防御のどちらかの役割を持つ持ち物だけ", () => {
  test.each([
    ["自分", attackerRegion],
    ["相手の候補", () => candidate(1)],
  ])("%s", async (_label, region) => {
    const { user } = renderScreen();
    await user.selectOptions(speciesSelect(region()), NORMAL.key);

    const names = optionNames(itemSelect(region()));
    expect(names).toEqual([
      judgeScreenText.unselectedOption,
      ...EXPECTED_EITHER_ITEMS.map((item) => item.nameJa),
    ]);
    expect(names).not.toContain(NO_ROLE_ITEM.nameJa);
    expect(names).not.toContain(UNRESOLVED_MEGA_STONE.nameJa);
  });

  test("roles の無いマスタ(古いサーバー)は絞らない(メガストーンだけ外す)", async () => {
    const legacy = ROLE_ITEMS.map(withoutRoleFields);
    const { user } = renderScreen(masterWith(legacy));
    await user.selectOptions(speciesSelect(attackerRegion()), NORMAL.key);

    const names = optionNames(itemSelect(attackerRegion()));
    expect(names).toContain(NO_ROLE_ITEM.nameJa);
    expect(names).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
  });
});

describe("メガ種族の固定中の表示", () => {
  test.each([
    ["自分", attackerRegion],
    ["相手の候補", () => candidate(1)],
  ])("%s: マスタの nameJa(日本語名)、理由は aria-describedby", async (_label, region) => {
    const { user } = renderScreen();
    await user.selectOptions(speciesSelect(region()), MEGA.key);

    const item = itemSelect(region());
    expect(item).toBeDisabled();
    expect(item).toHaveValue(ROLE_MEGA_FIRE_STONE.id);
    expect(item).toHaveDisplayValue(ROLE_MEGA_FIRE_STONE.nameJa);
    expect(item).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("基本種名が無いメガ種族でも、日本語の nameJa はそのまま(ADR-0328)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(speciesSelect(attackerRegion()), MEGA_UNNAMED.key);

    expect(itemSelect(attackerRegion())).toHaveDisplayValue(ROLE_MEGA_FIRE_STONE.nameJa);
  });
});
