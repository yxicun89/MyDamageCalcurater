import { describe, expect, it } from "vitest";
import { ON_DARK, ON_LIGHT, luminance, onColor } from "./contrast";

describe("onColor(地の明るさで文字色を決める)", () => {
  it.each([
    ["#f2b705", ON_DARK], // 黄
    ["#ffffff", ON_DARK],
    ["#f59e0b", ON_DARK], // 橙
    ["#7cb518", ON_DARK], // 黄緑
    ["#1d5fd1", ON_LIGHT], // 強調色
    ["#e0393e", ON_LIGHT], // 赤
    ["#4f46e5", ON_LIGHT],
    ["#000000", ON_LIGHT],
    ["bad", ON_LIGHT],
  ])("%s の上は %s", (bg, expected) => {
    expect(onColor(bg)).toBe(expected);
  });

  it("輝度は白が 1・黒が 0・読めなければ null", () => {
    expect(luminance("#ffffff")).toBeCloseTo(1, 5);
    expect(luminance("#000000")).toBe(0);
    expect(luminance("zzz")).toBeNull();
  });
});
