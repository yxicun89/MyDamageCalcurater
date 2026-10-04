// I-web-1・I-web-3(ADR-0329): 計算画面の攻撃側の「攻撃」「特攻」の入力(SP の数値・性格補正・プリセット)を
// engine に渡す SP・性格に直す純粋関数。
// 決めたこと(ADR-0329 §2〜§5):
//   - 攻撃(A)と特攻(C)を別々に持つ。各ブロックは SP の文字列(入力途中を保てる)と性格補正(上昇・補正なし・下降)。
//   - SP は整数 0〜32(符号・小数点・指数・全角は不可)。空欄は 0(ADR-0316 §4 の構築の SP 欄と同じ)。
//   - H・B・D・S の SP は 0 のまま(攻撃側プリセットの契約 engine/presets/attacker.json と同じ)。
//     したがって合計は最大 32 + 32 = 64 で、合計 66 の上限は構造上超えない(テストで固定)。
//   - プリセット(無振り・特化・振り(無補正))は attackerPresets の値をそのまま1ブロックへ当てはめたもの。
//     ブロックの値がどのプリセットとも一致しなければ「カスタム」(null)。
//   - 上昇・下降はそれぞれ1つのステータスにしか付かない(実在する性格の形)。A と C が同時に上昇/下降になる選択は
//     画面が入力時に無効にする(isModifierSelectable)。関数としては null を返す(防御的)。
//   - 性格はマスタの性格一覧から解決する(オンラインの natureId 解決 resolveNatureId と必ず一致させるため)。
//     1) A・C の補正の両方に合う性格(補正なしの側は上昇・下降のどちらでもない)を ID の昇順で最初に選ぶ。
//     2) 無ければ、選択中の技の分類が使う側(物理・変化 = A、特殊 = C)の補正だけに合う性格を ID の昇順で選ぶ
//        (使わない側は今の技のダメージに効かない)。使う側が補正なしなら無補正の性格。
//     3) それでも無ければ null(画面は明示エラーにして計算しない)。
//   - 両方補正なしは NEUTRAL_NATURE({plus:"",minus:""})。既定の要求を従来とバイト同一にするため。

import { describe, expect, test } from "vitest";
import { resolveNatureId } from "../api/apiEngine";
import type { MoveCategory, Stats, StatKey } from "../engine/types";
import { exampleNatures } from "../master/example/natures";
import type { MasterNature } from "../master/types";
import { ATTACKER_PRESET_KEYS, resolveAttackerPreset, type AttackerPresetKey } from "./attackerPresets";
import {
  ATTACK_STATS,
  DEFAULT_ATTACKER_STAT_INPUTS,
  NATURE_MODIFIERS,
  attackStatFor,
  isModifierSelectable,
  matchingPreset,
  parseSpText,
  presetInput,
  resolveAttackerNature,
  resolveAttackerStats,
  type AttackStatInput,
  type AttackerStatInputs,
  type NatureModifier,
} from "./attackerStatInputs";
import { MAX_SP_TOTAL, NEUTRAL_NATURE, ZERO_SP } from "./requests";

/** 架空の性格 25 種(上昇 × 下降の全組み合わせ。ID は「test-」で始める。実データは置かない。ADR-0002)。 */
const BOOSTABLE: readonly StatKey[] = ["atk", "def", "spa", "spd", "spe"];
const fullNatures: MasterNature[] = BOOSTABLE.flatMap((plus) =>
  BOOSTABLE.map((minus): MasterNature =>
    plus === minus
      ? { id: `test-nature-neutral-${plus}`, nameJa: `テスト無補正${plus}`, plus: null, minus: null }
      : { id: `test-nature-${plus}-${minus}`, nameJa: `テスト${plus}${minus}`, plus, minus },
  ),
);

function inputs(atk: Partial<AttackStatInput>, spa: Partial<AttackStatInput>): AttackerStatInputs {
  return {
    atk: { ...DEFAULT_ATTACKER_STAT_INPUTS.atk, ...atk },
    spa: { ...DEFAULT_ATTACKER_STAT_INPUTS.spa, ...spa },
  };
}

