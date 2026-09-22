// P4-12a(ADR-0303 §4): Web の例データを balance-svc の read model(services/balance/schema/ の3つ)にも書き出す。
// pokedex の read model に揃うまでの間、ローカルと E2E ではこの出力で balance-svc を起動し、Web の例データの ID
// (pokemonId = 種族キー・moveId・abilityId)がそのまま balance の API に通るようにする。
// 形は services/balance/schema/*.schema.json(ADR-0402)を読んで確かめる(必須キー・許されたキー・enum・pattern)。
// 特性: engine の効果定義から balance の正規化された効果に写せるものだけを書く(ADR-0303 §4)。
//   defImmuneTypes の要素・defResistType[t] = 0(P4-12a からの書き方)→ {kind: "immune", attackType: t}
//   defAbsorbTypes のキー         → {kind: "absorb", attackType: t}(副次効果は書かない。ADR-0106 §決定7)
//   defResistType[t] = m (> 0)  → {kind: "type_multiplier", attackType: t, numerator/denominator = m/4096 の既約分数}
//   reduceSuperEffective = r    → {kind: "super_effective_multiplier", numerator/denominator = r/4096 の既約分数}
//   既約分数の分子・分母が 1〜16 に収まらない効果は書かない。それ以外の効果(stabMod など)は書かない。
//   画面はどの特性も選べる(analyze は read model に無い abilityId を 422 にする)ので、特性はすべて書き、
//   写せる効果が無い特性は effects: [] にする。
// P2-3b(ADR-0106)追記: 出力順は immune(タイプ順)→ absorb(タイプ順)→ type_multiplier(タイプ順)→
//   super_effective_multiplier(ADR-0106 §決定7。データレーンの pokedex export と同じ並びに揃える)。
//   P4-12a 時点ではタイプ相性表の順に immune と type_multiplier を混ぜて出していたが、
//   ADR-0106 で absorb が増えたのを機に「種類ごとにまとめる」順へ改めた(以下のテストの期待値も合わせて変更)。

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

  test("defResistType の 0 は immune、正の値は m/4096 の既約分数の type_multiplier(immune が先、type_multiplier はタイプ順)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-defense",
          nameJa: "テストぼうぎょ",
          // 相性表の順と逆に並べる(type_multiplier の出力は相性表のタイプ順)。
          effect: { defResistType: { ice: 2048, ground: 0, fire: 2048, water: 5120 } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    // ADR-0106 §決定7: immune が先、そのあと type_multiplier がタイプ順(ここでは fire → ice → water)。
    const expected = [
      { kind: "immune", attackType: "ground" },
      { kind: "type_multiplier", attackType: "fire", ...reduced(2048, 4096) },
      { kind: "type_multiplier", attackType: "ice", ...reduced(2048, 4096) },
      { kind: "type_multiplier", attackType: "water", ...reduced(5120, 4096) },
    ];
    expect(model.abilities).toEqual([{ abilityId: "example-ability-defense", effects: expected }]);
    expect(expected.find((effect) => effect.attackType === "fire")).toMatchObject({
      numerator: 1,
      denominator: 2,
    });
  });

  test("defImmuneTypes(ADR-0106)は immune。defResistType の 0 と混ざってもタイプ順で1件ずつ", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-levitate",
          nameJa: "テストふゆう",
          // defImmuneTypes は相性表の順と逆に、defResistType の 0 とは別のタイプで指定する。
          effect: { defImmuneTypes: ["water", "fire"], defResistType: { ground: 0 } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    // タイプ順(fire < ground < water)にそろう。
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-levitate",
        effects: [
          { kind: "immune", attackType: "fire" },
          { kind: "immune", attackType: "ground" },
          { kind: "immune", attackType: "water" },
        ],
      },
    ]);
  });

  test("同じタイプが defImmuneTypes と defResistType の 0 の両方にあっても、immune は1件だけ(重複しない)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-levitate-overlap",
          nameJa: "テストふゆうかさなり",
          // ground は両方の書き方で無効を指定している。fire は defResistType の 0 だけ。
          effect: { defImmuneTypes: ["water", "ground"], defResistType: { ground: 0, fire: 0 } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    // ground が2件にならず、fire・ground・water の3件だけ(タイプ順)。
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-levitate-overlap",
        effects: [
          { kind: "immune", attackType: "fire" },
          { kind: "immune", attackType: "ground" },
          { kind: "immune", attackType: "water" },
        ],
      },
    ]);
  });

  test("defAbsorbTypes(ADR-0106)は absorb。副次効果(回復・能力上昇)は出さず attackType だけ", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-water-absorb",
          nameJa: "テストちょすい",
          effect: { defAbsorbTypes: { water: { healNumerator: 1, healDenominator: 4 } } },
        },
        {
          id: "example-ability-flash-fire",
          nameJa: "テストもらいび",
          // 副次効果なしの吸収({})も正しい値(ADR-0106 §決定2)。
          effect: { defAbsorbTypes: { fire: {} } },
        },
        {
          id: "example-ability-sap-sipper",
          nameJa: "テストそうしょく",
          effect: { defAbsorbTypes: { grass: { boostStat: "atk", boostStages: 1 } } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      { abilityId: "example-ability-water-absorb", effects: [{ kind: "absorb", attackType: "water" }] },
      { abilityId: "example-ability-flash-fire", effects: [{ kind: "absorb", attackType: "fire" }] },
      { abilityId: "example-ability-sap-sipper", effects: [{ kind: "absorb", attackType: "grass" }] },
    ]);
  });

  test("同じタイプが defAbsorbTypes と無効(の両方の書き方)にあると、無効が勝ち absorb は出さない(ADR-0106 §決定1)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-immune-wins-1",
          nameJa: "テストむこうゆうせん1",
          // fire は defImmuneTypes と defAbsorbTypes の両方にある(不正な入力だが、結果は immune に決める)。
          effect: { defImmuneTypes: ["fire"], defAbsorbTypes: { fire: {}, water: {} } },
        },
        {
          id: "example-ability-immune-wins-2",
          nameJa: "テストむこうゆうせん2",
          // fire は defResistType の 0(無効の別の書き方)と defAbsorbTypes の両方にある。
          effect: { defResistType: { fire: 0 }, defAbsorbTypes: { fire: {}, water: {} } },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-immune-wins-1",
        effects: [
          { kind: "immune", attackType: "fire" },
          { kind: "absorb", attackType: "water" },
        ],
      },
      {
        abilityId: "example-ability-immune-wins-2",
        effects: [
          { kind: "immune", attackType: "fire" },
          { kind: "absorb", attackType: "water" },
        ],
      },
    ]);
  });

  test("immune → absorb → type_multiplier → super_effective_multiplier の順(ADR-0106 §決定7。全種類そろえたとき)", () => {
    const model = toBalanceAbilities(
      withAbilities([
        {
          id: "example-ability-mixed",
          nameJa: "テストごちゃまぜ",
          effect: {
            // 種類ごとの中では意図的にタイプ順と逆に書く(出力は種類の中でもタイプ順にそろうことを見る)。
            defImmuneTypes: ["water", "ground"],
            defAbsorbTypes: { steel: {}, electric: {} },
            defResistType: { rock: 2048, fairy: 2048 },
            reduceSuperEffective: 3072,
          },
        },
      ]),
    );
    expectValid("abilities.schema.json", model);
    expect(model.abilities).toEqual([
      {
        abilityId: "example-ability-mixed",
        effects: [
          { kind: "immune", attackType: "ground" },
          { kind: "immune", attackType: "water" },
          { kind: "absorb", attackType: "electric" },
          { kind: "absorb", attackType: "steel" },
          { kind: "type_multiplier", attackType: "fairy", numerator: 1, denominator: 2 },
          { kind: "type_multiplier", attackType: "rock", numerator: 1, denominator: 2 },
          { kind: "super_effective_multiplier", numerator: 3, denominator: 4 },
        ],
      },
    ]);
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
