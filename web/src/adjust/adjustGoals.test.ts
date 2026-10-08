// F-11(ADR-0331 §5・§6・受け入れ条件 AC5): 目標の定数と、要求を組み立てる純粋関数。
// 確かめること:
//   P1 素早さのプリセット(最速 = S 32・素早さ上昇、準速 = S 32・補正なし、無振り = 0・補正なし。ほかの能力は 0)
//   P2 目標の相手の Individual(種類ごとのプリセット・性格の ID・メガはストーン・性格が無ければ null)
//   P3 AdjustGoal の組み立て(種類ごとに使う欄だけ。しきい値 100 と「先に使う技なし」は送らない)
//   P4 定数(上限は契約の maxItems と同じ 6・種類の並び・既定)
// 期待値は ADR-0331 §5 の表と api/openapi.yaml から手で書く(実装の写しにしない)。架空データだけを使う。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import type { Item } from "../engine/types";
import type { MasterNature, MasterSpecies } from "../master/types";
import {
  ADJUST_GOALS_ENABLED,
  ADJUST_GOAL_KINDS,
  DEFAULT_ADJUST_GOAL_KIND,
  DEFAULT_KO_PRESET,
  DEFAULT_SPEED_PRESET,
  DEFAULT_SURVIVE_PRESET,
  MAX_ADJUST_GOALS,
  SPEED_PRESET_KEYS,
  buildGoalRequest,
  goalOpponentIndividual,
  resolveSpeedPreset,
} from "./adjustGoals";

type Schemas = components["schemas"];

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

const NATURE_NEUTRAL: MasterNature = {
  id: "test-nature-neutral",
  nameJa: "テストまじめ",
  plus: null,
  minus: null,
};
const NATURE_PLUS_SPE: MasterNature = {
  id: "test-nature-plus-spe",
  nameJa: "テストおくびょう",
  plus: "spe",
  minus: "atk",
};
const NATURE_PLUS_ATK: MasterNature = {
  id: "test-nature-plus-atk",
  nameJa: "テストいじっぱり",
  plus: "atk",
  minus: "spa",
};
const NATURE_PLUS_SPA: MasterNature = {
  id: "test-nature-plus-spa",
  nameJa: "テストひかえめ",
  plus: "spa",
  minus: "atk",
};
const NATURE_PLUS_DEF: MasterNature = {
  id: "test-nature-plus-def",
  nameJa: "テストわんぱく",
  plus: "def",
  minus: "atk",
};
const NATURES = [NATURE_NEUTRAL, NATURE_PLUS_SPE, NATURE_PLUS_ATK, NATURE_PLUS_SPA, NATURE_PLUS_DEF];

const STONE: Item = { id: "test-megastone-a", nameJa: "テストメガナイト", effect: null };
const ITEMS: readonly Item[] = [{ id: "test-item-a", nameJa: "テストもちもの", effect: null }, STONE];

function species(key: string, extra: Partial<MasterSpecies> = {}): MasterSpecies {
  return {
    key,
    dexNo: Number(key.slice(0, 4)),
    form: Number(key.slice(5)),
    nameJa: "テストカソウギョ",
    types: ["water"],
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
    abilities: [],
    learnset: [],
    ...extra,
  };
}

const FISH = species("9002-000");
const MEGA = species("9102-001", { isMega: true, requiredItemId: STONE.id });
const ORPHAN = species("9103-001", { isMega: true, requiredItemId: "test-megastone-missing" });

describe("P4 定数", () => {
  test("目標の上限は契約の AdjustGoalsRequest.goals.maxItems と同じ 6", () => {
    expect(MAX_ADJUST_GOALS).toBe(6);
  });

  test("種類の並びは 素早さ・耐える・倒す、既定は素早さ", () => {
    expect(ADJUST_GOAL_KINDS).toEqual(["outspeed", "survive", "ko"]);
    expect(DEFAULT_ADJUST_GOAL_KIND).toBe("outspeed");
  });

  test("相手の振り方の既定: 素早さは最速・耐えるは特化・倒すは無振り", () => {
    expect(DEFAULT_SPEED_PRESET).toBe("fastest");
    expect(DEFAULT_SURVIVE_PRESET).toBe("x_full");
    expect(DEFAULT_KO_PRESET).toBe("none");
  });

  // 段階 B(calc-svc の adjustGoals。ADR-0177 §10)で有効にする(ADR-0331 §2 の仕様の変更)。
  test("段階 B(calc-svc の adjustGoals)が入ったのでモードを出す", () => {
    expect(ADJUST_GOALS_ENABLED).toBe(true);
  });
});

describe("P1 素早さのプリセット", () => {
  test("並びは速い順(最速・準速・無振り)", () => {
    expect(SPEED_PRESET_KEYS).toEqual(["fastest", "neutral_max", "none"]);
  });

  test.each([
    ["fastest", { ...ZERO, spe: 32 }, { plus: "spe", minus: "atk" }],
    ["neutral_max", { ...ZERO, spe: 32 }, { plus: "", minus: "" }],
    ["none", { ...ZERO }, { plus: "", minus: "" }],
  ] as const)("%s → SP %j・性格 %j", (key, sp, nature) => {
    expect(resolveSpeedPreset(key)).toEqual({ sp, nature });
  });

  test("呼ぶたびに新しいオブジェクト(共有の定数を書き換えない)", () => {
    const first = resolveSpeedPreset("fastest");
    const second = resolveSpeedPreset("fastest");
    expect(first).not.toBe(second);
    expect(first.sp).not.toBe(second.sp);
  });
});

