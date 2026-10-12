// CB-6: 連続技の組み立て(buildComboRequest)とランク変化の判定(changesRanks ほか)。

import { describe, expect, test } from "vitest";
import type { Individual, Move, Species, TypeChart } from "../engine/types";
import {
  MAX_COMBO_STEPS,
  buildComboRequest,
  canBeComboStep,
  canBeOpponentMove,
  changesRanks,
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
