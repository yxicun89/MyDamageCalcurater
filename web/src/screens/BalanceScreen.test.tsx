// P4-12a: タイプバランスの画面(ADR-0303 §2・§3、docs/type-balance-design.md §6 TB1/TB2・§10)。
// balance API のクライアントは fake(BalanceClient を props で注入)。マスタは架空の例データ(exampleMasterSource)。
// 確かめること:
//   - メンバーは最大6体(group「メンバーn」)。各メンバーはポケモン・特性(その種族の特性)・技1〜技4(learnset)を選ぶ
//   - 「メンバーを追加」(6体で押せない)・「メンバーnを削除」
//   - ポケモンを選んだメンバーだけを、枠の順に analyze へ送る({pokemonId, abilityId})。0体なら呼ばない
//   - 攻撃技(変化技でない技)を選んだメンバーが1体でもいれば coverage も呼ぶ(ポケモンを選んだ全メンバーを
//     枠の順に送り、技の無いメンバーは moveIds: []。同じメンバーの重複した技は1つにする)
//   - 防御相性の表(メンバー × 18タイプ。列は応答の順・タイプ名は typeNameJa)に「×2 弱点」のように文字で出す
//   - チームの集計(攻撃タイプごとの 弱点・うち×4・耐性・無効・等倍 の人数)・攻撃範囲(防御タイプごとの最大倍率・有効・抜群)
//   - 倍率・人数は応答の値をそのまま出す(Web で計算しない。ADR-0303 §1)
//   - 計算中・エラー(role=alert)・古い応答の無視

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/balance.gen";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { learnsetMoves } from "../domain/moves";
import type { Move } from "../engine/types";
import { typeNameJa } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { BalanceScreen } from "./BalanceScreen";

// P4-12b: 仮想敵(threats)とおすすめタイプ(recommendations)(ADR-0303 §7・§9、ADR-0400、ADR-0401)。
// 確かめること(この節は下の「仮想敵の枠」以降):
//   - 仮想敵の枠(group「仮想敵n」)はメンバーと同じ入力(ポケモン・特性・技1〜技4)で最大6体
//   - パーティ1体以上 かつ 仮想敵1体以上 のときだけ threats を呼ぶ({members, threats} とも ID だけ)
//   - 仮想敵ごとに表(行=メンバー、列=受ける倍率・与える倍率・安全・抜群)と、安全/抜群の人数を応答のまま出す
//   - safe / superEffective の語は応答の真偽値だけで決める(倍率から閾値を判定し直さない。ADR-0303 §7)
//   - パーティが1体以上そろえば recommendations を呼ぶ(新しい入力は無い・limit は送らない)
//   - 4つの呼び出しは独立(1つのエラー・遅延が他の表示を消さない。ADR-0303 §9)

type Schemas = components["schemas"];
type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];
type ThreatsPokemon = Schemas["ThreatsRequestPokemon"];
type RecommendationsMembers = Schemas["RecommendationsRequest"]["members"];

/** 18タイプの正準順(応答を作るためだけに使う。画面は応答の順に従う)。 */
const TYPES: readonly Schemas["TypeId"][] = [
  "normal",
  "fire",
  "water",
  "electric",
  "grass",
  "ice",
  "fighting",
  "poison",
  "ground",
  "flying",
  "psychic",
  "bug",
  "rock",
  "ghost",
  "dragon",
  "dark",
  "steel",
  "fairy",
];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

// ---- fake の BalanceClient(呼び出しを記録し、テストが応答を返す) ----

interface PendingCall<M, T> {
  readonly members: M;
  resolve(result: BalanceResult<T>): void;
}

/** threats の呼び出し1回(パーティと仮想敵の両方を記録する)。 */
interface ThreatsCall {
  readonly members: readonly ThreatsPokemon[];
  readonly threats: readonly ThreatsPokemon[];
  resolve(result: BalanceResult<Schemas["ThreatsResponse"]>): void;
}

/** recommendations の呼び出し1回(limit を送ったかどうかも記録する)。 */
interface RecommendationsCall {
  readonly members: RecommendationsMembers;
  readonly limit: number | undefined;
  resolve(result: BalanceResult<Schemas["RecommendationsResponse"]>): void;
}

interface FakeBalanceClient extends BalanceClient {
  readonly analyzeCalls: PendingCall<AnalyzeMembers, Schemas["AnalyzeResponse"]>[];
  readonly coverageCalls: PendingCall<CoverageMembers, Schemas["CoverageResponse"]>[];
  readonly threatsCalls: ThreatsCall[];
  readonly recommendationsCalls: RecommendationsCall[];
}

function createFakeBalanceClient(): FakeBalanceClient {
  const analyzeCalls: PendingCall<AnalyzeMembers, Schemas["AnalyzeResponse"]>[] = [];
  const coverageCalls: PendingCall<CoverageMembers, Schemas["CoverageResponse"]>[] = [];
  const threatsCalls: ThreatsCall[] = [];
  const recommendationsCalls: RecommendationsCall[] = [];
  return {
    analyzeCalls,
    coverageCalls,
    threatsCalls,
    recommendationsCalls,
    analyze(members) {
      return new Promise((resolve) => {
        analyzeCalls.push({ members: structuredClone(members) as AnalyzeMembers, resolve });
      });
    },
    coverage(members) {
      return new Promise((resolve) => {
        coverageCalls.push({ members: structuredClone(members) as CoverageMembers, resolve });
      });
    },
    threats(members, threats) {
      return new Promise((resolve) => {
        threatsCalls.push({
          members: structuredClone(members),
          threats: structuredClone(threats),
          resolve,
        });
      });
    },
    recommendations(members, limit) {
      return new Promise((resolve) => {
        recommendationsCalls.push({
          members: structuredClone(members) as RecommendationsMembers,
          limit,
          resolve,
        });
      });
    },
  };
}

function lastOf<T>(calls: readonly T[], name: string): T {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}

// ---- 応答を作る(値は画面がそのまま出すことを確かめるための任意の値) ----

