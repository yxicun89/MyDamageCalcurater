// I-web-1・I-web-3(ADR-0329): 計算画面の攻撃側の「攻撃」「特攻」の入力。engine は fake。
// 確かめること:
//   - 「技」の次に「攻撃」「特攻」の2ブロックを常に出し、選択中の技の分類が使う方を強調する
//     (見出しに「(この技で使用)」・aria-current="true"。色だけに頼らない)。旧「攻撃側の調整」は無くなる
//   - 各ブロック = プリセット(無振り・特化・振り(無補正))+ SP の数値入力(0〜32)+ 性格補正(上昇・補正なし・下降)
//   - プリセットを選ぶと数値欄・補正に値が入り、手で変えてどれとも一致しなくなると「カスタム」
//   - 数値・補正が要求の attacker.sp・attacker.nature に載る(H・B・D・S は 0 のまま)
//   - 技を切り替えても、攻守入れ替え・攻撃側の種族を変えても、両方の値が残る(ADR-0312 §6 の条件の寿命と同じ)
//   - A と C を同じ向き(上昇と上昇・下降と下降)にする選択は無効(実在しない性格を作らない)
//   - 範囲外・整数でない SP は明示エラー(aria-invalid・role=alert)で計算しない。空欄は 0
//   - 性格がマスタで解決できなければ明示エラーで計算しない
//   - 何も触らなければ要求は従来とバイト同一
//   - 入力を変えた直後は古い結果を出さない(CompletedCalc の流儀)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildIndividual,
  defaultAbility,
  defenderAbilityCandidates,
  selectableAbilities,
} from "../domain/requests";
import type { BulkRequest, Move } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";
import { bulkRow, createDeferredEngine, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";
import { chooseMove, moveTrigger } from "../test/movePicker";

const PHYSICAL = "examplemovetackle";
const SPECIAL = "examplemovewaterblast";

let master: MasterData;
let attacker: MasterSpecies;
let defender: MasterSpecies;
let thirdSpecies: MasterSpecies;
let physicalMove: Move;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const [first, second, third] = base.species;
  if (first === undefined || second === undefined || third === undefined) {
    throw new Error("例データが足りない");
  }
  // 物理技と特殊技の両方を覚える攻撃側にする(分類の切り替えで強調が A ↔ C に移ることを確かめるため)。
  attacker = { ...first, learnset: [PHYSICAL, SPECIAL] };
  defender = { ...second, learnset: [SPECIAL, PHYSICAL] };
  thirdSpecies = { ...third, learnset: [PHYSICAL, SPECIAL] };
  master = { ...base, species: [attacker, defender, thirdSpecies, ...base.species.slice(3)] };
  const found = master.moves.find((move) => move.id === PHYSICAL);
  if (found === undefined) {
    throw new Error("例データに物理技が無い");
  }
  physicalMove = found;
});

type Stat = "atk" | "spa";
const STAT_NAME: Record<Stat, string> = { atk: "攻撃", spa: "特攻" };
const USED_SUFFIX = "(この技で使用)";
const BLOCK_NAME: Record<Stat, RegExp> = {
  atk: /^攻撃(\(この技で使用\))?$/,
  spa: /^特攻(\(この技で使用\))?$/,
};

const block = (stat: Stat) => screen.getByRole("group", { name: BLOCK_NAME[stat] });
const spBox = (stat: Stat) => within(block(stat)).getByRole("textbox", { name: `${STAT_NAME[stat]}のSP` });
const presetGroup = (stat: Stat) =>
  within(block(stat)).getByRole("radiogroup", { name: `${STAT_NAME[stat]}の調整` });
const presetRadio = (stat: Stat, name: string) => within(presetGroup(stat)).getByRole("radio", { name });
const natureGroup = (stat: Stat) =>
  within(block(stat)).getByRole("radiogroup", { name: `${STAT_NAME[stat]}の性格補正` });
const natureRadio = (stat: Stat, name: "上昇" | "補正なし" | "下降") =>
  within(natureGroup(stat)).getByRole("radio", { name });

