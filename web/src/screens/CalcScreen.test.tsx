// P4-2: 計算画面(docs/design.md「画面: ダメージ計算」、requirements.md「相手側の一括表示」、ADR-0016 §2・§6)。
// engine は fake(ADR-0016 §8)。マスタは架空の例データ(exampleMasterSource)を使い、特定の名前には依存しない。
// 確かめること:
//   - 左右のカード(ポケモン・タイプ・エンブレム・持ち物)と技セレクタ
//   - 攻撃側・防御側・ダメージ技が揃ったら calcBulk を1回呼び、そのリクエストの中身(ADR-0016 §2・§6、ADR-0009)
//   - 返ってきた行を加工せずに表示(調整名・%幅・ダメージバー・確定数)
//   - 持ち物の候補の比較、攻守入れ替え、エラー表示、古い応答で新しい表示を上書きしないこと
// 攻撃側は P4-2 では無振り・無補正で固定(攻撃側プリセットの選択は P4-3)。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import { defaultAbility, defensiveItemCandidates, toEngineSpecies } from "../domain/requests";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import type { BulkRequest, Move } from "../engine/types";
import { typeNameJa, type TypeId } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  bulkRow,
  createDeferredEngine,
  createFakeEngine,
  engineError,
  ok,
  type FakeEngine,
} from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

function firstMoveOf(species: MasterSpecies): Move {
  const move = firstDamagingMove(species, master.moves);
  if (move === undefined) {
    throw new Error(`${species.key} がダメージ技を覚えない`);
  }
  return move;
}

function lastRequest(engine: FakeEngine): BulkRequest {
  const request = engine.bulkRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return request;
}

const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const attackerItemSelect = () => screen.getByRole("combobox", { name: "攻撃側の持ち物" });
const defenderItemSelect = () => screen.getByRole("combobox", { name: "防御側の持ち物" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });

function renderScreen(engine: FakeEngine = createFakeEngine()): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  return { user, engine };
}

async function choosePair(user: UserEvent, attacker: MasterSpecies, defender: MasterSpecies): Promise<void> {
  await user.selectOptions(attackerSpeciesSelect(), attacker.key);
  await user.selectOptions(defenderSpeciesSelect(), defender.key);
}

async function resultItems(): Promise<HTMLElement[]> {
  const list = await screen.findByRole("list", { name: "計算結果" });
  return within(list).getAllByRole("listitem");
}

