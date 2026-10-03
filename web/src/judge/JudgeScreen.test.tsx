// JD5: 判定の画面(ADR-0705 §4〜§8。受け入れ条件 2〜9)。
// JudgeClient は fake(props で注入)。engine(WASM)は使わず、master / masterSearch は入力補助にだけ使う。
// 確かめること(受け入れ条件):
//   A1 初期表示: 3つの領域・相手候補1件・マウント時は judge を呼ばない
//   A2 相手候補の増減: 6件まで追加でき7件目は追加できない / 1件のときは削除できない / 候補の入力は独立
//   A3 request の組み立て: 「判定する」で1回だけ呼び、省略可の欄は送らない(ADR-0705 §6)
//   A4 場の効果: 3つのチェックボックスが speedField に1対1で対応し、相手側の追い風は候補ごとに持たない
//   A5 結果: matchups を defenders と同じ順で出し、行と候補が対応する(取り違えを検出する fake で確かめる)
//   A6 エラー: コードごとの見出しとサーバーの message を出し、入力は消えない
//   A7 送信前の検査: SP・ランク・必須の欄が不正なら judge を呼ばない
//   A8 古い応答: 先に送った request の応答が後から届いても上書きしない
//   A9 マスタ: speciesList が false なら検索欄(技の引き方は A11)
//   --- issue #309(技・調整・数値欄の UI。ADR-0711)で足した・置き換えたもの ---
//   A10 技の選択: ポケモンを選ぶと覚える技(learnset)の select が出る。技の ID の自由入力は無い
//   A11 オンライン(masterSearch): 種族を検索で選ぶと resolveSpecies が返す技から選ぶ
//   A12 調整プリセット: 無振り・最速・A/C特化・(候補は)HB/HD特化で SP と性格が入る
//   A13 詳細: 数値の直接入力(SP6欄・ランク5欄)は「詳細」を開いたときだけ出す
//   A14 検証エラー: 該当欄に aria-invalid と文言(どの体かを添える)
// 架空データだけを使う(実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import { attackerPresetLabel } from "../domain/attackerPresets";
import { defenderPresetLabel } from "../domain/defenderPresets";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import type { Ability, Item, Move, StatKey, TypeChart } from "../engine/types";
import { judgeErrorText, judgeScreenText } from "../i18n/ja";
import { ONLINE_MASTER_CAPABILITIES, SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { createFakeSpeciesSearch } from "../test/onlineMaster";
import { JudgeScreen, MAX_DEFENDERS } from "./JudgeScreen";
import type { components } from "./judge.gen";
import type { JudgeClient, JudgeResult } from "./judgeClient";

type Schemas = components["schemas"];

// ---- fake の JudgeClient(呼び出しを記録し、テストが応答を返す。SpeedScreen.test.tsx と同じ形) ----

interface PendingCall<A, T> {
  readonly args: A;
  resolve(result: JudgeResult<T>): void;
}

interface FakeJudgeClient extends JudgeClient {
  readonly calls: PendingCall<Schemas["OutspeedAndKoRequest"], Schemas["OutspeedAndKoResponse"]>[];
}

function createFakeJudgeClient(): FakeJudgeClient {
  const calls: FakeJudgeClient["calls"] = [];
  return {
    calls,
    outspeedAndKo(request) {
      return new Promise((resolve) => {
        calls.push({ args: structuredClone(request), resolve });
      });
    },
  };
}

function lastCall(
  client: FakeJudgeClient,
): PendingCall<Schemas["OutspeedAndKoRequest"], Schemas["OutspeedAndKoResponse"]> {
  const call = client.calls.at(-1);
  if (call === undefined) {
    throw new Error("outspeedAndKo が呼ばれていない");
  }
  return call;
}

/** 1回だけ解決を流す(SpeedScreen.test.tsx と同じ形)。 */
async function flush(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

// ---- 架空のマスタ(9xxx-xxx の種族・test-* の ID。実マスタは使わない) ----

const emptyTypeChart: TypeChart = { types: ["fire", "water", "grass"], effectiveness: {} };

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
    baseStats: { hp: 70, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: ["test-ability-a"],
    learnset,
  };
}

function fakeMove(id: string, nameJa: string, category: Move["category"], power: number): Move {
  return { id, nameJa, type: "normal", category, power, priority: 0 };
}

// 技は架空。BIRD の learnset は「変化技が先頭」(既定の技は最初のダメージ技 = 計算画面の firstDamagingMove と同じ)。
const MOVE_STATUS = fakeMove("test-move-status", "テストへんかわざ", "status", 0);
const MOVE_PHYS = fakeMove("test-move-phys", "テストぶつりわざ", "physical", 80);
const MOVE_SPEC = fakeMove("test-move-spec", "テストとくしゅわざ", "special", 90);
const FAKE_MOVES: readonly Move[] = [MOVE_STATUS, MOVE_PHYS, MOVE_SPEC];

const BIRD = fakeSpecies(
  "9001-000",
  "テストカソウドリ",
  ["fire", "flying"],
  [MOVE_STATUS.id, MOVE_PHYS.id, MOVE_SPEC.id],
);
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"], [MOVE_SPEC.id]);
const GRASS = fakeSpecies("9003-000", "テストカソウソウ", ["grass"], [MOVE_PHYS.id]);
/** learnset が空の種族(技を1件も引けないときの扱いを確かめる)。 */
const NOMOVE = fakeSpecies("9004-000", "テストカソウナシ", ["grass"], []);

const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストせっかち",
  plus: "spe",
  minus: "def",
};
const NATURE_NEUTRAL: MasterNature = {
  id: "test-nature-neutral",
  nameJa: "テストまじめ",
  plus: null,
  minus: null,
};

// プリセットが指す性格(plus・minus の組)。master.natures から引く(ハードコードしない)。
const NATURE_JOLLY: MasterNature = {
  id: "test-nature-jolly",
  nameJa: "テストようき",
  plus: "spe",
  minus: "spa",
};
const NATURE_TIMID: MasterNature = {
  id: "test-nature-timid",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};
const NATURE_ADAMANT: MasterNature = {
  id: "test-nature-adamant",
  nameJa: "テストいじっぱり",
  plus: "atk",
  minus: "spa",
};
const NATURE_MODEST: MasterNature = {
  id: "test-nature-modest",
  nameJa: "テストひかえめ",
  plus: "spa",
  minus: "atk",
};
const NATURE_BOLD: MasterNature = {
  id: "test-nature-bold",
  nameJa: "テストずぶとい",
  plus: "def",
  minus: "atk",
};
const NATURE_CALM: MasterNature = {
  id: "test-nature-calm",
  nameJa: "テストおだやか",
  plus: "spd",
  minus: "atk",
};

const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };
const ITEM: Item = { id: "test-item-a", nameJa: "テストもちもの", effect: null };

/**
 * 画面が使うマスタ(オフライン相当。技の実体を master.moves から引ける)。
 * issue #309: 技はポケモンの learnset から選ぶので、master.moves に技の実体を持たせる。
 */
const master: MasterData = {
  species: [BIRD, FISH, GRASS, NOMOVE],
  moves: FAKE_MOVES,
  items: [ITEM],
  abilities: [ABILITY],
  natures: [
    NATURE_PLUS_SPE,
    NATURE_NEUTRAL,
    NATURE_JOLLY,
    NATURE_TIMID,
    NATURE_ADAMANT,
    NATURE_MODEST,
    NATURE_BOLD,
    NATURE_CALM,
  ],
  typeChart: emptyTypeChart,
  capabilities: { speciesList: true, moves: true, effects: false },
};

