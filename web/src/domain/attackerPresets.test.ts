// P4-3: 攻撃側(自分側)のプリセット(ADR-0300 §5、requirements.md「自分側のプリセット」)。
// 3件(none / x_full / x)の順序、技の分類ごとの SP・性格、表示名を確かめる。
// X は技の分類で決まる関連ステータス(物理 = atk、特殊 = spa)。下降補正は ADR-0010 §R1 の代表性格と同じ
// (X が atk なら spa、spa なら atk)。
// 変化技(status)の扱い(このテストで決める): 変化技はダメージを計算しない(画面は calcBulk を呼ばない)ので
// どの値でも結果は変わらないが、関数は全域にしておく。物理と同じ(X = atk、表示も A)として扱う。

import { describe, expect, test } from "vitest";
import type { MoveCategory, Stats, StatKey } from "../engine/types";
import {
  ATTACKER_PRESET_KEYS,
  DEFAULT_ATTACKER_PRESET,
  attackerPresetLabel,
  resolveAttackerPreset,
  type AttackerPresetKey,
} from "./attackerPresets";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL, NEUTRAL_NATURE, ZERO_SP } from "./requests";

const statKeys: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];

function spTotal(sp: Stats): number {
  return statKeys.reduce((sum, key) => sum + sp[key], 0);
}

describe("カタログ", () => {
  test("3件で、順序は none → x_full → x(ADR-0300 §5 の表の順)", () => {
    expect(ATTACKER_PRESET_KEYS).toEqual(["none", "x_full", "x"]);
  });

  test("既定は none(無振り。P4-2 の固定値と同じ)", () => {
    expect(DEFAULT_ATTACKER_PRESET).toBe("none");
  });

  test("SP の上限はドメイン規約どおり(1ステータス 32・合計 66)", () => {
    expect(MAX_SP_PER_STAT).toBe(32);
    expect(MAX_SP_TOTAL).toBe(66);
  });
});

describe("resolveAttackerPreset(SP・性格)", () => {
  test.each([
    ["none", "physical", ZERO_SP, NEUTRAL_NATURE],
    ["x_full", "physical", { ...ZERO_SP, atk: 32 }, { plus: "atk", minus: "spa" }],
    ["x", "physical", { ...ZERO_SP, atk: 32 }, NEUTRAL_NATURE],
    ["none", "special", ZERO_SP, NEUTRAL_NATURE],
    ["x_full", "special", { ...ZERO_SP, spa: 32 }, { plus: "spa", minus: "atk" }],
    ["x", "special", { ...ZERO_SP, spa: 32 }, NEUTRAL_NATURE],
    // 変化技は物理と同じ扱い(冒頭のコメント)
    ["none", "status", ZERO_SP, NEUTRAL_NATURE],
    ["x_full", "status", { ...ZERO_SP, atk: 32 }, { plus: "atk", minus: "spa" }],
    ["x", "status", { ...ZERO_SP, atk: 32 }, NEUTRAL_NATURE],
  ] as const)("%s × %s", (key, category, sp, nature) => {
    expect(resolveAttackerPreset(key, category)).toEqual({ sp, nature });
  });

  test.each(
    ATTACKER_PRESET_KEYS.flatMap((key) =>
      (["physical", "special", "status"] as const).map((category) => [key, category] as const),
    ),
  )("%s × %s は SP の上限(1ステータス・合計)を守る", (key, category) => {
    const { sp } = resolveAttackerPreset(key, category);
    for (const stat of statKeys) {
      expect(sp[stat]).toBeGreaterThanOrEqual(0);
      expect(sp[stat]).toBeLessThanOrEqual(MAX_SP_PER_STAT);
    }
    expect(spTotal(sp)).toBeLessThanOrEqual(MAX_SP_TOTAL);
  });

  test("返す SP・性格は呼ぶたびに同じ値で、共有の定数(ZERO_SP)を書き換えない", () => {
    const before = { ...ZERO_SP };
    resolveAttackerPreset("x_full", "physical");
    resolveAttackerPreset("x", "special");
    expect(ZERO_SP).toEqual(before);
    expect(resolveAttackerPreset("x_full", "special")).toEqual(resolveAttackerPreset("x_full", "special"));
  });
});

describe("attackerPresetLabel(表示名。文言は i18n/ja.ts)", () => {
  const expected: Record<MoveCategory, Record<AttackerPresetKey, string>> = {
    physical: { none: "無振り", x_full: "A特化", x: "A振り(無補正)" },
    special: { none: "無振り", x_full: "C特化", x: "C振り(無補正)" },
    // 変化技は物理と同じ表示(冒頭のコメント)
    status: { none: "無振り", x_full: "A特化", x: "A振り(無補正)" },
  };

  test.each(
    (["physical", "special", "status"] as const).flatMap((category) =>
      ATTACKER_PRESET_KEYS.map((key) => [category, key, expected[category][key]] as const),
    ),
  )("%s の %s は「%s」", (category, key, label) => {
    expect(attackerPresetLabel(key, category)).toBe(label);
  });
});
