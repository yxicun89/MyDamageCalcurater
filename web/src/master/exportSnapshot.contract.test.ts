// `make e2e` の web-e2e-online 修復(issue #72・ADR-0306 で常時実行になった3件のうちの1件)。
//
// toCalcSnapshot() の出力は calc-svc がそのまま読むマスタ一式(api/openapi.yaml の MasterExport。ADR-0204)で、
// 受け取り側(services/calc/internal/master/export.go の FromExport と、その先の services/internal/master)が
// 厳格に検証する。ズレると calc-svc が**起動しない**ため、オンライン E2E が丸ごと落ちる。
//
// このファイルは、その契約を毎回ソースから読んで突き合わせる契約テスト
// (attackerPresets.contract.test.ts / defenderPresets.contract.test.ts と同じ発想。コーディング規約 §2:
// 期待値を書き写さない)。読むのは次の4つ:
//   - api/openapi.yaml            … MasterExport 以下の required・PokeType の enum・code の enum
//   - services/calc/internal/master/export.go … masterExportRequiredFields(トップレベルの必須)
//   - services/internal/master/typechart.go   … ID の形式(codeIDPattern・speciesKeyPattern)
//   - services/internal/master/effects.go     … 効果の既知のキー名(PascalCase)
//
// 限界: Go の検証そのもの(FromExport)を TypeScript から呼ぶことはできない。最終的な確認は
// `make e2e`(または `cd web && npm run e2e:online`)で calc-svc が実際に起動することで行う。

