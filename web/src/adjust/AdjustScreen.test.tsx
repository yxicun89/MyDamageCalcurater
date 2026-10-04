// AJ6: 調整の画面(ADR-0319・受け入れ条件 AC2〜AC8)。
// AdjustClient は fake(props で注入)。engine(WASM)は使わず、master / masterSearch は入力補助にだけ使う。
// 確かめること:
//   S1 初期表示: 領域「自分」「調整の内容」「調整の結果」、モードの既定は「指数と 16n」、相手・目標の領域は出ない。
//      マウント・入力の変更だけでは API を呼ばない(「調整する」を押したときだけ。ADR-0705 §7 と同じ)
//   S2 request の組み立て(モードごと。契約 api/openapi.yaml のまま・省略可の欄は送らない)
//   S3 固定 SP とモードの切り替え(固定 SP は下限として送る・合計の表示・切り替えても入力が消えない)
//   S4 送信前の検査(違反していれば API を呼ばず、日本語の理由を role=alert で出す)
//   S5 結果の表示(指数・16n・最小 SP・配分・未対応の印・確率の切り捨て)
//   S6 エラー(code から日本語。サーバーの message は出さない。入力は消えない)
//   S7 古い応答・取り消し(再送信で前の呼び出しを abort し、遅れて届いた応答で上書きしない。画面を閉じたら abort)
//   S8 技を覚えるポケモンのパネル(ページング・空・エラー)
//   S9 a11y(見える見出し h2・見えるラベルが accessible name に含まれる・未選択の文言・radio group・aria-busy)
//   S10 マスタ(speciesList が false なら検索欄。技は種族の learnset から。変化技は選択肢に出さない)
// 架空データだけを使う(実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { MAX_SP_TOTAL, MAX_SP_PER_STAT } from "../domain/requests";
import { unsupportedMarkLabels } from "../domain/unsupportedLabels";
import { REQUEST_ABORTED_CODE, type Ability, type Item, type Move, type StatKey } from "../engine/types";
import { adjustErrorText, adjustScreenText, unsupportedText, type AdjustModeKey } from "../i18n/ja";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { accessibleNameOf, describedByTextsOf, visibleLabelOf } from "../test/accessibleName";
import { createFakeSpeciesSearch } from "../test/onlineMaster";
import { AdjustScreen } from "./AdjustScreen";
import type { AdjustClient, AdjustResult, LearnersPage } from "./adjustClient";
import { LEARNERS_PAGE_SIZE, STAB_MODIFIER } from "./adjustRequest";

type Schemas = components["schemas"];

const T = adjustScreenText;

// ---- fake の AdjustClient(呼び出しを記録し、テストがあとから応答を返す) ----

type Method = keyof AdjustClient;

interface RecordedCall {
  readonly method: Method;
  /** 呼び出しの引数(signal を除く。structuredClone 済み)。 */
  readonly args: readonly unknown[];
  readonly signal: AbortSignal | undefined;
  resolve(result: AdjustResult<unknown>): void;
}

interface FakeAdjustClient extends AdjustClient {
  readonly calls: RecordedCall[];
}

function createFakeAdjustClient(): FakeAdjustClient {
  const calls: RecordedCall[] = [];
  // 応答の型は操作ごとに違うが、テストが操作に合う応答を返す(AdjustResult<never> はどの AdjustResult<T> にも代入できる)。
  function record(
    method: Method,
    args: readonly unknown[],
    signal: AbortSignal | undefined,
  ): Promise<AdjustResult<never>> {
    return new Promise((resolve) => {
      calls.push({
        method,
        args: structuredClone(args),
        signal,
        resolve: (result) => {
          resolve(result as AdjustResult<never>);
        },
      });
    });
  }
  return {
    calls,
    indices: (request, signal) => record("indices", [request], signal),
    minSpToKo: (request, signal) => record("minSpToKo", [request], signal),
    minSpToSurvive: (request, signal) => record("minSpToSurvive", [request], signal),
    allocation: (request, signal) => record("allocation", [request], signal),
    goals: (request, signal) => record("goals", [request], signal),
    moveLearners: (moveId: string, page: LearnersPage, signal?: AbortSignal) =>
      record("moveLearners", [moveId, page], signal),
  };
}

function callsOf(client: FakeAdjustClient, method: Method): RecordedCall[] {
  return client.calls.filter((call) => call.method === method);
}

function lastCallOf(client: FakeAdjustClient, method: Method): RecordedCall {
  const call = callsOf(client, method).at(-1);
  if (call === undefined) {
    throw new Error(`${method} が呼ばれていない`);
  }
  return call;
}

/** 最後の呼び出しの request(第1引数)。 */
function lastRequestOf(client: FakeAdjustClient, method: Method): unknown {
  return lastCallOf(client, method).args[0];
}

async function respond(call: RecordedCall, result: AdjustResult<unknown>): Promise<void> {
  await act(async () => {
    call.resolve(result);
    await Promise.resolve();
  });
}

// ---- 架空のマスタ(9xxx-xxx の種族・test-* の ID。実マスタは使わない) ----

const MOVE_FIRE: Move = {
  id: "test-move-fire",
  nameJa: "テストほのおパンチ",
  type: "fire",
  category: "physical",
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

/** 自分に使う種族(ほのお・ひこう)。ほのおの技はタイプ一致、みずの技は不一致。 */
const BIRD = fakeSpecies(
  "9001-000",
  "テストカソウドリ",
  ["fire", "flying"],
  [MOVE_FIRE.id, MOVE_WATER.id, MOVE_STATUS.id],
);
/** 相手に使う種族(みず)。 */
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"], [MOVE_WATER.id, MOVE_TACKLE.id]);

const NATURE_NEUTRAL: MasterNature = {
  id: "test-nature-neutral",
  nameJa: "テストまじめ",
  plus: null,
  minus: null,
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
const ITEM: Item = { id: "test-item-a", nameJa: "テストもちもの", effect: null };

const master: MasterData = {
  species: [BIRD, FISH],
  moves: [MOVE_FIRE, MOVE_WATER, MOVE_TACKLE, MOVE_STATUS],
  items: [ITEM],
  abilities: [ABILITY],
  natures: [NATURE_NEUTRAL, NATURE_PLUS_ATK, NATURE_PLUS_SPA, NATURE_PLUS_DEF, NATURE_PLUS_SPD],
  typeChart: { types: ["fire", "water", "flying", "normal"], effectiveness: {} },
  capabilities: { speciesList: true, moves: true, effects: true },
};

const ZERO: Schemas["StatBlock"] = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

/** 自分(BIRD・まじめ・SP は固定した値)の Individual(省略可の欄は持たない)。 */
function selfIndividual(sp: Partial<Schemas["StatBlock"]> = {}): Schemas["Individual"] {
  return { speciesKey: BIRD.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO, ...sp } };
}

// ---- 架空の応答 ----

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
    prev16n: { hp: 144, sp: 0, spDelta: -4 },
    next16nMinus1: { hp: 175, sp: 20, spDelta: 16 },
    prev16nMinus1: null,
  },
};