/** 技の実体をどこからも引けないマスタ(learnset の ID を技に解決できない)。 */
const masterNoMoves: MasterData = {
  ...master,
  moves: [],
  capabilities: { speciesList: true, moves: false, effects: false },
};

// ---- 描画と、よく使う問い合わせ ----

function renderScreen(masterData: MasterData = master): { user: UserEvent; client: FakeJudgeClient } {
  const user = userEvent.setup();
  const client = createFakeJudgeClient();
  render(<JudgeScreen judgeClient={client} master={masterData} />);
  return { user, client };
}

/** 自分のポケモンの領域。 */
function attackerRegion(): HTMLElement {
  return screen.getByRole("region", { name: judgeScreenText.attackerRegionLabel });
}

/** 相手の候補の領域。 */
function defendersRegion(): HTMLElement {
  return screen.getByRole("region", { name: judgeScreenText.defendersRegionLabel });
}

/** 判定結果の領域。 */
function resultRegion(): HTMLElement {
  return screen.getByRole("region", { name: judgeScreenText.resultRegionLabel });
}

/** n 件目(1始まり)の相手候補の入力のまとまり。 */
function candidate(n: number): HTMLElement {
  return within(defendersRegion()).getByRole("group", { name: judgeScreenText.candidateGroupLabel(n) });
}

/** 今ある相手候補のまとまりの数。 */
function candidateCount(): number {
  return within(defendersRegion()).getAllByRole("group").length;
}

/** 結果の行(defenders と同じ順。data-defender-index で対応が読める)。 */
function matchupRows(): HTMLElement[] {
  return within(resultRegion()).getAllByTestId("judge-matchup");
}

function submitButton(): HTMLElement {
  return screen.getByRole("button", { name: judgeScreenText.submitLabel });
}

function addCandidateButton(): HTMLElement {
  return screen.getByRole("button", { name: judgeScreenText.addCandidateLabel });
}

/** 数値入力を入れ直す(既定値を消してから入れる)。 */
async function setNumber(user: UserEvent, input: HTMLElement, value: number): Promise<void> {
  await user.clear(input);
  await user.type(input, String(value));
}

/** 個体(自分 / 候補)の必須の欄を埋める。 */
async function fillIndividual(
  user: UserEvent,
  region: HTMLElement,
  species: MasterSpecies,
  nature: MasterNature,
): Promise<void> {
  await user.selectOptions(within(region).getByLabelText(judgeScreenText.speciesLabel), species.key);
  await user.selectOptions(within(region).getByLabelText(judgeScreenText.natureLabel), nature.id);
}

/** 技の select(自分 / 候補の領域の中)。issue #309: 技の ID の自由入力は無い。 */
function moveSelect(region: HTMLElement): HTMLElement {
  return within(region).getByRole("combobox", { name: judgeScreenText.moveLabel });
}

/** 技の select の選択肢の value(learnset の順)。 */
function moveOptionValues(region: HTMLElement): string[] {
  return within(moveSelect(region))
    .queryAllByRole("option")
    .map((option) => option.getAttribute("value") ?? "");
}

/** 「詳細」を開く(数値の直接入力は閉じている間は見えない。issue #309)。 */
async function openDetails(user: UserEvent, region: HTMLElement): Promise<void> {
  await user.click(within(region).getByText(judgeScreenText.detailsSummaryLabel));
}

/** 調整プリセットのラジオ(領域の中の radiogroup「調整」から名前で引く)。 */
function presetRadio(region: HTMLElement, label: string): HTMLElement {
  const group = within(region).getByRole("radiogroup", { name: judgeScreenText.presetGroupLabel });
  return within(group).getByRole("radio", { name: label });
}

/**
 * 自分 + 候補1件の、判定を通せる最小の入力。
 * 技は種族を選ぶと最初のダメージ技が入る(A10)ので選ばない: 自分 BIRD → MOVE_PHYS、候補 FISH → MOVE_SPEC。
 */
async function fillMinimalForm(user: UserEvent): Promise<void> {
  await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
  await fillIndividual(user, candidate(1), FISH, NATURE_NEUTRAL);
}

// ---- 架空の応答(候補ごとに**すべて違う値**にして、行の取り違えを検出する) ----

function ko(hits: number, guaranteed: boolean, displayChancePercent: number): Schemas["KOChance"] {
  return { hits, guaranteed, displayChancePercent };
}

function matchup(index: number, overrides: Partial<Schemas["Matchup"]> = {}): Schemas["Matchup"] {
  return {
    defenderIndex: index,
    outspeeds: true,
    speedTie: false,
    attackerSpeed: 180,
    defenderSpeed: 100 + index,
    attackerMovePriority: 0,
    defenderMovePriority: index,
    attackerMovesFirst: true,
    turnOrderTie: false,
    attackerKo: ko(1 + index, true, 100),
    defenderKo: ko(3 + index, false, 10 + index),
    attackerKoUnsupported: [],
    defenderKoUnsupported: [],
    ...overrides,
  };
}

// ---- A1: 初期表示 ----

describe("A1 初期表示", () => {
  test("自分のポケモン・相手の候補・判定結果の3つの領域が出る", () => {
    renderScreen();
    expect(attackerRegion()).toBeInTheDocument();
    expect(defendersRegion()).toBeInTheDocument();
    expect(resultRegion()).toBeInTheDocument();
  });

  test("相手候補は1件から始まる", () => {
    renderScreen();
    expect(candidateCount()).toBe(1);
    expect(candidate(1)).toBeInTheDocument();
  });

  test("マウントしただけでは judge を呼ばない(呼ぶのは「判定する」のときだけ。ADR-0705 §7)", () => {
    const { client } = renderScreen();
    expect(client.calls).toHaveLength(0);
  });

  // 置き換え(issue #309): 旧「技は ID の自由入力で、一覧から選べない理由の案内が出る」は、技が select になるので
  // 「技の ID を打つ欄が無い」へ(A10 の同名テスト)。
  test("技は select で、技の ID を打つ欄も「ID で入力します」の案内も無い(issue #309)", () => {
    renderScreen();
    for (const region of [attackerRegion(), candidate(1)]) {
      expect(moveSelect(region).tagName).toBe("SELECT");
      expect(within(region).queryByRole("textbox", { name: /技/ })).toBeNull();
    }
    expect(screen.queryByText(/ID から技を引く API/)).toBeNull();
  });
});

// ---- A2: 相手候補の増減(受け入れ条件2) ----

describe("A2 相手候補の増減", () => {
  test(`追加で ${String(MAX_DEFENDERS)} 件まで増え、それ以上は追加できない`, async () => {
    const { user } = renderScreen();
    for (let n = 1; n < MAX_DEFENDERS; n += 1) {
      await user.click(addCandidateButton());
    }
    expect(candidateCount()).toBe(MAX_DEFENDERS);
    expect(addCandidateButton()).toBeDisabled();
    expect(screen.getByText(judgeScreenText.maxCandidatesNotice(MAX_DEFENDERS))).toBeInTheDocument();

    // 上限に達した後にもう一度押しても増えない(disabled の二重のガード)。
    await user.click(addCandidateButton());
    expect(candidateCount()).toBe(MAX_DEFENDERS);
  });

  test("削除で減る。1件のときは削除できない(契約上 defenders は1件以上)", async () => {
    const { user } = renderScreen();
    expect(screen.getByRole("button", { name: judgeScreenText.removeCandidateLabel(1) })).toBeDisabled();

    await user.click(addCandidateButton());
    expect(candidateCount()).toBe(2);
    await user.click(screen.getByRole("button", { name: judgeScreenText.removeCandidateLabel(2) }));
    expect(candidateCount()).toBe(1);
    expect(screen.getByRole("button", { name: judgeScreenText.removeCandidateLabel(1) })).toBeDisabled();
  });

  // 置き換え(issue #309): 技 ID の自由入力が無くなったので、独立性は「性格」と「技の select」で確かめる。
  test("候補の入力は独立している(ある候補の性格・技が他の候補に混ざらない)", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(1), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);
    await user.selectOptions(moveSelect(candidate(1)), MOVE_SPEC.id);

    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_PLUS_SPE.id);
    expect(within(candidate(2)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_NEUTRAL.id);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_SPEC.id);
    expect(moveSelect(candidate(2))).toHaveValue(MOVE_PHYS.id);
  });

  test("削除しても残った候補の入力が保たれる(index のずれで値が動かない)", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(1), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);

    await user.click(screen.getByRole("button", { name: judgeScreenText.removeCandidateLabel(1) }));

    expect(candidateCount()).toBe(1);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.speciesLabel)).toHaveValue(GRASS.key);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_NEUTRAL.id);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_PHYS.id);
  });
});

