// ADR-0144 §3(I-web-13): 計算画面の「対戦の状態」(攻撃側・防御側の残りHP・多段の回数)。engine は fake。
// 仕様の正: docs/ai-shared/decisions/2026-10-10-data-move-mechanisms-stage3.md の Web レーンへの依頼。
// 確かめること:
//   B-1 既定は閉じていて、何も入れなければ従来と同じ(1対1の計算を呼ばない。一括の要求に battleState は無い)
//   B-2 残りHPを入れると各行が battleState つきの1対1の計算になる。一括の要求は従来と同じ。結果の近くに前提の状態を出す
//   B-3 満タン(最大と同じ値)・空に戻す・範囲外は送らない(範囲外は欄の下に誤り。計算も送らない)
//   B-4 回数は範囲の多段技のときだけ出す。既定は送らない。技を変えたら既定に戻る
//   B-5 最大HPが下がって残りHPが最大を超えたら、最大に合わせて案内を出す
//   B-6 お気に入りの保存に battleState が入り、復元で欄・要求に戻る

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import type { Move } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { bulkResultFor, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

type Favorite = components["schemas"]["Favorite"];

const PLAIN = "examplemovetackle";
const MULTI = "examplemovemultihit";
const MULTI_FIXED = "examplemovefixedhit";

let master: MasterData;
let attacker: MasterSpecies;
let defender: MasterSpecies;
let lowAttacker: MasterSpecies;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const tackle = base.moves.find((move) => move.id === PLAIN);
  const [first, second, third] = base.species;
  if (tackle === undefined || first === undefined || second === undefined || third === undefined) {
    throw new Error("例データが足りない");
  }
  const multi: Move = {
    ...tackle,
    id: MULTI,
    nameJa: "れんぞく",
    mechanismParams: { multiHit: { min: 2, max: 5 } },
  };
  const fixed: Move = {
    ...tackle,
    id: MULTI_FIXED,
    nameJa: "にかいぎり",
    mechanismParams: { multiHit: { min: 2, max: 2 } },
  };
  attacker = { ...first, learnset: [PLAIN, MULTI, MULTI_FIXED] };
  // 防御側の最大 HP の変更を確かめるため、種族値の HP が違う2種族にする。
  defender = { ...second, baseStats: { ...second.baseStats, hp: 100 } };
  lowAttacker = { ...third, baseStats: { ...third.baseStats, hp: 30 }, learnset: [PLAIN] };
  master = {
    ...base,
    species: [attacker, defender, lowAttacker, ...base.species.slice(3)],
    moves: [...base.moves, multi, fixed],
  };
});

const attackerMax = () => attacker.baseStats.hp + 75;
/** fake の calc / 一括の結果の defenderHP(行の最大 HP)。 */
const FAKE_ROW_HP = 88;

async function start(
  engine: FakeEngine = createFakeEngine(),
  moveId: string = PLAIN,
): Promise<{ user: UserEvent; engine: FakeEngine }> {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "技" }), moveId);
  await screen.findByRole("list", { name: "計算結果" });
  return { user, engine };
}

const toggle = () => screen.getByRole("button", { name: /^対戦の状態/ });
const defenderHp = () => screen.getByRole("textbox", { name: "防御側の残りHP" });
const attackerHp = () => screen.getByRole("textbox", { name: "攻撃側の残りHP" });

/** 1回の変更として入力する(1文字ずつ打つと途中の値で計算が走るため)。 */
async function enter(user: UserEvent, field: HTMLElement, text: string): Promise<void> {
  await user.click(field);
  await user.paste(text);
}

async function open(user: UserEvent): Promise<void> {
  await user.click(toggle());
  expect(toggle()).toHaveAttribute("aria-expanded", "true");
}

