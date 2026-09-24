// P4-16b/P4-17b: タイプバランスの画面を、機能が欠けたマスタ(オンライン相当)でどう見せるか
// (ADR-0304 A-9 → A-14 で置き換え)。
//
// P4-16b(A-9)は「speciesList と moves が両方そろわないマスタでは画面ごと使えない」と決めたが、
// A-13.1 で `capabilities.moves` が永続的に false と決まったため、オンラインではこの画面が永久に使えなかった。
// P4-17b(A-14.1)は可否の条件を「一覧がそろっているか」から**「入力の口があるか」**に置き換える:
//
//   speciesInputAvailable = capabilities.speciesList || masterSearch !== undefined
//   moveInputAvailable    = capabilities.moves || (!capabilities.speciesList && masterSearch !== undefined)
//   balanceAvailable      = speciesInputAvailable && moveInputAvailable
//
// 確かめること:
//   - 上の式の境界(A-14.1 の表)。とくに「検索口が無い組み合わせ」は今までどおり使えないままであること
//   - 検索口があるオンラインのマスタでは画面が使え、12枠(メンバー6・仮想敵6)が独立に検索・解決できること
//   - 解決した種族の特性・技(learnset)が各枠の欄に出ること(A-14.4)
//   - coverage を呼ぶかどうかの判定が、実体の分からない技 ID を攻撃技と見なさないこと(A-14.3)
//   - A-9 の元の懸念(技が無いまま誤解を招く診断を出す)が、新しい条件でも再発しないこと
//   - capabilities を省いた既存のマスタ(オフライン相当)では今までどおり

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import type { components } from "../api/balance.gen";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { learnsetMoves } from "../domain/moves";
import type { Move } from "../engine/types";
import { balanceScreenText, masterOnlineText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { ONLINE_MASTER_CAPABILITIES, SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type {
  MasterCapabilities,
  MasterData,
  MasterSpecies,
  MasterSpeciesSearch,
} from "../master/types";
import { createFakeSpeciesSearch, limitedMaster, type FakeSpeciesSearch } from "../test/onlineMaster";
import { BalanceScreen } from "./BalanceScreen";

type Schemas = components["schemas"];
type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];
type ThreatsPokemon = Schemas["ThreatsRequestPokemon"];

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

afterEach(() => {
  vi.useRealTimers();
});

const NO_SPECIES_LIST: MasterCapabilities = { speciesList: false, moves: true, effects: true };
const NO_MOVES: MasterCapabilities = { speciesList: true, moves: false, effects: true };
const NO_EFFECTS: MasterCapabilities = { speciesList: true, moves: true, effects: false };

function speciesAt(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

/** 攻撃技を1つ以上覚える種族と、その技(coverage は攻撃技を選んだメンバーがいないと呼ばれない)。 */
function speciesWithDamagingMove(): { species: MasterSpecies; move: Move } {
  for (const species of example.species) {
    const move = learnsetMoves(species, example.moves).find((candidate) => candidate.category !== "status");
    if (move !== undefined) {
      return { species, move };
    }
  }
  throw new Error("例データに攻撃技を覚える種族が無い");
}

/** 変化技と攻撃技を両方覚える種族(A-14.3 の moveById の判定を確かめるのに要る)。 */
function speciesWithStatusAndDamagingMove(): { species: MasterSpecies; status: Move; damaging: Move } {
  for (const species of example.species) {
    const moves = learnsetMoves(species, example.moves);
    const status = moves.find((move) => move.category === "status");
    const damaging = moves.find((move) => move.category !== "status");
    if (status !== undefined && damaging !== undefined) {
      return { species, status, damaging };
    }
  }
  throw new Error("例データに変化技と攻撃技を両方覚える種族が無い");
}

/** 例データの種族・特性・技をそのまま解決する検索口の fake(オンラインで渡される口の代わり)。 */
function onlineSearch(): FakeSpeciesSearch {
  return createFakeSpeciesSearch({
    species: example.species,
    abilities: example.abilities,
    // P4-17(A-13.3): resolveSpecies は種族・特性と一緒に learnset の技も返す。
    moves: example.moves,
  });
}

// ---- fake の BalanceClient(呼び出しの有無と、送った内容を記録する) ----

interface ThreatsCallArgs {
  readonly members: readonly ThreatsPokemon[];
  readonly threats: readonly ThreatsPokemon[];
}

/** 呼び出しを記録する fake。analyzeResult を渡さない限り応答は返さない(画面は計算中のまま)。 */
interface CountingBalanceClient extends BalanceClient {
  readonly calls: string[];
  readonly analyzeCalls: AnalyzeMembers[];
  readonly coverageCalls: CoverageMembers[];
  readonly threatsCalls: ThreatsCallArgs[];
}

interface CountingClientOptions {
  /** analyze だけ応答を返す(結果表の名前解決を確かめるテスト用)。 */
  readonly analyzeResult?: Schemas["AnalyzeResponse"];
}

function countingClient(options: CountingClientOptions = {}): CountingBalanceClient {
  const calls: string[] = [];
  const analyzeCalls: AnalyzeMembers[] = [];
  const coverageCalls: CoverageMembers[] = [];
  const threatsCalls: ThreatsCallArgs[] = [];
  const never = <T,>(name: string): Promise<T> => {
    calls.push(name);
    return new Promise<T>(() => undefined);
  };
  return {
    calls,
    analyzeCalls,
    coverageCalls,
    threatsCalls,
    analyze(members) {
      analyzeCalls.push(structuredClone(members) as AnalyzeMembers);
      if (options.analyzeResult === undefined) {
        return never("analyze");
      }
      calls.push("analyze");
      const result: BalanceResult<Schemas["AnalyzeResponse"]> = {
        ok: true,
        value: options.analyzeResult,
      };
      return Promise.resolve(result);
    },
    coverage(members) {
      coverageCalls.push(structuredClone(members) as CoverageMembers);
      return never("coverage");
    },
    threats(members, threats) {
      threatsCalls.push({ members: structuredClone(members), threats: structuredClone(threats) });
      return never("threats");
    },
    recommendations: () => never("recommendations"),
  };
}

interface RenderResult {
  readonly user: UserEvent;
  readonly client: CountingBalanceClient;
  readonly rerenderMaster: (next: MasterData) => void;
  /** 検索のデバウンスを進める(検索口を渡したときだけ意味がある)。 */
  readonly advance: (ms: number) => void;
}

/**
 * 検索口を渡すときだけ fake タイマーにする(検索のデバウンスを進めるため)。
 * ドロップダウンだけのテスト(オフライン相当の回帰)は実タイマーのまま = 今までと同じ条件で走らせる。
 */
function renderScreen(
  master: MasterData,
  search?: MasterSpeciesSearch,
  options: CountingClientOptions = {},
): RenderResult {
  const fakeTimers = search !== undefined;
  if (fakeTimers) {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  }
  const user = fakeTimers
    ? userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) })
    : userEvent.setup();
  const client = countingClient(options);
  const rendered = render(<BalanceScreen master={master} client={client} masterSearch={search} />);
  return {
    user,
    client,
    rerenderMaster: (next) => {
      rendered.rerender(<BalanceScreen master={next} client={client} masterSearch={search} />);
    },
    advance: (ms) => {
      act(() => {
        vi.advanceTimersByTime(ms);
      });
    },
  };
}