function plan(overrides: Partial<Schemas["AdjustAllocPlan"]> = {}): Schemas["AdjustAllocPlan"] {
  return {
    sp: { hp: 32, atk: 0, def: 18, spa: 0, spd: 16, spe: 0 },
    totalSp: 66,
    stats: { hp: 187, atk: 120, def: 108, spa: 80, spd: 106, spe: 130 },
    physicalBulk: 20196,
    specialBulk: 19822,
    speedMet: true,
    goalMet: false,
    chancePercent: 0,
    ...overrides,
  };
}

// ---- 描画と、よく使う問い合わせ ----

interface RenderOptions {
  readonly master?: MasterData;
  readonly masterSearch?: ReturnType<typeof createFakeSpeciesSearch>;
}

function renderScreen(options: RenderOptions = {}) {
  const user = userEvent.setup();
  const client = createFakeAdjustClient();
  const view = render(
    <AdjustScreen
      adjustClient={client}
      master={options.master ?? master}
      masterSearch={options.masterSearch}
    />,
  );
  return { user, client, view };
}

function region(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

function selfRegion(): HTMLElement {
  return region(T.selfRegionLabel);
}

function resultRegion(): HTMLElement {
  return region(T.resultRegionLabel);
}

function submitButton(): HTMLElement {
  return screen.getByRole("button", { name: T.submitLabel });
}

function fixedSpInput(stat: StatKey): HTMLElement {
  return screen.getByRole("spinbutton", { name: T.fixedSpLabel(stat) });
}

function ceilingInput(stat: StatKey): HTMLElement {
  return screen.getByRole("spinbutton", { name: T.ceilingLabel(stat) });
}

/** 数値入力を入れ直す(既定値の扱いに依らず、値を直接置き換える)。 */
function setNumber(input: HTMLElement, value: number | string): void {
  fireEvent.change(input, { target: { value: String(value) } });
}

async function chooseMode(user: UserEvent, mode: AdjustModeKey): Promise<void> {
  await user.click(screen.getByRole("radio", { name: T.modeLabel[mode] }));
}

interface SelfInput {
  readonly species?: MasterSpecies;
  readonly nature?: MasterNature;
  readonly move?: Move;
}

async function fillSelf(user: UserEvent, input: SelfInput = {}): Promise<void> {
  await user.selectOptions(
    screen.getByRole("combobox", { name: T.selfSpeciesLabel }),
    (input.species ?? BIRD).key,
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: T.selfNatureLabel }),
    (input.nature ?? NATURE_NEUTRAL).id,
  );
  if (input.move !== undefined) {
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfMoveLabel }), input.move.id);
  }
}

interface OpponentInput {
  readonly species?: MasterSpecies;
  /** 相手の調整(プリセットの key。attacker/defender のどちらのプリセットかはモードで決まる)。 */
  readonly preset?: string;
  readonly move?: Move;
}

async function fillOpponent(user: UserEvent, input: OpponentInput = {}): Promise<void> {
  await user.selectOptions(
    screen.getByRole("combobox", { name: T.opponentSpeciesLabel }),
    (input.species ?? FISH).key,
  );
  if (input.move !== undefined) {
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentMoveLabel }), input.move.id);
  }
  if (input.preset !== undefined) {
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentPresetLabel }), input.preset);
  }
}

async function setGoal(user: UserEvent, hits: number, thresholdPercent?: number): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: T.hitsLabel }), String(hits));
  if (thresholdPercent !== undefined) {
    await user.selectOptions(
      screen.getByRole("combobox", { name: T.thresholdLabel }),
      String(thresholdPercent),
    );
  }
}

// =====================================================================================
// S1 初期表示・呼び出しのタイミング
// =====================================================================================

describe("S1 初期表示と、API を呼ぶタイミング", () => {
  test("領域「自分」「調整の内容」「調整の結果」を出し、相手・目標・技を覚えるポケモンの領域は出さない", () => {
    renderScreen();
    expect(selfRegion()).toBeInTheDocument();
    expect(region(T.modeRegionLabel)).toBeInTheDocument();
    expect(resultRegion()).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: T.opponentRegionLabel })).toBeNull();
    expect(screen.queryByRole("region", { name: T.goalRegionLabel })).toBeNull();
    expect(screen.queryByRole("region", { name: T.learnersRegionLabel })).toBeNull();
    expect(within(resultRegion()).getByText(T.emptyResultNotice)).toBeInTheDocument();
  });

  test("モードの既定は「指数と 16n を見る」", () => {
    renderScreen();
    expect(screen.getByRole("radio", { name: T.modeLabel.indices })).toBeChecked();
  });

  test("マウントしただけ・入力を変えただけでは API を呼ばない(「調整する」を押したときだけ)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("hp"), 4);
    await chooseMode(user, "bulk");
    setNumber(ceilingInput("hp"), 20);
    await chooseMode(user, "minKo");
    await fillOpponent(user, { preset: "hb" });
    await setGoal(user, 2, 50);
    expect(client.calls).toHaveLength(0);
  });

  test("「調整する」を1回押すと、指数のモードでは indices を1回だけ呼ぶ", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    expect(client.calls.map((call) => call.method)).toEqual(["indices"]);
  });
});

// =====================================================================================
// S2 request の組み立て
// =====================================================================================

