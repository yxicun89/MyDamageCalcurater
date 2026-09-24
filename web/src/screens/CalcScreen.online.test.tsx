// P4-16b: 計算画面の、機能が欠けたマスタ(オンライン相当)での見え方(ADR-0304 A-5・A-10)。
// 画面は「オンラインかどうか」ではなく capabilities の各項目で分岐する(A-2)ので、項目ごとに確かめる。
// engine は fake、マスタは架空の例データから項目を落としたもの(test/onlineMaster.ts)。
// 確かめること:
//   - 技が無い(moves: false): 技のセレクトは残すが disabled・選択肢は空・案内を出す・計算しない
//   - 効果データが無い(effects: false): 「持ち物の候補も比較」を disabled にして案内・持ち物の選択は残す
//   - 種族の一覧が無い(speciesList: false): ドロップダウンの代わりに検索欄(A-4 のパラメータ・A-10 の形)
//   - capabilities を省いた既存のマスタでは、今までどおり(この画面の既存テストと同じ見え方)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { masterOnlineText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS, SPECIES_SEARCH_LIMIT } from "../master/onlineSource";
import type { MasterCapabilities, MasterData, MasterSpecies, MasterSpeciesSearch } from "../master/types";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import {
  createDeferredSpeciesSearch,
  createFakeSpeciesSearch,
  limitedMaster,
  type FakeSpeciesSearch,
} from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

/** 種族だけ検索で引くマスタ(P4-17 の形)。技・効果データはある。 */
const SEARCH_ONLY: MasterCapabilities = { speciesList: false, moves: true, effects: true };
/** 技だけ使えないマスタ(ADR-0304 §3 の欠落そのもの)。 */
const NO_MOVES: MasterCapabilities = { speciesList: true, moves: false, effects: true };
/** 持ち物・特性の効果データだけ無いマスタ(ADR-0304 A-1)。 */
const NO_EFFECTS: MasterCapabilities = { speciesList: true, moves: true, effects: false };

