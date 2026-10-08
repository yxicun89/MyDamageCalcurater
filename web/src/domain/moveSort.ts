// I-web-9 = F-02(ADR-0335): 技の選択肢の並び(習得順・五十音順・タイプ順)。画面の並べ替えだけで、engine・API は変えない。

import type { Move } from "../engine/types";

/** 並びの種類。learnset は従来の並び(種族の learnset の順のまま)。 */
export type MoveSortOrder = "learnset" | "kana" | "type";

export const MOVE_SORT_ORDERS: readonly MoveSortOrder[] = ["learnset", "kana", "type"];

export const DEFAULT_MOVE_SORT_ORDER: MoveSortOrder = "learnset";

export function isMoveSortOrder(value: unknown): value is MoveSortOrder {
  return typeof value === "string" && (MOVE_SORT_ORDERS as readonly string[]).includes(value);
}

/** タイプごとの群(optgroup の見出しのタイプ ID と、群の中の技。群の中は五十音順)。 */
export interface MoveTypeGroup {
  readonly type: string;
  readonly moves: readonly Move[];
}

// ひらがな/カタカナ・濁点は同じ字として比べ(同じ字の中で濁点は後)、長音は照合器の扱いに従う。
const collator = new Intl.Collator("ja");

function compareById(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function byKana(a: Move, b: Move): number {
  return collator.compare(a.nameJa, b.nameJa) || compareById(a.id, b.id);
}

/** 技のあるタイプだけ群にする。群の並びは types(マスタのタイプ表)、表に無いタイプは最後にタイプ ID の昇順。 */
export function moveTypeGroups(moves: readonly Move[], types: readonly string[]): MoveTypeGroup[] {
  const byType = new Map<string, Move[]>();
  for (const move of moves) {
    const list = byType.get(move.type);
    if (list === undefined) {
      byType.set(move.type, [move]);
    } else {
      list.push(move);
    }
  }
  const known = new Set(types);
  const unknown = [...byType.keys()].filter((type) => !known.has(type)).sort(compareById);
  return [...types, ...unknown].flatMap((type) => {
    const list = byType.get(type);
    // 相性表に同じタイプが重複しても群は1つ(Map から取り出して消す)。
    if (list === undefined) {
      return [];
    }
    byType.delete(type);
    return [{ type, moves: [...list].sort(byKana) }];
  });
}

/** 並びを適用した新しい配列を返す(入力は書き換えない)。 */
export function sortMoves(moves: readonly Move[], order: MoveSortOrder, types: readonly string[]): Move[] {
  switch (order) {
    case "learnset":
      return [...moves];
    case "kana":
      return [...moves].sort(byKana);
    case "type":
      return moveTypeGroups(moves, types).flatMap((group) => group.moves);
  }
}
