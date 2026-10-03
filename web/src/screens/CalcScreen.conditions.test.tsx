// issue #274(ADR-0312): 計算画面の「詳細」(急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク)。engine は fake。
// 仕様の正: docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」(iOS)、docs/adr/0501「issue #274」。
// 確かめること:
//   - 「詳細」は既定で閉じる disclosure(button・aria-expanded・キーボード)。閉じている間は中の入力が DOM に無い
//   - 既定のままなら calcBulk の要求は従来と同じ(critical・field・attacker.status・attacker.ranks が無い)
//   - 各入力が要求に出る(critical / attacker.status=burn / field.weather・terrain・defenderScreens / attacker.ranks)
//   - ランクは選択中の技の分類の関連ステータスだけを編集(物理=A・特殊=C)、-6..+6 で止まる、atk/spa は別々に保持して両方送る
//   - 条件は攻守入れ替え・種族・技の変更で消えない
//   - 入力を変えた直後は古い結果を出さない(CompletedCalc の流儀)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BulkRequest } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { bulkRow, createDeferredEngine, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

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
  // 物理技と特殊技の両方を覚える攻撃側にする(分類の切り替えで A ↔ C を確かめるため)。
  attacker = { ...first, learnset: [PHYSICAL, SPECIAL] };
  defender = second;
  master = { ...base, species: [attacker, defender, ...base.species.slice(2)] };
});

const toggle = () => screen.getByRole("button", { name: "詳細" });
const moveSelect = () => screen.getByRole("combobox", { name: "技" });

function renderScreen(engine: FakeEngine = createFakeEngine()): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  return { user, engine };
}

async function start(engine?: FakeEngine): Promise<{ user: UserEvent; engine: FakeEngine }> {
  const rendered = renderScreen(engine);
  await rendered.user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await rendered.user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await rendered.user.selectOptions(moveSelect(), PHYSICAL);
  return rendered;
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

const group = (name: string) => screen.getByRole("group", { name });
const radio = (groupName: string, name: string) => within(group(groupName)).getByRole("radio", { name });

describe("「詳細」の開閉", () => {
  test("既定は閉じていて、中の入力は DOM に無い", async () => {
    await start();
    expect(toggle()).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("checkbox", { name: "急所" })).toBeNull();
    expect(screen.queryByRole("checkbox", { name: "やけど" })).toBeNull();
    expect(screen.queryByRole("group", { name: "天候" })).toBeNull();
  });

  test("キーボード(Enter / Space)で開閉でき、開くと中の入力が出る", async () => {
    const { user } = await start();
    toggle().focus();
    await user.keyboard("{Enter}");
    expect(toggle()).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("checkbox", { name: "急所" })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "やけど" })).toBeInTheDocument();
    await user.keyboard(" ");
    expect(toggle()).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("checkbox", { name: "急所" })).toBeNull();
  });

  test("aria-controls が開いた領域を指す", async () => {
    const { user } = await start();
    await openDetails(user);
    const id = toggle().getAttribute("aria-controls");
    expect(id).not.toBeNull();
    expect(document.getElementById(id ?? "")).toContainElement(
      screen.getByRole("checkbox", { name: "急所" }),
    );
  });

  test("開いた中の並び: 急所・やけど → 天候 → フィールド → 防御側の壁 → 攻撃側のランク", async () => {
    const { user } = await start();
    await openDetails(user);
    expect(
      within(group("天候"))
        .getAllByRole("radio")
        .map((r) => r.closest("label")?.textContent),
    ).toEqual(["なし", "はれ", "あめ", "すなあらし", "ゆき"]);
    expect(
      within(group("フィールド"))
        .getAllByRole("radio")
        .map((r) => r.closest("label")?.textContent),
    ).toEqual(["なし", "エレキフィールド", "グラスフィールド", "サイコフィールド", "ミストフィールド"]);
    expect(
      within(group("防御側の壁"))
        .getAllByRole("checkbox")
        .map((r) => r.closest("label")?.textContent),
    ).toEqual(["リフレクター", "ひかりのかべ", "オーロラベール"]);
    expect(group("攻撃側のランク")).toBeInTheDocument();
    // 天候・フィールドの既定は「なし」
    expect(radio("天候", "なし")).toBeChecked();
    expect(radio("フィールド", "なし")).toBeChecked();
  });

  test("やけど以外の状態異常・攻撃側の壁・防御側の状態異常は出さない(防御側のランクは CalcScreen.defenderRanks.test.tsx)", async () => {
    const { user } = await start();
    await openDetails(user);
    for (const name of ["まひ", "どく", "もうどく", "ねむり", "こおり", "攻撃側の壁"]) {
      expect(screen.queryByText(name)).toBeNull();
    }
  });
});