type DefenseSpec = Partial<
  Record<Schemas["TypeId"], readonly [Schemas["DefenseMultiplier"], Schemas["DefenseCategory"]]>
>;

function analyzeResponse(
  members: AnalyzeMembers,
  options: { defense?: DefenseSpec; order?: readonly Schemas["TypeId"][] } = {},
): Schemas["AnalyzeResponse"] {
  const order = options.order ?? TYPES;
  return {
    members: members.map((member) => ({
      pokemonId: member.pokemonId,
      ...(member.abilityId === undefined ? {} : { abilityId: member.abilityId }),
      types: ["normal"],
      defense: order.map((attackType) => {
        const [multiplier, category] = options.defense?.[attackType] ?? (["1", "neutral"] as const);
        return { attackType, multiplier, category, source: "type", effect: "none" } as const;
      }),
    })),
    teamSummary: order.map((attackType, index) => ({
      attackType,
      // 型ごとに違う数(画面が応答をそのまま出すことの確認用。不変条件は見ない)。
      weak: index % 3,
      quadWeak: index % 2,
      resist: (index + 1) % 4,
      immune: index % 5 === 0 ? 1 : 0,
      neutral: members.length,
    })),
  };
}

function coverageResponse(members: CoverageMembers): Schemas["CoverageResponse"] {
  return {
    members: members.map((member) => ({
      pokemonId: member.pokemonId,
      moveIds: member.moveIds,
      attackTypes: [],
      coverage: TYPES.map((defenseType) => ({
        defenseType,
        bestMultiplier: "1",
        effective: true,
        superEffective: false,
      })),
    })),
    teamCoverage: TYPES.map((defenseType, index) => ({
      defenseType,
      bestMultiplier: (["2", "1", "1/2", "0"] as const)[index % 4] ?? "1",
      effectiveMembers: index % 3,
      superEffectiveMembers: index % 2,
    })),
  };
}

// ---- 画面の操作 ----

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

/** 特性を2つ以上持つ種族。 */
function speciesWithTwoAbilities(): MasterSpecies {
  const species = master.species.find((candidate) => candidate.abilities.length >= 2);
  if (species === undefined) {
    throw new Error("例データに特性を2つ以上持つ種族が無い");
  }
  return species;
}

/** 攻撃技を2つ以上と変化技を覚える種族。 */
function speciesWithMoves(): { species: MasterSpecies; damaging: Move[]; status: Move } {
  for (const species of master.species) {
    const moves = learnsetMoves(species, master.moves);
    const damaging = moves.filter((move) => move.category !== "status");
    const status = moves.find((move) => move.category === "status");
    if (damaging.length >= 2 && status !== undefined) {
      return { species, damaging, status };
    }
  }
  throw new Error("例データに攻撃技2つと変化技を覚える種族が無い");
}

function abilityName(id: string): string {
  const ability = master.abilities.find((candidate) => candidate.id === id);
  if (ability === undefined) {
    throw new Error(`例データに特性 ${id} が無い`);
  }
  return ability.nameJa;
}

function memberGroup(index: number): HTMLElement {
  return screen.getByRole("group", { name: `メンバー${String(index)}` });
}

async function selectSpecies(user: UserEvent, index: number, species: MasterSpecies): Promise<void> {
  await user.selectOptions(
    within(memberGroup(index)).getByRole("combobox", { name: "ポケモン" }),
    species.key,
  );
}

async function selectMove(user: UserEvent, index: number, slot: number, move: Move | null): Promise<void> {
  await user.selectOptions(
    within(memberGroup(index)).getByRole("combobox", { name: `技${String(slot)}` }),
    move === null ? "" : move.id,
  );
}

function renderScreen(client: FakeBalanceClient = createFakeBalanceClient()) {
  const user = userEvent.setup();
  render(<BalanceScreen master={master} client={client} />);
  return { user, client };
}

/** 表の中で、行見出しが name の行のセル(見出し以外)の文字列。 */
function rowCells(table: HTMLElement, name: string): string[] {
  const header = within(table).getByRole("rowheader", { name });
  const row = header.closest("tr");
  if (row === null) {
    throw new Error(`${name} の行が無い`);
  }
  return within(row)
    .getAllByRole("cell")
    .map((cell) => cell.textContent.trim());
}

function columnHeaders(table: HTMLElement): string[] {
  return within(table)
    .getAllByRole("columnheader")
    .map((cell) => cell.textContent.trim());
}

async function resolveAnalyze(client: FakeBalanceClient, options?: Parameters<typeof analyzeResponse>[1]) {
  const call = lastOf(client.analyzeCalls, "analyze");
  await act(async () => {
    call.resolve({ ok: true, value: analyzeResponse(call.members, options) });
    await Promise.resolve();
  });
}

// ---- P4-12b: 仮想敵・おすすめタイプの操作と応答 ----

function threatGroup(index: number): HTMLElement {
  return screen.getByRole("group", { name: `仮想敵${String(index)}` });
}

async function addThreat(user: UserEvent): Promise<void> {
  await user.click(screen.getByRole("button", { name: "仮想敵を追加" }));
}

async function selectThreatSpecies(user: UserEvent, index: number, species: MasterSpecies): Promise<void> {
  await user.selectOptions(
    within(threatGroup(index)).getByRole("combobox", { name: "ポケモン" }),
    species.key,
  );
}

async function selectThreatMove(
  user: UserEvent,
  index: number,
  slot: number,
  move: Move | null,
): Promise<void> {
  await user.selectOptions(
    within(threatGroup(index)).getByRole("combobox", { name: `技${String(slot)}` }),
    move === null ? "" : move.id,
  );
}

/** 仮想敵1体 × メンバー1人の相性(値は画面がそのまま出すことを確かめるための任意の値)。 */
interface MatchupSpec {
  readonly incoming: Schemas["MatchupMultiplier"];
  readonly outgoing: Schemas["MatchupMultiplier"];
  readonly safe: boolean;
  readonly superEffective: boolean;
}

const NEUTRAL_MATCHUP: MatchupSpec = { incoming: "1", outgoing: "1", safe: false, superEffective: false };

