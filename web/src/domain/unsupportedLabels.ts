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
