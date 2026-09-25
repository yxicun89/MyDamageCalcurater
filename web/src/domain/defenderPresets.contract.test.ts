// issue #275: 防御側プリセット(ADR-0009 §1)の正は engine の `DefenderPresetCatalog()`
// (engine/bulk.go。期待値は engine/bulk_test.go の TestDefenderPresetCatalogDefinitions)で、
// キーと順序は API 契約(api/openapi.yaml の `DefenderPreset` enum)にも出ている。
// SP・性格の値そのものは API にも WASM 境界にも出ていない(BulkCalcRequest はキーだけを送り、engine が
// 内部で解決する。ADR-0009 §5)ので、Web は attackerPresets.ts と同じく同じ規則を自分でも持つ。
//
// このテストは、その二重定義がズレていないことを確かめる契約テスト(issue #71 / ADR-0114 で攻撃側に
// 入れた attackerPresets.contract.test.ts と同じ発想)。値を書き写さず、毎回 engine/bulk.go と
// api/openapi.yaml を読む(コーディング規約 §2)。
//
// 攻撃側は engine/presets/attacker.json という言語非依存の正があるが、防御側はまだ Go のコードしか無いため、
// ここでは Go のソースの該当リテラルだけを読む。**将来 `engine/presets/defender.json` ができたら、この
// パーサは捨ててその JSON を読む**(DECISIONS.md 2026-09-25「防御側プリセットの JSON 契約化」の申し送り)。
// パースに失敗したら黙って通さず、その場で落とす(下の parse 系の expect)。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import type { MoveCategory, MoveCategoryOrNone, Nature, StatKey, Stats } from "../engine/types";
import { localPath } from "../test/localPath";
import {
  DEFAULT_DEFENDER_PRESET,
  DEFENDER_PRESET_KEYS,
  defenderPresetKeysFor,
  defenderPresetLabel,
  resolveDefenderPreset,
  type DefenderPresetKey,
} from "./defenderPresets";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL, NEUTRAL_NATURE, ZERO_SP } from "./requests";

/** engine/bulk.go の1エントリ(Go の識別子を解決した後の値)。 */
interface GoDefenderPreset {
  readonly key: string;
  readonly label: string;
  readonly sp: Stats;
  readonly nature: Nature;
  readonly applies: MoveCategoryOrNone;
}

/** Go の `Stats` のフィールド名 → Web の StatKey。 */
const goStatField: Readonly<Record<string, StatKey>> = {
  HP: "hp",
  Atk: "atk",
  Def: "def",
  SpA: "spa",
  SpD: "spd",
  Spe: "spe",
};

/** Go の `StatKey` 定数(engine/types.go)→ Web の StatKey。 */
const goStatConst: Readonly<Record<string, StatKey>> = {
  StatHP: "hp",
  StatAtk: "atk",
  StatDef: "def",
  StatSpA: "spa",
  StatSpD: "spd",
  StatSpe: "spe",
};

/** Go の `MoveCategory` 定数(engine/types.go)→ Web の MoveCategory。 */
const goCategoryConst: Readonly<Record<string, MoveCategory>> = {
  CategoryPhysical: "physical",
  CategorySpecial: "special",
  CategoryStatus: "status",
};

/** SP のリテラルに現れる Go の定数 → 数値(engine の MaxSPPerStat は Web の MAX_SP_PER_STAT と同じ規約値)。 */
const goNumberConst: Readonly<Record<string, number>> = {
  MaxSPPerStat: MAX_SP_PER_STAT,
  MaxSPTotal: MAX_SP_TOTAL,
};

function readEngineSource(): string {
  return readFileSync(localPath("../../../engine/bulk.go", import.meta.url), "utf8");
}

function readOpenApiSource(): string {
  return readFileSync(localPath("../../../api/openapi.yaml", import.meta.url), "utf8");
}

/** `PresetHBBoost PresetKey = "hb_boost"` の対応表。 */
function parsePresetKeyConsts(source: string): Map<string, string> {
  const consts = new Map<string, string>();
  for (const match of source.matchAll(/^\s*(Preset\w+)\s+PresetKey\s*=\s*"([^"]+)"/gm)) {
    const [, identifier, value] = match;
    if (identifier !== undefined && value !== undefined) {
      consts.set(identifier, value);
    }
  }
  return consts;
}

/** `Stats{HP: MaxSPPerStat, Def: MaxSPPerStat}` の中身(空もある)を Stats にする。 */
function parseStats(literal: string): Stats {
  const sp: Record<StatKey, number> = { ...ZERO_SP };
  const body = literal.trim();
  if (body === "") {
    return sp;
  }
  for (const field of body.split(",")) {
    const match = /^\s*(\w+):\s*(\w+)\s*$/.exec(field);
    expect(match, `SP のフィールドを解釈できない: ${field}`).not.toBeNull();
    const name = match?.[1] ?? "";
    const rawValue = match?.[2] ?? "";
    const stat = goStatField[name];
    expect(stat, `未知の Stats フィールド: ${name}`).toBeDefined();
    const value = /^\d+$/.test(rawValue) ? Number(rawValue) : goNumberConst[rawValue];
    expect(value, `SP の値を解釈できない: ${rawValue}`).toBeDefined();
    if (stat !== undefined && value !== undefined) {
      sp[stat] = value;
    }
  }
  return sp;
}

