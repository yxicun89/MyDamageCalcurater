// P4-4: 逆算画面(docs/design.md「画面: 逆算」、requirements.md「調整の推定(逆算)」、ADR-0300 §7、ADR-0010 §R)。
// engine は fake(ADR-0300 §8)。マスタは架空の例データ(exampleMasterSource)で、特定の名前には依存しない。
// 確かめること:
//   - 「与えたダメージ」= side defender(自分 = 攻撃側、技は自分の learnset、自分の調整は攻撃側プリセット)
//     「受けたダメージ」= side attacker(自分 = 防御側、技は相手の learnset、自分の調整は防御側プリセット。
//     issue #275 で「常に無振り固定・画面から変えられない」を直した。既定は無振りのまま)
//   - 観測は整数%(1〜100)か HP の実点数(1 以上)。単位は行ごとに「%」「HP」で切り替え、不正な入力は engine を呼ばない
//   - 「観測を追加」「観測nを削除」、観測はすべての有効な行を順に送る(空行は送らない)
//   - 持ち物候補は reverseItemCandidates(効果データから)、maxCandidates は送らない
//   - 結果は engine の順のまま候補カードにし、性格クラス・持ち物・SP 範囲(全部)・目安・近い候補・%幅を出す
//   - 全候補が観測と一致しない(exactCount 0)ときは role=status の案内を出し、候補一覧は残したまま
//     SP 範囲に「参考」の印を添える。%欄には「予測」のラベルを添える(issue #305)
//   - 防御側は H32 の仮定を出す。計算中・エラー(role=alert)・古い応答の無視・変化技

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterAll, beforeAll, describe, expect, test, vi } from "vitest";
import { resolveAttackerPreset } from "../domain/attackerPresets";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { NEUTRAL_NATURE, ZERO_SP, defaultAbility, toEngineSpecies } from "../domain/requests";
import { reverseItemCandidates } from "../domain/reverseItems";
import type { Item, Move, ReverseRequest, ReverseResult, UnsupportedMark } from "../engine/types";
import { reverseResultText, unsupportedText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  createDeferredReverseEngine,
  createFakeEngine,
  engineError,
  ok,
  reverseCandidate,
  type FakeEngine,
  type PendingReverse,
} from "../test/fakeEngine";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterAll(() => {
  vi.useRealTimers();
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

/** 変化技とダメージ技の両方を覚える種族(変化技の確認用)。 */
function speciesWithStatusMove(): { species: MasterSpecies; statusMove: Move } {
  for (const species of master.species) {
    const moves = learnsetMoves(species, master.moves);
    const statusMove = moves.find((move) => move.category === "status");
    if (statusMove !== undefined && moves.some((move) => move.category !== "status")) {
      return { species, statusMove };
    }
  }
  throw new Error("例データに変化技とダメージ技を両方覚える種族が無い");
}

function lastRequest(engine: FakeEngine): ReverseRequest {
  const request = engine.reverseRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcReverse が呼ばれていない");
  }
  return request;
}

const sideGroup = () => screen.getByRole("radiogroup", { name: "観測したダメージ" });
const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const myItemSelect = () => screen.getByRole("combobox", { name: "自分の持ち物" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });
const observationInput = (n: number) => screen.getByRole("textbox", { name: `観測${String(n)}` });
const unitGroup = (n: number) => screen.getByRole("radiogroup", { name: `観測${String(n)}の単位` });
const addObservationButton = () => screen.getByRole("button", { name: "観測を追加" });

/**
 * P4-18(issue 113): 観測の数値入力は 200ms の trailing debounce を挟むので、入力した後は
 * flushObservationDebounce() でその待ちを終わらせてから計算が始まる。タイマーは fake にし、
 * 実時間でも進める(shouldAdvanceTime)ことで waitFor / findBy* を今までどおり使う。
 * デバウンス自体の境界(何 ms で・何回呼ぶか)は ReverseScreen.debounce.test.tsx が決定的に確かめる。
 */
function renderScreen(engine: FakeEngine = createFakeEngine()): { user: UserEvent; engine: FakeEngine } {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  render(<ReverseScreen engine={engine} master={master} />);
  return { user, engine };
}

/** 観測の数値入力のデバウンスの待ちを終わらせる(issue 113)。 */
function flushObservationDebounce(): void {
  act(() => {
    vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
  });
}

/** 観測に数値を打ち、デバウンスの待ちを終わらせる(計算が始まるところまで進める)。 */
async function typeObservation(user: UserEvent, n: number, text: string): Promise<void> {
  await user.type(observationInput(n), text);
  flushObservationDebounce();
}

async function choosePair(user: UserEvent, mine: MasterSpecies, theirs: MasterSpecies): Promise<void> {
  await user.selectOptions(mySpeciesSelect(), mine.key);
  await user.selectOptions(theirSpeciesSelect(), theirs.key);
}

async function chooseReceived(user: UserEvent): Promise<void> {
  await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));
}

/** 自分側カードの「自分の調整」(与えたダメージ = 攻撃側プリセット、受けたダメージ = 防御側プリセット)。 */
const myPresetGroup = () => screen.getByRole("radiogroup", { name: "自分の調整" });

/** 「自分の調整」の選択肢の表示名を、画面に出ている順のまま返す。 */
function myPresetOptionLabels(): string[] {
  return within(myPresetGroup())
    .getAllByRole("radio")
    .map((radio) => (radio.closest("label")?.textContent ?? "").trim());
}

async function candidateCards(): Promise<HTMLElement[]> {
  const list = await screen.findByRole("list", { name: "推定結果" });
  return within(list).getAllByRole("listitem");
}

describe("初期表示", () => {
  test("既定は「与えたダメージ」、観測1行(単位は%)、engine は呼ばない", () => {
    const { engine } = renderScreen();
    expect(within(sideGroup()).getByRole("radio", { name: "与えたダメージ" })).toBeChecked();
    expect(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" })).not.toBeChecked();
    expect(mySpeciesSelect()).toBeInTheDocument();
    expect(theirSpeciesSelect()).toBeInTheDocument();
    expect(myItemSelect()).toBeInTheDocument();
    expect(moveSelect()).toBeInTheDocument();
    expect(observationInput(1)).toHaveValue("");
    expect(within(unitGroup(1)).getByRole("radio", { name: "%" })).toBeChecked();
    expect(screen.queryByRole("textbox", { name: "観測2" })).toBeNull();
    // 1行目は消せない(観測は1件以上)
    expect(screen.queryByRole("button", { name: "観測1を削除" })).toBeNull();
    expect(engine.reverseRequests).toHaveLength(0);
  });

  test("種族と技が揃っても、観測が空なら engine を呼ばない", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    expect(moveSelect()).toHaveValue(firstMoveOf(speciesAt(0)).id);
    expect(engine.reverseRequests).toHaveLength(0);
  });
});

