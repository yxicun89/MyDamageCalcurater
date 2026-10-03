// issue #515・ADR-0320 PR-B(docs/mega-evolution-spec.md §4-3): 判定画面の個体入力(自分・相手の候補)のメガシンカの持ち物固定。
// JudgeClient は fake。マスタは架空(9xxx の種族・test-* の ID)。技の入力は対象外(持ち物のみ)。
// 確かめること:
//   M1 メガ種族を選ぶと持ち物がストーンに固定される(disabled・ストーンの名前・理由の見える文言と aria-describedby)。自分・相手の両方
//   M2 非メガに変えると解除して未選択。メガストーンは単独の持ち物の選択肢に出ない
//   M3 要求(judgeClient)の itemId にストーンが載る(自分 attacker・相手 defenders)。非メガは従来どおり(選ばなければ送らない)
//   M4 ストーンがマスタに無いメガは、固定せず空にして理由を出す
//   M5 種族を検索で解決するマスタでも同じに動く

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { Ability, Item, Move, TypeChart } from "../engine/types";
import { judgeScreenText, megaItemText } from "../i18n/ja";
import { ONLINE_MASTER_CAPABILITIES, SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { createFakeSpeciesSearch } from "../test/onlineMaster";
import { JudgeScreen } from "./JudgeScreen";
import type { components } from "./judge.gen";
import type { JudgeClient, JudgeResult } from "./judgeClient";

type Schemas = components["schemas"];

interface PendingCall {
  readonly args: Schemas["OutspeedAndKoRequest"];
  resolve(result: JudgeResult<Schemas["OutspeedAndKoResponse"]>): void;
}

function createFakeJudgeClient(): JudgeClient & { readonly calls: PendingCall[] } {
  const calls: PendingCall[] = [];
  return {
    calls,
    outspeedAndKo(request) {
      return new Promise((resolve) => {
        calls.push({ args: structuredClone(request), resolve });
      });
    },
  };
}

const emptyTypeChart: TypeChart = { types: ["fire", "water"], effectiveness: {} };
const MOVE: Move = {
  id: "test-move-phys",
  nameJa: "テストぶつりわざ",
  type: "normal",
  category: "physical",
  power: 80,
  priority: 0,
};
const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };
const ITEM: Item = { id: "test-item-a", nameJa: "テストもちもの", effect: null };
const STONE: Item = { id: "test-megastone-a", nameJa: "テストメガナイト", effect: null };
const MISSING_STONE_ID = "test-megastone-missing";
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
const OTHER = species("9002-000", "テストカソウギョ");
const MEGA = species("9101-001", "メガテストカソウドリ", { isMega: true, requiredItemId: STONE.id });
const ORPHAN = species("9103-001", "メガテストカソウムシ", {
  isMega: true,
  requiredItemId: MISSING_STONE_ID,
});

const master: MasterData = {
  species: [NORMAL, OTHER, MEGA, ORPHAN],
  moves: [MOVE],
  items: [ITEM, STONE],
  abilities: [ABILITY],
  natures: [NATURE],
  typeChart: emptyTypeChart,
  capabilities: { speciesList: true, moves: true, effects: false },
};

afterEach(() => {
  vi.useRealTimers();
});

function renderScreen(data: MasterData = master) {
  const user = userEvent.setup();
  const client = createFakeJudgeClient();
  render(<JudgeScreen judgeClient={client} master={data} />);
  return { user, client };
}

const attackerRegion = () => screen.getByRole("region", { name: judgeScreenText.attackerRegionLabel });
const candidate = (n: number) =>
  within(screen.getByRole("region", { name: judgeScreenText.defendersRegionLabel })).getByRole("group", {
    name: judgeScreenText.candidateGroupLabel(n),
  });

function speciesSelect(region: HTMLElement): HTMLElement {
  return within(region).getByLabelText(judgeScreenText.speciesLabel);
}
function itemSelect(region: HTMLElement): HTMLElement {
  return within(region).getByRole("combobox", { name: judgeScreenText.itemLabel });
}
function optionNames(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);
}

async function pick(user: UserEvent, region: HTMLElement, key: string): Promise<void> {
  await user.selectOptions(speciesSelect(region), key);
}