function modifiers(
  atk: NatureModifier,
  spa: NatureModifier,
): Readonly<Record<"atk" | "spa", NatureModifier>> {
  return { atk, spa };
}

function total(sp: Stats): number {
  return (Object.keys(sp) as StatKey[]).reduce((sum, key) => sum + sp[key], 0);
}

describe("定数", () => {
  test("ブロックは 攻撃 → 特攻 の順、補正は 上昇 → 補正なし → 下降 の順", () => {
    expect(ATTACK_STATS).toEqual(["atk", "spa"]);
    expect(NATURE_MODIFIERS).toEqual(["up", "neutral", "down"]);
  });

  test("既定は両ブロックとも SP 0・補正なし(従来の既定 = 無振りと同じ)", () => {
    expect(DEFAULT_ATTACKER_STAT_INPUTS).toEqual({
      atk: { spText: "0", modifier: "neutral" },
      spa: { spText: "0", modifier: "neutral" },
    });
  });
});

describe("attackStatFor(技の分類 → 強調するブロック)", () => {
  test.each([
    ["physical", "atk"],
    ["special", "spa"],
    // 変化技は attackerPresets と同じく物理扱い(画面は変化技を選ばせないが、関数は全域にする)
    ["status", "atk"],
    // 技が無いときはどちらも強調しない
    [null, null],
  ] as const)("%s → %s", (category, stat) => {
    expect(attackStatFor(category)).toBe(stat);
  });
});

describe("parseSpText(SP の1欄)", () => {
  test.each([
    ["0", 0],
    ["32", 32],
    ["7", 7],
    ["032", 32],
    [" 12 ", 12],
    // 空欄は 0(ADR-0316 §4。1文字ずつ消す途中を弾かない)
    ["", 0],
    ["  ", 0],
  ])("%j は %d", (text, value) => {
    expect(parseSpText(text)).toEqual({ ok: true, value });
  });

  test.each(["33", "100", "-1", "1.5", "3.0", "1e1", "+3", "abc", "3a", "３"])(
    "%j は読めない(範囲外・整数でない)",
    (text) => {
      expect(parseSpText(text)).toEqual({ ok: false });
    },
  );
});

describe("プリセット ⇔ ブロックの値", () => {
  test.each([
    ["none", { spText: "0", modifier: "neutral" }],
    ["x_full", { spText: "32", modifier: "up" }],
    ["x", { spText: "32", modifier: "neutral" }],
  ] as const)("presetInput(%s)", (key, expected) => {
    expect(presetInput(key)).toEqual(expected);
  });

  // attackerPresets(engine/presets/attacker.json の契約)とずれないこと: 関連ステータスの SP と上昇補正が同じ。
  test.each(
    ATTACKER_PRESET_KEYS.flatMap((key) =>
      (["physical", "special"] as const).map((category) => [key, category] as const),
    ),
  )("presetInput(%s) は resolveAttackerPreset(%s) の関連ステータスと一致する", (key, category) => {
    const stat = category === "physical" ? "atk" : "spa";
    const preset = resolveAttackerPreset(key, category);
    const input = presetInput(key);
    expect(Number(input.spText)).toBe(preset.sp[stat]);
    expect(input.modifier).toBe(preset.nature.plus === stat ? "up" : "neutral");
  });

  test.each([
    [{ spText: "0", modifier: "neutral" }, "none"],
    [{ spText: "", modifier: "neutral" }, "none"],
    [{ spText: "32", modifier: "up" }, "x_full"],
    [{ spText: "032", modifier: "up" }, "x_full"],
    [{ spText: "32", modifier: "neutral" }, "x"],
    // どれとも一致しない = カスタム
    [{ spText: "20", modifier: "neutral" }, null],
    [{ spText: "0", modifier: "up" }, null],
    [{ spText: "32", modifier: "down" }, null],
    [{ spText: "33", modifier: "neutral" }, null],
    [{ spText: "1.5", modifier: "neutral" }, null],
  ] as const)("matchingPreset(%j) は %s", (input, expected) => {
    expect(matchingPreset(input)).toBe(expected);
  });

  test.each(ATTACKER_PRESET_KEYS)(
    "presetInput(%s) の値は matchingPreset で同じ Key に戻る",
    (key: AttackerPresetKey) => {
      expect(matchingPreset(presetInput(key))).toBe(key);
    },
  );
});

