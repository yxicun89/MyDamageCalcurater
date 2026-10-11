// G-01(ADR-0341)・G-05 方針「アイコンの決まり」: 技の分類 3 つ(ぶつり・とくしゅ・へんか)の自作アイコン。
// 名前は icon 名 movePhysical / moveSpecial / moveStatus。読み上げ用の名前は用語集の語(ぶつり・とくしゅ・へんか)を
// label で渡す。分類で色を変えず文字色のまま(currentColor)=色の指定を持たない。既存の Icon.test.tsx は変えない。

import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { ICON_NAMES, Icon, type IconName } from "./Icon";

const CATEGORY_ICONS: readonly { readonly name: IconName; readonly label: string }[] = [
  { name: "movePhysical", label: "ぶつり" },
  { name: "moveSpecial", label: "とくしゅ" },
  { name: "moveStatus", label: "へんか" },
];

describe("技の分類アイコン", () => {
  test("3 つとも ICON_NAMES にあり、形(path)が互いに違う", () => {
    for (const { name } of CATEGORY_ICONS) {
      expect(ICON_NAMES).toContain(name);
    }
    const shapes = CATEGORY_ICONS.map(({ name }) => {
      const { container, unmount } = render(<Icon name={name} />);
      const d = [...container.querySelectorAll("path")].map((p) => p.getAttribute("d")).join("|");
      unmount();
      return d;
    });
    for (const d of shapes) {
      expect(d.length).toBeGreaterThan(0);
    }
    expect(new Set(shapes).size).toBe(3);
  });

  test.each(CATEGORY_ICONS)("$name: 名前なしは装飾、名前ありは role=img の「$label」", ({ name, label }) => {
    const { container, unmount } = render(<Icon name={name} />);
    expect(container.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    unmount();
    render(<Icon name={name} label={label} />);
    expect(screen.getByRole("img", { name: label })).toBeInTheDocument();
  });

  test("色を直書きしない(線の色は文字色に従う)", () => {
    for (const { name } of CATEGORY_ICONS) {
      const { container, unmount } = render(<Icon name={name} />);
      const html = container.innerHTML;
      expect(html).not.toMatch(/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i);
      unmount();
    }
  });
});
