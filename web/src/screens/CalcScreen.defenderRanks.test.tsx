// issue #274 残り(Web 分。ADR-0315 案): 計算画面の「詳細」の「防御側のランク」。engine は fake。
// 約束(spec): 選択中の技の分類に関連する防御側ステータスだけを編集(物理 = B〈def〉・特殊 = D〈spd〉・変化/技なしは B)、
//   -6..+6、表示「B +1」「D -2」「B ±0」(攻撃側の「A +1」と同じ作り)。def / spd は別々に保持して両方送る。
//   既定(0・0)なら defenderOverride を送らない。条件は攻守入れ替え・種族・技の変更で消さない。古い結果を出さない。
//   防御側の状態異常は式に効かないので出さない。特性(#272)とは同じ defenderOverride に併存する(apiEngine 側のテスト)。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BulkRequest } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { bulkRow, createDeferredEngine, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";
import { chooseMove } from "../test/movePicker";

const PHYSICAL = "examplemovetackle";
const SPECIAL = "examplemovewaterblast";

let master: MasterData;
let attacker: MasterSpecies;
let defender: MasterSpecies;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const [first, second] = base.species;
  if (first === undefined || second === undefined) {
    throw new Error("例データが足りない");
  }
  attacker = { ...first, learnset: [PHYSICAL, SPECIAL] };
  defender = second;
  master = { ...base, species: [attacker, defender, ...base.species.slice(2)] };
});

const toggle = () => screen.getByRole("button", { name: "詳細" });
const group = (name: string) => screen.getByRole("group", { name });
const up = () => screen.getByRole("button", { name: "防御側のランクを上げる" });
const down = () => screen.getByRole("button", { name: "防御側のランクを下げる" });
const display = () => within(group("防御側のランク"));

async function start(
  engine: FakeEngine = createFakeEngine(),
): Promise<{ user: UserEvent; engine: FakeEngine }> {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await chooseMove(user, PHYSICAL);
  return { user, engine };
}

async function openDetails(user: UserEvent): Promise<void> {
  await user.click(toggle());
  expect(toggle()).toHaveAttribute("aria-expanded", "true");
}

async function lastRequest(engine: FakeEngine, until: (r: BulkRequest) => boolean = () => true) {
  await waitFor(() => {
    const request = engine.bulkRequests.at(-1);
    expect(request).toBeDefined();
    if (request !== undefined) {
      expect(until(request)).toBe(true);
    }
  });
  const request = engine.bulkRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return request;
}

describe("置き場所と文言", () => {
  test("閉じている間は DOM に無く、開くと「攻撃側のランク」の次に「防御側のランク」が出る", async () => {
    const { user } = await start();
    expect(screen.queryByRole("group", { name: "防御側のランク" })).toBeNull();
    await openDetails(user);
    const names = screen.getAllByRole("group").map((g) => g.querySelector("legend")?.textContent);
    expect(names.indexOf("防御側のランク")).toBe(names.indexOf("攻撃側のランク") + 1);
    expect(names.indexOf("攻撃側のランク")).toBe(names.indexOf("防御側の壁") + 1);
  });

  test("物理技は B を編集する(表示「B ±0」→「B +1」)", async () => {
    const { user } = await start();
    await openDetails(user);
    expect(display().getByText("B ±0")).toBeInTheDocument();
    await user.click(up());
    expect(display().getByText("B +1")).toBeInTheDocument();
  });

  test("特殊技は D を編集する(表示「D -2」)", async () => {
    const { user } = await start();
    await openDetails(user);
    await chooseMove(user, SPECIAL);
    expect(display().getByText("D ±0")).toBeInTheDocument();
    await user.click(down());
    await user.click(down());
    expect(display().getByText("D -2")).toBeInTheDocument();
  });

  test("防御側の状態異常は出さない(ダメージ式に効かない。ADR-0216)", async () => {
    const { user } = await start();
    await openDetails(user);
    expect(screen.queryByText("防御側の状態異常")).toBeNull();
  });
});

