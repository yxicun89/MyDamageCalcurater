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

type Schemas = components["schemas"];
type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];

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

interface FakeBalanceClient extends BalanceClient {
  readonly analyzeCalls: PendingCall<AnalyzeMembers, Schemas["AnalyzeResponse"]>[];
  readonly coverageCalls: PendingCall<CoverageMembers, Schemas["CoverageResponse"]>[];
}

function createFakeBalanceClient(): FakeBalanceClient {
  const analyzeCalls: PendingCall<AnalyzeMembers, Schemas["AnalyzeResponse"]>[] = [];
  const coverageCalls: PendingCall<CoverageMembers, Schemas["CoverageResponse"]>[] = [];
  return {
    analyzeCalls,
    coverageCalls,
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
