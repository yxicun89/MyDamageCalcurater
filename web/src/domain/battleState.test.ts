// ADR-0144 §3(I-web-13): 対戦の状態の入力 → battleState の純粋関数。
import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import {
  DEFAULT_BATTLE_STATE_INPUTS,
  exceedsMax,
  battleStateOfRow,
  defenderCurrentHpOfRow,
  hitsOptions,
  percentOfSavedHp,
  savedBattleState,
  inputsOfBattleState,
  multiHitRangeOf,
  parseHpInput,
  percentText,
  resolveBattleState,
} from "./battleState";

function move(multiHit?: { min: number; max: number } | null): Move {
  return {
    id: "m",
    nameJa: "技",
    type: "normal",
    category: "physical",
    power: 25,
    priority: 0,
    ...(multiHit === undefined ? {} : { mechanismParams: { multiHit } }),
  };
}

const limits = { attackerMaxHp: 150, defenderPresent: true };

describe("parseHpInput", () => {
  test.each([
    ["", { kind: "empty" }],
    ["  ", { kind: "empty" }],
    ["1", { kind: "ok", value: 1 }],
    ["150", { kind: "ok", value: 150 }],
    ["0", { kind: "error" }],
    ["151", { kind: "error" }],
    ["-3", { kind: "error" }],
    ["1.5", { kind: "error" }],
    ["abc", { kind: "error" }],
  ])("%j", (text, expected) => {
    expect(parseHpInput(text, 150)).toEqual(expected);
  });
});

describe("multiHitRangeOf", () => {
  test("min < max の多段だけ範囲を返す", () => {
    expect(multiHitRangeOf(move({ min: 2, max: 5 }))).toEqual({ min: 2, max: 5 });
    expect(multiHitRangeOf(move({ min: 2, max: 2 }))).toBeNull();
    expect(multiHitRangeOf(move(null))).toBeNull();
    expect(multiHitRangeOf(move())).toBeNull();
    expect(multiHitRangeOf(null)).toBeNull();
  });
  test("回数の選択肢は min..max", () => {
    expect(hitsOptions({ min: 2, max: 5 })).toEqual([2, 3, 4, 5]);
  });
});

describe("resolveBattleState", () => {
  test("既定・空・満タン(攻撃側は最大、防御側は100%)・既定の回数は何も入れない", () => {
    expect(
      resolveBattleState(DEFAULT_BATTLE_STATE_INPUTS, limits, move({ min: 2, max: 5 })).state,
    ).toBeUndefined();
    const full = { attackerHp: "150", defenderHp: "100", hits: null };
    expect(resolveBattleState(full, limits, null)).toEqual({
      state: undefined,
      attackerError: false,
      defenderError: false,
    });
  });
  test("指定した値だけ入る(防御側は割合のまま)", () => {
    const inputs = { attackerHp: "70", defenderHp: "40", hits: 3 };
    expect(resolveBattleState(inputs, limits, move({ min: 2, max: 5 })).state).toEqual({
      attackerCurrentHp: 70,
      defenderPercent: 40,
      hits: 3,
    });
  });
  test("範囲外は誤りで、その値は入れない(防御側は 1..100 の整数)", () => {
    for (const text of ["0", "101", "-1", "12.5"]) {
      const r = resolveBattleState({ attackerHp: "0", defenderHp: text, hits: null }, limits, null);
      expect(r).toEqual({ state: undefined, attackerError: true, defenderError: true });
    }
  });
  test("範囲の多段でない技・範囲外の回数は入れない", () => {
    const base = DEFAULT_BATTLE_STATE_INPUTS;
    expect(resolveBattleState({ ...base, hits: 3 }, limits, move({ min: 2, max: 2 })).state).toBeUndefined();
    expect(resolveBattleState({ ...base, hits: 9 }, limits, move({ min: 2, max: 5 })).state).toBeUndefined();
  });
  test("最大 HP が分からない側・防御側がいないときは検証も送信もしない", () => {
    const r = resolveBattleState(
      { attackerHp: "999", defenderHp: "999", hits: null },
      { attackerMaxHp: null, defenderPresent: false },
      null,
    );
    expect(r).toEqual({ state: undefined, attackerError: false, defenderError: false });
  });
});