describe("isModifierSelectable(実在しない性格の組み合わせを入力時に防ぐ)", () => {
  test("補正なしはいつでも選べる", () => {
    for (const other of NATURE_MODIFIERS) {
      expect(isModifierSelectable(inputs({}, { modifier: other }), "atk", "neutral")).toBe(true);
      expect(isModifierSelectable(inputs({ modifier: other }, {}), "spa", "neutral")).toBe(true);
    }
  });

  test.each([
    // [A の補正, C の補正, 変える側, 選ぼうとする補正, 選べるか]
    ["neutral", "up", "atk", "up", false],
    ["neutral", "down", "atk", "down", false],
    ["neutral", "up", "atk", "down", true],
    ["neutral", "down", "atk", "up", true],
    ["up", "neutral", "spa", "up", false],
    ["down", "neutral", "spa", "down", false],
    ["up", "neutral", "spa", "down", true],
    ["neutral", "neutral", "spa", "up", true],
    // 自分自身の今の値は選べる(上昇のまま上昇を選び直しても無効にしない)
    ["up", "down", "atk", "up", true],
  ] as const)("A=%s・C=%s のとき %s に %s は選べる=%s", (atk, spa, stat, modifier, expected) => {
    expect(isModifierSelectable(inputs({ modifier: atk }, { modifier: spa }), stat, modifier)).toBe(expected);
  });
});