interface ThreatsResponseOptions {
  /** 仮想敵 × メンバーの相性(既定は等倍・安全でない・抜群でない)。 */
  readonly matchup?: (threatIndex: number, memberIndex: number) => MatchupSpec;
  /** 集計の人数(既定は matchup から数える。応答のまま出すかを見るときだけ上書きする)。 */
  readonly counts?: (threatIndex: number) => {
    readonly safeMembers: number;
    readonly superEffectiveMembers: number;
  };
}

function threatsResponse(
  call: ThreatsCall,
  options: ThreatsResponseOptions = {},
): Schemas["ThreatsResponse"] {
  return {
    threats: call.threats.map((threat, threatIndex) => {
      const matchups = call.members.map((member, memberIndex) => ({
        pokemonId: member.pokemonId,
        ...(options.matchup?.(threatIndex, memberIndex) ?? NEUTRAL_MATCHUP),
      }));
      const counts = options.counts?.(threatIndex) ?? {
        safeMembers: matchups.filter((matchup) => matchup.safe).length,
        superEffectiveMembers: matchups.filter((matchup) => matchup.superEffective).length,
      };
      return {
        pokemonId: threat.pokemonId,
        ...(threat.abilityId === undefined ? {} : { abilityId: threat.abilityId }),
        attackTypes: [],
        matchups,
        ...counts,
      };
    }),
  };
}

function recommendationsResponse(
  overrides: Partial<Schemas["RecommendationsResponse"]> = {},
): Schemas["RecommendationsResponse"] {
  return { defenseHoles: [], offenseHoles: [], candidates: [], abilityOptions: [], ...overrides };
}

async function resolveThreats(client: FakeBalanceClient, options?: ThreatsResponseOptions): Promise<void> {
  const call = lastOf(client.threatsCalls, "threats");
  await act(async () => {
    call.resolve({ ok: true, value: threatsResponse(call, options) });
    await Promise.resolve();
  });
}

async function resolveRecommendations(
  client: FakeBalanceClient,
  overrides?: Partial<Schemas["RecommendationsResponse"]>,
): Promise<void> {
  const call = lastOf(client.recommendationsCalls, "recommendations");
  await act(async () => {
    call.resolve({ ok: true, value: recommendationsResponse(overrides) });
    await Promise.resolve();
  });
}

/** 仮想敵 n の結果のかたまり(表と人数の集計を含む region)。 */
function threatRegion(n: number, nameJa: string): HTMLElement {
  return screen.getByRole("region", { name: `仮想敵${String(n)}(${nameJa})` });
}

function recommendationsRegion(): HTMLElement {
  return screen.getByRole("region", { name: "おすすめタイプ" });
}

// ---- テスト ----

