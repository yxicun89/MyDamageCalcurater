// F-12(I-web-6、ADR-0331 §6): 見た目の基盤で CSS とアイコンが増えるので、CSS にも初期ロードの予算を置く。
// 正は docs/design.md「パフォーマンス予算」。build の最後に走る scripts/check-bundle-size.mjs が JS と CSS の
// 両方を予算と比べる(超えたら build が失敗する)。ここでは design.md と script の値が一致することを見る。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { bulletLine, level2SectionOf, readDesignDoc } from "../test/designDoc";
import { localPath } from "../test/localPath";

const budgetSection = level2SectionOf(readDesignDoc(), "パフォーマンス予算");
const script = readFileSync(localPath("../../scripts/check-bundle-size.mjs", import.meta.url), "utf8");

function kilobytes(line: string, label: string): number {
  const match = new RegExp(`${label}\\s*≤\\s*(\\d+)KB`).exec(line);
  if (match?.[1] === undefined) {
    throw new Error(`design.md の「${line}」に「${label} ≤ nKB」が無い`);
  }
  return Number(match[1]);
}

describe("初期ロードの予算(gzip)", () => {
  const line = bulletLine(budgetSection, "Web の初期ロード:");

  test("JS の予算は design.md と script で同じ(既存。WASM 除く)", () => {
    const js = kilobytes(line, "JS");
    expect(script).toContain(`JS_BUDGET_GZIP_BYTES = ${String(js)} * 1024`);
  });

  test("CSS の予算が design.md にあり、script も同じ値で CSS を数える", () => {
    const cssBudget = kilobytes(line, "CSS");
    expect(cssBudget).toBeGreaterThan(0);
    expect(script).toContain(`CSS_BUDGET_GZIP_BYTES = ${String(cssBudget)} * 1024`);
    expect(script).toMatch(/\.endsWith\(["']\.css["']\)/);
  });
});
