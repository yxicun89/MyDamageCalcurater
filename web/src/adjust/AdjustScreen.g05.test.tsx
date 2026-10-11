// I-web-13h(G-05、ADR-0339): 調整画面の見た目の作り直し(docs/design.md「見た目の作り直し方針(G-05)」→「画面ごとの方向」)。
// 確かめること(構造・API 呼び出し・検証の流れは既存の AdjustScreen*.test.tsx が守る。ここは新しい部品のクラス・役割だけ):
//   - 領域の見出し(h2)はアイコン + 短い名前。自分・相手(相手のいるモード)は選ぶと PokemonCard(名前 h3・タイプバッジ・エンブレム・持ち物の印)
//   - メガ種族の持ち物はストーン固定のまま(select は disabled)で、カードの持ち物の印にもストーンが出る
//   - 固定する SP・上限は − / + 付き(数値欄は残す。範囲外の入力は従来どおり誤りとして出し、送らない)
//   - 目標: 「目標を追加」は「+」の空きタイル(上限で無効・理由は aria-describedby)、カードは種類のアイコン + 相手の PokemonCard
//   - 結果: 振り方は大きい数字 + 能力ごとの横棒(装飾)、結論には成功/失敗のアイコン

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { megaStoneLabel } from "../domain/itemRoles";
import type { Item, Move } from "../engine/types";
import { adjustScreenText } from "../i18n/ja";
import { uiText } from "../i18n/ui";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { declarationMap, parseCss, splitSelectors, topLevelRules } from "../test/cssRules";
import { createFakeAdjustClient, lastCallOf, respond, type FakeAdjustClient } from "../test/fakeAdjustClient";
import { localPath } from "../test/localPath";
import { AdjustScreen } from "./AdjustScreen";
import { MAX_ADJUST_GOALS } from "./adjustGoals";

type Schemas = components["schemas"];

const T = adjustScreenText;

const MOVE: Move = {
  id: "test-move-fire",
  nameJa: "テストほのおパンチ",
  type: "fire",
  category: "physical",
  power: 90,
  priority: 0,
};
const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};
const STONE: Item = { id: "test-megastone-a", nameJa: "テストメガナイト", effect: null };
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };

function fakeSpecies(
  key: string,
  nameJa: string,
  types: readonly string[],
  extra: Partial<MasterSpecies> = {},
): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: Number(key.slice(5)),
    nameJa,
    types,
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: ["test-ability-a"],
    learnset: [MOVE.id],
    ...extra,
  };
}

const BIRD = fakeSpecies("9001-000", "テストカソウドリ", ["fire", "flying"]);
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"]);
const MEGA = fakeSpecies("9101-001", "メガテストカソウドリ", ["fire"], {
  isMega: true,
  requiredItemId: STONE.id,
});

const master: MasterData = {
  species: [BIRD, FISH, MEGA],
  moves: [MOVE],
  items: [STONE],
  abilities: [{ id: "test-ability-a", nameJa: "テストとくせい", effect: null }],
  natures: [NATURE, NATURE_PLUS_SPE],
  typeChart: { types: ["fire", "water", "flying"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: true },
};

function renderScreen(): { user: UserEvent; client: FakeAdjustClient } {
  const user = userEvent.setup();
  const client = createFakeAdjustClient();
  render(<AdjustScreen adjustClient={client} master={master} goalsEnabled />);
  return { user, client };
}

const region = (name: string) => screen.getByRole("region", { name });
const submitButton = () => screen.getByRole("button", { name: T.submitLabel });

async function chooseMode(user: UserEvent, key: keyof typeof T.modeLabel): Promise<void> {
  await user.click(screen.getByRole("radio", { name: T.modeLabel[key] }));
}

async function fillSelf(user: UserEvent, target: MasterSpecies = BIRD): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), target.key);
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
}

const indicesResult: Schemas["AdjustIndicesResult"] = {
  stats: { hp: 159, atk: 120, def: 90, spa: 80, spd: 91, spe: 130 },
  firepowerIndex: 16200,
  physicalBulkIndex: 14310,
  specialBulkIndex: 14469,
  hpLines: {
    hp: 159,
    sp: 4,
    current: "16n-1",
    next16n: null,
    prev16n: null,
    next16nMinus1: null,
    prev16nMinus1: null,
  },
};

