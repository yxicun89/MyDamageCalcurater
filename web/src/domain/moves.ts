// P4-2: 技セレクタの候補(種族の learnset → 技)と、既定で選ぶ技(最初のダメージ技)。
// 技の一覧はマスタのデータが正で、画面は learnset の順をそのまま使う(並べ替えない)。

import type { Move } from "../engine/types";
import type { MasterSpecies } from "../master/types";

/** 種族の learnset を技の実体に解決する(learnset の順のまま、一覧に無い ID は飛ばす)。 */
export function learnsetMoves(species: MasterSpecies, moves: readonly Move[]): Move[] {
  const byId = new Map(moves.map((move) => [move.id, move]));
  return species.learnset.flatMap((id) => {
    const move = byId.get(id);
    return move === undefined ? [] : [move];
  });
}

/** ダメージ技か(変化技以外)。 */
export function isDamagingMove(move: Move): boolean {
  return move.category !== "status";
}

/** 種族が覚える最初のダメージ技(learnset の順)。覚えていなければ undefined。 */
export function firstDamagingMove(species: MasterSpecies, moves: readonly Move[]): Move | undefined {
  return learnsetMoves(species, moves).find(isDamagingMove);
}