describe("S2 request の組み立て(契約のまま・省略可の欄は送らない)", () => {
  test("指数: タイプ一致の技なら moveId と modifier = 6144 を送る(damageModifier は送らない)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("hp"), 4);
    await user.click(submitButton());

    expect(lastRequestOf(client, "indices")).toEqual({
      individual: selfIndividual({ hp: 4 }),
      moveId: MOVE_FIRE.id,
      modifier: STAB_MODIFIER,
    } satisfies Schemas["AdjustIndicesRequest"]);
  });

  test("指数: タイプ不一致の技なら modifier を送らない(既定 4096 はサーバーが補う)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_WATER });
    await user.click(submitButton());

    expect(lastRequestOf(client, "indices")).toEqual({
      individual: selfIndividual(),
      moveId: MOVE_WATER.id,
    });
  });

  test("指数: 技を選ばなければ moveId・modifier を送らない(firepowerIndex は null で返る)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());

    expect(lastRequestOf(client, "indices")).toEqual({ individual: selfIndividual() });
  });

  test("特性・持ち物は選んだときだけ送る", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfAbilityLabel }), ABILITY.id);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfItemLabel }), ITEM.id);
    await user.click(submitButton());

    expect(lastRequestOf(client, "indices")).toEqual({
      individual: { ...selfIndividual(), abilityId: ABILITY.id, itemId: ITEM.id },
    });
  });

  test("倒せる最小: attacker = 自分、defender = 相手(防御側プリセットの SP・性格)、moveId = 自分の技", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("spe"), 32);
    await chooseMode(user, "minKo");
    await fillOpponent(user, { preset: "hb_full" });
    await setGoal(user, 2);
    await user.click(submitButton());

    expect(lastRequestOf(client, "minSpToKo")).toEqual({
      format: "single",
      attacker: selfIndividual({ spe: 32 }),
      defender: {
        speciesKey: FISH.key,
        level: 50,
        natureId: NATURE_PLUS_DEF.id,
        sp: { ...ZERO, hp: 32, def: 32 },
      },
      moveId: MOVE_FIRE.id,
      hits: 2,
    } satisfies Schemas["AdjustSearchRequest"]);
    // 今の振り方の指数も同じ送信で出す(ADR-0319 §2)。
    expect(lastRequestOf(client, "indices")).toEqual({
      individual: selfIndividual({ spe: 32 }),
      moveId: MOVE_FIRE.id,
      modifier: STAB_MODIFIER,
    });
  });

  test("確率を確定以外にしたときだけ thresholdPercent を送る", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "minKo");
    await fillOpponent(user, { preset: "none" });
    await setGoal(user, 3, 50);
    await user.click(submitButton());

    expect(lastRequestOf(client, "minSpToKo") as Schemas["AdjustSearchRequest"]).toMatchObject({
      hits: 3,
      thresholdPercent: 50,
      defender: { speciesKey: FISH.key, natureId: NATURE_NEUTRAL.id, sp: ZERO },
    });
  });

  test("耐えられる最小: attacker = 相手(攻撃側プリセット)、defender = 自分、moveId = 相手の技", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("spe"), 10);
    await chooseMode(user, "minSurvive");
    await fillOpponent(user, { move: MOVE_WATER, preset: "x_full" });
    await setGoal(user, 1);
    await user.click(submitButton());

    expect(lastRequestOf(client, "minSpToSurvive")).toEqual({
      format: "single",
      attacker: {
        speciesKey: FISH.key,
        level: 50,
        natureId: NATURE_PLUS_SPA.id,
        sp: { ...ZERO, spa: 32 },
      },
      defender: selfIndividual({ spe: 10 }),
      moveId: MOVE_WATER.id,
      hits: 1,
    } satisfies Schemas["AdjustSearchRequest"]);
  });

  test("耐久に振る(目標なし): mode=bulk・focus・ceiling は H/B/D だけ・minSpeed 0・goal を送らない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("spe"), 32);
    await chooseMode(user, "bulk");
    await user.click(submitButton());

    expect(lastRequestOf(client, "allocation")).toEqual({
      self: selfIndividual({ spe: 32 }),
      mode: "bulk",
      focus: "both",
      ceiling: { hp: 32, def: 32, spd: 32 },
      minSpeed: 0,
    } satisfies Schemas["AdjustAllocationRequest"]);
  });

  test("耐久に振る: 耐久の基準と上限は使用者が決めた値を送る(H/B/D の配分は自分の意思で)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.selectOptions(screen.getByRole("combobox", { name: T.focusLabel }), "physical");
    setNumber(ceilingInput("hp"), 20);
    setNumber(ceilingInput("spd"), 0);
    await user.click(submitButton());

    expect(lastRequestOf(client, "allocation") as Schemas["AdjustAllocationRequest"]).toMatchObject({
      mode: "bulk",
      focus: "physical",
      ceiling: { hp: 20, def: 32, spd: 0 },
    });
  });

  test("耐久に振る(目標あり): goal = 相手(攻撃側プリセット)・相手の技・発数", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    await fillOpponent(user, { move: MOVE_TACKLE, preset: "x" });
    await setGoal(user, 2, 90);
    await user.click(submitButton());

    expect((lastRequestOf(client, "allocation") as Schemas["AdjustAllocationRequest"]).goal).toEqual({
      format: "single",
      opponent: { speciesKey: FISH.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO, atk: 32 } },
      moveId: MOVE_TACKLE.id,
      hits: 2,
      thresholdPercent: 90,
    } satisfies Schemas["AdjustAllocGoal"]);
  });

  test("攻撃と素早さに振る(目標なし): offenseCategory は自分の技の分類に合わせ、ceiling は A(C) と S だけ", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_WATER });
    await chooseMode(user, "offense");
    setNumber(screen.getByRole("spinbutton", { name: T.minSpeedLabel }), 150);
    await user.click(submitButton());

    expect(lastRequestOf(client, "allocation")).toEqual({
      self: selfIndividual(),
      mode: "offense",
      offenseCategory: "special",
      ceiling: { spa: 32, spe: 32 },
      minSpeed: 150,
    } satisfies Schemas["AdjustAllocationRequest"]);
  });

  test("攻撃と素早さに振る: 素早さの目標が空なら minSpeed 0(目標なし)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "offense");
    await user.click(submitButton());

    expect(lastRequestOf(client, "allocation") as Schemas["AdjustAllocationRequest"]).toMatchObject({
      offenseCategory: "physical",
      ceiling: { atk: 32, spe: 32 },
      minSpeed: 0,
    });
  });

  test("攻撃と素早さに振る(目標あり): goal = 相手(防御側プリセット)・自分の技", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "offense");
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    await fillOpponent(user, { preset: "hb" });
    await setGoal(user, 1);
    await user.click(submitButton());

    expect((lastRequestOf(client, "allocation") as Schemas["AdjustAllocationRequest"]).goal).toEqual({
      format: "single",
      opponent: {
        speciesKey: FISH.key,
        level: 50,
        natureId: NATURE_NEUTRAL.id,
        sp: { ...ZERO, hp: 32, def: 32 },
      },
      moveId: MOVE_FIRE.id,
      hits: 1,
    } satisfies Schemas["AdjustAllocGoal"]);
  });
});

// =====================================================================================
// S3 固定 SP とモードの切り替え
// =====================================================================================

describe("S3 固定 SP とモードの切り替え", () => {
  test("固定する能力ポイントの合計を「合計 n / 66」で見せる", () => {
    renderScreen();
    setNumber(fixedSpInput("hp"), 4);
    setNumber(fixedSpInput("atk"), 10);
    expect(within(selfRegion()).getByText(T.fixedSpTotal(14, MAX_SP_TOTAL))).toBeInTheDocument();
  });

  test("固定 SP の6欄はどれも既定 0", () => {
    renderScreen();
    for (const stat of ["hp", "atk", "def", "spa", "spd", "spe"] as const) {
      expect(fixedSpInput(stat)).toHaveValue(0);
    }
  });

  test("上限の欄は既定 32(省略と同じ。ADR-0250 §3)", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "bulk");
    for (const stat of ["hp", "def", "spd"] as const) {
      expect(ceilingInput(stat)).toHaveValue(MAX_SP_PER_STAT);
    }
    // 素早さは見ない(耐久側に S の上限の欄を出さない)。
    expect(screen.queryByRole("spinbutton", { name: T.ceilingLabel("spe") })).toBeNull();
  });

  test("モードごとに出す欄が変わる(相手・目標は相手が要るモードだけ。相手の技は相手が攻撃するときだけ)", async () => {
    const { user } = renderScreen();

    await chooseMode(user, "bulk");
    expect(screen.getByRole("combobox", { name: T.focusLabel })).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: T.opponentRegionLabel })).toBeNull();
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    expect(region(T.opponentRegionLabel)).toBeInTheDocument();
    expect(region(T.goalRegionLabel)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: T.opponentMoveLabel })).toBeInTheDocument();

    await chooseMode(user, "minKo");
    expect(region(T.opponentRegionLabel)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: T.opponentMoveLabel })).toBeNull();
    expect(screen.queryByRole("combobox", { name: T.focusLabel })).toBeNull();

    await chooseMode(user, "minSurvive");
    expect(screen.getByRole("combobox", { name: T.opponentMoveLabel })).toBeInTheDocument();

    await chooseMode(user, "offense");
    expect(screen.getByRole("combobox", { name: T.offenseCategoryLabel })).toBeInTheDocument();
    expect(screen.getByRole("spinbutton", { name: T.minSpeedLabel })).toBeInTheDocument();

    await chooseMode(user, "indices");
    expect(screen.queryByRole("region", { name: T.opponentRegionLabel })).toBeNull();
    expect(screen.queryByRole("region", { name: T.goalRegionLabel })).toBeNull();
  });

  test("モードを切り替えても自分の入力と固定 SP は消えない", async () => {
    const { user } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("hp"), 4);
    await chooseMode(user, "minSurvive");
    await chooseMode(user, "indices");
    expect(screen.getByRole("combobox", { name: T.selfSpeciesLabel })).toHaveValue(BIRD.key);
    expect(screen.getByRole("combobox", { name: T.selfMoveLabel })).toHaveValue(MOVE_FIRE.id);
    expect(fixedSpInput("hp")).toHaveValue(4);
  });

  test("相手の調整の選択肢はプリセット(相手が攻撃するなら攻撃側、受けるなら防御側のカタログ)", async () => {
    const { user } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });

    await chooseMode(user, "minKo");
    const defenderPresets = screen.getByRole("combobox", { name: T.opponentPresetLabel });
    const defenderValues = within(defenderPresets)
      .getAllByRole("option")
      .map((option) => (option as HTMLOptionElement).value);
    // 物理技に対する防御側プリセット(engine の DefaultDefenderPresets と同じ絞り込み)。
    expect(defenderValues).toEqual(["none", "hp", "hb_boost", "hb", "hb_full"]);

    await chooseMode(user, "minSurvive");
    const attackerPresets = screen.getByRole("combobox", { name: T.opponentPresetLabel });
    const attackerValues = within(attackerPresets)
      .getAllByRole("option")
      .map((option) => (option as HTMLOptionElement).value);
    expect(attackerValues).toEqual(["none", "x_full", "x"]);
  });
});

