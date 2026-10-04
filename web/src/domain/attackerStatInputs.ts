// I-web-1・I-web-3(ADR-0329): 計算画面の攻撃側の「攻撃」「特攻」の入力(SP の数値・性格補正・プリセット)を
// engine に渡す SP・性格に直す純粋関数。
// 攻撃(A)と特攻(C)を別々に持つ。各ブロックは SP の文字列(入力途中を保てる)と性格補正(上昇・補正なし・下降)。
// H・B・D・S の SP は 0 のまま(攻撃側プリセットの契約 engine/presets/attacker.json と同じ)なので、
// 合計は最大 32 + 32 = 64 で、合計 66 の上限は構造上超えない。

import type { MoveCategory, Nature, Stats } from "../engine/types";
import type { MasterNature } from "../master/types";
import { ATTACKER_PRESET_KEYS, type AttackerPresetKey } from "./attackerPresets";
import { MAX_SP_PER_STAT, NEUTRAL_NATURE, ZERO_SP } from "./requests";
import { parseSpText } from "./spInput";

export { parseSpText };

/** 入力ブロックのステータス(攻撃 = atk、特攻 = spa)。表示順。 */
export type AttackStat = "atk" | "spa";
export const ATTACK_STATS: readonly AttackStat[] = ["atk", "spa"];

/** 性格補正(上昇・補正なし・下降)。表示順。 */
export type NatureModifier = "up" | "neutral" | "down";
export const NATURE_MODIFIERS: readonly NatureModifier[] = ["up", "neutral", "down"];

/** 1ブロックの入力。SP は入力途中を保てるよう文字列。 */
export interface AttackStatInput {
  readonly spText: string;
  readonly modifier: NatureModifier;
}

/** 画面が持つ攻撃側の入力(攻撃と特攻の両方)。 */
export type AttackerStatInputs = Readonly<Record<AttackStat, AttackStatInput>>;

/** 既定は両ブロックとも SP 0・補正なし(従来の既定 = 無振りと同じ)。 */
export const DEFAULT_ATTACKER_STAT_INPUTS: AttackerStatInputs = {
  atk: { spText: "0", modifier: "neutral" },
  spa: { spText: "0", modifier: "neutral" },
};

/** 技の分類が使うブロック(物理・変化 = 攻撃、特殊 = 特攻)。技が無ければ null(強調しない)。 */
export function attackStatFor(category: MoveCategory | null): AttackStat | null {
  if (category === null) {
    return null;
  }
  return category === "special" ? "spa" : "atk";
}

/** プリセットをブロックの値に直す(attackerPresets の関連ステータスの SP・上昇補正と同じ)。 */
export function presetInput(key: AttackerPresetKey): AttackStatInput {
  switch (key) {
    case "none":
      return { spText: "0", modifier: "neutral" };
    case "x_full":
      return { spText: String(MAX_SP_PER_STAT), modifier: "up" };
    case "x":
      return { spText: String(MAX_SP_PER_STAT), modifier: "neutral" };
  }
}

/** ブロックの値と一致するプリセット。どれとも一致しなければ null(カスタム)。 */
export function matchingPreset(input: AttackStatInput): AttackerPresetKey | null {
  const parsed = parseSpText(input.spText);
  if (!parsed.ok) {
    return null;
  }
  return (
    ATTACKER_PRESET_KEYS.find((key) => {
      const preset = presetInput(key);
      return Number(preset.spText) === parsed.value && preset.modifier === input.modifier;
    }) ?? null
  );
}

/**
 * stat のブロックを modifier にできるか。上昇・下降は1つのステータスにしか付かないので、
 * もう一方のブロックが同じ向きなら選べない(補正なしはいつでも選べる)。
 */
export function isModifierSelectable(
  inputs: AttackerStatInputs,
  stat: AttackStat,
  modifier: NatureModifier,
): boolean {
  if (modifier === "neutral") {
    return true;
  }
  return inputs[otherStat(stat)].modifier !== modifier;
}

function otherStat(stat: AttackStat): AttackStat {
  return stat === "atk" ? "spa" : "atk";
}

/** マスタの性格 nature が stat に modifier を掛けているか(補正なし = 上昇でも下降でもない)。 */
function appliesModifier(nature: MasterNature, stat: AttackStat, modifier: NatureModifier): boolean {
  switch (modifier) {
    case "up":
      return nature.plus === stat;
    case "down":
      return nature.minus === stat;
    case "neutral":
      return nature.plus !== stat && nature.minus !== stat;
  }
}