describe("メンバーの枠", () => {
  test("最初はメンバー1の枠が1つ(ポケモン・特性・技1〜技4)で、balance を呼ばず、表も出さない", () => {
    const { client } = renderScreen();
    const group = memberGroup(1);
    expect(within(group).getByRole("combobox", { name: "ポケモン" })).toHaveValue("");
    expect(within(group).getByRole("combobox", { name: "特性" })).toBeInTheDocument();
    for (const slot of [1, 2, 3, 4]) {
      expect(within(group).getByRole("combobox", { name: `技${String(slot)}` })).toBeInTheDocument();
    }
    expect(client.analyzeCalls).toHaveLength(0);
    expect(client.coverageCalls).toHaveLength(0);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  test("ポケモンの選択肢は例データの全種族(名前で出す)", () => {
    renderScreen();
    const select = within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" });
    for (const species of master.species) {
      expect(within(select).getByRole("option", { name: species.nameJa })).toHaveValue(species.key);
    }
  });

  test("「メンバーを追加」で6体まで枠が増え、6体では押せない", async () => {
    const { user } = renderScreen();
    const add = screen.getByRole("button", { name: "メンバーを追加" });
    for (let count = 2; count <= 6; count += 1) {
      await user.click(add);
      expect(memberGroup(count)).toBeInTheDocument();
    }
    expect(screen.getAllByRole("group", { name: /^メンバー\d$/ })).toHaveLength(6);
    expect(add).toBeDisabled();
  });

  test("「メンバー1を削除」で枠を消し、残りを詰めて送り直す", async () => {
    const { user, client } = renderScreen();
    const first = speciesAt(0);
    const second = speciesAt(1);
    await selectSpecies(user, 1, first);
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await selectSpecies(user, 2, second);
    expect(lastOf(client.analyzeCalls, "analyze").members.map((member) => member.pokemonId)).toEqual([
      first.key,
      second.key,
    ]);

    await user.click(screen.getByRole("button", { name: "メンバー1を削除" }));
    expect(screen.getAllByRole("group", { name: /^メンバー\d$/ })).toHaveLength(1);
    expect(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" })).toHaveValue(second.key);
    expect(lastOf(client.analyzeCalls, "analyze").members.map((member) => member.pokemonId)).toEqual([
      second.key,
    ]);
  });
});

describe("analyze(防御相性)", () => {
  test("ポケモンを選ぶと、そのメンバーを {pokemonId, abilityId(種族の最初の特性)} で送る", async () => {
    const { user, client } = renderScreen();
    const species = speciesAt(0);
    await selectSpecies(user, 1, species);
    expect(lastOf(client.analyzeCalls, "analyze").members).toEqual([
      { pokemonId: species.key, abilityId: species.abilities[0] },
    ]);
  });

  test("ポケモンを選んでいない枠は送らない(枠の順を保つ)", async () => {
    const { user, client } = renderScreen();
    const species = speciesAt(2);
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await selectSpecies(user, 3, species);
    expect(lastOf(client.analyzeCalls, "analyze").members).toEqual([
      { pokemonId: species.key, abilityId: species.abilities[0] },
    ]);
  });

  test("特性の選択肢はその種族の特性だけで、選び直すと abilityId を変えて送り直す", async () => {
    const { user, client } = renderScreen();
    const species = speciesWithTwoAbilities();
    await selectSpecies(user, 1, species);
    const abilitySelect = within(memberGroup(1)).getByRole("combobox", { name: "特性" });
    expect(
      within(abilitySelect)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(species.abilities.map(abilityName));
    const secondAbility = species.abilities[1] ?? "";
    await user.selectOptions(abilitySelect, secondAbility);
    expect(lastOf(client.analyzeCalls, "analyze").members).toEqual([
      { pokemonId: species.key, abilityId: secondAbility },
    ]);
  });

  test("応答を待つ間は「計算中」を出し、届いたら防御相性の表(メンバー × 18タイプ)を文字の倍率で出す", async () => {
    const { user, client } = renderScreen();
    const species = speciesAt(0);
    await selectSpecies(user, 1, species);
    expect(screen.getByText("計算中")).toBeInTheDocument();

    await resolveAnalyze(client, {
      defense: {
        fire: ["4", "quad_weak"],
        water: ["2", "weak"],
        grass: ["1/2", "resist"],
        ice: ["1/4", "quad_resist"],
        ground: ["0", "immune"],
        steel: ["3/4", "resist"],
      },
    });

    expect(screen.queryByText("計算中")).not.toBeInTheDocument();
    const table = screen.getByRole("table", { name: "防御相性" });
    expect(columnHeaders(table)).toEqual(["メンバー", ...TYPES.map((type) => typeNameJa[type])]);
    const cells = rowCells(table, species.nameJa);
    expect(cells).toHaveLength(18);
    const cellOf = (type: Schemas["TypeId"]) => cells[TYPES.indexOf(type)];
    expect(cellOf("fire")).toBe("×4 弱点");
    expect(cellOf("water")).toBe("×2 弱点");
    expect(cellOf("normal")).toBe("×1 等倍");
    expect(cellOf("grass")).toBe("×1/2 耐性");
    expect(cellOf("ice")).toBe("×1/4 耐性");
    expect(cellOf("ground")).toBe("×0 無効");
    // 特性の倍率も応答の文字列のまま(Web で計算しない)。
    expect(cellOf("steel")).toBe("×3/4 耐性");
  });

  test("列の並びは応答の順に従う(タイプの順をコードに持たない)", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    const reversed = [...TYPES].reverse();
    await resolveAnalyze(client, { order: reversed });
    const table = screen.getByRole("table", { name: "防御相性" });
    expect(columnHeaders(table)).toEqual(["メンバー", ...reversed.map((type) => typeNameJa[type])]);
  });

  test("チームの集計: 攻撃タイプごとに 弱点・うち×4・耐性・無効・等倍 の人数を応答のまま出す", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await selectSpecies(user, 2, speciesAt(1));
    const call = lastOf(client.analyzeCalls, "analyze");
    const response = analyzeResponse(call.members);
    await act(async () => {
      call.resolve({ ok: true, value: response });
      await Promise.resolve();
    });

    const table = screen.getByRole("table", { name: "チームの集計" });
    expect(columnHeaders(table)).toEqual(["攻撃タイプ", "弱点", "うち×4", "耐性", "無効", "等倍"]);
    for (const entry of response.teamSummary) {
      expect(rowCells(table, typeNameJa[entry.attackType])).toEqual([
        String(entry.weak),
        String(entry.quadWeak),
        String(entry.resist),
        String(entry.immune),
        String(entry.neutral),
      ]);
    }
    // 防御相性の表はメンバーごとに1行。
    const defense = screen.getByRole("table", { name: "防御相性" });
    expect(rowCells(defense, speciesAt(0).nameJa)).toHaveLength(18);
    expect(rowCells(defense, speciesAt(1).nameJa)).toHaveLength(18);
  });

  test("エラーは role=alert で知らせ、表を出さない", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    const call = lastOf(client.analyzeCalls, "analyze");
    await act(async () => {
      call.resolve({
        ok: false,
        error: { code: "balance_unavailable", message: "タイプバランスの API に接続できません" },
      });
      await Promise.resolve();
    });
    expect(screen.getByRole("alert")).toHaveTextContent("タイプバランスの API に接続できません");
    expect(screen.queryByRole("table", { name: "防御相性" })).not.toBeInTheDocument();
    expect(screen.queryByText("計算中")).not.toBeInTheDocument();
  });

  test("古い応答は無視する(後から届いた前の選択の応答で表を上書きしない)", async () => {
    const { user, client } = renderScreen();
    const first = speciesAt(0);
    const second = speciesAt(1);
    await selectSpecies(user, 1, first);
    const staleCall = lastOf(client.analyzeCalls, "analyze");
    await selectSpecies(user, 1, second);
    const freshCall = lastOf(client.analyzeCalls, "analyze");
    expect(freshCall).not.toBe(staleCall);

    await act(async () => {
      freshCall.resolve({ ok: true, value: analyzeResponse(freshCall.members) });
      await Promise.resolve();
    });
    await act(async () => {
      staleCall.resolve({ ok: true, value: analyzeResponse(staleCall.members) });
      await Promise.resolve();
    });

    const table = screen.getByRole("table", { name: "防御相性" });
    expect(within(table).getByRole("rowheader", { name: second.nameJa })).toBeInTheDocument();
    expect(within(table).queryByRole("rowheader", { name: first.nameJa })).not.toBeInTheDocument();
  });

  test("選択を変えると、新しい応答が届くまで前の表を出したままにせず「計算中」に戻す", async () => {
    const { user, client } = renderScreen();
    const first = speciesAt(0);
    const second = speciesAt(1);
    await selectSpecies(user, 1, first);
    await resolveAnalyze(client);
    expect(screen.getByRole("table", { name: "防御相性" })).toBeInTheDocument();

    // 新しい選択をしても、その応答が届くまでは古い表を残さない(CalcScreen と同じ約束)。
    await selectSpecies(user, 1, second);
    expect(screen.getByText("計算中")).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "防御相性" })).not.toBeInTheDocument();
  });

  test("全メンバーのポケモンを外すと表を消し、呼ばない", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    await resolveAnalyze(client);
    expect(screen.getByRole("table", { name: "防御相性" })).toBeInTheDocument();
    const callsBefore = client.analyzeCalls.length;

    await user.selectOptions(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }), "");
    expect(screen.queryByRole("table", { name: "防御相性" })).not.toBeInTheDocument();
    expect(client.analyzeCalls).toHaveLength(callsBefore);
  });
});