function renderScreen(
  engine: FakeEngine = createFakeEngine(),
  data: MasterData = master,
): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={data} />);
  return { user, engine };
}

async function start(
  engine?: FakeEngine,
  data?: MasterData,
): Promise<{ user: UserEvent; engine: FakeEngine }> {
  const rendered = renderScreen(engine, data);
  await rendered.user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await rendered.user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await chooseMove(rendered.user, PHYSICAL);
  return rendered;
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

/** 欄を空にしてから一度に貼り付ける(1文字ずつの途中の値で計算させない)。 */
async function setSp(user: UserEvent, stat: Stat, text: string): Promise<void> {
  const box = spBox(stat);
  await user.clear(box);
  if (text !== "") {
    await user.click(box);
    await user.paste(text);
  }
  expect(box).toHaveValue(text);
}

/** 非同期の effect を流しきる(呼ばれないことを確かめる前に)。 */
async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
  });
}

describe("2ブロックの表示と強調(I-web-3)", () => {
  test("物理技では「攻撃(この技で使用)」と「特攻」の2ブロックを出し、攻撃側だけ aria-current", async () => {
    await start();

    expect(block("atk")).toHaveAccessibleName(`攻撃${USED_SUFFIX}`);
    expect(block("atk")).toHaveAttribute("aria-current", "true");
    expect(block("spa")).toHaveAccessibleName("特攻");
    expect(block("spa")).not.toHaveAttribute("aria-current");
  });

  test("特殊技に替えると強調が特攻に移る(どちらのブロックも出たまま)", async () => {
    const { user } = await start();
    await chooseMove(user, SPECIAL);

    expect(block("spa")).toHaveAccessibleName(`特攻${USED_SUFFIX}`);
    expect(block("spa")).toHaveAttribute("aria-current", "true");
    expect(block("atk")).toHaveAccessibleName("攻撃");
    expect(block("atk")).not.toHaveAttribute("aria-current");
  });

  test("各ブロックにプリセット3つ・SP 欄・性格補正3つがあり、既定は 無振り・0・補正なし", async () => {
    await start();

    for (const [stat, letter] of [
      ["atk", "A"],
      ["spa", "C"],
    ] as const) {
      // 並びは none → x_full → x(attackerPresets と同じ。表示名は既存の attackerPresetText を踏襲)
      expect(within(presetGroup(stat)).getAllByRole("radio")).toEqual([
        presetRadio(stat, "無振り"),
        presetRadio(stat, `${letter}特化`),
        presetRadio(stat, `${letter}振り(無補正)`),
      ]);
      expect(presetRadio(stat, "無振り")).toBeChecked();
      expect(spBox(stat)).toHaveValue("0");
      expect(spBox(stat)).toHaveAttribute("inputmode", "numeric");
      const natures = within(natureGroup(stat)).getAllByRole("radio");
      expect(natures).toHaveLength(3);
      expect(natureRadio(stat, "上昇")).not.toBeChecked();
      expect(natureRadio(stat, "補正なし")).toBeChecked();
      expect(natureRadio(stat, "下降")).not.toBeChecked();
    }
  });

  test("並びは 技 → 攻撃 → 特攻 → 持ち物の候補の比較。旧「攻撃側の調整」は出さない", async () => {
    await start();

    const order = [
      moveTrigger(),
      block("atk"),
      block("spa"),
      screen.getByRole("checkbox", { name: "持ち物の候補も比較" }),
    ];
    for (let index = 1; index < order.length; index += 1) {
      const previous = order[index - 1];
      const current = order[index];
      if (previous === undefined || current === undefined) {
        throw new Error("並びの要素が足りない");
      }
      expect(previous.compareDocumentPosition(current) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
    expect(screen.queryByRole("radiogroup", { name: "攻撃側の調整" })).toBeNull();
  });

  test("攻撃側を選ぶまでは2ブロックを出さない", () => {
    renderScreen();
    expect(screen.queryByRole("group", { name: BLOCK_NAME.atk })).toBeNull();
    expect(screen.queryByRole("group", { name: BLOCK_NAME.spa })).toBeNull();
  });
});

describe("要求への反映(I-web-1)", () => {
  test("何も触らなければ要求は従来とバイト同一(無振り・無補正)", async () => {
    const { engine } = await start();
    const request = await lastRequest(engine, (r) => r.move.id === PHYSICAL);

    const expected = buildBulkRequest({
      attacker: buildIndividual(attacker, {
        sp: ZERO_SP,
        nature: NEUTRAL_NATURE,
        item: null,
        ability: defaultAbility(attacker, master.abilities),
      }),
      defenderSpecies: defender,
      move: physicalMove,
      typeChart: master.typeChart,
      defenderAbilities: defenderAbilityCandidates(
        defender,
        selectableAbilities(defender, master.abilities),
        null,
      ),
    });
    expect(JSON.stringify(request)).toBe(JSON.stringify(expected));
  });

  test("攻撃・特攻の SP の数値が attacker.sp に載り、H・B・D・S は 0 のまま", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "20");
    await setSp(user, "spa", "12");

    const request = await lastRequest(engine, (r) => r.attacker.sp.atk === 20 && r.attacker.sp.spa === 12);
    expect(request.attacker.sp).toEqual({ hp: 0, atk: 20, def: 0, spa: 12, spd: 0, spe: 0 });
    expect(request.attacker.nature).toEqual(NEUTRAL_NATURE);
  });

  test("空欄は 0 として計算する(エラーにしない)", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "20");
    await lastRequest(engine, (r) => r.attacker.sp.atk === 20);

    await setSp(user, "atk", "");

    await lastRequest(engine, (r) => r.attacker.sp.atk === 0);
    expect(spBox("atk")).not.toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("攻撃の性格補正が attacker.nature に載る(上昇 → +A、下降 → −A、補正なしに戻すと無補正)", async () => {
    const { user, engine } = await start();

    await user.click(natureRadio("atk", "上昇"));
    // 例データには A↑・C 補正なしの性格が無いので、物理技が使う A だけを合わせた +A/−C(ADR-0329 §4)
    await lastRequest(engine, (r) => r.attacker.nature.plus === "atk");
    expect(engine.bulkRequests.at(-1)?.attacker.nature).toEqual({ plus: "atk", minus: "spa" });

    await user.click(natureRadio("atk", "下降"));
    await lastRequest(engine, (r) => r.attacker.nature.minus === "atk");
    expect(engine.bulkRequests.at(-1)?.attacker.nature).toEqual({ plus: "def", minus: "atk" });

    await user.click(natureRadio("atk", "補正なし"));
    await lastRequest(engine, (r) => r.attacker.nature.plus === "" && r.attacker.nature.minus === "");
  });

  test("特殊技では特攻の性格補正が効く(C 上昇 → +C/−A)", async () => {
    const { user, engine } = await start();
    await chooseMove(user, SPECIAL);
    await user.click(natureRadio("spa", "上昇"));

    const request = await lastRequest(
      engine,
      (r) => r.move.id === SPECIAL && r.attacker.nature.plus === "spa",
    );
    expect(request.attacker.nature).toEqual({ plus: "spa", minus: "atk" });
  });

  test("A特化を選ぶと従来の A特化と同じ要求(A:32・+A/−C)", async () => {
    const { user, engine } = await start();
    await user.click(presetRadio("atk", "A特化"));

    const request = await lastRequest(engine, (r) => r.attacker.sp.atk === 32);
    expect(request.attacker).toMatchObject({
      sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 },
      nature: { plus: "atk", minus: "spa" },
    });
  });
});

