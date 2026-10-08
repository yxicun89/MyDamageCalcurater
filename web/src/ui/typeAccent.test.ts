// F-12(I-web-6、ADR-0334 §2): カードをタイプ色で染めるための CSS 変数(--card-type)を作る。
// 画面は選択中の種族の最初のタイプを渡し、.ui-card--typed がふち・上端の帯にこの変数を使う。
// 未知のタイプ ID でも壊さない(既定値つきの var。design.md「タイプバッジ」と同じ作法)。
// ID は CSS の変数名に埋め込むので、英小文字・数字・ハイフン以外を含む値は使わない(CSS の注入を防ぐ)。

import { describe, expect, test } from "vitest";
import { CARD_TYPE_VARIABLE, typeAccentStyle } from "./typeAccent";

describe("typeAccentStyle", () => {
  test("変数名は --card-type", () => {
    expect(CARD_TYPE_VARIABLE).toBe("--card-type");
  });

  test.each(["fire", "water", "electric", "fairy"])("%s → var(--type-<id>, var(--brand-primary))", (id) => {
    expect(typeAccentStyle(id)).toEqual({ "--card-type": `var(--type-${id}, var(--brand-primary))` });
  });

  test("相性表に無い ID でも既定値つきの var を返す(色は既定のブランド色に落ちる)", () => {
    expect(typeAccentStyle("stellar")).toEqual({
      "--card-type": "var(--type-stellar, var(--brand-primary))",
    });
  });

  test.each([undefined, ""])("種族が未選択(%s)なら変数を置かない", (id) => {
    expect(typeAccentStyle(id)).toEqual({});
  });

  test.each(["Fire", "fire;color:red", "fire)", "../x", "fire water", "ほのお"])(
    "CSS の変数名に使えない値(%s)は置かない",
    (id) => {
      expect(typeAccentStyle(id)).toEqual({});
    },
  );
});
