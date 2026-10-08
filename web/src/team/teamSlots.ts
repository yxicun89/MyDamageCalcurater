// 構築の6枠の状態を扱う純粋関数(ADR-0332 §1・§2)。画面(TeamMemberEditor)は枠ごとの下書きを持ち、
// 保存のときだけ種族の決まった枠を枠の順に詰めて API の members にする。

import type { components } from "../api/openapi.gen";
import { blankDraft, draftToMember, memberToDraft, type MemberDraft } from "./teamMember";

type Schemas = components["schemas"];

/** 枠の数(パーティの上限。api/openapi.yaml の TeamInput.members の maxItems)。 */
export const TEAM_SLOT_COUNT = 6;

/** 種族がまだ決まっていない枠。 */
export function isEmptySlot(slot: MemberDraft): boolean {
  return slot.speciesKey === null;
}

/** 保存済みのメンバーを前の枠から入れ、残りを空の枠(性格は渡した既定)にした TEAM_SLOT_COUNT 枠。 */
export function slotsFromMembers(
  members: readonly Schemas["TeamMember"][],
  defaultNatureId: string,
): MemberDraft[] {
  return Array.from({ length: TEAM_SLOT_COUNT }, (_, index) => {
    const member = members[index];
    return member === undefined ? blankDraft(defaultNatureId) : memberToDraft(member);
  });
}

export type SlotsResult =
  | { readonly ok: true; readonly members: Schemas["TeamMember"][] }
  /** invalidPositions = 保存できない枠の位置(1 始まり)。 */
  | { readonly ok: false; readonly invalidPositions: number[] };

/** 種族の決まった枠だけを枠の順に詰める。空の枠は飛ばす(保存の妨げにしない)。 */
export function slotsToMembers(slots: readonly MemberDraft[]): SlotsResult {
  const members: Schemas["TeamMember"][] = [];
  const invalidPositions: number[] = [];
  slots.forEach((slot, index) => {
    if (isEmptySlot(slot)) {
      return;
    }
    const result = draftToMember(slot);
    if (result.ok) {
      members.push(result.member);
    } else {
      invalidPositions.push(index + 1);
    }
  });
  return invalidPositions.length === 0 ? { ok: true, members } : { ok: false, invalidPositions };
}

/** index の枠を隣(delta = -1 で上、1 で下)と入れ替えた新しい配列。範囲外は同じ並びのまま。 */
export function swapSlots<T>(slots: readonly T[], index: number, delta: -1 | 1): T[] {
  const target = index + delta;
  const moved = slots[index];
  const other = slots[target];
  if (moved === undefined || other === undefined) {
    return [...slots];
  }
  return slots.map((slot, position) => (position === index ? other : position === target ? moved : slot));
}

/** index の枠だけを空に戻した新しい配列(他の枠は位置も中身も変えない)。 */
export function clearSlot(
  slots: readonly MemberDraft[],
  index: number,
  defaultNatureId: string,
): MemberDraft[] {
  return slots.map((slot, position) => (position === index ? blankDraft(defaultNatureId) : slot));
}

/**
 * 保存済みの members と下書きの違い。下書きが保存できない状態なら true。
 * 保存済みも下書きと同じ変換(memberToDraft → draftToMember)に通して比べる(省略された項目の null 化を差と数えない)。
 */
export function hasUnsavedChanges(
  saved: readonly Schemas["TeamMember"][],
  slots: readonly MemberDraft[],
): boolean {
  const draft = slotsToMembers(slots);
  if (!draft.ok) {
    return true;
  }
  const normalized: Schemas["TeamMember"][] = [];
  for (const member of saved) {
    const result = draftToMember(memberToDraft(member));
    if (!result.ok) {
      return true;
    }
    normalized.push(result.member);
  }
  return JSON.stringify(normalized) !== JSON.stringify(draft.members);
}

/** create・update の body。name は送らない(省略するとサーバーが既定名を入れる。ADR-0229)。 */
export function teamInputFromMembers(members: readonly Schemas["TeamMember"][]): Schemas["TeamInput"] {
  return { members: [...members] };
}
