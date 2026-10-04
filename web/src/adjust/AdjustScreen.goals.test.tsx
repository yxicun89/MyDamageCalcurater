// F-11(ADR-0331 §5〜§7・受け入れ条件 AC6〜AC11): 調整の「相手を選んで目標を選ぶ」モード(目標から振り方を決める)。
// AdjustClient は fake(props で注入)。段階 B(calc-svc)の前なので goalsEnabled で有効にして確かめる。
// 確かめること:
//   G1 構成: モードの radio(先頭に「目標から振り方を決める」・既定は従来どおり)、目標の追加・種類の切り替え・外す・上限・フォーカス
//   G2 要求: 「調整する」で indices と goals を並行して呼び、AdjustGoalsRequest を契約どおりに組み立てる
//   G3 送信前の検査(目標なし・相手・技・性格。API を呼ばない)
//   G4 結果: すべて満たす/一番近い振り方、目標ごとの行(送信時点の名前)、ランクの文、確率の切り捨て、未対応の印
//   G5 エラー: code から日本語、サーバーの message を出さない、目標は消えない
//   G6 取り消し: 送り直し・画面を閉じたら goals も abort、古い応答で上書きしない
//   G7 a11y: group の名前・見えるラベルを含む名前・aria-describedby
// 期待値の文言は i18n(adjustScreenText)の関数から組み立てる。架空データだけを使う(ADR-0002)。

import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { attackerPresetLabel } from "../domain/attackerPresets";
import { defenderPresetKeysFor, defenderPresetLabel } from "../domain/defenderPresets";
import { unsupportedMarkLabels } from "../domain/unsupportedLabels";
import { REQUEST_ABORTED_CODE, type Ability, type Move } from "../engine/types";
import { adjustErrorText, adjustScreenText, unsupportedText, type AdjustModeKey } from "../i18n/ja";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { accessibleNameOf, describedByTextsOf, visibleLabelOf } from "../test/accessibleName";
import {
  callsOf,
  createFakeAdjustClient,
  lastCallOf,
  lastRequestOf,
  respond,
  type FakeAdjustClient,
} from "../test/fakeAdjustClient";
import { AdjustScreen } from "./AdjustScreen";
import { formatChancePercent } from "./adjustFormat";
import { MAX_ADJUST_GOALS } from "./adjustGoals";

type Schemas = components["schemas"];

const T = adjustScreenText;

// ---- 架空のマスタ ----

const MOVE_FIRE: Move = {
  id: "test-move-fire",
  nameJa: "テストほのおパンチ",
  type: "fire",
  category: "physical",
  power: 90,
  priority: 0,
};
const MOVE_BOOST: Move = {
  id: "test-move-boost",
  nameJa: "テストニトロ",
  type: "fire",
  category: "physical",
  power: 50,
  priority: 0,
};
const MOVE_FLAME: Move = {
  id: "test-move-flame",
  nameJa: "テストかえんほうしゃ",
  type: "fire",
  category: "special",
  power: 90,
  priority: 0,
};
const MOVE_WATER: Move = {
  id: "test-move-water",
  nameJa: "テストみずでっぽう",
  type: "water",
  category: "special",
  power: 80,
  priority: 0,
};
const MOVE_TACKLE: Move = {
  id: "test-move-tackle",
  nameJa: "テストたいあたり",
  type: "normal",
  category: "physical",
  power: 40,
  priority: 0,
};
const MOVE_STATUS: Move = {
  id: "test-move-status",
  nameJa: "テストにらみつける",
  type: "normal",
  category: "status",
  power: 0,
  priority: 0,
};

function fakeSpecies(
  key: string,
  nameJa: string,
  types: readonly string[],
  learnset: readonly string[],
): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: 0,
    nameJa,
    types,
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: ["test-ability-a"],
    learnset,
  };
}

/** 自分(ほのお)。物理の技2つ(うち1つは素早さが上がる想定の技)と特殊の技1つ。 */
const BIRD = fakeSpecies(
  "9001-000",
  "テストカソウドリ",
  ["fire"],
  [MOVE_FIRE.id, MOVE_BOOST.id, MOVE_FLAME.id, MOVE_STATUS.id],
);
/** 相手(みず)。物理と特殊の技。 */
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"], [MOVE_WATER.id, MOVE_TACKLE.id]);
/** 相手2(ほのお)。 */
const BUG = fakeSpecies("9003-000", "テストカソウムシ", ["normal"], [MOVE_TACKLE.id]);