describe("与えたダメージ(side defender)", () => {
  test("自分 = 攻撃側(無振り)、相手の種族、自分の技、持ち物候補、整数%の観測で calcReverse を呼ぶ", async () => {
    const mine = speciesAt(0);
    const theirs = speciesAt(1);
    const move = firstMoveOf(mine);
    const { user, engine } = renderScreen();
    await choosePair(user, mine, theirs);
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }]);
    });
    const request = lastRequest(engine);
    expect(request.side).toBe("defender");
    expect(request.format).toBe("single");
    expect(request.known.species).toEqual(toEngineSpecies(mine));
    expect(request.known.sp).toEqual(ZERO_SP);
    expect(request.known.nature).toEqual(NEUTRAL_NATURE);
    expect(request.known.item).toBeNull();
    expect(request.known.ability).toEqual(defaultAbility(mine, master.abilities));
    expect(request.unknownSpecies).toEqual(toEngineSpecies(theirs));
    expect(request.unknownSpecies).not.toHaveProperty("learnset");
    expect(request.move).toEqual(move);
    expect(request.typeChart).toBe(master.typeChart);
    expect(request.itemCandidates).toEqual(reverseItemCandidates("defender", master.items, move).candidates);
    expect(request.itemCandidates?.[0]).toBeNull();
    expect(request).not.toHaveProperty("maxCandidates");
  });

  test("技の候補は自分(攻撃側)の learnset", async () => {
    const mine = speciesAt(0);
    const { user } = renderScreen();
    await choosePair(user, mine, speciesAt(1));
    const options = within(moveSelect())
      .getAllByRole("option")
      .map((option) => option.getAttribute("value"));
    expect(options).toEqual(learnsetMoves(mine, master.moves).map((move) => move.id));
  });

  test("自分の調整(攻撃側プリセット)を選ぶと known の SP・性格に入る", async () => {
    const mine = speciesAt(0);
    const move = firstMoveOf(mine);
    const { user, engine } = renderScreen();
    await choosePair(user, mine, speciesAt(1));
    const presetGroup = screen.getByRole("radiogroup", { name: "自分の調整" });
    expect(within(presetGroup).getByRole("radio", { name: "無振り" })).toBeChecked();
    const fullLabel = move.category === "special" ? "C特化" : "A特化";
    await user.click(within(presetGroup).getByRole("radio", { name: fullLabel }));
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }]);
    });
    const expected = resolveAttackerPreset("x_full", move.category);
    expect(lastRequest(engine).known.sp).toEqual(expected.sp);
    expect(lastRequest(engine).known.nature).toEqual(expected.nature);
  });

  test("自分の持ち物を選ぶと known.item に入る", async () => {
    const [item] = master.items;
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.selectOptions(myItemSelect(), item.id);
    await typeObservation(user, 1, "45");
    await waitFor(() => {
      expect(lastRequest(engine).known.item).toEqual(item);
    });
  });

  test("単位を HP に切り替えると damage の観測になる", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.click(within(unitGroup(1)).getByRole("radio", { name: "HP" }));
    await typeObservation(user, 1, "30");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 30 }]);
    });
  });
});

describe("受けたダメージ(side attacker)", () => {
  test("自分 = 防御側(既定は無振り)、相手 = 攻撃側、相手の技、攻撃側の持ち物候補、HP の実点数で呼ぶ", async () => {
    const mine = speciesAt(0);
    const theirs = speciesAt(1);
    const move = firstMoveOf(theirs);
    const { user, engine } = renderScreen();
    await chooseReceived(user);
    expect(within(unitGroup(1)).getByRole("radio", { name: "HP" })).toBeChecked();
    await choosePair(user, mine, theirs);
    // issue #275: 自分が防御側のときは、自分の耐久(防御側プリセット)を選べる。既定は無振りで、
    // そのときのリクエストは issue #275 以前と同じ(SP 0・補正なし)になる(回帰。下の known の検証)。
    expect(within(myPresetGroup()).getByRole("radio", { name: "無振り" })).toBeChecked();
    const options = within(moveSelect())
      .getAllByRole("option")
      .map((option) => option.getAttribute("value"));
    expect(options).toEqual(learnsetMoves(theirs, master.moves).map((candidate) => candidate.id));
    await typeObservation(user, 1, "60");

    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 60 }]);
    });
    const request = lastRequest(engine);
    expect(request.side).toBe("attacker");
    expect(request.known.species).toEqual(toEngineSpecies(mine));
    expect(request.known.sp).toEqual(ZERO_SP);
    expect(request.known.nature).toEqual(NEUTRAL_NATURE);
    expect(request.unknownSpecies).toEqual(toEngineSpecies(theirs));
    expect(request.move).toEqual(move);
    expect(request.itemCandidates).toEqual(reverseItemCandidates("attacker", master.items, move).candidates);
    expect(request).not.toHaveProperty("maxCandidates");
  });

  test("対象側を切り替えると観測は空の1行に戻り、単位はその側の既定になる", async () => {
    const { user } = renderScreen();
    await typeObservation(user, 1, "45");
    await user.click(addObservationButton());
    await chooseReceived(user);
    expect(observationInput(1)).toHaveValue("");
    expect(screen.queryByRole("textbox", { name: "観測2" })).toBeNull();
    expect(within(unitGroup(1)).getByRole("radio", { name: "HP" })).toBeChecked();
    await user.click(within(sideGroup()).getByRole("radio", { name: "与えたダメージ" }));
    expect(within(unitGroup(1)).getByRole("radio", { name: "%" })).toBeChecked();
  });

  test("受けたダメージのとき、観測を追加した行の単位も既定で HP になる", async () => {
    const { user } = renderScreen();
    await chooseReceived(user);
    await user.click(addObservationButton());
    expect(within(unitGroup(2)).getByRole("radio", { name: "HP" })).toBeChecked();
  });

  test("対象側を切り替えると、技が新しい側(攻撃側)の learnset から選び直される", async () => {
    const mine = speciesAt(0);
    const theirs = speciesAt(1);
    const { user, engine } = renderScreen();
    await choosePair(user, mine, theirs);
    expect(moveSelect()).toHaveValue(firstMoveOf(mine).id);

    await chooseReceived(user);
    expect(moveSelect()).toHaveValue(firstMoveOf(theirs).id);

    // moveSelect() の表示値だけだと、選び直しをしなくても(前の技の id が新しい learnset に無いとき)
    // ネイティブの select が黙って先頭の option を表示してしまい、見分けが付かない。engine に渡る技
    // (moveId が実際に新しい learnset の中の id として解決されていること)で確かめる。
    await typeObservation(user, 1, "60");
    await waitFor(() => {
      expect(lastRequest(engine).move).toEqual(firstMoveOf(theirs));
    });
  });
});

