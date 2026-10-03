// 判定の画面の送信前の検査(judgeValidation.ts)の境界値。契約の範囲(SP 0..32・合計 66・ランク -6..+6)を固定する。

import { describe, expect, test } from "vitest";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import { judgeScreenText } from "../i18n/ja";
import { emptyIndividual, type IndividualFormState, type RankKey } from "./individualForm";
import { type BodyErrors, validateBodies } from "./judgeValidation";

/** 必須の欄を埋めた1体(SP 0・ランク 0)。 */
function valid(overrides: Partial<IndividualFormState> = {}): IndividualFormState {
  return {
    ...emptyIndividual(),
    speciesKey: "9001-000",
    natureId: "test-nature",
    moveId: "test-move",
    ...overrides,
  };
}

function check(state: IndividualFormState): BodyErrors | undefined {
  return validateBodies([{ key: "a", who: "自分", state }]).errors.get("a");
}

describe("SP の範囲", () => {
  test.each([0, MAX_SP_PER_STAT])("%i は通る", (value) => {
    expect(check(valid({ sp: { ...emptyIndividual().sp, atk: value } }))).toBeUndefined();
  });

  test.each([-1, MAX_SP_PER_STAT + 1])("%i は止まり、その欄だけが誤りになる", (value) => {
    const errors = check(valid({ sp: { ...emptyIndividual().sp, atk: value } }));
    expect(errors?.spRange).toContain(judgeScreenText.spRangeMessage(MAX_SP_PER_STAT));
    expect([...(errors?.spRangeStats ?? [])]).toEqual(["atk"]);
  });
});

describe("SP の合計", () => {
  test("合計 66 は通り、67 は止まる", () => {
    const at66 = { hp: 32, atk: 32, def: 2, spa: 0, spd: 0, spe: 0 };
    expect(MAX_SP_TOTAL).toBe(66);
    expect(check(valid({ sp: at66 }))).toBeUndefined();
    const errors = check(valid({ sp: { ...at66, def: 3 } }));
    expect(errors?.spTotal).toContain(judgeScreenText.spTotalMessage(MAX_SP_TOTAL));
    expect(errors?.spRange).toBeNull();
  });
});

describe("ランクの範囲", () => {
  test.each(["-6", "6", "0"])("%s は通る", (raw) => {
    expect(check(valid({ ranks: { ...emptyIndividual().ranks, spe: raw } }))).toBeUndefined();
  });

  test.each(["-7", "7", "", "x"])("%j は止まり、その欄だけが誤りになる", (raw) => {
    const errors = check(valid({ ranks: { ...emptyIndividual().ranks, spe: raw } }));
    expect(errors?.rankRange).toContain(judgeScreenText.rankRangeMessage);
    expect([...(errors?.rankStats ?? [])]).toEqual<RankKey[]>(["spe"]);
  });
});

describe("必須の欄と複数体", () => {
  test("種族・性格・技が空なら、それぞれの欄の誤りになる", () => {
    const errors = check(emptyIndividual());
    expect(errors?.species).not.toBeNull();
    expect(errors?.nature).not.toBeNull();
    expect(errors?.move).not.toBeNull();
  });

  test("複数の体の誤りが同時に出て、文言にどの体かが入る。全体メッセージは必須が最優先", () => {
    const result = validateBodies([
      { key: "a", who: "自分", state: valid({ sp: { ...emptyIndividual().sp, hp: 33 } }) },
      { key: "c", who: "相手候補1", state: valid({ ranks: { ...emptyIndividual().ranks, def: "7" } }) },
      { key: "d", who: "相手候補2", state: valid({ moveId: "" }) },
    ]);
    expect([...result.errors.keys()]).toEqual(["a", "c", "d"]);
    expect(result.errors.get("a")?.spRange).toContain("自分");
    expect(result.errors.get("c")?.rankRange).toContain("相手候補1");
    expect(result.firstMessage).toBe(judgeScreenText.requiredMessage);
  });

  test("全体メッセージの優先順位は 必須 → SP 範囲 → SP 合計 → ランク", () => {
    const sp = { ...emptyIndividual().sp };
    const rank = { ...emptyIndividual().ranks, atk: "9" };
    const message = (states: IndividualFormState[]) =>
      validateBodies(states.map((state, i) => ({ key: String(i), who: "x", state }))).firstMessage;
    expect(message([valid({ ranks: rank }), valid({ sp: { ...sp, hp: 33 } })])).toBe(
      judgeScreenText.spRangeMessage(MAX_SP_PER_STAT),
    );
    expect(
      message([valid({ ranks: rank }), valid({ sp: { hp: 32, atk: 32, def: 3, spa: 0, spd: 0, spe: 0 } })]),
    ).toBe(judgeScreenText.spTotalMessage(MAX_SP_TOTAL));
    expect(message([valid({ ranks: rank })])).toBe(judgeScreenText.rankRangeMessage);
    expect(message([valid()])).toBeNull();
  });
});
