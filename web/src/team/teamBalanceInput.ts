// 構築の下書き(MemberDraft)を、バランスへ渡す入力(BalanceInputMember)にする(ADR-0421 §3)。
// 空の枠は speciesKey を "" にして渡す(useTeamBalance が無視する)。技の空欄は取り除く(重複は useTeamBalance が除く)。

import type { BalanceInputMember } from "../balance/types";
import type { MemberDraft } from "./teamMember";

export function balanceInputFromDraft(draft: MemberDraft): BalanceInputMember {
  return {
    speciesKey: draft.speciesKey ?? "",
    abilityId: draft.abilityId === "" ? null : draft.abilityId,
    moveIds: draft.moves.filter((moveId): moveId is string => moveId !== null && moveId !== ""),
  };
}