// issue #275(重大度 high): 「受けたダメージ」のとき、自分(防御側)の耐久調整が無振り固定で画面からも
// 変えられなかったため、H・B(D)を振った自分で受けた値を入れると相手の A(C) が過大評価されていた。
// 自分側カードに防御側プリセット(ADR-0009 §1 のカタログ。domain/defenderPresets.ts)を出す。
// 選択肢は engine の DefaultDefenderPresets と同じく技の分類で絞る(物理 = B 系、特殊 = D 系、変化技 = none/hp)。
describe("受けたダメージ(side attacker)の自分の耐久(防御側プリセット。issue #275)", () => {
  /** 指定した分類の技を覚える種族の、その技(相手 = 攻撃側の learnset から選ぶ)。 */
  function moveOf(species: MasterSpecies, category: Move["category"]): Move {
    const move = learnsetMoves(species, master.moves).find((candidate) => candidate.category === category);
    if (move === undefined) {
      throw new Error(`${species.key} が ${category} の技を覚えない`);
    }
    return move;
  }

  /** 物理技と特殊技を両方覚える種族(技を替えて分類が変わる場面の確認用)。 */
  function speciesWithBothCategories(): MasterSpecies {
    for (const species of master.species) {
      const moves = learnsetMoves(species, master.moves);
      if (
        moves.some((move) => move.category === "physical") &&
        moves.some((move) => move.category === "special")
      ) {
        return species;
      }
    }
    throw new Error("例データに物理技と特殊技を両方覚える種族が無い");
  }

  /** 受けたダメージを選び、自分と相手を選んで、相手の技を1つ選ぶ。 */
  async function chooseReceivedWithMove(user: UserEvent, theirs: MasterSpecies, move: Move): Promise<void> {
    await chooseReceived(user);
    await choosePair(user, speciesAt(0), theirs);
    await user.selectOptions(moveSelect(), move.id);
  }

  test("選択肢は技の分類で絞る(特殊技は D 系、物理技は B 系。カタログ順)", async () => {
    const theirs = speciesWithBothCategories();
    const { user } = renderScreen();
    await chooseReceivedWithMove(user, theirs, moveOf(theirs, "special"));
    expect(myPresetOptionLabels()).toEqual(["無振り", "H振り", "H振り+D補正", "HD振り", "HD特化"]);

    await user.selectOptions(moveSelect(), moveOf(theirs, "physical").id);
    expect(myPresetOptionLabels()).toEqual(["無振り", "H振り", "H振り+B補正", "HB振り", "HB特化"]);
  });

  test("HD特化 を選ぶと known の SP・性格がその耐久調整になる(H32/D32・D上昇)", async () => {
    const theirs = speciesWithBothCategories();
    const { user, engine } = renderScreen();
    await chooseReceivedWithMove(user, theirs, moveOf(theirs, "special"));
    await user.click(within(myPresetGroup()).getByRole("radio", { name: "HD特化" }));
    await typeObservation(user, 1, "60");

    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 60 }]);
    });
    const request = lastRequest(engine);
    expect(request.side).toBe("attacker");
    expect(request.known.sp).toEqual({ ...ZERO_SP, hp: 32, spd: 32 });
    expect(request.known.nature).toEqual({ plus: "spd", minus: "atk" });
  });

  test("H振り+B補正 を選ぶと known の SP・性格がその耐久調整になる(H32・B上昇)", async () => {
    const theirs = speciesWithBothCategories();
    const { user, engine } = renderScreen();
    await chooseReceivedWithMove(user, theirs, moveOf(theirs, "physical"));
    await user.click(within(myPresetGroup()).getByRole("radio", { name: "H振り+B補正" }));
    await typeObservation(user, 1, "60");

    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 60 }]);
    });
    expect(lastRequest(engine).known.sp).toEqual({ ...ZERO_SP, hp: 32 });
    expect(lastRequest(engine).known.nature).toEqual({ plus: "def", minus: "atk" });
  });

  test("技の分類が変わると、対になる耐久調整に読み替えて選択を残す(HD特化 → HB特化)", async () => {
    const theirs = speciesWithBothCategories();
    const { user, engine } = renderScreen();
    await chooseReceivedWithMove(user, theirs, moveOf(theirs, "special"));
    await user.click(within(myPresetGroup()).getByRole("radio", { name: "HD特化" }));

    await user.selectOptions(moveSelect(), moveOf(theirs, "physical").id);
    expect(within(myPresetGroup()).getByRole("radio", { name: "HB特化" })).toBeChecked();

    await typeObservation(user, 1, "60");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 60 }]);
    });
    expect(lastRequest(engine).known.sp).toEqual({ ...ZERO_SP, hp: 32, def: 32 });
    expect(lastRequest(engine).known.nature).toEqual({ plus: "def", minus: "atk" });
  });

  test("変化技のときは選択肢が 無振り / H振り だけになり、選択は H振り に落ちる", async () => {
    const { species: theirs, statusMove } = speciesWithStatusMove();
    const { user } = renderScreen();
    await chooseReceivedWithMove(user, theirs, moveOf(theirs, "physical"));
    await user.click(within(myPresetGroup()).getByRole("radio", { name: "HB特化" }));

    await user.selectOptions(moveSelect(), statusMove.id);
    expect(myPresetOptionLabels()).toEqual(["無振り", "H振り"]);
    expect(within(myPresetGroup()).getByRole("radio", { name: "H振り" })).toBeChecked();
  });

  test("観測したダメージの側を切り替えると、自分の調整も攻撃側 ↔ 防御側で入れ替わる", async () => {
    const mine = speciesAt(0);
    const { user } = renderScreen();
    await choosePair(user, mine, speciesAt(1));
    // 与えたダメージ(自分 = 攻撃側)は攻撃側プリセットのまま(回帰)
    const attackerFullLabel = firstMoveOf(mine).category === "special" ? "C特化" : "A特化";
    expect(within(myPresetGroup()).getByRole("radio", { name: attackerFullLabel })).toBeInTheDocument();
    expect(within(myPresetGroup()).queryByRole("radio", { name: "H振り" })).toBeNull();

    await chooseReceived(user);
    expect(within(myPresetGroup()).getByRole("radio", { name: "H振り" })).toBeInTheDocument();
    expect(within(myPresetGroup()).queryByRole("radio", { name: attackerFullLabel })).toBeNull();

    await user.click(within(sideGroup()).getByRole("radio", { name: "与えたダメージ" }));
    expect(within(myPresetGroup()).getByRole("radio", { name: attackerFullLabel })).toBeInTheDocument();
  });
});

