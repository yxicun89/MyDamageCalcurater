// P5-5b PR-A2(ADR-0316): メンバー編集の選択肢を作る純粋関数。選択肢はマスタから作る(ハードコードしない)。
// マスタに無い ID を持つメンバー(別のマスタ版で保存した構築など)も壊さず開けるよう、現在値だけは
// 選択肢に足す(足さないと select が現在値を表示できず、見た目と保存される値がずれる)。

import type { Ability, Item, Move } from "../engine/types";
import type { MasterData, MasterNature, MasterSpecies } from "../master/types";

interface Named {
  readonly id: string;
  readonly nameJa: string;
}

/** 選択肢に現在値の ID が無ければ、末尾に fallback(id) を足す(null は足さない)。 */
function withCurrent<T extends Named>(
  options: readonly T[],
  currentId: string | null,
  fallback: (id: string) => T,
): readonly T[] {
  if (currentId === null || options.some((option) => option.id === currentId)) {
    return options;
  }
  return [...options, fallback(currentId)];
}

export function itemOptions(items: readonly Item[], currentId: string | null): readonly Item[] {
  return withCurrent(items, currentId, (id) => ({ id, nameJa: id, effect: null }));
}

export function natureOptions(natures: readonly MasterNature[], currentId: string): readonly MasterNature[] {
  return withCurrent(natures, currentId, (id) => ({ id, nameJa: id, plus: null, minus: null }));
}

export function abilityOptions(abilities: readonly Ability[], currentId: string | null): readonly Ability[] {
  return withCurrent(abilities, currentId, (id) => ({ id, nameJa: id, effect: null }));
}

/** 技の選択肢: その種族の learnset を技の一覧(pool)で名前解決したもの + 現在値(learnset 外・解決不能)。 */
export function moveOptions(
  species: MasterSpecies | null,
  pool: readonly Move[],
  currentId: string | null,
): readonly Named[] {
  const candidates = (species?.learnset ?? []).flatMap((id) =>
    pool.filter((move) => move.id === id).slice(0, 1),
  );
  return withCurrent<Named>(candidates, currentId, (id) => ({
    id,
    nameJa: pool.find((move) => move.id === id)?.nameJa ?? id,
  }));
}

/** 新しい枠の性格の既定: 無補正(plus・minus とも null)の先頭、無ければ先頭、マスタが空なら ""。 */
export function defaultNatureId(master: MasterData): string {
  const neutral = master.natures.find((nature) => nature.plus === null && nature.minus === null);
  return (neutral ?? master.natures[0])?.id ?? "";
}
