// issue 271 / issue 270(ADR-0123): 「未対応」の印(technicalな ID)を表示名に解決する。
// 持ち物・技・ポケモンのリストをコードにハードコードしない(CLAUDE.md)ため、ID → 表示名はマスタ
// (master.moves / master.items / master.abilities)から引く。見つからなければ空文字を渡し、
// unsupportedText.markLabel 側が ID をそのまま出す(design.md「画面: ダメージ計算」)。

import type { Ability, Item, Move, UnsupportedMark } from "../engine/types";
import { unsupportedText } from "../i18n/ja";

function unsupportedMarkName(
  mark: UnsupportedMark,
  moves: readonly Move[],
  items: readonly Item[],
  abilities: readonly Ability[],
): string {
  switch (mark.target) {
    case "move":
      return moves.find((move) => move.id === mark.id)?.nameJa ?? "";
    case "attacker_item":
    case "defender_item":
      return items.find((item) => item.id === mark.id)?.nameJa ?? "";
    case "attacker_ability":
    case "defender_ability":
      return abilities.find((ability) => ability.id === mark.id)?.nameJa ?? "";
    default: {
      // 判別 union の網羅性チェック(コーディング規約 §4 TypeScript「判別 union は網羅性を検査する」)。
      const exhaustive: never = mark.target;
      throw new Error(`未知の UnsupportedTarget: ${JSON.stringify(exhaustive)}`);
    }
  }
}

/** 印 1 件の文言(ADR-0123)。ID → 表示名の解決込みで unsupportedText.markLabel を組み立てる。 */
export function unsupportedMarkLabel(
  mark: UnsupportedMark,
  moves: readonly Move[],
  items: readonly Item[],
  abilities: readonly Ability[],
): string {
  return unsupportedText.markLabel(mark, unsupportedMarkName(mark, moves, items, abilities));
}

/** 複数の印の文言(順は入力のまま。並べ替え・重複除去をしない。ADR-0300 §8)。 */
export function unsupportedMarkLabels(
  marks: readonly UnsupportedMark[],
  moves: readonly Move[],
  items: readonly Item[],
  abilities: readonly Ability[],
): string[] {
  return marks.map((mark) => unsupportedMarkLabel(mark, moves, items, abilities));
}

/** 印 1 件を一意に識別するキー(target・reason・id の組)。「全行(全候補)に共通か」の判定に使う。 */
function unsupportedMarkKey(mark: UnsupportedMark): string {
  return `${mark.target}\u0000${mark.reason}\u0000${mark.id}`;
}

/** {@link splitUnsupportedMarks} の戻り値。 */
export interface UnsupportedMarksSplit {
  /** 全行(全候補)に共通する印(結果の先頭に1回だけ出す。最初に現れた行の順)。 */
  readonly common: readonly UnsupportedMark[];
  /** 各行(候補)だけにある印(rowMarks と同じ長さ・同じ順。engine が返した順のまま)。 */
  readonly perRow: ReadonlyArray<readonly UnsupportedMark[]>;
}

/**
 * 全行(全候補)に共通して存在する印と、一部の行(候補)だけに存在する印を分ける
 * (iOS レーンの決定。docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、issue 271/270、
 * ADR-0501「P6-17」)。target で決め打ちせず、印の内容(target・reason・id の組)が rowMarks の
 * 全ての行(候補が無い・印が無い行も含む)に存在するかどうかで判定する。
 * 技の印は全行に付くことが多く、行ごとに出すと同じ文言が何度も並ぶため、共通の印は結果の先頭に
 * 1回だけ出し、行・候補ごとの表示からは外す(残りは今までどおりその行・候補だけに出す)。
 * rowMarks が空(結果が0件)のときは両方とも空を返す。
 */
export function splitUnsupportedMarks(
  rowMarks: ReadonlyArray<readonly UnsupportedMark[]>,
): UnsupportedMarksSplit {
  if (rowMarks.length === 0) {
    return { common: [], perRow: [] };
  }
  const rowCountByKey = new Map<string, number>();
  for (const marks of rowMarks) {
    const seenInRow = new Set<string>();
    for (const mark of marks) {
      const key = unsupportedMarkKey(mark);
      if (!seenInRow.has(key)) {
        seenInRow.add(key);
        rowCountByKey.set(key, (rowCountByKey.get(key) ?? 0) + 1);
      }
    }
  }
  const commonKeys = new Set(
    [...rowCountByKey.entries()].filter(([, count]) => count === rowMarks.length).map(([key]) => key),
  );
  const common: UnsupportedMark[] = [];
  const addedCommonKeys = new Set<string>();
  for (const marks of rowMarks) {
    for (const mark of marks) {
      const key = unsupportedMarkKey(mark);
      if (commonKeys.has(key) && !addedCommonKeys.has(key)) {
        addedCommonKeys.add(key);
        common.push(mark);
      }
    }
  }
  const perRow = rowMarks.map((marks) => marks.filter((mark) => !commonKeys.has(unsupportedMarkKey(mark))));
  return { common, perRow };
}
