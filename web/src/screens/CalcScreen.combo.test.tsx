// CB-6 段階 B(docs/plan/combo-calc.md §4-3 案 A・AC-19): 計算画面の結果の下の「連続技」。engine は fake(calcCombo を持つ)。
// 確かめること:
//   - 手が 1 つ(メインだけ)のときは「次の技」ボタンだけ。押すと追加の技のピッカーが出る(calcCombo はまだ呼ばない)
//   - 追加の手の候補は攻撃側の覚える技のうち、攻撃技かランクを変える変化技だけ。ランクが変わる技にはチップが付く
//   - 手が 2 件以上(または相手の技を選んだとき)に calcCombo を呼ぶ。攻撃側・防御側・場・条件は計算画面の値をそのまま使う
//   - 並べ替え(メインより前に追加の手を置ける)・削除・急所・最大 3 手
//   - 防御側の振りを変えると要求が変わる(UC-5)
//   - 結果: 各手のカード・合計・N 手で倒す・素早さ・相手の技の耐える確率・印・注記
//   - engine が calcCombo を持たない・失敗しても、単発の計算結果は残り、連続技の節の中だけに alert を出す(絶対ルール 5)
//   - 古い応答は新しい入力の結果を上書きしない・取り消しの signal を渡す

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BulkRequest, Move } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { bulkResultFor, createFakeEngine, engineError, ok } from "../test/fakeEngine";
import {
  comboResultFor,
  createComboEngine,
  createDeferredComboEngine,
  type ComboFakeEngine,
} from "../test/fakeCombo";
import { chooseMove } from "../test/movePicker";
import { CalcScreen } from "./CalcScreen";

const TACKLE = "examplemovetackle";
const FIREPUNCH = "examplemovefirepunch";
const GROWL = "examplemovegrowl";
const DANCE = "examplemovedance";
const CHARGE = "examplemovecharge";
const WATERBLAST = "examplemovewaterblast";

/** 変化技(りゅうのまい相当。攻撃 +1・素早さ +1)。 */
const danceMove: Move = {
  id: DANCE,
  nameJa: "テストりゅうのまい",
  type: "dragon",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1, spe: 1 } },
};
/** 攻撃技で素早さが上がる(ニトロチャージ相当)。 */
const chargeMove: Move = {
  id: CHARGE,
  nameJa: "テストチャージ",
  type: "fire",
  category: "physical",
  power: 50,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { spe: 1 } },
};

let master: MasterData;
let attacker: MasterSpecies;
let defender: MasterSpecies;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const [first, second] = base.species;
  if (first === undefined || second === undefined) {
    throw new Error("例データが足りない");
  }
  attacker = { ...first, learnset: [TACKLE, FIREPUNCH, GROWL, DANCE, CHARGE] };
  defender = { ...second, learnset: [WATERBLAST, TACKLE, GROWL] };
  master = {
    ...base,
    species: [attacker, defender, ...base.species.slice(2)],
    moves: [...base.moves, danceMove, chargeMove],
  };
});

/** 行ごとに防御側の SP が違う一括計算の応答(防御側の振りの選択の確認用。HP の SP を行の番号 × 4 にする)。 */
function respondWithDistinctRows(request: BulkRequest) {
  const bulk = bulkResultFor(request);
  return ok({
    ...bulk,
    rows: bulk.rows.map((row, index) => ({
      ...row,
      defender: { ...row.defender, sp: { ...row.defender.sp, hp: index * 4 } },
    })),
  });
}

async function start(engine: ComboFakeEngine = createComboEngine()) {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await chooseMove(user, TACKLE);
  await screen.findByRole("region", { name: "連続技" });
  return { user, engine };
}

const section = () => screen.getByRole("region", { name: "連続技" });

