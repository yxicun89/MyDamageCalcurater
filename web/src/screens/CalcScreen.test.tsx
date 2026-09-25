// P4-2: 計算画面(docs/design.md「画面: ダメージ計算」、requirements.md「相手側の一括表示」、ADR-0300 §2・§6)。
// engine は fake(ADR-0300 §8)。マスタは架空の例データ(exampleMasterSource)を使い、特定の名前には依存しない。
// 確かめること:
//   - 左右のカード(ポケモン・タイプ・エンブレム・持ち物)と技セレクタ
//   - 攻撃側・防御側・ダメージ技が揃ったら calcBulk を1回呼び、そのリクエストの中身(ADR-0300 §2・§6、ADR-0009)
//   - 返ってきた行を加工せずに表示(調整名・%幅・ダメージバー・確定数)
//   - 持ち物の候補の比較、攻守入れ替え、エラー表示、古い応答で新しい表示を上書きしないこと
// 攻撃側の既定は無振り・無補正。攻撃側プリセットの選択(P4-3、ADR-0300 §5)は末尾の describe で確かめる。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import { defaultAbility, defensiveItemCandidates, toEngineSpecies } from "../domain/requests";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import type { BulkRequest, Item, Move, UnsupportedMark } from "../engine/types";
import { typeNameJa, unsupportedText, type TypeId } from "../i18n/ja";
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

  // issue #306: タイプ名は「文字色にタイプ色」ではなくバッジ(背景にタイプ色 + 読める文字色)にする。
  // タイプ色は背景(bg.base)に対して多くが 4.5:1 に届かないため(design.md「タイプバッジ」)。
  test("タイプ名はバッジ: 背景が --type-<id>、文字色が --type-<id>-ink(design.md「タイプバッジ」)", async () => {
    const { user } = renderScreen();
    const attacker = speciesAt(0);
    const defender = speciesAt(1);
    await choosePair(user, attacker, defender);

    for (const [card, species] of [
      [attackerCard(), attacker],
      [defenderCard(), defender],
    ] as const) {
      for (const type of species.types) {
        const badge = within(card).getByText(typeNameJa[type as TypeId]);
        expect(badge).toHaveClass("calc-card__type");
        expect(badge.style.getPropertyValue("background-color")).toMatch(
          new RegExp(`^var\\(\\s*--type-${type}\\s*[,)]`),
        );
        expect(badge.style.getPropertyValue("color")).toMatch(
          new RegExp(`^var\\(\\s*--type-${type}-ink\\s*[,)]`),
        );
      }
    }
  });

  // issue #306 の異常系: マスタ由来の types は相性表の18種に限らない。
  // 未知の ID では CSS 変数が引けないので、必ず既定値つきの var(...) にして画面を壊さない。
  test("未知のタイプ ID でもバッジは ID をそのまま出し、色は既定値に落ちる", async () => {
    const unknownType = "mysteryType";
    const target = speciesAt(0);
    const patched: MasterData = {
      ...master,
      species: master.species.map((entry) =>
        entry.key === target.key ? { ...entry, types: [unknownType] } : entry,
      ),
    };
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={patched} />);
    await user.selectOptions(attackerSpeciesSelect(), target.key);

    const badge = within(attackerCard()).getByText(unknownType);
    expect(badge).toHaveClass("calc-card__type");
    // 既定値(`,` の後ろ)を必ず持つこと。`var(--type-mysteryType)` だけだと宣言ごと無効になる。
    expect(badge.style.getPropertyValue("background-color")).toMatch(
      new RegExp(`^var\\(\\s*--type-${unknownType}\\s*,`),
    );
    expect(badge.style.getPropertyValue("color")).toMatch(
      new RegExp(`^var\\(\\s*--type-${unknownType}-ink\\s*,`),
    );
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
  test("開いた時点では計算しない(engine.wasm の読み込みを初回の計算まで遅らせる。ADR-0300 §2)", () => {
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

  // issue #306: バーは同じ行の %幅 を目で分かる形にしただけなので装飾にする(design.md「画面: ダメージ計算」)。
  // 名前の無い meter(aria-valuenow だけ)として読み上げられるのを避ける。
  // 頭打ちの検査は aria-valuenow からバーの幅(見た目そのもの)に移す。検査の強さは落とさない。
  test("ダメージバーは幅に最大%を持ち、100% を超える分は 100 で頭打ち", async () => {
    const items = await renderWithRows();
    const widths = items.map((item) =>
      within(item).getByTestId("damage-bar-fill").style.getPropertyValue("width"),
    );
    expect(widths).toEqual(["85.3%", "100%", "52.5%"]);
  });

  test("ダメージバーは装飾: role=meter を持たず aria-hidden で、aria-value* も残さない", async () => {
    const items = await renderWithRows();
    expect(screen.queryAllByRole("meter")).toEqual([]);
    for (const item of items) {
      const bar = within(item).getByTestId("damage-bar");
      expect(bar).toHaveAttribute("aria-hidden", "true");
      for (const attribute of ["role", "aria-valuemin", "aria-valuemax", "aria-valuenow"]) {
        expect(bar.hasAttribute(attribute), `${attribute} が残っている`).toBe(false);
      }
    }
  });

  test("バーを装飾にしても、行の読み上げには 調整名・持ち物・%幅・確定数 が残る", async () => {
    const items = await renderWithRows();
    const first = items[0];
    if (first === undefined) {
      throw new Error("1行目が無い");
    }
    for (const text of ["無振り", "持ち物なし", "72.1〜85.3%", "確定2発"]) {
      expect(first.textContent).toContain(text);
    }
  });

  test("技の相性は結果の effectiveness から出す(TS で相性を計算しない)", async () => {
    await renderWithRows();
    expect(screen.getByText("ばつぐん(×2)")).toBeInTheDocument();
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

  test("入力を変えると、前の calcBulk 要求を abort する(issue 248。gateway の取り消し伝播はissue 113で実装済み)", async () => {
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
    expect(pending[0]?.signal?.aborted).toBe(false);

    await user.selectOptions(moveSelect(), secondMove.id);
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });

    expect(pending[0]?.signal?.aborted).toBe(true);
    expect(pending[1]?.signal?.aborted).toBe(false);
  });

  test("入力を変えると、応答が届くまで古い行を消して「計算中」を出す(ADR-0300 §8)", async () => {
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

describe("防御側の持ち物(ADR-0300 §6)", () => {
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

// P4-3: 攻撃側(自分側)のプリセット(ADR-0300 §5、requirements.md「自分側のプリセット」)。
// 決めたこと:
//   - 選択は攻撃側カードの中のラジオグループ(名前「攻撃側の調整」)。design.md「入力はタップで選ぶ」のピル型を想定し、
//     select ではなく radio にする。既定は無振り。
//   - 選んでいるのは Key(none / x_full / x)。技の分類が変わっても Key は保ち、表示名(A特化 ⇄ C特化 など)と
//     リクエストの SP・性格が分類に合わせて変わる。
//   - 攻守入れ替えでは Key を保つ(攻撃側の調整は「自分側」の設定で、入れ替え後も自分が攻撃側のため)。
//   - 変化技を選んでいるときの表示名は物理と同じ(domain/attackerPresets.test.ts)。
describe("攻撃側のプリセット(P4-3)", () => {
  const presetGroup = () => within(attackerCard()).getByRole("radiogroup", { name: "攻撃側の調整" });
  const presetRadio = (name: string) => within(presetGroup()).getByRole("radio", { name });

  /** 物理と特殊の両方のダメージ技を覚える種族と、その2つの技。 */
  function mixedAttacker(): { species: MasterSpecies; physical: Move; special: Move } {
    for (const species of master.species) {
      const moves = learnsetMoves(species, master.moves);
      const physical = moves.find((move) => move.category === "physical");
      const special = moves.find((move) => move.category === "special");
      if (physical !== undefined && special !== undefined) {
        return { species, physical, special };
      }
    }
    throw new Error("例データに物理と特殊の両方を覚える種族が無い");
  }

  /** 最初のダメージ技が指定の分類になる種族。 */
  function speciesWithFirstMove(category: "physical" | "special", exceptKey = ""): MasterSpecies {
    const found = master.species.find(
      (species) =>
        species.key !== exceptKey && firstDamagingMove(species, master.moves)?.category === category,
    );
    if (found === undefined) {
      throw new Error(`例データに最初のダメージ技が ${category} の種族が無い`);
    }
    return found;
  }

  test("攻撃側カードに3つの選択肢(物理: 無振り・A特化・A振り(無補正))がこの順で並び、既定は無振り", async () => {
    const { user } = renderScreen();
    const attacker = speciesWithFirstMove("physical");
    await choosePair(user, attacker, speciesWithFirstMove("special", attacker.key));

    const radios = within(presetGroup()).getAllByRole("radio");
    expect(radios).toHaveLength(3);
    expect(presetRadio("無振り")).toBeChecked();
    expect(presetRadio("A特化")).not.toBeChecked();
    expect(presetRadio("A振り(無補正)")).not.toBeChecked();
    // 並び順は none → x_full → x
    expect(radios.indexOf(presetRadio("無振り"))).toBe(0);
    expect(radios.indexOf(presetRadio("A特化"))).toBe(1);
    expect(radios.indexOf(presetRadio("A振り(無補正)"))).toBe(2);
  });

  test("特殊技を選んでいるときは C 表記(無振り・C特化・C振り(無補正))になる", async () => {
    const { user } = renderScreen();
    const attacker = speciesWithFirstMove("special");
    await choosePair(user, attacker, speciesWithFirstMove("physical", attacker.key));

    expect(presetRadio("無振り")).toBeChecked();
    expect(presetRadio("C特化")).toBeInTheDocument();
    expect(presetRadio("C振り(無補正)")).toBeInTheDocument();
    expect(within(presetGroup()).queryByRole("radio", { name: "A特化" })).toBeNull();
  });

  test("A特化を選ぶと計算し直し、攻撃側は A:32・他 0、性格は +atk / −spa", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesWithFirstMove("physical");
    await choosePair(user, attacker, speciesWithFirstMove("special", attacker.key));
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });

    await user.click(presetRadio("A特化"));

    expect(presetRadio("A特化")).toBeChecked();
    expect(presetRadio("無振り")).not.toBeChecked();
    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(2);
    });
    expect(lastRequest(engine).attacker).toMatchObject({
      level: 50,
      sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 },
      nature: { plus: "atk", minus: "spa" },
    });
  });

  test("A振り(無補正)は A:32・無補正、無振りに戻すと SP 0・無補正に戻る", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesWithFirstMove("physical");
    await choosePair(user, attacker, speciesWithFirstMove("special", attacker.key));

    await user.click(presetRadio("A振り(無補正)"));
    await waitFor(() => {
      expect(lastRequest(engine).attacker.sp).toEqual({ hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 });
    });
    expect(lastRequest(engine).attacker.nature).toEqual({ plus: "", minus: "" });

    await user.click(presetRadio("無振り"));
    await waitFor(() => {
      expect(lastRequest(engine).attacker.sp).toEqual({ hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 });
    });
    expect(lastRequest(engine).attacker.nature).toEqual({ plus: "", minus: "" });
  });

  test("A特化のまま特殊技に替えると、Key を保って表示は C特化、リクエストは C:32・+spa / −atk", async () => {
    const { user, engine } = renderScreen();
    const { species, physical, special } = mixedAttacker();
    const defender = master.species.find((candidate) => candidate.key !== species.key);
    if (defender === undefined) {
      throw new Error("例データの種族が足りない");
    }
    await choosePair(user, species, defender);
    await user.selectOptions(moveSelect(), physical.id);
    await user.click(presetRadio("A特化"));
    await waitFor(() => {
      expect(lastRequest(engine)).toMatchObject({ move: physical, attacker: { nature: { plus: "atk" } } });
    });

    await user.selectOptions(moveSelect(), special.id);

    expect(presetRadio("C特化")).toBeChecked();
    expect(within(presetGroup()).queryByRole("radio", { name: "A特化" })).toBeNull();
    await waitFor(() => {
      expect(lastRequest(engine).move).toEqual(special);
    });
    expect(lastRequest(engine).attacker).toMatchObject({
      sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
      nature: { plus: "spa", minus: "atk" },
    });
  });

  test("調整を替えると、応答が届くまで古い行を消して「計算中」を出す(古い調整の結果を新しい調整の結果として出さない)", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = renderScreen(engine);
    const attacker = speciesWithFirstMove("physical");
    const defender = speciesWithFirstMove("special", attacker.key);
    await choosePair(user, attacker, defender);
    await waitFor(() => {
      expect(pending).toHaveLength(1);
    });
    await act(async () => {
      pending[0]?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "無振りの結果" })] }),
      );
      await Promise.resolve();
    });
    expect(await screen.findByText("無振りの結果")).toBeInTheDocument();

    await user.click(presetRadio("A特化"));
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    expect(screen.queryByText("無振りの結果")).toBeNull();
    expect(await screen.findByText("計算中")).toBeInTheDocument();

    await act(async () => {
      pending[1]?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "A特化の結果" })] }),
      );
      await Promise.resolve();
    });
    expect(await screen.findByText("A特化の結果")).toBeInTheDocument();
  });

  test("攻守入れ替えでも調整の Key を保つ(物理の A特化 → 入れ替え後の特殊技では C特化)", async () => {
    const { user, engine } = renderScreen();
    const attacker = speciesWithFirstMove("physical");
    const defender = speciesWithFirstMove("special", attacker.key);
    await choosePair(user, attacker, defender);
    await user.click(presetRadio("A特化"));
    await waitFor(() => {
      expect(lastRequest(engine).attacker.nature).toEqual({ plus: "atk", minus: "spa" });
    });

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));

    expect(presetRadio("C特化")).toBeChecked();
    await waitFor(() => {
      expect(lastRequest(engine).attacker.species).toEqual(toEngineSpecies(defender));
    });
    expect(lastRequest(engine).attacker).toMatchObject({
      sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
      nature: { plus: "spa", minus: "atk" },
    });
  });
});

