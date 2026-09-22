// マスタの相性表データ(@typechart、testdata/golden/typechart.json)を engine の TypeChart DTO に変換する
// (ADR-0300 §3、ADR-0011 §13)。schemaVersion・source・note・excludedTypes などの説明用フィールドは
// engine の境界に渡さない(DisallowUnknownFields → unknown_field。ADR-0011 §4)。

import type { TypeChart } from "../engine/types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * @typechart のデータを engine の TypeChart({types, effectiveness})にする。
 * 形が違う入力は、黙って空の表にせず原因の分かる例外にする。
 */
export function typeChartFromData(data: unknown): TypeChart {
  if (!isRecord(data)) {
    throw new Error("typechart のデータがオブジェクトでない(相性表が壊れている)");
  }
  const { types, effectiveness } = data;
  if (!Array.isArray(types)) {
    throw new Error("typechart.types が配列でない(相性表が壊れている)");
  }
  if (!isRecord(effectiveness)) {
    throw new Error("typechart.effectiveness がオブジェクトでない(相性表が壊れている)");
  }
  return {
    types: types.map((value) => String(value)),
    effectiveness: effectiveness as TypeChart["effectiveness"],
  };
}