describe("プリセットと数値入力の関係", () => {
  test("プリセットを選ぶと SP 欄と性格補正に値が入る", async () => {
    const { user } = await start();

    await user.click(presetRadio("atk", "A特化"));
    expect(spBox("atk")).toHaveValue("32");
    expect(natureRadio("atk", "上昇")).toBeChecked();
    expect(presetRadio("atk", "A特化")).toBeChecked();

    await user.click(presetRadio("atk", "A振り(無補正)"));
    expect(spBox("atk")).toHaveValue("32");
    expect(natureRadio("atk", "補正なし")).toBeChecked();
    expect(presetRadio("atk", "A振り(無補正)")).toBeChecked();

    await user.click(presetRadio("atk", "無振り"));
    expect(spBox("atk")).toHaveValue("0");
    expect(natureRadio("atk", "補正なし")).toBeChecked();
    expect(presetRadio("atk", "無振り")).toBeChecked();
  });

  test("数値を手で変えてどのプリセットとも一致しなくなると「カスタム」(プリセットはどれも選ばれていない)", async () => {
    const { user } = await start();
    await user.click(presetRadio("atk", "A特化"));

    await setSp(user, "atk", "31");

    for (const radio of within(presetGroup("atk")).getAllByRole("radio")) {
      expect(radio).not.toBeChecked();
    }
    expect(within(block("atk")).getByText("カスタム")).toBeInTheDocument();
    // 性格補正は数値の変更で変わらない
    expect(natureRadio("atk", "上昇")).toBeChecked();

    // 手で 32 に戻すとプリセットと一致して A特化 が選ばれた表示に戻る
    await setSp(user, "atk", "32");
    expect(presetRadio("atk", "A特化")).toBeChecked();
    expect(within(block("atk")).queryByText("カスタム")).toBeNull();
  });

  test("性格補正を手で変えても SP は変わらず、プリセットとの一致で表示が決まる", async () => {
    const { user } = await start();
    await user.click(presetRadio("atk", "A振り(無補正)"));

    await user.click(natureRadio("atk", "上昇"));

    expect(spBox("atk")).toHaveValue("32");
    expect(presetRadio("atk", "A特化")).toBeChecked();
  });

  test("特攻のブロックのプリセットは C 表記(C特化・C振り(無補正))で、特攻だけに入る", async () => {
    const { user } = await start();
    await user.click(presetRadio("spa", "C特化"));

    expect(spBox("spa")).toHaveValue("32");
    expect(natureRadio("spa", "上昇")).toBeChecked();
    expect(spBox("atk")).toHaveValue("0");
    expect(natureRadio("atk", "補正なし")).toBeChecked();
  });
});

