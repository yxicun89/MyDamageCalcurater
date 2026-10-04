// ADR-0326(ADR-0175 §4): 逆算画面の持ち物を役割で絞り、メガストーンを日本語で表示する。
// マスタは架空の例データ + 架空のメガ種族 + 役割つきの持ち物(test/itemRolesMaster.ts)。engine は fake。
// 逆算の side は「逆算する相手の側」。与えたダメージ(side defender)では自分が攻撃側、受けたダメージ(side attacker)では
// 自分が防御側(domain/itemRoles.ts の reverseMyItemRole)。
// 確かめること:
//   - 自分の持ち物欄: 与えたダメージでは attacker の持ち物、受けたダメージでは defender の持ち物だけ(先頭は「持ち物なし」)
//   - 観測した側を切り替えて自分の持ち物が合わなくなったら「持ち物なし」に戻し、role="status" で通知して欄の説明に結ぶ
//   - 相手の持ち物候補(itemCandidates)は、相手の側の役割の持ち物だけ(役割なし・メガストーンは混ざらない)
//   - メガ種族の固定中の表示は「{基本種名}のメガストーン」(自分の欄・相手のカードの文の両方)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import type { ReverseRequest } from "../engine/types";
import { calcScreenText, megaItemText, reverseScreenText } from "../i18n/ja";
import { itemRoleText } from "../i18n/items";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterItem, MasterSpecies } from "../master/types";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import {
  BOTH_ITEM,
  EXPECTED_ATTACKER_ITEMS,
  EXPECTED_DEFENDER_ITEMS,
  NO_ROLE_ITEM,
  POWER_ITEM,
  ROLE_MEGA_FIRE_STONE,
  ROLE_MEGA_WATER_STONE,
  UNRESOLVED_MEGA_STONE,
  withItemRoles,
} from "../test/itemRolesMaster";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_FIRE_STONE_LABEL,
  MEGA_WATER,
  MEGA_WATER_STONE_LABEL,
} from "../test/megaMaster";
import { ReverseScreen } from "./ReverseScreen";

let example: MasterData;
let master: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
  master = withItemRoles(example);
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

const sideGroup = () => screen.getByRole("radiogroup", { name: "観測したダメージ" });
const receivedRadio = () => within(sideGroup()).getByRole("radio", { name: "受けたダメージ" });
const dealtRadio = () => within(sideGroup()).getByRole("radio", { name: "与えたダメージ" });
const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const myItemSelect = () => screen.getByRole("combobox", { name: "自分の持ち物" });
const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });
const observationInput = (n: number) => screen.getByRole("textbox", { name: `観測${String(n)}` });

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .queryAllByRole("option")
    .map((option) => option.textContent);
}

function lastRequest(engine: FakeEngine): ReverseRequest {
  const request = engine.reverseRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcReverse が呼ばれていない");
  }
  return request;
}

async function typeObservation(user: UserEvent, n: number, text: string): Promise<void> {
  await user.type(observationInput(n), text);
  act(() => {
    vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
  });
}

function renderScreen(data: MasterData = master): { user: UserEvent; engine: FakeEngine } {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const engine = createFakeEngine();
  render(<ReverseScreen engine={engine} master={data} />);
  return { user, engine };
}

const namesOf = (items: readonly MasterItem[]): string[] => items.map((item) => item.nameJa);

describe("自分の持ち物欄は、観測した側で決まる役割の持ち物だけ", () => {
  test("与えたダメージ(既定): 自分は攻撃側なので attacker の持ち物", () => {
    renderScreen();
    expect(dealtRadio()).toBeChecked();
    expect(optionNames(myItemSelect())).toEqual([
      calcScreenText.noItemOption,
      ...namesOf(EXPECTED_ATTACKER_ITEMS),
    ]);
  });

  test("受けたダメージ: 自分は防御側なので defender の持ち物", async () => {
    const { user } = renderScreen();
    await user.click(receivedRadio());
    expect(optionNames(myItemSelect())).toEqual([
      calcScreenText.noItemOption,
      ...namesOf(EXPECTED_DEFENDER_ITEMS),
    ]);
  });

  test("役割の無い持ち物・メガストーンはどちらでも出ない", async () => {
    const { user } = renderScreen();
    for (const radio of [dealtRadio, receivedRadio]) {
      await user.click(radio());
      const names = optionNames(myItemSelect());
      expect(names).not.toContain(NO_ROLE_ITEM.nameJa);
      expect(names).not.toContain(ROLE_MEGA_FIRE_STONE.nameJa);
      expect(names).not.toContain(UNRESOLVED_MEGA_STONE.nameJa);
    }
  });
});