const NATURE_NEUTRAL: MasterNature = {
  id: "test-nature-neutral",
  nameJa: "テストまじめ",
  plus: null,
  minus: null,
};
const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};
const NATURE_PLUS_ATK: MasterNature = {
  id: "test-nature-plus-atk",
  nameJa: "テストいじっぱり",
  plus: "atk",
  minus: "spa",
};
const NATURE_PLUS_SPA: MasterNature = {
  id: "test-nature-plus-spa",
  nameJa: "テストひかえめ",
  plus: "spa",
  minus: "atk",
};
const NATURE_PLUS_DEF: MasterNature = {
  id: "test-nature-plus-def",
  nameJa: "テストわんぱく",
  plus: "def",
  minus: "atk",
};
const NATURE_PLUS_SPD: MasterNature = {
  id: "test-nature-plus-spd",
  nameJa: "テストしんちょう",
  plus: "spd",
  minus: "atk",
};

const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };

const master: MasterData = {
  species: [BIRD, FISH, BUG],
  moves: [MOVE_FIRE, MOVE_BOOST, MOVE_FLAME, MOVE_WATER, MOVE_TACKLE, MOVE_STATUS],
  items: [],
  abilities: [ABILITY],
  natures: [
    NATURE_NEUTRAL,
    NATURE_PLUS_SPE,
    NATURE_PLUS_ATK,
    NATURE_PLUS_SPA,
    NATURE_PLUS_DEF,
    NATURE_PLUS_SPD,
  ],
  typeChart: { types: ["fire", "water", "normal"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: true },
};

const ZERO: Schemas["StatBlock"] = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

// ---- 架空の応答 ----

const indicesResult: Schemas["AdjustIndicesResult"] = {
  stats: { hp: 155, atk: 120, def: 90, spa: 80, spd: 90, spe: 130 },
  firepowerIndex: null,
  physicalBulkIndex: 13950,
  specialBulkIndex: 13950,
  hpLines: {
    hp: 155,
    sp: 0,
    current: "none",
    next16n: null,
    prev16n: null,
    next16nMinus1: null,
    prev16nMinus1: null,
  },
};

const plan: Schemas["AdjustGoalsPlan"] = {
  sp: { hp: 4, atk: 22, def: 0, spa: 0, spd: 0, spe: 20 },
  totalSp: 46,
  stats: { hp: 159, atk: 142, def: 90, spa: 80, spd: 90, spe: 150 },
};

function outspeedOutcome(
  overrides: Partial<Schemas["AdjustGoalOutcome"]> = {},
): Schemas["AdjustGoalOutcome"] {
  return {
    kind: "outspeed",
    met: true,
    chancePercent: null,
    selfSpeed: 225,
    opponentSpeed: 167,
    selfSpeedRank: 1,
    ...overrides,
  };
}

function damageOutcome(
  kind: "survive" | "ko",
  overrides: Partial<Schemas["AdjustGoalOutcome"]> = {},
): Schemas["AdjustGoalOutcome"] {
  return {
    kind,
    met: true,
    chancePercent: 100,
    selfSpeed: null,
    opponentSpeed: null,
    selfSpeedRank: null,
    ...overrides,
  };
}

function goalsResult(overrides: Partial<Schemas["AdjustGoalsResult"]> = {}): Schemas["AdjustGoalsResult"] {
  return { feasible: true, remaining: 20, plan, goals: [], unsupported: [], ...overrides };
}

// ---- 描画と操作 ----

function renderScreen(options: { readonly master?: MasterData; readonly goalsEnabled?: boolean } = {}) {
  const user = userEvent.setup();
  const client = createFakeAdjustClient();
  const view = render(
    <AdjustScreen
      adjustClient={client}
      master={options.master ?? master}
      goalsEnabled={options.goalsEnabled ?? true}
    />,
  );
  return { user, client, view };
}

const submitButton = () => screen.getByRole("button", { name: T.submitLabel });
const addButton = () => screen.getByRole("button", { name: T.addGoalLabel });
const goalsRegion = () => screen.getByRole("region", { name: T.goalRegionLabel });
const resultRegion = () => screen.getByRole("region", { name: T.resultRegionLabel });
const card = (n: number) => within(goalsRegion()).getByRole("group", { name: T.goalCardLegend(n) });
const field = (n: number, visibleLabel: string) =>
  screen.getByRole("combobox", { name: T.goalFieldName(n, visibleLabel) });

async function chooseMode(user: UserEvent, mode: AdjustModeKey): Promise<void> {
  await user.click(screen.getByRole("radio", { name: T.modeLabel[mode] }));
}

async function fillSelf(user: UserEvent): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
  await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE_NEUTRAL.id);
}