describe("実在しない性格の組み合わせを作らない", () => {
  test("特攻を上昇にすると、攻撃の「上昇」と「A特化」は選べない(下降・補正なしは選べる)", async () => {
    const { user } = await start();
    await user.click(natureRadio("spa", "上昇"));

    expect(natureRadio("atk", "上昇")).toBeDisabled();
    expect(presetRadio("atk", "A特化")).toBeDisabled();
    expect(natureRadio("atk", "下降")).toBeEnabled();
    expect(natureRadio("atk", "補正なし")).toBeEnabled();
    expect(presetRadio("atk", "A振り(無補正)")).toBeEnabled();
  });

  test("攻撃を下降にすると、特攻の「下降」は選べない", async () => {
    const { user } = await start();
    await user.click(natureRadio("atk", "下降"));

    expect(natureRadio("spa", "下降")).toBeDisabled();
    expect(natureRadio("spa", "上昇")).toBeEnabled();
  });

  test("A↑・C↓ は選べて、+A/−C を送る", async () => {
    const { user, engine } = await start();
    await user.click(natureRadio("atk", "上昇"));
    await user.click(natureRadio("spa", "下降"));

    expect(natureRadio("spa", "下降")).toBeChecked();
    const request = await lastRequest(engine, (r) => r.attacker.nature.plus === "atk");
    expect(request.attacker.nature).toEqual({ plus: "atk", minus: "spa" });
  });
});

