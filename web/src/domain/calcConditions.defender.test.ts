// issue #274 残り(Web 分。ADR-0315 案): 防御側のランク(B / D)を「詳細」の条件に足す純粋関数側の約束。
//   - def / spd は別々に保持し、どちらかが非 0 なら defenderOverride.ranks に 5 項目(atk/spa/spe は 0)で両方送る
//   - 既定(0・0)なら defenderOverride を作らない(要求は従来とバイト単位で同じ)
//   - 編集対象は 物理 = def(B)・特殊 = spd(D)・変化/技なし = def(B)
//   - 表示は攻撃側と同じ作りの「B +1」「D -2」「B ±0」

import { describe, expect, test } from "vitest";
import {
  DEFAULT_CALC_CONDITIONS,
  conditionRequestParts,
  defenderRankStatFor,
  formatRank,
  type CalcConditions,
} from "./calcConditions";

function conditions(patch: Partial<CalcConditions>): CalcConditions {
  return { ...DEFAULT_CALC_CONDITIONS, ...patch };
}

describe("既定値", () => {
  test("defenderRanks の既定は def 0・spd 0(攻撃側の ranks とは別のキー)", () => {
    expect(DEFAULT_CALC_CONDITIONS.defenderRanks).toEqual({ def: 0, spd: 0 });
    expect(DEFAULT_CALC_CONDITIONS.ranks).toEqual({ atk: 0, spa: 0 });
  });
});

describe("conditionRequestParts: 防御側のランク", () => {
  test("既定は defenderOverride を作らない(キーも無い)", () => {
    const parts = conditionRequestParts(DEFAULT_CALC_CONDITIONS);
    expect(parts).not.toHaveProperty("defenderOverride");
    expect(JSON.stringify(parts)).toBe("{}");
  });

  test("def だけ +1 でも 5 項目を送る(spd は 0 のまま)", () => {
    expect(conditionRequestParts(conditions({ defenderRanks: { def: 1, spd: 0 } }))).toEqual({
      defenderOverride: { ranks: { atk: 0, def: 1, spa: 0, spd: 0, spe: 0 } },
    });
  });

  test("def +2 と spd -3 を別々に保持して両方送る", () => {
    expect(conditionRequestParts(conditions({ defenderRanks: { def: 2, spd: -3 } }))).toEqual({
      defenderOverride: { ranks: { atk: 0, def: 2, spa: 0, spd: -3, spe: 0 } },
    });
  });

  test("境界: +6 と -6 はそのまま送る", () => {
    expect(
      conditionRequestParts(conditions({ defenderRanks: { def: 6, spd: -6 } })).defenderOverride?.ranks,
    ).toEqual({ atk: 0, def: 6, spa: 0, spd: -6, spe: 0 });
  });

  test("攻撃側のランクとは独立(攻撃側 ranks と defenderOverride.ranks は別の部品)", () => {
    const parts = conditionRequestParts(
      conditions({ ranks: { atk: 3, spa: 0 }, defenderRanks: { def: -1, spd: 0 } }),
    );
    expect(parts.ranks).toEqual({ atk: 3, def: 0, spa: 0, spd: 0, spe: 0 });
    expect(parts.defenderOverride?.ranks).toEqual({ atk: 0, def: -1, spa: 0, spd: 0, spe: 0 });
  });

  test("防御側のランクだけでは攻撃側の ranks を作らない", () => {
    expect(conditionRequestParts(conditions({ defenderRanks: { def: 1, spd: 0 } }))).not.toHaveProperty(
      "ranks",
    );
  });

  test("防御側の状態異常・特性は defenderOverride に入れない(状態異常は式に効かない。特性は別経路)", () => {
    const override = conditionRequestParts(
      conditions({ defenderRanks: { def: 1, spd: 0 } }),
    ).defenderOverride;
    expect(Object.keys(override ?? {})).toEqual(["ranks"]);
  });
});

describe("defenderRankStatFor / formatRank", () => {
  test("物理 = def(B)・特殊 = spd(D)・変化/技なし = def(B)", () => {
    expect(defenderRankStatFor("physical")).toBe("def");
    expect(defenderRankStatFor("special")).toBe("spd");
    expect(defenderRankStatFor("status")).toBe("def");
    expect(defenderRankStatFor(null)).toBe("def");
  });

  test.each([
    ["def", 1, "B +1"],
    ["spd", -2, "D -2"],
    ["def", 0, "B ±0"],
    ["spd", 6, "D +6"],
    ["def", -6, "B -6"],
  ] as const)("%s %i → %s", (stat, value, expected) => {
    expect(formatRank(stat, value)).toBe(expected);
  });
});
