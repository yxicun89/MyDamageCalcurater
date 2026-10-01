// issue 272(ADR-0311): 結果の行・候補に添える特性名の組み立て(純粋関数)。

import type { Ability } from "../engine/types";
import { calcScreenText } from "../i18n/ja";

/**
 * 行・候補の特性名(まとめた特性は区切って並べる)。表示しないときは null。
 *   - abilityIds が無い応答(特性を送らなかった / 古い応答)は出さない
 *   - 相手の種族の特性が1つだけなら、名前を出しても情報にならないので出さない
 *     (ただしまとめられた複数の特性があれば、種族の特性数によらず出す)
 * マスタに無い ID は ID のまま出す(未知データで画面を壊さない)。
 */
export function abilityNamesLabel(
  abilityIds: readonly string[] | undefined,
  abilities: readonly Ability[],
  speciesHasChoice: boolean,
): string | null {
  if (abilityIds === undefined || abilityIds.length === 0) {
    return null;
  }
  if (!speciesHasChoice && abilityIds.length < 2) {
    return null;
  }
  return abilityIds
    .map((id) => abilities.find((ability) => ability.id === id)?.nameJa ?? id)
    .join(calcScreenText.abilityNameSeparator);
}
