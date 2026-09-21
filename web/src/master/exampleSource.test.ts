// P4-2: 架空の例データ(web/src/master/example/)と exampleMasterSource(ADR-0016 §3)。
// 実マスタを Git に置かない(ADR-0002)ため、名前は「テスト」で始め、ID は「example-」で始め、
// 図鑑番号は実在と重ならない 9001 以降にする。タイプ相性表だけは架空にせず @typechart を読む。
// 形(DTO の契約どおりか)は wasmEngine.wasm.test.ts が本物の engine に通して確かめる。

import typeChartData from "@typechart";
import { beforeAll, describe, expect, test } from "vitest";
import { typeIds } from "../test/typeChart";
import { exampleMasterSource } from "./exampleSource";
import { typeChartFromData } from "./typeChart";
import type { MasterData } from "./types";

/** 架空データの名前・ID・図鑑番号の規則(ADR-0016 §3)。 */
const namePrefix = "テスト";
const idPrefix = "example-";
const firstFictionalDexNo = 9001;
/** engine の補正値の等倍(4096 基準。ADR-0004)。これより大きい倍率が「上げる」効果。 */
const modifierBase = 4096;
const statKeys = ["hp", "atk", "def", "spa", "spd", "spe"] as const;
const categories = ["physical", "special", "status"];

let master: MasterData;
let validTypes: Set<string>;

beforeAll(async () => {
  master = await exampleMasterSource.load();
  validTypes = new Set(typeIds());
});

function expectUnique(values: readonly string[]): void {
  expect(new Set(values).size).toBe(values.length);
}

describe("共通の規則", () => {
  test.each(["species", "moves", "items", "abilities"] as const)(
    "%s の名前は「テスト」で始まり、ID は「example-」で始まり、ID は重複しない",
    (collection) => {
      const entries = master[collection].map((entry) => ({
        id: "key" in entry ? entry.key : entry.id,
        nameJa: entry.nameJa,
      }));
      expect(entries.length).toBeGreaterThan(0);
      for (const entry of entries) {
        expect(entry.nameJa.startsWith(namePrefix), entry.nameJa).toBe(true);
        expect(entry.id.startsWith(idPrefix), entry.id).toBe(true);
      }
      expectUnique(entries.map((entry) => entry.id));
    },
  );

  test("相性表は @typechart のデータ(架空にしない)を engine の DTO にしたもの", () => {
    expect(master.typeChart).toEqual(typeChartFromData(typeChartData));
  });
});

describe("種族", () => {
  test("6種族以上で、タイプは5種類以上にまたがる", () => {
    expect(master.species.length).toBeGreaterThanOrEqual(6);
    expect(new Set(master.species.flatMap((species) => species.types)).size).toBeGreaterThanOrEqual(5);
  });

  test("図鑑番号は 9001 以降", () => {
    for (const species of master.species) {
      expect(species.dexNo, species.key).toBeGreaterThanOrEqual(firstFictionalDexNo);
    }
  });

  test("タイプは相性表にある ID で1〜2個、重複しない", () => {
    for (const species of master.species) {
      expect(species.types.length, species.key).toBeGreaterThanOrEqual(1);
      expect(species.types.length, species.key).toBeLessThanOrEqual(2);
      expectUnique(species.types);
      for (const type of species.types) {
        expect(validTypes.has(type), `${species.key}: ${type}`).toBe(true);
      }
    }
  });

  test("種族値は6ステータスとも正の整数", () => {
    for (const species of master.species) {
      for (const key of statKeys) {
        const value = species.baseStats[key];
        expect(Number.isInteger(value) && value > 0, `${species.key}.${key}`).toBe(true);
      }
    }
  });

  test("特性を1つ以上持ち、すべて特性の一覧に解決できる", () => {
    const abilityIds = new Set(master.abilities.map((ability) => ability.id));
    for (const species of master.species) {
      expect(species.abilities.length, species.key).toBeGreaterThan(0);
      for (const id of species.abilities) {
        expect(abilityIds.has(id), `${species.key}: ${id}`).toBe(true);
      }
    }
  });

  test("覚える技(learnset)はすべて技の一覧に解決でき、ダメージ技を1つ以上含む", () => {
    const movesById = new Map(master.moves.map((move) => [move.id, move]));
    for (const species of master.species) {
      expect(species.learnset.length, species.key).toBeGreaterThan(0);
      expectUnique(species.learnset);
      for (const id of species.learnset) {
        expect(movesById.has(id), `${species.key}: ${id}`).toBe(true);
      }
      const damaging = species.learnset.filter((id) => movesById.get(id)?.category !== "status");
      expect(damaging.length, species.key).toBeGreaterThan(0);
    }
  });

  test("画面の確認に要る組み合わせがある: ダメージ技を2つ以上覚える種族、変化技を覚える種族", () => {
    const movesById = new Map(master.moves.map((move) => [move.id, move]));
    const categoriesOf = (learnset: readonly string[]) => learnset.map((id) => movesById.get(id)?.category);
    expect(
      master.species.some(
        (species) => categoriesOf(species.learnset).filter((category) => category !== "status").length >= 2,
      ),
    ).toBe(true);
    expect(master.species.some((species) => categoriesOf(species.learnset).includes("status"))).toBe(true);
  });
});

describe("技", () => {
  test("分類は physical / special / status、タイプは相性表にある ID", () => {
    for (const move of master.moves) {
      expect(categories).toContain(move.category);
      expect(validTypes.has(move.type), `${move.id}: ${move.type}`).toBe(true);
    }
  });

  test("物理・特殊・変化がそれぞれあり、ダメージ技のタイプは3種類以上", () => {
    const byCategory = (category: string) => master.moves.filter((move) => move.category === category);
    expect(byCategory("physical").length).toBeGreaterThan(0);
    expect(byCategory("special").length).toBeGreaterThan(0);
    expect(byCategory("status").length).toBeGreaterThan(0);
    const damagingTypes = new Set(
      master.moves.filter((move) => move.category !== "status").map((move) => move.type),
    );
    expect(damagingTypes.size).toBeGreaterThanOrEqual(3);
  });

  test("ダメージ技の威力は正、変化技の威力は 0", () => {
    for (const move of master.moves) {
      if (move.category === "status") {
        expect(move.power, move.id).toBe(0);
      } else {
        expect(move.power, move.id).toBeGreaterThan(0);
      }
    }
  });
});

describe("持ち物(効果は engine の効果定義の形の架空の値)", () => {
  test("防御を上げる・特防を上げる・半減きのみ・ダメージ倍率の持ち物がそれぞれある", () => {
    const effects = master.items.map((item) => item.effect);
    expect(effects.some((effect) => (effect?.statMods?.def ?? 0) > modifierBase)).toBe(true);
    expect(effects.some((effect) => (effect?.statMods?.spd ?? 0) > modifierBase)).toBe(true);
    expect(effects.some((effect) => (effect?.damageMod ?? 0) > modifierBase)).toBe(true);
    expect(effects.some((effect) => (effect?.resistBerryType ?? "") !== "")).toBe(true);
  });

  test("半減きのみのタイプは相性表にある ID で、そのタイプのダメージ技が例データにある", () => {
    const damagingTypes = new Set(
      master.moves.filter((move) => move.category !== "status").map((move) => move.type),
    );
    const berryTypes = master.items.flatMap((item) => {
      const type = item.effect?.resistBerryType ?? "";
      return type === "" ? [] : [type];
    });
    for (const type of berryTypes) {
      expect(validTypes.has(type), type).toBe(true);
    }
    expect(berryTypes.some((type) => damagingTypes.has(type))).toBe(true);
  });
});