/** 追加の技のピッカーで技を選ぶ(ピッカーの名前は「追加の技」)。 */
async function addStep(user: UserEvent, id: string): Promise<void> {
  await user.click(within(section()).getByRole("combobox", { name: "追加の技" }));
  const listbox = screen.getByRole("listbox", { name: "追加の技" });
  const row = listbox.querySelector(`[role="option"][data-move-id="${id}"]`);
  if (row === null) {
    throw new Error(`追加の技 ${id} の行が無い`);
  }
  await user.click(row);
}

async function openAddPicker(user: UserEvent): Promise<void> {
  await user.click(within(section()).getByRole("button", { name: "次の技" }));
}

async function chooseOpponentMove(user: UserEvent, id: string): Promise<void> {
  await user.click(within(section()).getByRole("combobox", { name: "相手の技" }));
  const listbox = screen.getByRole("listbox", { name: "相手の技" });
  const row = listbox.querySelector(`[role="option"][data-move-id="${id}"]`);
  if (row === null) {
    throw new Error(`相手の技 ${id} の行が無い`);
  }
  await user.click(row);
}

async function lastCombo(engine: ComboFakeEngine, until: (n: number) => boolean = (n) => n > 0) {
  await waitFor(() => {
    expect(until(engine.comboRequests.length)).toBe(true);
  });
  const request = engine.comboRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcCombo が呼ばれていない");
  }
  return request;
}

const stepIds = (engine: ComboFakeEngine) => engine.comboRequests.at(-1)?.steps.map((s) => s.move.id);

describe("手が 1 つのとき", () => {
  test("「次の技」ボタンだけで、手のリスト・ピッカー・結果は出さず、calcCombo も呼ばない", async () => {
    const { engine } = await start();
    const region = section();
    expect(within(region).getByRole("button", { name: "次の技" })).toBeInTheDocument();
    expect(within(region).queryByRole("combobox", { name: "追加の技" })).toBeNull();
    expect(within(region).queryByRole("list", { name: "連続技の手" })).toBeNull();
    expect(engine.comboRequests).toHaveLength(0);
  });

  test("押すと追加の技のピッカーが出る(まだ計算しない)", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    expect(within(section()).getByRole("combobox", { name: "追加の技" })).toBeInTheDocument();
    expect(within(section()).getByRole("list", { name: "連続技の手" })).toBeInTheDocument();
    expect(engine.comboRequests).toHaveLength(0);
  });

  test("追加の技の候補は、攻撃技とランクを変える変化技だけ(ランクを変えない変化技は出ない)", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await user.click(within(section()).getByRole("combobox", { name: "追加の技" }));
    const ids = within(screen.getByRole("listbox", { name: "追加の技" }))
      .getAllByRole("option")
      .map((row) => row.getAttribute("data-move-id"))
      .filter((id) => id !== "");
    expect([...ids].sort()).toEqual([CHARGE, DANCE, FIREPUNCH, TACKLE].sort());
  });

  test("相手の技だけ選んでも(手が 1 つでも)calcCombo を呼ぶ", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await chooseOpponentMove(user, WATERBLAST);
    const request = await lastCombo(engine);
    expect(request.steps.map((s) => s.move.id)).toEqual([TACKLE]);
    expect(request.opponentMove?.id).toBe(WATERBLAST);
  });
});

