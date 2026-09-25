// issue #275: 防御側プリセット(ADR-0009 §1)の正は engine/presets/defender.json
// (ADR-0009 2026-09-25 追記。engine は go:embed で読み、期待値は engine/bulk_test.go の
// TestDefenderPresetCatalogDefinitions)で、キーと順序は API 契約(api/openapi.yaml の `DefenderPreset` enum)にも出ている。
// SP・性格の値そのものは API にも WASM 境界にも出ていない(BulkCalcRequest はキーだけを送り、engine が
// 内部で解決する。ADR-0009 §5)ので、Web は attackerPresets.ts と同じく同じ規則を自分でも持つ。
//
// このテストは、その二重定義がズレていないことを確かめる契約テスト(issue #71 / ADR-0114 で攻撃側に
// 入れた attackerPresets.contract.test.ts と同じ発想)。値を書き写さず、毎回 engine/presets/defender.json と
// api/openapi.yaml を読む(コーディング規約 §2)。
//
// 以前は engine/bulk.go の Go リテラルを正規表現で読んでいたが、JSON ができたので JSON を読む形に置き換えた
// (データレーンが engine の変更と同じ PR で差し替え。比べる項目と期待値は変えていない)。

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
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "./requests";

/** engine/presets/defender.json の1エントリ。 */
interface JsonDefenderPreset {
  readonly key: string;
  readonly label: string;
  readonly sp: Stats;
  readonly nature: Nature;
  readonly applies: MoveCategoryOrNone;
}

interface DefenderPresetCatalog {
  readonly schemaVersion: number;
  readonly presets: readonly JsonDefenderPreset[];
}

function readCatalog(): DefenderPresetCatalog {
  const path = localPath("../../../engine/presets/defender.json", import.meta.url);
  return JSON.parse(readFileSync(path, "utf8")) as DefenderPresetCatalog;
}

function readOpenApiSource(): string {
  return readFileSync(localPath("../../../api/openapi.yaml", import.meta.url), "utf8");
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

const catalogFile = readCatalog();
const catalog = catalogFile.presets;
const categories: readonly MoveCategory[] = ["physical", "special", "status"];

describe("engine/presets/defender.json との一致(ADR-0009 §1 が正)", () => {
  test("schemaVersion は Web が読める形(1)", () => {
    expect(catalogFile.schemaVersion).toBe(1);
  });

  test("カタログは8件ある", () => {
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

  test.each(catalog)("$key の表示名が JSON の label と一致する", (preset) => {
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