describe("B-1 既定", () => {
  test("既定は閉じていて、1対1の計算を呼ばない。一括の要求に battleState は無い", async () => {
    const { engine } = await start();
    expect(toggle()).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("textbox", { name: "防御側の残りHP" })).toBeNull();
    expect(engine.calcRequests).toHaveLength(0);
    for (const request of engine.bulkRequests) {
      expect(Object.keys(request)).not.toContain("battleState");
    }
  });

  test("開くと2つの欄に補足と「/ 最大」が出る(空なら満タン)", async () => {
    const { user } = await start();
    await open(user);
    expect(attackerHp()).toHaveValue("");
    expect(screen.getByText("空なら満タンで計算します")).toBeInTheDocument();
    expect(screen.getByText(`/ ${attackerMax()}`)).toBeInTheDocument();
    expect(screen.getByText("%")).toBeInTheDocument();
    expect(
      screen.getByText("割合(%)で入力します。確定数は残りHPから数えます(%表示は最大HPに対する値のまま)"),
    ).toBeInTheDocument();
  });
});

describe("B-2 残りHPを入れる", () => {
  test("各行が battleState つきの1対1の計算になり、前提の状態を結果の近くに出す", async () => {
    const { user, engine } = await start();
    const bulkCount = engine.bulkRequests.length;
    await open(user);
    await enter(user, defenderHp(), "50");
    await waitFor(() => {
      expect(engine.calcRequests.length).toBeGreaterThanOrEqual(5);
    });
    expect(engine.calcRequests.at(-1)?.battleState).toEqual({ defenderCurrentHp: FAKE_ROW_HP / 2 });
    expect(engine.calcRequests.at(-1)?.move.id).toBe(PLAIN);
    // 入力の途中(「5」)でも計算が走るので、最後の1回の一括の要求を見る
    expect(engine.bulkRequests.length).toBeGreaterThan(bulkCount);
    for (const request of engine.bulkRequests) {
      expect(Object.keys(request)).not.toContain("battleState");
    }
    expect(await screen.findByText("対戦の状態: 防御側 HP 50%")).toBeInTheDocument();
    expect(await screen.findAllByRole("listitem")).not.toHaveLength(0);
  });

  test("攻撃側・防御側の両方を入れると両方を送り、攻撃側は割合を横に出す(小数第1位・切り捨て)", async () => {
    const { user, engine } = await start();
    await open(user);
    await enter(user, attackerHp(), "1");
    await enter(user, defenderHp(), "70");
    await waitFor(() => {
      expect(engine.calcRequests.at(-1)?.battleState).toEqual({
        attackerCurrentHp: 1,
        defenderCurrentHp: Math.floor((FAKE_ROW_HP * 70) / 100),
      });
    });
    const expectedPercent = `${(Math.floor((1 * 1000) / attackerMax()) / 10).toFixed(1)}%`;
    expect(screen.getByText(expectedPercent)).toBeInTheDocument();
  });

  test("設定中は閉じていても目印が出る", async () => {
    const { user } = await start();
    await open(user);
    await enter(user, defenderHp(), "50");
    await user.click(toggle());
    expect(toggle()).toHaveTextContent("(設定中)");
  });
});

describe("B-3 満タン・空・範囲外", () => {
  test("最大と同じ値(満タン)は送らない。空に戻すと1対1の計算をやめる", async () => {
    const { user, engine } = await start();
    await open(user);
    await enter(user, defenderHp(), "100");
    await screen.findByRole("list", { name: "計算結果" });
    expect(engine.calcRequests).toHaveLength(0);
    await user.clear(defenderHp());
    await enter(user, defenderHp(), "50");
    await waitFor(() => {
      expect(engine.calcRequests.length).toBeGreaterThan(0);
    });
    await user.clear(defenderHp());
    const callsBefore = engine.calcRequests.length;
    await screen.findByRole("list", { name: "計算結果" });
    expect(engine.calcRequests).toHaveLength(callsBefore);
    expect(screen.queryByText(/^対戦の状態:/)).toBeNull();
  });

  test.each(["0", "-1", "abc", "1.5"])("範囲外(%j)は欄の下に誤りを出し、計算を送らない", async (text) => {
    const { user, engine } = await start();
    await open(user);
    const bulkBefore = engine.bulkRequests.length;
    await enter(user, defenderHp(), text);
    expect(await screen.findByRole("alert")).toHaveTextContent("1〜100で入力してください");
    expect(defenderHp()).toHaveAttribute("aria-invalid", "true");
    expect(engine.calcRequests).toHaveLength(0);
    // 範囲外の間は結果を出さない
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
    expect(engine.bulkRequests.length).toBe(bulkBefore);
  });

  test("最大を超える値も誤り", async () => {
    const { user, engine } = await start();
    await open(user);
    await enter(user, attackerHp(), String(attackerMax() + 1));
    expect(await screen.findByRole("alert")).toHaveTextContent(`1〜${attackerMax()}で入力してください`);
    expect(engine.calcRequests).toHaveLength(0);
  });
});

