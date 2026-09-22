// P4-12a(ADR-0303 §4): Web の例データを balance-svc の read model(services/balance/schema/ の3つ)にも書き出す。
// pokedex の read model に揃うまでの間、ローカルと E2E ではこの出力で balance-svc を起動し、Web の例データの ID
// (pokemonId = 種族キー・moveId・abilityId)がそのまま balance の API に通るようにする。
// 形は services/balance/schema/*.schema.json(ADR-0402)を読んで確かめる(必須キー・許されたキー・enum・pattern)。
// 特性: engine の効果定義から balance の正規化された効果に写せるものだけを書く(ADR-0303 §4)。
//   defResistType[t] = 0        → {kind: "immune", attackType: t}
//   defResistType[t] = m (> 0)  → {kind: "type_multiplier", attackType: t, numerator/denominator = m/4096 の既約分数}
//   reduceSuperEffective = r    → {kind: "super_effective_multiplier", numerator/denominator = r/4096 の既約分数}
//   既約分数の分子・分母が 1〜16 に収まらない効果は書かない。それ以外の効果(stabMod など)は書かない。
//   画面はどの特性も選べる(analyze は read model に無い abilityId を 422 にする)ので、特性はすべて書き、
//   写せる効果が無い特性は effects: [] にする。

import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { beforeAll, describe, expect, test } from "vitest";
import type { Ability } from "../engine/types";
import { localPath } from "../test/localPath";
import { exampleMasterSource } from "./exampleSource";
import { toBalanceAbilities, toBalanceMoves, toBalancePokemonTypes } from "./exportBalanceReadModel";
import type { MasterData } from "./types";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

/** JSON Schema のうち、ここで確かめる部分だけ(型は最小限。スキーマ全体の検証器は入れない)。 */
interface SchemaNode {
  readonly type?: string | readonly string[];
  readonly required?: readonly string[];
  readonly properties?: Readonly<Record<string, SchemaNode>>;
  readonly additionalProperties?: boolean;
  readonly items?: SchemaNode;
  readonly enum?: readonly string[];
  readonly const?: unknown;
  readonly pattern?: string;
  readonly minItems?: number;
  readonly maxItems?: number;
  readonly minimum?: number;
  readonly maximum?: number;
  readonly maxLength?: number;
  readonly $ref?: string;
  readonly $defs?: Readonly<Record<string, SchemaNode>>;
}

function loadSchema(name: string): SchemaNode {
  const path = localPath(`../../../services/balance/schema/${name}`, import.meta.url);
  return JSON.parse(readFileSync(path, "utf8")) as SchemaNode;
}

/**
 * 値をスキーマ(の一部。$ref は #/$defs/ だけ)で確かめ、違反を文字列の配列で返す。
 * 確かめるもの: type(object/array/string/integer/null)・required・additionalProperties: false・enum・const・
 * pattern・maxLength・minItems/maxItems・minimum/maximum。if/then(効果の種類ごとの必須)は別のテストで確かめる。
 */
function violations(root: SchemaNode, node: SchemaNode, value: unknown, path: string): string[] {
  if (node.$ref !== undefined) {
    const name = node.$ref.replace("#/$defs/", "");
    const target = root.$defs?.[name];
    if (target === undefined) {
      return [`${path}: 解決できない $ref ${node.$ref}`];
    }
    return violations(root, target, value, path);
  }
  const errors: string[] = [];
  const types = node.type === undefined ? [] : typeof node.type === "string" ? [node.type] : node.type;
  if (value === null) {
    return types.length === 0 || types.includes("null") ? [] : [`${path}: null は許されない`];
  }
  if (node.const !== undefined && value !== node.const) {
    errors.push(`${path}: ${JSON.stringify(value)} は const ${JSON.stringify(node.const)} と違う`);
  }
  if (node.enum !== undefined && (typeof value !== "string" || !node.enum.includes(value))) {
    errors.push(`${path}: ${JSON.stringify(value)} は enum に無い`);
  }
  if (typeof value === "string") {
    if (types.length > 0 && !types.includes("string")) {
      errors.push(`${path}: 文字列は許されない`);
    }
    if (node.pattern !== undefined && !new RegExp(node.pattern, "u").test(value)) {
      errors.push(`${path}: ${value} は pattern ${node.pattern} に合わない`);
    }
    if (node.maxLength !== undefined && value.length > node.maxLength) {
      errors.push(`${path}: 長さが ${String(node.maxLength)} を超える`);
    }
  } else if (typeof value === "number") {
    if (!Number.isInteger(value) || (types.length > 0 && !types.includes("integer"))) {
      errors.push(`${path}: 整数でない、または数は許されない`);
    }
    if (node.minimum !== undefined && value < node.minimum) {
      errors.push(`${path}: ${String(value)} < ${String(node.minimum)}`);
    }
    if (node.maximum !== undefined && value > node.maximum) {
      errors.push(`${path}: ${String(value)} > ${String(node.maximum)}`);
    }
  } else if (Array.isArray(value)) {
    if (types.length > 0 && !types.includes("array")) {
      errors.push(`${path}: 配列は許されない`);
    }
    if (node.minItems !== undefined && value.length < node.minItems) {
      errors.push(`${path}: 要素が ${String(node.minItems)} 未満`);
    }
    if (node.maxItems !== undefined && value.length > node.maxItems) {
      errors.push(`${path}: 要素が ${String(node.maxItems)} を超える`);
    }
    const itemSchema = node.items;
    if (itemSchema !== undefined) {
      value.forEach((item: unknown, index) => {
        errors.push(...violations(root, itemSchema, item, `${path}[${String(index)}]`));
      });
    }
  } else if (typeof value === "object") {
    if (types.length > 0 && !types.includes("object")) {
      errors.push(`${path}: オブジェクトは許されない`);
    }
    const record = value as Record<string, unknown>;
    for (const key of node.required ?? []) {
      if (!(key in record)) {
        errors.push(`${path}: 必須の ${key} が無い`);
      }
    }
    const properties = node.properties ?? {};
    for (const [key, child] of Object.entries(record)) {
      const childSchema = properties[key];
      if (childSchema === undefined) {
        if (node.additionalProperties === false) {
          errors.push(`${path}: 許されないキー ${key}`);
        }
        continue;
      }
      errors.push(...violations(root, childSchema, child, `${path}.${key}`));
    }
  } else {
    errors.push(`${path}: 想定外の値 ${JSON.stringify(value)}`);
  }
  return errors;
}

