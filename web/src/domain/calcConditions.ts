// issue 274(ADR-0312): 計算画面の「詳細」の条件(急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク)と、
// それを要求の部品に直す純粋関数。要求の形はここ1か所で決める。
// 既定のままなら何も足さない(要求は従来とバイト単位で同じ。false・none・全 0 も送らない)。

import type { Field, MoveCategory, Ranks, Screens } from "../engine/types";

/** 天候の ID(画面の並び。none は「なし」)。 */
export const WEATHER_IDS = ["none", "sun", "rain", "sand", "snow"] as const;
export type WeatherId = (typeof WEATHER_IDS)[number];

/** フィールドの ID(画面の並び。openapi の enum 順ではない)。 */
export const TERRAIN_IDS = ["none", "electric", "grassy", "psychic", "misty"] as const;
export type TerrainId = (typeof TERRAIN_IDS)[number];

/** ランクの範囲。 */
export const MIN_RANK = -6;
export const MAX_RANK = 6;

/** 画面で編集するランクのステータス(物理・変化 = atk、特殊 = spa)。 */
export type EditableRankStat = "atk" | "spa";

export interface CalcConditions {
  readonly critical: boolean;
  readonly burned: boolean;
  readonly weather: WeatherId;
  readonly terrain: TerrainId;
  readonly defenderScreens: Screens;
  readonly ranks: Readonly<Record<EditableRankStat, number>>;
}

export const DEFAULT_CALC_CONDITIONS: CalcConditions = {
  critical: false,
  burned: false,
  weather: "none",
  terrain: "none",
  defenderScreens: { reflect: false, lightScreen: false, auroraVeil: false },
  ranks: { atk: 0, spa: 0 },
};

/** 要求に足す部品。触っていない条件はキーごと無い。 */
export interface ConditionRequestParts {
  readonly critical?: true;
  readonly status?: "burn";
  readonly field?: Field;
  readonly ranks?: Ranks;
}

/** ランクを -6..+6 の整数に収める(小数は丸め、NaN は 0)。 */
export function clampRank(value: number): number {
  if (Number.isNaN(value)) {
    return 0;
  }
  return Math.min(MAX_RANK, Math.max(MIN_RANK, Math.round(value)));
}

/** 技の分類から、編集するランクのステータスを決める(変化技・技なしは atk)。 */
export function rankStatFor(category: MoveCategory | null): EditableRankStat {
  return category === "special" ? "spa" : "atk";
}

const RANK_STAT_LETTER: Record<EditableRankStat, string> = { atk: "A", spa: "C" };

/** 表示用の文字列(「A +1」「C -2」「A ±0」)。 */
export function formatRank(stat: EditableRankStat, value: number): string {
  const signed = value > 0 ? `+${String(value)}` : value < 0 ? String(value) : "±0";
  return `${RANK_STAT_LETTER[stat]} ${signed}`;
}

/** 条件を要求の部品にする。既定なら空のオブジェクト。 */
export function conditionRequestParts(conditions: CalcConditions): ConditionRequestParts {
  const { critical, burned, weather, terrain, defenderScreens, ranks } = conditions;
  const anyScreen = defenderScreens.reflect || defenderScreens.lightScreen || defenderScreens.auroraVeil;
  const field: Field = {
    ...(weather === "none" ? {} : { weather }),
    ...(terrain === "none" ? {} : { terrain }),
    ...(anyScreen ? { defenderScreens } : {}),
  };
  return {
    ...(critical ? { critical: true as const } : {}),
    ...(burned ? { status: "burn" as const } : {}),
    ...(Object.keys(field).length === 0 ? {} : { field }),
    ...(ranks.atk === 0 && ranks.spa === 0
      ? {}
      : { ranks: { atk: ranks.atk, def: 0, spa: ranks.spa, spd: 0, spe: 0 } }),
  };
}
