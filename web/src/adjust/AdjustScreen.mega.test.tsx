// F-10(ADR-0331 §1・受け入れ条件 AC1〜AC3): 調整画面のメガシンカの持ち物固定(計算・判定の ADR-0320 と、iOS の ADR-0509 と同じ挙動)。
// AdjustClient は fake。マスタは架空(9xxx の種族・test-* の ID)。
// 確かめること:
//   M1 自分にメガ種族を選ぶと持ち物がストーンに固定される(disabled・ストーンの名前・理由の見える文言と aria-describedby)
//   M2 メガ以外・未選択に変えると固定を外して未選択に戻す。メガどうしの切り替えは新しいストーン
//   M3 要求の自分の itemId はストーン(indices・各モード・goals)。相手がメガなら相手の itemId もストーン
//   M4 ストーンがマスタに無いメガは、空で disabled・理由。要求に itemId を送らない
//   M5 種族を検索で解決するマスタでも固定する

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import type { Ability, Item, Move } from "../engine/types";
import { adjustScreenText, megaItemText, type AdjustModeKey } from "../i18n/ja";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { createFakeAdjustClient, lastRequestOf, type FakeAdjustClient } from "../test/fakeAdjustClient";
import { createFakeSpeciesSearch } from "../test/onlineMaster";
import { AdjustScreen } from "./AdjustScreen";

type Schemas = components["schemas"];

const T = adjustScreenText;

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
const STONE_B: Item = { id: "test-megastone-b", nameJa: "テストメガナイトB", effect: null };
const MISSING_STONE_ID = "test-megastone-missing";
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };
const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};

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
const MEGA_B = species("9102-001", "メガテストカソウギョ", { isMega: true, requiredItemId: STONE_B.id });
const ORPHAN = species("9103-001", "メガテストカソウムシ", {
  isMega: true,
  requiredItemId: MISSING_STONE_ID,
});

const master: MasterData = {
  species: [NORMAL, OTHER, MEGA, MEGA_B, ORPHAN],
  moves: [MOVE],
  items: [ITEM, STONE, STONE_B],
  abilities: [ABILITY],
  natures: [NATURE, NATURE_PLUS_SPE],
  typeChart: { types: ["fire", "water", "normal"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: false },
};

function renderScreen(options: { readonly master?: MasterData; readonly goalsEnabled?: boolean } = {}) {
  const user = userEvent.setup();
  const client = createFakeAdjustClient();
  render(
    <AdjustScreen
      adjustClient={client}
      master={options.master ?? master}
      goalsEnabled={options.goalsEnabled}
    />,
  );
  return { user, client };
}

const itemSelect = () => screen.getByRole("combobox", { name: T.selfItemLabel });
const submitButton = () => screen.getByRole("button", { name: T.submitLabel });

async function chooseSelf(user: UserEvent, target: MasterSpecies): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), target.key);
}

async function fillSelf(user: UserEvent, target: MasterSpecies, withMove = false): Promise<void> {
  await chooseSelf(user, target);
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
  if (withMove) {
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfMoveLabel }), MOVE.id);
  }
}

async function chooseMode(user: UserEvent, mode: AdjustModeKey): Promise<void> {
  await user.click(screen.getByRole("radio", { name: T.modeLabel[mode] }));
}

function individualOf(client: FakeAdjustClient): Schemas["Individual"] {
  return (lastRequestOf(client, "indices") as Schemas["AdjustIndicesRequest"]).individual;
}

describe("M1 自分にメガ種族を選ぶと持ち物がストーンに固定される", () => {
  test("disabled・ストーンの名前・理由の見える文言と aria-describedby", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, MEGA);

    const item = itemSelect();
    expect(item).toBeDisabled();
    expect(item).toHaveValue(STONE.id);
    // ADR-0328: ストーンの nameJa が日本語ならそのまま見せる。
    expect(item).toHaveDisplayValue(STONE.nameJa);
    const selfRegion = screen.getByRole("region", { name: T.selfRegionLabel });
    expect(within(selfRegion).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(item).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("非メガの種族では固定しない(選べる・理由を出さない)", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, NORMAL);
    expect(itemSelect()).toBeEnabled();
    expect(screen.queryByText(megaItemText.lockedReason)).toBeNull();
  });
});

