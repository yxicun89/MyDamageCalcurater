// P5-5b PR-A2(ADR-0316): メンバー編集の純粋関数。画面なしで境界を固定する。
//   AC-U1 SP: 0 / 32 は通り 33 / -1 / 小数 / 文字は通らない・空欄は 0・合計 66 は通り 67 は通らない
//   AC-U2 メンバー ⇔ 下書き: 往復で値が変わらない(ニックネームを落とさない)・技は4枠・null を詰めて送る
//   AC-U3 保存できない理由: 種族未選択・技の重複・SP の問題(複数同時に返す)
//   AC-U4 種族変更: 特性は持てば保ち持たなければ選び直す・技は learnset にあるものだけ残す
// 架空のマスタ(例データ)だけを使う。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL, selectableAbilities } from "../domain/requests";
import type { StatKey } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  MAX_MEMBER_MOVES,
  SP_STATS,
  blankDraft,
  changeSpecies,
  checkSpDraft,
  draftToMember,
  memberToDraft,
  type MemberDraft,
} from "./teamMember";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function species(key: string): MasterSpecies {
  const found = master.species.find((candidate) => candidate.key === key);
  if (found === undefined) {
    throw new Error(`例データに ${key} が無い`);
  }
  return found;
}

const ZERO: Record<StatKey, string> = { hp: "0", atk: "0", def: "0", spa: "0", spd: "0", spe: "0" };

function sp(overrides: Partial<Record<StatKey, string>>): Record<StatKey, string> {
  return { ...ZERO, ...overrides };
}

describe("AC-U1 SP の検査(checkSpDraft)", () => {
  test("定数は契約どおり(各32・合計66・技4)", () => {
    expect(MAX_SP_PER_STAT).toBe(32);
    expect(MAX_SP_TOTAL).toBe(66);
    expect(MAX_MEMBER_MOVES).toBe(4);
    expect(SP_STATS).toEqual(["hp", "atk", "def", "spa", "spd", "spe"]);
  });

  test("全部 0 は通り、残りは 66", () => {
    const result = checkSpDraft(ZERO);
    expect(result).toEqual({
      sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      total: 0,
      remaining: 66,
      issues: [],
    });
  });

  test("1欄 32 は通る(境界ちょうど)", () => {
    const result = checkSpDraft(sp({ atk: "32" }));
    expect(result.issues).toEqual([]);
    expect(result.sp?.atk).toBe(32);
  });

  test.each(["33", "-1", "1.5", "abc", "1e1", "０"])("1欄が %s は、その欄の範囲外として拒否する", (bad) => {
    const result = checkSpDraft(sp({ def: bad }));
    expect(result.sp).toBeNull();
    expect(result.issues).toContainEqual({ kind: "stat", stat: "def" });
  });

  test("空欄は 0 として扱う(入力し直す途中で拒否しない)", () => {
    const result = checkSpDraft(sp({ spe: "" }));
    expect(result.issues).toEqual([]);
    expect(result.sp?.spe).toBe(0);
  });

  test("合計ちょうど 66 は通り、残りは 0(32 + 32 + 2)", () => {
    const result = checkSpDraft(sp({ atk: "32", spe: "32", hp: "2" }));
    expect(result.issues).toEqual([]);
    expect(result.total).toBe(66);
    expect(result.remaining).toBe(0);
    expect(result.sp).toEqual({ hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 });
  });

  test("合計 67 は拒否し、超過分(残りが負)を返す", () => {
    const result = checkSpDraft(sp({ atk: "32", spe: "32", hp: "3" }));
    expect(result.sp).toBeNull();
    expect(result.total).toBe(67);
    expect(result.remaining).toBe(-1);
    expect(result.issues).toEqual([{ kind: "total", total: 67 }]);
  });

  test("欄の範囲外と合計超過が同時なら両方を返す", () => {
    const result = checkSpDraft(sp({ hp: "33", atk: "32", def: "32" }));
    expect(result.issues).toContainEqual({ kind: "stat", stat: "hp" });
    expect(result.issues).toContainEqual({ kind: "total", total: 97 });
  });
});

const MEMBER: Schemas["TeamMember"] = {
  speciesKey: "9001-000",
  nickname: "テスト愛称",
  moveIds: ["examplemovetackle", "examplemovefirepunch"],
  itemId: "exampleitemdef",
  abilityId: "exampleabilitynone",
  natureId: "example-nature-atk",
  sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  teraType: "fire",
};