describe("要求への載せ方", () => {
  test("既定のままなら defenderOverride を送らない(「詳細」を開いても同じ)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    expect(await lastRequest(engine)).not.toHaveProperty("defenderOverride");
  });

  test("物理で B +1 → defenderOverride.ranks は 5 項目(def:1、他は 0)。攻撃側の ranks は作らない", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(up());
    const request = await lastRequest(engine, (r) => r.defenderOverride !== undefined);
    expect(request.defenderOverride).toEqual({ ranks: { atk: 0, def: 1, spa: 0, spd: 0, spe: 0 } });
    expect(request.attacker).not.toHaveProperty("ranks");
  });

  test("def と spd は別々に保持し、技の分類を往復しても消えず、要求には両方入る", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(up()); // B +1
    await chooseMove(user, SPECIAL);
    await user.click(down());
    await user.click(down()); // D -2
    let request = await lastRequest(engine, (r) => r.move.id === SPECIAL && r.defenderOverride !== undefined);
    expect(request.defenderOverride?.ranks).toEqual({ atk: 0, def: 1, spa: 0, spd: -2, spe: 0 });
    await chooseMove(user, PHYSICAL);
    expect(display().getByText("B +1")).toBeInTheDocument();
    request = await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    expect(request.defenderOverride?.ranks).toEqual({ atk: 0, def: 1, spa: 0, spd: -2, spe: 0 });
  });

  test("攻撃側のランクと独立: 両方触ると attacker.ranks と defenderOverride.ranks が別々に載る", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));
    await user.click(down());
    const request = await lastRequest(
      engine,
      (r) => r.attacker.ranks?.atk === 1 && r.defenderOverride?.ranks?.def === -1,
    );
    expect(request.attacker.ranks).toEqual({ atk: 1, def: 0, spa: 0, spd: 0, spe: 0 });
    expect(request.defenderOverride?.ranks).toEqual({ atk: 0, def: -1, spa: 0, spd: 0, spe: 0 });
  });

  test("元に戻す(+1 → ±0)と defenderOverride をまた送らない", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(up());
    await lastRequest(engine, (r) => r.defenderOverride !== undefined);
    await user.click(down());
    const request = await lastRequest(engine, (r) => r.defenderOverride === undefined);
    expect(request).not.toHaveProperty("defenderOverride");
  });

  test("境界: +6 で上げるボタンが止まり、-6 で下げるボタンが止まる", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    for (let i = 0; i < 6; i += 1) {
      await user.click(up());
    }
    expect(display().getByText("B +6")).toBeInTheDocument();
    expect(up()).toBeDisabled();
    expect(
      (await lastRequest(engine, (r) => r.defenderOverride?.ranks?.def === 6)).defenderOverride?.ranks?.def,
    ).toBe(6);
    for (let i = 0; i < 12; i += 1) {
      await user.click(down());
    }
    expect(display().getByText("B -6")).toBeInTheDocument();
    expect(down()).toBeDisabled();
  });
});

describe("条件の寿命と古い結果", () => {
  test("攻守入れ替え・種族・技の変更、「詳細」の開閉をしても防御側のランクは残る", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(up());
    await user.click(toggle());
    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
    await chooseMove(user, PHYSICAL);
    const request = await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    expect(request.defenderOverride?.ranks?.def).toBe(1);
    await openDetails(user);
    expect(display().getByText("B +1")).toBeInTheDocument();
  });

  test("変えた直後は「計算中」で古い結果を出さず、遅れて届いた古い応答は新しい結果を上書きしない", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = await start(engine);
    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(0);
    });
    const countBefore = pending.length;
    await act(async () => {
      pending
        .at(-1)
        ?.resolve(
          ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "ランクなしの結果" })] }),
        );
      await Promise.resolve();
    });
    expect(await screen.findByText("ランクなしの結果")).toBeInTheDocument();

    await openDetails(user);
    await user.click(up());
    await waitFor(() => {
      expect(pending).toHaveLength(countBefore + 1);
    });
    expect(screen.queryByText("ランクなしの結果")).toBeNull();
    expect(screen.getByText("計算中")).toBeInTheDocument();

    await user.click(up());
    await waitFor(() => {
      expect(pending).toHaveLength(countBefore + 2);
    });
    const [, older, newest] = pending.slice(-3);
    await act(async () => {
      newest?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "B+2の結果" })] }),
      );
      await Promise.resolve();
    });
    await act(async () => {
      older?.resolve(ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "B+1の結果" })] }));
      await Promise.resolve();
    });
    expect(screen.getByText("B+2の結果")).toBeInTheDocument();
    expect(screen.queryByText("B+1の結果")).toBeNull();
  });
});
