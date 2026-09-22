// P4-2: engine に渡すリクエストの組み立て(純粋関数)。形は ADR-0011 §3 の WASM 境界の DTO
// (engine/wasmapi/dto.go・requests.go)。境界は未知のフィールドを拒否する(unknown_field)ので、
// 画面のための追加フィールド(learnset)を engine に渡さないことも確かめる。
// 一括計算は presetKeys / presets を省き、engine の既定(技の分類で HB 系 / HD 系の5行)を使う(ADR-0009、ADR-0300 §6)。

import { describe, expect, test } from "vitest";
import type { Ability, Individual, Item, Move, Nature, TypeChart } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import {
  BATTLE_LEVEL,
  NEUTRAL_NATURE,
  NO_ABILITY,
  ZERO_SP,
  buildBulkRequest,
  buildCalcRequest,
  buildIndividual,
  defaultAbility,
  defenderItemVariants,
  defensiveItemCandidates,
  toEngineSpecies,
} from "./requests";

// 境界の DTO が受け付けるフィールド(engine/wasmapi の json タグ。実装の写しではなく契約から書いた期待値)。
const speciesFields = ["abilities", "baseStats", "dexNo", "form", "key", "nameJa", "types"];
const individualFields = [
  "ability",
  "item",
  "level",
  "nature",
  "ranks",
  "sp",
  "species",
  "status",
  "teraType",
];
const bulkRequestFields = [
  "attacker",
  "critical",
  "defenderSpecies",
  "field",
  "format",
  "itemVariants",
  "move",
  "presetKeys",
  "presets",
  "typeChart",
];
const calcRequestFields = ["attacker", "critical", "defender", "field", "format", "move", "typeChart"];

function expectOnlyFields(value: object, allowed: readonly string[]): void {
  expect(Object.keys(value).filter((key) => !allowed.includes(key))).toEqual([]);
}

const species: MasterSpecies = {
  key: "example-fire",
  dexNo: 9001,
  form: 0,
  nameJa: "テストほのお",
  types: ["fire", "flying"],
  baseStats: { hp: 78, atk: 84, def: 78, spa: 109, spd: 85, spe: 100 },
  abilities: ["example-ability-b", "example-ability-a"],
  learnset: ["example-move-physical"],
};
const abilities: Ability[] = [
  { id: "example-ability-a", nameJa: "テスト特性A", effect: null },
  { id: "example-ability-b", nameJa: "テスト特性B", effect: null },
];
const physicalFire: Move = {
  id: "example-move-physical",
  nameJa: "テスト物理",
  type: "fire",
  category: "physical",
  power: 80,
  priority: 0,
};
const specialFire: Move = {
  ...physicalFire,
  id: "example-move-special",
  nameJa: "テスト特殊",
  category: "special",
};
const specialWater: Move = { ...specialFire, id: "example-move-water", nameJa: "テスト水", type: "water" };
const statusMove: Move = {
  ...physicalFire,
  id: "example-move-status",
  nameJa: "テスト変化",
  category: "status",
  power: 0,
};
const typeChart: TypeChart = {
  types: ["fire", "water"],
  effectiveness: { fire: { water: 1 }, water: { fire: 4 } },
};
const choiceItem: Item = {
  id: "example-item-atk",
  nameJa: "テスト攻撃",
  effect: { statMods: { atk: 6144 } },
};

describe("定数", () => {
  test("レベルは 50 固定(CLAUDE.md ドメイン規約)", () => {
    expect(BATTLE_LEVEL).toBe(50);
  });

  test("無振りの SP はすべて 0、無補正の性格は上昇・下降なし", () => {
    expect(ZERO_SP).toEqual({ hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 });
    expect(NEUTRAL_NATURE).toEqual({ plus: "", minus: "" });
  });
});