// ---- A3: request の組み立て(受け入れ条件3) ----

describe("A3 request の組み立て", () => {
  test("「判定する」で1回だけ呼び、省略可の欄(ranks・abilityId・itemId・field・speedField)を送らない", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);

    await user.click(submitButton());

    expect(client.calls).toHaveLength(1);
    expect(lastCall(client).args).toEqual({
      format: "single",
      attacker: {
        speciesKey: BIRD.key,
        natureId: NATURE_PLUS_SPE.id,
        sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      },
      defenders: [
        {
          speciesKey: FISH.key,
          natureId: NATURE_NEUTRAL.id,
          sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
          moveId: MOVE_SPEC.id,
        },
      ],
      moveId: MOVE_PHYS.id,
    } satisfies Schemas["OutspeedAndKoRequest"]);
  });

  test("SP は6項目そのまま送る(合計の上限内)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await openDetails(user, candidate(1));
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe")), 32);
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("atk")), 30);
    await setNumber(user, within(candidate(1)).getByLabelText(judgeScreenText.spLabel("hp")), 4);

    await user.click(submitButton());

    const request = lastCall(client).args;
    expect(request.attacker.sp).toEqual({ hp: 0, atk: 30, def: 0, spa: 0, spd: 0, spe: 32 });
    expect(request.defenders[0]?.sp).toEqual({ hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 });
  });

  test("ランクを1つでも動かすと、ranks を5項目そろえて送る", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await openDetails(user, candidate(1));
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.rankLabel("spe")), 1);
    await setNumber(user, within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("def")), -2);

    await user.click(submitButton());

    const request = lastCall(client).args;
    expect(request.attacker.ranks).toEqual({ atk: 0, def: 0, spa: 0, spd: 0, spe: 1 });
    expect(request.defenders[0]?.ranks).toEqual({ atk: 0, def: -2, spa: 0, spd: 0, spe: 0 });
  });

  test("特性・持ち物は選んだときだけ送る(未選択の欄は送らない)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.selectOptions(
      within(attackerRegion()).getByLabelText(judgeScreenText.abilityLabel),
      ABILITY.id,
    );
    await user.selectOptions(within(attackerRegion()).getByLabelText(judgeScreenText.itemLabel), ITEM.id);

    await user.click(submitButton());

    const request = lastCall(client).args;
    expect(request.attacker.abilityId).toBe(ABILITY.id);
    expect(request.attacker.itemId).toBe(ITEM.id);
    // 候補側は未選択のまま(null も送らない。ADR-0705 §6)。
    expect(Object.keys(request.defenders[0] ?? {}).sort()).toEqual([
      "moveId",
      "natureId",
      "sp",
      "speciesKey",
    ]);
  });

  // 置き換え(issue #309): 旧「技 ID は前後の空白を落として送る」。技が select になり空白が入り得ないので、
  // 「選んだ技の ID(マスタの id そのまま)を、自分は request 直下・候補は候補の欄に送る」へ。
  test("選んだ技の ID(マスタの id)をそのまま送る(自分は直下、候補は候補の欄)", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(1), BIRD, NATURE_NEUTRAL);
    await user.selectOptions(moveSelect(attackerRegion()), MOVE_SPEC.id);
    await user.selectOptions(moveSelect(candidate(1)), MOVE_STATUS.id);

    await user.click(submitButton());

    expect(lastCall(client).args.moveId).toBe(MOVE_SPEC.id);
    expect(lastCall(client).args.defenders[0]?.moveId).toBe(MOVE_STATUS.id);
  });

  test("ダブルは未対応なので対戦形式の選択肢に出さず、format は single で送る(issue #288)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    const select = screen.getByLabelText(judgeScreenText.formatLabel);

    expect(within(select).queryByRole("option", { name: judgeScreenText.formatOption.double })).toBeNull();

    await user.click(submitButton());

    expect(lastCall(client).args.format).toBe("single");
  });

  test("候補を増やすと defenders が入力した順に並ぶ", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);

    await user.click(submitButton());

    expect(lastCall(client).args.defenders.map((entry) => entry.speciesKey)).toEqual([FISH.key, GRASS.key]);
    expect(lastCall(client).args.defenders.map((entry) => entry.moveId)).toEqual([
      MOVE_SPEC.id,
      MOVE_PHYS.id,
    ]);
  });

  test("判定中は「判定中」を出し、二重に送らない", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);

    await user.click(submitButton());

    expect(screen.getByText(judgeScreenText.loadingNotice)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
    expect(client.calls).toHaveLength(1);
  });
});

// ---- A4: 場の効果(受け入れ条件4) ----

describe("A4 場の効果(speedField)", () => {
  test("3つのチェックボックスが speedField の3欄に1対1で対応する", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(screen.getByLabelText(judgeScreenText.trickRoomLabel));
    await user.click(screen.getByLabelText(judgeScreenText.defenderTailwindLabel));

    await user.click(submitButton());

    expect(lastCall(client).args.speedField).toEqual({
      trickRoom: true,
      attackerTailwind: false,
      defenderTailwind: true,
    });
  });

  test("自分の側の追い風だけを立てても、3欄そろえて送る", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(screen.getByLabelText(judgeScreenText.attackerTailwindLabel));

    await user.click(submitButton());

    expect(lastCall(client).args.speedField).toEqual({
      trickRoom: false,
      attackerTailwind: true,
      defenderTailwind: false,
    });
  });

  test("相手の側の追い風は候補ごとではなく1つだけ(ADR-0703 §5・ADR-0705 §6)", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());
    await user.click(addCandidateButton());

    // 候補を3件に増やしても、相手側の追い風のチェックボックスは増えない。
    expect(screen.getAllByLabelText(judgeScreenText.defenderTailwindLabel)).toHaveLength(1);
    // すべての候補に同じように適用されることを文言で示す。
    expect(screen.getByText(judgeScreenText.defenderTailwindNotice)).toBeInTheDocument();
    expect(within(candidate(1)).queryByLabelText(judgeScreenText.defenderTailwindLabel)).toBeNull();
  });

  test("天候・地形・壁(field)は JD5 では送らない(ADR-0705 §6)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);

    await user.click(submitButton());

    expect(lastCall(client).args.field).toBeUndefined();
  });
});

// ---- A5: 結果(受け入れ条件5) ----