const memberGroup = (n: number) => screen.getByRole("group", { name: `メンバー${String(n)}` });
const threatGroup = (n: number) => screen.getByRole("group", { name: `仮想敵${String(n)}` });
const speciesField = (group: HTMLElement) =>
  within(group).getByRole("combobox", { name: balanceScreenText.speciesLabel });
const moveField = (group: HTMLElement, slot: number) =>
  within(group).getByRole("combobox", { name: balanceScreenText.moveLabel(slot) });

/** 枠の検索欄に名前を入れ、デバウンスを越えてから候補を選ぶ(A-10 の検索欄の形)。 */
async function chooseBySearch(
  rendered: RenderResult,
  group: HTMLElement,
  species: MasterSpecies,
): Promise<void> {
  await rendered.user.type(speciesField(group), species.nameJa);
  rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
  const option = await within(group).findByRole("option", { name: species.nameJa });
  await rendered.user.click(option);
}

/** effect が動く猶予だけ置く(応答を待つ画面ではないので waitFor は使わない)。 */
async function flushEffects(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
  });
}

// ---- A-14.1: 画面の可否の境界 ----

type SearchFactory = () => MasterSpeciesSearch | undefined;
const noSearch: SearchFactory = () => undefined;
const withSearch: SearchFactory = () => onlineSearch();

