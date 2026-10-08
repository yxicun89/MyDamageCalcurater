// F-12 PR-2(I-web-12、ADR-0336 §3): 調整画面への共通部品の適用(構造・挙動・API 呼び出し・アクセシブル名は変えない)。
// 自分/相手の領域は ui-card ui-card--typed(--card-type は選んだ種族の最初のタイプ)、目標カード(fieldset)は ui-card、
// モードの選択(ラジオ)は ui-chip、ボタンは ui-button、案内は ui-notice、結果の小領域は ui-card、行の一覧は ui-rows。
// 結果は「結論(目標を満たすか)→ 振り方(SP)→ 詳細(指数・16n)」の強弱を CSS の見た目で付ける。DOM の順は変えない。
//   結論 = adjust-screen__verdict(要素に data-met="true"|"false"。目標ごとの行・目標を満たす/満たさない・素早さを満たす/満たさない・倒せる/耐えられるの文)/ 振り方 = adjust-screen__plan / 詳細 = adjust-screen__detail
// 目標の「種類」は select のまま(ラジオに変えるのは構造の変更になるので、ui-field の見た目だけ。ADR-0336 §3)。

import { readFileSync } from "node:fs";
import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import type { Move } from "../engine/types";
import { adjustScreenText } from "../i18n/ja";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { declarationMap, parseCss, splitSelectors, topLevelRules } from "../test/cssRules";
import { createFakeAdjustClient, lastCallOf, respond, type FakeAdjustClient } from "../test/fakeAdjustClient";
import { localPath } from "../test/localPath";
import { AdjustScreen } from "./AdjustScreen";

type Schemas = components["schemas"];

const T = adjustScreenText;

const MOVE_FIRE: Move = {
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
    learnset: [MOVE_FIRE.id],
  };
}

const BIRD = fakeSpecies("9001-000", "テストカソウドリ", ["fire"]);
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"]);
const NATURE: MasterNature = { id: "test-nature-neutral", nameJa: "テストまじめ", plus: null, minus: null };

/** 目標「素早さを上回る」の相手の振り方(最速)が要る、素早さ上昇の性格。 */
const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};

const master: MasterData = {
  species: [BIRD, FISH],
  moves: [MOVE_FIRE],
  items: [],
  abilities: [{ id: "test-ability-a", nameJa: "テストとくせい", effect: null }],
  natures: [NATURE, NATURE_PLUS_SPE],
  typeChart: { types: ["fire", "water"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: true },
};

const typeVar = (typeId: string) => `var(--type-${typeId}, var(--brand-primary))`;

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

const indicesResult: Schemas["AdjustIndicesResult"] = {
  stats: { hp: 159, atk: 120, def: 90, spa: 80, spd: 91, spe: 130 },
  firepowerIndex: 16200,
  physicalBulkIndex: 14310,
  specialBulkIndex: 14469,
  hpLines: {
    hp: 159,
    sp: 4,
    current: "16n-1",
    next16n: { hp: 160, sp: 5, spDelta: 1 },
    prev16n: null,
    next16nMinus1: null,
    prev16nMinus1: null,
  },
};

describe("領域・カード", () => {
  test("自分の領域は ui-card ui-card--typed(adjust-screen__region も残す)。種族を選ぶと --card-type が最初のタイプ", async () => {
    const { user } = renderScreen();
    const self = region(T.selfRegionLabel);
    expect(self).toHaveClass("ui-card", "ui-card--typed", "adjust-screen__region");
    expect(self.style.getPropertyValue("--card-type")).toBe("");
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    expect(self.style.getPropertyValue("--card-type")).toBe(typeVar("fire"));
  });

  test("相手の領域も ui-card ui-card--typed で、相手の種族のタイプ色になる", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "minKo");
    const opponent = region(T.opponentRegionLabel);
    expect(opponent).toHaveClass("ui-card", "ui-card--typed");
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentSpeciesLabel }), FISH.key);
    expect(opponent.style.getPropertyValue("--card-type")).toBe(typeVar("water"));
  });

  test("目標カード(fieldset。group「目標 n」)は ui-card(adjust-screen__goal も残す)。種類は select のまま", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    const card = within(region(T.goalRegionLabel)).getByRole("group", { name: T.goalCardLegend(1) });
    expect(card).toHaveClass("ui-card", "adjust-screen__goal");
    expect(
      within(card).getByRole("combobox", { name: T.goalFieldName(1, T.goalKindFieldLabel) }),
    ).toBeInTheDocument();
  });
});