describe("resolveAttackerNature(マスタの性格一覧から解決)", () => {
  test("両方補正なしは NEUTRAL_NATURE(既定の要求を従来とバイト同一にする)", () => {
    for (const category of ["physical", "special", "status"] as const) {
      expect(resolveAttackerNature(fullNatures, modifiers("neutral", "neutral"), category)).toEqual(
        NEUTRAL_NATURE,
      );
      expect(resolveAttackerNature(exampleNatures, modifiers("neutral", "neutral"), category)).toEqual(
        NEUTRAL_NATURE,
      );
    }
  });

  // 25 種そろったマスタ: 技の分類によらず、A・C の両方に合う性格(補正なしの側は上昇・下降のどちらでもない)を
  // ID の昇順で最初に選ぶ。
  test.each([
    ["up", "down", { plus: "atk", minus: "spa" }],
    ["down", "up", { plus: "spa", minus: "atk" }],
    ["down", "neutral", { plus: "def", minus: "atk" }],
    ["neutral", "down", { plus: "def", minus: "spa" }],
  ] as const)("全種のマスタ: A=%s・C=%s → %j", (atk, spa, expected) => {
    for (const category of ["physical", "special"] as const) {
      expect(resolveAttackerNature(fullNatures, modifiers(atk, spa), category)).toEqual(expected);
    }
  });

  // 技が使う側が上昇でもう一方が補正なしなら、従来のプリセットと同じ代表性格(物理 +A/−C、特殊 +C/−A)を優先する。
  // 使わない側が上昇のときは ID の昇順(A↑・特殊技 → +A/−def、C↑・物理技 → +C/−def)。
  test.each([
    ["up", "neutral", "physical", { plus: "atk", minus: "spa" }],
    ["up", "neutral", "special", { plus: "atk", minus: "def" }],
    ["neutral", "up", "special", { plus: "spa", minus: "atk" }],
    ["neutral", "up", "physical", { plus: "spa", minus: "def" }],
  ] as const)("全種のマスタ: A=%s・C=%s・%s → %j", (atk, spa, category, expected) => {
    expect(resolveAttackerNature(fullNatures, modifiers(atk, spa), category)).toEqual(expected);
  });

  test.each([
    ["up", "up"],
    ["down", "down"],
  ] as const)("A=%s・C=%s は実在しない組み合わせなので null", (atk, spa) => {
    for (const category of ["physical", "special"] as const) {
      expect(resolveAttackerNature(fullNatures, modifiers(atk, spa), category)).toBeNull();
    }
  });

  // 例データ(性格が6種だけ)では両方に合う性格が無いことがある。そのときは技が使う側の補正だけを合わせる。
  test.each([
    // [A, C, 技の分類, 期待]
    ["up", "neutral", "physical", { plus: "atk", minus: "spa" }],
    ["up", "neutral", "special", NEUTRAL_NATURE],
    ["neutral", "up", "special", { plus: "spa", minus: "atk" }],
    ["neutral", "up", "physical", NEUTRAL_NATURE],
    ["neutral", "down", "special", { plus: "atk", minus: "spa" }],
    ["neutral", "down", "physical", NEUTRAL_NATURE],
    // 両方に合う性格がある(例データの def は +B/−A): ID の昇順で def が spd より先
    ["down", "neutral", "physical", { plus: "def", minus: "atk" }],
    ["down", "neutral", "special", { plus: "def", minus: "atk" }],
    ["up", "down", "special", { plus: "atk", minus: "spa" }],
    // 変化技は物理扱い
    ["up", "neutral", "status", { plus: "atk", minus: "spa" }],
  ] as const)("例データ: A=%s・C=%s・%s → %j", (atk, spa, category, expected) => {
    expect(resolveAttackerNature(exampleNatures, modifiers(atk, spa), category)).toEqual(expected);
  });

  test("技が使う側に合う性格がマスタに1つも無ければ null(画面は明示エラーにして計算しない)", () => {
    const withoutAtkBoost = exampleNatures.filter((nature) => nature.plus !== "atk");
    expect(resolveAttackerNature(withoutAtkBoost, modifiers("up", "neutral"), "physical")).toBeNull();
    // 技が使わない側だけが合わないなら、無補正で解決できる
    expect(resolveAttackerNature(withoutAtkBoost, modifiers("up", "neutral"), "special")).toEqual(
      NEUTRAL_NATURE,
    );
  });

  test("マスタの並び順に依存しない(ID の昇順で選ぶ)", () => {
    const reversed = [...fullNatures].reverse();
    // 使う側が上昇でもう一方が補正なしの代表性格の優先(物理 +A/−C)に当たらない組み合わせ(特殊技の A↑)で確かめる
    expect(resolveAttackerNature(reversed, modifiers("up", "neutral"), "special")).toEqual({
      plus: "atk",
      minus: "def",
    });
  });

  // オンライン(API)は natureId を送るので、解決した性格が必ず natureId に戻せること(WASM と API で同じ要求形)。
  test.each([
    ["fullNatures", fullNatures],
    ["exampleNatures", exampleNatures],
  ] as const)("%s: 解決できた性格はすべて resolveNatureId で natureId に戻せる", (_name, natures) => {
    for (const atk of NATURE_MODIFIERS) {
      for (const spa of NATURE_MODIFIERS) {
        for (const category of ["physical", "special"] as const satisfies readonly MoveCategory[]) {
          const nature = resolveAttackerNature(natures, modifiers(atk, spa), category);
          if (nature !== null) {
            expect(resolveNatureId(natures, nature)).toBeDefined();
          }
        }
      }
    }
  });

  test("解決した性格は、技が使う側に指定どおりの補正を掛ける", () => {
    for (const natures of [fullNatures, exampleNatures]) {
      for (const atk of NATURE_MODIFIERS) {
        for (const spa of NATURE_MODIFIERS) {
          for (const category of ["physical", "special"] as const) {
            const nature = resolveAttackerNature(natures, modifiers(atk, spa), category);
            if (nature === null) {
              continue;
            }
            const stat = category === "physical" ? "atk" : "spa";
            const wanted = stat === "atk" ? atk : spa;
            const applied =
              nature.plus === nature.minus
                ? "neutral"
                : nature.plus === stat
                  ? "up"
                  : nature.minus === stat
                    ? "down"
                    : "neutral";
            expect(applied).toBe(wanted);
          }
        }
      }
    }
  });
});