/** 1文字ずつ打つと途中の値(「12.5」の「12」など)で計算が走るので、検証の確認は貼り付けで一度に入れる。 */
async function pasteInto(user: UserEvent, element: HTMLElement, text: string): Promise<void> {
  await user.click(element);
  await user.paste(text);
  // 貼り付けも観測のテキストの変更なので、待ちを終わらせてから「engine を呼ばない」ことを確かめる
  // (待ちのせいで呼ばれていないだけ、を通してしまわないため)。
  flushObservationDebounce();
}

describe("観測の入力の検証", () => {
  test.each(["12.5", "101", "0", "-3", "abc"])(
    "%% の %j は不正として知らせ、engine を呼ばない",
    async (text) => {
      const { user, engine } = renderScreen();
      await choosePair(user, speciesAt(0), speciesAt(1));
      await pasteInto(user, observationInput(1), text);
      expect(observationInput(1)).toHaveAttribute("aria-invalid", "true");
      expect(screen.getByText("1〜100 の整数で入力してください")).toBeInTheDocument();
      expect(engine.reverseRequests).toHaveLength(0);
    },
  );

  test.each(["0", "1.5", "-3"])("HP の %j は不正として知らせ、engine を呼ばない", async (text) => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.click(within(unitGroup(1)).getByRole("radio", { name: "HP" }));
    await pasteInto(user, observationInput(1), text);
    expect(observationInput(1)).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("1 以上の整数で入力してください")).toBeInTheDocument();
    expect(engine.reverseRequests).toHaveLength(0);
  });

  test("有効な行があっても、不正な行が1つでもあれば engine を呼ばない", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    await user.click(addObservationButton());
    const callsBefore = engine.reverseRequests.length;
    await pasteInto(user, observationInput(2), "12.5");
    expect(observationInput(2)).toHaveAttribute("aria-invalid", "true");
    expect(observationInput(1)).not.toHaveAttribute("aria-invalid", "true");
    expect(engine.reverseRequests).toHaveLength(callsBefore);
    // 不正の間は古い候補を出さない
    expect(screen.queryByRole("list", { name: "推定結果" })).toBeNull();
  });
});

describe("観測を追加・削除", () => {
  test("「観測を追加」で行が増え、有効な観測を入力順に送る(空行は送らない)", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    await user.click(addObservationButton());
    expect(observationInput(2)).toHaveValue("");
    expect(within(unitGroup(2)).getByRole("radio", { name: "%" })).toBeChecked();
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }]);
    });

    await user.click(within(unitGroup(2)).getByRole("radio", { name: "HP" }));
    await typeObservation(user, 2, "50");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }, { damage: 50 }]);
    });
  });

  test("「観測2を削除」で行が消え、残りの観測で計算し直す", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    await user.click(addObservationButton());
    await typeObservation(user, 2, "50");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }, { percent: 50 }]);
    });

    await user.click(screen.getByRole("button", { name: "観測2を削除" }));
    expect(screen.queryByRole("textbox", { name: "観測2" })).toBeNull();
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }]);
    });
  });
});

