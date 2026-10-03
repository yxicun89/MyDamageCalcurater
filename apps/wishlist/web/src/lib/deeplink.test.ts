import { describe, expect, it } from "vitest";
import { buildDeeplink, isValidSearchTemplate } from "./deeplink";
import { deeplinkCases } from "../test/vectors";

// AC-LIB-02: 共通テストベクタ(deeplink)を全件満たす。
describe("buildDeeplink", () => {
  it("テストベクタが空でない", () => {
    expect(deeplinkCases.length).toBeGreaterThan(0);
  });
  it.each(deeplinkCases)("$template $query", (c) => {
    expect(buildDeeplink(c.template, c.query)).toBe(c.want);
  });
});

// AC-LIB-03: テンプレートの検証。http(s) で {q} を含むこと。
describe("isValidSearchTemplate", () => {
  it.each([
    ["https://jp.mercari.com/search?keyword={q}&status=on_sale", true],
    ["http://example.com/s?q={q}", true],
    ["https://example.com/s?q=", false],
    ["ftp://example.com/s?q={q}", false],
    ["javascript:alert({q})", false],
    ["example.com/s?q={q}", false],
    ["", false],
  ])("%s -> %s", (template, want) => {
    expect(isValidSearchTemplate(template)).toBe(want);
  });
});