describe("手を足す", () => {
  test("2 手になると calcCombo を呼び、要求は計算画面の攻撃側・防御側・タイプ表をそのまま使う", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    const request = await lastCombo(engine);
    expect(request.steps.map((s) => s.move.id)).toEqual([TACKLE, DANCE]);
    expect(request.format).toBe("single");
    expect(request.attacker.species.key).toBe(attacker.key);
    expect(request.defender.species.key).toBe(defender.key);
    expect(request.typeChart).toBe(master.typeChart);
    // 既定のままなら省略できる項目は送らない
    expect(request).not.toHaveProperty("field");
    expect(request).not.toHaveProperty("opponentMove");
    expect(request).not.toHaveProperty("opponentSpeed");
    expect(request).not.toHaveProperty("speedField");
    expect(request.steps[0]).not.toHaveProperty("critical");
  });

  test("ランクを変える技の行にチップ(攻撃 +1・素早さ +1)が付く", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    const row = within(section()).getByRole("listitem", { name: /テストりゅうのまい/ });
    expect(within(row).getByText("攻撃 +1")).toBeInTheDocument();
    expect(within(row).getByText("素早さ +1")).toBeInTheDocument();
  });

  test("最大 3 手: 3 手になると追加のピッカーは消え、上限の案内を出す", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await addStep(user, CHARGE);
    await lastCombo(engine, (n) => n >= 2);
    expect(within(section()).queryByRole("combobox", { name: "追加の技" })).toBeNull();
    expect(within(section()).getByText("手は最大3つまでです")).toBeInTheDocument();
    expect(stepIds(engine)).toEqual([TACKLE, DANCE, CHARGE]);
  });
});

describe("並べ替えと削除", () => {
  test("追加の手を上へ動かすと、メインより前に置ける(りゅうのまい → メイン)", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await user.click(within(section()).getByRole("button", { name: "テストりゅうのまいを上へ" }));
    await waitFor(() => {
      expect(stepIds(engine)).toEqual([DANCE, TACKLE]);
    });
  });

  test("先頭の手の「上へ」・末尾の手の「下へ」は押せない", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    expect(within(section()).getByRole("button", { name: "テストたいあたりを上へ" })).toBeDisabled();
    expect(within(section()).getByRole("button", { name: "テストりゅうのまいを下へ" })).toBeDisabled();
  });

  test("メインに「外す」は無く、追加の手は外せる。外して 1 手に戻ると結果を出さない", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    expect(within(section()).queryByRole("button", { name: "テストたいあたりを外す" })).toBeNull();
    await lastCombo(engine);
    await user.click(within(section()).getByRole("button", { name: "テストりゅうのまいを外す" }));
    await waitFor(() => {
      expect(within(section()).queryByText(/手で倒す/)).toBeNull();
    });
    expect(within(section()).getByRole("combobox", { name: "追加の技" })).toBeInTheDocument();
  });

  test("追加の手がなく相手の技もないとき「閉じる」で「次の技」ボタンだけに戻る", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await user.click(within(section()).getByRole("button", { name: "閉じる" }));
    expect(within(section()).getByRole("button", { name: "次の技" })).toBeInTheDocument();
    expect(within(section()).queryByRole("combobox", { name: "追加の技" })).toBeNull();
  });
});

describe("急所", () => {
  test("追加の手の急所は、その手の critical だけに出る", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, CHARGE);
    await user.click(within(section()).getByRole("checkbox", { name: "急所(2手目)" }));
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.steps[1]?.critical).toBe(true);
    });
    expect(engine.comboRequests.at(-1)?.steps[0]).not.toHaveProperty("critical");
  });

  test("変化技の手に急所の切り替えは無い", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    expect(within(section()).queryByRole("checkbox", { name: "急所(2手目)" })).toBeNull();
    expect(within(section()).getByRole("checkbox", { name: "急所(1手目)" })).toBeInTheDocument();
  });

  test("メインの急所は計算画面の条件(急所)と同じ値: 切り替えると単発の計算にも出る", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await user.click(within(section()).getByRole("checkbox", { name: "急所(1手目)" }));
    await waitFor(() => {
      expect(engine.bulkRequests.at(-1)?.critical).toBe(true);
    });
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.steps[0]?.critical).toBe(true);
    });
  });
});