describe("A5 結果の表示", () => {
  /** 候補2件を埋めて送り、matchups を返す。 */
  async function submitTwoCandidates(
    matchups: readonly Schemas["Matchup"][],
  ): Promise<{ client: FakeJudgeClient }> {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);
    await user.click(submitButton());
    const call = lastCall(client);
    await flush(() => {
      call.resolve({ ok: true, value: { matchups: [...matchups] } });
    });
    return { client };
  }

  test("行は defenders と同じ順・同じ件数で、候補の種族名と対応する(取り違えの検出)", async () => {
    await submitTwoCandidates([matchup(0), matchup(1)]);

    const rows = matchupRows();
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveAttribute("data-defender-index", "0");
    expect(rows[0]).toHaveTextContent(FISH.nameJa);
    expect(rows[1]).toHaveAttribute("data-defender-index", "1");
    expect(rows[1]).toHaveTextContent(GRASS.nameJa);
    // 候補ごとに違う値(素早さ・優先度・確定数)が、それぞれの行に出る。
    expect(rows[0]).toHaveTextContent(judgeScreenText.speedLabel(180, 100));
    expect(rows[1]).toHaveTextContent(judgeScreenText.speedLabel(180, 101));
    expect(rows[0]).toHaveTextContent(judgeScreenText.koGuaranteed(1));
    expect(rows[1]).toHaveTextContent(judgeScreenText.koGuaranteed(2));
  });

  test("応答が defenderIndex の降順で届いても、行は defenders と同じ順に並べる", async () => {
    await submitTwoCandidates([matchup(1), matchup(0)]);

    const rows = matchupRows();
    expect(rows.map((row) => row.getAttribute("data-defender-index"))).toEqual(["0", "1"]);
    expect(rows[0]).toHaveTextContent(FISH.nameJa);
    expect(rows[1]).toHaveTextContent(GRASS.nameJa);
  });

  test("素早さの比較・優先度・行動順を、judge が返した値のまま出す", async () => {
    await submitTwoCandidates([
      matchup(0, { outspeeds: true, speedTie: false, attackerMovePriority: 0, defenderMovePriority: 1 }),
    ]);

    const row = matchupRows()[0];
    expect(row).toHaveTextContent(judgeScreenText.outspeedsTrueLabel);
    expect(row).toHaveTextContent(judgeScreenText.priorityLabel(0, 1));
    expect(row).toHaveTextContent(judgeScreenText.attackerMovesFirstLabel);
  });

  test("同速は「同速」として出す(outspeeds の false と区別する。ADR-0700 §6-1)", async () => {
    await submitTwoCandidates([
      matchup(0, { outspeeds: false, speedTie: true, attackerSpeed: 150, defenderSpeed: 150 }),
    ]);

    const row = matchupRows()[0];
    expect(row).toHaveTextContent(judgeScreenText.speedTieLabel);
    expect(row).not.toHaveTextContent(judgeScreenText.outspeedsFalseLabel);
  });

  test("素早さで下回るときは、そのまま「下回る」を出す", async () => {
    await submitTwoCandidates([matchup(0, { outspeeds: false, speedTie: false })]);
    expect(matchupRows()[0]).toHaveTextContent(judgeScreenText.outspeedsFalseLabel);
  });

  test("行動順が決まらないとき(turnOrderTie)は、どちらが先かを断定しない(ADR-0704 §2)", async () => {
    await submitTwoCandidates([matchup(0, { attackerMovesFirst: false, turnOrderTie: true })]);

    const row = matchupRows()[0];
    expect(row).toHaveTextContent(judgeScreenText.turnOrderTieLabel);
    expect(row).not.toHaveTextContent(judgeScreenText.attackerMovesFirstLabel);
    expect(row).not.toHaveTextContent(judgeScreenText.defenderMovesFirstLabel);
  });

  test("相手が先に動くときは「相手が先に動く」を出す", async () => {
    await submitTwoCandidates([matchup(0, { attackerMovesFirst: false, turnOrderTie: false })]);
    expect(matchupRows()[0]).toHaveTextContent(judgeScreenText.defenderMovesFirstLabel);
  });

  test("双方向の確定数を、確定 / 乱数 / 倒せない の3通りで出す(丸めない。ADR-0704 §3)", async () => {
    await submitTwoCandidates([
      matchup(0, { attackerKo: ko(2, true, 100), defenderKo: ko(3, false, 42.5) }),
      matchup(1, { attackerKo: ko(0, false, 0), defenderKo: ko(1, true, 100) }),
    ]);

    const rows = matchupRows();
    expect(rows[0]).toHaveTextContent(judgeScreenText.attackerKoLabel);
    expect(rows[0]).toHaveTextContent(judgeScreenText.koGuaranteed(2));
    expect(rows[0]).toHaveTextContent(judgeScreenText.defenderKoLabel);
    expect(rows[0]).toHaveTextContent(judgeScreenText.koRandom(3, 42.5));
    // hits === 0 は「倒せない」(0発と書かない)。
    expect(rows[1]).toHaveTextContent(judgeScreenText.koNone);
    expect(rows[1]).toHaveTextContent(judgeScreenText.koGuaranteed(1));
  });

  test("送信前は結果の代わりに案内を出す", () => {
    renderScreen();
    expect(within(resultRegion()).getByText(judgeScreenText.emptyResultNotice)).toBeInTheDocument();
    expect(within(resultRegion()).queryAllByTestId("judge-matchup")).toHaveLength(0);
  });
});

// ---- A6: エラー(受け入れ条件6) ----

describe("A6 エラーの表示", () => {
  /** 最小の入力で送り、失敗を返す。 */
  async function submitAndFail(error: { code: string; message: string }): Promise<void> {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(submitButton());
    const call = lastCall(client);
    await flush(() => {
      call.resolve({ ok: false, error });
    });
  }

  test("コードごとの見出しと、サーバーの message(どの候補で失敗したか)を出す", async () => {
    await submitAndFail({ code: "unknown_move", message: "defenders[0]: unknown moveId" });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(judgeErrorText.unknown_move);
    // どの候補で失敗したかは message にしか入らない(ADR-0703 §3)ので、落とさずに出す。
    expect(screen.getByText(/defenders\[0\]/)).toBeInTheDocument();
  });

  test.each([
    ["invalid_request", judgeErrorText.invalid_request],
    ["unknown_species", judgeErrorText.unknown_species],
    ["unknown_nature", judgeErrorText.unknown_nature],
    ["request_too_large", judgeErrorText.request_too_large],
    ["upstream_unavailable", judgeErrorText.upstream_unavailable],
    ["internal_error", judgeErrorText.internal_error],
  ] as const)("%s の見出しを出す", async (code, expected) => {
    await submitAndFail({ code, message: `${code} happened` });
    expect(screen.getByRole("alert")).toHaveTextContent(expected);
  });

  test("Web 側の judge_unavailable(通信できない)でも画面が壊れない", async () => {
    await submitAndFail({ code: "judge_unavailable", message: judgeErrorText.judge_unavailable });

    expect(screen.getByRole("alert")).toHaveTextContent(judgeErrorText.judge_unavailable);
    // 同じ文言を二重に出さない(見出しと message が同じときは補助の行を出さない)。
    expect(screen.getAllByText(judgeErrorText.judge_unavailable)).toHaveLength(1);
  });

  test("未知のコードはサーバーの message をそのまま出す", async () => {
    await submitAndFail({ code: "test_unknown_code", message: "何か予期しないことが起きました" });
    expect(screen.getByRole("alert")).toHaveTextContent("何か予期しないことが起きました");
  });

  test("エラーの後も入力は残る(直して送り直せる)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(submitButton());
    const call = lastCall(client);
    await flush(() => {
      call.resolve({ ok: false, error: { code: "unknown_move", message: "attacker: unknown moveId" } });
    });

    expect(moveSelect(attackerRegion())).toHaveValue(MOVE_PHYS.id);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_SPEC.id);
    // 送り直せる(ボタンは戻っている)。
    expect(submitButton()).toBeEnabled();
    await user.click(submitButton());
    expect(client.calls).toHaveLength(2);
  });
});