// =====================================================================================
// S4 送信前の検査
// =====================================================================================

describe("S4 送信前の検査(API を呼ばずに日本語の理由を出す)", () => {
  async function expectBlocked(client: FakeAdjustClient, user: UserEvent, message: string): Promise<void> {
    await user.click(submitButton());
    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(message);
  }

  test("自分のポケモン・性格が無い", async () => {
    const { user, client } = renderScreen();
    await expectBlocked(client, user, T.selfRequiredMessage);
  });

  test("固定 SP が 0〜32 の外", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 33);
    await expectBlocked(client, user, T.spRangeMessage(MAX_SP_PER_STAT));
  });

  test.each([-1, 1.5])("固定 SP が %s(範囲の外・整数でない)", async (value) => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("atk"), value);
    await expectBlocked(client, user, T.spRangeMessage(MAX_SP_PER_STAT));
  });

  test("固定 SP の合計が 66 を超える", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 32);
    setNumber(fixedSpInput("atk"), 32);
    setNumber(fixedSpInput("def"), 3);
    await expectBlocked(client, user, T.spTotalMessage(MAX_SP_TOTAL));
  });

  test("固定 SP の合計がちょうど 66 なら送れる(境界)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 32);
    setNumber(fixedSpInput("atk"), 32);
    setNumber(fixedSpInput("def"), 2);
    await user.click(submitButton());
    expect(callsOf(client, "indices")).toHaveLength(1);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("倒せる最小: 自分の技が無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "minKo");
    await fillOpponent(user);
    await expectBlocked(client, user, T.selfMoveRequiredMessage);
  });

  test("倒せる最小: 相手のポケモンが無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "minKo");
    await expectBlocked(client, user, T.opponentRequiredMessage);
  });

  test("耐えられる最小: 相手の技が無い", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "minSurvive");
    await fillOpponent(user);
    await expectBlocked(client, user, T.opponentMoveRequiredMessage);
  });

  test("耐久に振る: 上限が固定 SP より小さい", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 10);
    await chooseMode(user, "bulk");
    setNumber(ceilingInput("hp"), 5);
    await expectBlocked(client, user, T.ceilingBelowFixedMessage);
  });

  test("攻撃と素早さに振る: S の上限が固定 SP より小さい", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("spe"), 10);
    await chooseMode(user, "offense");
    setNumber(ceilingInput("spe"), 5);
    await expectBlocked(client, user, T.ceilingBelowFixedMessage);
  });

  test("攻撃と素早さに振る(目標あり): 攻撃の分類と自分の技の分類が違う", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "offense");
    await user.selectOptions(screen.getByRole("combobox", { name: T.offenseCategoryLabel }), "special");
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    await fillOpponent(user);
    await expectBlocked(client, user, T.categoryMismatchMessage);
  });

  test("攻撃と素早さに振る: 素早さの目標が負", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "offense");
    setNumber(screen.getByRole("spinbutton", { name: T.minSpeedLabel }), -1);
    await expectBlocked(client, user, T.minSpeedMessage);
  });

  test("相手の調整に合う性格がマスタに無い(別の性格で代えずに止める)", async () => {
    const { user, client } = renderScreen({
      master: { ...master, natures: master.natures.filter((nature) => nature.id !== NATURE_PLUS_SPA.id) },
    });
    await fillSelf(user);
    await chooseMode(user, "minSurvive");
    await fillOpponent(user, { move: MOVE_WATER, preset: "x_full" });
    await expectBlocked(client, user, T.natureNotFoundMessage);
  });

  test("検査で止めても入力は消えず、直せばそのまま送れる", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 33);
    await user.click(submitButton());
    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("combobox", { name: T.selfSpeciesLabel })).toHaveValue(BIRD.key);

    setNumber(fixedSpInput("hp"), 32);
    await user.click(submitButton());
    expect(callsOf(client, "indices")).toHaveLength(1);
  });
});

// =====================================================================================
// S5 結果の表示
// =====================================================================================

