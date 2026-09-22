// P4-5: 架空の性格(ADR-0300 §3・ADR-0301 §2)。API の Nature と同じ形。名前は「テスト」で始め、
// ID は「example-」で始める。無補正を2つ(natureId の「ID の昇順で最初」の規則の確認用)、
// 攻撃側プリセット(A特化/C特化)・防御側プリセット(+B/-A・+D/-A)に要る4種の補正をそれぞれ1つ置く。

import type { MasterNature } from "../types";

export const exampleNatures: MasterNature[] = [
  { id: "example-nature-neutral-hardy", nameJa: "テストがんばりや", plus: null, minus: null },
  { id: "example-nature-neutral-docile", nameJa: "テストすなお", plus: null, minus: null },
  // 攻撃側 A特化(物理)。
  { id: "example-nature-atk", nameJa: "テストいじっぱり", plus: "atk", minus: "spa" },
  // 攻撃側 C特化(特殊)。
  { id: "example-nature-spa", nameJa: "テストひかえめ", plus: "spa", minus: "atk" },
  // 防御側 +B/-A(hb_boost・hb_full、逆算の plus)。
  { id: "example-nature-def", nameJa: "テストずぶとい", plus: "def", minus: "atk" },
  // 防御側 +D/-A(hd_boost・hd_full、逆算の plus)。
  { id: "example-nature-spd", nameJa: "テストおだやか", plus: "spd", minus: "atk" },
];