describe("M2 種族を変えたときの固定と解除", () => {
  test("メガ → 非メガで固定を外し、未選択に戻す", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, MEGA);
    await chooseSelf(user, NORMAL);
    expect(itemSelect()).toBeEnabled();
    expect(itemSelect()).toHaveValue("");
    expect(screen.queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("メガ → 未選択でも未選択に戻す", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, MEGA);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), "");
    expect(itemSelect()).toHaveValue("");
  });

  test("メガ → 別のメガは、新しいストーンに固定する", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, MEGA);
    await chooseSelf(user, MEGA_B);
    expect(itemSelect()).toBeDisabled();
    expect(itemSelect()).toHaveValue(STONE_B.id);
  });

  test("非メガで選んだ持ち物は、非メガどうしの切り替えでは保つ", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, NORMAL);
    await user.selectOptions(itemSelect(), ITEM.id);
    await chooseSelf(user, OTHER);
    expect(itemSelect()).toHaveValue(ITEM.id);
  });

  test("固定を外したあと、メガストーンは単独の選択肢に出ない", async () => {
    const { user } = renderScreen();
    await chooseSelf(user, MEGA);
    await chooseSelf(user, NORMAL);
    const names = within(itemSelect())
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(names).not.toContain(STONE.nameJa);
    expect(names).not.toContain(STONE_B.nameJa);
  });
});

describe("M3 要求の itemId", () => {
  test("指数: 自分の itemId はストーン", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, MEGA);
    await user.click(submitButton());
    expect(individualOf(client).itemId).toBe(STONE.id);
  });

  test("耐久に振る(allocation): self の itemId はストーン", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, MEGA);
    await chooseMode(user, "bulk");
    await user.click(submitButton());
    expect((lastRequestOf(client, "allocation") as Schemas["AdjustAllocationRequest"]).self.itemId).toBe(
      STONE.id,
    );
  });

  test("倒せる最小の振り方: 自分(attacker)と、メガの相手(defender)の両方にストーン", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, MEGA, true);
    await chooseMode(user, "minKo");
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentSpeciesLabel }), MEGA_B.key);
    await user.click(submitButton());
    const request = lastRequestOf(client, "minSpToKo") as Schemas["AdjustSearchRequest"];
    expect(request.attacker.itemId).toBe(STONE.id);
    expect(request.defender.itemId).toBe(STONE_B.id);
  });

  test("耐えられる最小の振り方: メガの相手(attacker)にストーン、非メガの自分には付けない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, NORMAL);
    await chooseMode(user, "minSurvive");
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentSpeciesLabel }), MEGA_B.key);
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentMoveLabel }), MOVE.id);
    await user.click(submitButton());
    const request = lastRequestOf(client, "minSpToSurvive") as Schemas["AdjustSearchRequest"];
    expect(request.attacker.itemId).toBe(STONE_B.id);
    expect(request.defender).not.toHaveProperty("itemId");
  });

  test("目標から振り方を決める: self と、メガの相手の opponent にストーン", async () => {
    const { user, client } = renderScreen({ goalsEnabled: true });
    await fillSelf(user, MEGA);
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalOpponentSpeciesFieldLabel) }),
      MEGA_B.key,
    );
    await user.click(submitButton());
    const request = lastRequestOf(client, "goals") as Schemas["AdjustGoalsRequest"];
    expect(request.self.itemId).toBe(STONE.id);
    expect(request.goals[0]?.opponent.itemId).toBe(STONE_B.id);
  });
});

describe("M4 ストーンがマスタに無いメガ", () => {
  test("空で disabled・理由(missingReason)。要求に itemId を送らない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, ORPHAN);
    expect(itemSelect()).toBeDisabled();
    expect(itemSelect()).toHaveValue("");
    expect(itemSelect()).toHaveAccessibleDescription(megaItemText.missingReason);
    await user.click(submitButton());
    expect(individualOf(client)).not.toHaveProperty("itemId");
  });
});

describe("M5 種族を検索で解決するマスタ", () => {
  test("検索でメガ種族を選ぶと固定され、要求にストーンが載る", async () => {
    const search = createFakeSpeciesSearch({ species: [NORMAL, MEGA], abilities: [ABILITY], moves: [MOVE] });
    const user = userEvent.setup();
    const client = createFakeAdjustClient();
    render(
      <AdjustScreen
        adjustClient={client}
        master={{
          ...master,
          species: [],
          moves: [],
          abilities: [],
          capabilities: { speciesList: false, moves: false, effects: false },
        }}
        masterSearch={search}
      />,
    );
    const selfRegion = screen.getByRole("region", { name: T.selfRegionLabel });
    await user.type(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), MEGA.nameJa);
    await user.click(await within(selfRegion).findByRole("option", { name: MEGA.nameJa }));
    await waitFor(() => {
      expect(itemSelect()).toHaveValue(STONE.id);
    });
    expect(itemSelect()).toBeDisabled();
    expect(itemSelect()).toHaveAccessibleDescription(megaItemText.lockedReason);

    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
    await user.click(submitButton());
    expect(individualOf(client).itemId).toBe(STONE.id);
  });
});