describe("toEngineSpecies", () => {
  test("engine の種族の形だけにし、画面のための learnset を落とす", () => {
    const engineSpecies = toEngineSpecies(species);
    expect(Object.keys(engineSpecies).sort()).toEqual(speciesFields);
    expect(engineSpecies).toEqual({
      key: species.key,
      dexNo: species.dexNo,
      form: species.form,
      nameJa: species.nameJa,
      types: species.types,
      baseStats: species.baseStats,
      abilities: species.abilities,
    });
  });
});

describe("defaultAbility", () => {
  test("種族の特性の先頭(一覧に解決できるもの)を使う", () => {
    expect(defaultAbility(species, abilities)).toEqual(abilities[1]);
  });

  test("解決できない ID は飛ばし、1つも解決できなければ特性なし", () => {
    expect(
      defaultAbility({ ...species, abilities: ["example-missing", "example-ability-a"] }, abilities),
    ).toEqual(abilities[0]);
    expect(defaultAbility({ ...species, abilities: [] }, abilities)).toEqual(NO_ABILITY);
    expect(NO_ABILITY).toMatchObject({ id: "" });
  });
});

describe("buildIndividual", () => {
  test("レベル 50・指定の SP・性格・持ち物・特性の個体を作り、種族は engine の形", () => {
    const sp = { ...ZERO_SP, atk: 32 };
    const nature: Nature = { plus: "atk", minus: "spa" };
    const individual = buildIndividual(species, {
      sp,
      nature,
      item: choiceItem,
      ability: abilities[0] as Ability,
    });
    expectOnlyFields(individual, individualFields);
    expect(individual).toMatchObject({
      species: toEngineSpecies(species),
      level: 50,
      sp,
      nature,
      item: choiceItem,
      ability: abilities[0],
    });
    expect(Object.keys(individual.species)).not.toContain("learnset");
  });

  test("持ち物なしは null(境界の item は null で「なし」)", () => {
    const individual = buildIndividual(species, {
      sp: ZERO_SP,
      nature: NEUTRAL_NATURE,
      item: null,
      ability: NO_ABILITY,
    });
    expect(individual.item).toBeNull();
  });

  test("ランク補正を持つならすべて 0(P4-2 ではランクを入力しない)", () => {
    const individual: Individual = buildIndividual(species, {
      sp: ZERO_SP,
      nature: NEUTRAL_NATURE,
      item: null,
      ability: NO_ABILITY,
    });
    // どちらの場合でも必ず何かを assert する(ranks を省くなら undefined であること、
    // 持たせるならすべて 0 であること)。条件の外に出ない assert だけの空振りを避ける。
    if (individual.ranks === undefined) {
      expect(individual.ranks).toBeUndefined();
    } else {
      expect(individual.ranks).toEqual({ atk: 0, def: 0, spa: 0, spd: 0, spe: 0 });
    }
  });
});

describe("buildBulkRequest", () => {
  const attacker = buildIndividual(species, {
    sp: ZERO_SP,
    nature: NEUTRAL_NATURE,
    item: null,
    ability: NO_ABILITY,
  });

  test("presetKeys・presets を省いて engine の既定の5行にし、typeChart を必ず含める", () => {
    const request = buildBulkRequest({ attacker, defenderSpecies: species, move: physicalFire, typeChart });
    expectOnlyFields(request, bulkRequestFields);
    expect(request).not.toHaveProperty("presetKeys");
    expect(request).not.toHaveProperty("presets");
    expect(request).not.toHaveProperty("itemVariants");
    expect(request).toMatchObject({
      format: "single",
      attacker,
      defenderSpecies: toEngineSpecies(species),
      move: physicalFire,
      typeChart,
    });
    expect(Object.keys(request.defenderSpecies)).not.toContain("learnset");
  });

  test("持ち物のバリアントを渡すとそのまま入る(null は「持ち物なし」として JSON でも null のまま)", () => {
    const request = buildBulkRequest({
      attacker,
      defenderSpecies: species,
      move: physicalFire,
      typeChart,
      itemVariants: [null, choiceItem],
    });
    expect(request.itemVariants).toEqual([null, choiceItem]);
    const roundTripped = JSON.parse(JSON.stringify(request)) as { itemVariants?: unknown };
    expect(roundTripped.itemVariants).toEqual([null, choiceItem]);
  });
});

