// P4-4: 逆算画面(docs/design.md「画面: 逆算」、requirements.md「調整の推定(逆算)」、ADR-0016 §7、ADR-0010 §R)。
// engine は fake(ADR-0016 §8)。マスタは架空の例データ(exampleMasterSource)で、特定の名前には依存しない。
// 確かめること:
//   - 「与えたダメージ」= side defender(自分 = 攻撃側、技は自分の learnset、自分の調整は攻撃側プリセット)
//     「受けたダメージ」= side attacker(自分 = 防御側で SP 0・補正なし、技は相手の learnset)
//   - 観測は整数%(1〜100)か HP の実点数(1 以上)。単位は行ごとに「%」「HP」で切り替え、不正な入力は engine を呼ばない
//   - 「観測を追加」「観測nを削除」、観測はすべての有効な行を順に送る(空行は送らない)
//   - 持ち物候補は reverseItemCandidates(効果データから)、maxCandidates は送らない
//   - 結果は engine の順のまま候補カードにし、性格クラス・持ち物・SP 範囲(全部)・目安・近い候補・%幅を出す
//   - 防御側は H32 の仮定を出す。計算中・エラー(role=alert)・古い応答の無視・変化技

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import { resolveAttackerPreset } from "../domain/attackerPresets";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import { NEUTRAL_NATURE, ZERO_SP, defaultAbility, toEngineSpecies } from "../domain/requests";
import { reverseItemCandidates } from "../domain/reverseItems";
import type { Move, ReverseRequest, ReverseResult } from "../engine/types";
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

function renderScreen(engine: FakeEngine = createFakeEngine()): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<ReverseScreen engine={engine} master={master} />);
  return { user, engine };
}

async function choosePair(user: UserEvent, mine: MasterSpecies, theirs: MasterSpecies): Promise<void> {
  await user.selectOptions(mySpeciesSelect(), mine.key);
  await user.selectOptions(theirSpeciesSelect(), theirs.key);
}

async function chooseReceived(user: UserEvent): Promise<void> {
  await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));
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
    await user.type(observationInput(1), "45");

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
    expect(request.itemCandidates).toEqual(reverseItemCandidates("defender", master.items, move));
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
    await user.type(observationInput(1), "45");

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
    await user.type(observationInput(1), "45");
    await waitFor(() => {
      expect(lastRequest(engine).known.item).toEqual(item);
    });
  });

  test("単位を HP に切り替えると damage の観測になる", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.click(within(unitGroup(1)).getByRole("radio", { name: "HP" }));
    await user.type(observationInput(1), "30");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ damage: 30 }]);
    });
  });
});

describe("受けたダメージ(side attacker)", () => {
  test("自分 = 防御側(SP 0・補正なし)、相手 = 攻撃側、相手の技、攻撃側の持ち物候補、HP の実点数で呼ぶ", async () => {
    const mine = speciesAt(0);
    const theirs = speciesAt(1);
    const move = firstMoveOf(theirs);
    const { user, engine } = renderScreen();
    await chooseReceived(user);
    expect(within(unitGroup(1)).getByRole("radio", { name: "HP" })).toBeChecked();
    await choosePair(user, mine, theirs);
    // 自分が防御側のときは攻撃側プリセットを出さない(P4-4 の既定は SP 0・補正なし)
    expect(screen.queryByRole("radiogroup", { name: "自分の調整" })).toBeNull();
    const options = within(moveSelect())
      .getAllByRole("option")
      .map((option) => option.getAttribute("value"));
    expect(options).toEqual(learnsetMoves(theirs, master.moves).map((candidate) => candidate.id));
    await user.type(observationInput(1), "60");

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
    expect(request.itemCandidates).toEqual(reverseItemCandidates("attacker", master.items, move));
    expect(request).not.toHaveProperty("maxCandidates");
  });

  test("対象側を切り替えると観測は空の1行に戻り、単位はその側の既定になる", async () => {
    const { user } = renderScreen();
    await user.type(observationInput(1), "45");
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
    await user.type(observationInput(1), "60");
    await waitFor(() => {
      expect(lastRequest(engine).move).toEqual(firstMoveOf(theirs));
    });
  });
});

/** 1文字ずつ打つと途中の値(「12.5」の「12」など)で計算が走るので、検証の確認は貼り付けで一度に入れる。 */
async function pasteInto(user: UserEvent, element: HTMLElement, text: string): Promise<void> {
  await user.click(element);
  await user.paste(text);
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
    await user.type(observationInput(1), "45");
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
    await user.type(observationInput(1), "45");
    await user.click(addObservationButton());
    expect(observationInput(2)).toHaveValue("");
    expect(within(unitGroup(2)).getByRole("radio", { name: "%" })).toBeChecked();
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }]);
    });

    await user.click(within(unitGroup(2)).getByRole("radio", { name: "HP" }));
    await user.type(observationInput(2), "50");
    await waitFor(() => {
      expect(lastRequest(engine).observations).toEqual([{ percent: 45 }, { damage: 50 }]);
    });
  });

  test("「観測2を削除」で行が消え、残りの観測で計算し直す", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.type(observationInput(1), "45");
    await user.click(addObservationButton());
    await user.type(observationInput(2), "50");
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
        itemId: "example-item-def",
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
    await user.type(observationInput(1), "45");
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
          itemId: "example-item-power",
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

  test("応答を待つ間は「計算中」を出す", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.type(observationInput(1), "45");
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
    await user.type(observationInput(1), "45");
    expect(await screen.findByRole("alert")).toHaveTextContent("観測の入力が不正: テスト");
  });

  test("古い応答が後から届いても、新しい観測の結果を上書きしない", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    const { user } = renderScreen(engine);
    await choosePair(user, speciesAt(0), speciesAt(1));
    await user.type(observationInput(1), "45");
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
    await user.type(observationInput(1), "45");
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
      await user.type(observationInput(1), "45");
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
      await user.type(observationInput(1), "50");

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
