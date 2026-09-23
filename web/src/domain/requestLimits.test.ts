// @vitest-environment node
// P4-19(issue #110、ADR-0208): 候補・観測の件数上限。値の正は api/openapi.yaml なので、
// domain/requestLimits.ts の定数が契約とずれていないことを YAML を読んで確かめる
// (コーディング規約 §2「同じ定義を複数箇所に書かない。意図的な重複には同期を検査するテストを置く」)。
// 併せて「上限までを先頭から取る」絞り込み(ADR-0300 §6・§7 のマスタの順序をそのまま使う規約)の境界を確かめる。

import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";
import { MAX_ITEM_CANDIDATES, MAX_ITEM_VARIANTS, MAX_OBSERVATIONS, limitToMax } from "./requestLimits";

const openapiLines = readFileSync(localPath("../../../api/openapi.yaml", import.meta.url), "utf8").split(
  "\n",
);

/** `<indent 空白><header>:` の行に続く、より深くインデントされた塊。無ければ分かる形で失敗させる。 */
function blockOf(lines: readonly string[], header: string, indent: number): string[] {
  const prefix = " ".repeat(indent);
  const start = lines.indexOf(`${prefix}${header}:`);
  if (start < 0) {
    throw new Error(`api/openapi.yaml に ${header}: (インデント ${String(indent)})が無い`);
  }
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => line.trim() !== "" && !line.startsWith(`${prefix} `));
  return end < 0 ? rest : rest.slice(0, end);
}

/** components.schemas.<schema>.properties.<property>.maxItems の値。 */
function maxItemsOf(schema: string, property: string): number {
  const propertyLines = blockOf(blockOf(blockOf(openapiLines, schema, 4), "properties", 6), property, 8);
  for (const line of propertyLines) {
    const matched = /^\s*maxItems:\s*(\d+)\s*$/.exec(line);
    if (matched?.[1] !== undefined) {
      return Number(matched[1]);
    }
  }
  throw new Error(`api/openapi.yaml の ${schema}.${property} に maxItems が無い`);
}

describe("api/openapi.yaml の maxItems と同じ値を持つ(ADR-0208 の上限。ずれたらこのテストが落ちる)", () => {
  test.each([
    ["BulkCalcRequest.itemVariants", "BulkCalcRequest", "itemVariants", MAX_ITEM_VARIANTS],
    ["ReverseRequest.itemCandidates", "ReverseRequest", "itemCandidates", MAX_ITEM_CANDIDATES],
    ["ReverseRequest.observations", "ReverseRequest", "observations", MAX_OBSERVATIONS],
  ] as const)("%s", (_label, schema, property, constant) => {
    expect(maxItemsOf(schema, property)).toBe(constant);
  });
});

describe("limitToMax(上限までを先頭から取る。並べ替えない)", () => {
  const values = Array.from({ length: 10 }, (_value, index) => index);

  // 上限 10 に対して 9件(上限-1)・10件(ちょうど上限)・11件(上限+1)。
  test.each([
    ["上限-1 件はそのまま", values.slice(0, 9), [0, 1, 2, 3, 4, 5, 6, 7, 8], false],
    ["ちょうど上限は切らない", values, [0, 1, 2, 3, 4, 5, 6, 7, 8, 9], false],
    ["上限+1 件は末尾を1件落とす", [...values, 10], [0, 1, 2, 3, 4, 5, 6, 7, 8, 9], true],
  ] as const)("%s", (_label, input, expected, truncated) => {
    const limited = limitToMax(input, 10);
    expect(limited.values).toEqual(expected);
    expect(limited.truncated).toBe(truncated);
  });

  test("空の配列は空のまま(truncated は false)", () => {
    expect(limitToMax([], MAX_ITEM_VARIANTS)).toEqual({ values: [], truncated: false });
  });

  test("落とすのは末尾だけで、残った要素の順と実体は変わらない", () => {
    const items = [{ id: "a" }, { id: "b" }, { id: "c" }];
    const limited = limitToMax(items, 2);
    expect(limited.values[0]).toBe(items[0]);
    expect(limited.values[1]).toBe(items[1]);
    expect(limited.values).toHaveLength(2);
    expect(limited.truncated).toBe(true);
  });
});
