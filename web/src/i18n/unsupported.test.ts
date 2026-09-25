// issue 271 / issue 270(Web レーン): 「未対応」の印(ADR-0123)の文言資源。
// 文言・書式は iOS レーンの決定(docs/ai-shared/DECISIONS.md 2026-09-25「未対応の印の表示」、
// ADR-0501「P6-17」)に揃える。正は ADR-0123 §2 の表と api/openapi.yaml の UnsupportedMark(生成物は
// web/src/api/openapi.gen.ts)。
// 確かめること:
//   - target 5 種・reason 15 種すべてに日本語のラベルがあり、余分なキーが無い(契約との過不足)
//   - ラベルは空でなく、技術用語(engine・スキーマ・機構・enum の値そのもの)をそのまま出さない
//   - alt_offense_stat・alt_defense_stat・effectiveness_change の文言に「特殊」を使わない
//     (ダメージ計算の特殊技分類と紛れるため。iOS critic 指摘 2026-09-25)
//   - markLabel は `<対象>「<名前>」(<理由>)` の書式で、reason が unsupported_effect のときは
//     理由の括弧を省く(iOS レーンの書式と同じ)
//   - notice/rowLabel は markLabel 済みの文言を読点区切りで並べ、iOS レーンの書式(結果の上/行・候補カード)
//     と一致する

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
  });

  test("利用者向けの言葉にする(技術用語や enum の値そのものを出さない)", () => {
    const labels = [...Object.values(unsupportedText.target), ...Object.values(unsupportedText.reason)];
    for (const label of labels) {
      for (const jargon of ["engine", "スキーマ", "機構", "ADR", "_"]) {
        expect(label).not.toContain(jargon);
      }
    }
  });

  // iOS critic 指摘(2026-09-25): 「特殊」はダメージ計算の特殊技分類(物理/特殊)を指すため、
  // alt_offense_stat・alt_defense_stat・effectiveness_change の文言には使わない。
  test.each(["alt_offense_stat", "alt_defense_stat", "effectiveness_change"] as const)(
    "%s の文言に「特殊」を使わない",
    (reason) => {
      expect(unsupportedText.reason[reason]).not.toContain("特殊");
    },
  );
});

describe("markLabel(印 1 件の文言)", () => {
  const moveMark: UnsupportedMark = {
    target: "move",
    reason: "multi_hit",
    id: "example-move-doublepunch",
  };

  test("`<対象>「<名前>」(<理由>)` の書式になる(iOS レーンの書式)", () => {
    const label = unsupportedText.markLabel(moveMark, "テストれんぞくパンチ");
    expect(label).toBe(
      `${unsupportedText.target.move}「テストれんぞくパンチ」(${unsupportedText.reason.multi_hit})`,
    );
  });

  test("表示名が分からない(空文字)ときは ID をそのまま出す", () => {
    const label = unsupportedText.markLabel(moveMark, "");
    expect(label).toContain(moveMark.id);
    expect(label).toContain(unsupportedText.reason.multi_hit);
  });

  // iOS レーンの決定(DECISIONS.md 2026-09-25): reason が unsupported_effect のときは、
  // 「対象名だけで意味が通る」ため理由の括弧を省く。
  test("reason が unsupported_effect のときは理由の括弧を省く", () => {
    const mark: UnsupportedMark = {
      target: "attacker_item",
      reason: "unsupported_effect",
      id: "example-item-x",
    };
    const label = unsupportedText.markLabel(mark, "テストもちもの");
    expect(label).toBe(`${unsupportedText.target.attacker_item}「テストもちもの」`);
    expect(label).not.toContain("(");
    expect(label).not.toContain(")");
  });

  test("unsupported_effect 以外は必ず理由を括弧付きで含む", () => {
    for (const reason of allReasons) {
      if (reason === "unsupported_effect") {
        continue;
      }
      const label = unsupportedText.markLabel({ target: "move", reason, id: "x" }, "テスト技");
      expect(label).toContain(`(${unsupportedText.reason[reason]})`);
    }
  });

  test.each([
    ["attacker_item", "unsupported_effect"],
    ["attacker_ability", "unsupported_effect"],
    ["defender_item", "unsupported_effect"],
    ["defender_ability", "unsupported_effect"],
  ] as const)("持ち物・特性の印(%s)も target と名前が分かる文言になる", (target, reason) => {
    const label = unsupportedText.markLabel({ target, reason, id: "example-item-x" }, "テストもちもの");
    expect(label).toContain(unsupportedText.target[target]);
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

describe("notice(結果の上に出す案内)", () => {
  test("iOS レーンの書式: 「この結果は正確でない可能性があります(未対応: <印>、<印>)」", () => {
    expect(unsupportedText.notice(["印A", "印B"])).toBe(
      "この結果は正確でない可能性があります(未対応: 印A、印B)",
    );
  });

  test("印が1件のときも同じ書式になる", () => {
    expect(unsupportedText.notice(["印A"])).toBe("この結果は正確でない可能性があります(未対応: 印A)");
  });
});

describe("rowLabel(行・候補カードに出す文言)", () => {
  test("iOS レーンの書式: 「未対応: <印>、<印>」", () => {
    expect(unsupportedText.rowLabel(["印A", "印B"])).toBe("未対応: 印A、印B");
  });

  test("印が1件のときも同じ書式になる", () => {
    expect(unsupportedText.rowLabel(["印A"])).toBe("未対応: 印A");
  });
});