describe("領域の見出し", () => {
  test("自分・調整の内容・調整の結果は h2(アイコン + 短い名前)", () => {
    renderScreen();
    for (const name of [T.selfRegionLabel, T.modeRegionLabel, T.resultRegionLabel]) {
      const heading = within(region(name)).getByRole("heading", { level: 2, name });
      expect(heading.querySelector("svg")).not.toBeNull();
    }
  });
});

describe("自分・相手のポケモンカード", () => {
  test("未選択の間はカードを出さず、選ぶと名前(h3)・タイプバッジ・エンブレムが出る", async () => {
    const { user } = renderScreen();
    const self = region(T.selfRegionLabel);
    expect(self.querySelector(".ui-pokemon-card")).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    const card = self.querySelector(".ui-pokemon-card");
    expect(card).not.toBeNull();
    expect(within(self).getByRole("heading", { level: 3, name: BIRD.nameJa })).toBeInTheDocument();
    expect(card?.querySelectorAll(".ui-badge")).toHaveLength(BIRD.types.length);
    expect(card?.querySelector('[data-testid="pokemon-icon-emblem"]')).not.toBeNull();
  });

  test("メガ種族: 持ち物の欄はストーンに固定(disabled)のまま、カードの持ち物の印にもストーンが出る", async () => {
    const { user } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), MEGA.key);
    const itemSelect = screen.getByRole("combobox", { name: T.selfItemLabel });
    expect(itemSelect).toBeDisabled();
    const card = region(T.selfRegionLabel).querySelector(".ui-pokemon-card");
    expect(card?.querySelector(".ui-pokemon-card__item")?.textContent).toBe(
      megaStoneLabel(MEGA, STONE.nameJa),
    );
  });

  test("相手(相手が要るモード)も選ぶとカードが出る", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "minKo");
    const opponent = region(T.opponentRegionLabel);
    expect(opponent.querySelector(".ui-pokemon-card")).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentSpeciesLabel }), FISH.key);
    expect(within(opponent).getByRole("heading", { level: 3, name: FISH.nameJa })).toBeInTheDocument();
  });
});

describe("増減ボタン(固定する SP・上限)", () => {
  const fixedHp = () => screen.getByRole("spinbutton", { name: T.fixedSpLabel("hp") });

  test("− / + で値が 1 ずつ動く。0 では減らせず、32 では増やせない(aria-disabled)", async () => {
    const { user } = renderScreen();
    const plus = screen.getByRole("button", { name: `${T.fixedSpLabel("hp")}${uiText.stepperIncrease}` });
    const minus = screen.getByRole("button", { name: `${T.fixedSpLabel("hp")}${uiText.stepperDecrease}` });
    expect(minus).toHaveAttribute("aria-disabled", "true");
    await user.click(plus);
    expect(fixedHp()).toHaveValue(1);
    await user.click(minus);
    expect(fixedHp()).toHaveValue(0);
    await user.clear(fixedHp());
    await user.type(fixedHp(), "32");
    expect(plus).toHaveAttribute("aria-disabled", "true");
  });

  test("範囲外(33)を打つと、従来どおり誤りとして出し、API を呼ばない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.clear(fixedHp());
    await user.type(fixedHp(), "33");
    expect(fixedHp()).toHaveValue(33);
    await user.click(submitButton());
    expect(screen.getByRole("alert")).toHaveTextContent(T.spRangeMessage(32));
    expect(client.calls).toHaveLength(0);
  });

  test("上限の欄にも − / + がある(耐久に振る)", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "bulk");
    expect(
      screen.getByRole("button", { name: `${T.ceilingLabel("hp")}${uiText.stepperDecrease}` }),
    ).toBeInTheDocument();
  });
});