interface GoalInput {
  readonly kind: "outspeed" | "survive" | "ko";
  readonly species?: MasterSpecies;
  readonly preset?: string;
  readonly move?: Move;
  readonly hits?: number;
  readonly thresholdPercent?: number;
}

/** 目標を1つ足して入力する(n はその目標の番号)。 */
async function addGoal(user: UserEvent, n: number, input: GoalInput): Promise<void> {
  await user.click(addButton());
  await user.selectOptions(field(n, T.goalKindFieldLabel), input.kind);
  if (input.species !== undefined) {
    await user.selectOptions(field(n, T.goalOpponentSpeciesFieldLabel), input.species.key);
  }
  if (input.move !== undefined) {
    const moveLabel =
      input.kind === "survive"
        ? T.goalOpponentMoveFieldLabel
        : input.kind === "ko"
          ? T.goalSelfMoveFieldLabel
          : T.goalBoostMoveFieldLabel;
    await user.selectOptions(field(n, moveLabel), input.move.id);
  }
  if (input.preset !== undefined) {
    await user.selectOptions(field(n, T.goalPresetFieldLabel), input.preset);
  }
  if (input.hits !== undefined) {
    await user.selectOptions(field(n, T.hitsLabel), String(input.hits));
  }
  if (input.thresholdPercent !== undefined) {
    await user.selectOptions(field(n, T.thresholdLabel), String(input.thresholdPercent));
  }
}

function optionTexts(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);
}

function optionValues(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => (option as HTMLOptionElement).value);
}

/** 「素早さ+倒す」の2目標を入れて送る(判定レーンの元の要望と同じ形)。 */
async function submitOutspeedAndKo(user: UserEvent): Promise<void> {
  await fillSelf(user);
  await chooseMode(user, "goals");
  await addGoal(user, 1, { kind: "outspeed", species: FISH, preset: "fastest", move: MOVE_BOOST });
  await addGoal(user, 2, { kind: "ko", species: FISH, move: MOVE_FIRE, preset: "hb_full", hits: 1 });
  await user.click(submitButton());
}

async function respondBoth(client: FakeAdjustClient, result: Schemas["AdjustGoalsResult"]): Promise<void> {
  await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
  await respond(lastCallOf(client, "goals"), { ok: true, value: result });
}

// =====================================================================================
// G1 構成
// =====================================================================================