import { readFileSync } from "node:fs";
import { beforeAll, describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";
import { exampleMasterSource } from "./exampleSource";
import { toCalcSnapshot } from "./exportSnapshot";
import type { MasterData } from "./types";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function readRepoFile(relative: string): string {
  return readFileSync(localPath(`../../../${relative}`, import.meta.url), "utf8");
}

const openApiSource = readRepoFile("api/openapi.yaml");
const calcExportSource = readRepoFile("services/calc/internal/master/export.go");
const typeChartSource = readRepoFile("services/internal/master/typechart.go");
const effectsSource = readRepoFile("services/internal/master/effects.go");

/**
 * api/openapi.yaml の components/schemas から1つのスキーマの本文を取り出す。
 * 末尾のスキーマ(次の見出しが無い)も読めるよう、番兵の見出しを足した文字列から探す。
 */
function schemaBlock(name: string): string {
  const matched = new RegExp(`^ {4}${name}:$([\\s\\S]*?)^ {4}\\w`, "m").exec(`${openApiSource}\n    ZZEndOfSchemas:`);
  expect(matched, `api/openapi.yaml の ${name} スキーマを読めない`).not.toBeNull();
  return matched?.[1] ?? "";
}

/** `[a, b, c]` 形式(改行を含んでよい)の一覧を読む。 */
function bracketList(source: string, key: string, label: string): string[] {
  const matched = new RegExp(`${key}:\\s*\\[([^\\]]+)\\]`).exec(source);
  expect(matched, `${label} を読めない`).not.toBeNull();
  return (matched?.[1] ?? "")
    .split(",")
    .map((value) => value.trim())
    .filter((value) => value !== "");
}

/** スキーマの required(このマスタ一式では必須=そのまま「持つべきキー」)。 */
function requiredFields(name: string): string[] {
  return bracketList(schemaBlock(name), "required", `${name} の required`);
}

/** Go のソースから `"..."` の並びを読む(var ブロック / map リテラル)。 */
function goQuotedNames(source: string, pattern: RegExp, label: string): string[] {
  const block = pattern.exec(source);
  expect(block, `${label} を読めない`).not.toBeNull();
  return [...(block?.[1] ?? "").matchAll(/"([^"]+)"/g)].map((m) => m[1] ?? "");
}

/** Go の `regexp.MustCompile(`...`)` のパターンを読む。 */
function goRegexp(source: string, name: string): RegExp {
  const matched = new RegExp(`${name}\\s*=\\s*regexp\\.MustCompile\\(\`([^\`]+)\`\\)`).exec(source);
  expect(matched, `${name} を読めない`).not.toBeNull();
  return new RegExp(matched?.[1] ?? "$^");
}

function sortedKeys(value: object): string[] {
  return Object.keys(value).sort();
}

describe("api/openapi.yaml の MasterExport との一致(ADR-0204。契約は calc-svc 側が正)", () => {
  test("最上位のキーが MasterExport.required と過不足なく一致する", () => {
    const snapshot = toCalcSnapshot(master);
    expect(sortedKeys(snapshot)).toEqual([...requiredFields("MasterExport")].sort());
  });

  test("最上位のキーが calc-svc の masterExportRequiredFields と過不足なく一致する", () => {
    const required = goQuotedNames(
      calcExportSource,
      /masterExportRequiredFields\s*=\s*\[\]string\{([\s\S]*?)\}/,
      "masterExportRequiredFields",
    );
    expect(required.length).toBeGreaterThan(0);
    expect(sortedKeys(toCalcSnapshot(master))).toEqual([...required].sort());
  });

  test.each([
    ["MasterType", "types"],
    ["MasterTypeChartEntry", "typeChart"],
    ["MasterSpecies", "species"],
    ["MasterMove", "moves"],
    ["MasterItem", "items"],
    ["MasterAbility", "abilities"],
    ["MasterNature", "natures"],
  ] as const)("%s の required が、書き出した %s の各要素のキーと一致する", (schema, field) => {
    const expected = [...requiredFields(schema)].sort();
    const rows: readonly object[] = toCalcSnapshot(master)[field];
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(sortedKeys(row)).toEqual(expected);
    }
  });

  test("種族の abilities の各行が MasterSpeciesAbility.required と一致する", () => {
    const expected = [...requiredFields("MasterSpeciesAbility")].sort();
    for (const species of toCalcSnapshot(master).species) {
      expect(species.abilities.length).toBeGreaterThan(0);
      for (const row of species.abilities) {
        expect(sortedKeys(row)).toEqual(expected);
      }
    }
  });

  test("タイプを指すフィールドはすべて PokeType の enum の値(未知のタイプを混ぜない。ADR-0118)", () => {
    const pokeTypes = new Set(bracketList(schemaBlock("PokeType"), "enum", "PokeType の enum"));
    expect(pokeTypes.size).toBe(18);
    const snapshot = toCalcSnapshot(master);
    for (const type of snapshot.types) {
      expect(pokeTypes, `types の ${type.id}`).toContain(type.id);
    }
    for (const entry of snapshot.typeChart) {
      expect(pokeTypes).toContain(entry.attackType);
      expect(pokeTypes).toContain(entry.defenseType);
    }
    for (const species of snapshot.species) {
      expect(pokeTypes).toContain(species.type1);
      if (species.type2 !== null) {
        expect(pokeTypes).toContain(species.type2);
      }
    }
    for (const move of snapshot.moves) {
      expect(pokeTypes).toContain(move.type);
    }
  });

  test("相性表の code は MasterTypeChartEntry の enum の値だけ", () => {
    const codes = new Set(
      bracketList(schemaBlock("MasterTypeChartEntry"), "code: \\{ type: integer, enum", "code の enum").map(
        (value) => Number(value),
      ),
    );
    expect(codes).toEqual(new Set([0, 1, 2, 4]));
    for (const entry of toCalcSnapshot(master).typeChart) {
      expect(codes).toContain(entry.code);
    }
  });
});