// ---- A7: 送信前の検査(受け入れ条件7) ----

describe("A7 送信前の検査(judge を呼ばずに理由を出す)", () => {
  test("種族・性格・技 ID が空のままでは呼ばない", async () => {
    const { user, client } = renderScreen();

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
  });

  // 置き換え(issue #309): 技は種族を選ぶと既定が入るので「技だけ空」は learnset が引けない種族で作る。
  test("候補の技だけが空でも呼ばない(候補の moveId は契約上必須)", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(1), NOMOVE, NATURE_NEUTRAL);

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
  });

  test(`SP が1項目でも ${String(MAX_SP_PER_STAT)} を超えたら呼ばない`, async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await setNumber(
      user,
      within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe")),
      MAX_SP_PER_STAT + 1,
    );

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.spRangeMessage(MAX_SP_PER_STAT));
  });

  test(`SP の合計が ${String(MAX_SP_TOTAL)} を超えたら呼ばない(CLAUDE.md ドメイン規約)`, async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, candidate(1));
    const stats: readonly StatKey[] = ["hp", "atk", "def"];
    for (const stat of stats) {
      await setNumber(user, within(candidate(1)).getByLabelText(judgeScreenText.spLabel(stat)), 32);
    }

    await user.click(submitButton());

    // 32 + 32 + 32 = 96 > 66。
    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.spTotalMessage(MAX_SP_TOTAL));
  });

  test("ランクが -6..+6 の外なら呼ばない", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.rankLabel("spe")), 7);

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.rankRangeMessage);
  });
});

// ---- A8: 古い応答(受け入れ条件8) ----

describe("A8 古い応答は無視する", () => {
  test("先に送った request の応答が後から届いても、後で送った方の応答を上書きしない", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);

    const button = submitButton();
    // 送信ボタンは判定中は disabled になる(二重送信を防ぐ主な仕組み)が、React の状態更新が
    // 画面に反映される前に2回叩かれた場合の備え(連番ガード。ADR-0705 §7)を直接確かめるため、
    // ここでは userEvent ではなく fireEvent で disabled が反映される前に2回連続で叩く
    // (同じ act() の中で同期的に呼ぶことで、両方とも disabled になる前に onClick を通す)。
    act(() => {
      fireEvent.click(button);
      fireEvent.click(button);
    });
    expect(client.calls).toHaveLength(2);
    const [first, second] = client.calls;
    if (first === undefined || second === undefined) {
      throw new Error("2回の送信が記録されていない");
    }

    // 後から送った方(second)を先に解決し、続いて先に送った方(first)を解決する。
    // 連番ガードが無ければ、後から届いた first の応答が second の結果を上書きしてしまう。
    await flush(() => {
      second.resolve({ ok: true, value: { matchups: [matchup(0, { attackerKo: ko(2, true, 100) })] } });
    });
    await flush(() => {
      first.resolve({ ok: true, value: { matchups: [matchup(0, { attackerKo: ko(4, true, 100) })] } });
    });

    await waitFor(() => {
      expect(matchupRows()[0]).toHaveTextContent(judgeScreenText.koGuaranteed(2));
    });
    expect(matchupRows()[0]).not.toHaveTextContent(judgeScreenText.koGuaranteed(4));
  });
});

// ---- A9: マスタの使い方(受け入れ条件・ADR-0705 §5) ----

describe("A9 マスタは入力補助にだけ使う", () => {
  test("speciesList が false なら、ドロップダウンの代わりに検索欄を出す(ADR-0304 §1)", () => {
    const search = createFakeSpeciesSearch({ species: [BIRD, FISH, GRASS], abilities: [ABILITY] });
    const client = createFakeJudgeClient();
    render(
      <JudgeScreen
        judgeClient={client}
        master={{
          ...master,
          species: [],
          capabilities: { speciesList: false, moves: false, effects: false },
        }}
        masterSearch={search}
      />,
    );

    expect(
      within(attackerRegion()).getByRole("combobox", { name: judgeScreenText.speciesLabel }),
    ).toBeInTheDocument();
    expect(
      within(candidate(1)).getByRole("combobox", { name: judgeScreenText.speciesLabel }),
    ).toBeInTheDocument();
  });

  test("性格・特性・持ち物はマスタの一覧から選ぶ(ハードコードしない。CLAUDE.md ドメイン規約)", () => {
    renderScreen();
    const natures = within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel);
    expect(within(natures).getByRole("option", { name: NATURE_PLUS_SPE.nameJa })).toBeInTheDocument();
    expect(within(natures).getByRole("option", { name: NATURE_NEUTRAL.nameJa })).toBeInTheDocument();
    expect(
      within(within(attackerRegion()).getByLabelText(judgeScreenText.abilityLabel)).getByRole("option", {
        name: ABILITY.nameJa,
      }),
    ).toBeInTheDocument();
    expect(
      within(within(attackerRegion()).getByLabelText(judgeScreenText.itemLabel)).getByRole("option", {
        name: ITEM.nameJa,
      }),
    ).toBeInTheDocument();
  });
});

// ---- A10: 技の選択(issue #309。ADR-0711。計算画面と同じ「種族の learnset から選ぶ」) ----