describe("カード", () => {
  test("攻撃側(左)と防御側(右)のカードに、全種族のポケモンの選択と「なし」+全持ち物の持ち物の選択がある", () => {
    renderScreen();
    expect(within(attackerCard()).getByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
    expect(within(defenderCard()).getByRole("combobox", { name: "防御側のポケモン" })).toBeInTheDocument();
    for (const select of [attackerSpeciesSelect(), defenderSpeciesSelect()]) {
      for (const species of master.species) {
        expect(within(select).getByRole("option", { name: species.nameJa })).toHaveValue(species.key);
      }
    }
    for (const select of [attackerItemSelect(), defenderItemSelect()]) {
      expect(within(select).getByRole("option", { name: "なし" })).toHaveValue("");
      for (const item of master.items) {
        expect(within(select).getByRole("option", { name: item.nameJa })).toHaveValue(item.id);
      }
    }
  });

  test("ポケモンを選ぶと、カードに名前・タイプ(表示名)・タイプ色のエンブレム(画像なしで成立)を出す", async () => {
    const { user } = renderScreen();
    const attacker = speciesAt(0);
    const defender = speciesAt(1);
    await choosePair(user, attacker, defender);

    for (const [card, species] of [
      [attackerCard(), attacker],
      [defenderCard(), defender],
    ] as const) {
      expect(within(card).getByRole("heading", { name: species.nameJa })).toBeInTheDocument();
      for (const type of species.types) {
        expect(within(card).getByText(typeNameJa[type as TypeId])).toBeInTheDocument();
      }
      // 色の値は CSS 変数(P4-1 の --type-<id>)で参照し、TS に色を書かない
      expect(within(card).getByTestId("type-emblem").getAttribute("style") ?? "").toContain(
        `var(--type-${species.types[0] ?? ""})`,
      );
    }
  });
});

describe("技セレクタ", () => {
  test("攻撃側の learnset の技を順に出し、分類と威力を併記し、最初のダメージ技を選んでおく", async () => {
    const { user } = renderScreen();
    const attacker = speciesAt(0);
    await user.selectOptions(attackerSpeciesSelect(), attacker.key);

    const options = within(moveSelect()).getAllByRole("option");
    const moves = learnsetMoves(attacker, master.moves);
    const moveOptions = options.filter((option) => option.getAttribute("value") !== "");
    expect(moveOptions.map((option) => option.getAttribute("value"))).toEqual(moves.map((move) => move.id));
    const categoryLabel = { physical: "物理", special: "特殊", status: "変化" } as const;
    moves.forEach((move, index) => {
      const text = moveOptions[index]?.textContent ?? "";
      expect(text).toContain(move.nameJa);
      expect(text).toContain(categoryLabel[move.category]);
      if (move.category !== "status") {
        expect(text).toContain(String(move.power));
      }
    });
    expect(moveSelect()).toHaveValue(firstMoveOf(attacker).id);
  });
});

describe("計算の呼び出し", () => {
  test("開いた時点では計算しない(engine.wasm の読み込みを初回の計算まで遅らせる。ADR-0016 §2)", () => {
    const { engine } = renderScreen();
    expect(engine.bulkRequests).toHaveLength(0);
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
  });

  test("攻撃側だけでは計算せず、防御側も選ぶと calcBulk を1回呼ぶ", async () => {
    const { user, engine } = renderScreen();
    await user.selectOptions(attackerSpeciesSelect(), speciesAt(0).key);
    expect(engine.bulkRequests).toHaveLength(0);
    await user.selectOptions(defenderSpeciesSelect(), speciesAt(1).key);
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });
  });

  test("リクエスト: 攻撃側はレベル50・無振り・無補正・既定の特性・持ち物なし、防御側は種族だけ、presetKeys は省略", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesAt(0);
    const defender = speciesAt(1);
    await choosePair(user, attacker, defender);
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });

    const request = lastRequest(engine);
    expect(request).toMatchObject({
      format: "single",
      attacker: {
        species: toEngineSpecies(attacker),
        level: 50,
        sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
        nature: { plus: "", minus: "" },
        ability: defaultAbility(attacker, master.abilities),
        item: null,
      },
      defenderSpecies: toEngineSpecies(defender),
      move: firstMoveOf(attacker),
      typeChart: master.typeChart,
    });
    expect(request).not.toHaveProperty("presetKeys");
    expect(request).not.toHaveProperty("presets");
    expect(request).not.toHaveProperty("itemVariants");
    // engine の境界は未知のフィールドを拒否する。画面のための learnset を渡さない。
    expect(Object.keys(request.attacker.species)).not.toContain("learnset");
    expect(Object.keys(request.defenderSpecies)).not.toContain("learnset");
  });

  test("入力を変えるたびに計算し直す(防御側・技・攻撃側の持ち物)", async () => {
    const { user, engine } = renderScreen();
    const attacker = master.species.find(
      (species) =>
        learnsetMoves(species, master.moves).filter((move) => move.category !== "status").length >= 2,
    );
    if (attacker === undefined) {
      throw new Error("ダメージ技を2つ以上覚える種族が例データに無い");
    }
    const others = master.species.filter((species) => species.key !== attacker.key);
    const [firstDefender, secondDefender] = others;
    if (firstDefender === undefined || secondDefender === undefined) {
      throw new Error("例データの種族が足りない");
    }
    await choosePair(user, attacker, firstDefender);
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });

    await user.selectOptions(defenderSpeciesSelect(), secondDefender.key);
    await waitFor(() => {
      expect(lastRequest(engine).defenderSpecies.key).toBe(secondDefender.key);
    });

    const secondMove = learnsetMoves(attacker, master.moves).filter((move) => move.category !== "status")[1];
    if (secondMove === undefined) {
      throw new Error("2つ目のダメージ技が無い");
    }
    await user.selectOptions(moveSelect(), secondMove.id);
    await waitFor(() => {
      expect(lastRequest(engine).move).toEqual(secondMove);
    });

    const item = master.items[0];
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    await user.selectOptions(attackerItemSelect(), item.id);
    await waitFor(() => {
      expect(lastRequest(engine).attacker.item).toEqual(item);
    });
  });

  test("攻撃側を変えたとき、今の技を新しい攻撃側が覚えなければ、その最初のダメージ技に戻す", async () => {
    const { user, engine } = renderScreen();
    const first = speciesAt(0);
    const firstMove = firstMoveOf(first);
    const other = master.species.find(
      (species) => species.key !== first.key && !species.learnset.includes(firstMove.id),
    );
    if (other === undefined) {
      throw new Error(`例データに ${firstMove.id} を覚えない別の種族が無い`);
    }
    await choosePair(user, first, speciesAt(1));
    await user.selectOptions(attackerSpeciesSelect(), other.key);
    expect(moveSelect()).toHaveValue(firstMoveOf(other).id);
    await waitFor(() => {
      expect(lastRequest(engine).move).toEqual(firstMoveOf(other));
    });
  });

  test("変化技を選ぶと計算せず、その旨を出し、結果の行を消す", async () => {
    const { user, engine } = renderScreen();
    const attacker = master.species.find((species) =>
      learnsetMoves(species, master.moves).some((move) => move.category === "status"),
    );
    if (attacker === undefined) {
      throw new Error("変化技を覚える種族が例データに無い");
    }
    const statusMove = learnsetMoves(attacker, master.moves).find((move) => move.category === "status");
    const defender = master.species.find((species) => species.key !== attacker.key);
    if (statusMove === undefined || defender === undefined) {
      throw new Error("例データが足りない");
    }
    await choosePair(user, attacker, defender);
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });
    await resultItems();

    await user.selectOptions(moveSelect(), statusMove.id);
    expect(await screen.findByText(/変化技/)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
    expect(engine.bulkRequests).toHaveLength(1);
  });
});

