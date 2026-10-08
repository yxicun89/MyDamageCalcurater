// ADR-0332 §1・§2(F-08 / I-web-7): 構築の6枠の状態を扱う純粋関数(team/teamSlots.ts。実装は後続)。
// 確かめること:
//   S-1 slotsFromMembers: 常に6枠。保存済みメンバーが前から入り、残りは空の枠(speciesKey が null・性格は既定)
//   S-2 slotsToMembers: 種族の決まった枠だけを枠の順に詰める(空の枠は飛ばし、保存の妨げにしない)。不正な枠は位置(1 始まり)で返す
//   S-3 swapSlots / clearSlot: 隣と入れ替え(空の枠とも)・範囲外は変えない / 外した枠だけが空になり他の枠は動かない
//   S-4 hasUnsavedChanges: 保存済みと同じなら false、変更・不正な下書きなら true
//   S-5 teamInputFromMembers: create/update の body は members だけ(name キーを持たない。ADR-0229)
// 架空の ID だけを使う(ADR-0002)。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { memberToDraft, type MemberDraft } from "./teamMember";
import {
  TEAM_SLOT_COUNT,
  clearSlot,
  hasUnsavedChanges,
  isEmptySlot,
  slotsFromMembers,
  slotsToMembers,
  swapSlots,
  teamInputFromMembers,
} from "./teamSlots";

type Schemas = components["schemas"];

const NATURE = "example-nature-neutral";

const FIRE: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  nickname: "テスト愛称",
  moveIds: ["examplemovetackle", "examplemovegrowl"],
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: "fire",
};

const ELECTRIC: Schemas["TeamMember"] = {
  speciesKey: "9004-000",
  moveIds: ["examplemovethunder"],
  itemId: null,
  abilityId: null,
  natureId: "example-nature-spa",
  sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 32 },
  teraType: null,
};

function speciesKeys(slots: readonly MemberDraft[]): (string | null)[] {
  return slots.map((slot) => slot.speciesKey);
}

/** 枠を1つ取り出す(無ければテストの誤り)。 */
function at(slots: readonly MemberDraft[], index: number): MemberDraft {
  const found = slots[index];
  if (found === undefined) {
    throw new Error(`${String(index + 1)}枠目が無い`);
  }
  return found;
}

function setSlot(slots: readonly MemberDraft[], index: number, next: MemberDraft): MemberDraft[] {
  return slots.map((slot, position) => (position === index ? next : slot));
}

describe("S-1 slotsFromMembers", () => {
  test("枠の数は6(パーティの上限と同じ)", () => {
    expect(TEAM_SLOT_COUNT).toBe(6);
  });

  test("メンバー0体: 6つとも空の枠。性格は渡した既定", () => {
    const slots = slotsFromMembers([], NATURE);
    expect(slots).toHaveLength(TEAM_SLOT_COUNT);
    for (const slot of slots) {
      expect(isEmptySlot(slot)).toBe(true);
      expect(slot.natureId).toBe(NATURE);
      expect(slot.moves.every((move) => move === null)).toBe(true);
    }
  });

  test("メンバー2体: 前の2枠に保存済みの内容(memberToDraft と同じ)、残り4枠は空", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    expect(slots).toHaveLength(TEAM_SLOT_COUNT);
    expect(slots[0]).toEqual(memberToDraft(FIRE));
    expect(slots[1]).toEqual(memberToDraft(ELECTRIC));
    expect(speciesKeys(slots)).toEqual(["9001-000", "9004-000", null, null, null, null]);
  });

  test("メンバー6体: 空の枠は無い", () => {
    const six = Array.from({ length: 6 }, () => FIRE);
    expect(slotsFromMembers(six, NATURE).some(isEmptySlot)).toBe(false);
  });
});

describe("S-2 slotsToMembers", () => {
  test("空の枠は飛ばし、種族の決まった枠だけを枠の順に詰める", () => {
    const base = slotsFromMembers([], NATURE);
    const slots = setSlot(setSlot(base, 4, memberToDraft(ELECTRIC)), 1, memberToDraft(FIRE));
    const result = slotsToMembers(slots);
    expect(result).toEqual({ ok: true, members: [FIRE, ELECTRIC] });
  });

  test("6つとも空なら members は空(保存できる)", () => {
    expect(slotsToMembers(slotsFromMembers([], NATURE))).toEqual({ ok: true, members: [] });
  });

  test("保存済みメンバーは無変更なら同じ値に戻る(ニックネーム・特性 null を落とさない)", () => {
    expect(slotsToMembers(slotsFromMembers([FIRE, ELECTRIC], NATURE))).toEqual({
      ok: true,
      members: [FIRE, ELECTRIC],
    });
  });

  test("不正な枠(SP 合計 67・SP 33)があれば ok: false で、その位置(1 始まり)を返す", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    const overTotal: MemberDraft = { ...at(slots, 0), sp: { ...at(slots, 0).sp, hp: "3" } };
    const overStat: MemberDraft = { ...at(slots, 1), sp: { ...at(slots, 1).sp, spa: "33" } };
    const result = slotsToMembers(setSlot(setSlot(slots, 0, overTotal), 1, overStat));
    expect(result).toEqual({ ok: false, invalidPositions: [1, 2] });
  });

  test("空の枠は不正の理由にならない(種族未選択のエラーにしない)", () => {
    const slots = slotsFromMembers([FIRE], NATURE);
    const result = slotsToMembers(slots);
    expect(result.ok).toBe(true);
  });
});