function expectValid(schemaName: string, value: unknown): void {
  const schema = loadSchema(schemaName);
  // JSON にしたもの(ファイルに書くもの)で確かめる(undefined のキーは消える)。
  const json: unknown = JSON.parse(JSON.stringify(value));
  expect(violations(schema, schema, json, "$")).toEqual([]);
}

/** 既約分数の分子・分母(テストの期待値を作るため)。 */
function reduced(numerator: number, denominator: number): { numerator: number; denominator: number } {
  const gcd = (a: number, b: number): number => (b === 0 ? a : gcd(b, a % b));
  const divisor = gcd(numerator, denominator);
  return { numerator: numerator / divisor, denominator: denominator / divisor };
}

function withAbilities(abilities: readonly Ability[]): MasterData {
  return { ...master, abilities };
}

describe("toBalancePokemonTypes(pokemon-types.schema.json)", () => {
  test("スキーマに合う(schemaVersion 1・pokemon の各要素)", () => {
    expectValid("pokemon-types.schema.json", toBalancePokemonTypes(master));
  });

  test("種族ごとに pokemonId = 種族キー・nameJa・types・abilityIds(種族の特性)を同じ順で書く", () => {
    const model = toBalancePokemonTypes(master);
    expect(model.schemaVersion).toBe(1);
    expect(model.pokemon).toEqual(
      master.species.map((species) => ({
        pokemonId: species.key,
        nameJa: species.nameJa,
        types: species.types,
        abilityIds: species.abilities,
      })),
    );
  });

  test("learnset・baseStats など balance が知らないキーは書かない", () => {
    for (const entry of toBalancePokemonTypes(master).pokemon) {
      expect(Object.keys(entry).sort()).toEqual(["abilityIds", "nameJa", "pokemonId", "types"]);
    }
  });
});

describe("toBalanceMoves(moves.schema.json)", () => {
  test("スキーマに合う", () => {
    expectValid("moves.schema.json", toBalanceMoves(master));
  });

  test("技ごとに moveId・type・category だけを同じ順で書く(変化技も含める)", () => {
    const model = toBalanceMoves(master);
    expect(model.schemaVersion).toBe(1);
    expect(model.moves).toEqual(
      master.moves.map((move) => ({ moveId: move.id, type: move.type, category: move.category })),
    );
    expect(model.moves.some((move) => move.category === "status")).toBe(true);
  });
});