describe("防御側の振り(UC-5)", () => {
  test("計算画面の行(振り)から選べ、選ぶと要求の防御側が変わる", async () => {
    const engine = createComboEngine(undefined, createFakeEngine(respondWithDistinctRows));
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, DANCE);
    const first = await lastCombo(engine);
    expect(first.defender.sp.hp).toBe(0);
    const select = within(section()).getByRole("combobox", { name: "相手の振り" });
    expect(
      within(select)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toContain("HB振り");
    await user.selectOptions(select, "HB振り");
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.defender.sp.hp).toBe(12);
    });
  });

  test("計算画面のランク(攻撃側の攻撃ランク・防御側の防御ランク)を要求に引き継ぐ", async () => {
    const { user, engine } = await start();
    await user.click(screen.getByRole("button", { name: "詳細" }));
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));
    await user.click(screen.getByRole("button", { name: "防御側のランクを上げる" }));
    await openAddPicker(user);
    await addStep(user, DANCE);
    await waitFor(() => {
      const request = engine.comboRequests.at(-1);
      expect(request?.attacker.ranks?.atk).toBe(1);
      expect(request?.defender.ranks?.def).toBe(1);
    });
  });

  test("天候などの場も要求に引き継ぐ", async () => {
    const { user, engine } = await start();
    await user.click(screen.getByRole("button", { name: "詳細" }));
    await user.click(
      within(screen.getByRole("group", { name: "天候" })).getByRole("radio", { name: "はれ" }),
    );
    await openAddPicker(user);
    await addStep(user, DANCE);
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.field?.weather).toBe("sun");
    });
  });
});

describe("素早さと相手の技の入力", () => {
  test("相手の素早さを防御側の振りから計算する切り替え(既定は送らない)", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await lastCombo(engine);
    const check = within(section()).getByRole("checkbox", { name: "相手の素早さを防御側の振りから計算する" });
    expect(check).not.toBeChecked();
    await user.click(check);
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.opponentSpeed).toBe("from_defender");
    });
    await user.click(check);
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)).not.toHaveProperty("opponentSpeed");
    });
  });

  test("素早さの場(トリックルーム・追い風)は折りたたみで、チェックすると speedField に出る", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    const toggle = within(section()).getByRole("button", { name: "素早さの場" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(within(section()).queryByRole("checkbox", { name: "トリックルーム" })).toBeNull();
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    await user.click(within(section()).getByRole("checkbox", { name: "トリックルーム" }));
    await user.click(within(section()).getByRole("checkbox", { name: "追い風(自分)" }));
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.speedField).toEqual({ trickRoom: true, selfTailwind: true });
    });
  });

  test("相手の技の候補は防御側の覚える攻撃技だけ(変化技は出ない)。選ばない行で外せる", async () => {
    const { user, engine } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await user.click(within(section()).getByRole("combobox", { name: "相手の技" }));
    const ids = within(screen.getByRole("listbox", { name: "相手の技" }))
      .getAllByRole("option")
      .map((row) => row.getAttribute("data-move-id"));
    expect(ids).toContain(WATERBLAST);
    expect(ids).toContain(TACKLE);
    expect(ids).not.toContain(GROWL);
    await user.click(
      within(screen.getByRole("listbox", { name: "相手の技" })).getByRole("option", { name: /選ばない/ }),
    );
    expect(engine.comboRequests.at(-1)).not.toHaveProperty("opponentMove");
  });
});

