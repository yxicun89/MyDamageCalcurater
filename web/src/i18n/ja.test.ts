// P4-1: タイプの表示名は web/src/i18n/ja.ts の文言資源が ID → 表示名で持つ(ADR-0300 §4)。
// 一覧は相性表のデータ(testdata/golden/typechart.json の types)が正で、表示名はその全 ID を過不足なく覆う。

import { expect, test } from "vitest";
import { readDesignDoc, typeColorsByJapaneseName } from "../test/designDoc";
import { typeIds } from "../test/typeChart";
import { typeNameJa } from "./ja";

test("相性表の types の全 ID に日本語の表示名があり、余分な ID は無い", () => {
  expect(Object.keys(typeNameJa).sort()).toEqual([...typeIds()].sort());
});

test("表示名は空でなく、重複しない", () => {
  const names = Object.values(typeNameJa);
  expect(names.every((name) => name.trim().length > 0)).toBe(true);
  expect(new Set(names).size).toBe(names.length);
});

test("表示名は design.md のタイプ色の表と同じ呼び名", () => {
  const designNames = [...typeColorsByJapaneseName(readDesignDoc()).keys()].sort();
  expect(Object.values(typeNameJa).sort()).toEqual(designNames);
});
