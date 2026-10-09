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

const JAPANESE_CHARACTER = /[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u;

/**
 * メガストーンの表示名(ADR-0328)。マスタの nameJa がひらがな・カタカナ・漢字を1文字以上含めばそのまま返す
 * (全角英数字だけ・英語・空・null は日本語として使えない)。使えないときは、基本種名があれば
 * 「{基本種名}のメガストーン」、無ければ「メガストーン」(名前を推測しない)。
 */
export function megaStoneDisplayName(
  stoneNameJa: string | null | undefined,
  baseSpeciesNameJa: string | null | undefined,
): string {
  const stoneName = stoneNameJa?.trim();
  if (stoneName !== undefined && JAPANESE_CHARACTER.test(stoneName)) {
    return stoneName;
  }
  const baseName = baseSpeciesNameJa?.trim();
  return baseName === undefined || baseName === ""
    ? itemRoleText.megaStoneUnnamed
    : itemRoleText.megaStoneOf(baseName);
}

/** メガ種族の固定中に持ち物欄へ出す名前。stoneNameJa はその種族が必要とするメガストーンのマスタ名。 */
export function megaStoneLabel(species: MasterSpecies, stoneNameJa?: string | null): string {
  return megaStoneDisplayName(stoneNameJa, species.baseSpeciesNameJa);
}

/**
 * engine に渡す持ち物の形(id・nameJa・effect だけ。境界は未知のフィールドを拒否する)。roles・isMegaStone は画面のための
 * 項目なので落とす。ただし防御側の持ち物(keepMegaStone)のメガストーンは isMegaStone: true を残す: 防御側の持ち物を払い落とす技が
 * メガストーンを除くため(ADR-0143)。攻撃側の持ち物には要らない。偽・省略は作らない。
 */
export function toEngineItem(item: MasterItem, keepMegaStone = false): Item {
  if (item.roles === undefined && item.isMegaStone === undefined) {
    return item;
  }
  return {
    id: item.id,
    nameJa: item.nameJa,
    effect: item.effect,
    ...(keepMegaStone && item.isMegaStone === true ? { isMegaStone: true } : {}),
  };
}

/**
 * 持ち物名を ID から引く表示用の一覧(結果の行・未対応の印)。メガストーンの nameJa は megaStoneDisplayName を通す
 * (日本語名はそのまま、使えないときは、その ID を requiredItemId に持つ species の基本種名から「{基本種名}のメガストーン」、
 * 引けなければ「メガストーン」。ADR-0328)。ストーンが1つも無ければ同じ配列を返す。要求(engine)には使わない(表示専用)。
 */
export function itemsWithStoneLabels(
  items: readonly MasterItem[],
  species: readonly (MasterSpecies | null)[],
  stoneIds?: ReadonlySet<string>,
): readonly MasterItem[] {
  const stones = stoneIds ?? new Set<string>();
  const baseNames = new Map<string, string | null | undefined>();
  for (const candidate of species) {
    if (candidate?.isMega === true && candidate.requiredItemId != null) {
      baseNames.set(candidate.requiredItemId, candidate.baseSpeciesNameJa);
    }
  }
  if (!items.some((item) => isMegaStoneItem(item, stones) || baseNames.has(item.id))) {
    return items;
  }
  return items.map((item) =>
    isMegaStoneItem(item, stones) || baseNames.has(item.id)
      ? { ...item, nameJa: megaStoneDisplayName(item.nameJa, baseNames.get(item.id)) }
      : item,
  );
}
