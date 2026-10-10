// ADR-0144 §3(I-web-13): 対戦の状態の入力 → battleState の純粋関数。
import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import {
  DEFAULT_BATTLE_STATE_INPUTS,
  exceedsMax,
  hitsOptions,
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

const limits = { attackerMaxHp: 150, defenderMaxHp: 175 };

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
  test("既定・空・満タン・既定の回数は何も入れない", () => {
    expect(
      resolveBattleState(DEFAULT_BATTLE_STATE_INPUTS, limits, move({ min: 2, max: 5 })).state,
    ).toBeUndefined();
    const full = { attackerHp: "150", defenderHp: "175", hits: null };
    const resolved = resolveBattleState(full, limits, null);
    expect(resolved).toEqual({ state: undefined, attackerError: false, defenderError: false });
  });
  test("指定した値だけ入る", () => {
    const inputs = { attackerHp: "70", defenderHp: "", hits: 3 };
    expect(resolveBattleState(inputs, limits, move({ min: 2, max: 5 })).state).toEqual({
      attackerCurrentHp: 70,
      hits: 3,
    });
  });
  test("範囲外は誤りで、その値は入れない", () => {
    const r = resolveBattleState({ attackerHp: "0", defenderHp: "176", hits: null }, limits, null);
    expect(r).toEqual({ state: undefined, attackerError: true, defenderError: true });
  });
  test("範囲の多段でない技・範囲外の回数は入れない", () => {
    expect(
      resolveBattleState({ ...DEFAULT_BATTLE_STATE_INPUTS, hits: 3 }, limits, move({ min: 2, max: 2 })).state,
    ).toBeUndefined();
    expect(
      resolveBattleState({ ...DEFAULT_BATTLE_STATE_INPUTS, hits: 9 }, limits, move({ min: 2, max: 5 })).state,
    ).toBeUndefined();
  });
  test("最大 HP が分からない側は検証も送信もしない", () => {
    const r = resolveBattleState(
      { attackerHp: "999", defenderHp: "10", hits: null },
      { attackerMaxHp: null, defenderMaxHp: null },
      null,
    );
    expect(r).toEqual({ state: undefined, attackerError: false, defenderError: false });
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
    expect(inputsOfBattleState(undefined)).toEqual(DEFAULT_BATTLE_STATE_INPUTS);
    expect(inputsOfBattleState({ attackerCurrentHp: 10, defenderCurrentHp: 20, hits: 4 })).toEqual({
      attackerHp: "10",
      defenderHp: "20",
      hits: 4,
    });
  });
});