describe("G1 目標の追加・切り替え・外す", () => {
  test("有効なら「目標から振り方を決める」が radio の先頭。既定のモードは従来どおり「指数と 16n を見る」", () => {
    renderScreen();
    const group = screen.getByRole("radiogroup", { name: T.modeGroupLabel });
    expect(
      within(group)
        .getAllByRole("radio")
        .map((radio) => accessibleNameOf(radio)),
    ).toEqual([
      T.modeLabel.goals,
      T.modeLabel.indices,
      T.modeLabel.bulk,
      T.modeLabel.offense,
      T.modeLabel.minKo,
      T.modeLabel.minSurvive,
    ]);
    expect(screen.getByRole("radio", { name: T.modeLabel.indices })).toBeChecked();
  });

  // 段階 B(ADR-0177 §10)で既定を有効に改めた(ADR-0331 §2 の仕様の変更)。無効の経路は props で引き続き確かめる。
  test("既定(ADJUST_GOALS_ENABLED)で「目標から振り方を決める」を出す", () => {
    render(<AdjustScreen adjustClient={createFakeAdjustClient()} master={master} />);
    expect(screen.getByRole("radio", { name: T.modeLabel.goals })).toBeInTheDocument();
  });

  test("goalsEnabled={false} なら「目標から振り方を決める」を出さない", () => {
    render(<AdjustScreen adjustClient={createFakeAdjustClient()} master={master} goalsEnabled={false} />);
    expect(screen.queryByRole("radio", { name: T.modeLabel.goals })).toBeNull();
  });

  test("モードを選ぶと領域「目標」と「目標を追加」を出し、領域「相手」は出さない。目標が無いときは案内の文", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    expect(within(goalsRegion()).getByRole("heading", { name: T.goalRegionLabel, level: 2 })).toBeVisible();
    expect(within(goalsRegion()).getByText(T.noGoalsNotice)).toBeVisible();
    expect(addButton()).toBeEnabled();
    expect(screen.queryByRole("region", { name: T.opponentRegionLabel })).toBeNull();
    expect(within(goalsRegion()).queryAllByRole("group")).toHaveLength(0);
  });

  test("追加すると「目標 1」のカードができ、種類の既定は「素早さを上回る」、フォーカスは種類へ", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await user.click(addButton());

    expect(card(1)).toBeInTheDocument();
    const kind = field(1, T.goalKindFieldLabel);
    expect(kind).toHaveValue("outspeed");
    expect(kind).toHaveFocus();
    expect(optionTexts(kind)).toEqual([
      T.goalKindOption.outspeed,
      T.goalKindOption.survive,
      T.goalKindOption.ko,
    ]);
    expect(within(goalsRegion()).queryByText(T.noGoalsNotice)).toBeNull();
  });

  test("素早さ: 振り方は 最速・準速・無振り(既定は最速)。先に使う技は「使わない」+ 自分のダメージ技。発数・確率は出さない", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await user.click(addButton());

    const preset = field(1, T.goalPresetFieldLabel);
    expect(optionTexts(preset)).toEqual([
      T.speedPresetOption.fastest,
      T.speedPresetOption.neutral_max,
      T.speedPresetOption.none,
    ]);
    expect(preset).toHaveValue("fastest");
    const boost = field(1, T.goalBoostMoveFieldLabel);
    expect(boost).toHaveValue("");
    expect(optionTexts(boost)[0]).toBe(T.goalBoostMoveNone);
    expect(optionValues(boost).filter((value) => value !== "")).toEqual([
      MOVE_FIRE.id,
      MOVE_BOOST.id,
      MOVE_FLAME.id,
    ]);
    expect(within(card(1)).queryByRole("combobox", { name: T.goalFieldName(1, T.hitsLabel) })).toBeNull();
  });

  test("耐える: 相手の技(相手の learnset のダメージ技)・攻撃側の振り方(既定は特化。技の分類で A/C)・発数・確率", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "survive", species: FISH, move: MOVE_WATER });

    expect(optionValues(field(1, T.goalOpponentMoveFieldLabel)).filter((value) => value !== "")).toEqual([
      MOVE_WATER.id,
      MOVE_TACKLE.id,
    ]);
    const preset = field(1, T.goalPresetFieldLabel);
    expect(preset).toHaveValue("x_full");
    expect(optionTexts(preset)).toEqual([
      attackerPresetLabel("none", "special"),
      attackerPresetLabel("x_full", "special"),
      attackerPresetLabel("x", "special"),
    ]);
    expect(field(1, T.hitsLabel)).toHaveValue("1");
    expect(field(1, T.thresholdLabel)).toHaveValue("100");
    expect(
      within(card(1)).queryByRole("combobox", { name: T.goalFieldName(1, T.goalBoostMoveFieldLabel) }),
    ).toBeNull();
  });

  test("倒す: 自分の技・防御側の振り方(自分の技の分類で絞る。既定は無振り)・発数・確率", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "ko", species: FISH, move: MOVE_FLAME });

    expect(optionValues(field(1, T.goalSelfMoveFieldLabel)).filter((value) => value !== "")).toEqual([
      MOVE_FIRE.id,
      MOVE_BOOST.id,
      MOVE_FLAME.id,
    ]);
    const preset = field(1, T.goalPresetFieldLabel);
    expect(preset).toHaveValue("none");
    expect(optionValues(preset)).toEqual([...defenderPresetKeysFor("special")]);
  });

  test("種類を変えると相手のポケモンは保ち、振り方と技は新しい種類の既定に戻す", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "survive", species: FISH, move: MOVE_TACKLE, preset: "none" });
    await user.selectOptions(field(1, T.goalKindFieldLabel), "ko");

    expect(field(1, T.goalOpponentSpeciesFieldLabel)).toHaveValue(FISH.key);
    expect(field(1, T.goalSelfMoveFieldLabel)).toHaveValue("");
    expect(field(1, T.goalPresetFieldLabel)).toHaveValue("none");

    await user.selectOptions(field(1, T.goalKindFieldLabel), "outspeed");
    expect(field(1, T.goalPresetFieldLabel)).toHaveValue("fastest");
    expect(field(1, T.goalBoostMoveFieldLabel)).toHaveValue("");
  });

  test("外すと番号を詰め、フォーカスは「目標を追加」へ", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH });
    await addGoal(user, 2, { kind: "ko", species: BUG, move: MOVE_FIRE });

    await user.click(within(card(1)).getByRole("button", { name: T.removeGoalName(1) }));

    expect(within(goalsRegion()).getAllByRole("group")).toHaveLength(1);
    expect(field(1, T.goalKindFieldLabel)).toHaveValue("ko");
    expect(field(1, T.goalOpponentSpeciesFieldLabel)).toHaveValue(BUG.key);
    expect(addButton()).toHaveFocus();
  });

  test(`目標は ${String(MAX_ADJUST_GOALS)} 件まで。上限で「目標を追加」を押せなくし、理由を aria-describedby で結ぶ`, async () => {
    const { user } = renderScreen();
    await chooseMode(user, "goals");
    for (let i = 0; i < MAX_ADJUST_GOALS; i += 1) {
      await user.click(addButton());
    }
    expect(within(goalsRegion()).getAllByRole("group")).toHaveLength(MAX_ADJUST_GOALS);
    expect(addButton()).toBeDisabled();
    expect(describedByTextsOf(addButton())).toContain(T.goalLimitHint(MAX_ADJUST_GOALS));
  });

  test("モードを切り替えて戻っても目標は消えない", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "ko", species: FISH, move: MOVE_FIRE });
    await chooseMode(user, "bulk");
    await chooseMode(user, "goals");
    expect(field(1, T.goalKindFieldLabel)).toHaveValue("ko");
    expect(field(1, T.goalSelfMoveFieldLabel)).toHaveValue(MOVE_FIRE.id);
  });

  test("自分のポケモンを変えて選べなくなった自分の技(倒す・先に使う技)は未選択に戻す", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "ko", species: FISH, move: MOVE_FIRE });
    await addGoal(user, 2, { kind: "outspeed", species: FISH, move: MOVE_BOOST });
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BUG.key);
    expect(field(1, T.goalSelfMoveFieldLabel)).toHaveValue("");
    expect(field(2, T.goalBoostMoveFieldLabel)).toHaveValue("");
  });
});

