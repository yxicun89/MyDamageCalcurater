// P5-3c(ADR-0327 §2): 計算画面の攻撃側からお気に入りの作成本文を組み立てる純粋関数。

import { describe, expect, test } from "vitest";
import { MAX_FAVORITE_LABEL_LENGTH, favoriteInputOf } from "./favoriteInput";

const SP = { hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0 } as const;

function inputWith(label: string, itemId: string | null = null) {
  return favoriteInputOf({ label, speciesKey: "9001-000", natureId: "fake-nature", sp: SP, itemId });
}

describe("favoriteInputOf", () => {
  test("label がちょうど 30 コードポイントなら切らない", () => {
    const label = "あ".repeat(MAX_FAVORITE_LABEL_LENGTH);
    expect(inputWith(label).label).toBe(label);
  });

  test("31 コードポイントなら 30 に切る", () => {
    expect(inputWith("あ".repeat(MAX_FAVORITE_LABEL_LENGTH + 1)).label).toBe(
      "あ".repeat(MAX_FAVORITE_LABEL_LENGTH),
    );
  });

  test("サロゲートペア(絵文字)を割らない", () => {
    const label = `${"あ".repeat(MAX_FAVORITE_LABEL_LENGTH - 1)}😀😀`;
    const cut = inputWith(label).label ?? "";
    expect(Array.from(cut)).toHaveLength(MAX_FAVORITE_LABEL_LENGTH);
    expect(cut.endsWith("😀")).toBe(true);
  });

  test("itemId が null ならキーごと省き、あれば入れる。level は 50", () => {
    expect("itemId" in inputWith("x").individual).toBe(false);
    expect(inputWith("x", "fake-item").individual.itemId).toBe("fake-item");
    expect(inputWith("x").individual.level).toBe(50);
  });
});