describe("結果の表示", () => {
  const defenderResult: ReverseResult = {
    side: "defender",
    stat: "def",
    assumedHpSp: 32,
    exactCount: 2,
    candidates: [
      reverseCandidate({
        natureClass: "neutral",
        itemId: "",
        ranges: [{ min: 0, max: 3 }],
        spCount: 4,
        exact: true,
        minPercent: 40.2,
        maxPercent: 47.8,
      }),
      reverseCandidate({
        natureClass: "plus",
        nature: { plus: "def", minus: "atk" },
        itemId: "exampleitemdef",
        ranges: [
          { min: 4, max: 7 },
          { min: 9, max: 12 },
        ],
        spCount: 8,
        exact: true,
        minPercent: 38,
        maxPercent: 45.1,
      }),
      reverseCandidate({
        natureClass: "plus",
        nature: { plus: "def", minus: "atk" },
        itemId: "",
        ranges: [{ min: 32, max: 32 }],
        spCount: 1,
        exact: false,
        mismatch: 3,
        minPercent: 33.3,
        maxPercent: 39.9,
      }),
    ],
  };

  async function renderWithResult(result: ReverseResult): Promise<void> {
    const engine = createFakeEngine(undefined, () => ok(result));
    const { user } = renderScreen(engine);
    if (result.side === "attacker") {
      await chooseReceived(user);
    }
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
  }

  test("防御側は H32 の仮定を出し、候補を engine の順のままカードにする", async () => {
    await renderWithResult(defenderResult);
    const cards = await candidateCards();
    expect(screen.getByText(/H32 を仮定/)).toBeInTheDocument();
    expect(cards).toHaveLength(3);
    const [first, second, third] = cards;
    if (first === undefined || second === undefined || third === undefined) {
      throw new Error("候補カードが3件でない");
    }

    expect(within(first).getByText("補正なし")).toBeInTheDocument();
    expect(within(first).getByText("持ち物なし")).toBeInTheDocument();
    expect(within(first).getByText("B 0〜3")).toBeInTheDocument();
    expect(within(first).getByText(/H振り/)).toBeInTheDocument();
    expect(within(first).getByText("40.2〜47.8%")).toBeInTheDocument();
    expect(within(first).queryByText("近い候補")).toBeNull();

    expect(within(second).getByText("B上昇")).toBeInTheDocument();
    expect(within(second).getByText("テストぼうぎょだま")).toBeInTheDocument();
    // 区間は畳まずに全部出す(ADR-0010 §R3)
    expect(within(second).getByText("B 4〜7, 9〜12")).toBeInTheDocument();
    expect(within(second).getByText("38.0〜45.1%")).toBeInTheDocument();
    expect(within(second).queryByText(/HB特化|H振り/)).toBeNull();

    expect(within(third).getByText("B 32")).toBeInTheDocument();
    expect(within(third).getByText(/HB特化/)).toBeInTheDocument();
    expect(within(third).getByText("近い候補")).toBeInTheDocument();
  });

  test("攻撃側は仮定を出さず、A/C の表記と攻撃側の目安を使う", async () => {
    await renderWithResult({
      side: "attacker",
      stat: "spa",
      assumedHpSp: 0,
      exactCount: 1,
      candidates: [
        reverseCandidate({
          natureClass: "plus",
          nature: { plus: "spa", minus: "atk" },
          itemId: "exampleitempower",
          ranges: [{ min: 28, max: 32 }],
          spCount: 5,
        }),
        reverseCandidate({ natureClass: "neutral", ranges: [{ min: 0, max: 0 }], spCount: 1 }),
      ],
    });
    const [first, second] = await candidateCards();
    if (first === undefined || second === undefined) {
      throw new Error("候補カードが2件でない");
    }
    expect(screen.queryByText(/を仮定/)).toBeNull();
    expect(within(first).getByText("C上昇")).toBeInTheDocument();
    expect(within(first).getByText("テストちからのたま")).toBeInTheDocument();
    expect(within(first).getByText("C 28〜32")).toBeInTheDocument();
    expect(within(first).getByText(/C特化/)).toBeInTheDocument();
    expect(within(second).getByText("C 0")).toBeInTheDocument();
    expect(within(second).getByText(/無振り/)).toBeInTheDocument();
  });

  // issue #305: 観測を厳密に説明できる候補(exact)が1件も無いとき、その旨が分かるようにする。
  // 一致の判定そのものは engine が返した exact / exactCount をそのまま使う(TS で再計算・再判定しない。
  // ADR-0300 §8「Web は返ってきた値を加工せずに表示する」)。
  const noExactResult: ReverseResult = {
    ...defenderResult,
    exactCount: 0,
    candidates: defenderResult.candidates.map((candidate) => ({
      ...candidate,
      exact: false,
      mismatch: 3,
    })),
  };

  test("全候補が観測と一致しないときは role=status で理由の案内を出す(issue #305)", async () => {
    await renderWithResult(noExactResult);
    // critic指摘: role="status" は観測の上限の案内(L936付近)にも使われるので、role だけでなく
    // 文言でも絞り込む(将来 status が複数同時に出るテストを足しても複数マッチで落ちないように)。
    const notice = await screen.findByText(reverseResultText.noExactCandidateNotice);
    expect(notice.closest('[role="status"]')).not.toBeNull();
  });

  test("全候補が不一致でも候補一覧は消さず、件数分そのまま出す(issue #305)", async () => {
    await renderWithResult(noExactResult);
    const cards = await candidateCards();
    expect(cards).toHaveLength(noExactResult.candidates.length);
    const [first, second, third] = cards;
    if (first === undefined || second === undefined || third === undefined) {
      throw new Error("候補カードが3件でない");
    }
    // 性格クラス・持ち物・SP 範囲・目安は今までどおり出る(SP 範囲には「参考」の印が添わるので部分一致で見る)
    expect(within(first).getByText("補正なし")).toBeInTheDocument();
    expect(within(first).getByText("持ち物なし")).toBeInTheDocument();
    expect(within(first).getByText(/B 0〜3/)).toBeInTheDocument();
    expect(within(first).getByText(/H振り/)).toBeInTheDocument();
    expect(within(second).getByText("B上昇")).toBeInTheDocument();
    expect(within(second).getByText("テストぼうぎょだま")).toBeInTheDocument();
    expect(within(second).getByText(/B 4〜7, 9〜12/)).toBeInTheDocument();
    expect(within(third).getByText(/B 32/)).toBeInTheDocument();
    expect(within(third).getByText(/HB特化/)).toBeInTheDocument();
    // 個別の「近い候補」ラベルは今までどおり全件に付く(弱めない)
    expect(screen.getAllByText(reverseResultText.closeCandidateLabel)).toHaveLength(cards.length);
  });

  test("全候補が不一致のとき、SP 範囲は「参考」と分かる印を添える(issue #305)", async () => {
    await renderWithResult(noExactResult);
    const cards = await candidateCards();
    for (const card of cards) {
      expect(
        within(card).getByText(reverseResultText.referenceRangeLabel, { exact: false }),
      ).toBeInTheDocument();
    }
  });

  test("一致する候補が1件以上あるときは案内も「参考」の印も出さない(issue #305 の正常系)", async () => {
    await renderWithResult(defenderResult);
    expect(await candidateCards()).toHaveLength(3);
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByText(reverseResultText.noExactCandidateNotice)).toBeNull();
    expect(screen.queryByText(reverseResultText.referenceRangeLabel, { exact: false })).toBeNull();
  });

  test("候補が0件のときは全件不一致の案内を出さない(issue #305)", async () => {
    await renderWithResult({ ...defenderResult, exactCount: 0, candidates: [] });
    expect(await screen.findByRole("list", { name: "推定結果" })).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  test("%欄には「予測」のラベルを添える(issue #305)", async () => {
    await renderWithResult(defenderResult);
    const [first] = await candidateCards();
    if (first === undefined) {
      throw new Error("候補カードが無い");
    }
    expect(
      within(first).getByText(reverseResultText.predictedPercentLabel, { exact: false }),
    ).toBeInTheDocument();
    // 値そのものの表記は変えない(ラベルは別の要素に分ける。既存の完全一致の期待値を壊さないため)
    expect(within(first).getByText("40.2〜47.8%")).toBeInTheDocument();
  });

  test("応答を待つ間は「計算中」を出す", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    expect(await screen.findByText("計算中")).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "推定結果" })).toBeNull();

    const last = pending.at(-1);
    if (last === undefined) {
      throw new Error("calcReverse が呼ばれていない");
    }
    // resolve() は Promise を解決するだけで、その .then(setCompleted) はマイクロタスクとして後で走る。
    // act を async にして中で await することで、そのマイクロタスクの実行まで待ってから次のアサーションに進む
    // (同期の act だと .then が走る前に次の行に進んでしまい、アサーションが何も検証しなくなる)。
    await act(async () => {
      last.resolve(ok(defenderResult));
      await Promise.resolve();
    });
    expect(await candidateCards()).toHaveLength(3);
    expect(screen.queryByText("計算中")).toBeNull();
  });

  test("engine のエラーは role=alert で message を出す", async () => {
    const engine = createFakeEngine(undefined, () =>
      engineError("invalid_observation", "観測の入力が不正: テスト"),
    );
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    expect(await screen.findByRole("alert")).toHaveTextContent("観測の入力が不正: テスト");
  });

  test("古い応答が後から届いても、新しい観測の結果を上書きしない", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    // P4-18(issue 113): 打っている途中の「4」ではもう計算しないので、待ちを2回終わらせて
    // 「4」の計算と「45」の計算を別々に起こす(古い応答を作る条件は今までと同じ: 先に送った要求が後で届く)。
    await typeObservation(user, 1, "4");
    await typeObservation(user, 1, "5");
    await waitFor(() => {
      expect(pending.at(-1)?.request.observations).toEqual([{ percent: 45 }]);
    });
    const stale = pending.find((entry) => entry.request.observations[0]?.percent === 4);
    const fresh = pending.at(-1);
    if (stale === undefined || fresh === undefined) {
      throw new Error("「4」と「45」の2回の呼び出しが要る");
    }
    const freshResult: ReverseResult = {
      ...defenderResult,
      candidates: [reverseCandidate({ ranges: [{ min: 20, max: 23 }] })],
    };
    const staleResult: ReverseResult = {
      ...defenderResult,
      candidates: [reverseCandidate({ ranges: [{ min: 1, max: 1 }] })],
    };
    // どちらの resolve も、その .then(setCompleted) のマイクロタスクの実行まで act 内で待つ
    // (同期の act だと .then 未実行のままアサーションに進み、古い応答を無視できているかを
    // 実際には確認しないテストになってしまう。cancelled ガードを外すと、この await のおかげで
    // 2回目の act の中で stale の setCompleted が実際に走り、以下のアサーションが失敗する)。
    await act(async () => {
      fresh.resolve(ok(freshResult));
      await Promise.resolve();
    });
    expect(await screen.findByText("B 20〜23")).toBeInTheDocument();
    await act(async () => {
      stale.resolve(ok(staleResult));
      await Promise.resolve();
    });
    expect(screen.getByText("B 20〜23")).toBeInTheDocument();
    expect(screen.queryByText("B 1")).toBeNull();
  });

  test("変化技を選ぶと「変化技はダメージを計算しません」を出し、engine を呼ばない", async () => {
    const { species, statusMove } = speciesWithStatusMove();
    const { user, engine } = renderScreen();
    await choosePair(user, species, speciesAt(1));
    await user.selectOptions(moveSelect(), statusMove.id);
    await typeObservation(user, 1, "45");
    expect(screen.getByText("変化技はダメージを計算しません")).toBeInTheDocument();
    expect(engine.reverseRequests).toHaveLength(0);
  });

  // P4-4 critic 指摘: CompletedCalc の比較(観測・プリセット・技・相手の種族)が古い候補を正しく消すことの確認。
  // この比較を side だけに弱めると、下のどのテストも(観測を変えても古い候補が残ってしまうため)失敗するはず。
  describe("結果が出た後に入力を変えると、古い候補を消して「計算中」に戻る", () => {
    async function showInitialResult(
      user: UserEvent,
      pending: PendingReverse[],
      mine: MasterSpecies,
      theirs: MasterSpecies,
    ): Promise<void> {
      await choosePair(user, mine, theirs);
      await typeObservation(user, 1, "45");
      const first = pending.at(-1);
      if (first === undefined) {
        throw new Error("calcReverse が呼ばれていない");
      }
      await act(async () => {
        first.resolve(ok(defenderResult));
        await Promise.resolve();
      });
      expect(await candidateCards()).toHaveLength(3);
    }

    function expectBackToLoading(): Promise<HTMLElement> {
      return screen.findByText("計算中").then((notice) => {
        expect(screen.queryByRole("list", { name: "推定結果" })).toBeNull();
        return notice;
      });
    }

    test("観測を変えると", async () => {
      const { engine, pending } = createDeferredReverseEngine();
      const { user } = renderScreen(engine);
      await showInitialResult(user, pending, speciesAt(0), speciesAt(1));

      await user.clear(observationInput(1));
      await typeObservation(user, 1, "50");

      expect(await expectBackToLoading()).toBeInTheDocument();
    });

    test("自分の調整(プリセット)を変えると", async () => {
      const mine = speciesAt(0);
      const { engine, pending } = createDeferredReverseEngine();
      const { user } = renderScreen(engine);
      await showInitialResult(user, pending, mine, speciesAt(1));

      const presetGroup = screen.getByRole("radiogroup", { name: "自分の調整" });
      const fullLabel = firstMoveOf(mine).category === "special" ? "C特化" : "A特化";
      await user.click(within(presetGroup).getByRole("radio", { name: fullLabel }));

      expect(await expectBackToLoading()).toBeInTheDocument();
    });

    // issue #275: 受けたダメージのときの自分の耐久(防御側プリセット)も計算のやり直しの対象
    // (CompletedReverse の比較に入れる)。入れ忘れると古い候補が残る。
    test("受けたダメージで自分の耐久(防御側プリセット)を変えると", async () => {
      const { engine, pending } = createDeferredReverseEngine();
      const { user } = renderScreen(engine);
      await chooseReceived(user);
      await choosePair(user, speciesAt(0), speciesAt(1));
      await typeObservation(user, 1, "60");
      const first = pending.at(-1);
      if (first === undefined) {
        throw new Error("calcReverse が呼ばれていない");
      }
      await act(async () => {
        first.resolve(ok(defenderResult));
        await Promise.resolve();
      });
      expect(await candidateCards()).toHaveLength(3);

      await user.click(within(myPresetGroup()).getByRole("radio", { name: "H振り" }));

      expect(await expectBackToLoading()).toBeInTheDocument();
    });

    test("技を変えると", async () => {
      const mine = speciesAt(0);
      const damagingMoves = learnsetMoves(mine, master.moves).filter((move) => move.category !== "status");
      const secondMove = damagingMoves[1];
      if (secondMove === undefined) {
        throw new Error("例データに2つ以上のダメージ技を覚える種族が要る(example-fire を想定)");
      }
      const { engine, pending } = createDeferredReverseEngine();
      const { user } = renderScreen(engine);
      await showInitialResult(user, pending, mine, speciesAt(1));

      await user.selectOptions(moveSelect(), secondMove.id);

      expect(await expectBackToLoading()).toBeInTheDocument();
    });

    test("相手の種族を変えると", async () => {
      const { engine, pending } = createDeferredReverseEngine();
      const { user } = renderScreen(engine);
      await showInitialResult(user, pending, speciesAt(0), speciesAt(1));

      await user.selectOptions(theirSpeciesSelect(), speciesAt(2).key);

      expect(await expectBackToLoading()).toBeInTheDocument();
    });
  });
});