describe("結果の表示(engine の値を加工せずに出す)", () => {
  const rows = [
    bulkRow({ preset: "none", presetLabel: "無振り", minPercent: 72.1, maxPercent: 85.3, effectiveness: 2 }),
    bulkRow({
      preset: "hp",
      presetLabel: "H振り",
      minPercent: 100,
      maxPercent: 120.5,
      effectiveness: 2,
      ko: { hits: 1, guaranteed: true, chancePercent: 0, displayChancePercent: 100 },
    }),
    bulkRow({
      preset: "hb_full",
      presetLabel: "HB特化",
      minPercent: 44.3,
      maxPercent: 52.5,
      effectiveness: 2,
      // chancePercent を1桁に丸めると displayChancePercent と違う値("49.9" vs "50.0")になるようにし、
      // formatKO が生値 chancePercent を使ってしまう回帰を検出できるようにする(ADR-0006)。
      ko: { hits: 2, guaranteed: false, chancePercent: 49.94, displayChancePercent: 50.0 },
    }),
  ];

  async function renderWithRows() {
    const engine = createFakeEngine((request) =>
      ok({ defenderSpeciesKey: request.defenderSpecies.key, rows }),
    );
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    return resultItems();
  }

  test("行は engine の順のまま1行ずつ: 調整名(presetLabel)・%幅・確定数", async () => {
    const items = await renderWithRows();
    expect(items).toHaveLength(rows.length);
    const expected = [
      ["無振り", "72.1〜85.3%", "確定2発"],
      ["H振り", "100.0〜120.5%", "確定1発"],
      ["HB特化", "44.3〜52.5%", "乱数2発(50.0%)"],
    ];
    expected.forEach(([label, range, ko], index) => {
      const item = items[index];
      if (item === undefined) {
        throw new Error(`${index} 行目が無い`);
      }
      expect(within(item).getByText(label ?? "")).toBeInTheDocument();
      expect(within(item).getByText(range ?? "")).toBeInTheDocument();
      expect(within(item).getByText(ko ?? "")).toBeInTheDocument();
    });
  });

  test("ダメージバーは最大%を値に持ち、100% を超える分は 100 で頭打ち", async () => {
    const items = await renderWithRows();
    const values = items.map((item) => {
      const meter = within(item).getByRole("meter");
      expect(meter).toHaveAttribute("aria-valuemin", "0");
      expect(meter).toHaveAttribute("aria-valuemax", "100");
      return meter.getAttribute("aria-valuenow");
    });
    expect(values).toEqual(["85.3", "100", "52.5"]);
  });

  test("技の相性は結果の effectiveness から出す(TS で相性を計算しない)", async () => {
    await renderWithRows();
    expect(screen.getByText("効果はばつぐん")).toBeInTheDocument();
  });

  test("engine のエラーは message を role=alert で出し、結果の行を出さない", async () => {
    const engine = createFakeEngine(() => engineError("invalid_input", "攻撃側の入力が不正: テスト"));
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    expect(await screen.findByRole("alert")).toHaveTextContent("攻撃側の入力が不正: テスト");
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
  });

  test("古い計算の応答が後から届いても、新しい入力の結果を上書きしない", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = renderScreen(engine);
    const attacker = master.species.find(
      (species) =>
        learnsetMoves(species, master.moves).filter((move) => move.category !== "status").length >= 2,
    );
    const defender = master.species.find((species) => species.key !== attacker?.key);
    if (attacker === undefined || defender === undefined) {
      throw new Error("例データが足りない");
    }
    const secondMove = learnsetMoves(attacker, master.moves).filter((move) => move.category !== "status")[1];
    if (secondMove === undefined) {
      throw new Error("2つ目のダメージ技が無い");
    }
    await choosePair(user, attacker, defender);
    await waitFor(() => {
      expect(pending).toHaveLength(1);
    });
    await user.selectOptions(moveSelect(), secondMove.id);
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });

    const [older, newer] = pending;
    await act(async () => {
      newer?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "新しい結果" })] }),
      );
      await Promise.resolve();
    });
    await act(async () => {
      older?.resolve(ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "古い結果" })] }));
      await Promise.resolve();
    });
    expect(screen.getByText("新しい結果")).toBeInTheDocument();
    expect(screen.queryByText("古い結果")).toBeNull();
  });

  test("入力を変えると、応答が届くまで古い行を消して「計算中」を出す(ADR-0016 §8)", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = renderScreen(engine);
    const attacker = master.species.find(
      (species) =>
        learnsetMoves(species, master.moves).filter((move) => move.category !== "status").length >= 2,
    );
    const defender = master.species.find((species) => species.key !== attacker?.key);
    if (attacker === undefined || defender === undefined) {
      throw new Error("例データが足りない");
    }
    const secondMove = learnsetMoves(attacker, master.moves).filter((move) => move.category !== "status")[1];
    if (secondMove === undefined) {
      throw new Error("2つ目のダメージ技が無い");
    }
    await choosePair(user, attacker, defender);
    await waitFor(() => {
      expect(pending).toHaveLength(1);
    });
    await act(async () => {
      pending[0]?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "最初の結果" })] }),
      );
      await Promise.resolve();
    });
    expect(await screen.findByText("最初の結果")).toBeInTheDocument();

    await user.selectOptions(moveSelect(), secondMove.id);
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    // 新しい入力の応答をまだ待っている間: 古い行は消え、「計算中」を出す。
    expect(screen.queryByText("最初の結果")).toBeNull();
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
    const loadingNotice = await screen.findByText("計算中");
    expect(loadingNotice.closest('[aria-busy="true"]')).not.toBeNull();

    await act(async () => {
      pending[1]?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "新しい結果" })] }),
      );
      await Promise.resolve();
    });
    expect(await screen.findByText("新しい結果")).toBeInTheDocument();
    expect(screen.queryByText("計算中")).toBeNull();
  });
});

