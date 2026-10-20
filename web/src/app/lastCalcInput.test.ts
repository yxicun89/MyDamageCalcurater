// ADR-0358 段 2: 計算画面の直近の入力を localStorage に 1 件保存する(端末の中だけ)。
// calcMode.ts と同じ流儀: ストレージが無い・例外・壊れた値でも失敗させず、無いものとして扱う。

import { afterEach, beforeEach, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import {
  LAST_CALC_INPUT_SCHEMA_VERSION,
  LAST_CALC_INPUT_STORAGE_KEY,
  favoriteOfLastCalcInput,
  loadLastCalcInput,
  saveLastCalcInput,
} from "./lastCalcInput";
import type { StorageLike } from "../api/storage";

type FavoriteInput = components["schemas"]["FavoriteInput"];

const throwingStorage: StorageLike = {
  getItem() {
    throw new DOMException("denied", "SecurityError");
  },
  setItem() {
    throw new DOMException("denied", "QuotaExceededError");
  },
};

const attackerOnly: FavoriteInput = {
  label: "あ",
  individual: {
    speciesKey: "0001-000",
    level: 50,
    natureId: "adamant",
    sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
  },
};

const withCalc = {
  label: "あ → い",
  individual: attackerOnly.individual,
  calc: {
    attacker: attackerOnly.individual,
    defender: { ...attackerOnly.individual, speciesKey: "0002-000" },
    move: { id: "tackle" },
  },
} as unknown as FavoriteInput;

beforeEach(() => {
  localStorage.clear();
});
afterEach(() => {
  localStorage.clear();
});

describe("直近の計算入力の保存", () => {
  test("保存先のキーと版", () => {
    expect(LAST_CALC_INPUT_STORAGE_KEY).toBe("pokecalc.lastCalcInput");
    expect(LAST_CALC_INPUT_SCHEMA_VERSION).toBe(1);
  });

  test("何も保存していなければ null", () => {
    expect(loadLastCalcInput()).toBeNull();
  });

  test.each([attackerOnly, withCalc])("保存した入力をそのまま読み戻せる(1 件だけ。上書き)", (input) => {
    saveLastCalcInput(attackerOnly);
    saveLastCalcInput(input);
    expect(loadLastCalcInput()).toEqual(input);
  });

  test("保存値に版が付く", () => {
    saveLastCalcInput(attackerOnly);
    const stored = JSON.parse(localStorage.getItem(LAST_CALC_INPUT_STORAGE_KEY) ?? "null") as {
      schemaVersion: number;
    };
    expect(stored.schemaVersion).toBe(LAST_CALC_INPUT_SCHEMA_VERSION);
  });

  test.each([
    "",
    "not json",
    "null",
    "[]",
    "42",
    JSON.stringify({ schemaVersion: 999, input: attackerOnly }),
    JSON.stringify({ schemaVersion: 1 }),
    JSON.stringify({ schemaVersion: 1, input: { label: "x" } }),
    JSON.stringify({ schemaVersion: 1, input: { label: "x", individual: { speciesKey: 1 } } }),
    JSON.stringify({
      schemaVersion: 1,
      input: { label: "x", individual: attackerOnly.individual, calc: { attacker: {} } },
    }),
  ])("壊れた・版違いの保存値 %j は無いものとして扱う", (stored) => {
    localStorage.setItem(LAST_CALC_INPUT_STORAGE_KEY, stored);
    expect(loadLastCalcInput()).toBeNull();
  });

  test("ストレージが例外を投げても、読むときは null、書くときは例外にしない", () => {
    expect(loadLastCalcInput(throwingStorage)).toBeNull();
    expect(() => {
      saveLastCalcInput(attackerOnly, throwingStorage);
    }).not.toThrow();
  });

  test("ストレージが無い(null)ときも失敗しない", () => {
    expect(loadLastCalcInput(null)).toBeNull();
    expect(() => {
      saveLastCalcInput(attackerOnly, null);
    }).not.toThrow();
  });

  test("お気に入りの復元に渡せる形(Favorite)にする", () => {
    const favorite = favoriteOfLastCalcInput(withCalc);
    expect(favorite.label).toBe("あ → い");
    expect(favorite.individual).toEqual(withCalc.individual);
    expect(favorite.calc).toEqual(withCalc.calc);
  });
});
