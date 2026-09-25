// issue 275: 逆算の「受けたダメージ」で、自分(防御側)の耐久調整を選べるようにするための
// 防御側プリセット(ADR-0009 §1 のカタログ8件)の Web 側の定義。
//
// 正は engine の `DefenderPresetCatalog()`(engine/bulk.go)。この値は API 経由では取れない
// (BulkCalcRequest はキーだけを送り、engine が内部で解決する。ADR-0009 §5)ので、
// attackerPresets.ts と同じく Web 側にも同じ規則を持つ(engine との一致は
// defenderPresets.contract.test.ts が確かめる)。

import type { MoveCategory, MoveCategoryOrNone, Nature, Stats } from "../engine/types";
import { defenderPresetText } from "../i18n/ja";
import { MAX_SP_PER_STAT, NEUTRAL_NATURE, ZERO_SP } from "./requests";

/** 防御側プリセットの Key(api/openapi.yaml の DefenderPreset enum と一致)。 */
export type DefenderPresetKey = "none" | "hp" | "hb_boost" | "hb" | "hb_full" | "hd_boost" | "hd" | "hd_full";

/** カタログの順序(ADR-0009 §1、engine/bulk.go の DefenderPresetCatalog() と同じ。耐久が上がる順)。 */
export const DEFENDER_PRESET_KEYS: readonly DefenderPresetKey[] = [
  "none",
  "hp",
  "hb_boost",
  "hb",
  "hb_full",
  "hd_boost",
  "hd",
  "hd_full",
];

/** 既定は無振り(issue 275 以前の固定値と同じ)。 */
export const DEFAULT_DEFENDER_PRESET: DefenderPresetKey = "none";

/** resolveDefenderPreset が返す SP・性格。 */
export interface ResolvedDefenderPreset {
  readonly sp: Stats;
  readonly nature: Nature;
}

const BOOST_DEF: Nature = { plus: "def", minus: "atk" };
const BOOST_SPD: Nature = { plus: "spd", minus: "atk" };

/** Key → SP・性格・技の分類による絞り込み(engine/bulk.go の DefenderPresetCatalog() と同じ値)。 */
const CATALOG: Readonly<
  Record<
    DefenderPresetKey,
    { readonly sp: Stats; readonly nature: Nature; readonly applies: MoveCategoryOrNone }
  >
> = {
  none: { sp: { ...ZERO_SP }, nature: { ...NEUTRAL_NATURE }, applies: "" },
  hp: { sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT }, nature: { ...NEUTRAL_NATURE }, applies: "" },
  hb_boost: { sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT }, nature: { ...BOOST_DEF }, applies: "physical" },
  hb: {
    sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT, def: MAX_SP_PER_STAT },
    nature: { ...NEUTRAL_NATURE },
    applies: "physical",
  },
  hb_full: {
    sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT, def: MAX_SP_PER_STAT },
    nature: { ...BOOST_DEF },
    applies: "physical",
  },
  hd_boost: { sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT }, nature: { ...BOOST_SPD }, applies: "special" },
  hd: {
    sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT, spd: MAX_SP_PER_STAT },
    nature: { ...NEUTRAL_NATURE },
    applies: "special",
  },
  hd_full: {
    sp: { ...ZERO_SP, hp: MAX_SP_PER_STAT, spd: MAX_SP_PER_STAT },
    nature: { ...BOOST_SPD },
    applies: "special",
  },
};

/**
 * Key から、engine に渡す SP・性格を求める(engine/bulk.go の DefenderPresetCatalog() と同じ値)。
 * ZERO_SP・NEUTRAL_NATURE・カタログの値を直接返さず新しいオブジェクトにし、共有の定数を書き換えない。
 */
export function resolveDefenderPreset(key: DefenderPresetKey): ResolvedDefenderPreset {
  const entry = CATALOG[key];
  return { sp: { ...entry.sp }, nature: { ...entry.nature } };
}

/** プリセットの表示名(文言は i18n/ja.ts、engine の Label と同じ)。 */
export function defenderPresetLabel(key: DefenderPresetKey): string {
  return defenderPresetText[key];
}

/**
 * 技の分類ごとの選択肢(engine の DefaultDefenderPresets と同じ絞り込み)。
 * 物理は Applies が空文字列または "physical"、特殊は空文字列または "special"、
 * 変化技は空文字列のプリセットだけ(= none・hp の2件)。
 */
export function defenderPresetKeysFor(category: MoveCategory): readonly DefenderPresetKey[] {
  return DEFENDER_PRESET_KEYS.filter((key) => {
    const applies = CATALOG[key].applies;
    return applies === "" || applies === category;
  });
}

/**
 * 技の分類が変わったとき、対になるプリセットへ読み替える(issue 275)。
 * B 系 ↔ D 系(振り方の意図は保つ)、変化技は H 振りまで落とす(それ以外は無振りのまま)。
 * これは表示専用の読み替えで、呼び出し側の state は元の key を保持したままにすること
 * (ReverseScreen.tsx の `effectiveDefenderPresetKey` を参照)。物理→変化技→物理のように分類を
 * 行き来すると、変化技を経由しても元のプリセット(例: HB特化)が復活する。
 */
export function defenderPresetForCategory(key: DefenderPresetKey, category: MoveCategory): DefenderPresetKey {
  const options = defenderPresetKeysFor(category);
  if (options.includes(key)) {
    return key;
  }
  if (category === "status") {
    return "hp";
  }
  const counterpart: Partial<Record<DefenderPresetKey, DefenderPresetKey>> =
    category === "physical"
      ? { hd_boost: "hb_boost", hd: "hb", hd_full: "hb_full" }
      : { hb_boost: "hd_boost", hb: "hd", hb_full: "hd_full" };
  return counterpart[key] ?? DEFAULT_DEFENDER_PRESET;
}
