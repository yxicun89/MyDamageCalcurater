// G-01(ADR-0341): 技の選択肢の並び。タイプ順だけ(ADR-0335 の習得順・五十音順の切り替えは廃止)。
// types(マスタのタイプ表 master.typeChart.types)の並びで群にし、群の中は五十音順。画面の並べ替えだけで engine・API は変えない。

import type { Move } from "../engine/types";

// ひらがな/カタカナ・濁点は同じ字として比べ(同じ字の中で濁点は後)、長音は照合器の扱いに従う。
const collator = new Intl.Collator("ja");

function compareById(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function byKana(a: Move, b: Move): number {
  return collator.compare(a.nameJa, b.nameJa) || compareById(a.id, b.id);
}

/**
 * タイプ順に並べた新しい配列を返す(入力は書き換えない)。群の並びは types、表に無いタイプは最後にタイプ ID の昇順。
 * 群の中は五十音順(同順位は技 ID の昇順)。入力の順に依らず決定的。
 */
export function orderMoves(moves: readonly Move[], types: readonly string[]): Move[] {
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
    return [...list].sort(byKana);
  });
}