// P4-17b(ADR-0304 A-14.1)で**意図的に条件を絞った**節。P4-16b の時点では
// 「speciesList か moves が false」だけで使えないと判定していたが、それだと A-13.1 で moves が
// 永続的に false と決まったオンラインで画面が永久に使えない。ここに残すのは
// 「ポケモンか技を選ぶ口がそもそも無い」組み合わせ(= A-9 の懸念がそのまま残る組み合わせ)だけにし、
// 口がそろう組み合わせは下の「検索口があれば使える」節で新たに固定する。
// 既存の3ケースはどれも検索口を渡していなかったので、期待する見え方は1つも弱めていない。
describe.each<[string, MasterCapabilities, SearchFactory]>([
  ["ポケモンの一覧が無く、検索口も渡されていない(speciesList: false)", NO_SPECIES_LIST, noSearch],
  ["技の一覧が無く、検索口も渡されていない(moves: false)", NO_MOVES, noSearch],
  ["オンラインのマスタで、検索口が渡されていない(両方とも無い)", ONLINE_MASTER_CAPABILITIES, noSearch],
  // A-14.1 の表の最終行: 種族はドロップダウンで選ぶので resolveSpecies が走らず、技が永久に届かない。
  ["技の一覧が無く、種族はドロップダウンで選ぶ(検索口はあるが使われない)", NO_MOVES, withSearch],
])("%s とき、画面ごと使えないことを案内する(ADR-0304 A-14.1)", (_name, capabilities, search) => {
  test("使えない旨の案内を出す", () => {
    renderScreen(limitedMaster(example, capabilities), search());
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
  });

  test("パーティ・仮想敵の入力は残すが、全部 disabled にする", () => {
    renderScreen(limitedMaster(example, capabilities), search());
    for (const group of [memberGroup(1), threatGroup(1)]) {
      expect(speciesField(group)).toBeDisabled();
      expect(within(group).getByRole("combobox", { name: balanceScreenText.abilityLabel })).toBeDisabled();
      for (const slot of [1, 2, 3, 4]) {
        expect(moveField(group, slot)).toBeDisabled();
      }
    }
    expect(screen.getByRole("button", { name: balanceScreenText.addMemberLabel })).toBeDisabled();
    expect(screen.getByRole("button", { name: balanceScreenText.addThreatLabel })).toBeDisabled();
  });

  test("balance API を1本も呼ばない(誤解を招く診断を出さない)", async () => {
    const { client } = renderScreen(limitedMaster(example, capabilities), search());
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await flushEffects();
    expect(client.calls).toEqual([]);
  });
});

describe("持ち物・特性の効果データだけが無いマスタ(effects: false)", () => {
  test("balance API は ID しか送らないので、今までどおり動く", async () => {
    const master = limitedMaster(example, NO_EFFECTS);
    const { user, client } = renderScreen(master);
    expect(screen.queryByText(masterOnlineText.balanceUnavailable)).toBeNull();

    const select = speciesField(memberGroup(1));
    expect(select).not.toBeDisabled();
    await user.selectOptions(select, speciesAt(0).key);

    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
  });
});

describe("capabilities を省いたマスタ(オフライン相当)は今までどおり", () => {
  test("案内を出さず、メンバーを選べば analyze を呼ぶ", async () => {
    const { user, client } = renderScreen(example);
    expect(screen.queryByText(masterOnlineText.balanceUnavailable)).toBeNull();

    await user.selectOptions(speciesField(memberGroup(1)), speciesAt(0).key);

    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
  });

  test("種族を選んでいなくても技の欄は有効のまま(capabilities.moves が true。A-14.4 の回帰)", () => {
    renderScreen(example);
    for (const slot of [1, 2, 3, 4]) {
      expect(moveField(memberGroup(1), slot)).not.toBeDisabled();
    }
  });

  test("技の案内(movesUnavailable)はこの画面では出さない(A-14.4)", () => {
    renderScreen(example);
    expect(screen.queryByText(masterOnlineText.movesUnavailable)).toBeNull();
  });
});

