// issue #275: 逆算の「受けたダメージ」で、自分(防御側)の耐久調整を選べるようにするための
// 防御側プリセット(ADR-0009 §1 のカタログ8件)の Web 側の定義。
//
// 正は engine の `DefenderPresetCatalog()`(engine/bulk.go、期待値は engine/bulk_test.go の
// TestDefenderPresetCatalogDefinitions)。この値は API 経由では取れない(BulkCalcRequest はキーだけを送り、
// engine が内部で解決する。ADR-0009 §5)ので、attackerPresets.ts と同じく Web 側にも同じ規則を持つ。
// engine との一致は defenderPresets.contract.test.ts が engine/bulk.go を読んで確かめる
// (このファイルは「Web の今の挙動」を固定するテスト。attackerPresets.test.ts ↔ contract.test.ts と同じ分担)。
//
// このテストで決めること(engine に無い、Web の画面のための規則):
//   - 技の分類ごとの選択肢(engine の DefaultDefenderPresets と同じ絞り込み)
//   - 技の分類が変わったときの読み替え(B 系 ↔ D 系の対応。変化技は H 振りまで落とす)。
//     画面のラジオグループに常に1つ checked が残るようにするため(issue #275 の UI)。

import { describe, expect, test } from "vitest";
import type { MoveCategory, Nature, Stats, StatKey } from "../engine/types";
import {
  DEFAULT_DEFENDER_PRESET,
  DEFENDER_PRESET_KEYS,
  defenderPresetForCategory,
  defenderPresetKeysFor,
  defenderPresetLabel,
  resolveDefenderPreset,
  type DefenderPresetKey,
} from "./defenderPresets";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL, NEUTRAL_NATURE, ZERO_SP } from "./requests";

const statKeys: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];
const categories: readonly MoveCategory[] = ["physical", "special", "status"];

function spTotal(sp: Stats): number {
  return statKeys.reduce((sum, key) => sum + sp[key], 0);
}

const BOOST_DEF: Nature = { plus: "def", minus: "atk" };
const BOOST_SPD: Nature = { plus: "spd", minus: "atk" };

describe("カタログ", () => {
  test("8件で、順序は ADR-0009 §1 のカタログ順(耐久が上がる順)", () => {
    expect(DEFENDER_PRESET_KEYS).toEqual([
      "none",
      "hp",
      "hb_boost",
      "hb",
      "hb_full",
      "hd_boost",
      "hd",
      "hd_full",
    ]);
  });

  test("既定は none(無振り。issue #275 以前の固定値と同じ)", () => {
    expect(DEFAULT_DEFENDER_PRESET).toBe("none");
  });
});

describe("resolveDefenderPreset(SP・性格)", () => {
  // engine/bulk.go の DefenderPresetCatalog() と同じ値(engine との一致は contract.test.ts が確かめる)。
  // 防御側プリセットはキーごとに振り先が決まっている(B 系 / D 系)ので、攻撃側と違い技の分類を取らない。
  test.each([
    ["none", ZERO_SP, NEUTRAL_NATURE],
    ["hp", { ...ZERO_SP, hp: 32 }, NEUTRAL_NATURE],
    ["hb_boost", { ...ZERO_SP, hp: 32 }, BOOST_DEF],
    ["hb", { ...ZERO_SP, hp: 32, def: 32 }, NEUTRAL_NATURE],
    ["hb_full", { ...ZERO_SP, hp: 32, def: 32 }, BOOST_DEF],
    ["hd_boost", { ...ZERO_SP, hp: 32 }, BOOST_SPD],
    ["hd", { ...ZERO_SP, hp: 32, spd: 32 }, NEUTRAL_NATURE],
    ["hd_full", { ...ZERO_SP, hp: 32, spd: 32 }, BOOST_SPD],
  ] as const)("%s", (key, sp, nature) => {
    expect(resolveDefenderPreset(key)).toEqual({ sp, nature });
  });

  test.each(DEFENDER_PRESET_KEYS)("%s は SP の上限(1ステータス 32・合計 66)を守る", (key) => {
    const { sp } = resolveDefenderPreset(key);
    for (const stat of statKeys) {
      expect(sp[stat]).toBeGreaterThanOrEqual(0);
      expect(sp[stat]).toBeLessThanOrEqual(MAX_SP_PER_STAT);
    }
    expect(spTotal(sp)).toBeLessThanOrEqual(MAX_SP_TOTAL);
  });

  test.each(DEFENDER_PRESET_KEYS)("%s の性格補正は HP を指さない(engine の validate と同じ規約)", (key) => {
    const { nature } = resolveDefenderPreset(key);
    expect(nature.plus).not.toBe("hp");
    expect(nature.minus).not.toBe("hp");
  });

  test.each(DEFENDER_PRESET_KEYS)("%s の性格補正は上昇・下降が揃っているか、両方とも空", (key) => {
    const { nature } = resolveDefenderPreset(key);
    expect(nature.plus === "").toBe(nature.minus === "");
    if (nature.plus !== "") {
      expect(nature.plus).not.toBe(nature.minus);
    }
  });

  test("返す SP・性格は呼ぶたびに同じ値で、共有の定数(ZERO_SP・NEUTRAL_NATURE)を書き換えない", () => {
    const zeroBefore = { ...ZERO_SP };
    const neutralBefore = { ...NEUTRAL_NATURE };
    resolveDefenderPreset("hb_full");
    resolveDefenderPreset("none");
    expect(ZERO_SP).toEqual(zeroBefore);
    expect(NEUTRAL_NATURE).toEqual(neutralBefore);
    expect(resolveDefenderPreset("hd")).toEqual(resolveDefenderPreset("hd"));
  });
});

