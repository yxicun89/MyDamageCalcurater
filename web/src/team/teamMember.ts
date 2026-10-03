// P5-5b PR-A2(ADR-0316): 構築のメンバー編集の純粋関数(SP の検査・メンバー ⇔ 下書きの変換・種族変更)。
//
// 画面(TeamScreen)は下書き(MemberDraft)を持ち、保存のときだけ draftToMember で API の TeamMember にする。
// 入力中の文字列(SP)をそのまま持てるよう、下書きの SP は文字列(範囲外・空欄も一時的に持てる)。

import type { components } from "../api/openapi.gen";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import type { Ability, StatKey } from "../engine/types";
import type { MasterSpecies } from "../master/types";

type Schemas = components["schemas"];

/** 1体が覚える技の数の上限(api/openapi.yaml の TeamMember.moveIds の maxItems)。 */
export const MAX_MEMBER_MOVES = 4;

/** SP の6ステータス(表示順 H・A・B・C・D・S)。 */
export const SP_STATS: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];

/** 画面が持つメンバー1体の下書き。技は常に MAX_MEMBER_MOVES 個の枠(空きは null)。 */
export interface MemberDraft {
  readonly speciesKey: string | null;
  readonly nickname: string | null;
  readonly moves: readonly (string | null)[];
  readonly itemId: string | null;
  readonly abilityId: string | null;
  readonly natureId: string;
  readonly sp: Readonly<Record<StatKey, string>>;
  readonly teraType: Schemas["PokeType"] | null;
}

/** SP の入力の問題(ステータス単位の範囲外 / 合計超過)。 */
export type SpIssue =
  { readonly kind: "stat"; readonly stat: StatKey } | { readonly kind: "total"; readonly total: number };

/** checkSpDraft の結果。問題が1つでもあれば sp は null。 */
export interface SpCheck {
  readonly sp: Schemas["StatBlock"] | null;
  /** 数値として読めた欄の合計(空欄は 0)。 */
  readonly total: number;
  /** MAX_SP_TOTAL - total(超過のときは負)。 */
  readonly remaining: number;
  readonly issues: readonly SpIssue[];
}

/** メンバー1体を保存できない理由。 */
export type MemberIssue =
  | { readonly kind: "speciesRequired" }
  | { readonly kind: "moveDuplicate"; readonly moveId: string }
  | { readonly kind: "sp"; readonly issue: SpIssue };

export type DraftResult =
  | { readonly ok: true; readonly member: Schemas["TeamMember"] }
  | { readonly ok: false; readonly issues: readonly MemberIssue[] };

/** SP の入力として読める形(符号・小数点・指数・全角を含まない 10 進整数)。 */
const SP_DIGITS = /^\d+$/;

/** SP の6欄の文字列を検査する(空欄 = 0。整数で 0〜MAX_SP_PER_STAT、合計 MAX_SP_TOTAL 以下)。 */
export function checkSpDraft(draft: Readonly<Record<StatKey, string>>): SpCheck {
  const issues: SpIssue[] = [];
  const read = (stat: StatKey): number => {
    const text = draft[stat].trim();
    if (text === "") {
      return 0;
    }
    if (!SP_DIGITS.test(text)) {
      issues.push({ kind: "stat", stat });
      return 0;
    }
    const value = Number(text);
    if (value > MAX_SP_PER_STAT) {
      issues.push({ kind: "stat", stat });
    }
    return value;
  };
  const values: Schemas["StatBlock"] = {
    hp: read("hp"),
    atk: read("atk"),
    def: read("def"),
    spa: read("spa"),
    spd: read("spd"),
    spe: read("spe"),
  };
  const total = SP_STATS.reduce((sum, stat) => sum + values[stat], 0);
  if (total > MAX_SP_TOTAL) {
    issues.push({ kind: "total", total });
  }
  return { sp: issues.length === 0 ? values : null, total, remaining: MAX_SP_TOTAL - total, issues };
}