describe("A10 技はポケモンの覚える技(learnset)から選ぶ", () => {
  test("種族を選ぶ前の技の select は disabled で、選択肢が無い(自分側・候補とも)", () => {
    renderScreen();
    for (const region of [attackerRegion(), candidate(1)]) {
      expect(moveSelect(region)).toBeDisabled();
      expect(moveOptionValues(region)).toEqual([]);
    }
  });

  test("種族を選ぶと、learnset の技が learnset の順のまま(変化技も含めて)選べる", async () => {
    const { user } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);

    expect(moveSelect(attackerRegion())).toBeEnabled();
    expect(moveOptionValues(attackerRegion())).toEqual([MOVE_STATUS.id, MOVE_PHYS.id, MOVE_SPEC.id]);
    // 選択肢には技の名前が出る(ID を打たせない)。
    const options = within(moveSelect(attackerRegion())).getAllByRole("option");
    expect(options[0]).toHaveTextContent(MOVE_STATUS.nameJa);
    expect(options[1]).toHaveTextContent(MOVE_PHYS.nameJa);
    // 候補側も同じ(自分と同じ部品)。他の種族の技は混ざらない。
    await fillIndividual(user, candidate(1), FISH, NATURE_NEUTRAL);
    expect(moveOptionValues(candidate(1))).toEqual([MOVE_SPEC.id]);
  });

  test("種族を選ぶと最初のダメージ技が既定で選ばれる(変化技が先頭でも。計算画面の firstDamagingMove と同じ)", async () => {
    const { user } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    expect(moveSelect(attackerRegion())).toHaveValue(MOVE_PHYS.id);
  });

  test("種族を変えると、新しい種族の learnset の既定の技へ替わる(前の種族の技が残らない)", async () => {
    const { user } = renderScreen();
    await fillIndividual(user, candidate(1), BIRD, NATURE_NEUTRAL);
    await user.selectOptions(moveSelect(candidate(1)), MOVE_SPEC.id);

    await user.selectOptions(within(candidate(1)).getByLabelText(judgeScreenText.speciesLabel), FISH.key);

    expect(moveOptionValues(candidate(1))).toEqual([MOVE_SPEC.id]);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_SPEC.id);
    await user.selectOptions(within(candidate(1)).getByLabelText(judgeScreenText.speciesLabel), GRASS.key);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_PHYS.id);
  });

  test("技は自分と候補で別々に選べる", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(1), BIRD, NATURE_NEUTRAL);
    await user.selectOptions(moveSelect(attackerRegion()), MOVE_SPEC.id);

    expect(moveSelect(attackerRegion())).toHaveValue(MOVE_SPEC.id);
    expect(moveSelect(candidate(1))).toHaveValue(MOVE_PHYS.id);

    await user.click(submitButton());
    expect(lastCall(client).args.moveId).toBe(MOVE_SPEC.id);
    expect(lastCall(client).args.defenders[0]?.moveId).toBe(MOVE_PHYS.id);
  });

  test("learnset が空の種族は、技の select が disabled のままで案内が出る。判定は送らない", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(1), NOMOVE, NATURE_NEUTRAL);

    expect(within(candidate(1)).queryByText(judgeScreenText.moveUnavailableNotice)).toBeInTheDocument();
    expect(moveSelect(candidate(1))).toBeDisabled();
    expect(within(attackerRegion()).queryByText(judgeScreenText.moveUnavailableNotice)).toBeNull();

    await user.click(submitButton());
    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
  });

  // 置き換え(issue #309): 旧 A9「master.moves が空でも ID を打てば判定を送れる」。ID の自由入力をやめたので、
  // 「技の実体を引けないマスタでは、技の select は disabled・案内を出し、ID を打つ逃げ道を作らない」へ。
  test("技の実体を引けないマスタ(master.moves が空)では、select は disabled・案内を出し、ID の入力欄に逃げない", async () => {
    const { user, client } = renderScreen(masterNoMoves);
    await fillMinimalForm(user);

    expect(moveSelect(attackerRegion())).toBeDisabled();
    expect(moveOptionValues(attackerRegion())).toEqual([]);
    expect(within(attackerRegion()).getByText(judgeScreenText.moveUnavailableNotice)).toBeInTheDocument();
    expect(within(attackerRegion()).queryByRole("textbox", { name: /技/ })).toBeNull();

    await user.click(submitButton());
    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
  });
});

// ---- A11: オンライン(masterSearch)。種族が master.species に無いときの技の引き方(ADR-0711) ----

describe("A11 種族を検索で選ぶ(speciesList が false)ときは resolveSpecies が返す技から選ぶ", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  /** オンライン相当のマスタ(種族・技・特性は全件を持たない)。 */
  const onlineMaster: MasterData = {
    ...master,
    species: [],
    moves: [],
    abilities: [],
    capabilities: ONLINE_MASTER_CAPABILITIES,
  };

  function renderOnline(
    search = createFakeSpeciesSearch({
      species: [BIRD, FISH, NOMOVE],
      abilities: [ABILITY],
      moves: FAKE_MOVES,
    }),
  ) {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const client = createFakeJudgeClient();
    render(<JudgeScreen judgeClient={client} master={onlineMaster} masterSearch={search} />);
    return { user, client, search };
  }

  async function chooseBySearch(user: UserEvent, region: HTMLElement, species: MasterSpecies): Promise<void> {
    await user.type(
      within(region).getByRole("combobox", { name: judgeScreenText.speciesLabel }),
      species.nameJa,
    );
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await within(region).findByRole("option", { name: species.nameJa }));
  }

  test("検索で種族を選ぶと、解決した learnset の技が選べて、最初のダメージ技が既定になる", async () => {
    const { user, search } = renderOnline();
    await chooseBySearch(user, attackerRegion(), BIRD);

    await waitFor(() => {
      expect(moveOptionValues(attackerRegion())).toEqual([MOVE_STATUS.id, MOVE_PHYS.id, MOVE_SPEC.id]);
    });
    expect(search.resolvedKeys).toEqual([BIRD.key]);
    expect(moveSelect(attackerRegion())).toBeEnabled();
    expect(moveSelect(attackerRegion())).toHaveValue(MOVE_PHYS.id);
  });

  test("自分と候補で別の種族を検索すると、それぞれの learnset が出て、request に選んだ技の ID が入る", async () => {
    const { user, client } = renderOnline();
    await chooseBySearch(user, attackerRegion(), BIRD);
    await chooseBySearch(user, candidate(1), FISH);
    await waitFor(() => {
      expect(moveOptionValues(candidate(1))).toEqual([MOVE_SPEC.id]);
    });
    await user.selectOptions(
      within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel),
      NATURE_PLUS_SPE.id,
    );
    await user.selectOptions(
      within(candidate(1)).getByLabelText(judgeScreenText.natureLabel),
      NATURE_NEUTRAL.id,
    );

    await user.click(submitButton());

    expect(lastCall(client).args.attacker.speciesKey).toBe(BIRD.key);
    expect(lastCall(client).args.moveId).toBe(MOVE_PHYS.id);
    expect(lastCall(client).args.defenders[0]?.moveId).toBe(MOVE_SPEC.id);
  });

  test("同じ欄で種族を検索し直すと、技の候補は新しい種族の learnset に替わる", async () => {
    const { user } = renderOnline();
    await chooseBySearch(user, attackerRegion(), BIRD);
    await waitFor(() => {
      expect(moveOptionValues(attackerRegion())).toHaveLength(3);
    });

    const input = within(attackerRegion()).getByRole("combobox", { name: judgeScreenText.speciesLabel });
    await user.clear(input);
    await chooseBySearch(user, attackerRegion(), FISH);

    await waitFor(() => {
      expect(moveOptionValues(attackerRegion())).toEqual([MOVE_SPEC.id]);
    });
    expect(moveSelect(attackerRegion())).toHaveValue(MOVE_SPEC.id);
  });

  test("解決した種族が技を1件も覚えない(learnset が空)なら、select は disabled・案内を出す。ID の入力欄は出さない", async () => {
    const { user } = renderOnline();
    await chooseBySearch(user, candidate(1), NOMOVE);

    expect(await within(candidate(1)).findByText(judgeScreenText.moveUnavailableNotice)).toBeInTheDocument();
    expect(moveSelect(candidate(1))).toBeDisabled();
    expect(within(candidate(1)).queryByRole("textbox", { name: /技/ })).toBeNull();
  });

  test("検索で種族を選ぶだけで、選択済みの無振りが性格・SP に入り、そのまま送信できる", async () => {
    const { user, client } = renderOnline();
    await chooseBySearch(user, attackerRegion(), BIRD);
    await chooseBySearch(user, candidate(1), FISH);
    await waitFor(() => {
      expect(moveOptionValues(candidate(1))).toEqual([MOVE_SPEC.id]);
    });

    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(
      NATURE_NEUTRAL.id,
    );
    await user.click(submitButton());

    expect(client.calls).toHaveLength(1);
    expect(lastCall(client).args.attacker.natureId).toBe(NATURE_NEUTRAL.id);
    expect(lastCall(client).args.defenders[0]?.natureId).toBe(NATURE_NEUTRAL.id);
  });

  test("種族の解決に失敗したら(resolveSpecies が reject)技は選べないまま。判定は送らない", async () => {
    const base = createFakeSpeciesSearch({ species: [BIRD, FISH], abilities: [ABILITY], moves: FAKE_MOVES });
    const failing = {
      searchSpecies: base.searchSpecies.bind(base),
      resolveSpecies: () => Promise.reject(new Error("テスト用の失敗")),
    };
    const { user, client } = renderOnline(Object.assign(base, failing));
    await user.type(
      within(attackerRegion()).getByRole("combobox", { name: judgeScreenText.speciesLabel }),
      BIRD.nameJa,
    );
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await within(attackerRegion()).findByRole("option", { name: BIRD.nameJa }));

    expect(moveSelect(attackerRegion())).toBeDisabled();
    expect(moveOptionValues(attackerRegion())).toEqual([]);
    await user.click(submitButton());
    expect(client.calls).toHaveLength(0);
  });
});

