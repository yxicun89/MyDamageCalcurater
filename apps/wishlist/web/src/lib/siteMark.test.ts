import { describe, expect, it } from "vitest";
import { siteColor, siteMark } from "./siteMark";

// phase5-spec 5-3「サイトのマーク」/「Web に反映する点」。iOS の SiteMark と同じ規則。
describe("siteMark", () => {
  it("通常はサイト名の先頭 1 文字", () => {
    expect(siteMark("メルカリ")).toBe("メ");
    expect(siteMark("Amazon")).toBe("A");
    expect(siteMark("駿河屋")).toBe("駿");
    expect(siteMark("魂ウェブ")).toBe("魂");
    expect(siteMark("")).toBe("");
  });

  it("Yahoo! で始まる名前は、その後ろの最初の 1 文字(フリマ「フ」・ショッピング「シ」。同じ「Y」を並べない)", () => {
    expect(siteMark("Yahoo!フリマ")).toBe("フ");
    expect(siteMark("Yahoo!ショッピング")).toBe("シ");
    expect(siteMark("Yahoo!")).toBe("Y");
  });
});

describe("siteColor", () => {
  it("名前の表から引く。Yahoo!ショッピングは Yahoo!フリマと別の色。無い名前は --muted", () => {
    expect(siteColor("メルカリ")).toBe("#ff4d6d");
    expect(siteColor("Yahoo!フリマ")).toBe("#ef233c");
    expect(siteColor("Yahoo!ショッピング")).toBe("#2f9e44");
    expect(siteColor("その他")).toBe("var(--muted)");
  });
});