describe("A-14.1 のガードが実際に効いていること(P4-16b からの回帰ガード)", () => {
  test("メンバーを選んだ状態で、検索口の無いオンラインの capabilities に切り替わると analyze を呼び直さない", async () => {
    // 先に使えるマスタで選ばせ、analyze が呼ばれることを確認する(入力が空だから呼ばれない、という
    // 見せかけの緑を避けるため)。member.speciesKey は画面のローカル state なので、master が変わっても
    // 選択は残る = balanceAvailable のガード以外に呼び出しを止めるものが無い状態を作れる。
    const { user, client, rerenderMaster } = renderScreen(example);
    await user.selectOptions(speciesField(memberGroup(1)), speciesAt(0).key);
    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
    const callsBeforeSwitch = client.calls.length;

    rerenderMaster(limitedMaster(example, ONLINE_MASTER_CAPABILITIES));

    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await flushEffects();
    expect(client.calls.length).toBe(callsBeforeSwitch);
  });

  test("技・仮想敵まで選んだ状態で切り替わっても、4つの診断すべてを呼び直さない(P4-16c(6))", async () => {
    // 上のテストはポケモンしか選ばないので、coverage(攻撃技が要る)・threats(仮想敵が要る)は
    // ガードが無くても呼ばれない = ガード削除を検知できない。ここでは4つとも実際に呼ばれた状態を作り、
    // capabilities が切り替わったあとに1本も増えないことを確かめる(critic 指摘の回帰ガード)。
    const { user, client, rerenderMaster } = renderScreen(example);
    const { species, move } = speciesWithDamagingMove();

    await user.selectOptions(speciesField(memberGroup(1)), species.key);
    await user.selectOptions(moveField(memberGroup(1), 1), move.id);
    await user.selectOptions(speciesField(threatGroup(1)), speciesAt(1).key);

    await waitFor(() => {
      expect(client.calls).toEqual(
        expect.arrayContaining(["analyze", "coverage", "threats", "recommendations"]),
      );
    });
    const callsBeforeSwitch = [...client.calls];

    rerenderMaster(limitedMaster(example, ONLINE_MASTER_CAPABILITIES));

    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await flushEffects();
    expect(client.calls).toEqual(callsBeforeSwitch);
  });
});

describe("capabilities.moves の意味は変えない(ADR-0304 A-13.1 の回帰ガード)", () => {
  // P4-17b でこの画面はオンラインでも使えるようになるが、それは「種族を選ぶと、その種族の learnset が
  // 解決される」経路を各枠に広げたからであって、技の**全件一覧**は今も取れない。
  test("ONLINE_MASTER_CAPABILITIES.moves は false のまま(全件一覧は取れない)", () => {
    expect(ONLINE_MASTER_CAPABILITIES.moves).toBe(false);
  });

  test("オンラインのマスタの moves は空のまま(技は種族の解決と一緒にしか届かない)", () => {
    expect(limitedMaster(example, ONLINE_MASTER_CAPABILITIES).moves).toEqual([]);
  });
});

// ---- P4-17b: 検索口があればオンラインでも使える(A-14.1・A-14.2・A-14.4) ----

describe("検索口があるオンラインのマスタ(P4-17b、ADR-0304 A-14.1)", () => {
  test("使えない案内を出さず、メンバー・仮想敵の追加ボタンも押せる", () => {
    renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());

    expect(screen.queryByText(masterOnlineText.balanceUnavailable)).toBeNull();
    expect(screen.getByRole("button", { name: balanceScreenText.addMemberLabel })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: balanceScreenText.addThreatLabel })).not.toBeDisabled();
  });

  test("ポケモンの欄はドロップダウンではなく検索欄になる(accessible name は今までのまま。A-10)", () => {
    renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());

    for (const group of [memberGroup(1), threatGroup(1)]) {
      const field = speciesField(group);
      expect(field).not.toBeDisabled();
      expect(field).toHaveAttribute("placeholder", masterOnlineText.speciesSearchLabel);
    }
  });

  test("検索で種族を選ぶと analyze を呼ぶ(送るのは種族 key と既定の特性)", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const species = speciesAt(0);

    await chooseBySearch(rendered, memberGroup(1), species);

    await waitFor(() => {
      expect(rendered.client.analyzeCalls.at(-1)).toEqual([
        { pokemonId: species.key, abilityId: species.abilities[0] },
      ]);
    });
  });

  test("解決した種族の特性が、特性の欄に名前で出る(master.abilities が空でも)", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const species = example.species.find((candidate) => candidate.abilities.length >= 2) ?? speciesAt(0);

    await chooseBySearch(rendered, memberGroup(1), species);

    const abilitySelect = within(memberGroup(1)).getByRole("combobox", {
      name: balanceScreenText.abilityLabel,
    });
    await waitFor(() => {
      for (const abilityId of species.abilities) {
        const ability = example.abilities.find((candidate) => candidate.id === abilityId);
        expect(ability).toBeDefined();
        expect(within(abilitySelect).getByRole("option", { name: ability?.nameJa })).toHaveValue(abilityId);
      }
    });
  });

  test("解決した種族の learnset が、技の欄の選択肢に出る(A-14.4)", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const species = speciesAt(0);
    const moves = learnsetMoves(species, example.moves);
    expect(moves.length).toBeGreaterThan(0);

    await chooseBySearch(rendered, memberGroup(1), species);

    await waitFor(() => {
      for (const move of moves) {
        expect(within(moveField(memberGroup(1), 1)).getByRole("option", { name: move.nameJa })).toHaveValue(
          move.id,
        );
      }
    });
  });

  test("技の欄は、その枠の種族が解決するまで disabled(A-13.2 をこの画面に当てはめる)", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    for (const slot of [1, 2, 3, 4]) {
      expect(moveField(memberGroup(1), slot)).toBeDisabled();
    }

    await chooseBySearch(rendered, memberGroup(1), speciesAt(0));

    await waitFor(() => {
      expect(moveField(memberGroup(1), 1)).not.toBeDisabled();
    });
    // 他の枠は独立: メンバー1を解決しても仮想敵1の技はまだ選べない。
    expect(moveField(threatGroup(1), 1)).toBeDisabled();
  });

  test("同じ案内を枠の数だけ並べない(movesUnavailable はこの画面では出さない。A-14.4)", () => {
    renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    expect(screen.queryAllByText(masterOnlineText.movesUnavailable)).toHaveLength(0);
  });
});