// ---- A12: 調整プリセット(issue #309。ADR-0711。攻撃側プリセット・防御側プリセットを再利用 + 最速) ----

describe("A12 調整プリセット(SP・性格がまとめて入る)", () => {
  test("自分側・候補の両方に、無振り・最速・攻撃特化・HB特化・HD特化のラジオが出て、既定は無振り", () => {
    renderScreen();
    for (const region of [attackerRegion(), candidate(1)]) {
      const labels = [
        attackerPresetLabel("none", "physical"),
        judgeScreenText.fastestPresetLabel,
        attackerPresetLabel("x_full", "physical"),
        defenderPresetLabel("hb_full"),
        defenderPresetLabel("hd_full"),
      ];
      for (const label of labels) {
        expect(presetRadio(region, label)).toBeInTheDocument();
      }
      expect(presetRadio(region, attackerPresetLabel("none", "physical"))).toBeChecked();
    }
  });

  test("最速: 素早さに全振り(SP 32)+ 素早さ上昇の性格。物理技ならようき(下降は特攻)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);

    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));

    expect(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel)).toBeChecked();
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_JOLLY.id);
    await openDetails(user, attackerRegion());
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe"))).toHaveValue(32);
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("atk"))).toHaveValue(0);

    await user.click(submitButton());
    expect(lastCall(client).args.attacker.natureId).toBe(NATURE_JOLLY.id);
    expect(lastCall(client).args.attacker.sp).toEqual({ hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 32 });
  });

  test("最速は、使う技が特殊技ならおくびょう(下降は攻撃)。技を替えると読み替える", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_JOLLY.id);

    await user.selectOptions(moveSelect(attackerRegion()), MOVE_SPEC.id);

    expect(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel)).toBeChecked();
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_TIMID.id);
  });

  test("攻撃特化は、使う技の分類で A / C に替わる(計算画面の攻撃側プリセットと同じ)。技を替えても選びは保たれる", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);

    // 物理技(既定): A特化 = 攻撃に全振り + 攻撃上昇(いじっぱり)。
    await user.click(presetRadio(attackerRegion(), attackerPresetLabel("x_full", "physical")));
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(
      NATURE_ADAMANT.id,
    );
    await openDetails(user, attackerRegion());
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("atk"))).toHaveValue(32);

    // 特殊技へ替える: ラジオの表示名は C特化 になり、選びは保たれ、SP・性格も C 側へ読み替わる。
    await user.selectOptions(moveSelect(attackerRegion()), MOVE_SPEC.id);
    expect(presetRadio(attackerRegion(), attackerPresetLabel("x_full", "special"))).toBeChecked();
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(
      NATURE_MODEST.id,
    );
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spa"))).toHaveValue(32);
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("atk"))).toHaveValue(0);
  });

  test("HB特化は HP と防御に 32 + 防御上昇の性格、HD特化は HP と特防に 32 + 特防上昇の性格(防御側プリセットと同じ値)", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, candidate(1));

    await user.click(presetRadio(candidate(1), defenderPresetLabel("hb_full")));
    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_BOLD.id);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("hp"))).toHaveValue(32);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("def"))).toHaveValue(32);

    await user.click(presetRadio(candidate(1), defenderPresetLabel("hd_full")));
    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_CALM.id);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("def"))).toHaveValue(0);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("spd"))).toHaveValue(32);
  });

  test("無振りに戻すと SP が全部 0・性格が無補正に戻る", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));

    await user.click(presetRadio(attackerRegion(), attackerPresetLabel("none", "physical")));

    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(
      NATURE_NEUTRAL.id,
    );
    await openDetails(user, attackerRegion());
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe"))).toHaveValue(0);
  });

  test("プリセットは自分と候補で独立している(自分を最速にしても候補は変わらない)", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);

    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));

    expect(presetRadio(candidate(1), attackerPresetLabel("none", "physical"))).toBeChecked();
    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_NEUTRAL.id);
  });

  test("SP や性格を手で変えると、どのプリセットも選ばれていない状態になる(値は手で入れたまま)", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));
    await openDetails(user, attackerRegion());

    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("hp")), 4);
    const group = within(attackerRegion()).getByRole("radiogroup", {
      name: judgeScreenText.presetGroupLabel,
    });
    for (const radio of within(group).getAllByRole("radio")) {
      expect(radio).not.toBeChecked();
    }
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe"))).toHaveValue(32);

    // 性格を手で替えた場合も同じ。
    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));
    await user.selectOptions(
      within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel),
      NATURE_NEUTRAL.id,
    );
    for (const radio of within(group).getAllByRole("radio")) {
      expect(radio).not.toBeChecked();
    }
  });
});

// ---- A13: 数値の直接入力は「詳細」に畳む(issue #309。requirements.md §2) ----

describe("A13 SP6欄・ランク5欄は「詳細」を開いたときだけ出す", () => {
  const SP_KEYS: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];
  const RANK_KEYS: readonly StatKey[] = ["atk", "def", "spa", "spd", "spe"];

  /** 閉じている間は DOM に無いか、あっても見えない。 */
  function expectNotVisible(region: HTMLElement, label: string): void {
    const field = within(region).queryByLabelText(label);
    if (field !== null) {
      expect(field).not.toBeVisible();
    }
  }

  test("初期表示では、自分も候補も SP・ランクの数値欄は見えず、「詳細」の開閉だけが見える", () => {
    renderScreen();
    for (const region of [attackerRegion(), candidate(1)]) {
      expect(within(region).getByText(judgeScreenText.detailsSummaryLabel)).toBeVisible();
      for (const stat of SP_KEYS) {
        expectNotVisible(region, judgeScreenText.spLabel(stat));
      }
      for (const stat of RANK_KEYS) {
        expectNotVisible(region, judgeScreenText.rankLabel(stat));
      }
    }
  });

  test("種族・性格・特性・持ち物・技・調整は、詳細を開かなくても見える", () => {
    renderScreen();
    for (const region of [attackerRegion(), candidate(1)]) {
      for (const label of [
        judgeScreenText.speciesLabel,
        judgeScreenText.natureLabel,
        judgeScreenText.abilityLabel,
        judgeScreenText.itemLabel,
        judgeScreenText.moveLabel,
      ]) {
        expect(within(region).getByLabelText(label)).toBeVisible();
      }
      expect(
        within(region).getByRole("radiogroup", { name: judgeScreenText.presetGroupLabel }),
      ).toBeVisible();
    }
  });

  test("「詳細」を開くと SP6欄・ランク5欄が見える。開閉は自分と候補ごとに独立している", async () => {
    const { user } = renderScreen();
    await openDetails(user, attackerRegion());

    for (const stat of SP_KEYS) {
      expect(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel(stat))).toBeVisible();
    }
    for (const stat of RANK_KEYS) {
      expect(within(attackerRegion()).getByLabelText(judgeScreenText.rankLabel(stat))).toBeVisible();
    }
    // 候補は閉じたまま。
    expectNotVisible(candidate(1), judgeScreenText.spLabel("spe"));
  });

  test("「詳細」を閉じても入力した値は残り、送られる(畳んだだけで捨てない)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe")), 20);
    await openDetails(user, attackerRegion()); // 閉じる

    expectNotVisible(attackerRegion(), judgeScreenText.spLabel("spe"));
    await user.click(submitButton());
    expect(lastCall(client).args.attacker.sp.spe).toBe(20);
  });

  test("候補を追加すると、追加した候補の詳細も閉じた状態で始まる", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());
    expectNotVisible(candidate(2), judgeScreenText.spLabel("hp"));
  });
});

