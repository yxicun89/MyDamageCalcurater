// P4-2: タイプ相性表のデータ(testdata/golden/typechart.json、Vite の別名 @typechart)を
// engine の TypeChart DTO({types, effectiveness})に変換する(ADR-0300 §3、ADR-0011 §13、ADR-0013)。
// typechart.json は schemaVersion・source・note・excludedTypes などの説明用フィールドも持つが、
// engine の境界は未知のフィールドを拒否する(DisallowUnknownFields → unknown_field)ので、2つだけを渡す。

import { readFileSync } from "node:fs";
import typeChartData from "@typechart";
import { expect, test } from "vitest";
import { typeChartPath } from "../test/typeChart";
import { typeChartFromData } from "./typeChart";

/** 期待値は実装を通さず、ファイルを直接読んで作る(独立な検証)。 */
function chartOnDisk(): { types: unknown; effectiveness: unknown } {
  const parsed = JSON.parse(readFileSync(typeChartPath, "utf8")) as {
    types: unknown;
    effectiveness: unknown;
  };
  return { types: parsed.types, effectiveness: parsed.effectiveness };
}

test("@typechart の内容を types と effectiveness の2つだけの DTO にする", () => {
  const chart = typeChartFromData(typeChartData);
  expect(Object.keys(chart).sort()).toEqual(["effectiveness", "types"]);
  expect(chart).toEqual(chartOnDisk());
});

test("説明用のフィールド(schemaVersion など)は DTO に含めない", () => {
  const chart = typeChartFromData({
    schemaVersion: 1,
    source: "x",
    note: "y",
    excludedTypes: ["stellar"],
    types: ["fire", "water"],
    effectiveness: { fire: { water: 1 }, water: { fire: 4 } },
  });
  expect(chart).toEqual({
    types: ["fire", "water"],
    effectiveness: { fire: { water: 1 }, water: { fire: 4 } },
  });
});

test.each([
  ["null", null],
  ["types が無い", { effectiveness: {} }],
  ["effectiveness が無い", { types: ["fire"] }],
  ["types が配列でない", { types: "fire", effectiveness: {} }],
])("形が違うデータ(%s)は原因の分かる例外にする(黙って空の表にしない)", (_label, data) => {
  expect(() => typeChartFromData(data)).toThrow(/typechart|相性表/);
});
