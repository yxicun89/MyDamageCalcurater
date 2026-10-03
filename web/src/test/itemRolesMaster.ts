// テスト専用(ADR-0326・ADR-0175 §4): 持ち物に roles・isMegaStone を付けたマスタ(オンラインの pokedex-svc が返す形)。
// 例データ(架空)の持ち物に、ADR-0175 §1 の規則で決まる役割を手で書き、役割の無い持ち物と両方の役割を持つ持ち物、
// メガストーン(roles は空・isMegaStone は true)を足す。名前はすべて架空(ADR-0002)。
// 役割は手書きの期待値(実装の関数で計算しない。テストが実装を写さないように)。

import { exampleItems } from "../master/example/items";
import type { MasterData, MasterItem } from "../master/types";
import { MEGA_FIRE_STONE, MEGA_WATER_STONE, withMegaFixture } from "./megaMaster";

function exampleItem(id: string): MasterItem {
  const found = exampleItems.find((item) => item.id === id);
  if (found === undefined) {
    throw new Error(`例データに持ち物 ${id} が無い`);
  }
  return found;
}

/** 防御を上げる(defender だけ)。 */
export const DEF_ITEM: MasterItem = {
  ...exampleItem("exampleitemdef"),
  roles: ["defender"],
  isMegaStone: false,
};
/** 特防を上げる(defender だけ)。 */
export const SPD_ITEM: MasterItem = {
  ...exampleItem("exampleitemspd"),
  roles: ["defender"],
  isMegaStone: false,
};
/** 最終ダメージ倍率(attacker だけ)。 */
export const POWER_ITEM: MasterItem = {
  ...exampleItem("exampleitempower"),
  roles: ["attacker"],
  isMegaStone: false,
};
/** 半減きのみ(defender だけ)。 */
export const BERRY_ITEM: MasterItem = {
  ...exampleItem("exampleitemfireberry"),
  roles: ["defender"],
  isMegaStone: false,
};
/** 攻撃と防御の両方を上げる架空の持ち物(attacker と defender)。 */
export const BOTH_ITEM: MasterItem = {
  id: "exampleitemboth",
  nameJa: "テストりょうほうだま",
  effect: { statMods: { atk: 6144, def: 6144 } },
  roles: ["attacker", "defender"],
  isMegaStone: false,
};
/** ダメージに効かない架空の持ち物(オボンのみ のような回復の持ち物の代わり。roles は空)。 */
export const NO_ROLE_ITEM: MasterItem = {
  id: "exampleitemheal",
  nameJa: "テストかいふくのみ",
  effect: null,
  roles: [],
  isMegaStone: false,
};
/** メガストーン(効果があっても roles は常に空。ADR-0175 §1)。 */
export const ROLE_MEGA_FIRE_STONE: MasterItem = { ...MEGA_FIRE_STONE, roles: [], isMegaStone: true };
export const ROLE_MEGA_WATER_STONE: MasterItem = { ...MEGA_WATER_STONE, roles: [], isMegaStone: true };
/**
 * どのメガ種族の requiredItemId にも現れないメガストーン(オンラインで、まだ解決していないメガ種族のストーン)。
 * megaStoneItemIds(種族から導く集合)では判別できず、isMegaStone だけが外せる。
 */
export const UNRESOLVED_MEGA_STONE: MasterItem = {
  id: "examplemegastoneunresolved",
  nameJa: "Examplite Z",
  effect: null,
  roles: [],
  isMegaStone: true,
};

/** マスタの順(この順で選択肢に出る)。 */
export const ROLE_ITEMS: readonly MasterItem[] = [
  DEF_ITEM,
  SPD_ITEM,
  POWER_ITEM,
  BERRY_ITEM,
  BOTH_ITEM,
  NO_ROLE_ITEM,
  ROLE_MEGA_FIRE_STONE,
  ROLE_MEGA_WATER_STONE,
  UNRESOLVED_MEGA_STONE,
];

/** 攻撃側の欄の選択肢(手書きの期待値。マスタの順)。 */
export const EXPECTED_ATTACKER_ITEMS: readonly MasterItem[] = [POWER_ITEM, BOTH_ITEM];
/** 防御側の欄の選択肢。 */
export const EXPECTED_DEFENDER_ITEMS: readonly MasterItem[] = [DEF_ITEM, SPD_ITEM, BERRY_ITEM, BOTH_ITEM];
/** 攻撃・防御のどちらかの役割を持つ持ち物(判定・調整)。 */
export const EXPECTED_EITHER_ITEMS: readonly MasterItem[] = [
  DEF_ITEM,
  SPD_ITEM,
  POWER_ITEM,
  BERRY_ITEM,
  BOTH_ITEM,
];
/** 役割で絞らない欄(構築の編集)。メガストーンだけ外す。 */
export const EXPECTED_ANY_ITEMS: readonly MasterItem[] = [
  DEF_ITEM,
  SPD_ITEM,
  POWER_ITEM,
  BERRY_ITEM,
  BOTH_ITEM,
  NO_ROLE_ITEM,
];

/** roles・isMegaStone を持たない形(古いサーバー・古いキャッシュ・例データの持ち物)。 */
export function withoutRoleFields(item: MasterItem): MasterItem {
  return { id: item.id, nameJa: item.nameJa, effect: item.effect };
}

/** 例データ + 架空のメガ種族(test/megaMaster.ts)に、役割つきの持ち物を差し替えたマスタ。 */
export function withItemRoles(base: MasterData): MasterData {
  return { ...withMegaFixture(base), items: ROLE_ITEMS };
}