// ---- A14: 検証エラーを該当欄に出す(issue #309。どの体かも示す) ----

describe("A14 検証エラーは該当欄に aria-invalid と文言で出す(どの体かを添える)", () => {
  /** aria-invalid が true か。 */
  function isInvalid(element: HTMLElement): boolean {
    return element.getAttribute("aria-invalid") === "true";
  }

  test("SP が範囲外: 該当の SP 欄だけが aria-invalid で、自分と理由を文言で示す。詳細は自動で開く", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await setNumber(
      user,
      within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe")),
      MAX_SP_PER_STAT + 1,
    );
    await openDetails(user, attackerRegion()); // 閉じてから送る

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    const field = within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe"));
    expect(field).toBeVisible();
    expect(isInvalid(field)).toBe(true);
    expect(field).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.spRangeMessage(MAX_SP_PER_STAT)),
    );
    expect(field).toHaveAccessibleDescription(expect.stringContaining(judgeScreenText.attackerWhoLabel));
    // 他の SP 欄・候補の欄は invalid にしない。
    expect(isInvalid(within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("atk")))).toBe(false);
    await openDetails(user, candidate(1));
    expect(isInvalid(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("spe")))).toBe(false);
  });

  test("SP の合計超過: その体の SP 6欄が aria-invalid で、相手候補何番かを文言で示す", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);
    await openDetails(user, candidate(2));
    for (const stat of ["hp", "atk", "def"] as const) {
      await setNumber(user, within(candidate(2)).getByLabelText(judgeScreenText.spLabel(stat)), 32);
    }

    await user.click(submitButton());

    for (const stat of ["hp", "atk", "def", "spa", "spd", "spe"] as const) {
      const field = within(candidate(2)).getByLabelText(judgeScreenText.spLabel(stat));
      expect(isInvalid(field)).toBe(true);
      expect(field).toHaveAccessibleDescription(
        expect.stringContaining(judgeScreenText.spTotalMessage(MAX_SP_TOTAL)),
      );
      expect(field).toHaveAccessibleDescription(
        expect.stringContaining(judgeScreenText.candidateGroupLabel(2)),
      );
    }
    // 候補1・自分は無関係。
    await openDetails(user, candidate(1));
    expect(isInvalid(within(candidate(1)).getByLabelText(judgeScreenText.spLabel("hp")))).toBe(false);
  });

  test("ランクが範囲外: 該当のランク欄だけが aria-invalid で、相手候補1と理由を示す", async () => {
    const { user } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, candidate(1));
    await setNumber(user, within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("def")), 7);

    await user.click(submitButton());

    const field = within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("def"));
    expect(isInvalid(field)).toBe(true);
    expect(field).toHaveAccessibleDescription(expect.stringContaining(judgeScreenText.rankRangeMessage));
    expect(field).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.candidateGroupLabel(1)),
    );
    expect(isInvalid(within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("atk")))).toBe(false);
  });

  test("複数の体の誤りを同時に出す(最初の1件で止めない)。それぞれに誰の誤りかが付く", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    await openDetails(user, candidate(1));
    await setNumber(user, within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("hp")), 33);
    await setNumber(user, within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("spe")), -7);

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    const attackerField = within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("hp"));
    const candidateField = within(candidate(1)).getByLabelText(judgeScreenText.rankLabel("spe"));
    expect(isInvalid(attackerField)).toBe(true);
    expect(isInvalid(candidateField)).toBe(true);
    expect(attackerField).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.attackerWhoLabel),
    );
    expect(candidateField).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.candidateGroupLabel(1)),
    );
  });

  test("必須の欄(ポケモン・性格)が空: その欄が aria-invalid になり、体が分かる。全体の alert も従来どおり出る", async () => {
    const { user, client } = renderScreen();

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
    for (const region of [attackerRegion(), candidate(1)]) {
      for (const label of [judgeScreenText.speciesLabel, judgeScreenText.natureLabel]) {
        expect(isInvalid(within(region).getByLabelText(label))).toBe(true);
      }
    }
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.attackerWhoLabel),
    );
    expect(within(candidate(1)).getByLabelText(judgeScreenText.natureLabel)).toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.candidateGroupLabel(1)),
    );
  });

  test("learnset が引けず技が空: 技の select が aria-invalid になる", async () => {
    const { user } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await fillIndividual(user, candidate(1), NOMOVE, NATURE_NEUTRAL);

    await user.click(submitButton());

    expect(isInvalid(moveSelect(candidate(1)))).toBe(true);
    expect(isInvalid(moveSelect(attackerRegion()))).toBe(false);
  });

  test("誤りを直して送り直すと aria-invalid と文言が消え、judge が呼ばれる", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await openDetails(user, attackerRegion());
    const field = within(attackerRegion()).getByLabelText(judgeScreenText.spLabel("spe"));
    await setNumber(user, field, MAX_SP_PER_STAT + 1);
    await user.click(submitButton());
    expect(isInvalid(field)).toBe(true);

    await setNumber(user, field, MAX_SP_PER_STAT);
    await user.click(submitButton());

    expect(isInvalid(field)).toBe(false);
    expect(field).not.toHaveAccessibleDescription(
      expect.stringContaining(judgeScreenText.spRangeMessage(MAX_SP_PER_STAT)),
    );
    expect(client.calls).toHaveLength(1);
  });
});

// ---- A15: 種族を選ぶだけでプリセットの SP・性格が入る(選択済みの調整と値の整合) ----

describe("A15 種族を選ぶと、選択済みのプリセットが性格・SP に入る", () => {
  test("自分・候補とも、種族を選ぶだけで性格(無振り=無補正)が入り、そのまま送信できる", async () => {
    const { user, client } = renderScreen();
    await user.selectOptions(within(attackerRegion()).getByLabelText(judgeScreenText.speciesLabel), BIRD.key);
    await user.selectOptions(within(candidate(1)).getByLabelText(judgeScreenText.speciesLabel), FISH.key);

    for (const region of [attackerRegion(), candidate(1)]) {
      expect(within(region).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_NEUTRAL.id);
      expect(presetRadio(region, attackerPresetLabel("none", "physical"))).toBeChecked();
    }
    await user.click(submitButton());

    expect(client.calls).toHaveLength(1);
    expect(lastCall(client).args.attacker.natureId).toBe(NATURE_NEUTRAL.id);
    expect(lastCall(client).args.attacker.sp).toEqual({ hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 });
  });

  test("先にプリセット(最速)を選んでから種族を選ぶと、性格・SP が入る", async () => {
    const { user } = renderScreen();
    await user.click(presetRadio(attackerRegion(), judgeScreenText.fastestPresetLabel));
    await user.selectOptions(within(attackerRegion()).getByLabelText(judgeScreenText.speciesLabel), BIRD.key);

    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(NATURE_JOLLY.id);
  });

  test("性格を手で選んだ体は、種族を替えても上書きしない", async () => {
    const { user } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await user.selectOptions(within(attackerRegion()).getByLabelText(judgeScreenText.speciesLabel), FISH.key);

    expect(within(attackerRegion()).getByLabelText(judgeScreenText.natureLabel)).toHaveValue(
      NATURE_PLUS_SPE.id,
    );
  });
});
