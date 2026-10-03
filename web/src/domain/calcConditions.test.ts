// issue #274(ADR-0312): 計算画面の「詳細」の条件(急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク)を
// 要求の部品に直す純粋関数。実装は src/domain/calcConditions.ts(未実装 = このテストは失敗する)。
// 約束(DECISIONS.md 2026-09-25「計算条件の入力 UI」、iOS に揃える):
//   - 既定のままなら何も足さない(要求は従来とバイト単位で同じ。critical:false・field:{}・ranks:全0 も送らない)
//   - 触った分だけ足す。ランクは atk/spa を別々に持ち、どれかが 0 でなければ 5 項目すべてを送る(def/spd/spe は 0)

import { describe, expect, test } from "vitest";
import {
  DEFAULT_CALC_CONDITIONS,
  MAX_RANK,
  MIN_RANK,
  TERRAIN_IDS,
  WEATHER_IDS,
  clampRank,
  conditionRequestParts,
  formatRank,
  rankStatFor,
  type CalcConditions,
} from "./calcConditions";

function conditions(patch: Partial<CalcConditions>): CalcConditions {
  return { ...DEFAULT_CALC_CONDITIONS, ...patch };
}

describe("既定値と選択肢", () => {
  test("既定は 急所 off・やけど off・天候なし・フィールドなし・壁なし・ランク 0", () => {
    expect(DEFAULT_CALC_CONDITIONS).toEqual({
      critical: false,
      burned: false,
      weather: "none",
      terrain: "none",
      defenderScreens: { reflect: false, lightScreen: false, auroraVeil: false },
      ranks: { atk: 0, spa: 0 },
      // issue #274 残り(ADR-0315): 防御側のランク。既定 0 のときは要求に何も載せない。
      defenderRanks: { def: 0, spd: 0 },
    });
  });

  test("天候・フィールドの並びは DECISIONS.md のとおり(フィールドは openapi の enum 順ではない)", () => {
    expect([...WEATHER_IDS]).toEqual(["none", "sun", "rain", "sand", "snow"]);
    expect([...TERRAIN_IDS]).toEqual(["none", "electric", "grassy", "psychic", "misty"]);
  });

  test("ランクの範囲は -6..+6", () => {
    expect([MIN_RANK, MAX_RANK]).toEqual([-6, 6]);
  });
});

describe("conditionRequestParts: 既定なら何も足さない", () => {
  test("既定は空のオブジェクト(キーが1つも無い。undefined のキーも作らない)", () => {
    const parts = conditionRequestParts(DEFAULT_CALC_CONDITIONS);
    expect(parts).toEqual({});
    expect(Object.keys(parts)).toEqual([]);
    expect(JSON.stringify(parts)).toBe("{}");
  });
});

describe("conditionRequestParts: 触った分だけ足す", () => {
  test("急所 on は critical:true だけ", () => {
    expect(conditionRequestParts(conditions({ critical: true }))).toEqual({ critical: true });
  });

  test("やけど on は status:'burn' だけ(他の状態異常は送らない)", () => {
    expect(conditionRequestParts(conditions({ burned: true }))).toEqual({ status: "burn" });
  });

  test.each(["sun", "rain", "sand", "snow"] as const)("天候 %s は field.weather だけ", (weather) => {
    expect(conditionRequestParts(conditions({ weather }))).toEqual({ field: { weather } });
  });

  test("天候 none は何も足さない(境界: 'none' を送らない)", () => {
    expect(conditionRequestParts(conditions({ weather: "none" }))).toEqual({});
  });

  test.each(["electric", "grassy", "psychic", "misty"] as const)(
    "フィールド %s は field.terrain だけ",
    (terrain) => {
      expect(conditionRequestParts(conditions({ terrain }))).toEqual({ field: { terrain } });
    },
  );

  test("壁は独立で、1つでも on なら field.defenderScreens に 3 項目すべてを入れる(攻撃側の壁は送らない)", () => {
    const parts = conditionRequestParts(
      conditions({ defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false } }),
    );
    expect(parts).toEqual({
      field: { defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false } },
    });
    expect(parts.field).not.toHaveProperty("attackerScreens");
  });

  test("天候・フィールド・壁をすべて on にすると 1 つの field にまとまる", () => {
    const screens = { reflect: true, lightScreen: true, auroraVeil: true };
    expect(
      conditionRequestParts(conditions({ weather: "rain", terrain: "misty", defenderScreens: screens })),
    ).toEqual({ field: { weather: "rain", terrain: "misty", defenderScreens: screens } });
  });
});

describe("conditionRequestParts: ランク(atk/spa を別々に持って両方送る)", () => {
  test("atk だけ +1 でも 5 項目すべてを送る(spa は 0 のまま)", () => {
    expect(conditionRequestParts(conditions({ ranks: { atk: 1, spa: 0 } }))).toEqual({
      ranks: { atk: 1, def: 0, spa: 0, spd: 0, spe: 0 },
    });
  });

  test("atk +2 と spa -3 を別々に保持して両方送る", () => {
    expect(conditionRequestParts(conditions({ ranks: { atk: 2, spa: -3 } })).ranks).toEqual({
      atk: 2,
      def: 0,
      spa: -3,
      spd: 0,
      spe: 0,
    });
  });

  test("境界: +6 と -6 はそのまま送る", () => {
    expect(conditionRequestParts(conditions({ ranks: { atk: 6, spa: -6 } })).ranks).toEqual({
      atk: 6,
      def: 0,
      spa: -6,
      spd: 0,
      spe: 0,
    });
  });

  test("ランクが両方 0 なら ranks を送らない", () => {
    expect(conditionRequestParts(conditions({ ranks: { atk: 0, spa: 0 } }))).not.toHaveProperty("ranks");
  });
});

describe("clampRank / rankStatFor", () => {
  test.each([
    [-7, -6],
    [-6, -6],
    [0, 0],
    [6, 6],
    [7, 6],
    [100, 6],
    [-100, -6],
  ])("clampRank(%i) = %i", (input, expected) => {
    expect(clampRank(input)).toBe(expected);
  });

  test("clampRank は整数に丸める(小数・NaN は 0 に寄せる)", () => {
    expect(clampRank(2.6)).toBe(3);
    expect(clampRank(Number.NaN)).toBe(0);
  });

  test("編集対象は 物理=atk(A)・特殊=spa(C)。変化技・技なしは atk", () => {
    expect(rankStatFor("physical")).toBe("atk");
    expect(rankStatFor("special")).toBe("spa");
    expect(rankStatFor("status")).toBe("atk");
    expect(rankStatFor(null)).toBe("atk");
  });
});

describe("formatRank(表示は iOS と同じ「A +1」「C -2」「A ±0」)", () => {
  test.each([
    ["atk", 1, "A +1"],
    ["spa", -2, "C -2"],
    ["atk", 0, "A ±0"],
    ["spa", 6, "C +6"],
    ["atk", -6, "A -6"],
  ] as const)("%s %i → %s", (stat, value, expected) => {
    expect(formatRank(stat, value)).toBe(expected);
  });
});
