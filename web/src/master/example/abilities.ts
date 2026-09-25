// P4-2: 架空の例データ(ADR-0300 §3)。実マスタを Git に置かない(ADR-0002)ため、
// 名前は「テスト」で始め、ID は「example」で始める。効果の数値は engine の効果定義の形に合わせた架空の値。
// P4-5(ADR-0301 §5 追記): ID にハイフンを含めない(calc-svc の共通マスタの codeIDPattern が
// `^[a-z0-9]+$` のみを許すため。services/internal/master/typechart.go)。

import type { Ability } from "../../engine/types";

export const exampleAbilities: Ability[] = [
  { id: "exampleabilitynone", nameJa: "テストむこう", effect: null },
  // タイプ一致補正を上げる架空の特性(engine.ModifierAdaptability 相当の 8192)。
  { id: "exampleabilityadapt", nameJa: "テストてきおう", effect: { stabMod: 8192 } },
];