describe("12枠(メンバー6・仮想敵6)が独立に検索・解決できる(ADR-0304 A-14.2)", () => {
  test("メンバー2体を別々に検索して選ぶと、2体とも枠の順で analyze に載る", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const first = speciesAt(0);
    const second = speciesAt(1);

    await chooseBySearch(rendered, memberGroup(1), first);
    await rendered.user.click(screen.getByRole("button", { name: balanceScreenText.addMemberLabel }));
    await chooseBySearch(rendered, memberGroup(2), second);

    await waitFor(() => {
      expect(rendered.client.analyzeCalls.at(-1)).toEqual([
        { pokemonId: first.key, abilityId: first.abilities[0] },
        { pokemonId: second.key, abilityId: second.abilities[0] },
      ]);
    });
    // 後から解決した種族が、先に解決した枠の技の選択肢を上書きしていないこと。
    for (const [group, species] of [
      [memberGroup(1), first],
      [memberGroup(2), second],
    ] as const) {
      const options = within(moveField(group, 1)).getAllByRole("option");
      const values = options.map((option) => (option as HTMLOptionElement).value).filter((v) => v !== "");
      expect(values).toEqual(learnsetMoves(species, example.moves).map((move) => move.id));
    }
  });

  test("仮想敵の枠もメンバーとは独立に検索して解決でき、threats に両方が載る", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const mine = speciesAt(0);
    const theirs = speciesAt(2);

    await chooseBySearch(rendered, memberGroup(1), mine);
    await chooseBySearch(rendered, threatGroup(1), theirs);

    await waitFor(() => {
      expect(rendered.client.threatsCalls.at(-1)).toEqual({
        members: [{ pokemonId: mine.key, moveIds: [], abilityId: mine.abilities[0] }],
        threats: [{ pokemonId: theirs.key, moveIds: [], abilityId: theirs.abilities[0] }],
      });
    });
  });

  test("同じ種族を2枠で選んでも、両方の枠で技を選べる(覚え書きは種族 key で引く)", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const species = speciesWithDamagingMove().species;

    await chooseBySearch(rendered, memberGroup(1), species);
    await rendered.user.click(screen.getByRole("button", { name: balanceScreenText.addMemberLabel }));
    await chooseBySearch(rendered, memberGroup(2), species);

    await waitFor(() => {
      expect(moveField(memberGroup(2), 1)).not.toBeDisabled();
    });
    expect(moveField(memberGroup(1), 1)).not.toBeDisabled();
  });

  test("結果表の行見出しは、解決した種族の日本語名になる(key のままにしない。A-14.2)", async () => {
    const species = speciesAt(0);
    const analyzeResult: Schemas["AnalyzeResponse"] = {
      members: [{ pokemonId: species.key, types: ["normal"], defense: [] }],
      teamSummary: [],
    };
    const rendered = renderScreen(
      limitedMaster(example, ONLINE_MASTER_CAPABILITIES),
      onlineSearch(),
      { analyzeResult },
    );

    await chooseBySearch(rendered, memberGroup(1), species);

    const table = await screen.findByRole("table", { name: balanceScreenText.defenseTableLabel });
    expect(within(table).getByRole("rowheader", { name: species.nameJa })).toBeInTheDocument();
  });
});