describe("割合(%)の換算 max(1, floor(最大 × % / 100))", () => {
  test.each([
    [1, 175, 1],
    [1, 207, 2],
    [99, 175, 173],
    [99, 207, 204],
    [50, 175, 87],
    [1, 50, 1],
    [1, 10, 1],
  ])("%i%% × 最大 %i → %i", (percent, max, expected) => {
    expect(defenderCurrentHpOfRow(percent, max)).toBe(expected);
  });
  test("100% と、換算結果が最大以上になる行は満タン(null)", () => {
    expect(defenderCurrentHpOfRow(100, 175)).toBeNull();
    expect(defenderCurrentHpOfRow(99, 1)).toBeNull();
    expect(defenderCurrentHpOfRow(1, 1)).toBeNull();
  });
  test("H振り行と無振り行で同じ割合になる(実数値は行ごとに違う)", () => {
    const state = { defenderPercent: 50 };
    expect(battleStateOfRow(state, 175)).toEqual({ defenderCurrentHp: 87 });
    expect(battleStateOfRow(state, 207)).toEqual({ defenderCurrentHp: 103 });
  });
  test("行に付けるものが無ければ undefined。攻撃側・回数は全行に付く", () => {
    expect(battleStateOfRow({ defenderPercent: 1 }, 1)).toBeUndefined();
    expect(battleStateOfRow({ attackerCurrentHp: 5, defenderPercent: 1, hits: 3 }, 1)).toEqual({
      attackerCurrentHp: 5,
      hits: 3,
    });
  });
});

describe("お気に入り・履歴の保存形(実数値)と復元", () => {
  test("保存は無振りの最大 HP の行に換算した実数値", () => {
    expect(savedBattleState({ defenderPercent: 50, hits: 3 }, 175)).toEqual({
      defenderCurrentHp: 87,
      hits: 3,
    });
  });
  test("無振りの最大 HP が分からないときは防御側を保存しない", () => {
    expect(savedBattleState({ defenderPercent: 50, attackerCurrentHp: 9 }, null)).toEqual({
      attackerCurrentHp: 9,
    });
    expect(savedBattleState({ defenderPercent: 50 }, null)).toBeUndefined();
  });
  test("復元は、その実数値に届く最小の整数%(行が特定できないので近似)。往復して保存値以上になる(99% が上限なので最大-1 だけは 173 になる)", () => {
    expect(percentOfSavedHp(87, 175)).toBe("50");
    expect(percentOfSavedHp(1, 175)).toBe("1");
    expect(percentOfSavedHp(174, 175)).toBe("99");
    for (const hp of [1, 2, 50, 87, 103, 173]) {
      const percent = Number(percentOfSavedHp(hp, 175));
      expect(defenderCurrentHpOfRow(percent, 175) ?? 175).toBeGreaterThanOrEqual(hp);
    }
  });
  test("最大以上・最大が分からない保存値は満タン(空)", () => {
    expect(percentOfSavedHp(175, 175)).toBe("");
    expect(percentOfSavedHp(300, 175)).toBe("");
    expect(percentOfSavedHp(50, null)).toBe("");
  });
});

describe("そのほか", () => {
  test("割合は小数第1位で切り捨て", () => {
    expect(percentText(1, 3)).toBe("33.3%");
    expect(percentText(2, 3)).toBe("66.6%");
    expect(percentText(100, 100)).toBe("100.0%");
  });
  test("最大を超えているか", () => {
    expect(exceedsMax("151", 150)).toBe(true);
    expect(exceedsMax("150", 150)).toBe(false);
    expect(exceedsMax("", 150)).toBe(false);
    expect(exceedsMax("x", 150)).toBe(false);
  });
  test("保存された battleState を入力に戻す", () => {
    expect(inputsOfBattleState(undefined, 175)).toEqual(DEFAULT_BATTLE_STATE_INPUTS);
    expect(inputsOfBattleState({ attackerCurrentHp: 10, defenderCurrentHp: 87, hits: 4 }, 175)).toEqual({
      attackerHp: "10",
      defenderHp: "50",
      hits: 4,
    });
  });
});
