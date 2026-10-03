// ADR-0326(ADR-0175 §4 のクライアント規約): 持ち物の選択肢を「その欄の役割」で絞る共通ドメイン(純粋関数)。
// 計算・逆算・判定・調整・構築の編集のすべての持ち物欄が、この1か所の関数で選択肢を作る。
// 役割はサーバーが返す roles だけを読み、効果(effect)から再導出しない。roles が無い持ち物は役割では絞らない。
// メガストーンは選択肢に出さない(isMegaStone、無ければ megaStoneItemIds の集合で判別)。固定に使うストーンは
// 絞り込む前の全件から domain/mega.ts の megaItemLock で引く。

import type { Item, ReverseSide } from "../engine/types";
import { itemRoleText } from "../i18n/items";
import type { ItemRole, MasterItem, MasterSpecies } from "../master/types";

/**
 * 持ち物欄の役割。
 * - attacker / defender: その側の役割(roles に含む持ち物だけ)。計算の攻撃側・防御側、逆算の自分。
 * - either: どちらかの役割を持つ持ち物(roles が空でない)。判定(双方向の確定数)・調整(指数は火力と耐久の両方)。
 * - any: 役割で絞らない(メガストーンだけ外す)。構築の編集(実際の対戦で持たせる持ち物を記録するため)。
 */
export type ItemRoleFilter = ItemRole | "either" | "any";

/** 判定画面の持ち物欄(自分・相手の候補とも。双方向に確定数を出すので攻撃・防御の両方)。 */
export const JUDGE_ITEM_ROLE_FILTER: ItemRoleFilter = "either";
/** 調整画面の自分の持ち物欄(モードを切り替えても入力を消さないので、モードに依らず両方)。 */
export const ADJUST_ITEM_ROLE_FILTER: ItemRoleFilter = "either";
/** 構築の編集の持ち物欄(ダメージに効かない持ち物も構築として記録する)。 */
export const TEAM_ITEM_ROLE_FILTER: ItemRoleFilter = "any";

/**
 * 逆算で自分の持ち物欄の役割。side は逆算する相手の側(ReverseSide)なので、自分はその反対
 * (与えたダメージ = side "defender" → 自分は attacker、受けたダメージ = side "attacker" → 自分は defender)。
 */
export function reverseMyItemRole(side: ReverseSide): ItemRole {
  return side === "defender" ? "attacker" : "defender";
}

/** 持ち物がメガストーンか(isMegaStone が true、または isMegaStone が無く stoneIds に含まれる)。 */
export function isMegaStoneItem(item: MasterItem, stoneIds: ReadonlySet<string>): boolean {
  return item.isMegaStone ?? stoneIds.has(item.id);
}

/**
 * 持ち物がその欄の役割に合うか(メガストーンかどうかは見ない)。roles が無い持ち物は常に true
 * (役割が分からないので絞らない)。any は常に true。
 */
export function itemMatchesRole(item: MasterItem, filter: ItemRoleFilter): boolean {
  if (filter === "any" || item.roles === undefined) {
    return true;
  }
  if (filter === "either") {
    return item.roles.length > 0;
  }
  return item.roles.includes(filter);
}

/**
 * 持ち物欄の選択肢(「持ち物なし」は含めない。欄が先頭に足す)。メガストーンを外し、役割に合うものだけを
 * マスタの順のまま返す(並べ替えない・実体は保つ)。stoneIds は isMegaStone の無い持ち物の判別用(省略は空集合)。
 */
export function itemsForRole(
  items: readonly MasterItem[],
  filter: ItemRoleFilter,
  stoneIds?: ReadonlySet<string>,
): readonly MasterItem[] {
  const stones = stoneIds ?? new Set<string>();
  return items.filter((item) => !isMegaStoneItem(item, stones) && itemMatchesRole(item, filter));
}

export interface ItemAfterRoleChangeInput {
  /** 絞り込む前の全件。 */
  readonly items: readonly MasterItem[];
  /** 変わった後の欄の役割。 */
  readonly role: ItemRole;
  /** いま選んでいる持ち物 ID(未選択は "")。 */
  readonly currentItemId: string;
}

export interface ItemAfterRoleChange {
  /** 変わった後の持ち物 ID(合わなければ "")。 */
  readonly itemId: string;
  /** 役割に合わず外した持ち物(通知の名前に使う)。外さなかったら null。 */
  readonly dropped: MasterItem | null;
}

/**
 * 攻守入れ替え・観測した側の切り替えで欄の役割が変わったときの持ち物。役割に合えば保ち、合わなければ未選択に戻して
 * 外した持ち物を返す(画面は itemRoleText.droppedNotice を role="status" で出す)。未選択・全件に無い ID はそのまま。
 * メガの固定はここでは扱わない(固定は種族から毎回導く)。
 */
export function itemAfterRoleChange(input: ItemAfterRoleChangeInput): ItemAfterRoleChange {
  const { items, role, currentItemId } = input;
  const current = items.find((item) => item.id === currentItemId);
  if (current === undefined || current.isMegaStone === true || itemMatchesRole(current, role)) {
    return { itemId: currentItemId, dropped: null };
  }
  return { itemId: "", dropped: current };
}

/**
 * メガ種族の固定中に持ち物欄へ出す名前(ADR-0175 §4)。baseSpeciesNameJa があれば「{基本種名}のメガストーン」、
 * null・省略・空白なら「メガストーン」(名前を推測しない。ストーンの nameJa は使わない)。
 */
export function megaStoneLabel(species: MasterSpecies): string {
  const name = species.baseSpeciesNameJa?.trim();
  return name === undefined || name === "" ? itemRoleText.megaStoneUnnamed : itemRoleText.megaStoneOf(name);
}

/** engine・calc-svc に渡す持ち物の形(id・nameJa・effect だけ。境界は未知のフィールドを拒否する)。 */
export function toEngineItem(item: MasterItem): Item {
  if (item.roles === undefined && item.isMegaStone === undefined) {
    return item;
  }
  return { id: item.id, nameJa: item.nameJa, effect: item.effect };
}
