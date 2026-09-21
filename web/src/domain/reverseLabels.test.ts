// P4-4: 逆算の結果の表示の言葉(ADR-0016 §7、ADR-0010 §R3「目安の名前は表示層が付ける」)。
// SP の範囲は ranges をそのまま全部出す(1区間に畳まない)。目安の名前は、範囲に SP 0 / 32 が入っているとき、
// 性格クラスとの組で併記する:
//   防御側(H32 前提): 0+補正なし = H振り、0+上昇 = H振り+B(D)補正、32+補正なし = HB(HD)振り、32+上昇 = HB(HD)特化
//   攻撃側: 0+補正なし = 無振り、0+上昇 = A(C)補正のみ、32+補正なし = A(C)振り、32+上昇 = A(C)特化
// engine の NatureClass は "neutral" / "plus"(engine/reverse.go。下降補正は探索しない。ADR-0010 §R1)。

import { describe, expect, test } from "vitest";
import type { Item, ReverseResult, SPRange } from "../engine/types";
import {
  formatSPRanges,
  natureClassLabel,
  reverseAssumptionNote,
  reverseGuideNames,
  reverseItemLabel,
} from "./reverseLabels";

describe("formatSPRanges", () => {
  test.each([
    ["def", [{ min: 8, max: 8 }], "B 8"],
    ["def", [{ min: 4, max: 7 }], "B 4〜7"],
    [
      "def",
      [
        { min: 4, max: 7 },
        { min: 9, max: 12 },
      ],
      "B 4〜7, 9〜12",
    ],
    [
      "spd",
      [
        { min: 0, max: 0 },
        { min: 2, max: 5 },
        { min: 30, max: 32 },
      ],
      "D 0, 2〜5, 30〜32",
    ],
    ["atk", [{ min: 0, max: 32 }], "A 0〜32"],
    ["spa", [{ min: 20, max: 23 }], "C 20〜23"],
  ] as const)("%s %j → %s", (stat, ranges, expected) => {
    expect(formatSPRanges(stat, ranges)).toBe(expected);
  });
});

describe("natureClassLabel", () => {
  test.each([
    ["neutral", "def", "補正なし"],
    ["plus", "def", "B上昇"],
    ["plus", "spd", "D上昇"],
    ["plus", "atk", "A上昇"],
    ["plus", "spa", "C上昇"],
    ["neutral", "spa", "補正なし"],
  ] as const)("%s × %s → %s", (natureClass, stat, expected) => {
    expect(natureClassLabel(natureClass, stat)).toBe(expected);
  });
});

describe("reverseGuideNames", () => {
  const zero: SPRange[] = [{ min: 0, max: 3 }];
  const top: SPRange[] = [{ min: 30, max: 32 }];
  const middle: SPRange[] = [{ min: 4, max: 20 }];
  const both: SPRange[] = [
    { min: 0, max: 2 },
    { min: 32, max: 32 },
  ];

  test.each([
    ["defender", "def", "neutral", zero, ["H振り"]],
    ["defender", "def", "plus", zero, ["H振り+B補正"]],
    ["defender", "def", "neutral", top, ["HB振り"]],
    ["defender", "def", "plus", top, ["HB特化"]],
    ["defender", "spd", "neutral", zero, ["H振り"]],
    ["defender", "spd", "plus", zero, ["H振り+D補正"]],
    ["defender", "spd", "neutral", top, ["HD振り"]],
    ["defender", "spd", "plus", top, ["HD特化"]],
    ["defender", "def", "neutral", both, ["H振り", "HB振り"]],
    ["defender", "def", "neutral", middle, []],
    ["attacker", "atk", "neutral", zero, ["無振り"]],
    ["attacker", "atk", "plus", zero, ["A補正のみ"]],
    ["attacker", "atk", "neutral", top, ["A振り"]],
    ["attacker", "atk", "plus", top, ["A特化"]],
    ["attacker", "spa", "neutral", zero, ["無振り"]],
    ["attacker", "spa", "plus", zero, ["C補正のみ"]],
    ["attacker", "spa", "neutral", top, ["C振り"]],
    ["attacker", "spa", "plus", both, ["C補正のみ", "C特化"]],
    ["attacker", "spa", "plus", middle, []],
  ] as const)("%s %s %s %j → %j", (side, stat, natureClass, ranges, expected) => {
    expect(reverseGuideNames(side, stat, natureClass, ranges)).toEqual(expected);
  });

  test("0 / 32 が区間の端でなく内側にあっても含むとみなす", () => {
    expect(reverseGuideNames("defender", "def", "neutral", [{ min: 0, max: 32 }])).toEqual([
      "H振り",
      "HB振り",
    ]);
  });
});

describe("reverseItemLabel", () => {
  const items: Item[] = [{ id: "berry", nameJa: "テストきのみ", effect: null }];

  test("空の itemId は「持ち物なし」", () => {
    expect(reverseItemLabel("", items)).toBe("持ち物なし");
  });

  test("マスタにある ID は名前、無い ID は ID のまま(未知データで画面を壊さない)", () => {
    expect(reverseItemLabel("berry", items)).toBe("テストきのみ");
    expect(reverseItemLabel("unknown-id", items)).toBe("unknown-id");
  });
});

describe("reverseAssumptionNote", () => {
  const base: ReverseResult = {
    side: "defender",
    stat: "def",
    assumedHpSp: 32,
    exactCount: 0,
    candidates: [],
  };

  test("防御側は assumedHpSp から「H32 を仮定」を出す", () => {
    expect(reverseAssumptionNote(base)).toMatch(/H32 を仮定/);
    expect(reverseAssumptionNote({ ...base, assumedHpSp: 20 })).toMatch(/H20 を仮定/);
  });

  test("攻撃側は仮定を出さない(null)", () => {
    expect(reverseAssumptionNote({ ...base, side: "attacker", stat: "atk", assumedHpSp: 0 })).toBeNull();
  });
});
