// P4-2: 架空の例データ(ADR-0016 §3)。実マスタを Git に置かない(ADR-0002)ため、
// 名前は「テスト」で始め、ID は「example-」で始める。効果の数値は engine の効果定義の形に合わせた架空の値。

import type { Ability } from "../../engine/types";

export const exampleAbilities: Ability[] = [
  { id: "example-ability-none", nameJa: "テストむこう", effect: null },
  // タイプ一致補正を上げる架空の特性(engine.ModifierAdaptability 相当の 8192)。
  { id: "example-ability-adapt", nameJa: "テストてきおう", effect: { stabMod: 8192 } },
];