describe("coverage(攻撃範囲)", () => {
  test("技の選択肢はその種族の覚える技(と「なし」)", async () => {
    const { user } = renderScreen();
    const { species } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    const select = within(memberGroup(1)).getByRole("combobox", { name: "技1" });
    expect(
      within(select)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["なし", ...learnsetMoves(species, master.moves).map((move) => move.nameJa)]);
  });

  test("攻撃技を選ぶまでは coverage を呼ばず、攻撃範囲の表も出さない(変化技だけでも呼ばない)", async () => {
    const { user, client } = renderScreen();
    const { species, status } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, status);
    expect(client.coverageCalls).toHaveLength(0);
    expect(screen.queryByRole("table", { name: "攻撃範囲" })).not.toBeInTheDocument();
    // analyze は技によらず呼ぶ。
    expect(client.analyzeCalls.length).toBeGreaterThan(0);
  });

  test("攻撃技を選ぶと、ポケモンを選んだ全メンバーを枠の順に {pokemonId, moveIds} で送る(重複は1つ)", async () => {
    const { user, client } = renderScreen();
    const { species, damaging, status } = speciesWithMoves();
    const [moveA, moveB] = damaging;
    if (moveA === undefined || moveB === undefined) {
      throw new Error("攻撃技が2つ要る");
    }
    const other = speciesAt(master.species.indexOf(species) === 0 ? 1 : 0);
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, moveA);
    await selectMove(user, 1, 2, status);
    await selectMove(user, 1, 3, moveA);
    await selectMove(user, 1, 4, moveB);
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await selectSpecies(user, 2, other);

    expect(lastOf(client.coverageCalls, "coverage").members).toEqual([
      { pokemonId: species.key, moveIds: [moveA.id, status.id, moveB.id] },
      { pokemonId: other.key, moveIds: [] },
    ]);
  });

  test("攻撃範囲の表: 防御タイプごとに 最大倍率(文字)・有効・抜群 の人数を応答のまま出す", async () => {
    const { user, client } = renderScreen();
    const { species, damaging } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, damaging[0] ?? null);
    const call = lastOf(client.coverageCalls, "coverage");
    const response = coverageResponse(call.members);
    await act(async () => {
      call.resolve({ ok: true, value: response });
      await Promise.resolve();
    });

    const table = await screen.findByRole("table", { name: "攻撃範囲" });
    expect(columnHeaders(table)).toEqual(["防御タイプ", "最大倍率", "有効", "抜群"]);
    const expectedLabel: Record<string, string> = {
      "2": "×2 抜群",
      "1": "×1 等倍",
      "1/2": "×1/2 いまひとつ",
      "0": "×0 無効",
    };
    for (const entry of response.teamCoverage) {
      expect(rowCells(table, typeNameJa[entry.defenseType])).toEqual([
        entry.bestMultiplier === null ? "攻撃技なし" : expectedLabel[entry.bestMultiplier],
        String(entry.effectiveMembers),
        String(entry.superEffectiveMembers),
      ]);
    }
  });

  test("coverage のエラーは role=alert で知らせ、防御相性の表は出したままにする", async () => {
    const { user, client } = renderScreen();
    const { species, damaging } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, damaging[0] ?? null);
    await resolveAnalyze(client);
    const call = lastOf(client.coverageCalls, "coverage");
    await act(async () => {
      call.resolve({ ok: false, error: { code: "unknown_move", message: "unknown moveId: x" } });
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("unknown moveId: x");
    });
    expect(screen.getByRole("table", { name: "防御相性" })).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "攻撃範囲" })).not.toBeInTheDocument();
  });

  test("ポケモンを選び直すと、そのメンバーの技を外す(覚えない技を送らない)", async () => {
    const { user, client } = renderScreen();
    const { species, damaging } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, damaging[0] ?? null);
    const other = master.species.find(
      (candidate) => candidate.key !== species.key && !candidate.learnset.includes(damaging[0]?.id ?? ""),
    );
    if (other === undefined) {
      throw new Error("その技を覚えない別の種族が要る");
    }
    const coverageBefore = client.coverageCalls.length;
    await selectSpecies(user, 1, other);
    for (const slot of [1, 2, 3, 4]) {
      expect(within(memberGroup(1)).getByRole("combobox", { name: `技${String(slot)}` })).toHaveValue("");
    }
    expect(client.coverageCalls).toHaveLength(coverageBefore);
  });
});

// ---- P4-12b: 仮想敵(threats)とおすすめタイプ(recommendations) ----

