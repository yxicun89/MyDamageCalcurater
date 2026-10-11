// F-12(I-web-6、ADR-0334 §2): インライン SVG の小さなアイコン部品。外部の依存(アイコンフォント・ライブラリ)を入れない。
// 装飾のアイコン(名前を渡さない)は支援技術から隠し(aria-hidden)、意味を持つアイコンだけ名前(role="img" + aria-label)を持つ。
// 色は文字色に従う(currentColor)。色の直書きはしない(noColorLiterals.test.ts の対象でもある)。

import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { ICON_NAMES, Icon, type IconName } from "./Icon";

/** ADR-0334 §2: タブ(画面)ごとのアイコン + このアプリについて + 未知の画面の既定 + 操作の小さなアイコン。 */
const REQUIRED_ICONS: readonly IconName[] = [
  "calc",
  "reverse",
  "speed",
  "balance",
  "judge",
  "team",
  "favorites",
  "adjust",
  "about",
  "screen",
  "swap",
  "plus",
  "trash",
  "edit",
  "search",
  "alert",
  "info",
  "check",
  // G-05(ADR-0339)
  "minus",
  "close",
  "open",
  "history",
  "help",
  "up",
  "down",
];

function svgOf(container: HTMLElement): SVGSVGElement {
  const svg = container.querySelector("svg");
  if (svg === null) {
    throw new Error("svg が描かれていない");
  }
  return svg;
}

describe("Icon", () => {
  test("ADR-0334 のアイコンがそろっている(名前の重複なし)", () => {
    expect(ICON_NAMES).toEqual(expect.arrayContaining([...REQUIRED_ICONS]));
    expect(new Set(ICON_NAMES).size).toBe(ICON_NAMES.length);
  });

  test.each([...REQUIRED_ICONS])("%s: 名前なしは装飾(aria-hidden・フォーカスしない・role なし)", (name) => {
    const { container } = render(<Icon name={name} />);
    const svg = svgOf(container);
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg).toHaveAttribute("focusable", "false");
    expect(svg).not.toHaveAttribute("role");
    expect(svg).not.toHaveAttribute("aria-label");
    expect(svg).toHaveClass("ui-icon");
    expect(svg).toHaveAttribute("viewBox", "0 0 24 24");
  });

  test("名前を渡すと role=img と aria-label を持ち、aria-hidden を付けない", () => {
    render(<Icon name="alert" label="注意" />);
    const img = screen.getByRole("img", { name: "注意" });
    expect(img.tagName.toLowerCase()).toBe("svg");
    expect(img).not.toHaveAttribute("aria-hidden");
  });

  test("寸法の既定は 20、size で変えられる", () => {
    const { container, rerender } = render(<Icon name="calc" />);
    expect(svgOf(container)).toHaveAttribute("width", "20");
    expect(svgOf(container)).toHaveAttribute("height", "20");
    rerender(<Icon name="calc" size={16} />);
    expect(svgOf(container)).toHaveAttribute("width", "16");
    expect(svgOf(container)).toHaveAttribute("height", "16");
  });

  test("className を足せる(ui-icon は残す)", () => {
    const { container } = render(<Icon name="team" className="app-tabs__icon" />);
    expect(svgOf(container)).toHaveClass("ui-icon", "app-tabs__icon");
  });

  test.each([...REQUIRED_ICONS])("%s: 色は currentColor か none だけ(色の直書きなし)", (name) => {
    const { container } = render(<Icon name={name} />);
    const painted = [svgOf(container), ...svgOf(container).querySelectorAll("*")].flatMap((element) =>
      ["fill", "stroke", "color", "stop-color"].flatMap((attribute) => {
        const value = element.getAttribute(attribute);
        return value === null ? [] : [value];
      }),
    );
    expect(painted.filter((value) => value !== "currentColor" && value !== "none")).toEqual([]);
    expect(svgOf(container).innerHTML).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgba?\(/);
  });

  test("アイコンごとに形が違う(同じ図形の使い回しでない)", () => {
    const shapes = REQUIRED_ICONS.map((name) => {
      const { container, unmount } = render(<Icon name={name} />);
      const markup = svgOf(container).innerHTML;
      unmount();
      return markup;
    });
    expect(new Set(shapes).size).toBe(REQUIRED_ICONS.length);
  });

  test("画像ファイル・外部 URL を参照しない(<image>・<use href> を使わない)", () => {
    for (const name of REQUIRED_ICONS) {
      const { container, unmount } = render(<Icon name={name} />);
      expect(container.querySelector("image, use")).toBeNull();
      unmount();
    }
  });
});