describe("buildCalcRequest", () => {
  test("1対1の計算リクエスト(形式 single・typeChart 必須)", () => {
    const attacker = buildIndividual(species, {
      sp: ZERO_SP,
      nature: NEUTRAL_NATURE,
      item: null,
      ability: NO_ABILITY,
    });
    const request = buildCalcRequest({ attacker, defender: attacker, move: specialFire, typeChart });
    expectOnlyFields(request, calcRequestFields);
    expect(request).toMatchObject({
      format: "single",
      attacker,
      defender: attacker,
      move: specialFire,
      typeChart,
    });
  });
});

describe("defensiveItemCandidates(ADR-0300 §6。効果データから選び、ID・名前では選ばない)", () => {
  const items: Item[] = [
    { id: "example-def", nameJa: "テスト防御", effect: { statMods: { def: 6144 } } },
    { id: "example-spd", nameJa: "テスト特防", effect: { statMods: { spd: 6144 } } },
    { id: "example-both", nameJa: "テスト両方", effect: { statMods: { def: 6144, spd: 6144 } } },
    { id: "example-def-down", nameJa: "テスト防御ダウン", effect: { statMods: { def: 2048 } } },
    choiceItem,
    { id: "example-fire-berry", nameJa: "テストほのおきのみ", effect: { resistBerryType: "fire" } },
    { id: "example-water-berry", nameJa: "テストみずきのみ", effect: { resistBerryType: "water" } },
    { id: "example-damage", nameJa: "テストダメージ", effect: { damageMod: 5324 } },
    // 名前は防御を上げそうでも、効果が無ければ候補にしない
    { id: "example-def-name-only", nameJa: "テスト防御アップ", effect: null },
  ];

  test.each([
    ["物理・ほのお", physicalFire, ["example-def", "example-both", "example-fire-berry"]],
    ["特殊・ほのお", specialFire, ["example-spd", "example-both", "example-fire-berry"]],
    ["特殊・みず", specialWater, ["example-spd", "example-both", "example-water-berry"]],
    ["変化技", statusMove, []],
  ] as const)(
    "%s の技には、対応する防御側ステータスを上げる持ち物と技のタイプの半減きのみ(マスタの順)",
    (_label, move, ids) => {
      expect(defensiveItemCandidates(items, move).map((item) => item.id)).toEqual(ids);
    },
  );
});

describe("defenderItemVariants(防御側の持ち物の選択と「持ち物の候補も比較」のトグル)", () => {
  const defenseItem: Item = { id: "example-def", nameJa: "テスト防御", effect: { statMods: { def: 6144 } } };
  const berry: Item = { id: "example-berry", nameJa: "テストきのみ", effect: { resistBerryType: "fire" } };
  const candidates = [defenseItem, berry];

  test.each([
    ["比較なし・持ち物なし → 指定しない(engine は持ち物なしで5行)", null, false, undefined],
    ["比較なし・持ち物あり → その持ち物だけ", choiceItem, false, [choiceItem]],
    ["比較あり・持ち物なし → なし + 候補", null, true, [null, defenseItem, berry]],
    ["比較あり・候補にある持ち物 → 重複させない", berry, true, [null, defenseItem, berry]],
    [
      "比較あり・候補にない持ち物 → なし・選んだ持ち物・候補の順",
      choiceItem,
      true,
      [null, choiceItem, defenseItem, berry],
    ],
  ] as const)("%s", (_label, selectedItem, compare, expected) => {
    expect(defenderItemVariants({ selectedItem, compare, candidates })).toEqual(expected);
  });
});
