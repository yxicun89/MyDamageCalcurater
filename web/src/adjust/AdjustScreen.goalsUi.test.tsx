// I-web-18(ADR-0354): 目標カードを「見て分かる」形にした部品の確認(構造・API 呼び出し・検証の流れは既存の AdjustScreen*.test.tsx が守る)。
//   - 目標の種類は 3 択の区切りボタン(radiogroup + radio。アイコンつき。選択中はチェックの印で色だけに頼らない。矢印キーで切り替え)
//   - 目標の相手: これまでに選んだ相手の絵のタイル(押すと相手になる。選択中は aria-pressed)。select は残る
//   - 振り方: 4 択以下は区切りボタン、5 択以上(倒す)は select のまま
//   - 技: 技ピッカー(見えるラベルは短い語、読み上げ名は「目標 n の…」)
//   - 補足の説明は「説明」ボタンの奥(開くまで本文は出ない)。欄の aria-describedby は従来どおり説明の文へ結ぶ

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import { adjustScreenText } from "../i18n/ja";
import { uiText } from "../i18n/ui";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { describedByTextsOf } from "../test/accessibleName";
import { createFakeAdjustClient } from "../test/fakeAdjustClient";
import { AdjustScreen } from "./AdjustScreen";

const T = adjustScreenText;

const MOVE: Move = {
  id: "test-move-fire",
  nameJa: "テストほのおパンチ",
  type: "fire",
  category: "physical",
  power: 90,
  priority: 0,
};

function fakeSpecies(key: string, nameJa: string, types: readonly string[]): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: 0,
    nameJa,
    types,
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: ["test-ability-a"],
    learnset: [MOVE.id],
  };
}

const BIRD = fakeSpecies("9001-000", "テストカソウドリ", ["fire"]);
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"]);
const BUG = fakeSpecies("9003-000", "テストカソウムシ", ["normal"]);
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };

const master: MasterData = {
  species: [BIRD, FISH, BUG],
  moves: [MOVE],
  items: [],
  abilities: [{ id: "test-ability-a", nameJa: "テストとくせい", effect: null }],
  natures: [NATURE],
  typeChart: { types: ["fire", "water", "normal"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: true },
};

async function setup() {
  const user = userEvent.setup();
  render(<AdjustScreen adjustClient={createFakeAdjustClient()} master={master} goalsEnabled />);
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
  await user.click(screen.getByRole("radio", { name: T.modeLabel.goals }));
  await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
  return { user };
}

const card = (n: number) =>
  within(screen.getByRole("region", { name: T.goalRegionLabel })).getByRole("group", {
    name: T.goalCardLegend(n),
  });

describe("目標の種類(区切りボタン)", () => {
  test("radiogroup に 3 つの radio。選択中だけチェックの印(文字でも分かる)で、矢印キーで選択が動く", async () => {
    const { user } = await setup();
    const group = within(card(1)).getByRole("radiogroup", { name: T.goalFieldName(1, T.goalKindFieldLabel) });
    const radios = within(group).getAllByRole("radio");
    expect(radios.map((radio) => radio.textContent)).toEqual([
      T.goalKindOption.outspeed,
      T.goalKindOption.survive,
      T.goalKindOption.ko,
    ]);
    // 全部にアイコン(選択中はチェック)。色だけに頼らない。
    for (const radio of radios) {
      expect(radio.querySelector("svg")).not.toBeNull();
    }
    expect(radios[0]).toHaveAttribute("aria-checked", "true");
    expect(radios[1]).toHaveAttribute("aria-checked", "false");
    radios[0]?.focus();
    await user.keyboard("{ArrowRight}");
    expect(radios[1]).toHaveAttribute("aria-checked", "true");
    expect(radios[1]).toHaveFocus();
  });
});

describe("目標の相手(絵のタイル)", () => {
  test("ほかの目標で選んだ相手が絵のタイルで並び、押すと相手になる(select はそのまま残る)", async () => {
    const { user } = await setup();
    // 1 件目はまだ誰も選んでいないのでタイルは無い。
    expect(within(card(1)).queryByText(T.goalRecentOpponentsLabel)).toBeNull();
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalOpponentSpeciesFieldLabel) }),
      FISH.key,
    );
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));

    expect(within(card(2)).getByText(T.goalRecentOpponentsLabel)).toBeVisible();
    const tile = within(card(2)).getByRole("button", { name: FISH.nameJa, pressed: false });
    await user.click(tile);
    expect(within(card(2)).getByRole("button", { name: FISH.nameJa, pressed: true })).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: T.goalFieldName(2, T.goalOpponentSpeciesFieldLabel) }),
    ).toHaveValue(FISH.key);
  });
});

describe("振り方と技", () => {
  test("素早さ(3 択)の振り方は区切りボタン。倒す(5 択以上)は select のまま", async () => {
    const { user } = await setup();
    expect(
      within(card(1)).getByRole("radiogroup", { name: T.goalFieldName(1, T.goalPresetFieldLabel) }),
    ).toBeInTheDocument();
    await user.click(within(card(1)).getByRole("radio", { name: T.goalKindOption.ko }));
    expect(
      within(card(1)).getByRole("combobox", { name: T.goalFieldName(1, T.goalPresetFieldLabel) }),
    ).toBeInTheDocument();
  });

  test("技は技ピッカー: 見えるラベルは短い語、読み上げ名は「目標 n の…」で、見えるラベルを含む", async () => {
    await setup();
    const trigger = screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalBoostMoveFieldLabel) });
    expect(trigger).toHaveAttribute("aria-haspopup", "listbox");
    expect(within(card(1)).getByText(T.goalBoostMoveFieldLabel, { selector: "label" })).toBeVisible();
  });
});

describe("補足の説明は「説明」ボタンの奥", () => {
  test("先に使う技の補足は開くまで画面に出ない。開くと本文が出て、欄の aria-describedby は常に説明の文を指す", async () => {
    const { user } = await setup();
    const trigger = screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalBoostMoveFieldLabel) });
    expect(describedByTextsOf(trigger)).toContain(T.goalBoostMoveHint);
    const help = within(card(1)).getByRole("button", { name: uiText.help });
    expect(help).toHaveAttribute("aria-expanded", "false");
    expect(card(1).querySelector(".ui-help__body")).toBeNull();
    await user.click(help);
    expect(card(1).querySelector(".ui-help__body")).toHaveTextContent(T.goalBoostMoveHint);
  });

  test("固定する SP の補足は「説明」の奥にあり、開くまで本文は出ない", async () => {
    await setup();
    const group = screen.getByRole("group", { name: T.fixedSpGroupLabel });
    expect(within(group).getByRole("button", { name: uiText.help })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(group.querySelector(".ui-help__body")).toBeNull();
  });
});
