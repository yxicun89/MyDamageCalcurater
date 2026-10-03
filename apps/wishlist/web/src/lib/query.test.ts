import { describe, expect, it } from "vitest";
import { buildQuery } from "./query";
import { buildCases } from "../test/vectors";

// AC-LIB-01: Go と共通のテストベクタ(testdata/query-cases.json の build)を全件満たす。
describe("buildQuery", () => {
  it("テストベクタが空でない", () => {
    expect(buildCases.length).toBeGreaterThan(0);
  });
  it.each(buildCases)("$note", (c) => {
    expect(
      buildQuery({
        template: c.template,
        name: c.name,
        option: c.option === "" ? null : c.option,
        queryOverride: c.query_override,
        siteQuery: c.site_query,
      }),
    ).toBe(c.want);
  });
  // option が空文字の事例だけが対象(option が入っている事例は null と結果が違うのが正しい)。
  const emptyOptionCases = buildCases.filter((c) => c.option === "");
  it("空 option の事例がベクタにある", () => {
    expect(emptyOptionCases.length).toBeGreaterThan(0);
  });
  it.each(emptyOptionCases)("option が空文字でも null でも同じ結果: $note", (c) => {
    const base = {
      template: c.template,
      name: c.name,
      queryOverride: c.query_override,
      siteQuery: c.site_query,
    };
    expect(buildQuery({ ...base, option: c.option })).toBe(buildQuery({ ...base, option: null }));
  });
});
