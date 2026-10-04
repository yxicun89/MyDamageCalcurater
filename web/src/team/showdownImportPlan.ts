// P5-5e(ADR-0321 §3): parse の結果から、作成する構築のメンバーを決める純粋関数。
// メガ種族の持ち物は megaItemLock でストーンに直す(ADR-0320)。非メガは触らない。

import type { components } from "../api/openapi.gen";
import { megaItemLock } from "../domain/mega";
import type { MasterData } from "../master/types";
import type { ParseResult, ShowdownIssue } from "./showdownFormat";

type TeamMember = components["schemas"]["TeamMember"];

export interface ImportNote {
  readonly kind: "mega_item_fixed" | "mega_item_unavailable";
  /** plan.members の添字(parse の issue の memberIndex〈入力の何体目か〉とは、落ちたメンバーがあるとずれる)。 */
  readonly memberIndex: number;
  readonly speciesKey: string;
}

export interface ImportPlan {
  readonly members: TeamMember[];
  readonly issues: ShowdownIssue[];
  readonly notes: ImportNote[];
  readonly canCreate: boolean;
}

export function planShowdownImport(result: ParseResult, master: MasterData): ImportPlan {
  const notes: ImportNote[] = [];
  const members = result.members.map((member, memberIndex): TeamMember => {
    const species = master.species.find((s) => s.key === member.speciesKey) ?? null;
    const lock = megaItemLock(species, master.items);
    if (lock.kind === "locked") {
      if (member.itemId === lock.item.id) {
        return { ...member };
      }
      notes.push({ kind: "mega_item_fixed", memberIndex, speciesKey: member.speciesKey });
      return { ...member, itemId: lock.item.id };
    }
    if (lock.kind === "missing") {
      notes.push({ kind: "mega_item_unavailable", memberIndex, speciesKey: member.speciesKey });
      return { ...member, itemId: null };
    }
    return { ...member };
  });
  return { members, issues: [...result.issues], notes, canCreate: members.length >= 1 };
}