// =====================================================================================
// G2 要求
// =====================================================================================

describe("G2 要求の組み立て", () => {
  test("目標を入れているだけでは API を呼ばない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "ko", species: FISH, move: MOVE_FIRE });
    expect(client.calls).toHaveLength(0);
  });

  test("素早さ+倒す: indices と goals を並行して呼び、goals は契約どおり(ceiling・しきい値 100 は送らない)", async () => {
    const { user, client } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE_NEUTRAL.id);
    // 固定 SP は下限として self.sp に載る(allocation と同じ)。
    const hpInput = screen.getByRole("spinbutton", { name: T.fixedSpLabel("hp") });
    await user.clear(hpInput);
    await user.type(hpInput, "4");
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH, preset: "fastest", move: MOVE_BOOST });
    await addGoal(user, 2, { kind: "ko", species: FISH, move: MOVE_FIRE, preset: "hb_full", hits: 1 });
    await user.click(submitButton());

    expect(client.calls.map((call) => call.method)).toEqual(["indices", "goals"]);
    expect(lastRequestOf(client, "goals")).toEqual({
      format: "single",
      self: { speciesKey: BIRD.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO, hp: 4 } },
      goals: [
        {
          kind: "outspeed",
          opponent: {
            speciesKey: FISH.key,
            level: 50,
            natureId: NATURE_PLUS_SPE.id,
            sp: { ...ZERO, spe: 32 },
          },
          moveId: MOVE_BOOST.id,
        },
        {
          kind: "ko",
          opponent: {
            speciesKey: FISH.key,
            level: 50,
            natureId: NATURE_PLUS_DEF.id,
            sp: { ...ZERO, hp: 32, def: 32 },
          },
          moveId: MOVE_FIRE.id,
          hits: 1,
        },
      ],
    } satisfies Schemas["AdjustGoalsRequest"]);
  });

  test("耐えるを2つ(物理・特殊): 相手の技の分類でプリセットの能力と性格が変わり、しきい値 90・2発を送る", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, {
      kind: "survive",
      species: FISH,
      move: MOVE_TACKLE,
      preset: "x_full",
      hits: 2,
      thresholdPercent: 90,
    });
    await addGoal(user, 2, { kind: "survive", species: FISH, move: MOVE_WATER, preset: "x_full" });
    await user.click(submitButton());

    const request = lastRequestOf(client, "goals") as Schemas["AdjustGoalsRequest"];
    expect(request.goals).toEqual([
      {
        kind: "survive",
        opponent: { speciesKey: FISH.key, level: 50, natureId: NATURE_PLUS_ATK.id, sp: { ...ZERO, atk: 32 } },
        moveId: MOVE_TACKLE.id,
        hits: 2,
        thresholdPercent: 90,
      },
      {
        kind: "survive",
        opponent: { speciesKey: FISH.key, level: 50, natureId: NATURE_PLUS_SPA.id, sp: { ...ZERO, spa: 32 } },
        moveId: MOVE_WATER.id,
        hits: 1,
      },
    ]);
  });

  test("素早さ・先に使う技なし: moveId を送らない。準速・無振りは補正なしの性格", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH, preset: "neutral_max" });
    await addGoal(user, 2, { kind: "outspeed", species: BUG, preset: "none" });
    await user.click(submitButton());

    const request = lastRequestOf(client, "goals") as Schemas["AdjustGoalsRequest"];
    expect(request.goals).toEqual([
      {
        kind: "outspeed",
        opponent: { speciesKey: FISH.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO, spe: 32 } },
      },
      {
        kind: "outspeed",
        opponent: { speciesKey: BUG.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO } },
      },
    ]);
  });

  test("ほかのモードの API(allocation・min-sp-*)は呼ばない", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    expect(callsOf(client, "allocation")).toHaveLength(0);
    expect(callsOf(client, "minSpToKo")).toHaveLength(0);
    expect(callsOf(client, "minSpToSurvive")).toHaveLength(0);
  });
});

