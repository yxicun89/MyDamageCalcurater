// ADR-0338 §3: 計算履歴の行を、お気に入りと同じ「計算に使う」の経路に流すための Favorite 形への変換(純粋。保存しない)。

import type { components } from "../api/openapi.gen";

type Schemas = components["schemas"];

/** 履歴の行を Favorite の形に包む。calc は同じ値、individual は攻撃側、日時は occurredAt。 */
export function historyEntryAsFavorite(
  entry: Schemas["CalcHistoryEntry"],
  label: string,
): Schemas["Favorite"] {
  return {
    id: `history:${entry.occurredAt}`,
    label,
    individual: entry.calc.attacker,
    calc: entry.calc,
    createdAt: entry.occurredAt,
    updatedAt: entry.occurredAt,
  };
}
