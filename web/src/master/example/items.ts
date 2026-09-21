// P4-2: 架空の例データ(ADR-0300 §3)。効果は engine の効果定義の形の架空の値で、実在の持ち物の再現ではない。
// defensiveItemCandidates(domain/requests.ts)の確認に要る組み合わせ: 防御を上げる・特防を上げる・
// 半減きのみ(タイプは example/moves.ts のダメージ技と一致させる)・ダメージ倍率、をそれぞれ1つ以上持つ。

import type { Item } from "../../engine/types";

export const exampleItems: Item[] = [
  // 防御を1.5倍(6144/4096)にする架空の持ち物。
  { id: "example-item-def", nameJa: "テストぼうぎょだま", effect: { statMods: { def: 6144 } } },
  // 特防を1.5倍にする架空の持ち物。
  { id: "example-item-spd", nameJa: "テストとくぼうだま", effect: { statMods: { spd: 6144 } } },
  // 最終ダメージを約1.3倍(5324/4096)にする架空の持ち物。
  { id: "example-item-power", nameJa: "テストちからのたま", effect: { damageMod: 5324 } },
  // ほのおタイプの技を半減する架空のきのみ(example/moves.ts の物理・ほのお技と対応)。
  { id: "example-item-fireberry", nameJa: "テストひやしのみ", effect: { resistBerryType: "fire" } },
];