// P4-19(issue #110、ADR-0208、DECISIONS.md 2026-09-23): 候補・観測の件数上限。
// 期待値(16件・64通り)は api/openapi.yaml の maxItems から直接書く(実装の写しにしない)。
// 定数とのずれは domain/requestLimits.test.ts が openapi.yaml を読んで検出する。
describe("件数の上限(issue #110)", () => {
  const observationLimitReason = "観測は16件までです。追加するには、どれかの行を削除してください";
  const itemsTruncatedNotice = "持ち物の候補が多いため、先頭から64通りまでで計算しています";

  /** 観測が count 行になるまで「観測を追加」を押す(初期状態は1行)。 */
  async function addObservationsUntil(user: UserEvent, count: number): Promise<void> {
    for (let rows = 1; rows < count; rows += 1) {
      await user.click(addObservationButton());
    }
  }

  test("15件(上限-1)までは追加でき、理由は出ない", async () => {
    const { user } = renderScreen();
    await addObservationsUntil(user, 15);
    expect(screen.getByRole("textbox", { name: "観測15" })).toBeInTheDocument();
    expect(addObservationButton()).toBeEnabled();
    expect(screen.queryByText(observationLimitReason)).toBeNull();
  });

  test("16件(上限)に達すると「観測を追加」が無効になり、理由がスクリーンリーダーにも伝わる", async () => {
    const { user } = renderScreen();
    await addObservationsUntil(user, 16);
    expect(screen.getByRole("textbox", { name: "観測16" })).toBeInTheDocument();
    expect(addObservationButton()).toBeDisabled();

    // 理由は live region(role=status)で読み上げられ、ボタンからも aria-describedby で指す
    const reason = screen.getByRole("status");
    expect(reason).toHaveTextContent(observationLimitReason);
    expect(addObservationButton()).toHaveAttribute("aria-describedby", reason.id);
  });

  test("上限に達した後は行が増えない(上限+1 にならない)", async () => {
    const { user } = renderScreen();
    await addObservationsUntil(user, 16);
    await user.click(addObservationButton());
    expect(screen.queryByRole("textbox", { name: "観測17" })).toBeNull();
  });

  test("1行削除すると再び追加でき、理由も消える", async () => {
    const { user } = renderScreen();
    await addObservationsUntil(user, 16);
    await user.click(screen.getByRole("button", { name: "観測16を削除" }));
    expect(screen.queryByRole("textbox", { name: "観測16" })).toBeNull();
    expect(addObservationButton()).toBeEnabled();
    expect(screen.queryByText(observationLimitReason)).toBeNull();
  });

  /** 物理・特殊のどちらの技でも防御側の候補になる架空の持ち物を count 件(マスタの順)。 */
  function defenseItems(count: number): Item[] {
    return Array.from({ length: count }, (_value, index) => ({
      id: `example-many-def-${String(index)}`,
      nameJa: `テスト防御${String(index)}`,
      effect: { statMods: { def: 6144, spd: 6144 } },
    }));
  }

  function renderWithItems(count: number): { user: UserEvent; engine: FakeEngine } {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const engine = createFakeEngine();
    render(<ReverseScreen engine={engine} master={{ ...master, items: defenseItems(count) }} />);
    return { user, engine };
  }

  async function observeOnce(user: UserEvent): Promise<void> {
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
  }

  test("持ち物候補がちょうど64通り(なし + 63件)なら全部送り、絞り込みの案内は出さない", async () => {
    const { user, engine } = renderWithItems(63);
    await observeOnce(user);
    await waitFor(() => {
      expect(lastRequest(engine).itemCandidates).toHaveLength(64);
    });
    expect(screen.queryByText(itemsTruncatedNotice)).toBeNull();
  });

  test("持ち物候補が64通りを超えるときは64通りに絞って送り、絞り込んだことを画面に出す", async () => {
    const { user, engine } = renderWithItems(64);
    await observeOnce(user);
    await waitFor(() => {
      expect(lastRequest(engine).itemCandidates).toHaveLength(64);
    });
    // 先頭は持ち物なし、続きはマスタの順のまま(並べ替え・間引きをしない)
    const sent = lastRequest(engine).itemCandidates;
    expect(sent?.[0]).toBeNull();
    expect(sent?.[1]?.id).toBe("example-many-def-0");
    expect(sent?.at(-1)?.id).toBe("example-many-def-62");
    expect(screen.getByText(itemsTruncatedNotice)).toBeInTheDocument();
  });
});