describe("S5 結果の表示", () => {
  test("指数: 実数値・火力指数・物理/特殊耐久指数・補正の注記を出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });

    const indices = region(T.indicesHeading);
    expect(within(indices).getByText(T.statsLine(indicesResult.stats))).toBeInTheDocument();
    expect(within(indices).getByText(T.indexLine(T.firepowerIndexLabel, 16200))).toBeInTheDocument();
    expect(within(indices).getByText(T.indexLine(T.physicalBulkLabel, 14310))).toBeInTheDocument();
    expect(within(indices).getByText(T.indexLine(T.specialBulkLabel, 14469))).toBeInTheDocument();
    expect(within(indices).getByText(T.indexNote)).toBeInTheDocument();
  });

  test("指数: 火力指数が null(技なし)なら「技を選ぶと出します」", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), {
      ok: true,
      value: { ...indicesResult, firepowerIndex: null },
    });

    expect(
      within(region(T.indicesHeading)).getByText(T.indexLine(T.firepowerIndexLabel, T.firepowerIndexNone)),
    ).toBeInTheDocument();
  });

  test("16n: 今の HP とライン、次・前の 16n / 16n-1(無ければ「なし」)を出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });

    const indices = region(T.indicesHeading);
    expect(within(indices).getByText(T.hpCurrent(159, T.hpLineKindLabel["16n-1"]))).toBeInTheDocument();
    expect(within(indices).getByText(T.hpLinePoint(T.next16nLabel, 160, 5, 1))).toBeInTheDocument();
    expect(within(indices).getByText(T.hpLinePoint(T.prev16nLabel, 144, 0, -4))).toBeInTheDocument();
    expect(within(indices).getByText(T.hpLinePoint(T.next16nMinus1Label, 175, 20, 16))).toBeInTheDocument();
    expect(within(indices).getByText(T.hpLineNone(T.prev16nMinus1Label))).toBeInTheDocument();
    // 差の符号: 次は「+」付き、前は「-」。
    expect(T.hpLinePoint(T.next16nLabel, 160, 5, 1)).toContain("+1");
    expect(T.hpLinePoint(T.prev16nLabel, 144, 0, -4)).toContain("-4");
  });

  test("16n: ラインに乗っていないときは「16n でも 16n-1 でもない」", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), {
      ok: true,
      value: { ...indicesResult, hpLines: { ...indicesResult.hpLines, hp: 158, current: "none" } },
    });
    expect(
      within(region(T.indicesHeading)).getByText(T.hpCurrent(158, T.hpLineKindLabel.none)),
    ).toBeInTheDocument();
  });

  async function submitMinKo() {
    const rendered = renderScreen();
    await fillSelf(rendered.user, { move: MOVE_FIRE });
    await chooseMode(rendered.user, "minKo");
    await fillOpponent(rendered.user, { preset: "hb" });
    await setGoal(rendered.user, 2);
    await rendered.user.click(submitButton());
    await respond(lastCallOf(rendered.client, "indices"), { ok: true, value: indicesResult });
    return rendered;
  }

  test("倒せる最小(満たせる): 「A に n 振れば m発で倒せます(確率 100%)」", async () => {
    const { client } = await submitMinKo();
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: true, sp: 12, chancePercent: 100, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);

    expect(within(resultRegion()).getByText(T.koFeasible("atk", 12, 2, "100%"))).toBeInTheDocument();
  });

  test("倒せる最小(満たせない): 上限まで振っても届かないことと、届く確率(切り捨て)を出す", async () => {
    const { client } = await submitMinKo();
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: false, sp: 32, chancePercent: 99.99, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);

    // 99.99% を「100%」と出さない(確定に見せない)。
    expect(within(resultRegion()).getByText(T.koInfeasible("atk", 32, 2, "99.9%"))).toBeInTheDocument();
    expect(within(resultRegion()).queryByText(/100%/)).toBeNull();
  });

  test("耐えられる最小: 満たせる組と満たせない組の文言", async () => {
    const rendered = renderScreen();
    await fillSelf(rendered.user);
    await chooseMode(rendered.user, "minSurvive");
    await fillOpponent(rendered.user, { move: MOVE_WATER });
    await setGoal(rendered.user, 1);
    await rendered.user.click(submitButton());
    await respond(lastCallOf(rendered.client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(rendered.client, "minSpToSurvive"), {
      ok: true,
      value: {
        stat: "spd",
        searchLimit: 66,
        feasible: true,
        hpSp: 20,
        statSp: 12,
        totalSp: 32,
        bulkIndex: 20000,
        chancePercent: 100,
        unsupported: [],
      },
    } satisfies AdjustResult<Schemas["AdjustSurviveResult"]>);
    expect(within(resultRegion()).getByText(T.surviveFeasible("spd", 20, 12, 1, "100%"))).toBeInTheDocument();

    await rendered.user.click(submitButton());
    await respond(lastCallOf(rendered.client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(rendered.client, "minSpToSurvive"), {
      ok: true,
      value: {
        stat: "spd",
        searchLimit: 66,
        feasible: false,
        hpSp: 32,
        statSp: 30,
        totalSp: 62,
        bulkIndex: 30000,
        chancePercent: 87.5,
        unsupported: [],
      },
    } satisfies AdjustResult<Schemas["AdjustSurviveResult"]>);
    expect(
      within(resultRegion()).getByText(T.surviveInfeasible("spd", 32, 30, 1, "87.5%")),
    ).toBeInTheDocument();
  });

  test("耐久に振る(目標なし): 残り SP・指数最大の振り方を出し、最小の振り方は「目標を指定すると…」", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "allocation"), {
      ok: true,
      value: { remaining: 66, maxIndex: plan(), minSp: null, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustAllocationResult"]>);

    expect(within(resultRegion()).getByText(T.remainingLabel(66))).toBeInTheDocument();
    const maxIndex = region(T.maxIndexHeading);
    expect(within(maxIndex).getByText(T.planSpLine(plan().sp))).toBeInTheDocument();
    expect(within(maxIndex).getByText(T.planTotal(66))).toBeInTheDocument();
    expect(within(maxIndex).getByText(T.indexLine(T.physicalBulkLabel, 20196))).toBeInTheDocument();
    expect(within(maxIndex).getByText(T.indexLine(T.specialBulkLabel, 19822))).toBeInTheDocument();
    // 耐久側は素早さを見ないので、素早さの目標の文言を出さない。
    expect(within(maxIndex).queryByText(T.speedMet)).toBeNull();
    expect(screen.queryByRole("region", { name: T.minSpHeading })).toBeNull();
    expect(within(resultRegion()).getByText(T.minSpNotRequested)).toBeInTheDocument();
  });

  test("目標あり: 最小の振り方と、目標を満たすか(確率)を両方の案に出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    await fillOpponent(user, { move: MOVE_TACKLE });
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    const minSp = plan({
      sp: { hp: 12, atk: 0, def: 4, spa: 0, spd: 0, spe: 0 },
      totalSp: 16,
      goalMet: true,
      chancePercent: 100,
    });
    await respond(lastCallOf(client, "allocation"), {
      ok: true,
      value: { remaining: 66, maxIndex: plan({ goalMet: true, chancePercent: 100 }), minSp, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustAllocationResult"]>);

    const minSpRegion = region(T.minSpHeading);
    expect(within(minSpRegion).getByText(T.planSpLine(minSp.sp))).toBeInTheDocument();
    expect(within(minSpRegion).getByText(T.planTotal(16))).toBeInTheDocument();
    expect(within(minSpRegion).getByText(T.goalMet("100%"))).toBeInTheDocument();
    expect(within(region(T.maxIndexHeading)).getByText(T.goalMet("100%"))).toBeInTheDocument();
  });

  test("目標に届かない最小の組は「目標に届きません」を出す(満たしたように見せない)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
    await fillOpponent(user, { move: MOVE_TACKLE });
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "allocation"), {
      ok: true,
      value: {
        remaining: 66,
        maxIndex: plan({ goalMet: false, chancePercent: 62.5 }),
        minSp: plan({ goalMet: false, chancePercent: 62.5 }),
        unsupported: [],
      },
    } satisfies AdjustResult<Schemas["AdjustAllocationResult"]>);

    expect(within(region(T.minSpHeading)).getByText(T.goalNotMet("62.5%"))).toBeInTheDocument();
    expect(within(region(T.minSpHeading)).queryByText(T.goalMet("62.5%"))).toBeNull();
  });

  test("攻撃と素早さに振る: 素早さの目標を満たすか(speedMet)を出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await chooseMode(user, "offense");
    setNumber(screen.getByRole("spinbutton", { name: T.minSpeedLabel }), 200);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "allocation"), {
      ok: true,
      value: {
        remaining: 66,
        maxIndex: plan({ sp: { ...ZERO, atk: 32, spe: 32 }, totalSp: 64, speedMet: false }),
        minSp: null,
        unsupported: [],
      },
    } satisfies AdjustResult<Schemas["AdjustAllocationResult"]>);

    expect(within(region(T.maxIndexHeading)).getByText(T.speedNotMet)).toBeInTheDocument();
  });

  test("未対応の印があれば、名前をマスタから引いて結果の先頭に注意を出す(数値は変えない)", async () => {
    const { client } = await submitMinKo();
    const marks: Schemas["UnsupportedMark"][] = [
      { target: "move", reason: "unsupported_effect", id: MOVE_FIRE.id },
    ];
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: true, sp: 12, chancePercent: 100, unsupported: marks },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);

    const notice = unsupportedText.notice(
      unsupportedMarkLabels(marks, master.moves, master.items, master.abilities),
    );
    expect(notice).toContain(MOVE_FIRE.nameJa);
    expect(within(resultRegion()).getByText(notice)).toBeInTheDocument();
    expect(within(resultRegion()).getByText(T.koFeasible("atk", 12, 2, "100%"))).toBeInTheDocument();
  });

  test("オンライン相当のマスタ(技・特性の一覧が空)でも、特性名・技名を解決済みの一覧から日本語で出す", async () => {
    const search = createFakeSpeciesSearch({
      species: [BIRD, FISH],
      abilities: [ABILITY],
      moves: [MOVE_FIRE, MOVE_WATER, MOVE_TACKLE, MOVE_STATUS],
    });
    const { user, client } = renderScreen({
      master: {
        ...master,
        species: [],
        moves: [],
        abilities: [],
        capabilities: { speciesList: false, moves: true, effects: true },
      },
      masterSearch: search,
    });
    await user.type(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.nameJa);
    await user.click(await within(selfRegion()).findByRole("option", { name: BIRD.nameJa }));
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE_NEUTRAL.id);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfMoveLabel }), MOVE_FIRE.id);
    await chooseMode(user, "minKo");
    await user.type(screen.getByRole("combobox", { name: T.opponentSpeciesLabel }), FISH.nameJa);
    await user.click(await screen.findByRole("option", { name: FISH.nameJa }));
    await user.selectOptions(screen.getByRole("combobox", { name: T.opponentPresetLabel }), "hb");
    await setGoal(user, 2);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    const marks: Schemas["UnsupportedMark"][] = [
      { target: "move", reason: "unsupported_effect", id: MOVE_FIRE.id },
      { target: "attacker_ability", reason: "unsupported_effect", id: ABILITY.id },
    ];
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: true, sp: 12, chancePercent: 100, unsupported: marks },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);

    const notice = within(resultRegion()).getByText(/未対応/);
    expect(notice).toHaveTextContent(MOVE_FIRE.nameJa);
    expect(notice).toHaveTextContent(ABILITY.nameJa);
    expect(notice).not.toHaveTextContent(MOVE_FIRE.id);
    expect(notice).not.toHaveTextContent(ABILITY.id);
  });

  test("未対応の印が無ければ注意を出さない", async () => {
    const { client } = await submitMinKo();
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: true, sp: 12, chancePercent: 100, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);
    expect(within(resultRegion()).queryByText(/未対応/)).toBeNull();
  });

  test("応答がそろうまで結果を出さない(指数だけ先に届いても、片方だけの結果を出さない)", async () => {
    const { client } = await submitMinKo();
    expect(screen.queryByRole("region", { name: T.indicesHeading })).toBeNull();
    await respond(lastCallOf(client, "minSpToKo"), {
      ok: true,
      value: { stat: "atk", searchLimit: 32, feasible: true, sp: 12, chancePercent: 100, unsupported: [] },
    } satisfies AdjustResult<Schemas["AdjustKOResult"]>);
    expect(region(T.indicesHeading)).toBeInTheDocument();
  });
});