describe("防御側の持ち物(ADR-0016 §6)", () => {
  test("トグルなしで防御側の持ち物を選ぶと、itemVariants はその持ち物1つで、行に持ち物名を出す", async () => {
    const { user, engine } = renderScreen();
    const item = master.items[0];
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.selectOptions(defenderItemSelect(), item.id);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toEqual([item]);
    });
    await waitFor(async () => {
      const items = await resultItems();
      expect(items.every((row) => within(row).queryByText(item.nameJa) !== null)).toBe(true);
    });
  });

  test("「持ち物の候補も比較」を入れると、なし + 効果データから選んだ候補を itemVariants に渡し、行を持ち物で見分けられる", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesAt(0);
    await choosePair(user, attacker, speciesAt(1));
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });

    await user.click(screen.getByRole("checkbox", { name: "持ち物の候補も比較" }));
    const candidates = defensiveItemCandidates(master.items, firstMoveOf(attacker));
    expect(candidates.length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toEqual([null, ...candidates]);
    });

    // fake はバリアントごとに5行を返す(bulkResultFor)
    await waitFor(async () => {
      expect(await resultItems()).toHaveLength(5 * (candidates.length + 1));
    });
    const items = await resultItems();
    const rowsLabelled = (label: string) =>
      items.filter((row) => within(row).queryByText(label) !== null).length;
    expect(rowsLabelled("持ち物なし")).toBe(5);
    for (const candidate of candidates) {
      expect(rowsLabelled(candidate.nameJa)).toBe(5);
    }
  });

  test("トグルを外すと itemVariants を送らない(持ち物なしのとき)", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    const toggle = screen.getByRole("checkbox", { name: "持ち物の候補も比較" });
    await user.click(toggle);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toBeDefined();
    });
    await user.click(toggle);
    await waitFor(() => {
      expect(lastRequest(engine)).not.toHaveProperty("itemVariants");
    });
  });
});

