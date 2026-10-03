// ADR-0326: 持ち物の役割(攻撃側/防御側)による絞り込みと、メガストーンの日本語表示の文言。
// 計算・逆算・判定・調整・構築の編集の各レーンが同じ語を使うので、レーン別ではなく持ち物の文言として1か所に置き、
// 画面から直接 import する(ADR-0323。ja.ts の再エクスポートには足さない)。ja.ts を import しない(循環を避ける)。
// iOS は同じ語にそろえる(docs/mega-evolution-spec.md §4-3)。語を変えるときは iOS と同時に直す。

import type { ItemRole } from "../master/types";

/** 役割の見える名前(絞り込みで外した持ち物の通知に使う)。 */
const roleLabel: Record<ItemRole, string> = {
  attacker: "攻撃側",
  defender: "防御側",
};

export const itemRoleText = {
  /**
   * メガ種族を選んで持ち物が固定されているときの、持ち物欄の表示(ADR-0175 §4)。ストーンの nameJa は使わない
   * (上流に日本語名の無いストーンは英語のため。名前は推測しない)。
   */
  megaStoneOf: (baseSpeciesNameJa: string): string => `${baseSpeciesNameJa}のメガストーン`,
  /** 基本種名が分からない(baseSpeciesNameJa が null・省略)ときの固定の表示。 */
  megaStoneUnnamed: "メガストーン",
  /**
   * 攻守入れ替え・観測した側の切り替えで、選んでいた持ち物がその側の役割に合わなくなり外したときの通知
   * (role="status"。黙って外さない)。
   */
  droppedNotice: (itemNameJa: string, role: ItemRole): string =>
    `${itemNameJa}は${roleLabel[role]}では計算に影響しないため、持ち物を外しました`,
};