describe("coverage を呼ぶかどうかの判定(ADR-0304 A-14.3 の moveById)", () => {
  test("変化技だけを選んだメンバーでは coverage を呼ばない(オンラインでも)", async () => {
    // 退行の再現: 判定が master.moves(オンラインでは空)から分類を引いていると、
    // moveById.get(id) が undefined になり `undefined !== "status"` が true =「攻撃技あり」と
    // 誤判定して coverage を呼んでしまう。実体が分からない ID は攻撃技と見なさない(fail-closed)。
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const { species, status } = speciesWithStatusAndDamagingMove();

    await chooseBySearch(rendered, memberGroup(1), species);
    await waitFor(() => {
      expect(moveField(memberGroup(1), 1)).not.toBeDisabled();
    });
    await rendered.user.selectOptions(moveField(memberGroup(1), 1), status.id);

    await waitFor(() => {
      expect(rendered.client.calls).toContain("analyze");
    });
    await flushEffects();
    expect(rendered.client.calls).not.toContain("coverage");
  });

  test("同じ枠に攻撃技を足すと coverage を呼び、選んだ技を枠の順で送る", async () => {
    const rendered = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    const { species, status, damaging } = speciesWithStatusAndDamagingMove();

    await chooseBySearch(rendered, memberGroup(1), species);
    await waitFor(() => {
      expect(moveField(memberGroup(1), 1)).not.toBeDisabled();
    });
    await rendered.user.selectOptions(moveField(memberGroup(1), 1), status.id);
    await rendered.user.selectOptions(moveField(memberGroup(1), 2), damaging.id);

    await waitFor(() => {
      expect(rendered.client.coverageCalls.at(-1)).toEqual([
        { pokemonId: species.key, moveIds: [status.id, damaging.id] },
      ]);
    });
  });

  test("オフライン相当のマスタでも、変化技だけなら coverage を呼ばない(今までどおり)", async () => {
    const { user, client } = renderScreen(example);
    const { species, status } = speciesWithStatusAndDamagingMove();

    await user.selectOptions(speciesField(memberGroup(1)), species.key);
    await user.selectOptions(moveField(memberGroup(1), 1), status.id);

    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
    await flushEffects();
    expect(client.calls).not.toContain("coverage");
  });
});

describe("A-9 の元の懸念が新しい条件でも再発しないこと(ADR-0304 A-14.1)", () => {
  test("技をどこからも取れないマスタ(moves: false・検索口なし)は、今も画面ごと止める", async () => {
    // A-9 が避けたかった「技が永久に空のまま threats / recommendations を呼ぶ」状態は、
    // 技の入る口が1つも無いこの組み合わせでだけ起こる。ここは P4-17b でも変えない。
    const { client } = renderScreen(limitedMaster(example, NO_MOVES));
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await flushEffects();
    expect(client.calls).toEqual([]);
  });

  test("検索口があるオンラインで種族だけ選んだ状態は、オフラインで種族だけ選んだ状態と同じ呼び出しになる", async () => {
    // 新しい条件の根拠: オンラインの「まだ技を選んでいないメンバー」は、オフラインの同じ状態に帰着する
    // (どちらも技を選べる口はあり、選んでいないだけ)。呼び出しの条件は ADR-0400 §1・ADR-0303 §7 のまま。
    const online = renderScreen(limitedMaster(example, ONLINE_MASTER_CAPABILITIES), onlineSearch());
    await chooseBySearch(online, memberGroup(1), speciesAt(0));
    await chooseBySearch(online, threatGroup(1), speciesAt(1));
    await waitFor(() => {
      expect(online.client.calls).toEqual(
        expect.arrayContaining(["analyze", "threats", "recommendations"]),
      );
    });
    // 呼ばれた API の顔ぶれ(重複を除く)が、オフラインで種族だけ選んだときと同じであること。
    // オフライン側の顔ぶれは BalanceScreen.test.tsx が固定しているので、ここでは値で突き合わせる。
    expect([...new Set(online.client.calls)].sort()).toEqual(["analyze", "recommendations", "threats"]);
  });
});