describe("攻守入れ替え", () => {
  test("ポケモンと持ち物を入れ替え、技は新しい攻撃側の最初のダメージ技にして計算し直す", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesAt(0);
    const defender = speciesAt(1);
    const [attackerItem, defenderItem] = master.items;
    if (attackerItem === undefined || defenderItem === undefined) {
      throw new Error("例データに持ち物が2つ以上要る");
    }
    await choosePair(user, attacker, defender);
    await user.selectOptions(attackerItemSelect(), attackerItem.id);
    await user.selectOptions(defenderItemSelect(), defenderItem.id);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toEqual([defenderItem]);
    });
    const callsBeforeSwap = engine.bulkRequests.length;

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));

    expect(attackerSpeciesSelect()).toHaveValue(defender.key);
    expect(defenderSpeciesSelect()).toHaveValue(attacker.key);
    expect(attackerItemSelect()).toHaveValue(defenderItem.id);
    expect(defenderItemSelect()).toHaveValue(attackerItem.id);
    expect(moveSelect()).toHaveValue(firstMoveOf(defender).id);
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(callsBeforeSwap);
    });
    expect(lastRequest(engine)).toMatchObject({
      attacker: { species: toEngineSpecies(defender), item: defenderItem },
      defenderSpecies: toEngineSpecies(attacker),
      move: firstMoveOf(defender),
      itemVariants: [attackerItem],
    });
  });
});