describe("値の寿命(技・攻守入れ替え・種族の変更で消さない)", () => {
  test("技を物理 → 特殊 → 物理と切り替えても両方の値が残り、要求には両方の SP が載る", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "20");
    await user.click(natureRadio("atk", "上昇"));
    await setSp(user, "spa", "12");

    await chooseMove(user, SPECIAL);

    expect(spBox("atk")).toHaveValue("20");
    expect(natureRadio("atk", "上昇")).toBeChecked();
    expect(spBox("spa")).toHaveValue("12");
    const special = await lastRequest(engine, (r) => r.move.id === SPECIAL);
    expect(special.attacker.sp).toEqual({ hp: 0, atk: 20, def: 0, spa: 12, spd: 0, spe: 0 });
    // 特殊技は C を使う。C は補正なしなので、例データでは無補正の性格になる(ADR-0329 §4)
    expect(special.attacker.nature).toEqual(NEUTRAL_NATURE);

    await chooseMove(user, PHYSICAL);

    expect(spBox("atk")).toHaveValue("20");
    expect(spBox("spa")).toHaveValue("12");
    const physical = await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    expect(physical.attacker.nature).toEqual({ plus: "atk", minus: "spa" });
  });

  test("攻守入れ替えでも値を保つ(攻撃側の調整は自分側の設定。旧プリセットの Key と同じ)", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "20");
    await setSp(user, "spa", "12");

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));

    expect(spBox("atk")).toHaveValue("20");
    expect(spBox("spa")).toHaveValue("12");
    const request = await lastRequest(engine, (r) => r.attacker.species.key === defender.key);
    expect(request.attacker.sp).toEqual({ hp: 0, atk: 20, def: 0, spa: 12, spd: 0, spe: 0 });
  });

  test("攻撃側の種族を変えても値を保つ", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "20");
    await user.click(natureRadio("atk", "下降"));

    await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), thirdSpecies.key);

    expect(spBox("atk")).toHaveValue("20");
    expect(natureRadio("atk", "下降")).toBeChecked();
    const request = await lastRequest(engine, (r) => r.attacker.species.key === thirdSpecies.key);
    expect(request.attacker.sp.atk).toBe(20);
    expect(request.attacker.nature.minus).toBe("atk");
  });
});

describe("範囲外・不正な入力(ADR-0316 §4 と同じ明示エラー)", () => {
  test.each(["33", "-1", "1.5", "abc"])(
    "攻撃の SP %j は aria-invalid と alert を出し、計算しない",
    async (text) => {
      const { user, engine } = await start();
      await lastRequest(engine, (r) => r.move.id === PHYSICAL);
      await screen.findByRole("list", { name: "計算結果" });
      await setSp(user, "atk", "");
      await lastRequest(engine, (r) => r.attacker.sp.atk === 0);
      const before = engine.bulkRequests.length;

      await user.click(spBox("atk"));
      await user.paste(text);
      await flush();

      expect(spBox("atk")).toHaveAttribute("aria-invalid", "true");
      expect(screen.getByRole("alert")).toHaveTextContent("攻撃のSPは0〜32の整数で入力してください");
      expect(engine.bulkRequests).toHaveLength(before);
      // 古い結果を出したままにしない
      expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
    },
  );

  test("技が使わない側(物理技での特攻)が不正でも計算しない(要求に両方の SP を載せるため)", async () => {
    const { user, engine } = await start();
    await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    await setSp(user, "spa", "");
    await lastRequest(engine, (r) => r.attacker.sp.spa === 0);
    const before = engine.bulkRequests.length;

    await user.click(spBox("spa"));
    await user.paste("40");
    await flush();

    expect(spBox("spa")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toHaveTextContent("特攻のSPは0〜32の整数で入力してください");
    expect(engine.bulkRequests).toHaveLength(before);
  });

  test("直すとエラーが消えて計算し直す", async () => {
    const { user, engine } = await start();
    await setSp(user, "atk", "33");
    expect(spBox("atk")).toHaveAttribute("aria-invalid", "true");

    await setSp(user, "atk", "32");

    expect(spBox("atk")).not.toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByRole("alert")).toBeNull();
    await lastRequest(engine, (r) => r.attacker.sp.atk === 32);
  });

  test("性格がマスタで解決できなければ alert を出して計算しない", async () => {
    const withoutAtkBoost: MasterData = {
      ...master,
      natures: master.natures.filter((nature) => nature.plus !== "atk"),
    };
    const { user, engine } = await start(undefined, withoutAtkBoost);
    await lastRequest(engine, (r) => r.move.id === PHYSICAL);
    const before = engine.bulkRequests.length;

    await user.click(natureRadio("atk", "上昇"));
    await flush();

    expect(screen.getByRole("alert")).toHaveTextContent(
      "この性格補正の組み合わせに当たる性格が、データにありません",
    );
    expect(engine.bulkRequests).toHaveLength(before);
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
  });
});

