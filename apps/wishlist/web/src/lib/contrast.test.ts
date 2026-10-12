import { describe, expect, it } from "vitest";
import { ON_DARK, ON_LIGHT, luminance, onColor } from "./contrast";
import { COLOR_SWATCHES } from "../ui/icons";

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
  // 設定の色見本 12 色すべての期待値(見本を変えたら、ここで読めるかを確かめ直す)
  const DARK_ON = new Set([
    "#f0642a",
    "#f59e0b",
    "#f2b705",
    "#7cb518",
    "#16a34a",
    "#0d9488",
    "#0ea5e9",
    "#e04f9a",
  ]);
  it.each(COLOR_SWATCHES.map((s) => [s.label, s.color] as const))("色見本 %s(%s)", (_label, color) => {
    expect(onColor(color)).toBe(DARK_ON.has(color) ? ON_DARK : ON_LIGHT);
  });
  it("色見本は 12 色で、期待値の表と同じ色を持つ", () => {
    expect(COLOR_SWATCHES.map((s) => s.color).sort()).toEqual(
      [
        "#f0642a",
        "#f59e0b",
        "#f2b705",
        "#7cb518",
        "#16a34a",
        "#0d9488",
        "#0ea5e9",
        "#e04f9a",
        "#e0393e",
        "#1f6fe5",
        "#4f46e5",
        "#7c4dff",
      ].sort(),
    );
  });
});