function speciesAt(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const compareToggle = () => screen.getByRole("checkbox", { name: /持ち物の候補/ });

interface RenderResult {
  readonly user: UserEvent;
  readonly engine: FakeEngine;
  /** fake タイマーを進める(検索のデバウンス)。 */
  advance(ms: number): void;
}

function renderScreen(master: MasterData, search?: MasterSpeciesSearch): RenderResult {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const engine = createFakeEngine();
  render(<CalcScreen engine={engine} master={master} masterSearch={search} />);
  return {
    user,
    engine,
    advance(ms) {
      act(() => {
        vi.advanceTimersByTime(ms);
      });
    },
  };
}

/** 例データの種族・特性をそのまま引ける検索口の fake。 */
function fakeSearch(limit?: number): FakeSpeciesSearch {
  return createFakeSpeciesSearch({
    species: example.species,
    abilities: example.abilities,
    ...(limit === undefined ? {} : { limit }),
  });
}

/**
 * 名前が共通の接頭辞("テスト")で始まる架空の種族を SPECIES_SEARCH_LIMIT 件以上作る
 * (createFakeSpeciesSearch は limit オプションを渡さない限り母集団の件数をそのまま返すので、
 * 実際に SPECIES_SEARCH_LIMIT に「達した」状態を作るには母集団自体をこの件数以上にする必要がある。
 * 元は limit: 1 でごまかしていたが、それだと SPECIES_SEARCH_LIMIT の実値と無関係に常に
 * speciesSearchTruncated が出る実装(重大な文言バグ)を見逃す。critic 指摘で修正)。
 */
function manySpecies(count: number): readonly MasterSpecies[] {
  const base = speciesAt(0);
  return Array.from({ length: count }, (_, index) => ({
    ...base,
    key: `${base.key}-many-${index}`,
    nameJa: `テスト種族${index}`,
  }));
}

/** 検索欄に名前を入れ、デバウンスを越えてから候補を選ぶ。 */
async function chooseBySearch(
  rendered: RenderResult,
  card: HTMLElement,
  label: string,
  species: MasterSpecies,
): Promise<void> {
  const input = within(card).getByRole("combobox", { name: label });
  await rendered.user.type(input, species.nameJa);
  rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
  const option = await within(card).findByRole("option", { name: species.nameJa });
  await rendered.user.click(option);
}

describe("技が使えないマスタ(capabilities.moves === false。ADR-0304 §3・A-5)", () => {
  test("技のセレクトは残るが disabled で、技の選択肢が1件も無い", () => {
    renderScreen(limitedMaster(example, NO_MOVES));
    expect(moveSelect()).toBeDisabled();
    expect(within(moveSelect()).queryAllByRole("option")).toHaveLength(0);
  });

  test("技を選べないことを案内する", () => {
    renderScreen(limitedMaster(example, NO_MOVES));
    expect(screen.getByText(masterOnlineText.movesUnavailable)).toBeInTheDocument();
  });

  test("攻撃側・防御側を選んでも計算しない(壊れた結果を出さない。ADR-0304 §4)", async () => {
    const rendered = renderScreen(limitedMaster(example, NO_MOVES));
    await rendered.user.selectOptions(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      speciesAt(0).key,
    );
    await rendered.user.selectOptions(
      within(defenderCard()).getByRole("combobox", { name: "防御側のポケモン" }),
      speciesAt(1).key,
    );

    expect(rendered.engine.bulkRequests).toHaveLength(0);
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
  });
});

describe("効果データが無いマスタ(capabilities.effects === false。ADR-0304 A-1・A-5)", () => {
  test("「持ち物の候補も比較」は disabled で、案内を添える", () => {
    renderScreen(limitedMaster(example, NO_EFFECTS));
    expect(compareToggle()).toBeDisabled();
    expect(compareToggle()).not.toBeChecked();
    expect(screen.getByText(masterOnlineText.itemCandidatesUnavailable)).toBeInTheDocument();
  });

  test("持ち物そのものの選択は残る(API の計算は itemId だけを送るので成立する)", () => {
    const master = limitedMaster(example, NO_EFFECTS);
    renderScreen(master);
    const select = within(defenderCard()).getByRole("combobox", { name: "防御側の持ち物" });
    expect(select).not.toBeDisabled();
    for (const item of master.items) {
      expect(within(select).getByRole("option", { name: item.nameJa })).toHaveValue(item.id);
    }
  });

  test("計算のリクエストに持ち物の候補が混ざらない(選んだ持ち物だけを送る)", async () => {
    const master = limitedMaster(example, NO_EFFECTS);
    const rendered = renderScreen(master);
    const attacker = speciesAt(0);
    await rendered.user.selectOptions(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      attacker.key,
    );
    await rendered.user.selectOptions(
      within(defenderCard()).getByRole("combobox", { name: "防御側のポケモン" }),
      speciesAt(1).key,
    );
    const item = master.items[0];
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    await rendered.user.selectOptions(
      within(defenderCard()).getByRole("combobox", { name: "防御側の持ち物" }),
      item.id,
    );

    await waitFor(() => {
      expect(rendered.engine.bulkRequests.at(-1)?.itemVariants).toEqual([item]);
    });
  });
});

describe("種族の一覧が無いマスタ(capabilities.speciesList === false。ADR-0304 §1・A-4・A-10)", () => {
  test("ドロップダウンではなく検索欄を出し、入力前は候補を出さない", () => {
    renderScreen(limitedMaster(example, SEARCH_ONLY), fakeSearch());
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });
    expect(input).not.toBeDisabled();
    expect(within(attackerCard()).queryAllByRole("option")).toHaveLength(0);
    expect(within(attackerCard()).getByText(masterOnlineText.speciesSearchEmpty)).toBeInTheDocument();
  });

  test("空白だけの入力では検索しない(空 = 全件にしない)", async () => {
    const search = fakeSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    await rendered.user.type(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      "   ",
    );
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(search.searchCalls).toHaveLength(0);
  });

  test("続けて入力しても、デバウンスの後に最後のクエリで1回だけ検索する", async () => {
    const search = fakeSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });

    await rendered.user.type(input, "テスト");
    expect(search.searchCalls).toHaveLength(0);

    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    expect(search.searchCalls.map((call) => call.query)).toEqual(["テスト"]);
  });

  test("候補を選ぶと種族を解決し、カードに名前とタイプを出す", async () => {
    const search = fakeSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const attacker = speciesAt(0);

    await chooseBySearch(rendered, attackerCard(), "攻撃側のポケモン", attacker);

    expect(search.resolvedKeys).toEqual([attacker.key]);
    expect(within(attackerCard()).getByRole("heading", { name: attacker.nameJa })).toBeInTheDocument();
    expect(within(attackerCard()).getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("攻守そろうと計算し、解決した種族と、解決で返った特性をリクエストに入れる", async () => {
    const search = fakeSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const attacker = speciesAt(0);
    const defender = speciesAt(1);

    await chooseBySearch(rendered, attackerCard(), "攻撃側のポケモン", attacker);
    await chooseBySearch(rendered, defenderCard(), "防御側のポケモン", defender);

    await waitFor(() => {
      expect(rendered.engine.bulkRequests).not.toHaveLength(0);
    });
    const request = rendered.engine.bulkRequests.at(-1);
    expect(request?.attacker.species.key).toBe(attacker.key);
    expect(request?.defenderSpecies.key).toBe(defender.key);
    // MasterData.abilities は空(公開 API に特性の全件一覧が無い。A-1)。画面が resolveSpecies の
    // 特性を覚えていないと、ここは「特性なし」(id: "")に落ちる。
    expect(request?.attacker.ability.id).toBe(attacker.abilities[0]);
  });

  test("一致する種族が無ければ、その旨を出して候補を出さない", async () => {
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), fakeSearch());
    await rendered.user.type(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      "一致しない名前",
    );
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(
      await within(attackerCard()).findByText(masterOnlineText.speciesSearchNoResult),
    ).toBeInTheDocument();
    expect(within(attackerCard()).queryAllByRole("option")).toHaveLength(0);
  });

  test("候補が SPECIES_SEARCH_LIMIT に達したら、全件ではないことを明示する", async () => {
    // 母集団自体を SPECIES_SEARCH_LIMIT 件ちょうどにし、実際に上限へ「達した」状態を作る
    // (limit オプションで人為的に絞ると、実装が候補数を見ずに常に案内を出していても検知できない)。
    const search = createFakeSpeciesSearch({
      species: manySpecies(SPECIES_SEARCH_LIMIT),
      abilities: example.abilities,
    });
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    await rendered.user.type(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      "テスト",
    );
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(
      await within(attackerCard()).findByText(masterOnlineText.speciesSearchTruncated),
    ).toBeInTheDocument();
    // 否定側のテストと対称に、候補の件数も見る(P4-16c(7)。案内の文言だけを見ていると、
    // 候補を1件も出さずに案内だけ出す実装でも緑になる)。
    expect(await within(attackerCard()).findAllByRole("option")).toHaveLength(SPECIES_SEARCH_LIMIT);
  });

  test("候補が SPECIES_SEARCH_LIMIT 未満なら、全件ではないことを明示しない(critic 指摘の回帰ガード)", async () => {
    // manySpecies(SPECIES_SEARCH_LIMIT) は達するが、1件少ない母集団では出ないことを確かめる
    // (「候補が1件でもあれば常に出す」実装への回帰を検知する)。
    const search = createFakeSpeciesSearch({
      species: manySpecies(SPECIES_SEARCH_LIMIT - 1),
      abilities: example.abilities,
    });
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    await rendered.user.type(
      within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" }),
      "テスト",
    );
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(await within(attackerCard()).findAllByRole("option")).toHaveLength(SPECIES_SEARCH_LIMIT - 1);
    expect(
      within(attackerCard()).queryByText(masterOnlineText.speciesSearchTruncated),
    ).not.toBeInTheDocument();
  });

  test("検索に失敗したら、その旨を出して古い候補を残さない", async () => {
    const search = createDeferredSpeciesSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });

    await rendered.user.type(input, "テスト");
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    const first = search.searchCalls[0];
    if (first === undefined) {
      throw new Error("検索が呼ばれていない");
    }
    act(() => {
      first.reject(new Error("テストの検索失敗"));
    });

    expect(await within(attackerCard()).findByText(masterOnlineText.speciesSearchFailed)).toBeInTheDocument();
    expect(within(attackerCard()).queryAllByRole("option")).toHaveLength(0);
  });

  test("入力が変わると前の検索を取り消し、古い応答で新しい候補を上書きしない", async () => {
    const search = createDeferredSpeciesSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });
    const stale = speciesAt(0);
    const fresh = speciesAt(1);

    await rendered.user.type(input, "テス");
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    await rendered.user.type(input, "ト");
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);

    expect(search.searchCalls).toHaveLength(2);
    const [older, newer] = search.searchCalls;
    if (older === undefined || newer === undefined) {
      throw new Error("検索が2回呼ばれていない");
    }
    expect(older.signal?.aborted).toBe(true);

    act(() => {
      newer.resolve([fresh]);
      older.resolve([stale]);
    });

    expect(await within(attackerCard()).findByRole("option", { name: fresh.nameJa })).toBeInTheDocument();
    expect(within(attackerCard()).queryByRole("option", { name: stale.nameJa })).toBeNull();
  });

  test("入力を空へ戻したあとに前の検索が届いても、候補を出さない(P4-16c(4))", async () => {
    // 「入力中 → 全部消す」の直後に、取り消したはずの検索が遅れて応答するケース。
    // 取り消しを見ずに setState する実装だと、空の入力欄の下に候補が出たままになる。
    const search = createDeferredSpeciesSearch();
    const rendered = renderScreen(limitedMaster(example, SEARCH_ONLY), search);
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });
    const stale = speciesAt(0);

    await rendered.user.type(input, "テスト");
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    expect(search.searchCalls).toHaveLength(1);

    await rendered.user.clear(input);
    rendered.advance(SPECIES_SEARCH_DEBOUNCE_MS);
    // 空の入力では検索し直さない(空 = 全件にしない)。
    expect(search.searchCalls).toHaveLength(1);

    const pending = search.searchCalls[0];
    if (pending === undefined) {
      throw new Error("検索が呼ばれていない");
    }
    expect(pending.signal?.aborted).toBe(true);
    act(() => {
      pending.resolve([stale]);
    });

    expect(within(attackerCard()).queryAllByRole("option")).toHaveLength(0);
    expect(within(attackerCard()).getByText(masterOnlineText.speciesSearchEmpty)).toBeInTheDocument();
  });

  test("検索口が渡されていなければ、空のドロップダウンを出さず検索欄を disabled にする", () => {
    renderScreen(limitedMaster(example, SEARCH_ONLY));
    const input = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });
    expect(input).toBeDisabled();
    expect(within(attackerCard()).queryAllByRole("option")).toHaveLength(0);
    expect(within(attackerCard()).getByText(masterOnlineText.speciesSearchFailed)).toBeInTheDocument();
  });
});

describe("capabilities を省いたマスタ(オフライン相当)は今までどおり", () => {
  test("技のセレクトは使え、案内も検索欄も出さない", () => {
    renderScreen(example);
    expect(moveSelect()).not.toBeDisabled();
    expect(compareToggle()).not.toBeDisabled();
    expect(screen.queryByText(masterOnlineText.movesUnavailable)).toBeNull();
    expect(screen.queryByText(masterOnlineText.itemCandidatesUnavailable)).toBeNull();
    expect(screen.queryByText(masterOnlineText.speciesSearchEmpty)).toBeNull();
    // ポケモンの選択はドロップダウンのまま(全種族が選択肢に出る)。
    const select = within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" });
    for (const species of example.species) {
      expect(within(select).getByRole("option", { name: species.nameJa })).toHaveValue(species.key);
    }
  });
});