function toNature(nature: MasterNature): Nature {
  return { plus: nature.plus ?? "", minus: nature.minus ?? "" };
}

/**
 * A・C の補正から、マスタの性格を解決する(ADR-0329 §4)。オンラインの natureId 解決(resolveNatureId)と
 * 必ず一致させるため、マスタの性格だけを返す。解決できなければ null。
 *   1. 両方補正なし → NEUTRAL_NATURE
 *   2. A・C の両方に合う性格を ID の昇順で最初に選ぶ。ただし技が使う側が上昇でもう一方が補正なしなら、
 *      マイナスをもう一方の攻撃系に置く性格(物理 = +A/−C、特殊 = +C/−A。従来のプリセットと同じ)を優先
 *   3. 無ければ、技の分類が使う側だけを合わせる(使う側が補正なしなら NEUTRAL_NATURE)
 * A と C が同じ向きの組み合わせは実在しないので null。
 */
export function resolveAttackerNature(
  natures: readonly MasterNature[],
  modifiers: Readonly<Record<AttackStat, NatureModifier>>,
  category: MoveCategory,
): Nature | null {
  const { atk, spa } = modifiers;
  if (atk === "neutral" && spa === "neutral") {
    return { ...NEUTRAL_NATURE };
  }
  if (atk === spa) {
    return null;
  }
  const sorted = [...natures].sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  // 技が使う側が上昇でもう一方が補正なしのときは、従来のプリセット(ADR-0010 §R1 の代表性格。物理 = +A/−C、
  // 特殊 = +C/−A)と同じ性格を優先する(素早さなど他の能力に効くため、お気に入りの個体が変わらないように)。
  const preferred = attackStatFor(category) ?? "atk";
  if (modifiers[preferred] === "up" && modifiers[otherStat(preferred)] === "neutral") {
    const representative = sorted.find(
      (nature) => nature.plus === preferred && nature.minus === otherStat(preferred),
    );
    if (representative !== undefined) {
      return toNature(representative);
    }
  }
  const both = sorted.find(
    (nature) => appliesModifier(nature, "atk", atk) && appliesModifier(nature, "spa", spa),
  );
  if (both !== undefined) {
    return toNature(both);
  }
  const used = attackStatFor(category) ?? "atk";
  const usedModifier = modifiers[used];
  if (usedModifier === "neutral") {
    return { ...NEUTRAL_NATURE };
  }
  const usedOnly = sorted.find((nature) => appliesModifier(nature, used, usedModifier));
  return usedOnly === undefined ? null : toNature(usedOnly);
}

/** 入力が要求にできない理由。 */
export type AttackerStatIssue =
  { readonly kind: "sp"; readonly stat: AttackStat } | { readonly kind: "nature" };

/** resolveAttackerStats の結果。 */
export type AttackerStatsResult =
  | { readonly ok: true; readonly sp: Stats; readonly nature: Nature }
  | { readonly ok: false; readonly issues: readonly AttackerStatIssue[] };

/**
 * 画面の入力を要求の SP・性格にする。攻撃と特攻の両方の SP を載せる(技が使わない側もそのまま)。
 * 共有の定数(ZERO_SP・NEUTRAL_NATURE)は書き換えず、新しいオブジェクトを返す。
 */
export function resolveAttackerStats(
  inputs: AttackerStatInputs,
  natures: readonly MasterNature[],
  category: MoveCategory,
): AttackerStatsResult {
  const issues: AttackerStatIssue[] = [];
  const sp: { -readonly [K in keyof Stats]: Stats[K] } = { ...ZERO_SP };
  for (const stat of ATTACK_STATS) {
    const parsed = parseSpText(inputs[stat].spText);
    if (parsed.ok) {
      sp[stat] = parsed.value;
    } else {
      issues.push({ kind: "sp", stat });
    }
  }
  const nature = resolveAttackerNature(
    natures,
    { atk: inputs.atk.modifier, spa: inputs.spa.modifier },
    category,
  );
  if (nature === null) {
    issues.push({ kind: "nature" });
  }
  if (issues.length > 0 || nature === null) {
    return { ok: false, issues };
  }
  return { ok: true, sp, nature };
}