describe("B-4 多段の回数", () => {
  test("範囲の多段技のときだけ「回数」を出す(固定回数・多段でない技では出さない)", async () => {
    const { user } = await start();
    expect(screen.queryByRole("combobox", { name: "回数" })).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: "技" }), MULTI_FIXED);
    expect(screen.queryByRole("combobox", { name: "回数" })).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: "技" }), MULTI);
    const hits = screen.getByRole("combobox", { name: "回数" });
    expect(
      within(hits)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["既定(通常 3 回/スキルリンク等 5 回)", "2 回", "3 回", "4 回", "5 回"]);
    expect(hits).toHaveValue("");
  });

  test("既定は送らない。選ぶと hits を送り、技を変えると既定に戻る", async () => {
    const { user, engine } = await start(createFakeEngine(), MULTI);
    expect(engine.calcRequests).toHaveLength(0);
    await user.selectOptions(screen.getByRole("combobox", { name: "回数" }), "4");
    await waitFor(() => {
      expect(engine.calcRequests.at(-1)?.battleState).toEqual({ hits: 4 });
    });
    expect(await screen.findByText("対戦の状態: 4 回")).toBeInTheDocument();
    for (const request of engine.bulkRequests) {
      expect(Object.keys(request)).not.toContain("battleState");
    }
    await user.selectOptions(screen.getByRole("combobox", { name: "技" }), PLAIN);
    expect(screen.queryByRole("combobox", { name: "回数" })).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: "技" }), MULTI);
    expect(screen.getByRole("combobox", { name: "回数" })).toHaveValue("");
    const before = engine.calcRequests.length;
    await screen.findByRole("list", { name: "計算結果" });
    expect(engine.calcRequests).toHaveLength(before);
  });
});

describe("B-5 攻撃側の最大HPの変更で丸める", () => {
  test("攻撃側を最大HPの低い種族に変えると、残りHPを最大に合わせて案内する", async () => {
    const { user } = await start();
    await open(user);
    await enter(user, attackerHp(), String(attackerMax() - 1));
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), lowAttacker.key);
    const lowMax = lowAttacker.baseStats.hp + 75;
    await waitFor(() => {
      expect(attackerHp()).toHaveValue(String(lowMax));
    });
    expect(screen.getByText(`最大HP(${lowMax})に合わせました`)).toBeInTheDocument();
  });
});

describe("B-7 防御側の割合は行ごとの最大HPで実数値にする", () => {
  test("H振り行と無振り行で同じ割合。換算して満タンになる行・100% は付けない", async () => {
    const rowHps = [88, 100, 120, 1, 200];
    const engine = createFakeEngine((request) => {
      const base = bulkResultFor(request);
      return ok({
        ...base,
        rows: base.rows.map((row, index) => ({
          ...row,
          result: { ...row.result, defenderHP: rowHps[index] ?? 88 },
        })),
      });
    });
    const { user } = await start(engine);
    await open(user);
    await enter(user, defenderHp(), "1");
    await waitFor(() => {
      expect(engine.calcRequests.length).toBeGreaterThanOrEqual(4);
    });
    const sent = engine.calcRequests.slice(-4).map((request) => request.battleState);
    // 最大 1 の行は 1% でも満タン(換算結果が最大以上)なので計算し直さない。ほかは max(1, floor(最大 × 1%))。
    expect(sent).toEqual([
      { defenderCurrentHp: 1 },
      { defenderCurrentHp: 1 },
      { defenderCurrentHp: 1 },
      { defenderCurrentHp: 2 },
    ]);
  });

  test("100% は付けない(1対1の計算を呼ばない)", async () => {
    const { user, engine } = await start();
    await open(user);
    await enter(user, defenderHp(), "100");
    await screen.findByRole("list", { name: "計算結果" });
    expect(engine.calcRequests).toHaveLength(0);
  });
});

