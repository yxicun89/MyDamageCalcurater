// ADR-0326(ADR-0175 §4): 計算画面の持ち物欄を役割で絞り、メガストーンを日本語で表示する。
// マスタは架空の例データ + 架空のメガ種族 + 役割つきの持ち物(test/itemRolesMaster.ts)。engine は fake。
// 確かめること:
//   - 攻撃側の欄は attacker の持ち物だけ、防御側の欄は defender の持ち物だけ(先頭は「持ち物なし」)。
//     役割の無い持ち物・メガストーン(まだ解決していないメガ種族のものも)はどちらにも出ない
//   - メガ種族を選ぶと欄は固定され、表示はマスタの nameJa(日本語名。英語名などはフォールバック「{基本種名}のメガストーン」。ADR-0328)。
//     基本種名が無いメガ種族は「メガストーン」。固定の理由は見える文言と aria-describedby の両方
//   - 要求の持ち物は固定したストーン(engine の Item の形。roles・isMegaStone は載らない)
//   - 攻守入れ替え: 両方の役割を持つ持ち物は保つ。合わなくなった持ち物は「持ち物なし」に戻し、role="status" で
//     通知して欄の aria-describedby に結ぶ。持ち物を選び直す・種族を変えると通知は消える
//   - 「持ち物の候補も比較」の候補は防御側の役割の持ち物だけ
//   - roles の無いマスタ(古いサーバー・例データ)は従来どおり絞らない(CalcScreen.mega.test.tsx が既存の挙動を確かめる)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BulkRequest } from "../engine/types";
import { calcScreenText, megaItemText } from "../i18n/ja";
import { itemRoleText } from "../i18n/items";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { bulkRow, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import {
  BOTH_ITEM,
  DEF_ITEM,
  EXPECTED_ATTACKER_ITEMS,
  EXPECTED_DEFENDER_ITEMS,
  NO_ROLE_ITEM,
  POWER_ITEM,
  ROLE_MEGA_FIRE_STONE,
  UNRESOLVED_MEGA_STONE,
  withItemRoles,
} from "../test/itemRolesMaster";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_FIRE_STONE_LABEL,
  MEGA_WATER,
  MEGA_WATER_STONE,
  UNNAMED_MEGA_STONE_LABEL,
} from "../test/megaMaster";
import { CalcScreen } from "./CalcScreen";

let example: MasterData;
let master: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
  master = withItemRoles(example);
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

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .queryAllByRole("option")
    .map((option) => option.textContent);
}

function lastRequest(engine: FakeEngine): BulkRequest {
  const request = engine.bulkRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return request;
}

function renderScreen(
  data: MasterData = master,
  engine: FakeEngine = createFakeEngine(),
): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={data} />);
  return { user, engine };
}

/** 通知(role="status")。文言がそのまま status 要素の中にあること。 */
function expectStatus(container: HTMLElement, text: string): void {
  const found = within(container).getByText(text);
  expect(found).toHaveAttribute("role", "status");
}

describe("持ち物の選択肢はその側の役割の持ち物だけ", () => {
  test("攻撃側の欄は「持ち物なし」+ attacker の持ち物(マスタの順)", () => {
    renderScreen();
    expect(optionNames(attackerItemSelect())).toEqual([
      calcScreenText.noItemOption,
      ...EXPECTED_ATTACKER_ITEMS.map((item) => item.nameJa),
    ]);
  });

  test("防御側の欄は「持ち物なし」+ defender の持ち物(マスタの順)", () => {
    renderScreen();
    expect(optionNames(defenderItemSelect())).toEqual([
      calcScreenText.noItemOption,
      ...EXPECTED_DEFENDER_ITEMS.map((item) => item.nameJa),
    ]);
  });

  test("役割の無い持ち物とメガストーン(解決していないメガ種族のものも)は、どちらの欄にも出ない", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    for (const select of [attackerItemSelect(), defenderItemSelect()]) {
      const names = optionNames(select);
      expect(names).not.toContain(NO_ROLE_ITEM.nameJa);
      expect(names).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
      expect(names).not.toContain(UNRESOLVED_MEGA_STONE.nameJa);
    }
  });

  test("選んだ持ち物は engine の Item の形で要求に載る(roles・isMegaStone を載せない)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerItemSelect(), POWER_ITEM.id);
    await user.selectOptions(defenderItemSelect(), DEF_ITEM.id);

    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toEqual({
        id: POWER_ITEM.id,
        nameJa: POWER_ITEM.nameJa,
        effect: POWER_ITEM.effect,
      });
    });
    expect(lastRequest(engine).attacker.item).not.toHaveProperty("roles");
    expect(lastRequest(engine).itemVariants).toEqual([
      { id: DEF_ITEM.id, nameJa: DEF_ITEM.nameJa, effect: DEF_ITEM.effect },
    ]);
  });
});

