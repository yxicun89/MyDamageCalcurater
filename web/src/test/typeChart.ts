// テスト専用: 相性表のデータ(testdata/golden/typechart.json)からタイプ ID の一覧を読む。
// タイプの一覧はこのファイル(相性表)が正であり(ADR-0300 §4)、タイプ関連のテストの期待値はここから作る
// (コーディング規約 §2)。tokens.test.ts と ja.test.ts の両方で使うため、ここに1か所だけ持つ。

import { readFileSync } from "node:fs";
import { localPath } from "./localPath";

export const typeChartPath = localPath("../../../testdata/golden/typechart.json", import.meta.url);

/** 相性表(testdata/golden/typechart.json)が持つ18タイプの ID。 */
export function typeIds(): string[] {
  const chart: unknown = JSON.parse(readFileSync(typeChartPath, "utf8"));
  if (typeof chart !== "object" || chart === null || !("types" in chart) || !Array.isArray(chart.types)) {
    throw new Error("typechart.json に types の配列が無い");
  }
  return chart.types.map(String);
}