describe("S-3 swapSlots・clearSlot", () => {
  test("下へ: 隣の枠と入れ替わる。空の枠とも入れ替わる", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    expect(speciesKeys(swapSlots(slots, 0, 1))).toEqual(["9004-000", "9001-000", null, null, null, null]);
    expect(speciesKeys(swapSlots(slots, 1, 1))).toEqual(["9001-000", null, "9004-000", null, null, null]);
  });

  test("1体目の上・6体目の下は何も変えない(同じ並び)", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    expect(speciesKeys(swapSlots(slots, 0, -1))).toEqual(speciesKeys(slots));
    expect(speciesKeys(swapSlots(slots, 5, 1))).toEqual(speciesKeys(slots));
  });

  test("外す: その枠だけが空になり(既定の性格)、他の枠は位置も中身も変わらない", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    const cleared = clearSlot(slots, 0, NATURE);
    expect(cleared).toHaveLength(TEAM_SLOT_COUNT);
    expect(isEmptySlot(at(cleared, 0))).toBe(true);
    expect(at(cleared, 0).natureId).toBe(NATURE);
    expect(cleared[1]).toEqual(memberToDraft(ELECTRIC));
    expect(slotsToMembers(cleared)).toEqual({ ok: true, members: [ELECTRIC] });
  });

  test("元の配列は変えない(純粋関数)", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    const before = speciesKeys(slots);
    swapSlots(slots, 0, 1);
    clearSlot(slots, 1, NATURE);
    expect(speciesKeys(slots)).toEqual(before);
  });
});

describe("S-4 hasUnsavedChanges", () => {
  test("開いた直後(保存済みと同じ)は false", () => {
    expect(hasUnsavedChanges([FIRE, ELECTRIC], slotsFromMembers([FIRE, ELECTRIC], NATURE))).toBe(false);
    expect(hasUnsavedChanges([], slotsFromMembers([], NATURE))).toBe(false);
  });

  test("性格を変える・枠を入れ替える・外す・空の枠に種族を入れると true", () => {
    const slots = slotsFromMembers([FIRE, ELECTRIC], NATURE);
    expect(
      hasUnsavedChanges([FIRE, ELECTRIC], setSlot(slots, 0, { ...at(slots, 0), natureId: NATURE })),
    ).toBe(true);
    expect(hasUnsavedChanges([FIRE, ELECTRIC], swapSlots(slots, 0, 1))).toBe(true);
    expect(hasUnsavedChanges([FIRE, ELECTRIC], clearSlot(slots, 1, NATURE))).toBe(true);
    expect(
      hasUnsavedChanges([FIRE, ELECTRIC], setSlot(slots, 3, { ...at(slots, 3), speciesKey: "9002-000" })),
    ).toBe(true);
  });

  test("空の枠を空の枠と入れ替えても、保存する members が同じなら false", () => {
    const slots = slotsFromMembers([FIRE], NATURE);
    expect(hasUnsavedChanges([FIRE], swapSlots(slots, 3, 1))).toBe(false);
  });

  test("不正な下書き(保存できない)は true", () => {
    const slots = slotsFromMembers([FIRE], NATURE);
    const invalid = setSlot(slots, 0, { ...at(slots, 0), sp: { ...at(slots, 0).sp, hp: "1e" } });
    expect(hasUnsavedChanges([FIRE], invalid)).toBe(true);
  });
});

describe("S-5 teamInputFromMembers", () => {
  test("body は members だけで、name キーを持たない(空文字・null・既定名も入れない。ADR-0229)", () => {
    const input = teamInputFromMembers([FIRE]);
    expect(Object.keys(input)).toEqual(["members"]);
    expect("name" in input).toBe(false);
    expect(input.members).toEqual([FIRE]);
  });

  test("空の構築も members: [] だけ", () => {
    expect(teamInputFromMembers([])).toStrictEqual({ members: [] });
  });
});