describe("仮想敵の枠", () => {
  test("最初は仮想敵1の枠が1つ(ポケモン・特性・技1〜技4)で、threats を呼ばない", () => {
    const { client } = renderScreen();
    const group = threatGroup(1);
    expect(within(group).getByRole("combobox", { name: "ポケモン" })).toHaveValue("");
    expect(within(group).getByRole("combobox", { name: "特性" })).toBeInTheDocument();
    for (const slot of [1, 2, 3, 4]) {
      expect(within(group).getByRole("combobox", { name: `技${String(slot)}` })).toBeInTheDocument();
    }
    // 仮想敵の枠はメンバーの枠と別(メンバーは1つのまま)。
    expect(screen.getAllByRole("group", { name: /^メンバー\d$/ })).toHaveLength(1);
    expect(client.threatsCalls).toHaveLength(0);
    expect(client.recommendationsCalls).toHaveLength(0);
  });

  test("「仮想敵を追加」で6体まで枠が増え、6体では押せない(メンバーの枠は増えない)", async () => {
    const { user } = renderScreen();
    const add = screen.getByRole("button", { name: "仮想敵を追加" });
    for (let count = 2; count <= 6; count += 1) {
      await user.click(add);
      expect(threatGroup(count)).toBeInTheDocument();
    }
    expect(screen.getAllByRole("group", { name: /^仮想敵\d$/ })).toHaveLength(6);
    expect(screen.getAllByRole("group", { name: /^メンバー\d$/ })).toHaveLength(1);
    expect(add).toBeDisabled();
  });

  test("「仮想敵1を削除」で枠を消し、残りを詰めて送り直す", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const first = speciesAt(1);
    const second = speciesAt(2);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, first);
    await addThreat(user);
    await selectThreatSpecies(user, 2, second);
    expect(lastOf(client.threatsCalls, "threats").threats.map((threat) => threat.pokemonId)).toEqual([
      first.key,
      second.key,
    ]);

    await user.click(screen.getByRole("button", { name: "仮想敵1を削除" }));
    expect(screen.getAllByRole("group", { name: /^仮想敵\d$/ })).toHaveLength(1);
    expect(within(threatGroup(1)).getByRole("combobox", { name: "ポケモン" })).toHaveValue(second.key);
    expect(lastOf(client.threatsCalls, "threats").threats.map((threat) => threat.pokemonId)).toEqual([
      second.key,
    ]);
  });

  test("仮想敵1を空けたまま仮想敵2だけ選ぶと、結果の見出しも「仮想敵2」になる(入力欄の番号とずれない)", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const second = speciesAt(1);
    await selectSpecies(user, 1, mine);
    // 仮想敵1 は空のまま、追加した仮想敵2 だけにポケモンを選ぶ。
    await addThreat(user);
    await selectThreatSpecies(user, 2, second);
    await resolveThreats(client);

    // 送るのは仮想敵2だけ(仮想敵1は空なので含めない)。
    expect(lastOf(client.threatsCalls, "threats").threats.map((threat) => threat.pokemonId)).toEqual([
      second.key,
    ]);
    // 結果の見出しは、応答の並び順(0番目)ではなく、入力欄の番号(仮想敵2)に合わせる。
    expect(threatRegion(2, second.nameJa)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^仮想敵1/ })).not.toBeInTheDocument();
  });

  test("仮想敵の技の選択肢もその種族の覚える技(と「なし」)", async () => {
    const { user } = renderScreen();
    const { species } = speciesWithMoves();
    await selectThreatSpecies(user, 1, species);
    const select = within(threatGroup(1)).getByRole("combobox", { name: "技1" });
    expect(
      within(select)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["なし", ...learnsetMoves(species, master.moves).map((move) => move.nameJa)]);
  });
});

