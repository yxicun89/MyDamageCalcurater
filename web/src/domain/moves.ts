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
  return !isStatusMove(move);
}

/**
 * 変化技か。計算・逆算が変化技を受け取ったときに要求を送らず案内する安全網の判定(UI からは選べないが、
 * 外から技 ID が入る経路が将来できても要求を送らない。ADR-0328 §1)。
 */
export function isStatusMove(move: Move): boolean {
  return move.category === "status";
}

/**
 * 技の選択肢(ADR-0328): 種族の learnset のうちダメージを与える技(物理・特殊)だけ。learnset の順のまま。
 * 計算・逆算・調整の技欄はすべてこの関数で選択肢を作る。
 */
export function damagingLearnsetMoves(species: MasterSpecies, moves: readonly Move[]): Move[] {
  return learnsetMoves(species, moves).filter(isDamagingMove);
}

/** 種族が覚える最初のダメージ技(learnset の順)。覚えていなければ undefined。 */
export function firstDamagingMove(species: MasterSpecies, moves: readonly Move[]): Move | undefined {
  return damagingLearnsetMoves(species, moves)[0];
}
