// I-web-13d(G-05、ADR-0347): 構築の編集画面の 6 枠のタイルの格子。埋まった枠はポケモンカード
// (画像かエンブレム・名前・タイプバッジ・持ち物の印)、空きの枠は「+」のタイル。押すとその枠の欄へ移る。
// 状態は持たない(下書きは TeamMemberEditor が持つ)。欄の編集そのものは下の枠(TeamMemberFields)でする。

import type { ReactNode } from "react";
import { megaStoneLabel } from "../domain/itemRoles";
import { megaItemLock } from "../domain/mega";
import { teamMemberText } from "../i18n/ja";
import type { MasterItem, MasterSpecies } from "../master/types";
import { PokemonCard } from "../ui/PokemonCard";
import { Tile } from "../ui/Tile";

/** 格子の 1 枠(species が null の間は、種族が未選択か、まだ引けていない)。 */
export interface TeamSlotView {
  readonly speciesKey: string | null;
  readonly species: MasterSpecies | null;
  readonly itemId: string | null;
}

export interface TeamSlotGridProps {
  readonly slots: readonly TeamSlotView[];
  readonly items: readonly MasterItem[];
  /** 押された枠(0 始まり)。 */
  readonly onSelect: (index: number) => void;
}

/** カードに出す持ち物の名前。メガ種族は固定のストーン、無い・引けないときは出さない。 */
function itemMarkName(slot: TeamSlotView, items: readonly MasterItem[]): string | undefined {
  const lock = megaItemLock(slot.species, items);
  if (lock.kind === "locked" && slot.species !== null) {
    return megaStoneLabel(slot.species, lock.item.nameJa);
  }
  if (lock.kind !== "none" || slot.itemId === null) {
    return undefined;
  }
  return items.find((item) => item.id === slot.itemId)?.nameJa;
}

export function TeamSlotGrid({ slots, items, onSelect }: TeamSlotGridProps): ReactNode {
  return (
    <div role="group" aria-label={teamMemberText.slotsLabel} className="team-slots">
      {slots.map((slot, index) => {
        const position = index + 1;
        if (slot.speciesKey === null) {
          return (
            <Tile
              key={index}
              empty
              label={teamMemberText.slotAddLabel(position)}
              className="team-slots__tile"
              tabIndex={-1}
              onClick={() => {
                onSelect(index);
              }}
            />
          );
        }
        return (
          <PokemonCard
            key={index}
            speciesKey={slot.speciesKey}
            name={slot.species?.nameJa ?? teamMemberText.memberLegend(position)}
            types={slot.species?.types ?? []}
            itemName={itemMarkName(slot, items)}
            className="team-slots__card"
            tabIndex={-1}
            onClick={() => {
              onSelect(index);
            }}
          />
        );
      })}
    </div>
  );
}