describe("threats(仮想敵の診断)", () => {
  test("パーティだけ・仮想敵だけでは呼ばず、両方そろったら呼ぶ", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    expect(client.threatsCalls).toHaveLength(0);

    await selectThreatSpecies(user, 1, speciesAt(1));
    expect(client.threatsCalls.length).toBeGreaterThan(0);

    // パーティのポケモンを外すと(仮想敵は残っていても)呼ばず、表も消す。
    await resolveThreats(client);
    const callsAfterFirst = client.threatsCalls.length;
    await user.selectOptions(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }), "");
    expect(client.threatsCalls).toHaveLength(callsAfterFirst);
    expect(screen.queryByRole("region", { name: /^仮想敵1/ })).not.toBeInTheDocument();
  });

  test("パーティと仮想敵を ID だけで送る(同じ枠の重複した技は1つ・特性も送る)", async () => {
    const { user, client } = renderScreen();
    const { species, damaging, status } = speciesWithMoves();
    const [moveA, moveB] = damaging;
    if (moveA === undefined || moveB === undefined) {
      throw new Error("攻撃技が2つ要る");
    }
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, moveA);
    await selectMove(user, 1, 2, moveA);
    await selectThreatSpecies(user, 1, species);
    await selectThreatMove(user, 1, 1, moveB);
    await selectThreatMove(user, 1, 2, status);

    const call = lastOf(client.threatsCalls, "threats");
    expect(call.members).toEqual([
      { pokemonId: species.key, moveIds: [moveA.id], abilityId: species.abilities[0] },
    ]);
    expect(call.threats).toEqual([
      { pokemonId: species.key, moveIds: [moveB.id, status.id], abilityId: species.abilities[0] },
    ]);
  });

  test("応答を待つ間は「仮想敵を計算中」を出し、届いたら仮想敵ごとの表を出す", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const enemy = speciesAt(1);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, enemy);
    expect(screen.getByText("仮想敵を計算中")).toBeInTheDocument();

    await resolveThreats(client, {
      matchup: () => ({ incoming: "2", outgoing: "1/2", safe: false, superEffective: false }),
    });

    expect(screen.queryByText("仮想敵を計算中")).not.toBeInTheDocument();
    const table = within(threatRegion(1, enemy.nameJa)).getByRole("table", { name: "相性" });
    expect(columnHeaders(table)).toEqual(["メンバー", "受ける倍率", "与える倍率", "安全", "抜群"]);
    expect(rowCells(table, mine.nameJa)).toEqual(["×2", "×1/2", "注意", "ふつう"]);
  });

  test("攻撃技が無い側の倍率(null)は「攻撃技なし」", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const enemy = speciesAt(1);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, enemy);
    await resolveThreats(client, {
      matchup: () => ({ incoming: null, outgoing: null, safe: false, superEffective: false }),
    });

    const table = within(threatRegion(1, enemy.nameJa)).getByRole("table", { name: "相性" });
    expect(rowCells(table, mine.nameJa)).toEqual(["攻撃技なし", "攻撃技なし", "注意", "ふつう"]);
  });

  test("安全・抜群の語は応答の真偽値だけで決める(倍率から判定し直さない)", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const first = speciesAt(1);
    const second = speciesAt(2);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, first);
    await addThreat(user);
    await selectThreatSpecies(user, 2, second);
    // 契約上ありえない組(×1/2 なのに safe=false、×2 なのに superEffective=false など)でも応答に従う。
    await resolveThreats(client, {
      matchup: (threatIndex) =>
        threatIndex === 0
          ? { incoming: "1/2", outgoing: "2", safe: false, superEffective: false }
          : { incoming: "2", outgoing: "1/2", safe: true, superEffective: true },
    });

    const firstTable = within(threatRegion(1, first.nameJa)).getByRole("table", { name: "相性" });
    expect(rowCells(firstTable, mine.nameJa)).toEqual(["×1/2", "×2", "注意", "ふつう"]);
    const secondTable = within(threatRegion(2, second.nameJa)).getByRole("table", { name: "相性" });
    expect(rowCells(secondTable, mine.nameJa)).toEqual(["×2", "×1/2", "安全", "抜群"]);
  });

  test("安全・抜群の人数は応答の値をそのまま出す(行から数え直さない)", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const enemy = speciesAt(1);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, enemy);
    await resolveThreats(client, {
      matchup: () => NEUTRAL_MATCHUP,
      counts: () => ({ safeMembers: 5, superEffectiveMembers: 4 }),
    });

    const region = threatRegion(1, enemy.nameJa);
    expect(within(region).getByText("安全に受けられる 5人")).toBeInTheDocument();
    expect(within(region).getByText("抜群を取れる 4人")).toBeInTheDocument();
  });

  test("メンバーが2人なら、仮想敵の表にも2行出す(応答の順)", async () => {
    const { user, client } = renderScreen();
    const first = speciesAt(0);
    const second = speciesAt(1);
    const enemy = speciesAt(2);
    await selectSpecies(user, 1, first);
    await user.click(screen.getByRole("button", { name: "メンバーを追加" }));
    await selectSpecies(user, 2, second);
    await selectThreatSpecies(user, 1, enemy);
    await resolveThreats(client, {
      matchup: (_threatIndex, memberIndex) =>
        memberIndex === 0
          ? { incoming: "4", outgoing: "0", safe: false, superEffective: false }
          : { incoming: "0", outgoing: "4", safe: true, superEffective: true },
    });

    const table = within(threatRegion(1, enemy.nameJa)).getByRole("table", { name: "相性" });
    expect(rowCells(table, first.nameJa)).toEqual(["×4", "×0", "注意", "ふつう"]);
    expect(rowCells(table, second.nameJa)).toEqual(["×0", "×4", "安全", "抜群"]);
  });

  test("仮想敵を選び直すと、新しい応答が届くまで前の表を残さず「仮想敵を計算中」に戻す", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const first = speciesAt(1);
    const second = speciesAt(2);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, first);
    await resolveThreats(client);
    expect(threatRegion(1, first.nameJa)).toBeInTheDocument();

    await selectThreatSpecies(user, 1, second);
    expect(screen.getByText("仮想敵を計算中")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^仮想敵1/ })).not.toBeInTheDocument();
  });

  test("パーティを選び直したときも、新しい応答が届くまで仮想敵の表を残さない", async () => {
    const { user, client } = renderScreen();
    const enemy = speciesAt(2);
    await selectSpecies(user, 1, speciesAt(0));
    await selectThreatSpecies(user, 1, enemy);
    await resolveThreats(client);
    expect(threatRegion(1, enemy.nameJa)).toBeInTheDocument();

    await selectSpecies(user, 1, speciesAt(1));
    expect(screen.getByText("仮想敵を計算中")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^仮想敵1/ })).not.toBeInTheDocument();
  });

  test("古い応答は無視する(後から届いた前の仮想敵の応答で表を上書きしない)", async () => {
    const { user, client } = renderScreen();
    const mine = speciesAt(0);
    const first = speciesAt(1);
    const second = speciesAt(2);
    await selectSpecies(user, 1, mine);
    await selectThreatSpecies(user, 1, first);
    const staleCall = lastOf(client.threatsCalls, "threats");
    await selectThreatSpecies(user, 1, second);
    const freshCall = lastOf(client.threatsCalls, "threats");
    expect(freshCall).not.toBe(staleCall);

    await act(async () => {
      freshCall.resolve({ ok: true, value: threatsResponse(freshCall) });
      await Promise.resolve();
    });
    await act(async () => {
      staleCall.resolve({ ok: true, value: threatsResponse(staleCall) });
      await Promise.resolve();
    });

    expect(threatRegion(1, second.nameJa)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: `仮想敵1(${first.nameJa})` })).not.toBeInTheDocument();
  });

  test("threats のエラーは role=alert で知らせ、防御相性の表は出したままにする", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    await selectThreatSpecies(user, 1, speciesAt(1));
    await resolveAnalyze(client);
    const call = lastOf(client.threatsCalls, "threats");
    await act(async () => {
      call.resolve({ ok: false, error: { code: "unknown_pokemon", message: "unknown pokemonId: 9999-000" } });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getAllByRole("alert").map((alert) => alert.textContent)).toContain(
        "unknown pokemonId: 9999-000",
      );
    });
    expect(screen.getByRole("table", { name: "防御相性" })).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^仮想敵1/ })).not.toBeInTheDocument();
    expect(screen.queryByText("仮想敵を計算中")).not.toBeInTheDocument();
  });
});