describe("観測した側の切り替えで、自分の持ち物が合わなくなったとき", () => {
  test("「持ち物なし」に戻し、role=status で通知して欄の説明に結ぶ", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(myItemSelect(), POWER_ITEM.id);

    await user.click(receivedRadio());

    expect(myItemSelect()).toHaveValue("");
    const text = itemRoleText.droppedNotice(POWER_ITEM.nameJa, "defender");
    const notice = within(myCard()).getByText(text);
    expect(notice).toHaveAttribute("role", "status");
    expect(myItemSelect()).toHaveAccessibleDescription(text);
  });

  test("両方の役割を持つ持ち物は保つ(通知なし)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(myItemSelect(), BOTH_ITEM.id);

    await user.click(receivedRadio());

    expect(myItemSelect()).toHaveValue(BOTH_ITEM.id);
    expect(within(myCard()).queryByRole("status")).toBeNull();
  });

  test("要求の known.item は null(外した持ち物を送らない)", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(theirSpeciesSelect(), normalSpecies(1).key);
    await user.selectOptions(myItemSelect(), POWER_ITEM.id);

    await user.click(receivedRadio());
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).side).toBe("attacker");
    expect(lastRequest(engine).known.item).toBeNull();
  });
});

describe("相手の持ち物候補は、相手の側の役割の持ち物だけ", () => {
  test.each([
    ["与えたダメージ(相手は防御側)", "defender", EXPECTED_DEFENDER_ITEMS],
    ["受けたダメージ(相手は攻撃側)", "attacker", EXPECTED_ATTACKER_ITEMS],
  ] as const)("%s", async (_label, side, allowedItems) => {
    const { user, engine } = renderScreen();
    if (side === "attacker") {
      await user.click(receivedRadio());
    }
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(theirSpeciesSelect(), normalSpecies(1).key);
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    const request = lastRequest(engine);
    expect(request.side).toBe(side);
    const candidates = request.itemCandidates ?? [];
    const candidateIds = candidates.map((item) => item?.id ?? null);
    const allowed = new Set<string | null>([null, ...allowedItems.map((item) => item.id)]);
    expect(candidateIds.every((id) => allowed.has(id))).toBe(true);
    expect(candidateIds).not.toContain(NO_ROLE_ITEM.id);
    expect(candidateIds).not.toContain(ROLE_MEGA_WATER_STONE.id);
    expect(candidateIds).not.toContain(UNRESOLVED_MEGA_STONE.id);
    // 候補は engine の Item の形(roles・isMegaStone を載せない)。
    for (const item of candidates) {
      if (item !== null) {
        expect(item).not.toHaveProperty("roles");
      }
    }
  });
});

describe("メガ種族の固定中の表示", () => {
  test("自分: 欄はストーンの nameJa、理由は aria-describedby", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), MEGA_FIRE.key);

    expect(myItemSelect()).toBeDisabled();
    expect(myItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(myItemSelect()).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(myItemSelect()).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("自分のメガの固定は、観測した側を切り替えても保たれ、通知は出ない", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), MEGA_FIRE.key);

    await user.click(receivedRadio());

    expect(myItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(within(myCard()).queryByRole("status")).toBeNull();
  });

  test("相手: カードの文は「持ち物: {ストーンの nameJa}」", async () => {
    const { user } = renderScreen();
    await user.selectOptions(theirSpeciesSelect(), MEGA_WATER.key);

    expect(within(theirCard()).getByText(megaItemText.fixedItemName(MEGA_WATER_STONE_LABEL))).toBeVisible();
    expect(within(theirCard()).getByText(megaItemText.lockedReason)).toBeVisible();
  });
});

describe("結果の候補の行にメガストーンの英語名を出さない", () => {
  test("相手がメガ種族なら、候補の行の持ち物は「{基本種名}のメガストーン」", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);
    await user.selectOptions(theirSpeciesSelect(), MEGA_WATER.key);
    await typeObservation(user, 1, "45");

    const list = await screen.findByRole("list", { name: reverseScreenText.resultsListLabel });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(within(row).getByText(MEGA_WATER_STONE_LABEL)).toBeVisible();
    }
  });
});
