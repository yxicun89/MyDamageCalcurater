// issue 271 / issue 270(Web レーン): 「未対応」の印(ADR-0123)の文言資源。
// 正は ADR-0123 §2 の表と api/openapi.yaml の UnsupportedMark(生成物は web/src/api/openapi.gen.ts)。
// 確かめること:
//   - target 5 種・reason 15 種すべてに日本語のラベルがあり、余分なキーが無い(契約との過不足)
//   - ラベルは空でなく、技術用語(engine・スキーマ・機構・enum の値そのもの)をそのまま出さない
//   - markLabel は target・ID(またはその表示名)・reason を全部含む
//     (issue の依頼「技・持ち物・特性のどれが原因か〈target・id〉と共に示す」)

import { describe, expect, test } from "vitest";
import type { UnsupportedMark, UnsupportedReason, UnsupportedTarget } from "../engine/types";
import { unsupportedText } from "./ja";

/**
 * 契約の enum の値(api/openapi.yaml の UnsupportedMark)。ここは手書きの複製だが、
 * 下の Exclude による網羅性チェックで DTO のユニオン(engine/types.ts)との漏れを型で検出する
 * (コーディング規約 §2「独立した検証」。TypeId と同じ作法)。
 */
const allTargets = [
  "move",
  "attacker_item",
  "attacker_ability",
  "defender_item",
  "defender_ability",
] as const satisfies readonly UnsupportedTarget[];

const allReasons = [
  "alt_defense_stat",
  "alt_offense_stat",
  "always_crit",
  "effectiveness_change",
  "field_specific",
  "fixed_damage",
  "ignore_defense_ranks",
  "move_specific",
  "multi_hit",
  "ohko",
  "priority_change",
  "type_change",
  "variable_power",
  "zero_power",
  "unsupported_effect",
] as const satisfies readonly UnsupportedReason[];

/** 上の一覧がユニオンを覆っていなければ、この型が never でなくなりコンパイルが落ちる。 */
type MissingTarget = Exclude<UnsupportedTarget, (typeof allTargets)[number]>;
type MissingReason = Exclude<UnsupportedReason, (typeof allReasons)[number]>;
const missingTargets: MissingTarget[] = [];
const missingReasons: MissingReason[] = [];

describe("未対応の印のラベル", () => {
  test("契約の target 5 種に過不足なくラベルがある", () => {
    expect(missingTargets).toEqual([]);
    expect(Object.keys(unsupportedText.target).sort()).toEqual([...allTargets].sort());
  });

  test("契約の reason 15 種に過不足なくラベルがある", () => {
    expect(missingReasons).toEqual([]);
    expect(Object.keys(unsupportedText.reason).sort()).toEqual([...allReasons].sort());
  });

  test("ラベルは空でなく、重複しない", () => {
    for (const labels of [Object.values(unsupportedText.target), Object.values(unsupportedText.reason)]) {
      expect(labels.every((label) => label.trim().length > 0)).toBe(true);
      expect(new Set(labels).size).toBe(labels.length);
    }
    expect(unsupportedText.badgeLabel.trim().length).toBeGreaterThan(0);
    expect(unsupportedText.notice.trim().length).toBeGreaterThan(0);
  });

  test("利用者向けの言葉にする(技術用語や enum の値そのものを出さない)", () => {
    const labels = [
      ...Object.values(unsupportedText.target),
      ...Object.values(unsupportedText.reason),
      unsupportedText.badgeLabel,
      unsupportedText.notice,
    ];
    for (const label of labels) {
      for (const jargon of ["engine", "スキーマ", "機構", "ADR", "_"]) {
        expect(label).not.toContain(jargon);
      }
    }
  });
});

describe("markLabel(印 1 件の文言)", () => {
  const moveMark: UnsupportedMark = {
    target: "move",
    reason: "multi_hit",
    id: "example-move-doublepunch",
  };

  test("target のラベル・表示名・reason のラベルを全部含む", () => {
    const label = unsupportedText.markLabel(moveMark, "テストれんぞくパンチ");
    expect(label).toContain(unsupportedText.target.move);
    expect(label).toContain("テストれんぞくパンチ");
    expect(label).toContain(unsupportedText.reason.multi_hit);
  });

  test("表示名が分からない(空文字)ときは ID をそのまま出す", () => {
    const label = unsupportedText.markLabel(moveMark, "");
    expect(label).toContain(moveMark.id);
    expect(label).toContain(unsupportedText.reason.multi_hit);
  });

  test.each([
    ["attacker_item", "unsupported_effect"],
    ["attacker_ability", "unsupported_effect"],
    ["defender_item", "unsupported_effect"],
    ["defender_ability", "unsupported_effect"],
  ] as const)("持ち物・特性の印(%s)も target と reason が分かる文言になる", (target, reason) => {
    const label = unsupportedText.markLabel({ target, reason, id: "example-item-x" }, "テストもちもの");
    expect(label).toContain(unsupportedText.target[target]);
    expect(label).toContain(unsupportedText.reason[reason]);
    expect(label).toContain("テストもちもの");
  });

  test("target・reason の全組み合わせで空でない文言になる", () => {
    for (const target of allTargets) {
      for (const reason of allReasons) {
        expect(unsupportedText.markLabel({ target, reason, id: "x" }, "").trim().length).toBeGreaterThan(0);
      }
    }
  });
});