// =====================================================================================
// S6 エラー
// =====================================================================================

describe("S6 エラー(code から日本語。サーバーの message は出さない)", () => {
  test.each([
    ["invalid_input", "hits must be in 1..10"],
    ["unknown_move", "unknown moveId: test-move-fire"],
    ["master_unavailable", "master is not ready"],
    ["adjust_unavailable", "調整のサーバーに接続できません"],
  ] as const)("%s → adjustErrorText の文言", async (code, message) => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: false, error: { code, message } });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(adjustErrorText[code]);
    if (message !== adjustErrorText[code]) {
      expect(screen.queryByText(new RegExp(message.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")))).toBeNull();
    }
  });

  test("未知のコードは汎用の文言(英語の message をそのまま出さない)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), {
      ok: false,
      error: { code: "something_new", message: "internal: boom at engine.go:42" },
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(adjustErrorText.fallback);
    expect(screen.queryByText(/engine\.go/)).toBeNull();
  });

  test("片方の呼び出しが失敗したら、もう片方が成功していても結果を出さずにエラーを出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    await respond(lastCallOf(client, "allocation"), {
      ok: false,
      error: { code: "invalid_input", message: "ceiling below lower bound" },
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(adjustErrorText.invalid_input);
    expect(screen.queryByRole("region", { name: T.indicesHeading })).toBeNull();
  });

  test("エラーのあとも入力は残り、もう一度送れる", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    setNumber(fixedSpInput("hp"), 4);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), {
      ok: false,
      error: { code: "adjust_unavailable", message: "調整のサーバーに接続できません" },
    });

    expect(screen.getByRole("combobox", { name: T.selfSpeciesLabel })).toHaveValue(BIRD.key);
    expect(fixedSpInput("hp")).toHaveValue(4);
    await user.click(submitButton());
    expect(callsOf(client, "indices")).toHaveLength(2);
  });
});

// =====================================================================================
// S7 古い応答・取り消し
// =====================================================================================

describe("S7 古い応答・取り消し", () => {
  test("計算中は結果の領域が aria-busy で「計算中」を出し、ボタンは押せるまま(押し直せる)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    expect(resultRegion()).toHaveAttribute("aria-busy", "true");
    expect(within(resultRegion()).getByText(T.loadingNotice)).toBeInTheDocument();
    expect(submitButton()).toBeEnabled();

    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    expect(resultRegion()).not.toHaveAttribute("aria-busy", "true");
  });

  test("送り直すと前の呼び出しの signal を abort し、遅れて届いた前の応答で上書きしない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    setNumber(fixedSpInput("hp"), 4);
    await user.click(submitButton());
    const first = lastCallOf(client, "indices");
    expect(first.signal).toBeDefined();
    expect(first.signal?.aborted).toBe(false);

    setNumber(fixedSpInput("hp"), 5);
    await user.click(submitButton());
    const second = lastCallOf(client, "indices");
    expect(second).not.toBe(first);
    expect(first.signal?.aborted).toBe(true);

    await respond(second, {
      ok: true,
      value: { ...indicesResult, hpLines: { ...indicesResult.hpLines, hp: 160, sp: 5, current: "16n" } },
    });
    // 前の応答が後から届いても上書きしない(取り消し扱いの応答・成功の応答のどちらでも)。
    await respond(first, { ok: true, value: indicesResult });
    expect(
      within(region(T.indicesHeading)).getByText(T.hpCurrent(160, T.hpLineKindLabel["16n"])),
    ).toBeInTheDocument();
    expect(
      within(region(T.indicesHeading)).queryByText(T.hpCurrent(159, T.hpLineKindLabel["16n-1"])),
    ).toBeNull();
  });

  test("取り消した呼び出しの request_aborted はエラーとして出さない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    const first = lastCallOf(client, "indices");
    await user.click(submitButton());
    await respond(first, {
      ok: false,
      error: { code: REQUEST_ABORTED_CODE, message: "新しい入力で調整を取り消しました" },
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("画面を閉じたら(unmount)計算中の呼び出しを abort する", async () => {
    const { user, client, view } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "bulk");
    await user.click(submitButton());
    const signals = client.calls.map((call) => call.signal);
    expect(signals.length).toBeGreaterThan(0);
    view.unmount();
    for (const signal of signals) {
      expect(signal?.aborted).toBe(true);
    }
  });
});