describe("チップ・ボタン", () => {
  test("モード(ラジオ)は ui-chip。選択中だけ ui-chip--selected で、切り替えに追従する", async () => {
    const { user } = renderScreen();
    const group = screen.getByRole("radiogroup", { name: T.modeGroupLabel });
    for (const radio of within(group).getAllByRole("radio")) {
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip", "adjust-screen__choice");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
    await chooseMode(user, "bulk");
    expect(screen.getByRole("radio", { name: T.modeLabel.bulk }).closest("label")).toHaveClass(
      "ui-chip--selected",
    );
    expect(screen.getByRole("radio", { name: T.modeLabel.indices }).closest("label")).not.toHaveClass(
      "ui-chip--selected",
    );
  });

  test("「調整する」は ui-button ui-button--primary(adjust-screen__submit も残す)", () => {
    renderScreen();
    expect(submitButton()).toHaveClass("ui-button", "ui-button--primary", "adjust-screen__submit");
  });

  test("「目標を追加」「目標を外す」は ui-button--secondary", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    expect(screen.getByRole("button", { name: T.addGoalLabel })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    expect(screen.getByRole("button", { name: T.removeGoalName(1) })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
  });
});

describe("案内", () => {
  test("結果が無いときの案内は ui-notice ui-notice--empty", () => {
    renderScreen();
    expect(screen.getByText(T.emptyResultNotice)).toHaveClass("ui-notice", "ui-notice--empty");
  });

  test("目標が無いときの案内は ui-notice ui-notice--empty", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    expect(screen.getByText(T.noGoalsNotice)).toHaveClass("ui-notice", "ui-notice--empty");
  });

  test("計算中は ui-notice ui-notice--loading、送信前の検査の違反(role=alert)は ui-notice--error", async () => {
    const { user } = renderScreen();
    await user.click(submitButton());
    expect(screen.getByRole("alert")).toHaveClass("ui-notice", "ui-notice--error");
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
    await user.click(submitButton());
    expect(screen.getByText(T.loadingNotice)).toHaveClass("ui-notice", "ui-notice--loading");
  });
});

describe("結果の強弱(DOM の順は変えない)", () => {
  async function submitIndices(): Promise<void> {
    const { user, client } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
  }

  test("結果の小領域(今の振り方の指数)は ui-card(adjust-screen__subregion も残す)", async () => {
    await submitIndices();
    const indices = within(region(T.resultRegionLabel)).getByRole("region", { name: T.indicesHeading });
    expect(indices).toHaveClass("ui-card", "adjust-screen__subregion");
  });

  test("指数・ステータス・16n の行は詳細(adjust-screen__detail)、16n の一覧は ui-rows", async () => {
    await submitIndices();
    const result = region(T.resultRegionLabel);
    expect(within(result).getByText(T.statsLine(indicesResult.stats))).toHaveClass("adjust-screen__detail");
    expect(within(result).getByText(T.indexNote)).toHaveClass("adjust-screen__detail");
    const lines = result.querySelector("ul.adjust-screen__lines");
    expect(lines).toHaveClass("ui-rows");
  });

  test("目標モード: 目標ごとの結果の各行は結論(li が adjust-screen__verdict と data-met を持つ)、SP の行は振り方(adjust-screen__plan)、DOM の順は 振り方 → 結論", async () => {
    const { user, client } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE.id);
    await chooseMode(user, "goals");
    await user.click(screen.getByRole("button", { name: T.addGoalLabel }));
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalOpponentSpeciesFieldLabel) }),
      FISH.key,
    );
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.goalFieldName(1, T.goalPresetFieldLabel) }),
      "fastest",
    );
    await user.click(submitButton());
    const plan = {
      sp: { hp: 32, atk: 0, def: 18, spa: 0, spd: 16, spe: 0 },
      totalSp: 66,
      stats: indicesResult.stats,
    };
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
    const outcomes = within(result).getByRole("list", { name: T.goalOutcomesLabel });
    expect(outcomes).toHaveClass("ui-rows", "adjust-screen__lines");
    for (const item of within(outcomes).getAllByRole("listitem")) {
      expect(item).toHaveClass("adjust-screen__verdict");
      expect(item).toHaveAttribute("data-met", "true");
    }
    const spLine = within(result).getByText(T.planSpLine(plan.sp));
    expect(spLine).toHaveClass("adjust-screen__plan");
    // 順序(構造)は今までどおり: 振り方の行が先、目標ごとの結果が後。
    expect(spLine.compareDocumentPosition(outcomes) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});

describe("CSS の強弱(AdjustScreen.css)", () => {
  const rules = topLevelRules(
    parseCss(readFileSync(localPath("./AdjustScreen.css", import.meta.url), "utf8")),
  );
  const declarationsOf = (selector: string) =>
    declarationMap(rules.filter((rule) => splitSelectors(rule.selector).includes(selector)));

  test("結論は大きい文字(--font-size-heading 以上のトークン)", () => {
    expect(declarationsOf(".adjust-screen__verdict").get("font-size")).toMatch(
      /^var\(--font-size-(heading|title|result)\)$/,
    );
  });

  test("詳細は控えめ(--font-size-caption か 補助色 --text-secondary)", () => {
    const detail = declarationsOf(".adjust-screen__detail");
    expect(`${detail.get("font-size") ?? ""} ${detail.get("color") ?? ""}`).toMatch(
      /--font-size-caption|--text-secondary/,
    );
  });

  test("結論の満たす/満たさないは成功色・危険色のトークンで色分けする(色リテラルを書かない)", () => {
    expect(declarationsOf('.adjust-screen__verdict[data-met="true"]').get("color")).toMatch(
      /var\(--success\)/,
    );
    expect(declarationsOf('.adjust-screen__verdict[data-met="false"]').get("color")).toMatch(
      /var\(--danger\)/,
    );
  });
});