describe("B-6 お気に入りの保存と復元", () => {
  function okRecord() {
    const createMock = vi.fn<RecordClient["createFavorite"]>((input) =>
      Promise.resolve<RecordResult<{ favorite: Favorite; created: boolean }>>({
        ok: true,
        value: {
          favorite: {
            id: "1",
            label: input.label ?? null,
            individual: input.individual,
            ...(input.calc === undefined ? {} : { calc: input.calc }),
            createdAt: "2026-10-11T01:00:00Z",
            updatedAt: "2026-10-11T01:00:00Z",
          },
          created: true,
        },
      }),
    );
    const unused = () => Promise.reject(new Error("このテストでは使わない"));
    const client: RecordClient = {
      createFavorite: createMock,
      listFrequentOpponents: () => Promise.resolve({ ok: true, value: [] }),
      deleteDeviceData: unused,
      listFavorites: unused,
      deleteFavorite: unused,
      listCalcHistory: unused,
    };
    return { client, createMock };
  }

  test("保存した calc に battleState が入る", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    const { client, createMock } = okRecord();
    render(<CalcScreen engine={engine} master={master} recordClient={client} />);
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "技" }), MULTI);
    await user.selectOptions(screen.getByRole("combobox", { name: "回数" }), "3");
    await user.click(toggle());
    await enter(user, defenderHp(), "50");
    await user.click(screen.getByRole("button", { name: /お気に入り/ }));
    await waitFor(() => {
      expect(createMock).toHaveBeenCalled();
    });
    expect(createMock.mock.calls.at(-1)?.[0].calc?.battleState).toEqual({
      defenderCurrentHp: Math.floor(((defender.baseStats.hp + 75) * 50) / 100),
      hits: 3,
    });
  });

  test("復元すると欄・回数・要求に戻る", async () => {
    const engine = createFakeEngine();
    const favorite: Favorite = {
      id: "7",
      label: "保存",
      individual: {
        speciesKey: attacker.key,
        level: 50,
        natureId: master.natures[0]?.id ?? "",
        sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      },
      calc: {
        format: "single",
        attacker: {
          speciesKey: attacker.key,
          level: 50,
          natureId: master.natures[0]?.id ?? "",
          sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
        },
        defender: {
          speciesKey: defender.key,
          level: 50,
          natureId: master.natures[0]?.id ?? "",
          sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
        },
        moveId: MULTI,
        battleState: { attackerCurrentHp: 40, defenderCurrentHp: 60, hits: 4 },
      },
      createdAt: "2026-10-11T01:00:00Z",
      updatedAt: "2026-10-11T01:00:00Z",
    };
    const user = userEvent.setup();
    render(<CalcScreen engine={engine} master={master} restoreRequest={{ token: 1, favorite }} />);
    await waitFor(() => {
      expect(engine.calcRequests.at(-1)?.battleState).toEqual({
        attackerCurrentHp: 40,
        // 欄は 35%(実数値 60 を無振り最大 175 で戻した値)。fake の行の最大 88 に換算し直すと 30
        defenderCurrentHp: Math.floor((FAKE_ROW_HP * 35) / 100),
        hits: 4,
      });
    });
    expect(screen.getByRole("combobox", { name: "回数" })).toHaveValue("4");
    await user.click(toggle());
    expect(attackerHp()).toHaveValue("40");
    // 実数値 60 は、無振りの最大 HP(175)に届く最小の整数%(35)に戻す
    expect(defenderHp()).toHaveValue("35");
  });
});