describe("結果の表示", () => {
  test("各手のカード: 技名・ダメージの最小〜最大%・変化技は「ダメージなし」", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    const list = await within(section()).findByRole("list", { name: "連続技の各手の結果" });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("テストたいあたり");
    expect(items[0]).toHaveTextContent("30.1〜35.4%");
    expect(items[1]).toHaveTextContent("テストりゅうのまい");
    expect(items[1]).toHaveTextContent("ダメージなし");
    expect(within(items[1] as HTMLElement).getByText("攻撃 +1")).toBeInTheDocument();
  });

  test("合計(最小〜最大%)と『N 手で倒す 51.6%』", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    expect(await within(section()).findByText("2 手で倒す 51.6%")).toBeInTheDocument();
    expect(within(section()).getByText("合計 91.4〜108.6%")).toBeInTheDocument();
    // 合計のバー(装飾。幅は 100% で頭打ち)
    expect(within(section()).getByTestId("combo-total-bar-fill")).toHaveStyle({ width: "100%" });
  });

  test("確定は『N 手で倒す(確定)』、不可能は『倒せない』", async () => {
    const guaranteed = createComboEngine((request) =>
      ok(
        comboResultFor(request, { ko: { chancePercent: 100, guaranteed: true, displayChancePercent: 100 } }),
      ),
    );
    const first = await start(guaranteed);
    await openAddPicker(first.user);
    await addStep(first.user, FIREPUNCH);
    expect(await within(section()).findByText("2 手で倒す(確定)")).toBeInTheDocument();
  });

  test("不可能は『倒せない』", async () => {
    const impossible = createComboEngine((request) =>
      ok(comboResultFor(request, { ko: { chancePercent: 0, guaranteed: false, displayChancePercent: 0 } })),
    );
    const { user } = await start(impossible);
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    expect(await within(section()).findByText("倒せない")).toBeInTheDocument();
  });

  test("素早さ『132 → 198』と、相手の素早さが分からないときは最後の手のライン", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    expect(await within(section()).findByText("素早さ 132 → 198")).toBeInTheDocument();
    expect(
      within(section()).getByText("197 以下を抜く / 198 は同速 / 199 以上は相手が先"),
    ).toBeInTheDocument();
  });

  test("相手の素早さが分かるとき(from_defender)は相手の素早さを出し、ラインは出さない", async () => {
    const engine = createComboEngine((request) => ok(comboResultFor(request, { opponentSpeedKnown: true })));
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, DANCE);
    expect(await within(section()).findByText("相手の素早さ 169")).toBeInTheDocument();
    expect(within(section()).queryByText(/は同速/)).toBeNull();
  });

  test("相手の技: 耐える確率・相手の行動回数・1 回ぶんのダメージ", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    await chooseOpponentMove(user, WATERBLAST);
    expect(await within(section()).findByText("耐える 100%")).toBeInTheDocument();
    expect(within(section()).getByText("相手の行動 1〜2回")).toBeInTheDocument();
    expect(within(section()).getByText("1回ぶん 40.2〜47.8%")).toBeInTheDocument();
  });

  test("相手の技を選んでいなければ「耐える」は出ない", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    await within(section()).findByText("2 手で倒す 51.6%");
    expect(within(section()).queryByText(/耐える/)).toBeNull();
  });

  test("場は全手で同じ・命中は考えない、の注記", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    await within(section()).findByText("2 手で倒す 51.6%");
    expect(
      within(section()).getByText("場は全手で同じ状態が続きます。命中は考えません。"),
    ).toBeInTheDocument();
  });

  test("未対応の印(choice_locked・defender_full_hp_assumed・speed_undetermined)を日本語で出す", async () => {
    const engine = createComboEngine((request) =>
      ok(
        comboResultFor(request, {
          unsupported: [
            { target: "attacker_item", reason: "choice_locked", id: "x" },
            { target: "defender_ability", reason: "defender_full_hp_assumed", id: "y" },
            { target: "attacker_ability", reason: "speed_undetermined", id: "z" },
          ],
        }),
      ),
    );
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, FIREPUNCH);
    const notice = await within(section()).findByRole("status");
    expect(notice).toHaveTextContent("この結果は正確でない可能性があります");
    expect(notice).toHaveTextContent("技を変えられない");
    expect(notice).toHaveTextContent("満タン");
    expect(notice).toHaveTextContent("素早さ");
    // 「詳細は不明」に落ちていない(連続技の印は専用の語を持つ)
    expect(notice).not.toHaveTextContent("詳細は不明");
  });
});