// =====================================================================================
// G3 送信前の検査
// =====================================================================================

describe("G3 送信前の検査(API を呼ばず role=alert で理由)", () => {
  async function expectAlert(client: FakeAdjustClient, message: string): Promise<void> {
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(client.calls).toHaveLength(0);
  }

  test("目標が無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await user.click(submitButton());
    await expectAlert(client, T.goalsRequiredMessage);
  });

  test("相手のポケモンが無い(番号付き)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH });
    await addGoal(user, 2, { kind: "outspeed" });
    await user.click(submitButton());
    await expectAlert(client, T.goalOpponentRequiredMessage(2));
  });

  test("耐える: 相手の技が無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "survive", species: FISH });
    await user.click(submitButton());
    await expectAlert(client, T.goalOpponentMoveRequiredMessage(1));
  });

  test("倒す: 自分の技が無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "ko", species: FISH });
    await user.click(submitButton());
    await expectAlert(client, T.goalSelfMoveRequiredMessage(1));
  });

  test("相手の振り方に合う性格がマスタに無い(最速 = 素早さ上昇の性格が無い)", async () => {
    const { user, client } = renderScreen({
      master: { ...master, natures: master.natures.filter((nature) => nature.id !== NATURE_PLUS_SPE.id) },
    });
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH, preset: "fastest" });
    await user.click(submitButton());
    await expectAlert(client, T.goalNatureNotFoundMessage(1));
  });

  test("自分のポケモン・性格の検査は従来どおり先に出る", async () => {
    const { user, client } = renderScreen();
    await chooseMode(user, "goals");
    await user.click(submitButton());
    await expectAlert(client, T.selfRequiredMessage);
  });
});

// =====================================================================================
// G4 結果
// =====================================================================================