describe("defenderPresetKeysFor(技の分類ごとの選択肢。engine の DefaultDefenderPresets と同じ絞り込み)", () => {
  test.each([
    ["physical", ["none", "hp", "hb_boost", "hb", "hb_full"]],
    ["special", ["none", "hp", "hd_boost", "hd", "hd_full"]],
    ["status", ["none", "hp"]],
  ] as const)("%s は %j", (category, keys) => {
    expect(defenderPresetKeysFor(category)).toEqual(keys);
  });

  test.each(categories)("%s の選択肢はカタログの部分列(順序を入れ替えない)", (category) => {
    const keys = defenderPresetKeysFor(category);
    const catalogOrder = DEFENDER_PRESET_KEYS.filter((key) => keys.includes(key));
    expect(keys).toEqual(catalogOrder);
  });

  test.each(categories)("%s の選択肢には必ず既定(none)が入る", (category) => {
    expect(defenderPresetKeysFor(category)).toContain(DEFAULT_DEFENDER_PRESET);
  });
});

describe("defenderPresetForCategory(分類が変わったときの読み替え)", () => {
  // 技の分類が変わると選択肢が入れ替わる(B 系 ↔ D 系)。振り方の意図(H だけ / 振り切り / 特化)は
  // 保ったまま、対になるプリセットへ読み替える。変化技は none / hp しか無いので H 振りまで落とす
  // (B・D の振り分けは変化技では意味を持たない。そもそもダメージを計算しない)。
  test.each([
    ["physical", "none", "none"],
    ["physical", "hp", "hp"],
    ["physical", "hb_boost", "hb_boost"],
    ["physical", "hb", "hb"],
    ["physical", "hb_full", "hb_full"],
    ["physical", "hd_boost", "hb_boost"],
    ["physical", "hd", "hb"],
    ["physical", "hd_full", "hb_full"],
    ["special", "none", "none"],
    ["special", "hp", "hp"],
    ["special", "hb_boost", "hd_boost"],
    ["special", "hb", "hd"],
    ["special", "hb_full", "hd_full"],
    ["special", "hd_boost", "hd_boost"],
    ["special", "hd", "hd"],
    ["special", "hd_full", "hd_full"],
    ["status", "none", "none"],
    ["status", "hp", "hp"],
    ["status", "hb_boost", "hp"],
    ["status", "hb", "hp"],
    ["status", "hb_full", "hp"],
    ["status", "hd_boost", "hp"],
    ["status", "hd", "hp"],
    ["status", "hd_full", "hp"],
  ] as const)("%s のとき %s → %s", (category, key, expected) => {
    expect(defenderPresetForCategory(key, category)).toBe(expected);
  });

  test.each(categories.flatMap((category) => DEFENDER_PRESET_KEYS.map((key) => [category, key] as const)))(
    "%s × %s の結果は、その分類の選択肢に入っている(ラジオが必ず1つ checked になる)",
    (category, key) => {
      expect(defenderPresetKeysFor(category)).toContain(defenderPresetForCategory(key, category));
    },
  );

  test.each(categories.flatMap((category) => DEFENDER_PRESET_KEYS.map((key) => [category, key] as const)))(
    "%s × %s は冪等(同じ分類に読み替え直しても変わらない)",
    (category, key) => {
      const once = defenderPresetForCategory(key, category);
      expect(defenderPresetForCategory(once, category)).toBe(once);
    },
  );

  test("物理 → 特殊 → 物理 と往復しても元のプリセットに戻る(振り方の意図を落とさない)", () => {
    for (const key of defenderPresetKeysFor("physical")) {
      const toSpecial = defenderPresetForCategory(key, "special");
      expect(defenderPresetForCategory(toSpecial, "physical")).toBe(key);
    }
  });
});

describe("defenderPresetLabel(表示名。文言は i18n/ja.ts、engine の Label と同じ)", () => {
  const expected: Record<DefenderPresetKey, string> = {
    none: "無振り",
    hp: "H振り",
    hb_boost: "H振り+B補正",
    hb: "HB振り",
    hb_full: "HB特化",
    hd_boost: "H振り+D補正",
    hd: "HD振り",
    hd_full: "HD特化",
  };

  test.each(DEFENDER_PRESET_KEYS)("%s の表示名", (key) => {
    expect(defenderPresetLabel(key)).toBe(expected[key]);
  });

  test("表示名は8件とも違う(ラジオの名前で見分けが付く)", () => {
    const labels = DEFENDER_PRESET_KEYS.map((key) => defenderPresetLabel(key));
    expect(new Set(labels).size).toBe(labels.length);
  });
});
