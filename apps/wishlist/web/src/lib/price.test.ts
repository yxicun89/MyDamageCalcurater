import { describe, expect, it } from "vitest";
import type { SuspiciousReason } from "../api/types";
import { formatAge, formatJstDate, formatRange, formatYen, isHttpUrl, reasonLabel } from "./price";

// AC-EST-08: 金額・日付・理由の表示(純粋関数)
describe("formatYen", () => {
  it.each([
    [0, "¥0"],
    [300, "¥300"],
    [3000, "¥3,000"],
    [1234567, "¥1,234,567"],
  ])("%i → %s", (n, want) => {
    expect(formatYen(n)).toBe(want);
  });
});

describe("formatRange", () => {
  it.each<[number, number | null | undefined, string]>([
    [3000, 4500, "¥3,000〜¥4,500"],
    [3000, null, "¥3,000〜"],
    [3000, undefined, "¥3,000〜"],
    [3000, 3000, "¥3,000〜¥3,000"],
  ])("low=%i mid=%s → %s", (low, mid, want) => {
    expect(formatRange(low, mid)).toBe(want);
  });
});

describe("formatJstDate(JST の M/D。ゼロ詰めしない)", () => {
  it.each([
    ["2026-10-03T00:30:00Z", "10/3"],
    ["2026-10-02T15:30:00Z", "10/3"], // UTC では 10/2 だが JST では 10/3 0:30
    ["2026-10-02T14:59:59Z", "10/2"],
    ["2026-12-31T15:00:00Z", "1/1"], // 年またぎ
    ["2026-01-05T00:00:00Z", "1/5"],
    ["2026-10-03T08:00:00+09:00", "10/3"],
    ["2026-10-03T00:00:00+09:00", "10/3"],
  ])("%s → %s", (iso, want) => {
    expect(formatJstDate(iso)).toBe(want);
  });
});

describe("formatAge(JST の暦日の差)", () => {
  const now = new Date("2026-10-03T03:00:00Z"); // JST 10/3 12:00
  it.each([
    ["2026-10-03T03:00:00Z", "今日"],
    ["2026-10-02T15:00:00Z", "今日"], // JST 0:00 ちょうど
    ["2026-10-02T14:59:00Z", "1日前"], // 12 時間前でも暦日は前日
    ["2026-10-02T00:00:00Z", "1日前"],
    ["2026-09-30T00:00:00Z", "3日前"],
    ["2026-09-03T00:00:00Z", "30日前"],
    ["2026-10-04T00:00:00Z", "今日"], // 未来(時計のずれ)は今日
  ])("%s → %s", (iso, want) => {
    expect(formatAge(iso, now)).toBe(want);
  });
});

describe("reasonLabel", () => {
  it.each<[SuspiciousReason, string]>([
    ["title_mismatch", "商品名が一致しない"],
    ["too_cheap", "安すぎる"],
    ["below_min", "下限価格未満"],
  ])("%s → %s", (r, want) => {
    expect(reasonLabel(r)).toBe(want);
  });
});

describe("isHttpUrl(リンク・画像にしてよいのは http(s) だけ)", () => {
  it.each([
    ["http://shop.example/a", true],
    ["https://shop.example/a?b=1", true],
    ["HTTPS://SHOP.EXAMPLE/", true],
    ["javascript:alert(1)", false],
    ["data:image/png;base64,AAAA", false],
    ["ftp://shop.example/a", false],
    ["/images/a.png", false],
    ["//shop.example/a", false],
    ["not a url", false],
    ["", false],
  ])("%s → %s", (url, want) => {
    expect(isHttpUrl(url)).toBe(want);
  });
});
