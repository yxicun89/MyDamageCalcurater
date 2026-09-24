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
//   A9 マスタ: speciesList が false なら検索欄、技は master.moves が空でも ID で入力できる(ADR-0304 §3)
// 架空データだけを使う(実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import type { Ability, Item, StatKey, TypeChart } from "../engine/types";
import { judgeErrorText, judgeScreenText } from "../i18n/ja";
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

function fakeSpecies(key: string, nameJa: string, types: readonly string[]): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: 0,
    nameJa,
    types,
    baseStats: { hp: 70, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: ["test-ability-a"],
    learnset: ["test-move"],
  };
}

const BIRD = fakeSpecies("9001-000", "テストカソウドリ", ["fire", "flying"]);
const FISH = fakeSpecies("9002-000", "テストカソウギョ", ["water"]);
const GRASS = fakeSpecies("9003-000", "テストカソウソウ", ["grass"]);

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

const ABILITY: Ability = { id: "test-ability-a", nameJa: "テストとくせい", effect: null };
const ITEM: Item = { id: "test-item-a", nameJa: "テストもちもの", effect: null };

/**
 * 画面が使うマスタ。**技は空**(オンラインでは capabilities.moves が false で、ID から技を引く公開 API が
 * 無い。ADR-0304 §3)。判定の画面はそれでも成立する(技は ID の自由入力。ADR-0705 §5)。
 */
const master: MasterData = {
  species: [BIRD, FISH, GRASS],
  moves: [],
  items: [ITEM],
  abilities: [ABILITY],
  natures: [NATURE_PLUS_SPE, NATURE_NEUTRAL],
  typeChart: emptyTypeChart,
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

/** 自分 + 候補1件の、判定を通せる最小の入力。 */
async function fillMinimalForm(user: UserEvent): Promise<void> {
  await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
  await user.type(within(attackerRegion()).getByLabelText(judgeScreenText.moveIdLabel), "test-move");
  await fillIndividual(user, candidate(1), FISH, NATURE_NEUTRAL);
  await user.type(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel), "test-defender-move-0");
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

  test("技は ID の自由入力で、一覧から選べない理由の案内が出る(ADR-0304 §3・ADR-0705 §5)", () => {
    renderScreen();
    expect(within(attackerRegion()).getByLabelText(judgeScreenText.moveIdLabel)).toHaveAttribute(
      "type",
      "text",
    );
    expect(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel)).toHaveAttribute("type", "text");
    expect(screen.getAllByText(judgeScreenText.moveIdHint).length).toBeGreaterThan(0);
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

  test("候補の入力は独立している(ある候補の技 ID が他の候補に混ざらない)", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());

    await user.type(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel), "test-move-one");
    await user.type(within(candidate(2)).getByLabelText(judgeScreenText.moveIdLabel), "test-move-two");

    expect(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel)).toHaveValue("test-move-one");
    expect(within(candidate(2)).getByLabelText(judgeScreenText.moveIdLabel)).toHaveValue("test-move-two");
  });

  test("削除しても残った候補の入力が保たれる(index のずれで値が動かない)", async () => {
    const { user } = renderScreen();
    await user.click(addCandidateButton());
    await user.type(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel), "test-move-one");
    await user.type(within(candidate(2)).getByLabelText(judgeScreenText.moveIdLabel), "test-move-two");

    await user.click(screen.getByRole("button", { name: judgeScreenText.removeCandidateLabel(1) }));

    expect(candidateCount()).toBe(1);
    expect(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel)).toHaveValue("test-move-two");
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
          moveId: "test-defender-move-0",
        },
      ],
      moveId: "test-move",
    } satisfies Schemas["OutspeedAndKoRequest"]);
  });

  test("SP は6項目そのまま送る(合計の上限内)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
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

  test("技 ID は前後の空白を落として送る", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await user.type(within(attackerRegion()).getByLabelText(judgeScreenText.moveIdLabel), "  test-move  ");
    await fillIndividual(user, candidate(1), FISH, NATURE_NEUTRAL);
    await user.type(
      within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel),
      " test-defender-move-0 ",
    );

    await user.click(submitButton());

    expect(lastCall(client).args.moveId).toBe("test-move");
    expect(lastCall(client).args.defenders[0]?.moveId).toBe("test-defender-move-0");
  });

  test("対戦形式を切り替えると format が変わる(judge は calc-svc へそのまま渡す)", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.selectOptions(screen.getByLabelText(judgeScreenText.formatLabel), "double");

    await user.click(submitButton());

    expect(lastCall(client).args.format).toBe("double");
  });

  test("候補を増やすと defenders が入力した順に並ぶ", async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
    await user.click(addCandidateButton());
    await fillIndividual(user, candidate(2), GRASS, NATURE_NEUTRAL);
    await user.type(within(candidate(2)).getByLabelText(judgeScreenText.moveIdLabel), "test-defender-move-1");

    await user.click(submitButton());

    expect(lastCall(client).args.defenders.map((entry) => entry.speciesKey)).toEqual([FISH.key, GRASS.key]);
    expect(lastCall(client).args.defenders.map((entry) => entry.moveId)).toEqual([
      "test-defender-move-0",
      "test-defender-move-1",
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
    await user.type(within(candidate(2)).getByLabelText(judgeScreenText.moveIdLabel), "test-defender-move-1");
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

    expect(within(attackerRegion()).getByLabelText(judgeScreenText.moveIdLabel)).toHaveValue("test-move");
    expect(within(candidate(1)).getByLabelText(judgeScreenText.moveIdLabel)).toHaveValue(
      "test-defender-move-0",
    );
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

  test("候補の技 ID だけが空でも呼ばない(候補の moveId は契約上必須)", async () => {
    const { user, client } = renderScreen();
    await fillIndividual(user, attackerRegion(), BIRD, NATURE_PLUS_SPE);
    await user.type(within(attackerRegion()).getByLabelText(judgeScreenText.moveIdLabel), "test-move");
    await fillIndividual(user, candidate(1), FISH, NATURE_NEUTRAL);

    await user.click(submitButton());

    expect(client.calls).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent(judgeScreenText.requiredMessage);
  });

  test(`SP が1項目でも ${String(MAX_SP_PER_STAT)} を超えたら呼ばない`, async () => {
    const { user, client } = renderScreen();
    await fillMinimalForm(user);
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

  test("master.moves が空(オンライン)でも判定を送れる(技は ID の自由入力。ADR-0304 §3)", async () => {
    const { user, client } = renderScreen();
    expect(master.moves).toHaveLength(0);

    await fillMinimalForm(user);
    await user.click(submitButton());

    expect(client.calls).toHaveLength(1);
    expect(lastCall(client).args.moveId).toBe("test-move");
  });
});
