// issue #515・ADR-0320: メガシンカの持ち物固定の文言(i18n/ja.ts の megaItemText)。iOS は同じ語にそろえる
// (docs/mega-evolution-spec.md §4-3)ので、語を変えるときは iOS と同時に直す。画面は直書きせずここから引く。

import { expect, test } from "vitest";
import { megaItemText } from "./ja";

test("固定の理由は仕様の例どおり(iOS と同じ語)", () => {
  expect(megaItemText.lockedReason).toBe("メガシンカするので、持ち物はメガストーンに決まっています");
});

test("文言は空でなく、互いに異なる", () => {
  const texts = [
    megaItemText.lockedReason,
    megaItemText.missingReason,
    megaItemText.compareDisabledReason,
    megaItemText.fixedItemName("テストナイト"),
  ];
  expect(texts.every((text) => text.trim().length > 0)).toBe(true);
  expect(new Set(texts).size).toBe(texts.length);
});

test("相手の固定の持ち物の表示にはメガストーンの名前が入る", () => {
  expect(megaItemText.fixedItemName("テストナイト")).toContain("テストナイト");
});