describe("古い結果を出さない(CompletedCalc の流儀)", () => {
  test("SP を変えると、応答が届くまで古い行を消して「計算中」を出す", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = await start(engine);
    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(0);
    });
    await act(async () => {
      pending
        .at(-1)
        ?.resolve(ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "古い結果" })] }));
      await Promise.resolve();
    });
    expect(await screen.findByText("古い結果")).toBeInTheDocument();
    const before = pending.length;

    await user.click(spBox("atk"));
    await user.keyboard("{Backspace}");
    await user.paste("20");

    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(before);
    });
    expect(screen.queryByText("古い結果")).toBeNull();
    expect(await screen.findByText("計算中")).toBeInTheDocument();

    await act(async () => {
      pending
        .at(-1)
        ?.resolve(ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "新しい結果" })] }));
      await Promise.resolve();
    });
    expect(await screen.findByText("新しい結果")).toBeInTheDocument();
  });

  test("性格補正を変えると、応答が届くまで古い行を出さない", async () => {
    const { engine, pending } = createDeferredEngine();
    const { user } = await start(engine);
    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(0);
    });
    await act(async () => {
      pending
        .at(-1)
        ?.resolve(ok({ defenderSpeciesKey: defender.key, rows: [bulkRow({ presetLabel: "古い結果" })] }));
      await Promise.resolve();
    });
    expect(await screen.findByText("古い結果")).toBeInTheDocument();

    await user.click(natureRadio("atk", "上昇"));

    expect(screen.queryByText("古い結果")).toBeNull();
    expect(await screen.findByText("計算中")).toBeInTheDocument();
  });
});

describe("実データ相当(25性格)のマスタでも、特化は従来の代表性格(ADR-0010 §R1)になる", () => {
  const stats = ["atk", "def", "spa", "spd", "spe"] as const;
  const fullNatures: MasterNature[] = stats.flatMap((plus) =>
    stats.map((minus): MasterNature =>
      plus === minus
        ? { id: `test-nature-neutral-${plus}`, nameJa: `テスト無補正${plus}`, plus: null, minus: null }
        : { id: `test-nature-${plus}-${minus}`, nameJa: `テスト${plus}${minus}`, plus, minus },
    ),
  );

  test("A特化は +atk/−spa(物理)、C特化は +spa/−atk(特殊)を送る", async () => {
    const { user, engine } = await start(undefined, { ...master, natures: fullNatures });

    await user.click(presetRadio("atk", "A特化"));
    const physical = await lastRequest(engine, (r) => r.attacker.sp.atk === 32);
    expect(physical.attacker.nature).toEqual({ plus: "atk", minus: "spa" });

    await user.click(presetRadio("atk", "無振り"));
    await chooseMove(user, SPECIAL);
    await user.click(presetRadio("spa", "C特化"));
    const special = await lastRequest(engine, (r) => r.move.id === SPECIAL && r.attacker.sp.spa === 32);
    expect(special.attacker.nature).toEqual({ plus: "spa", minus: "atk" });
  });
});

describe("同じ向きにできない理由の説明", () => {
  test("無効な選択肢があるブロックの両ラジオグループが、理由の文を aria-describedby で結ぶ", async () => {
    const { user } = await start();
    expect(presetGroup("atk")).not.toHaveAttribute("aria-describedby");

    await user.click(natureRadio("spa", "上昇"));

    const reason = "攻撃と特攻の両方を上昇、または両方を下降にすることはできません";
    for (const group of [presetGroup("atk"), natureGroup("atk")]) {
      const id = group.getAttribute("aria-describedby");
      expect(id).not.toBeNull();
      expect(document.getElementById(id ?? "")).toHaveTextContent(reason);
    }
  });
});
