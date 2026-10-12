// CB-6: 連続技の組み立て(buildComboRequest)とランク変化の判定(changesRanks ほか)。

import { describe, expect, test } from "vitest";
import type { Individual, Move, Species, StatKey, TypeChart } from "../engine/types";
import {
  DEFAULT_SELF_SPEED,
  MAX_COMBO_STEPS,
  applySelfSpeed,
  buildComboRequest,
  canBeComboStep,
  canBeOpponentMove,
  changesRanks,
  maxSelfSpeedSp,
  rankEffectOf,
} from "./combo";

const typeChart: TypeChart = { types: ["normal"], effectiveness: {} };
const species: Species = {
  key: "9002-000",
  dexNo: 9002,
  form: 0,
  nameJa: "テストみず",
  types: ["normal"],
  baseStats: { hp: 90, atk: 75, def: 80, spa: 95, spd: 85, spe: 70 },
  abilities: [],
};
const individual: Individual = {
  species,
  level: 50,
  nature: { plus: "", minus: "" },
  ability: { id: "", nameJa: "", effect: null },
  item: null,
  sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
};

function move(id: string, extra: Partial<Move> = {}): Move {
  return { id, nameJa: id, type: "normal", category: "physical", power: 80, priority: 0, ...extra };
}

const attack = move("attack");
const dance = move("dance", {
  category: "status",
  power: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1, spe: 1 } },
});
const noopStatus = move("noop", { category: "status", power: 0 });

describe("ランクが変わる技の判定", () => {
  test("effect が無い・null・stages が空・全部 0 は変わらない", () => {
    expect(changesRanks(attack)).toBe(false);
    expect(changesRanks(move("n", { effect: null }))).toBe(false);
    expect(changesRanks(move("e", { effect: { chance: 100, target: "self", stages: {} } }))).toBe(false);
    expect(changesRanks(move("z", { effect: { chance: 100, target: "self", stages: { atk: 0 } } }))).toBe(
      false,
    );
  });

  test("stages に 0 でない値があれば変わる(下がる技も含む)", () => {
    expect(changesRanks(dance)).toBe(true);
    expect(changesRanks(move("down", { effect: { chance: 100, target: "self", stages: { def: -1 } } }))).toBe(
      true,
    );
    expect(rankEffectOf(dance)).toBe(dance.effect);
    expect(rankEffectOf(attack)).toBeNull();
  });

  test("連続技の手: 攻撃技と、ランクを変える変化技は置ける。ランク変化の無い変化技は置けない", () => {
    expect(canBeComboStep(attack)).toBe(true);
    expect(canBeComboStep(dance)).toBe(true);
    expect(canBeComboStep(noopStatus)).toBe(false);
  });

  test("相手の技は攻撃技だけ", () => {
    expect(canBeOpponentMove(attack)).toBe(true);
    expect(canBeOpponentMove(dance)).toBe(false);
  });
});