describe("G4 結果の表示", () => {
  test("すべて満たす: 見出し・能力ポイント・合計・実数値・残り・目標ごとの行(ランクの文・確率)", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await respondBoth(
      client,
      goalsResult({ goals: [outspeedOutcome(), damageOutcome("ko", { chancePercent: 100 })] }),
    );

    const result = resultRegion();
    const planRegion = within(result).getByRole("region", { name: T.goalsPlanHeading });
    expect(within(planRegion).getByRole("heading", { name: T.goalsPlanHeading, level: 3 })).toBeVisible();
    expect(within(planRegion).getByText(T.planSpLine(plan.sp))).toBeVisible();
    expect(within(planRegion).getByText(T.planTotal(plan.totalSp))).toBeVisible();
    expect(within(planRegion).getByText(T.statsLine(plan.stats))).toBeVisible();
    expect(within(planRegion).getByText(T.remainingLabel(20))).toBeVisible();
    expect(within(result).queryByText(T.goalsInfeasibleNotice)).toBeNull();

    const outcomes = within(result).getByRole("list", { name: T.goalOutcomesLabel });
    expect(
      within(outcomes)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([
      T.outspeedOutcome(1, T.goalOpponentName(FISH.nameJa, T.speedPresetOption.fastest), true, 225, 167, {
        moveName: MOVE_BOOST.nameJa,
        rank: 1,
      }),
      T.koOutcome(
        2,
        T.goalOpponentName(FISH.nameJa, defenderPresetLabel("hb_full")),
        MOVE_FIRE.nameJa,
        1,
        true,
        formatChancePercent(100),
      ),
    ]);
    // 今の振り方の指数(従来の表示)も出す。
    expect(within(result).getByRole("region", { name: T.indicesHeading })).toBeInTheDocument();
  });

  test("満たせない: 「目標に一番近い振り方」と案内の文、満たせない行は確率を切り捨てで出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "survive", species: FISH, move: MOVE_WATER, preset: "x_full", hits: 2 });
    await user.click(submitButton());
    await respondBoth(
      client,
      goalsResult({
        feasible: false,
        remaining: 0,
        goals: [damageOutcome("survive", { met: false, chancePercent: 87.56 })],
      }),
    );

    const result = resultRegion();
    expect(within(result).getByRole("region", { name: T.goalsNearestHeading })).toBeInTheDocument();
    expect(within(result).getByText(T.goalsInfeasibleNotice)).toBeVisible();
    const expected = T.surviveOutcome(
      1,
      T.goalOpponentName(FISH.nameJa, attackerPresetLabel("x_full", "special")),
      MOVE_WATER.nameJa,
      2,
      false,
      formatChancePercent(87.56),
    );
    expect(within(result).getByText(expected)).toBeVisible();
    expect(formatChancePercent(87.56)).toBe("87.5%");
  });

  test("先に使う技で素早さが上がらない(ランク 0)なら、その旨の文。先に使う技なしなら前置きなし", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "outspeed", species: FISH, preset: "fastest", move: MOVE_FIRE });
    await addGoal(user, 2, { kind: "outspeed", species: BUG, preset: "none" });
    await user.click(submitButton());
    await respondBoth(
      client,
      goalsResult({
        feasible: false,
        goals: [
          outspeedOutcome({ met: false, selfSpeed: 150, opponentSpeed: 167, selfSpeedRank: 0 }),
          outspeedOutcome({ met: true, selfSpeed: 150, opponentSpeed: 130, selfSpeedRank: 0 }),
        ],
      }),
    );

    const result = resultRegion();
    expect(
      within(result).getByText(
        T.outspeedOutcome(1, T.goalOpponentName(FISH.nameJa, T.speedPresetOption.fastest), false, 150, 167, {
          moveName: MOVE_FIRE.nameJa,
          rank: 0,
        }),
      ),
    ).toBeVisible();
    expect(
      within(result).getByText(
        T.outspeedOutcome(2, T.goalOpponentName(BUG.nameJa, T.speedPresetOption.none), true, 150, 130, null),
      ),
    ).toBeVisible();
  });

  test("目標ごとの行の名前は送信した時点のもの(送信後に入力を変えても変わらない)", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await user.selectOptions(field(1, T.goalOpponentSpeciesFieldLabel), BUG.key);
    await respondBoth(client, goalsResult({ goals: [outspeedOutcome(), damageOutcome("ko")] }));
    expect(
      within(resultRegion()).getByText(
        T.outspeedOutcome(1, T.goalOpponentName(FISH.nameJa, T.speedPresetOption.fastest), true, 225, 167, {
          moveName: MOVE_BOOST.nameJa,
          rank: 1,
        }),
      ),
    ).toBeVisible();
  });

  test("未対応の印を結果の先頭に1回出す", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    const marks: Schemas["UnsupportedMark"][] = [
      { target: "attacker_ability", reason: "unsupported_effect", id: ABILITY.id },
    ];
    await respondBoth(
      client,
      goalsResult({ goals: [outspeedOutcome(), damageOutcome("ko")], unsupported: marks }),
    );
    // 既存の全モードと同じ文言(unsupportedMarkLabels を通す)。
    const notice = unsupportedText.notice(
      unsupportedMarkLabels(marks, master.moves, master.items, master.abilities),
    );
    expect(notice).toContain(ABILITY.nameJa);
    expect(within(resultRegion()).getByText(notice)).toBeVisible();
  });

  test("indices と goals の両方がそろうまで結果を出さない(計算中)", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await respond(lastCallOf(client, "goals"), {
      ok: true,
      value: goalsResult({ goals: [outspeedOutcome(), damageOutcome("ko")] }),
    });
    expect(resultRegion()).toHaveAttribute("aria-busy", "true");
    expect(within(resultRegion()).queryByRole("list", { name: T.goalOutcomesLabel })).toBeNull();
  });
});