describe("メガ種族の固定中の表示", () => {
  test("攻撃側: ストーンの nameJa を見せ、理由を aria-describedby で結ぶ", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);

    const select = attackerItemSelect();
    expect(select).toBeDisabled();
    expect(select).toHaveValue(MEGA_FIRE_STONE.id);
    expect(select).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(optionNames(select)).toContain(MEGA_FIRE_STONE_LABEL);
    expect(select).toHaveAccessibleDescription(megaItemText.lockedReason);
    expect(within(attackerCard()).getByText(megaItemText.lockedReason)).toBeVisible();
  });

  test("防御側も同じ(防御側の役割に合わないストーンでも、固定は全件から引く)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(defenderSpeciesSelect(), MEGA_FIRE.key);

    expect(defenderItemSelect()).toBeDisabled();
    expect(defenderItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(defenderItemSelect()).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
  });

  test("ストーンの nameJa が英語で基本種名も無いメガ種族は「メガストーン」だけを見せる(名前を推測しない)", async () => {
    const unnamed: MasterSpecies = { ...MEGA_WATER, baseSpeciesNameJa: null };
    const data: MasterData = {
      ...master,
      items: master.items.map((item) =>
        item.id === MEGA_WATER_STONE.id ? { ...item, nameJa: "Examplite W" } : item,
      ),
      species: master.species.map((species) => (species.key === MEGA_WATER.key ? unnamed : species)),
    };
    const { user } = renderScreen(data);
    await user.selectOptions(attackerSpeciesSelect(), MEGA_WATER.key);

    expect(attackerItemSelect()).toHaveDisplayValue(UNNAMED_MEGA_STONE_LABEL);
  });

  test("要求の持ち物は固定したストーン(engine の Item の形)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toEqual(MEGA_FIRE_STONE);
    });
    expect(lastRequest(engine).attacker.item).not.toHaveProperty("isMegaStone");
  });

  test("メガでない種族に変えると固定が外れ、選択肢は攻撃側の役割の持ち物に戻る", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);

    expect(attackerItemSelect()).toBeEnabled();
    expect(attackerItemSelect()).toHaveValue("");
    expect(optionNames(attackerItemSelect())).toEqual([
      calcScreenText.noItemOption,
      ...EXPECTED_ATTACKER_ITEMS.map((item) => item.nameJa),
    ]);
  });
});