describe("目標のタイルとカード", () => {
  test("「目標を追加」は「+」の空きタイル。上限では無効で、理由が aria-describedby", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    const add = () => screen.getByRole("button", { name: T.addGoalLabel });
    expect(add()).toHaveClass("ui-tile", "ui-tile--empty");
    for (let i = 0; i < MAX_ADJUST_GOALS; i += 1) {
      await user.click(add());
    }
    expect(add()).toBeDisabled();
    const describedBy = add().getAttribute("aria-describedby");
    expect(describedBy).not.toBeNull();
    expect(document.getElementById(describedBy ?? "")).toHaveTextContent(T.goalLimitHint(MAX_ADJUST_GOALS));
  });

  test("カードは種類のアイコン(装飾)を持ち、相手を選ぶとポケモンカードが出る", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    const card = within(region(T.goalRegionLabel)).getByRole("group", { name: T.goalCardLegend(1) });
    expect(card.querySelector(".adjust-screen__goal-kind svg")).not.toBeNull();
    expect(card.querySelector(".ui-pokemon-card")).toBeNull();
    await user.selectOptions(
      within(card).getByRole("combobox", { name: T.goalFieldName(1, T.goalOpponentSpeciesFieldLabel) }),
      FISH.key,
    );
    expect(card.querySelector(".ui-pokemon-card.ui-pokemon-card--small")).not.toBeNull();
    expect(card.querySelectorAll(".ui-pokemon-card .ui-badge")).toHaveLength(FISH.types.length);
  });

  test("「外す」を押すとフォーカスが「目標を追加」へ戻る", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    await user.click(screen.getByRole("button", { name: T.removeGoalName(1) }));
    expect(screen.getByRole("button", { name: T.addGoalLabel })).toHaveFocus();
  });
});

describe("結果", () => {
  const plan = {
    sp: { hp: 32, atk: 0, def: 18, spa: 0, spd: 16, spe: 0 },
    totalSp: 66,
    stats: indicesResult.stats,
  };

  test("振り方は能力ごとの横棒(装飾。aria-hidden)。幅は SP / 32 の割合", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalOpponentSpeciesFieldLabel) }),
      FISH.key,
    );
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "goals"), {
      ok: true,
      value: {
        feasible: true,
        remaining: 0,
        plan,
        unsupported: [],
        goals: [
          {
            kind: "outspeed",
            met: true,
            chancePercent: null,
            selfSpeed: 150,
            opponentSpeed: 140,
            selfSpeedRank: 0,
          },
        ],
      },
    });
    const result = region(T.resultRegionLabel);
    const bars = result.querySelector(".adjust-sp-bars");
    expect(bars).toHaveAttribute("aria-hidden", "true");
    const fills = bars?.querySelectorAll<HTMLElement>(".adjust-sp-bars__fill") ?? [];
    expect(fills).toHaveLength(6);
    expect(fills[0]?.style.width).toBe("100%");
    expect(fills[1]?.style.width).toBe("0%");
    expect(fills[2]?.style.width).toBe(`${String((18 / 32) * 100)}%`);
    // 結論の行(目標ごと)に成功のアイコン。文字は従来のまま
    const outcomes = within(result).getByRole("list", { name: T.goalOutcomesLabel });
    expect(outcomes.querySelector("li svg")).not.toBeNull();
    // 合計は大きい数字
    expect(within(result).getByText(T.planTotal(66))).toBeInTheDocument();
  });
});

describe("CSS(AdjustScreen.css)", () => {
  const rules = topLevelRules(
    parseCss(readFileSync(localPath("./AdjustScreen.css", import.meta.url), "utf8")),
  );
  const declarationsOf = (selector: string) =>
    declarationMap(rules.filter((rule) => splitSelectors(rule.selector).includes(selector)));

  test("振り方の数字は大きい(--font-size-title)、横棒の塗りは brand.primary のトークン", () => {
    expect(declarationsOf(".adjust-sp-bars__value").get("font-size")).toBe("var(--font-size-title)");
    expect(declarationsOf(".adjust-sp-bars__fill").get("background")).toBe("var(--brand-primary)");
  });

  test("横棒の幅の変化は操作時の遷移だけ(常時動くアニメーションを置かない)", () => {
    const css = readFileSync(localPath("./AdjustScreen.css", import.meta.url), "utf8");
    expect(css).not.toMatch(/@keyframes|animation\s*:/);
  });
});