// =====================================================================================
// S8 技を覚えるポケモン
// =====================================================================================

function learner(index: number): Schemas["SpeciesSummary"] {
  return {
    key: `${String(9100 + index)}-000`,
    dexNo: 9100 + index,
    form: 0,
    nameJa: `テストオボエル${String(index)}`,
    types: ["normal"],
  };
}

describe("S8 技を覚えるポケモン(技の欄から開くパネル)", () => {
  function selfLearnersButton(): HTMLElement {
    return screen.getByRole("button", { name: T.selfLearnersButtonName });
  }

  test("技を選ぶまでボタンは押せない", async () => {
    const { user } = renderScreen();
    expect(selfLearnersButton()).toBeDisabled();
    await fillSelf(user, { move: MOVE_FIRE });
    expect(selfLearnersButton()).toBeEnabled();
  });

  test("押すと技の ID・1ページ目(limit = LEARNERS_PAGE_SIZE・offset 0)で呼び、技名の見出しと一覧を出す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());

    const call = lastCallOf(client, "moveLearners");
    expect(call.args).toEqual([MOVE_FIRE.id, { limit: LEARNERS_PAGE_SIZE, offset: 0 }]);
    expect(call.signal).toBeDefined();
    // 調整の API は呼ばない(パネルは独立)。
    expect(client.calls.map((entry) => entry.method)).toEqual(["moveLearners"]);

    await respond(call, { ok: true, value: [learner(1), learner(2)] });
    const panel = region(T.learnersRegionLabel);
    expect(
      within(panel).getByRole("heading", { name: T.learnersHeading(MOVE_FIRE.nameJa) }),
    ).toBeInTheDocument();
    expect(
      within(panel)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([learner(1).nameJa, learner(2).nameJa]);
    // 1ページに満たなければ続きは無い。
    expect(within(panel).queryByRole("button", { name: T.learnersMore })).toBeNull();
  });

  test("1ページちょうど返ったら「続きを読み込む」で次の offset を呼び、一覧に足す", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());
    const firstPage = Array.from({ length: LEARNERS_PAGE_SIZE }, (_, i) => learner(i));
    await respond(lastCallOf(client, "moveLearners"), { ok: true, value: firstPage });

    const panel = region(T.learnersRegionLabel);
    await user.click(within(panel).getByRole("button", { name: T.learnersMore }));
    expect(lastCallOf(client, "moveLearners").args).toEqual([
      MOVE_FIRE.id,
      { limit: LEARNERS_PAGE_SIZE, offset: LEARNERS_PAGE_SIZE },
    ]);
    await respond(lastCallOf(client, "moveLearners"), { ok: true, value: [learner(LEARNERS_PAGE_SIZE)] });

    expect(within(panel).getAllByRole("listitem")).toHaveLength(LEARNERS_PAGE_SIZE + 1);
    expect(within(panel).queryByRole("button", { name: T.learnersMore })).toBeNull();
  });

  test("続きの読み込みに失敗しても一覧は残り、エラーを出して、もう一度押せる", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());
    const firstPage = Array.from({ length: LEARNERS_PAGE_SIZE }, (_, i) => learner(i));
    await respond(lastCallOf(client, "moveLearners"), { ok: true, value: firstPage });

    const panel = region(T.learnersRegionLabel);
    await user.click(within(panel).getByRole("button", { name: T.learnersMore }));
    await respond(lastCallOf(client, "moveLearners"), {
      ok: false,
      error: { code: "adjust_unavailable", message: "x" },
    });
    expect(within(panel).getByRole("alert")).toHaveTextContent(adjustErrorText.adjust_unavailable);
    expect(within(panel).getAllByRole("listitem")).toHaveLength(LEARNERS_PAGE_SIZE);

    await user.click(within(panel).getByRole("button", { name: T.learnersMore }));
    expect(lastCallOf(client, "moveLearners").args).toEqual([
      MOVE_FIRE.id,
      { limit: LEARNERS_PAGE_SIZE, offset: LEARNERS_PAGE_SIZE },
    ]);
    await respond(lastCallOf(client, "moveLearners"), { ok: true, value: [learner(LEARNERS_PAGE_SIZE)] });
    expect(within(panel).getAllByRole("listitem")).toHaveLength(LEARNERS_PAGE_SIZE + 1);
    expect(within(panel).queryByRole("alert")).toBeNull();
  });

  test("一致なしは「この技を覚えるポケモンはいません」", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());
    await respond(lastCallOf(client, "moveLearners"), { ok: true, value: [] });
    expect(within(region(T.learnersRegionLabel)).getByText(T.learnersEmpty)).toBeInTheDocument();
  });

  test("エラーは code から日本語(サーバーの message は出さない)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());
    await respond(lastCallOf(client, "moveLearners"), {
      ok: false,
      error: { code: "not_found", message: "move not found: test-move-fire" },
    });
    const panel = region(T.learnersRegionLabel);
    expect(within(panel).getByRole("alert")).toHaveTextContent(adjustErrorText.not_found);
    expect(screen.queryByText(/move not found/)).toBeNull();
  });

  test("別の技で開き直すと前の呼び出しを abort し、新しい技の一覧に置き換える", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(selfLearnersButton());
    const first = lastCallOf(client, "moveLearners");

    await user.selectOptions(screen.getByRole("combobox", { name: T.selfMoveLabel }), MOVE_WATER.id);
    await user.click(selfLearnersButton());
    const second = lastCallOf(client, "moveLearners");
    expect(second.args[0]).toBe(MOVE_WATER.id);
    expect(first.signal?.aborted).toBe(true);

    await respond(second, { ok: true, value: [learner(7)] });
    await respond(first, { ok: true, value: [learner(1)] });
    const panel = region(T.learnersRegionLabel);
    expect(
      within(panel).getByRole("heading", { name: T.learnersHeading(MOVE_WATER.nameJa) }),
    ).toBeInTheDocument();
    expect(
      within(panel)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([learner(7).nameJa]);
  });

  test("相手の技からも開ける(耐えられる最小のモード)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await chooseMode(user, "minSurvive");
    await fillOpponent(user, { move: MOVE_TACKLE });
    await user.click(screen.getByRole("button", { name: T.opponentLearnersButtonName }));
    expect(lastCallOf(client, "moveLearners").args[0]).toBe(MOVE_TACKLE.id);
  });

  test("「調整する」はパネルの API を呼ばない", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user, { move: MOVE_FIRE });
    await user.click(submitButton());
    expect(callsOf(client, "moveLearners")).toHaveLength(0);
  });
});

// =====================================================================================
// S9 a11y
// =====================================================================================