describe("攻守入れ替え", () => {
  test("両方の役割を持つ持ち物は、入れ替えても保つ(通知なし)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerItemSelect(), BOTH_ITEM.id);

    await user.click(swapButton());

    expect(defenderItemSelect()).toHaveValue(BOTH_ITEM.id);
    expect(screen.queryByText(itemRoleText.droppedNotice(BOTH_ITEM.nameJa, "defender"))).toBeNull();
  });

  test("役割に合わなくなった持ち物は「持ち物なし」に戻し、role=status で通知して欄の説明に結ぶ", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerItemSelect(), POWER_ITEM.id);
    await user.selectOptions(defenderItemSelect(), DEF_ITEM.id);

    await user.click(swapButton());

    // 攻撃側にいた種族(いまは防御側)の持ち物 POWER_ITEM は防御側では効かない。
    expect(defenderItemSelect()).toHaveValue("");
    expect(defenderItemSelect()).toHaveDisplayValue(calcScreenText.noItemOption);
    const defenderNotice = itemRoleText.droppedNotice(POWER_ITEM.nameJa, "defender");
    expectStatus(defenderCard(), defenderNotice);
    expect(defenderItemSelect()).toHaveAccessibleDescription(defenderNotice);
    // 防御側にいた種族(いまは攻撃側)の持ち物 DEF_ITEM は攻撃側では効かない。
    expect(attackerItemSelect()).toHaveValue("");
    const attackerNotice = itemRoleText.droppedNotice(DEF_ITEM.nameJa, "attacker");
    expectStatus(attackerCard(), attackerNotice);
    expect(attackerItemSelect()).toHaveAccessibleDescription(attackerNotice);

    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toBeNull();
    });
    expect(lastRequest(engine).itemVariants).toBeUndefined();
  });

  test("持ち物を選び直すと、その欄の通知は消える", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerItemSelect(), POWER_ITEM.id);
    await user.click(swapButton());
    const text = itemRoleText.droppedNotice(POWER_ITEM.nameJa, "defender");
    expect(within(defenderCard()).getByText(text)).toBeVisible();

    await user.selectOptions(defenderItemSelect(), DEF_ITEM.id);

    expect(within(defenderCard()).queryByText(text)).toBeNull();
    expect(defenderItemSelect()).not.toHaveAccessibleDescription(text);
  });

  test("種族を変えると、その欄の通知は消える", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(attackerItemSelect(), POWER_ITEM.id);
    await user.click(swapButton());
    const text = itemRoleText.droppedNotice(POWER_ITEM.nameJa, "defender");

    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(2).key);

    expect(within(defenderCard()).queryByText(text)).toBeNull();
  });

  test("メガ種族の固定は入れ替えても保たれ、通知は出ない(ストーンは役割で外さない)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await user.click(swapButton());

    expect(defenderItemSelect()).toBeDisabled();
    expect(defenderItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(defenderItemSelect()).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(within(defenderCard()).queryByRole("status")).toBeNull();
  });
});

describe("「持ち物の候補も比較」", () => {
  test("候補は防御側の役割の持ち物だけ(攻撃側の持ち物・役割なし・メガストーンは混ざらない)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), normalSpecies(1).key);

    await user.click(compareToggle());

    await waitFor(() => {
      expect(lastRequest(engine).itemVariants?.length ?? 0).toBeGreaterThan(1);
    });
    const variantIds = (lastRequest(engine).itemVariants ?? []).map((item) => item?.id ?? null);
    const allowed = new Set<string | null>([null, ...EXPECTED_DEFENDER_ITEMS.map((item) => item.id)]);
    expect(variantIds.every((id) => allowed.has(id))).toBe(true);
    expect(variantIds).not.toContain(POWER_ITEM.id);
    expect(variantIds).not.toContain(NO_ROLE_ITEM.id);
    expect(variantIds).not.toContain(ROLE_MEGA_FIRE_STONE.id);
  });
});

describe("結果の行・未対応の印にメガストーンの英語名を出さない", () => {
  test("防御側がメガ種族なら、結果の各行の持ち物は「{基本種名}のメガストーン」", async () => {
    const { user } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_FIRE.key);

    const list = await screen.findByRole("list", { name: calcScreenText.resultsListLabel });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(within(row).getByText(MEGA_FIRE_STONE_LABEL)).toBeVisible();
    }
  });

  test("未対応の印の持ち物名も、ストーンはマスタの nameJa", async () => {
    const stoneMark = {
      target: "defender_item",
      reason: "unsupported_effect",
      id: MEGA_FIRE_STONE.id,
    } as const;
    const engine = createFakeEngine((request) =>
      ok({
        defenderSpeciesKey: request.defenderSpecies.key,
        rows: [bulkRow({ itemId: MEGA_FIRE_STONE.id, unsupported: [stoneMark] })],
      }),
    );
    const { user } = renderScreen(master, engine);
    await user.selectOptions(attackerSpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_FIRE.key);

    const matches = await screen.findAllByText(new RegExp(MEGA_FIRE_STONE_LABEL));
    expect(matches.some((element) => element.closest('[role="status"]') !== null)).toBe(true);
  });
});