describe("既定のままなら要求は従来と同じ", () => {
  test("「詳細」を開いて何も触らなくても、critical・field・attacker.status・attacker.ranks を送らない", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    const request = await lastRequest(engine);
    expect(request).not.toHaveProperty("critical");
    expect(request).not.toHaveProperty("field");
    expect(request.attacker).not.toHaveProperty("status");
    expect(request.attacker).not.toHaveProperty("ranks");
  });

  test("触って元に戻した(急所 on→off・天候 はれ→なし・ランク +1→±0)ら、また何も送らない", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    await user.click(radio("天候", "はれ"));
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));
    await lastRequest(engine, (r) => r.critical === true);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    await user.click(radio("天候", "なし"));
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを下げる" }));
    const request = await lastRequest(
      engine,
      (r) => r.critical === undefined && r.field === undefined && r.attacker.ranks === undefined,
    );
    expect(request).not.toHaveProperty("critical");
    expect(request).not.toHaveProperty("field");
    expect(request.attacker).not.toHaveProperty("ranks");
  });
});

describe("各入力が要求に出る", () => {
  test("急所 → critical:true", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    expect((await lastRequest(engine, (r) => r.critical === true)).critical).toBe(true);
  });

  test("やけど → attacker.status:'burn'(防御側には付けない)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "やけど" }));
    const request = await lastRequest(engine, (r) => r.attacker.status === "burn");
    expect(request.attacker.status).toBe("burn");
  });

  test.each([
    ["はれ", "sun"],
    ["あめ", "rain"],
    ["すなあらし", "sand"],
    ["ゆき", "snow"],
  ])("天候 %s → field.weather:%s", async (label, id) => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(radio("天候", label));
    const request = await lastRequest(engine, (r) => r.field?.weather === id);
    expect(request.field).toEqual({ weather: id });
  });

  test("天候を「なし」に戻すと field を送らない(none を送らない)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(radio("天候", "あめ"));
    await lastRequest(engine, (r) => r.field?.weather === "rain");
    await user.click(radio("天候", "なし"));
    const request = await lastRequest(engine, (r) => r.field === undefined);
    expect(request).not.toHaveProperty("field");
  });

  test.each([
    ["エレキフィールド", "electric"],
    ["グラスフィールド", "grassy"],
    ["サイコフィールド", "psychic"],
    ["ミストフィールド", "misty"],
  ])("フィールド %s → field.terrain:%s", async (label, id) => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(radio("フィールド", label));
    const request = await lastRequest(engine, (r) => r.field?.terrain === id);
    expect(request.field).toEqual({ terrain: id });
  });

  test("壁は独立のトグル: ひかりのかべ → リフレクターを足しても両方 on のまま(field.defenderScreens)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "ひかりのかべ" }));
    let request = await lastRequest(engine, (r) => r.field?.defenderScreens?.lightScreen === true);
    expect(request.field).toEqual({
      defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false },
    });
    await user.click(screen.getByRole("checkbox", { name: "リフレクター" }));
    request = await lastRequest(engine, (r) => r.field?.defenderScreens?.reflect === true);
    expect(request.field?.defenderScreens).toEqual({ reflect: true, lightScreen: true, auroraVeil: false });
    expect(request.field).not.toHaveProperty("attackerScreens");
    await user.click(screen.getByRole("checkbox", { name: "オーロラベール" }));
    request = await lastRequest(engine, (r) => r.field?.defenderScreens?.auroraVeil === true);
    expect(request.field?.defenderScreens).toEqual({ reflect: true, lightScreen: true, auroraVeil: true });
  });

  test("物理技でも ひかりのかべ をそのまま送る(無関係な補正は engine が処理する)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    expect(moveSelect()).toHaveValue(PHYSICAL);
    await user.click(screen.getByRole("checkbox", { name: "ひかりのかべ" }));
    const request = await lastRequest(engine, (r) => r.field?.defenderScreens?.lightScreen === true);
    expect(request.move.category).toBe("physical");
  });
});