describe("AC-U2 メンバー ⇔ 下書き", () => {
  test("技は MAX_MEMBER_MOVES 枠に詰め、SP は文字列、他の項目は持ち回す", () => {
    const draft = memberToDraft(MEMBER);
    expect(draft.moves).toEqual(["examplemovetackle", "examplemovefirepunch", null, null]);
    expect(draft.moves).toHaveLength(MAX_MEMBER_MOVES);
    expect(draft.sp).toEqual({ hp: "2", atk: "32", def: "0", spa: "0", spd: "0", spe: "32" });
    expect(draft.nickname).toBe("テスト愛称");
    expect(draft.itemId).toBe("exampleitemdef");
    expect(draft.teraType).toBe("fire");
  });

  test("往復しても値が変わらない(ニックネームを落とさない)", () => {
    expect(draftToMember(memberToDraft(MEMBER))).toEqual({ ok: true, member: MEMBER });
  });

  test("持ち物・特性・テラスタイプ・ニックネームが未設定(null / 省略)でも往復できる", () => {
    const bare: Schemas["TeamMember"] = {
      speciesKey: "9002-000",
      moveIds: [],
      natureId: "example-nature-neutral-hardy",
      sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
    };
    const result = draftToMember(memberToDraft(bare));
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.member.moveIds).toEqual([]);
      expect(result.member.itemId ?? null).toBeNull();
      expect(result.member.abilityId ?? null).toBeNull();
      expect(result.member.teraType ?? null).toBeNull();
    }
  });

  test("技の途中の空き枠は詰めて送る(順序は保つ)", () => {
    const draft: MemberDraft = {
      ...memberToDraft(MEMBER),
      moves: [null, "examplemovefirepunch", null, "examplemovegrowl"],
    };
    const result = draftToMember(draft);
    expect(result.ok && result.member.moveIds).toEqual(["examplemovefirepunch", "examplemovegrowl"]);
  });

  test("新しい下書きは種族なし・技なし・SP すべて 0 で、指定の性格を持つ", () => {
    const draft = blankDraft("example-nature-neutral-hardy");
    expect(draft.speciesKey).toBeNull();
    expect(draft.moves).toEqual([null, null, null, null]);
    expect(draft.natureId).toBe("example-nature-neutral-hardy");
    expect(draft.sp).toEqual(ZERO);
    expect(draft.itemId).toBeNull();
    expect(draft.abilityId).toBeNull();
    expect(draft.teraType).toBeNull();
  });
});

describe("AC-U3 保存できない理由(draftToMember)", () => {
  test("種族が未選択", () => {
    const result = draftToMember(blankDraft("example-nature-neutral-hardy"));
    expect(result).toEqual({ ok: false, issues: [{ kind: "speciesRequired" }] });
  });

  test("同じ技を2枠に選んでいる", () => {
    const draft: MemberDraft = {
      ...memberToDraft(MEMBER),
      moves: ["examplemovetackle", "examplemovetackle", null, null],
    };
    const result = draftToMember(draft);
    expect(result).toEqual({ ok: false, issues: [{ kind: "moveDuplicate", moveId: "examplemovetackle" }] });
  });

  test("SP の問題と種族未選択を同時に返す(1つずつ直させない)", () => {
    const draft: MemberDraft = { ...blankDraft("example-nature-neutral-hardy"), sp: sp({ hp: "33" }) };
    const result = draftToMember(draft);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.issues).toContainEqual({ kind: "speciesRequired" });
      expect(result.issues).toContainEqual({ kind: "sp", issue: { kind: "stat", stat: "hp" } });
    }
  });
});

describe("AC-U4 種族変更(changeSpecies)", () => {
  const fire = (): MasterSpecies => species("9001-000");
  const water = (): MasterSpecies => species("9002-000");
  const electric = (): MasterSpecies => species("9004-000");

  test("特性: 新しい種族が持たなければ先頭に選び直す", () => {
    const base: MemberDraft = {
      ...memberToDraft(MEMBER),
      speciesKey: "9004-000",
      abilityId: "exampleabilityadapt",
    };
    const changed = changeSpecies(base, fire(), selectableAbilities(fire(), master.abilities));
    expect(changed.speciesKey).toBe("9001-000");
    expect(changed.abilityId).toBe(fire().abilities[0]);
  });

  test("特性: 新しい種族も持っていればそのまま保つ", () => {
    const base: MemberDraft = {
      ...memberToDraft(MEMBER),
      speciesKey: "9001-000",
      abilityId: "exampleabilitynone",
    };
    const changed = changeSpecies(base, electric(), selectableAbilities(electric(), master.abilities));
    expect(changed.abilityId).toBe("exampleabilitynone");
  });

  test("特性: 選択肢が空(特性の無いマスタ)なら null", () => {
    const changed = changeSpecies(memberToDraft(MEMBER), fire(), []);
    expect(changed.abilityId).toBeNull();
  });

  test("技: 新しい種族の learnset にあるものだけ残し、前に詰める", () => {
    // 9001 の技 [たいあたり, かえんパンチ, なきごえ] → 9002 の learnset は [みずでっぽう, なきごえ]
    const base: MemberDraft = {
      ...memberToDraft(MEMBER),
      moves: ["examplemovetackle", "examplemovefirepunch", "examplemovegrowl", null],
    };
    const changed = changeSpecies(base, water(), selectableAbilities(water(), master.abilities));
    expect(changed.moves).toEqual(["examplemovegrowl", null, null, null]);
  });

  test("持ち物・性格・SP・テラスタイプ・ニックネームは変えない", () => {
    const base = memberToDraft(MEMBER);
    const changed = changeSpecies(base, water(), selectableAbilities(water(), master.abilities));
    expect(changed).toMatchObject({
      itemId: base.itemId,
      natureId: base.natureId,
      sp: base.sp,
      teraType: base.teraType,
      nickname: base.nickname,
    });
  });
});