describe("P2 目標の相手の Individual", () => {
  test("素早さ・最速: S 32 と素早さ上昇の性格(持ち物は送らない)", () => {
    expect(
      goalOpponentIndividual({
        species: FISH,
        preset: { kind: "outspeed", key: "fastest" },
        natures: NATURES,
        items: ITEMS,
      }),
    ).toEqual({
      speciesKey: FISH.key,
      level: 50,
      natureId: NATURE_PLUS_SPE.id,
      sp: { ...ZERO, spe: 32 },
    } satisfies Schemas["Individual"]);
  });

  test("素早さ・準速: 補正なしの性格はマスタの先頭の無補正", () => {
    expect(
      goalOpponentIndividual({
        species: FISH,
        preset: { kind: "outspeed", key: "neutral_max" },
        natures: NATURES,
        items: ITEMS,
      }),
    ).toEqual({ speciesKey: FISH.key, level: 50, natureId: NATURE_NEUTRAL.id, sp: { ...ZERO, spe: 32 } });
  });

  test.each([
    ["physical", NATURE_PLUS_ATK, { ...ZERO, atk: 32 }],
    ["special", NATURE_PLUS_SPA, { ...ZERO, spa: 32 }],
  ] as const)("耐える・特化: 相手の技が %s なら %s の性格と、その能力に 32", (category, nature, sp) => {
    expect(
      goalOpponentIndividual({
        species: FISH,
        preset: { kind: "survive", key: "x_full", category },
        natures: NATURES,
        items: ITEMS,
      }),
    ).toEqual({ speciesKey: FISH.key, level: 50, natureId: nature.id, sp });
  });

  test("倒す・HB特化: 防御側プリセット(H 32・B 32・防御上昇)", () => {
    expect(
      goalOpponentIndividual({
        species: FISH,
        preset: { kind: "ko", key: "hb_full" },
        natures: NATURES,
        items: ITEMS,
      }),
    ).toEqual({
      speciesKey: FISH.key,
      level: 50,
      natureId: NATURE_PLUS_DEF.id,
      sp: { ...ZERO, hp: 32, def: 32 },
    });
  });

  test("メガ種族はストーンの itemId を付ける(ADR-0331 §1)", () => {
    const individual = goalOpponentIndividual({
      species: MEGA,
      preset: { kind: "outspeed", key: "none" },
      natures: NATURES,
      items: ITEMS,
    });
    expect(individual?.itemId).toBe(STONE.id);
  });

  test("ストーンをマスタから引けないメガ種族は itemId を付けない(別の持ち物で代えない)", () => {
    const individual = goalOpponentIndividual({
      species: ORPHAN,
      preset: { kind: "outspeed", key: "none" },
      natures: NATURES,
      items: ITEMS,
    });
    expect(individual).not.toBeNull();
    expect(individual).not.toHaveProperty("itemId");
  });

  test("プリセットの性格がマスタに無ければ null(別の性格で代えない)", () => {
    expect(
      goalOpponentIndividual({
        species: FISH,
        preset: { kind: "outspeed", key: "fastest" },
        natures: [NATURE_NEUTRAL],
        items: ITEMS,
      }),
    ).toBeNull();
  });
});

describe("P3 AdjustGoal の組み立て", () => {
  const opponent: Schemas["Individual"] = {
    speciesKey: FISH.key,
    level: 50,
    natureId: NATURE_NEUTRAL.id,
    sp: { ...ZERO },
  };

  test("素早さ: 先に使う技なしなら kind と opponent だけ(hits・しきい値は送らない)", () => {
    expect(
      buildGoalRequest({ kind: "outspeed", opponent, moveId: null, hits: 3, thresholdPercent: 50 }),
    ).toEqual({
      kind: "outspeed",
      opponent,
    } satisfies Schemas["AdjustGoal"]);
  });

  test("素早さ: 先に使う技を選んだら moveId を送る", () => {
    expect(
      buildGoalRequest({
        kind: "outspeed",
        opponent,
        moveId: "test-move-boost",
        hits: 1,
        thresholdPercent: 100,
      }),
    ).toEqual({ kind: "outspeed", opponent, moveId: "test-move-boost" });
  });

  test.each(["survive", "ko"] as const)("%s: moveId・hits を送り、しきい値 100(確定)は省略する", (kind) => {
    expect(
      buildGoalRequest({ kind, opponent, moveId: "test-move-fire", hits: 2, thresholdPercent: 100 }),
    ).toEqual({
      kind,
      opponent,
      moveId: "test-move-fire",
      hits: 2,
    });
  });

  test.each(["survive", "ko"] as const)("%s: しきい値が 100 以外なら thresholdPercent を送る", (kind) => {
    expect(
      buildGoalRequest({ kind, opponent, moveId: "test-move-fire", hits: 1, thresholdPercent: 75 }),
    ).toEqual({
      kind,
      opponent,
      moveId: "test-move-fire",
      hits: 1,
      thresholdPercent: 75,
    });
  });

  test("入力の opponent を書き換えない", () => {
    const before = structuredClone(opponent);
    buildGoalRequest({ kind: "ko", opponent, moveId: "test-move-fire", hits: 1, thresholdPercent: 50 });
    expect(opponent).toEqual(before);
  });
});