describe("toBalanceAbilities(abilities.schema.json)", () => {
  test("例データでスキーマに合い、特性をすべて同じ順で書く(写せる効果が無ければ effects: [])", () => {
    const model = toBalanceAbilities(master);
    expectValid("abilities.schema.json", model);
    expect(model.schemaVersion).toBe(1);
    expect(model.abilities.map((entry) => entry.abilityId)).toEqual(
      master.abilities.map((ability) => ability.id),
    );
    // 例データの特性(効果なし・タイプ一致補正)は、どちらも防御相性の効果を持たない。
    for (const entry of model.abilities) {
      expect(entry.effects).toEqual([]);
    }
  });

  test("defResistType の 0 は immune、正の値は m/4096 の既約分数の type_multiplier(相性表のタイプ順)", () => {
    const typeOrder = master.typeChart.types;
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-defense",
          nameJa: "テストぼうぎょ",
          // 相性表の順と逆に並べる(出力は相性表のタイプ順)。
          effect: { defResistType: { ice: 2048, ground: 0, fire: 2048, water: 5120 } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    const expected = [
      { kind: "immune", attackType: "ground" },
      { kind: "type_multiplier", attackType: "ice", ...reduced(2048, 4096) },
      { kind: "type_multiplier", attackType: "fire", ...reduced(2048, 4096) },
      { kind: "type_multiplier", attackType: "water", ...reduced(5120, 4096) },
    ].sort((a, b) => typeOrder.indexOf(a.attackType) - typeOrder.indexOf(b.attackType));
    expect(model.abilities).toEqual([{ abilityId: "example-ability-defense", effects: expected }]);
    expect(expected.find((effect) => effect.attackType === "fire")).toMatchObject({
      numerator: 1,
      denominator: 2,
    });
  });

  test("reduceSuperEffective は r/4096 の既約分数の super_effective_multiplier(attackType を持たない。タイプの効果の後)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-filter",
          nameJa: "テストフィルター",
          effect: { reduceSuperEffective: 3072, defResistType: { fire: 2048 } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-filter",
        effects: [
          { kind: "type_multiplier", attackType: "fire", numerator: 1, denominator: 2 },
          { kind: "super_effective_multiplier", numerator: 3, denominator: 4 },
        ],
      },
    ]);
  });

  test("既約分数が 1〜16 に収まらない効果・防御相性に関係しない効果は書かない", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-odd",
          nameJa: "テストはした",
          // 4097/4096・1/4096 は分母が 16 を超える。stabMod・offBoostType・ignoresBurn は防御相性ではない。
          effect: {
            defResistType: { fire: 4097, water: 1 },
            reduceSuperEffective: 4095,
            stabMod: 8192,
            offBoostType: "fire",
            offBoostTypeMod: 6144,
            ignoresBurn: true,
          },
        },
        { id: "example-ability-null", nameJa: "テストなし", effect: null },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      { abilityId: "example-ability-odd", effects: [] },
      { abilityId: "example-ability-null", effects: [] },
    ]);
  });

  test("境界値: 分母 16(書く)と分母 32(落とす)、分子 16(書く)と分子 17(落とす)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-boundary",
          nameJa: "テストきょうかい",
          effect: {
            // 256/4096 = 1/16(境界。書かれる)、128/4096 = 1/32(範囲外。落ちる)。
            defResistType: { fire: 256, water: 128 },
            // 65536/4096 = 16/1(境界。書かれる)。
            reduceSuperEffective: 65536,
          },
        },
        {
          id: "example-ability-boundary-over",
          nameJa: "テストきょうかいこえ",
          // 69632/4096 = 17/1(範囲外。落ちる)。
          effect: { reduceSuperEffective: 69632 },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-boundary",
        effects: [
          { kind: "type_multiplier", attackType: "fire", numerator: 1, denominator: 16 },
          { kind: "super_effective_multiplier", numerator: 16, denominator: 1 },
        ],
      },
      { abilityId: "example-ability-boundary-over", effects: [] },
    ]);
  });

  test("効果の数値は整数で書く(balance のローダーは 2.0 のような小数を拒否する)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        { id: "example-ability-half", nameJa: "テストはんげん", effect: { defResistType: { fire: 2048 } } },
      ]),
    );
    const json = JSON.stringify(model);
    expect(json).toContain('"numerator":1');
    expect(json).toContain('"denominator":2');
  });
});

describe("入力を書き換えない", () => {
  test("3つとも入力のマスタを書き換えない", () => {
    const before = JSON.stringify(master);
    toBalancePokemonTypes(master);
    toBalanceMoves(master);
    toBalanceAbilities(master);
    expect(JSON.stringify(master)).toBe(before);
  });
});

describe("web/scripts/export-example-master.mjs(balance の read model も書く)", () => {
  test("calc のスナップショットと同じディレクトリに balance の3ファイルを書く", () => {
    const webRoot = localPath("../../", import.meta.url);
    const outDir = mkdtempSync(join(tmpdir(), "pokecalc-export-balance-"));
    const outPath = join(outDir, "master.json");
    try {
      execFileSync(process.execPath, [join(webRoot, "scripts", "export-example-master.mjs"), outPath], {
        cwd: webRoot,
        stdio: "pipe",
        timeout: 60_000,
      });
      const read = (name: string): unknown => JSON.parse(readFileSync(join(outDir, name), "utf8"));
      expect(read("balance-pokemon-types.json")).toEqual(toBalancePokemonTypes(master));
      expect(read("balance-moves.json")).toEqual(toBalanceMoves(master));
      expect(read("balance-abilities.json")).toEqual(toBalanceAbilities(master));
    } finally {
      rmSync(outDir, { recursive: true, force: true });
    }
  }, 90_000);
});