describe("resolveAttackerStats(画面の入力 → 要求の SP・性格)", () => {
  test("既定は ZERO_SP・NEUTRAL_NATURE(従来の無振りとバイト同一)", () => {
    const result = resolveAttackerStats(DEFAULT_ATTACKER_STAT_INPUTS, exampleNatures, "physical");
    expect(result).toEqual({ ok: true, sp: ZERO_SP, nature: NEUTRAL_NATURE });
    if (result.ok) {
      expect(JSON.stringify(result.sp)).toBe(JSON.stringify(ZERO_SP));
      expect(JSON.stringify(result.nature)).toBe(JSON.stringify(NEUTRAL_NATURE));
    }
  });

  test("攻撃と特攻の両方の SP を載せ、H・B・D・S は 0 のまま(技の分類によらない)", () => {
    for (const category of ["physical", "special"] as const) {
      const result = resolveAttackerStats(
        inputs({ spText: "20" }, { spText: "12" }),
        exampleNatures,
        category,
      );
      expect(result).toEqual({
        ok: true,
        sp: { hp: 0, atk: 20, def: 0, spa: 12, spd: 0, spe: 0 },
        nature: NEUTRAL_NATURE,
      });
    }
  });

  test("空欄は 0 として計算する", () => {
    const result = resolveAttackerStats(inputs({ spText: "" }, { spText: "" }), exampleNatures, "physical");
    expect(result).toEqual({ ok: true, sp: ZERO_SP, nature: NEUTRAL_NATURE });
  });

  test("A特化相当(A 32・上昇)は従来の A特化と同じ要求(例データ: +atk / −spa)", () => {
    const result = resolveAttackerStats(inputs(presetInput("x_full"), {}), exampleNatures, "physical");
    expect(result).toEqual({ ok: true, ...resolveAttackerPreset("x_full", "physical") });
  });

  test("C特化相当(C 32・上昇)は従来の C特化と同じ要求(例データ: +spa / −atk)", () => {
    const result = resolveAttackerStats(inputs({}, presetInput("x_full")), exampleNatures, "special");
    expect(result).toEqual({ ok: true, ...resolveAttackerPreset("x_full", "special") });
  });

  test("両方 32(最大)でも合計は 64 で MAX_SP_TOTAL(66)を超えない", () => {
    const result = resolveAttackerStats(
      inputs({ spText: "32", modifier: "up" }, { spText: "32", modifier: "down" }),
      fullNatures,
      "physical",
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(total(result.sp)).toBe(64);
      expect(total(result.sp)).toBeLessThanOrEqual(MAX_SP_TOTAL);
    }
  });

  test.each(["33", "-1", "1.5", "abc"])("攻撃の SP %j は sp の問題(atk)で ok=false", (text) => {
    const result = resolveAttackerStats(inputs({ spText: text }, {}), exampleNatures, "physical");
    expect(result).toEqual({ ok: false, issues: [{ kind: "sp", stat: "atk" }] });
  });

  test("技が使わない側(物理技での特攻)の SP が不正でも ok=false(要求に載せられないため計算しない)", () => {
    const result = resolveAttackerStats(inputs({}, { spText: "33" }), exampleNatures, "physical");
    expect(result).toEqual({ ok: false, issues: [{ kind: "sp", stat: "spa" }] });
  });

  test("両方不正なら両方の問題を 攻撃 → 特攻 の順で返す", () => {
    const result = resolveAttackerStats(
      inputs({ spText: "40" }, { spText: "-3" }),
      exampleNatures,
      "special",
    );
    expect(result).toEqual({
      ok: false,
      issues: [
        { kind: "sp", stat: "atk" },
        { kind: "sp", stat: "spa" },
      ],
    });
  });

  test("性格が解決できなければ nature の問題で ok=false", () => {
    const withoutAtkBoost = exampleNatures.filter((nature) => nature.plus !== "atk");
    const result = resolveAttackerStats(inputs({ modifier: "up" }, {}), withoutAtkBoost, "physical");
    expect(result).toEqual({ ok: false, issues: [{ kind: "nature" }] });
  });

  test("共有の定数(ZERO_SP・NEUTRAL_NATURE)を書き換えない", () => {
    const result = resolveAttackerStats(DEFAULT_ATTACKER_STAT_INPUTS, exampleNatures, "physical");
    if (!result.ok) {
      throw new Error("既定が失敗した");
    }
    expect(result.sp).not.toBe(ZERO_SP);
    expect(result.nature).not.toBe(NEUTRAL_NATURE);
  });
});
