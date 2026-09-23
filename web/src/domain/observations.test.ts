// P4-4: 逆算の観測の入力(ADR-0300 §7、ADR-0010 §R2)。観測は整数%(1〜100)か、HP の実点数(1 以上の整数)。
// 小数の表示%は観測に使わない(ADR-0300 §7)ので、画面の入力は整数だけを受け付け、engine に渡す前に弾く。
// engine に渡す Observation は percent / damage のちょうど1つを持つ(percentTenths は画面から使わない)。

import { describe, expect, test } from "vitest";
import { canAddObservation, defaultObservationUnit, parseObservation } from "./observations";

describe("parseObservation(%)", () => {
  test.each([
    ["1", 1],
    ["45", 45],
    ["100", 100],
    [" 45 ", 45],
  ])("%j は percent %d の観測", (text, percent) => {
    expect(parseObservation("percent", text)).toEqual({ status: "valid", observation: { percent } });
  });

  test.each(["0", "101", "12.5", "-3", "1e2", "abc", "4５", "0x10"])("%j は不正", (text) => {
    expect(parseObservation("percent", text)).toEqual({ status: "invalid" });
  });

  test.each(["", "   "])("%j(空)は empty で、不正扱いにしない", (text) => {
    expect(parseObservation("percent", text)).toEqual({ status: "empty" });
  });
});

describe("parseObservation(HP)", () => {
  test.each([
    ["1", 1],
    ["60", 60],
    ["250", 250],
  ])("%j は damage %d の観測", (text, damage) => {
    expect(parseObservation("damage", text)).toEqual({ status: "valid", observation: { damage } });
  });

  test.each(["0", "1.5", "-3", "abc"])("%j は不正", (text) => {
    expect(parseObservation("damage", text)).toEqual({ status: "invalid" });
  });

  test("空は empty", () => {
    expect(parseObservation("damage", "")).toEqual({ status: "empty" });
  });
});

describe("defaultObservationUnit", () => {
  test("与えたダメージ(defender)は%、受けたダメージ(attacker)は HP の実点数", () => {
    expect(defaultObservationUnit("defender")).toBe("percent");
    expect(defaultObservationUnit("attacker")).toBe("damage");
  });
});

// P4-19(issue #110、ADR-0208): 観測は 16 件まで(api/openapi.yaml の ReverseRequest.observations の maxItems)。
// 期待値の 16 は契約から直接書く(実装の写しにしない)。定数とのずれは requestLimits.test.ts が検出する。
describe("canAddObservation(観測の件数上限)", () => {
  test.each([
    [0, true],
    [1, true],
    [15, true],
    [16, false],
    [17, false],
  ])("いま %d 件のとき、追加できるか = %s", (currentCount, expected) => {
    expect(canAddObservation(currentCount)).toBe(expected);
  });
});