describe("失敗しても単発の計算は壊さない(絶対ルール 5)", () => {
  test("calcCombo を持たない engine: 節の中だけに alert、単発の結果は残る", async () => {
    const { user } = await start(createFakeEngine() as ComboFakeEngine);
    await openAddPicker(user);
    await addStep(user, DANCE);
    const alert = await within(section()).findByRole("alert");
    expect(alert).toHaveTextContent("この計算エンジンは連続技に対応していません");
    // 単発の結果(一括計算の行)は残る
    expect(screen.getAllByTestId("damage-bar").length).toBeGreaterThan(0);
  });

  test("calcCombo が失敗: 節の中だけに alert、単発の結果は残る", async () => {
    const engine = createComboEngine(() => engineError("invalid_input", "手が多すぎます"));
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, DANCE);
    const alert = await within(section()).findByRole("alert");
    expect(alert).toHaveTextContent("連続技を計算できませんでした");
    expect(alert).toHaveTextContent("手が多すぎます");
    expect(screen.getAllByTestId("damage-bar").length).toBeGreaterThan(0);
  });

  test("取り消し(request_aborted)はエラーとして出さない", async () => {
    const engine = createComboEngine(() => engineError("request_aborted", "取り消し"));
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, DANCE);
    await lastCombo(engine);
    expect(within(section()).queryByRole("alert")).toBeNull();
  });
});

describe("古い応答・取り消し", () => {
  test("古い入力の応答が後から届いても、新しい入力の結果を上書きしない。古い計算には abort を伝える", async () => {
    const { engine, pending } = createDeferredComboEngine();
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, CHARGE);
    await waitFor(() => {
      expect(pending).toHaveLength(1);
    });
    await user.click(within(section()).getByRole("checkbox", { name: "急所(2手目)" }));
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    expect(pending[0]?.signal?.aborted).toBe(true);
    await act(async () => {
      pending[1]?.resolve(
        ok(
          comboResultFor(pending[1].request, {
            ko: { chancePercent: 80, guaranteed: false, displayChancePercent: 80 },
          }),
        ),
      );
      await Promise.resolve();
    });
    expect(await within(section()).findByText("2 手で倒す 80.0%")).toBeInTheDocument();
    await act(async () => {
      pending[0]?.resolve(
        ok(
          comboResultFor(pending[0].request, {
            ko: { chancePercent: 10, guaranteed: false, displayChancePercent: 10 },
          }),
        ),
      );
      await Promise.resolve();
    });
    expect(within(section()).getByText("2 手で倒す 80.0%")).toBeInTheDocument();
    expect(within(section()).queryByText("2 手で倒す 10.0%")).toBeNull();
  });

  test("入力を変えた直後は古い結果を出さず、計算中を示す", async () => {
    const { engine, pending } = createDeferredComboEngine();
    const { user } = await start(engine);
    await openAddPicker(user);
    await addStep(user, CHARGE);
    await waitFor(() => {
      expect(pending).toHaveLength(1);
    });
    await act(async () => {
      pending[0]?.resolve(ok(comboResultFor(pending[0].request)));
      await Promise.resolve();
    });
    await within(section()).findByText("2 手で倒す 51.6%");
    await user.click(within(section()).getByRole("checkbox", { name: "急所(2手目)" }));
    expect(within(section()).queryByText("2 手で倒す 51.6%")).toBeNull();
    expect(within(section()).getByText("連続技を計算中…")).toBeInTheDocument();
  });
});

describe("攻撃側を変えたとき", () => {
  test("覚えない追加の手は外れる(残りの手が 1 つなら結果は消える)", async () => {
    const { user } = await start();
    await openAddPicker(user);
    await addStep(user, DANCE);
    await within(section()).findByText("2 手で倒す 51.6%");
    // 攻撃側を、りゅうのまいを覚えない種族へ(防御側の種族は覚えない)
    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), defender.key);
    await waitFor(() => {
      expect(within(section()).queryByText(/手で倒す/)).toBeNull();
    });
  });
});
