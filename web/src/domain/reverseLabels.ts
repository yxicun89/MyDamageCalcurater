// P4-4: 逆算の結果の表示の言葉(ADR-0016 §7、ADR-0010 §R3「目安の名前は表示層が付ける」)。
// SP の範囲は ranges をそのまま全部出す(1区間に畳まない)。目安の名前は、範囲に SP 0 / 32 が入っているとき、
// 性格クラスとの組で併記する:
//   防御側(H32 前提): 0+補正なし = H振り、0+上昇 = H振り+B(D)補正、32+補正なし = HB(HD)振り、32+上昇 = HB(HD)特化
//   攻撃側: 0+補正なし = 無振り、0+上昇 = A(C)補正のみ、32+補正なし = A(C)振り、32+上昇 = A(C)特化
// engine の NatureClass は "neutral" / "plus"(engine/reverse.go。下降補正は探索しない。ADR-0010 §R1)。

import type { Item, NatureClass, ReverseResult, ReverseSide, SPRange, StatKey } from "../engine/types";
import { calcScreenText, reverseResultText, statLetterJa } from "../i18n/ja";
import { MAX_SP_PER_STAT } from "./requests";

/** SP の下限(逆算が探索する範囲の下端。ADR-0010 §R1)。 */
const MIN_SP = 0;

/** ranges が SP を含むか(区間の端でなく内側でも含むとみなす)。 */
function containsSp(ranges: readonly SPRange[], sp: number): boolean {
  return ranges.some((range) => range.min <= sp && sp <= range.max);
}

/** SP 範囲の表記("B 4〜7, 9〜12"のように、区間は畳まずカンマ区切りで全部出す)。 */
export function formatSPRanges(stat: StatKey, ranges: readonly SPRange[]): string {
  const body = ranges
    .map((range) =>
      range.min === range.max ? String(range.min) : `${String(range.min)}〜${String(range.max)}`,
    )
    .join(", ");
  return `${statLetterJa[stat]} ${body}`;
}

/** 性格クラスの表示名("補正なし"、または関連ステータスの文字+"上昇")。 */
export function natureClassLabel(natureClass: NatureClass, stat: StatKey): string {
  if (natureClass === "neutral") {
    return reverseResultText.natureClassNeutral;
  }
  return `${statLetterJa[stat]}${reverseResultText.natureClassPlusSuffix}`;
}

function defenderZeroName(natureClass: NatureClass, letter: string): string {
  const guide = reverseResultText.guide;
  return natureClass === "plus"
    ? `${guide.defenderZeroPlusPrefix}${letter}${guide.defenderZeroPlusSuffix}`
    : guide.defenderZeroNeutral;
}

function defenderFullName(natureClass: NatureClass, letter: string): string {
  const guide = reverseResultText.guide;
  return natureClass === "plus"
    ? `${guide.defenderFullPlusPrefix}${letter}${guide.defenderFullPlusSuffix}`
    : `${guide.defenderFullNeutralPrefix}${letter}${guide.defenderFullNeutralSuffix}`;
}

function attackerZeroName(natureClass: NatureClass, letter: string): string {
  const guide = reverseResultText.guide;
  return natureClass === "plus" ? `${letter}${guide.attackerZeroPlusSuffix}` : guide.attackerZeroNeutral;
}

function attackerFullName(natureClass: NatureClass, letter: string): string {
  const guide = reverseResultText.guide;
  return natureClass === "plus"
    ? `${letter}${guide.attackerFullPlusSuffix}`
    : `${letter}${guide.attackerFullNeutralSuffix}`;
}

/**
 * 目安の名前(ADR-0010 §R3。必須ではないが表示層が付けてよい)。範囲が SP 0 を含めば「無振り」系、
 * SP 32(MAX_SP_PER_STAT)を含めば「特化」系の名前を、性格クラスと組んで返す。どちらも含まなければ空配列。
 */
export function reverseGuideNames(
  side: ReverseSide,
  stat: StatKey,
  natureClass: NatureClass,
  ranges: readonly SPRange[],
): string[] {
  const letter = statLetterJa[stat];
  const names: string[] = [];
  if (containsSp(ranges, MIN_SP)) {
    names.push(
      side === "defender" ? defenderZeroName(natureClass, letter) : attackerZeroName(natureClass, letter),
    );
  }
  if (containsSp(ranges, MAX_SP_PER_STAT)) {
    names.push(
      side === "defender" ? defenderFullName(natureClass, letter) : attackerFullName(natureClass, letter),
    );
  }
  return names;
}

/** 持ち物の表示名(空の itemId は「持ち物なし」。マスタに無い ID はそのまま出す)。 */
export function reverseItemLabel(itemId: string, items: readonly Item[]): string {
  if (itemId === "") {
    return calcScreenText.noItemRowLabel;
  }
  return items.find((item) => item.id === itemId)?.nameJa ?? itemId;
}

/** 防御側の結果に添える H32 前提の注記。攻撃側は前提を置かないので null。 */
export function reverseAssumptionNote(result: ReverseResult): string | null {
  if (result.side !== "defender") {
    return null;
  }
  return reverseResultText.assumedHpNote(result.assumedHpSp);
}
