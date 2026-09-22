// P4-4: 逆算の持ち物候補(ADR-0300 §7、requirements.md「逆算の持ち物候補」)。ID・名前では選ばず、効果データから選ぶ。
//   防御側(side defender)= domain/requests.ts の isDefensiveItemCandidate と判定を共有する
//     (なし / 技の分類の防御側ステータス(物理→def、特殊→spd)を上げる / 技のタイプの半減きのみ)
//   攻撃側(side attacker)= なし / ダメージ倍率(damageMod)/ 分類の威力(powerMod。powerCategory が技の分類か全分類)
//                          / 技のタイプの強化(boostType が技のタイプ)/ 技の分類の攻撃側ステータス
//                          (物理→atk、特殊→spa)を上げる statMods(こだわり系のような持ち物。
//                          M-C には無いが、別レギュレーションでマスタに戻れば自動で候補に復活する。
//                          requirements.md「逆算の持ち物候補」)
// 先頭は必ず null(持ち物なし)、続きはマスタの順のまま(並べ替えない)。

import type { Item, Move, ReverseSide, StatKey } from "../engine/types";
import { NEUTRAL_MODIFIER, isDefensiveItemCandidate } from "./requests";

/**
 * 攻撃側の候補: 最終ダメージ倍率、分類の威力(全分類向けも含む)、技のタイプの強化、
 * 技の分類の攻撃側ステータス(物理→atk、特殊→spa)を上げる statMods、のいずれか。
 */
function isAttackerCandidate(item: Item, move: Move): boolean {
  const effect = item.effect;
  if (effect === null) {
    return false;
  }
  if ((effect.damageMod ?? 0) > NEUTRAL_MODIFIER) {
    return true;
  }
  const powerCategory = effect.powerCategory ?? "";
  if (
    (effect.powerMod ?? 0) > NEUTRAL_MODIFIER &&
    (powerCategory === move.category || powerCategory === "")
  ) {
    return true;
  }
  if (effect.boostType === move.type) {
    return true;
  }
  const relevantStat: StatKey = move.category === "special" ? "spa" : "atk";
  return (effect.statMods?.[relevantStat] ?? 0) > NEUTRAL_MODIFIER;
}

/**
 * 逆算で探索する持ち物候補(先頭は必ず null = 持ち物なし)。マスタの順序をそのまま使う(並べ替えない)。
 * 返す `Item` は渡された実体をそのまま(コピーしない。engine に解決済みの効果を渡すため)。
 */
export function reverseItemCandidates(
  side: ReverseSide,
  items: readonly Item[],
  move: Move,
): ReadonlyArray<Item | null> {
  const isCandidate = side === "defender" ? isDefensiveItemCandidate : isAttackerCandidate;
  return [null, ...items.filter((item) => isCandidate(item, move))];
}