describe("recommendations(おすすめタイプ)", () => {
  test("パーティが1体そろったら、analyze と同じ内容を limit 無しで送る", async () => {
    const { user, client } = renderScreen();
    const { species, damaging } = speciesWithMoves();
    await selectSpecies(user, 1, species);
    await selectMove(user, 1, 1, damaging[0] ?? null);

    const call = lastOf(client.recommendationsCalls, "recommendations");
    expect(call.members).toEqual([
      { pokemonId: species.key, moveIds: [damaging[0]?.id], abilityId: species.abilities[0] },
    ]);
    // 既定の件数(10)はサーバーが決める(ADR-0303 §7: limit は省略)。
    expect(call.limit).toBeUndefined();
  });

  test("パーティにポケモンがいなければ呼ばず、仮想敵の入力では呼び直さない", async () => {
    const { user, client } = renderScreen();
    await selectThreatSpecies(user, 1, speciesAt(1));
    expect(client.recommendationsCalls).toHaveLength(0);

    await selectSpecies(user, 1, speciesAt(0));
    const callsAfterParty = client.recommendationsCalls.length;
    expect(callsAfterParty).toBeGreaterThan(0);

    // 仮想敵は recommendations の入力ではない(ADR-0303 §7)。
    await selectThreatSpecies(user, 1, speciesAt(2));
    expect(client.recommendationsCalls).toHaveLength(callsAfterParty);
  });

  test("応答を待つ間は「おすすめタイプを計算中」を出し、届いたら防御の穴・攻撃範囲の穴を出す(無ければ「なし」)", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    expect(screen.getByText("おすすめタイプを計算中")).toBeInTheDocument();

    await resolveRecommendations(client, { defenseHoles: ["fire", "water"], offenseHoles: [] });

    expect(screen.queryByText("おすすめタイプを計算中")).not.toBeInTheDocument();
    const region = recommendationsRegion();
    expect(within(region).getByText(`防御の穴: ${typeNameJa.fire}・${typeNameJa.water}`)).toBeInTheDocument();
    expect(within(region).getByText("攻撃範囲の穴: なし")).toBeInTheDocument();
  });

  test("候補の表: タイプ・ふさぐ穴・該当ポケモンを応答の順で出す(名前が無ければ ID)", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    const candidates: Schemas["TypeCandidate"][] = [
      {
        types: ["water", "steel"],
        defenseCovered: ["fire", "grass"],
        offenseCovered: ["dragon"],
        weaknesses: 3,
        pokemon: [
          { pokemonId: "9101-000", nameJa: "テストみずはがね", types: ["water", "steel"], exactMatch: true },
          { pokemonId: "9102-000", types: ["water", "steel"], exactMatch: false },
        ],
      },
      {
        types: ["ghost"],
        defenseCovered: [],
        offenseCovered: ["ghost"],
        weaknesses: 2,
        pokemon: [],
      },
    ];
    await resolveRecommendations(client, { candidates });

    const table = within(recommendationsRegion()).getByRole("table", { name: "おすすめタイプの候補" });
    expect(columnHeaders(table)).toEqual(["タイプ", "ふさぐ防御の穴", "ふさぐ攻撃範囲の穴", "ポケモン"]);
    expect(rowCells(table, `${typeNameJa.water}・${typeNameJa.steel}`)).toEqual([
      `${typeNameJa.fire}・${typeNameJa.grass}`,
      typeNameJa.dragon,
      "テストみずはがね・9102-000",
    ]);
    expect(rowCells(table, typeNameJa.ghost)).toEqual(["なし", typeNameJa.ghost, "なし"]);
  });

  test("特性で補える表: 防御の穴ごとに ポケモン・特性・倍率 を出す(いなければ「なし」)", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    const abilityId = "example-ability-none";
    const abilityOptions: Schemas["AbilityOption"][] = [
      {
        attackType: "fire",
        pokemon: [{ pokemonId: "9103-000", nameJa: "テストほのおけし", abilityId, multiplier: "1/2" }],
      },
      { attackType: "water", pokemon: [] },
    ];
    await resolveRecommendations(client, { defenseHoles: ["fire", "water"], abilityOptions });

    const table = within(recommendationsRegion()).getByRole("table", { name: "特性で補えるポケモン" });
    expect(columnHeaders(table)).toEqual(["攻撃タイプ", "ポケモン"]);
    // 倍率には category が無いので語(弱点・耐性)を付けない(multiplierLabel)。
    expect(rowCells(table, typeNameJa.fire)).toEqual([`テストほのおけし(${abilityName(abilityId)} ×1/2)`]);
    expect(rowCells(table, typeNameJa.water)).toEqual(["なし"]);
  });

  test("選択を変えると、新しい応答が届くまで前の結果を残さず「おすすめタイプを計算中」に戻す", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    await resolveRecommendations(client, { defenseHoles: ["fire"] });
    expect(recommendationsRegion()).toBeInTheDocument();

    await selectSpecies(user, 1, speciesAt(1));
    expect(screen.getByText("おすすめタイプを計算中")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "おすすめタイプ" })).not.toBeInTheDocument();
  });

  test("古い応答は無視する(後から届いた前の選択の応答で結果を上書きしない)", async () => {
    const { user, client } = renderScreen();
    await selectSpecies(user, 1, speciesAt(0));
    const staleCall = lastOf(client.recommendationsCalls, "recommendations");
    await selectSpecies(user, 1, speciesAt(1));
    const freshCall = lastOf(client.recommendationsCalls, "recommendations");
    expect(freshCall).not.toBe(staleCall);

    await act(async () => {
      freshCall.resolve({ ok: true, value: recommendationsResponse({ defenseHoles: ["water"] }) });
      await Promise.resolve();
    });
    await act(async () => {
      staleCall.resolve({ ok: true, value: recommendationsResponse({ defenseHoles: ["fire"] }) });
      await Promise.resolve();
    });

    const region = recommendationsRegion();
    expect(within(region).getByText(`防御の穴: ${typeNameJa.water}`)).toBeInTheDocument();
    expect(within(region).queryByText(`防御の穴: ${typeNameJa.fire}`)).not.toBeInTheDocument();
  });

  test("recommendations のエラーは role=alert で知らせ、仮想敵の表は出したままにする", async () => {
    const { user, client } = renderScreen();
    const enemy = speciesAt(1);
    await selectSpecies(user, 1, speciesAt(0));
    await selectThreatSpecies(user, 1, enemy);
    await resolveThreats(client);
    const call = lastOf(client.recommendationsCalls, "recommendations");
    await act(async () => {
      call.resolve({
        ok: false,
        error: { code: "balance_unavailable", message: "タイプバランスの API に接続できません" },
      });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getAllByRole("alert").map((alert) => alert.textContent)).toContain(
        "タイプバランスの API に接続できません",
      );
    });
    expect(threatRegion(1, enemy.nameJa)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "おすすめタイプ" })).not.toBeInTheDocument();
    expect(screen.queryByText("おすすめタイプを計算中")).not.toBeInTheDocument();
  });
});