// P4-19(issue #110、ADR-0208、DECISIONS.md 2026-09-23): itemVariants は「持ち物なし」を含めて 64 通りまで
// (api/openapi.yaml の BulkCalcRequest.itemVariants の maxItems)。超えたまま送ると API は 400 invalid_input、
// engine も上限超過で失敗するので、画面に渡す前に先頭から絞り込み、絞り込んだことを利用者に出す。
// 期待値の 64 は契約から直接書く(定数とのずれは domain/requestLimits.test.ts が検出する)。
describe("持ち物候補の件数の上限(issue #110)", () => {
  const itemsTruncatedNotice = "持ち物の候補が多いため、先頭から64通りまでで計算しています";

  /** 物理・特殊のどちらの技でも防御側の候補になる架空の持ち物を count 件(マスタの順)。 */
  function defenseItems(count: number): Item[] {
    return Array.from({ length: count }, (_value, index) => ({
      id: `example-many-def-${String(index)}`,
      nameJa: `テスト防御${String(index)}`,
      effect: { statMods: { def: 6144, spd: 6144 } },
    }));
  }

  async function compareWithItems(count: number): Promise<FakeEngine> {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={{ ...master, items: defenseItems(count) }} />);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.click(screen.getByRole("checkbox", { name: "持ち物の候補も比較" }));
    return engine;
  }

  test("ちょうど64通り(なし + 63件)なら全部渡し、絞り込みの案内は出さない", async () => {
    const engine = await compareWithItems(63);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toHaveLength(64);
    });
    expect(screen.queryByText(itemsTruncatedNotice)).toBeNull();
  });

  test("64通りを超えるときは64通りに絞って渡し、絞り込んだことを画面に出す", async () => {
    const engine = await compareWithItems(64);
    await waitFor(() => {
      expect(lastRequest(engine).itemVariants).toHaveLength(64);
    });
    // 先頭は持ち物なし、続きはマスタの順のまま(並べ替え・間引きをしない)
    const sent = lastRequest(engine).itemVariants;
    expect(sent?.[0]).toBeNull();
    expect(sent?.[1]?.id).toBe("example-many-def-0");
    expect(sent?.at(-1)?.id).toBe("example-many-def-62");
    expect(screen.getByText(itemsTruncatedNotice)).toBeInTheDocument();
  });
});

