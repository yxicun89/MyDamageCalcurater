// ADR-0328 §1: status-move の安全網。UI からは変化技を選べないので、画面の分岐が使う判定をここで固定する
// (計算・逆算の画面は isStatusMove が true のとき要求を送らず status-move の案内にする)。

import { expect, test } from "vitest";
import type { Move } from "../engine/types";
import { isDamagingMove, isStatusMove } from "./moves";

const base: Move = {
  id: "example",
  nameJa: "テスト",
  type: "normal",
  category: "physical",
  power: 50,
  priority: 0,
};

test.each([
  ["status", true],
  ["physical", false],
  ["special", false],
] as const)("isStatusMove(%s) = %s、isDamagingMove はその反対", (category, expected) => {
  const move = { ...base, category };
  expect(isStatusMove(move)).toBe(expected);
  expect(isDamagingMove(move)).toBe(!expected);
});
