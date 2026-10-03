// issue 515・ADR-0320(docs/mega-evolution-spec.md §4-3): メガシンカの持ち物固定の共通ドメイン(純粋関数)。
// 計算・逆算(PR-A)と、構築の編集・判定(PR-B)が同じ関数を使う。名前・ID を直書きせず、
// マスタの isMega / requiredItemId だけから導く。

import type { Item } from "../engine/types";
import type { MasterSpecies } from "../master/types";

/** メガシンカ後の種族か。isMega が true のときだけ true(省略・null は false)。 */
export function isMegaSpecies(species: MasterSpecies | null): boolean {
  return species?.isMega === true;
}

/** メガ種族の requiredItemId に現れる持ち物 ID の集合(メガストーンの判別用。持ち物がマスタにあるかは見ない)。 */
export function megaStoneItemIds(speciesList: readonly MasterSpecies[]): ReadonlySet<string> {
  const ids = new Set<string>();
  for (const species of speciesList) {
    if (isMegaSpecies(species) && species.requiredItemId !== undefined && species.requiredItemId !== null) {
      ids.add(species.requiredItemId);
    }
  }
  return ids;
}

/** 種族の持ち物の固定。none = メガでない、locked = メガストーンに固定、missing = メガだがストーンを引けない。 */
export type MegaItemLock =
  { readonly kind: "none" } | { readonly kind: "locked"; readonly item: Item } | { readonly kind: "missing" };

export function megaItemLock(species: MasterSpecies | null, items: readonly Item[]): MegaItemLock {
  if (species === null || !isMegaSpecies(species)) {
    return { kind: "none" };
  }
  const requiredId = species.requiredItemId;
  const item =
    requiredId === undefined || requiredId === null
      ? undefined
      : items.find((entry) => entry.id === requiredId);
  return item === undefined ? { kind: "missing" } : { kind: "locked", item };
}

export interface ItemIdAfterSpeciesChangeInput {
  readonly previous: MasterSpecies | null;
  readonly next: MasterSpecies | null;
  readonly items: readonly Item[];
  readonly currentItemId: string;
}

/**
 * 種族を変えたときの持ち物 ID。メガへ → ストーン(引けなければ空)、メガから非メガ・未選択へ → 空
 * (メガストーンを残さない)、それ以外は現在の持ち物を保つ。
 */
export function itemIdAfterSpeciesChange(input: ItemIdAfterSpeciesChangeInput): string {
  const { previous, next, items, currentItemId } = input;
  const lock = megaItemLock(next, items);
  if (lock.kind === "locked") {
    return lock.item.id;
  }
  if (lock.kind === "missing" || isMegaSpecies(previous)) {
    return "";
  }
  return currentItemId;
}

/** 固定された持ち物(メガストーン。引けなければ null)、固定が無ければ選んだ持ち物。 */
export function lockedOrChosenItem(
  lock: MegaItemLock,
  pickable: readonly Item[],
  chosenId: string,
): Item | null {
  switch (lock.kind) {
    case "locked":
      return lock.item;
    case "missing":
      return null;
    case "none":
      return pickable.find((item) => item.id === chosenId) ?? null;
  }
}