// issue 271 / issue 270(Web レーン。ADR-0123): engine が正しく計算できない技・持ち物・特性を選んだとき、
// 数値は今までどおり出しつつ「この結果は正しく計算できていない可能性がある」印を出す。
// 置き場所・文言は iOS レーンの決定(docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、
// ADR-0501「P6-17」)に揃える:
//   - **全行に共通する印**(target・reason・id が同じ)は、結果の**先頭に1回**(role=status)だけ出す。
//     技由来の印は全行に付くことが多く、行ごとに出すと同じ文言が何度も並ぶため(iOS レーンの指摘)。
//     target で決め打ちせず、印の内容が全行にあるかどうかで判定する(splitUnsupportedMarks)。
//   - **一部の行だけにある印**(防御側の持ち物バリアントなどで行ごとに変わる印)は、その**行だけ**に出す。
//   - 色・アイコンだけに頼らない: 印は必ず文字(「未対応: <印>、<印>」、印は「<対象>「<名前>」(<理由>)」)で出す。
//     アイコン(⚠)は装飾として aria-hidden にする。
describe("未対応の印(issue 271 / issue 270)", () => {
  const multiHit = (moveId: string): UnsupportedMark => ({
    target: "move",
    reason: "multi_hit",
    id: moveId,
  });
  const attackerItemMark = (itemId: string): UnsupportedMark => ({
    target: "attacker_item",
    reason: "unsupported_effect",
    id: itemId,
  });
  const defenderAbilityMark = (abilityId: string): UnsupportedMark => ({
    target: "defender_ability",
    reason: "unsupported_effect",
    id: abilityId,
  });

  function firstItem(): Item {
    const item = master.items[0];
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    return item;
  }

  function firstAbility(): { id: string; nameJa: string } {
    const ability = master.abilities[0];
    if (ability === undefined) {
      throw new Error("例データに特性が無い");
    }
    return ability;
  }

  /** 行ごとの印を決めて画面を描き、結果の行を返す。 */
  async function renderWithMarks(marksByRow: ReadonlyArray<readonly UnsupportedMark[]>) {
    const rows = marksByRow.map((unsupported, index) =>
      bulkRow({
        preset: index === 0 ? "none" : "hp",
        presetLabel: index === 0 ? "無振り" : "H振り",
        unsupported,
      }),
    );
    const engine = createFakeEngine((request) =>
      ok({ defenderSpeciesKey: request.defenderSpecies.key, rows }),
    );
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    return resultItems();
  }

  test("印が無ければ何も出ない(正常系。今までの見た目を変えない)", async () => {
    const items = await renderWithMarks([[], []]);
    expect(items).toHaveLength(2);
    expect(screen.queryByTestId("unsupported-icon")).toBeNull();
    for (const item of items) {
      expect(within(item).queryByTestId("unsupported-icon")).toBeNull();
    }
  });

  test("一部の行だけにある印は、その行に「未対応: <対象>「<名前>」(<理由>)」の形で出る", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first, second] = await renderWithMarks([[multiHit(move.id)], []]);
    if (first === undefined || second === undefined) {
      throw new Error("結果が2行でない");
    }
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(multiHit(move.id), move.nameJa),
    ]);
    expect(within(first).getByText(expectedRowLabel)).toBeInTheDocument();
    // 印の無いもう一方の行には出ない(結果の先頭にも出ない。全行共通ではないため)
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  test("持ち物・特性の印も、どちら側の何が原因かが分かる文言で1行にまとまる", async () => {
    const item = firstItem();
    const ability = firstAbility();
    const itemMark = attackerItemMark(item.id);
    const abilityMark = defenderAbilityMark(ability.id);
    const [first] = await renderWithMarks([[itemMark, abilityMark], []]);
    if (first === undefined) {
      throw new Error("1行目が無い");
    }
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(itemMark, item.nameJa),
      unsupportedText.markLabel(abilityMark, ability.nameJa),
    ]);
    expect(within(first).getByText(expectedRowLabel)).toBeInTheDocument();
  });

  test("印はその行だけに出る(印の無い行には出さない)", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first, second] = await renderWithMarks([[], [multiHit(move.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("結果が2行でない");
    }
    expect(within(first).queryByTestId("unsupported-icon")).toBeNull();
    expect(within(second).getByTestId("unsupported-icon")).toBeInTheDocument();
    // %幅・確定数は今までどおり全行に出る(印が付いても数値を消さない)
    for (const row of [first, second]) {
      expect(within(row).getByText("72.1〜85.3%")).toBeInTheDocument();
    }
  });

  // iOS レーンの決定(DECISIONS.md 2026-09-25): 「全行(全候補)が持つ印は結果の上に1回、残りはその行だけ」。
  // target で決め打ちせず「全行にあるか」で決めるため、両方の行に同じ内容(target・reason・id)の印があれば、
  // その印は結果の先頭にまとめ、行には出さない(技の印が行ごとに5〜10回並ぶのを避けるため)。
  test("全行に共通する印は結果の先頭に1回だけ出て、どの行にも出ない", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first, second] = await renderWithMarks([[multiHit(move.id)], [multiHit(move.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("結果が2行でない");
    }
    const expectedNotice = unsupportedText.notice([
      unsupportedText.markLabel(multiHit(move.id), move.nameJa),
    ]);
    const notice = await screen.findByText(expectedNotice);
    expect(notice.closest('[role="status"]')).not.toBeNull();
    expect(screen.getAllByText(expectedNotice)).toHaveLength(1);
    // 共通の印は行には残らない(行に「未対応」のアイコンが出ない)
    expect(within(first).queryByTestId("unsupported-icon")).toBeNull();
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
    // 先頭の案内のアイコンの分だけ、画面全体では1つだけアイコンが出る
    expect(screen.getAllByTestId("unsupported-icon")).toHaveLength(1);
  });

  // 技由来の印(全行共通になりやすい)+ 持ち物バリアントで変わる印(一部の行だけ)が混在するケース。
  test("全行共通の印と行固有の印が混在するとき、共通は先頭に、残りはその行だけに出る", async () => {
    const move = firstMoveOf(speciesAt(0));
    const item = firstItem();
    const commonMark = multiHit(move.id);
    const rowOnlyMark = attackerItemMark(item.id);
    const [first, second] = await renderWithMarks([[commonMark, rowOnlyMark], [multiHit(move.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("結果が2行でない");
    }
    const expectedNotice = unsupportedText.notice([unsupportedText.markLabel(commonMark, move.nameJa)]);
    expect(await screen.findByText(expectedNotice)).toBeInTheDocument();
    // 行固有の印(itemMark)だけが1行目に残る。共通の印(multiHit)は1行目からは消える
    const expectedRowLabel = unsupportedText.rowLabel([unsupportedText.markLabel(rowOnlyMark, item.nameJa)]);
    expect(within(first).getByText(expectedRowLabel)).toBeInTheDocument();
    expect(within(first).queryByText(unsupportedText.markLabel(commonMark, move.nameJa))).toBeNull();
    // 2行目は行固有の印が無いので、行には何も出ない
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
  });

  test("装飾アイコンは aria-hidden=true で支援技術から隠す", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first] = await renderWithMarks([[multiHit(move.id)], []]);
    if (first === undefined) {
      throw new Error("1行目が無い");
    }
    const icon = within(first).getByTestId("unsupported-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  test("色・アイコンだけに頼らない: 印は支援技術にも読める文字で出す", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first] = await renderWithMarks([[multiHit(move.id)], []]);
    if (first === undefined) {
      throw new Error("1行目が無い");
    }
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(multiHit(move.id), move.nameJa),
    ]);
    const mark = within(first).getByText(expectedRowLabel);
    // 印の文字が aria-hidden の中(= 読み上げられない飾り)に入っていないこと
    expect(mark.closest('[aria-hidden="true"]')).toBeNull();
  });

  test("engine が返した印の順を変えない(並べ替え・重複除去をしない。ADR-0300 §8)", async () => {
    const move = firstMoveOf(speciesAt(0));
    const item = firstItem();
    const zeroPower: UnsupportedMark = { target: "move", reason: "zero_power", id: move.id };
    const itemMark = attackerItemMark(item.id);
    const [first] = await renderWithMarks([[multiHit(move.id), zeroPower, itemMark], []]);
    if (first === undefined) {
      throw new Error("1行目が無い");
    }
    const expected = [
      unsupportedText.markLabel(multiHit(move.id), move.nameJa),
      unsupportedText.markLabel(zeroPower, move.nameJa),
      unsupportedText.markLabel(itemMark, item.nameJa),
    ];
    expect(within(first).getByText(unsupportedText.rowLabel(expected))).toBeInTheDocument();
    // 行の読み上げ順(= DOM の順)が engine の並びと同じであること
    const rowText = first.textContent;
    let cursor = 0;
    for (const text of expected) {
      const index = rowText.indexOf(text, cursor);
      expect(index).toBeGreaterThanOrEqual(0);
      cursor = index + text.length;
    }
  });
});