describe("M1 メガ種族を選ぶと持ち物がストーンに固定される", () => {
  test.each([
    ["自分", attackerRegion],
    ["相手の候補", () => candidate(1)],
  ])("%s: disabled・ストーンの名前・理由の見える文言と aria-describedby", async (_label, region) => {
    const { user } = renderScreen();
    await pick(user, region(), MEGA.key);

    const item = itemSelect(region());
    expect(item).toBeDisabled();
    expect(item).toHaveValue(STONE.id);
    expect(item).toHaveDisplayValue(STONE.nameJa);
    expect(within(region()).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(item).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("自分がメガでも、相手の持ち物欄は固定されない(個体ごと)", async () => {
    const { user } = renderScreen();
    await pick(user, attackerRegion(), MEGA.key);

    expect(itemSelect(candidate(1))).toBeEnabled();
    expect(within(candidate(1)).queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("先に別の持ち物を選んでいても、メガ種族に変えるとストーンに置き換わる", async () => {
    const { user } = renderScreen();
    await pick(user, attackerRegion(), NORMAL.key);
    await user.selectOptions(itemSelect(attackerRegion()), ITEM.id);

    await pick(user, attackerRegion(), MEGA.key);

    expect(itemSelect(attackerRegion())).toHaveValue(STONE.id);
  });
});

describe("M2 非メガに変えると解除・ストーンは単独の選択肢に出ない", () => {
  test("メガ → 非メガ: 解除して未選択", async () => {
    const { user } = renderScreen();
    await pick(user, attackerRegion(), MEGA.key);
    await pick(user, attackerRegion(), OTHER.key);

    const item = itemSelect(attackerRegion());
    expect(item).toBeEnabled();
    expect(item).toHaveValue("");
    expect(item).not.toHaveAccessibleDescription(megaItemText.lockedReason);
    expect(within(attackerRegion()).queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("持ち物の選択肢はストーンを除いたマスタの持ち物(自分・相手とも)", async () => {
    const { user } = renderScreen();
    await pick(user, attackerRegion(), NORMAL.key);
    await pick(user, candidate(1), OTHER.key);

    for (const region of [attackerRegion(), candidate(1)]) {
      expect(optionNames(itemSelect(region))).toEqual([judgeScreenText.unselectedOption, ITEM.nameJa]);
    }
  });
});

describe("M3 要求の itemId", () => {
  test("メガ種族のとき、attacker・defenders の itemId にストーンが載る", async () => {
    const { user, client } = renderScreen();
    await pick(user, attackerRegion(), MEGA.key);
    await pick(user, candidate(1), MEGA.key);
    await user.click(screen.getByRole("button", { name: judgeScreenText.submitLabel }));

    expect(client.calls).toHaveLength(1);
    const args = client.calls[0]?.args;
    expect(args?.attacker.itemId).toBe(STONE.id);
    expect(args?.defenders[0]?.itemId).toBe(STONE.id);
  });

  test("非メガは従来どおり(持ち物を選ばなければ itemId を送らない)", async () => {
    const { user, client } = renderScreen();
    await pick(user, attackerRegion(), NORMAL.key);
    await pick(user, candidate(1), OTHER.key);
    await user.click(screen.getByRole("button", { name: judgeScreenText.submitLabel }));

    const args = client.calls[0]?.args;
    expect(args?.attacker).not.toHaveProperty("itemId");
    expect(args?.defenders[0]).not.toHaveProperty("itemId");
  });

  test("メガ → 非メガに戻すと、ストーンは送られない", async () => {
    const { user, client } = renderScreen();
    await pick(user, attackerRegion(), MEGA.key);
    await pick(user, attackerRegion(), NORMAL.key);
    await pick(user, candidate(1), OTHER.key);
    await user.click(screen.getByRole("button", { name: judgeScreenText.submitLabel }));

    expect(client.calls[0]?.args.attacker).not.toHaveProperty("itemId");
  });
});

describe("M4 ストーンがマスタに無いメガ", () => {
  test("固定せず空にして、理由(missingReason)を出す。送る itemId も無い", async () => {
    const { user, client } = renderScreen();
    await pick(user, attackerRegion(), NORMAL.key);
    await user.selectOptions(itemSelect(attackerRegion()), ITEM.id);
    await pick(user, attackerRegion(), ORPHAN.key);
    await pick(user, candidate(1), OTHER.key);

    const item = itemSelect(attackerRegion());
    expect(item).toBeDisabled();
    expect(item).toHaveValue("");
    expect(within(attackerRegion()).getByText(megaItemText.missingReason)).toBeVisible();
    expect(item).toHaveAccessibleDescription(megaItemText.missingReason);

    await user.click(screen.getByRole("button", { name: judgeScreenText.submitLabel }));
    expect(client.calls[0]?.args.attacker).not.toHaveProperty("itemId");
  });
});

describe("M5 種族を検索で解決するマスタ", () => {
  const onlineMaster: MasterData = {
    ...master,
    species: [],
    moves: [],
    abilities: [],
    capabilities: ONLINE_MASTER_CAPABILITIES,
  };

  async function chooseBySearch(user: UserEvent, region: HTMLElement, target: MasterSpecies): Promise<void> {
    await user.type(
      within(region).getByRole("combobox", { name: judgeScreenText.speciesLabel }),
      target.nameJa,
    );
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await within(region).findByRole("option", { name: target.nameJa }));
  }

  test("検索でメガ種族を選ぶと固定され、非メガに選び直すと解除される。要求にストーンが載る", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const client = createFakeJudgeClient();
    const search = createFakeSpeciesSearch({
      species: [NORMAL, OTHER, MEGA, ORPHAN],
      abilities: [ABILITY],
      moves: [MOVE],
    });
    render(<JudgeScreen judgeClient={client} master={onlineMaster} masterSearch={search} />);

    await chooseBySearch(user, attackerRegion(), MEGA);
    await waitFor(() => {
      expect(itemSelect(attackerRegion())).toHaveValue(STONE.id);
    });
    expect(itemSelect(attackerRegion())).toBeDisabled();
    expect(itemSelect(attackerRegion())).toHaveAccessibleDescription(megaItemText.lockedReason);

    await chooseBySearch(user, candidate(1), OTHER);
    await user.click(screen.getByRole("button", { name: judgeScreenText.submitLabel }));
    expect(client.calls[0]?.args.attacker.itemId).toBe(STONE.id);

    await user.clear(within(attackerRegion()).getByRole("combobox", { name: judgeScreenText.speciesLabel }));
    await chooseBySearch(user, attackerRegion(), NORMAL);
    await waitFor(() => {
      expect(itemSelect(attackerRegion())).toBeEnabled();
    });
    expect(itemSelect(attackerRegion())).toHaveValue("");
  });
});