describe("buildComboRequest", () => {
  const base = { attacker: individual, defender: individual, typeChart };

  test("最小の入力: format は single、任意の項目はキーを作らない", () => {
    const request = buildComboRequest({ ...base, steps: [{ move: attack }] });
    expect(request).toEqual({
      format: "single",
      attacker: individual,
      defender: individual,
      steps: [{ move: attack }],
      typeChart,
    });
    expect(Object.keys(request)).not.toContain("opponentMove");
    expect(Object.keys(request)).not.toContain("speedField");
  });

  test("急所は true のときだけ、多段の回数は指定があるときだけ載せる", () => {
    const request = buildComboRequest({
      ...base,
      steps: [
        { move: attack, critical: false },
        { move: attack, critical: true, hits: 3 },
      ],
    });
    expect(request.steps).toEqual([{ move: attack }, { move: attack, critical: true, hits: 3 }]);
  });

  test("場・相手の技・相手の素早さ・素早さの場を渡す。素早さの場が全部 false なら送らない", () => {
    const withAll = buildComboRequest({
      ...base,
      steps: [{ move: dance }, { move: attack }],
      field: { weather: "sun" },
      opponentMove: attack,
      opponentSpeed: "from_defender",
      speedField: { selfTailwind: true },
    });
    expect(withAll).toMatchObject({
      field: { weather: "sun" },
      opponentMove: attack,
      opponentSpeed: "from_defender",
      speedField: { selfTailwind: true },
    });
    const allFalse = buildComboRequest({
      ...base,
      steps: [{ move: attack }],
      speedField: { trickRoom: false, selfTailwind: false, opponentTailwind: false },
    });
    expect(Object.keys(allFalse)).not.toContain("speedField");
  });

  test(`手は先頭から ${String(MAX_COMBO_STEPS)} 件まで`, () => {
    expect(MAX_COMBO_STEPS).toBe(3);
    const steps = ["a", "b", "c", "d"].map((id) => ({ move: move(id) }));
    const request = buildComboRequest({ ...base, steps });
    expect(request.steps.map((step) => step.move.id)).toEqual(["a", "b", "c"]);
  });
});

describe("applySelfSpeed(連続技の自分の素早さ。I-web-32)", () => {
  const withNature = (plus: StatKey | "", minus: StatKey | ""): Individual => ({
    ...individual,
    nature: { plus, minus },
    sp: { ...individual.sp, atk: 32 },
  });

  test("既定(SP 0・性格そのまま・ランク 0)は個体をそのまま返す(今の挙動と同一)", () => {
    const base = withNature("atk", "spa");
    expect(applySelfSpeed(base, DEFAULT_SELF_SPEED)).toBe(base);
  });

  test("SP と ランクを素早さだけ上書きする(他の SP・ランクは残す)", () => {
    const base: Individual = { ...withNature("", ""), ranks: { atk: 2, def: 0, spa: 0, spd: 0, spe: 0 } };
    const result = applySelfSpeed(base, { sp: 20, nature: "keep", rank: 1 });
    expect(result.sp).toEqual({ ...base.sp, spe: 20 });
    expect(result.ranks).toEqual({ atk: 2, def: 0, spa: 0, spd: 0, spe: 1 });
  });

  test("ランクが無い個体にランクを足すと他は 0", () => {
    const result = applySelfSpeed(withNature("", ""), { sp: 0, nature: "keep", rank: -2 });
    expect(result.ranks).toEqual({ atk: 0, def: 0, spa: 0, spd: 0, spe: -2 });
  });

  test("SP は 1 能力 32 と合計 66 に収める", () => {
    const base: Individual = { ...individual, sp: { ...individual.sp, atk: 32, spa: 30 } };
    expect(applySelfSpeed(base, { sp: 99, nature: "keep", rank: 0 }).sp.spe).toBe(4);
    expect(maxSelfSpeedSp(base)).toBe(4);
    expect(maxSelfSpeedSp(individual)).toBe(32);
  });

  test.each([
    ["atk", "spa", "plus", "spe", "spa"],
    ["", "", "plus", "spe", "spa"],
    ["spe", "atk", "plus", "spe", "atk"],
    ["atk", "spe", "plus", "spe", "spa"],
    ["spa", "def", "plus", "spe", "def"],
    ["atk", "spa", "minus", "atk", "spe"],
    ["", "", "minus", "def", "spe"],
    ["spe", "atk", "minus", "def", "spe"],
    ["atk", "spe", "neutral", "", ""],
    ["spe", "atk", "neutral", "", ""],
    ["atk", "spa", "neutral", "atk", "spa"],
  ] as const)("性格 +%s -%s に素早さ %s を指定 → +%s -%s", (plus, minus, choice, expectPlus, expectMinus) => {
    const result = applySelfSpeed(withNature(plus, minus), { sp: 0, nature: choice, rank: 0 });
    expect(result.nature).toEqual({ plus: expectPlus, minus: expectMinus });
  });
});