/** 技を MAX_MEMBER_MOVES 枠に前から詰める(空きは null)。 */
function toMoveSlots(moveIds: readonly string[]): (string | null)[] {
  return Array.from({ length: MAX_MEMBER_MOVES }, (_, index) => moveIds[index] ?? null);
}

/** 何も入っていない新しいメンバーの下書き(種族は未選択。性格は呼び出し側が決めた ID)。 */
export function blankDraft(natureId: string): MemberDraft {
  return {
    speciesKey: null,
    nickname: null,
    moves: toMoveSlots([]),
    itemId: null,
    abilityId: null,
    natureId,
    sp: { hp: "0", atk: "0", def: "0", spa: "0", spd: "0", spe: "0" },
    teraType: null,
  };
}

/** API の TeamMember → 下書き(技を MAX_MEMBER_MOVES 枠に詰める。ニックネームなどは持ち回す)。 */
export function memberToDraft(member: Schemas["TeamMember"]): MemberDraft {
  return {
    speciesKey: member.speciesKey,
    nickname: member.nickname ?? null,
    moves: toMoveSlots(member.moveIds),
    itemId: member.itemId ?? null,
    abilityId: member.abilityId ?? null,
    natureId: member.natureId,
    sp: {
      hp: String(member.sp.hp),
      atk: String(member.sp.atk),
      def: String(member.sp.def),
      spa: String(member.sp.spa),
      spd: String(member.sp.spd),
      spe: String(member.sp.spe),
    },
    teraType: member.teraType ?? null,
  };
}

/** 下書き → API の TeamMember。保存できない理由があれば ok: false(API は呼ばない)。 */
export function draftToMember(draft: MemberDraft): DraftResult {
  const issues: MemberIssue[] = [];
  if (draft.speciesKey === null) {
    issues.push({ kind: "speciesRequired" });
  }
  const moveIds = draft.moves.filter((moveId): moveId is string => moveId !== null);
  for (const moveId of new Set(moveIds)) {
    if (moveIds.filter((candidate) => candidate === moveId).length > 1) {
      issues.push({ kind: "moveDuplicate", moveId });
    }
  }
  const spCheck = checkSpDraft(draft.sp);
  for (const issue of spCheck.issues) {
    issues.push({ kind: "sp", issue });
  }
  if (draft.speciesKey === null || spCheck.sp === null || issues.length > 0) {
    return { ok: false, issues };
  }
  const member: Schemas["TeamMember"] = {
    speciesKey: draft.speciesKey,
    moveIds,
    itemId: draft.itemId,
    abilityId: draft.abilityId,
    natureId: draft.natureId,
    sp: spCheck.sp,
    teraType: draft.teraType,
    // ニックネームは編集 UI を持たないが、update は全置換なので読んだ値を落とさず送り返す。
    ...(draft.nickname === null ? {} : { nickname: draft.nickname }),
  };
  return { ok: true, member };
}

/**
 * 種族を変えた下書き。特性は新しい種族が持つなら保ち、持たなければ先頭(無ければ null)に選び直す。
 * 技は新しい種族の learnset にあるものだけ残し(順は保つ・前に詰める)、他は外す。持ち物・性格・SP・
 * テラスタイプ・ニックネームは保つ。`abilities` は新しい種族の選択肢(domain/requests.ts の selectableAbilities)。
 */
export function changeSpecies(
  draft: MemberDraft,
  species: MasterSpecies,
  abilities: readonly Ability[],
): MemberDraft {
  const keepsAbility = abilities.some((ability) => ability.id === draft.abilityId);
  const keptMoves = draft.moves.filter(
    (moveId): moveId is string => moveId !== null && species.learnset.includes(moveId),
  );
  return {
    ...draft,
    speciesKey: species.key,
    abilityId: keepsAbility ? draft.abilityId : (abilities[0]?.id ?? null),
    moves: toMoveSlots(keptMoves),
  };
}
