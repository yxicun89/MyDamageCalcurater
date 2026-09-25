// issue #71(ADR-0114): engine/presets/attacker.json を攻撃側プリセットの唯一の正にした
// (docs/adr/0114-attacker-preset-catalog.md)。Web の attackerPresets.ts はまだ独自に同じ値(順序・
// relevantSp・nature)を持っている(engine への移管は別タスク)ので、両者がズレていないことをここで
// 確かめる。attackerPresets.test.ts は「今のWebの挙動」を固定するテストで、こちらは「JSONと一致しているか」
// を確かめる契約テスト(JSON の値を書き写さず、毎回ファイルを読む。コーディング規約 §2)。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import type { MoveCategory } from "../engine/types";
import { localPath } from "../test/localPath";
import {
  ATTACKER_PRESET_KEYS,
  DEFAULT_ATTACKER_PRESET,
  resolveAttackerPreset,
  type AttackerPresetKey,
} from "./attackerPresets";
import { MAX_SP_PER_STAT, NEUTRAL_NATURE, ZERO_SP } from "./requests";

type RelevantStat = "atk" | "spa";

interface PresetEntry {
  readonly key: string;
  readonly relevantSp: number;
  readonly nature: "neutral" | "boost";
}

interface AttackerPresetCatalog {
  readonly schemaVersion: number;
  readonly default: string;
  readonly relevantStat: Record<MoveCategory, RelevantStat>;
  readonly boostMinus: Record<RelevantStat, RelevantStat>;
  readonly presets: readonly PresetEntry[];
}

function readCatalog(): AttackerPresetCatalog {
  const path = localPath("../../../engine/presets/attacker.json", import.meta.url);
  return JSON.parse(readFileSync(path, "utf8")) as AttackerPresetCatalog;
}

const catalog = readCatalog();
const categories: readonly MoveCategory[] = ["physical", "special", "status"];

describe("engine/presets/attacker.json との一致(ADR-0114: JSON が唯一の正)", () => {
  test("schemaVersion は Web が読める形(1)", () => {
    expect(catalog.schemaVersion).toBe(1);
  });

  test("カタログのキー・順序が Web の ATTACKER_PRESET_KEYS と一致する", () => {
    expect(ATTACKER_PRESET_KEYS).toEqual(catalog.presets.map((preset) => preset.key));
  });

  test("既定値(default)が Web の DEFAULT_ATTACKER_PRESET と一致する", () => {
    expect(DEFAULT_ATTACKER_PRESET).toBe(catalog.default);
  });

  test("boostMinus は relevantStat に現れるステータスだけを、互いに反対向きに持つ(atk↔spa)", () => {
    expect(catalog.boostMinus).toEqual({ atk: "spa", spa: "atk" });
  });

  describe.each(categories)("技の分類 %s", (category) => {
    const relevantStat = catalog.relevantStat[category];

    test(`relevantStat[${category}] は atk か spa`, () => {
      expect(["atk", "spa"]).toContain(relevantStat);
    });

    test.each(catalog.presets)("プリセット $key が resolveAttackerPreset と一致する", (preset) => {
      const expectedSp =
        preset.relevantSp === 0 ? { ...ZERO_SP } : { ...ZERO_SP, [relevantStat]: preset.relevantSp };
      const expectedNature =
        preset.nature === "boost"
          ? { plus: relevantStat, minus: catalog.boostMinus[relevantStat] }
          : { ...NEUTRAL_NATURE };

      const resolved = resolveAttackerPreset(preset.key as AttackerPresetKey, category);

      expect(resolved.sp).toEqual(expectedSp);
      expect(resolved.nature).toEqual(expectedNature);
    });
  });

  test("relevantSp はドメイン規約の上限(1ステータス32)を超えない", () => {
    for (const preset of catalog.presets) {
      expect(preset.relevantSp).toBeGreaterThanOrEqual(0);
      expect(preset.relevantSp).toBeLessThanOrEqual(MAX_SP_PER_STAT);
    }
  });
});