/** engine/bulk.go の `DefenderPresetCatalog()` のリテラルを読む。 */
function parseCatalog(): GoDefenderPreset[] {
  const source = readEngineSource();
  const consts = parsePresetKeyConsts(source);
  expect(consts.size, "engine/bulk.go の PresetKey 定数を読めない").toBeGreaterThan(0);

  const block =
    /func DefenderPresetCatalog\(\) \[\]DefenderPreset \{[^{]*\[\]DefenderPreset\{\n([\s\S]*?)\n\t\}\n\}/.exec(
      source,
    );
  expect(block, "engine/bulk.go の DefenderPresetCatalog() のリテラルを読めない").not.toBeNull();

  const entryPattern =
    /^\s*\{Key:\s*(\w+),\s*Label:\s*"([^"]*)",\s*SP:\s*Stats\{([^}]*)\},\s*Nature:\s*(?:NatureNeutral|Nature\{Plus:\s*(\w+),\s*Minus:\s*(\w+)\})(?:,\s*Applies:\s*(\w+))?\},?\s*$/;

  return (block?.[1] ?? "").split("\n").map((line) => {
    const match = entryPattern.exec(line);
    expect(match, `カタログの行を解釈できない: ${line}`).not.toBeNull();
    const keyConst = match?.[1] ?? "";
    const key = consts.get(keyConst);
    expect(key, `未知の PresetKey 定数: ${keyConst}`).toBeDefined();
    const plus = match?.[4];
    const minus = match?.[5];
    const appliesConst = match?.[6];
    const nature: Nature =
      plus === undefined || minus === undefined
        ? { ...NEUTRAL_NATURE }
        : { plus: goStatConst[plus] ?? "", minus: goStatConst[minus] ?? "" };
    const applies: MoveCategoryOrNone =
      appliesConst === undefined ? "" : (goCategoryConst[appliesConst] ?? "");
    if (appliesConst !== undefined) {
      expect(goCategoryConst[appliesConst], `未知の MoveCategory 定数: ${appliesConst}`).toBeDefined();
    }
    if (plus !== undefined && minus !== undefined) {
      expect(goStatConst[plus], `未知の StatKey 定数: ${plus}`).toBeDefined();
      expect(goStatConst[minus], `未知の StatKey 定数: ${minus}`).toBeDefined();
    }
    return {
      key: key ?? "",
      label: match?.[2] ?? "",
      sp: parseStats(match?.[3] ?? ""),
      nature,
      applies,
    };
  });
}

/** api/openapi.yaml の `DefenderPreset` enum(キーと順序の契約)。 */
function parseOpenApiEnum(): string[] {
  const source = readOpenApiSource();
  const schema = /^ {4}DefenderPreset:$([\s\S]*?)^ {4}\w/m.exec(source);
  expect(schema, "api/openapi.yaml の DefenderPreset スキーマを読めない").not.toBeNull();
  const enumMatch = /enum:\s*\[([^\]]+)\]/.exec(schema?.[1] ?? "");
  expect(enumMatch, "api/openapi.yaml の DefenderPreset の enum を読めない").not.toBeNull();
  return (enumMatch?.[1] ?? "").split(",").map((value) => value.trim());
}

const catalog = parseCatalog();
const categories: readonly MoveCategory[] = ["physical", "special", "status"];

describe("engine/bulk.go の DefenderPresetCatalog() との一致(ADR-0009 §1 が正)", () => {
  test("カタログは8件読めている(パーサが黙って空を返していない)", () => {
    expect(catalog).toHaveLength(8);
  });

  test("キーと順序が Web の DEFENDER_PRESET_KEYS と一致する", () => {
    expect(DEFENDER_PRESET_KEYS).toEqual(catalog.map((preset) => preset.key));
  });

  test.each(catalog)("$key の SP・性格が resolveDefenderPreset と一致する", (preset) => {
    const resolved = resolveDefenderPreset(preset.key as DefenderPresetKey);
    expect(resolved.sp).toEqual(preset.sp);
    expect(resolved.nature).toEqual(preset.nature);
  });

  test.each(catalog)("$key の表示名が engine の Label と一致する", (preset) => {
    expect(defenderPresetLabel(preset.key as DefenderPresetKey)).toBe(preset.label);
  });

  test.each(categories)(
    "%s の選択肢が engine の DefaultDefenderPresets(Applies による絞り込み)と一致する",
    (category) => {
      const expected = catalog
        .filter((preset) => preset.applies === "" || preset.applies === category)
        .map((preset) => preset.key);
      expect(defenderPresetKeysFor(category)).toEqual(expected);
    },
  );

  test("engine 側の SP もドメイン規約の上限(1ステータス 32・合計 66)を守っている", () => {
    const statKeys: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];
    for (const preset of catalog) {
      let total = 0;
      for (const stat of statKeys) {
        const value = preset.sp[stat];
        expect(value).toBeGreaterThanOrEqual(0);
        expect(value).toBeLessThanOrEqual(MAX_SP_PER_STAT);
        total += value;
      }
      expect(total).toBeLessThanOrEqual(MAX_SP_TOTAL);
    }
  });
});

describe("api/openapi.yaml の DefenderPreset enum との一致(API 契約が正)", () => {
  test("キーと順序が Web の DEFENDER_PRESET_KEYS と一致する", () => {
    expect(DEFENDER_PRESET_KEYS).toEqual(parseOpenApiEnum());
  });

  test("既定(none)は enum に含まれる", () => {
    expect(parseOpenApiEnum()).toContain(DEFAULT_DEFENDER_PRESET);
  });
});