// issue 271 / issue 270(Web レーン。ADR-0123): engine が正しく計算できない技・持ち物・特性のときは、
// 候補と予測%は今までどおり出しつつ「この結果は正しく計算できていない可能性がある」印を出す。
// 置き場所・文言は iOS レーンの決定(docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、
// ADR-0501「P6-17」)に揃える(CalcScreen.test.tsx の同名 describe と同じ考え方):
//   - **全候補に共通する印**(target・reason・id が同じ)は、候補一覧の**先頭に1回**(role=status)だけ出す。
//   - **一部の候補だけにある印**(候補ごとに持ち物が違う等)は、その**候補カードだけ**に出す
//     (issue 305 の「近い候補」「参考」と同じ並びに置く)。
describe("未対応の印(issue 271 / issue 270)", () => {
  const moveMark = (moveId: string): UnsupportedMark => ({
    target: "move",
    reason: "variable_power",
    id: moveId,
  });
  const itemMark = (itemId: string): UnsupportedMark => ({
    target: "defender_item",
    reason: "unsupported_effect",
    id: itemId,
  });

  function firstItem(): Item {
    const item = master.items[0];
    if (item === undefined) {
      throw new Error("例データに持ち物が無い");
    }
    return item;
  }

  /** 候補ごとの印を決めて画面を描く(与えたダメージ = side defender)。 */
  async function renderWithMarks(marksByCandidate: ReadonlyArray<readonly UnsupportedMark[]>) {
    const result: ReverseResult = {
      side: "defender",
      stat: "def",
      assumedHpSp: 32,
      exactCount: marksByCandidate.length,
      candidates: marksByCandidate.map((unsupported, index) =>
        reverseCandidate({
          natureClass: index === 0 ? "neutral" : "plus",
          nature: index === 0 ? { plus: "", minus: "" } : { plus: "def", minus: "atk" },
          unsupported,
        }),
      ),
    };
    const engine = createFakeEngine(undefined, () => ok(result));
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");
    return candidateCards();
  }

  test("印が無ければ何も出ない(正常系。今までの見た目を変えない)", async () => {
    const cards = await renderWithMarks([[], []]);
    expect(cards).toHaveLength(2);
    expect(screen.queryByTestId("unsupported-icon")).toBeNull();
  });

  test("一部の候補だけにある印は、その候補に「未対応: <対象>「<名前>」(<理由>)」の形で出る", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first, second] = await renderWithMarks([[moveMark(move.id)], []]);
    if (first === undefined || second === undefined) {
      throw new Error("候補カードが2件でない");
    }
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(moveMark(move.id), move.nameJa),
    ]);
    expect(within(first).getByText(expectedRowLabel)).toBeInTheDocument();
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  test("持ち物の印は、その候補にだけ出る(候補ごとに持ち物が違うため)", async () => {
    const item = firstItem();
    const [first, second] = await renderWithMarks([[], [itemMark(item.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("候補カードが2件でない");
    }
    expect(within(first).queryByTestId("unsupported-icon")).toBeNull();
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(itemMark(item.id), item.nameJa),
    ]);
    expect(within(second).getByText(expectedRowLabel)).toBeInTheDocument();
    // 予測%・SP 範囲は今までどおり両方の候補に出る(印が付いても候補を消さない)
    for (const card of [first, second]) {
      expect(within(card).getByText("40.2〜47.8%")).toBeInTheDocument();
    }
  });

  // iOS レーンの決定(DECISIONS.md 2026-09-25): 全候補に共通する印は先頭に1回、残りはその候補だけ。
  test("全候補に共通する印は候補一覧の先頭に1回だけ出て、どの候補にも出ない", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first, second] = await renderWithMarks([[moveMark(move.id)], [moveMark(move.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("候補カードが2件でない");
    }
    const expectedNotice = unsupportedText.notice([
      unsupportedText.markLabel(moveMark(move.id), move.nameJa),
    ]);
    const notice = await screen.findByText(expectedNotice);
    expect(notice.closest('[role="status"]')).not.toBeNull();
    expect(screen.getAllByText(expectedNotice)).toHaveLength(1);
    expect(within(first).queryByTestId("unsupported-icon")).toBeNull();
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
    expect(screen.getAllByTestId("unsupported-icon")).toHaveLength(1);
  });

  // 技由来の印(全候補共通になりやすい)+ 持ち物バリアントで変わる印(一部の候補だけ)が混在するケース。
  test("全候補共通の印と候補固有の印が混在するとき、共通は先頭に、残りはその候補だけに出る", async () => {
    const move = firstMoveOf(speciesAt(0));
    const item = firstItem();
    const commonMark = moveMark(move.id);
    const candidateOnlyMark = itemMark(item.id);
    const [first, second] = await renderWithMarks([[commonMark, candidateOnlyMark], [moveMark(move.id)]]);
    if (first === undefined || second === undefined) {
      throw new Error("候補カードが2件でない");
    }
    const expectedNotice = unsupportedText.notice([unsupportedText.markLabel(commonMark, move.nameJa)]);
    expect(await screen.findByText(expectedNotice)).toBeInTheDocument();
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(candidateOnlyMark, item.nameJa),
    ]);
    expect(within(first).getByText(expectedRowLabel)).toBeInTheDocument();
    expect(within(first).queryByText(unsupportedText.markLabel(commonMark, move.nameJa))).toBeNull();
    expect(within(second).queryByTestId("unsupported-icon")).toBeNull();
  });

  test("装飾アイコンは aria-hidden=true で支援技術から隠す", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first] = await renderWithMarks([[moveMark(move.id)], []]);
    if (first === undefined) {
      throw new Error("候補カードが無い");
    }
    const icon = within(first).getByTestId("unsupported-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  test("色・アイコンだけに頼らない: 印は支援技術にも読める文字で出す", async () => {
    const move = firstMoveOf(speciesAt(0));
    const [first] = await renderWithMarks([[moveMark(move.id)], []]);
    if (first === undefined) {
      throw new Error("候補カードが無い");
    }
    const expectedRowLabel = unsupportedText.rowLabel([
      unsupportedText.markLabel(moveMark(move.id), move.nameJa),
    ]);
    const mark = within(first).getByText(expectedRowLabel);
    expect(mark.closest('[aria-hidden="true"]')).toBeNull();
  });

  // critic 指摘: issue #305 の noExactCandidateNotice(role=status)と、本タスクの unsupportedText.notice
  // (role=status)が同時に出るケース。両方が独立した role=status の要素として共存することを固定する。
  test("issue #305 の全候補不一致の案内と、未対応の案内は同時に出せる(両方 role=status)", async () => {
    const move = firstMoveOf(speciesAt(0));
    const commonMark = moveMark(move.id);
    const result: ReverseResult = {
      side: "defender",
      stat: "def",
      assumedHpSp: 32,
      exactCount: 0,
      candidates: [
        reverseCandidate({ natureClass: "neutral", exact: false, mismatch: 3, unsupported: [commonMark] }),
        reverseCandidate({
          natureClass: "plus",
          nature: { plus: "def", minus: "atk" },
          exact: false,
          mismatch: 3,
          unsupported: [moveMark(move.id)],
        }),
      ],
    };
    const engine = createFakeEngine(undefined, () => ok(result));
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await typeObservation(user, 1, "45");

    const noExactNotice = await screen.findByText(reverseResultText.noExactCandidateNotice);
    const expectedUnsupportedNotice = unsupportedText.notice([
      unsupportedText.markLabel(commonMark, move.nameJa),
    ]);
    const unsupportedNotice = screen.getByText(expectedUnsupportedNotice);

    expect(noExactNotice.closest('[role="status"]')).not.toBeNull();
    expect(unsupportedNotice.closest('[role="status"]')).not.toBeNull();
    // 別々の要素として独立に存在する(どちらかがもう片方を上書き・吸収していない)
    expect(noExactNotice).not.toBe(unsupportedNotice);
    expect(screen.getAllByRole("status")).toHaveLength(2);
  });
});