describe("攻撃側のランク", () => {
  const up = () => screen.getByRole("button", { name: "攻撃側のランクを上げる" });
  const down = () => screen.getByRole("button", { name: "攻撃側のランクを下げる" });
  const display = () => within(group("攻撃側のランク"));

  test("物理技は A を編集する。表示は「A ±0」→「A +1」、要求の ranks は 5 項目", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    expect(display().getByText("A ±0")).toBeInTheDocument();
    await user.click(up());
    expect(display().getByText("A +1")).toBeInTheDocument();
    const request = await lastRequest(engine, (r) => r.attacker.ranks?.atk === 1);
    expect(request.attacker.ranks).toEqual({ atk: 1, def: 0, spa: 0, spd: 0, spe: 0 });
  });

  test("特殊技は C を編集する(表示「C -2」)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.selectOptions(moveSelect(), SPECIAL);
    expect(display().getByText("C ±0")).toBeInTheDocument();
    await user.click(down());
    await user.click(down());
    expect(display().getByText("C -2")).toBeInTheDocument();
    const request = await lastRequest(engine, (r) => r.attacker.ranks?.spa === -2);
    expect(request.attacker.ranks).toEqual({ atk: 0, def: 0, spa: -2, spd: 0, spe: 0 });
  });

  test("atk と spa は別々に保持し、技の分類を往復しても消えず、要求には両方入る", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(up()); // A +1
    await user.selectOptions(moveSelect(), SPECIAL);
    await user.click(up());
    await user.click(up()); // C +2
    let request = await lastRequest(engine, (r) => r.move.id === SPECIAL && r.attacker.ranks?.spa === 2);
    expect(request.attacker.ranks).toEqual({ atk: 1, def: 0, spa: 2, spd: 0, spe: 0 });
    await user.selectOptions(moveSelect(), PHYSICAL);
    expect(display().getByText("A +1")).toBeInTheDocument();
    request = await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    expect(request.attacker.ranks).toEqual({ atk: 1, def: 0, spa: 2, spd: 0, spe: 0 });
  });

  test("境界: +6 で上げるボタンが止まり、-6 で下げるボタンが止まる", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    for (let i = 0; i < 6; i += 1) {
      await user.click(up());
    }
    expect(display().getByText("A +6")).toBeInTheDocument();
    expect(up()).toBeDisabled();
    await user.click(up());
    expect(display().getByText("A +6")).toBeInTheDocument();
    expect((await lastRequest(engine, (r) => r.attacker.ranks?.atk === 6)).attacker.ranks?.atk).toBe(6);

    for (let i = 0; i < 12; i += 1) {
      await user.click(down());
    }
    expect(display().getByText("A -6")).toBeInTheDocument();
    expect(down()).toBeDisabled();
    expect((await lastRequest(engine, (r) => r.attacker.ranks?.atk === -6)).attacker.ranks?.atk).toBe(-6);
  });
});

describe("条件は入れ替え・種族・技の変更で消さない", () => {
  test("攻守入れ替え・攻撃側の種族変更・技の変更をしても、条件が残り要求に出続ける", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    await user.click(screen.getByRole("checkbox", { name: "やけど" }));
    await user.click(radio("天候", "ゆき"));
    await user.click(radio("フィールド", "ミストフィールド"));
    await user.click(screen.getByRole("checkbox", { name: "オーロラベール" }));
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
    await user.selectOptions(moveSelect(), SPECIAL);

    const request = await lastRequest(engine, (r) => r.move.id === SPECIAL);
    expect(request.critical).toBe(true);
    expect(request.attacker.status).toBe("burn");
    expect(request.field).toEqual({
      weather: "snow",
      terrain: "misty",
      defenderScreens: { reflect: false, lightScreen: false, auroraVeil: true },
    });
    expect(request.attacker.ranks?.atk).toBe(1);
    expect(screen.getByRole("checkbox", { name: "急所" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "やけど" })).toBeChecked();
  });

  test("「詳細」を閉じても条件は効いたまま(閉じ=リセットではない)", async () => {
    const { user, engine } = await start();
    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    await user.click(toggle());
    expect(toggle()).toHaveAttribute("aria-expanded", "false");
    await user.selectOptions(moveSelect(), SPECIAL);
    expect((await lastRequest(engine, (r) => r.move.id === SPECIAL)).critical).toBe(true);
  });
});

describe("入力を変えたら古い結果を出さない", () => {
  test("条件を変えた直後は「計算中」で、古い条件の応答が後から届いても新しい結果を上書きしない", async () => {
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
          ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "条件なしの結果" })] }),
        );
      await Promise.resolve();
    });
    expect(await screen.findByText("条件なしの結果")).toBeInTheDocument();

    await openDetails(user);
    await user.click(screen.getByRole("checkbox", { name: "急所" }));
    await waitFor(() => {
      expect(pending).toHaveLength(countBefore + 1);
    });
    // 条件を変えた直後は、古い条件の行を出さず「計算中」にする
    expect(screen.queryByText("条件なしの結果")).toBeNull();
    expect(screen.getByText("計算中")).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: "やけど" }));
    await waitFor(() => {
      expect(pending).toHaveLength(countBefore + 2);
    });
    const [, newerButOlderThanLast, last] = pending.slice(-3);
    await act(async () => {
      last?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "急所とやけどの結果" })] }),
      );
      await Promise.resolve();
    });
    await act(async () => {
      newerButOlderThanLast?.resolve(
        ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "急所だけの結果" })] }),
      );
      await Promise.resolve();
    });
    expect(screen.getByText("急所とやけどの結果")).toBeInTheDocument();
    expect(screen.queryByText("急所だけの結果")).toBeNull();
  });
});