describe("S9 a11y(docs/design.md「入力のラベル」・WCAG 2.2 SC 2.5.3 / 3.3.2)", () => {
  /** 範囲の中の select・数値入力・文字入力に、見えるラベルがあり accessible name に含まれること。 */
  function expectVisibleLabelsInNames(container: HTMLElement): void {
    const controls = [
      ...within(container).queryAllByRole("combobox"),
      ...within(container).queryAllByRole("textbox"),
      ...within(container).queryAllByRole("spinbutton"),
    ];
    expect(controls.length).toBeGreaterThan(0);
    for (const control of controls) {
      const name = accessibleNameOf(control);
      const visible = visibleLabelOf(control);
      expect(visible, `accessible name「${name}」の欄に、結び付いた見えるラベルが無い`).not.toBeNull();
      expect(visible).not.toBe("");
      expect(name).toContain(visible ?? "");
    }
  }

  test("各領域に、領域の名前と同じ見える見出し(h2)がある", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "minSurvive");
    for (const name of [
      T.selfRegionLabel,
      T.modeRegionLabel,
      T.opponentRegionLabel,
      T.goalRegionLabel,
      T.resultRegionLabel,
    ]) {
      expect(within(region(name)).getByRole("heading", { name, level: 2 })).toBeVisible();
    }
  });

  test("欄の accessible name は「<領域の見出し>の<ラベル>」で、見えるラベルは短い語", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "minSurvive");
    expect(T.selfSpeciesLabel).toBe(`${T.selfRegionLabel}の${T.speciesFieldLabel}`);
    expect(T.selfMoveLabel).toBe(`${T.selfRegionLabel}の${T.moveFieldLabel}`);
    expect(T.opponentSpeciesLabel).toBe(`${T.opponentRegionLabel}の${T.speciesFieldLabel}`);
    expect(T.opponentMoveLabel).toBe(`${T.opponentRegionLabel}の${T.moveFieldLabel}`);
    expect(T.opponentPresetLabel).toBe(`${T.opponentRegionLabel}の${T.presetFieldLabel}`);
    expect(visibleLabelOf(screen.getByRole("combobox", { name: T.selfSpeciesLabel }))).toBe(
      T.speciesFieldLabel,
    );
    expect(visibleLabelOf(screen.getByRole("combobox", { name: T.opponentMoveLabel }))).toBe(
      T.moveFieldLabel,
    );
  });

  test.each(["indices", "bulk", "offense", "minKo", "minSurvive"] as const)(
    "%s: すべての欄に見えるラベルがあり、accessible name に含まれる",
    async (mode) => {
      const { user } = renderScreen();
      await chooseMode(user, mode);
      if (mode === "bulk" || mode === "offense") {
        await user.click(screen.getByRole("checkbox", { name: T.useGoalLabel }));
      }
      expectVisibleLabelsInNames(document.body);
    },
  );

  test("未選択の select は空の表示にせず、何を選ぶか分かる文言を出す", async () => {
    const { user } = renderScreen();
    await chooseMode(user, "minSurvive");
    for (const [name, placeholder] of [
      [T.selfSpeciesLabel, T.speciesPlaceholder],
      [T.selfNatureLabel, T.naturePlaceholder],
      [T.selfMoveLabel, T.movePlaceholder],
      [T.opponentSpeciesLabel, T.speciesPlaceholder],
      [T.opponentMoveLabel, T.movePlaceholder],
    ] as const) {
      const select = screen.getByRole("combobox", { name });
      expect(select).toHaveValue("");
      expect((select.querySelector("option[value='']")?.textContent ?? "").trim()).toBe(placeholder);
    }
  });

  // 段階 B(ADR-0177 §10)で「目標から振り方を決める」を既定で出す(ADR-0331 §結果の仕様の変更。5 → 6 つ)。
  test("モードは「調整の内容」の radio group で、6つの選択肢を持つ(先頭は目標から振り方を決める)", () => {
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
  });

  test("説明の要る数値欄は aria-describedby で説明の文と結び付ける", async () => {
    const { user } = renderScreen();
    expect(describedByTextsOf(fixedSpInput("hp"))).toContain(T.fixedSpHint);
    await chooseMode(user, "offense");
    expect(describedByTextsOf(screen.getByRole("spinbutton", { name: T.minSpeedLabel }))).toContain(
      T.minSpeedHint,
    );
  });

  test("「覚えるポケモン」ボタンは、見える文字を accessible name に含む(どちらの技かは名前で区別)", () => {
    renderScreen();
    const button = screen.getByRole("button", { name: T.selfLearnersButtonName });
    expect(button).toHaveTextContent(T.learnersButtonLabel);
    expect(T.selfLearnersButtonName).toContain(T.learnersButtonLabel);
    expect(T.opponentLearnersButtonName).toContain(T.learnersButtonLabel);
  });

  test("送信してもフォーカスは「調整する」に残る(結果へ勝手に移さない)", async () => {
    const { user, client } = renderScreen();
    await fillSelf(user);
    await user.click(submitButton());
    await respond(lastCallOf(client, "indices"), { ok: true, value: indicesResult });
    expect(submitButton()).toHaveFocus();
  });
});

// =====================================================================================
// S10 マスタの使い方
// =====================================================================================

describe("S10 マスタは入力補助にだけ使う", () => {
  test("技の選択肢は選んだ種族の learnset から引き、変化技は出さない", async () => {
    const { user } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    const moves = screen.getByRole("combobox", { name: T.selfMoveLabel });
    const values = within(moves)
      .getAllByRole("option")
      .map((option) => (option as HTMLOptionElement).value)
      .filter((value) => value !== "");
    expect(values).toEqual([MOVE_FIRE.id, MOVE_WATER.id]);
  });

  test("性格・特性・持ち物はマスタの一覧から選ぶ(ハードコードしない)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfSpeciesLabel }), BIRD.key);
    expect(
      within(screen.getByRole("combobox", { name: T.selfNatureLabel })).getByRole("option", {
        name: NATURE_PLUS_SPA.nameJa,
      }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("combobox", { name: T.selfAbilityLabel })).getByRole("option", {
        name: ABILITY.nameJa,
      }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("combobox", { name: T.selfItemLabel })).getByRole("option", {
        name: ITEM.nameJa,
      }),
    ).toBeInTheDocument();
  });

  test("speciesList が false なら検索欄で種族を選び、技は解決した learnset から選べる", async () => {
    const search = createFakeSpeciesSearch({
      species: [BIRD, FISH],
      abilities: [ABILITY],
      moves: [MOVE_FIRE, MOVE_WATER, MOVE_TACKLE, MOVE_STATUS],
    });
    const { user, client } = renderScreen({
      master: {
        ...master,
        species: [],
        moves: [],
        abilities: [],
        capabilities: { speciesList: false, moves: false, effects: false },
      },
      masterSearch: search,
    });

    const input = screen.getByRole("combobox", { name: T.selfSpeciesLabel });
    await user.type(input, BIRD.nameJa);
    await user.click(await within(selfRegion()).findByRole("option", { name: BIRD.nameJa }));
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfNatureLabel }), NATURE_NEUTRAL.id);
    await user.selectOptions(screen.getByRole("combobox", { name: T.selfMoveLabel }), MOVE_FIRE.id);
    await user.click(submitButton());

    expect(lastRequestOf(client, "indices")).toEqual({
      individual: selfIndividual(),
      moveId: MOVE_FIRE.id,
      modifier: STAB_MODIFIER,
    });
  });
});