describe("services/internal/master の検証との一致(FromExport が通る形であること)", () => {
  test("技・持ち物・特性の ID と showdownId が codeIDPattern を満たす(Web が API に送る ID そのもの)", () => {
    const codeID = goRegexp(typeChartSource, "codeIDPattern");
    // 念のため: 読めた正規表現が期待どおり「ハイフンを許さない」ものであること(読み間違いの検知)。
    expect(codeID.test("examplemovetackle")).toBe(true);
    expect(codeID.test("example-move-tackle")).toBe(false);

    const snapshot = toCalcSnapshot(master);
    for (const move of snapshot.moves) {
      expect(codeID.test(move.id), `技 ${move.id}`).toBe(true);
    }
    for (const item of snapshot.items) {
      expect(codeID.test(item.id), `持ち物 ${item.id}`).toBe(true);
    }
    for (const ability of snapshot.abilities) {
      expect(codeID.test(ability.id), `特性 ${ability.id}`).toBe(true);
    }
    for (const species of snapshot.species) {
      expect(codeID.test(species.showdownId), `showdownId ${species.showdownId}`).toBe(true);
    }
    // 性格の ID は calc-svc 側で形式を見ない(buildNatures は空・重複だけを見る)ので、ここでは要求しない。
  });

  test("種族の key が speciesKeyPattern を満たし、showdownId は重複しない", () => {
    const speciesKey = goRegexp(typeChartSource, "speciesKeyPattern");
    const snapshot = toCalcSnapshot(master);
    const showdownIds = new Set<string>();
    for (const species of snapshot.species) {
      expect(speciesKey.test(species.key), `key ${species.key}`).toBe(true);
      showdownIds.add(species.showdownId);
    }
    expect(showdownIds.size).toBe(snapshot.species.length);
  });

  test("種族が参照する abilityId は、書き出した abilities に存在する(FromExport が参照を検査する)", () => {
    const snapshot = toCalcSnapshot(master);
    const known = new Set(snapshot.abilities.map((ability) => ability.id));
    for (const species of snapshot.species) {
      for (const row of species.abilities) {
        expect(known, `${species.key} の ${row.abilityId}`).toContain(row.abilityId);
      }
    }
  });

  test.each([
    ["items", "itemEffectFields"],
    ["abilities", "abilityEffectFields"],
  ] as const)("%s の効果のキーは %s(PascalCase)にあるものだけ", (field, goVar) => {
    const known = new Set(
      goQuotedNames(effectsSource, new RegExp(`${goVar}\\s*=\\s*map\\[string\\]bool\\{([\\s\\S]*?)\\}`), goVar),
    );
    expect(known.size).toBeGreaterThan(0);
    const rows = toCalcSnapshot(master)[field];
    // 効果を持つ要素が1つも無いと、この検査は何も確かめない(例データには効果付きがある)。
    expect(rows.some((row) => row.effect !== null)).toBe(true);
    for (const row of rows) {
      if (row.effect === null) {
        continue;
      }
      const keys = Object.keys(row.effect);
      // 共通マスタは空のオブジェクトを「補正が1つも無い」として拒む。
      expect(keys.length, `${row.id} の効果が空`).toBeGreaterThan(0);
      for (const key of keys) {
        expect(known, `${row.id} の効果のキー ${key}`).toContain(key);
      }
    }
  });
});

describe("オンライン E2E の設定(playwright.online.config.ts)", () => {
  const configSource = readRepoFile("web/playwright.online.config.ts");

  test("calc-svc が廃止した CALC_TYPECHART_PATH を渡さない(渡すと起動しない。ADR-0204)", () => {
    const mainSource = readRepoFile("services/calc/cmd/calc/main.go");
    // calc-svc 側が「設定されていたら起動しない」ままであることを先に確かめる(前提の固定)。
    expect(mainSource).toContain('envTypeChartPath = "CALC_TYPECHART_PATH"');
    expect(mainSource).toMatch(/lookup\(envTypeChartPath\)/);
    expect(configSource).not.toContain("CALC_TYPECHART_PATH");
  });

  test("相性表のファイル(testdata/golden/typechart.json)を calc-svc に渡さない(マスタ一式に含める)", () => {
    expect(configSource).not.toContain("typechart.json");
  });

  test("calc-svc にはマスタ一式(CALC_MASTER_PATH)だけを渡す", () => {
    expect(configSource).toContain("CALC_MASTER_PATH");
  });
});
