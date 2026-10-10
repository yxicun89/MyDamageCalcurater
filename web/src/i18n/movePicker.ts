// G-01(ADR-0341): 技ピッカーの文言。分類は用語集の語(ぶつり・とくしゅ・へんか。漢字にしない)。iOS と同じ語。

import type { MoveCategory } from "../engine/types";

export const movePickerText = {
  searchLabel: "技を検索",
  noResults: "見つかりません",
  category: { physical: "ぶつり", special: "とくしゅ", status: "へんか" } satisfies Record<
    MoveCategory,
    string
  >,
} as const;