// =====================================================================================
// G5 エラー
// =====================================================================================

describe("G5 エラー", () => {
  test("goals の失敗は code から日本語。サーバーの message は出さず、目標は消えない", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "goals"), {
      ok: false,
      error: { code: "invalid_input", message: "goals[1]: internal detail" },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(adjustErrorText.invalid_input);
    expect(screen.queryByText(/internal detail/)).toBeNull();
    expect(field(2, T.goalSelfMoveFieldLabel)).toHaveValue(MOVE_FIRE.id);
    expect(within(resultRegion()).queryByRole("list", { name: T.goalOutcomesLabel })).toBeNull();
  });

  test("段階 B 前の 404(not_found)も日本語の文言", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "goals"), { ok: false, error: { code: "not_found", message: "x" } });
    expect(await screen.findByRole("alert")).toHaveTextContent(adjustErrorText.not_found);
  });
});

// =====================================================================================
// G6 取り消し
// =====================================================================================

describe("G6 取り消しと古い応答", () => {
  test("送り直すと前の goals の呼び出しを abort し、遅れて届いた古い応答で上書きしない", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    const firstGoals = lastCallOf(client, "goals");
    const firstIndices = lastCallOf(client, "indices");
    await user.click(submitButton());
    expect(firstGoals.signal?.aborted).toBe(true);
    expect(callsOf(client, "goals")).toHaveLength(2);

    await respond(firstIndices, { ok: true, value: indicesResult });
    await respond(firstGoals, {
      ok: true,
      value: goalsResult({ goals: [outspeedOutcome(), damageOutcome("ko")] }),
    });
    expect(within(resultRegion()).queryByRole("list", { name: T.goalOutcomesLabel })).toBeNull();
  });

  test("request_aborted はエラーとして出さない", async () => {
    const { user, client } = renderScreen();
    await submitOutspeedAndKo(user);
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "goals"), {
      ok: false,
      error: { code: REQUEST_ABORTED_CODE, message: "x" },
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("画面を閉じたら計算中の goals を abort する", async () => {
    const { user, client, view } = renderScreen();
    await submitOutspeedAndKo(user);
    const call = lastCallOf(client, "goals");
    view.unmount();
    expect(call.signal?.aborted).toBe(true);
  });
});

// =====================================================================================
// G7 a11y
// =====================================================================================

describe("G7 a11y", () => {
  test("カードは group「目標 n」。欄の accessible name は「目標 n の<見えるラベル>」で見えるラベルを含む", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await addGoal(user, 1, { kind: "survive", species: FISH });

    for (const label of [
      T.goalKindFieldLabel,
      T.goalOpponentSpeciesFieldLabel,
      T.goalPresetFieldLabel,
      T.goalOpponentMoveFieldLabel,
      T.hitsLabel,
      T.thresholdLabel,
    ]) {
      const control = within(card(1)).getByRole("combobox", { name: T.goalFieldName(1, label) });
      expect(visibleLabelOf(control)).toBe(label);
      expect(accessibleNameOf(control)).toContain(label);
    }
    const remove = within(card(1)).getByRole("button", { name: T.removeGoalName(1) });
    expect(remove).toHaveTextContent(T.removeGoalLabel);
    expect(accessibleNameOf(remove)).toContain(T.removeGoalLabel);
  });

  test("先に使う技の欄は説明の文を aria-describedby で結ぶ", async () => {
    const { user } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "goals");
    await user.click(addButton());
    expect(describedByTextsOf(field(1, T.goalBoostMoveFieldLabel))).toContain(T.goalBoostMoveHint);
  });
});
