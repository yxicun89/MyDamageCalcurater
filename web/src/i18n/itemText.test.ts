// ADR-0326(ADR-0175 §4): 持ち物の文言(i18n/items.ts の itemRoleText)。iOS は同じ語にそろえる
// (docs/mega-evolution-spec.md §4-3)ので、語を変えるときは iOS と同時に直す。画面は直書きせずここから引く。

import { expect, test } from "vitest";
import { itemRoleText } from "./items";

test("固定中の表示は「{基本種名}のメガストーン」(ユーザー報告の例: ルカリオのメガストーン)", () => {
  expect(itemRoleText.megaStoneOf("ルカリオ")).toBe("ルカリオのメガストーン");
});

test("基本種名が分からないときは「メガストーン」だけ", () => {
  expect(itemRoleText.megaStoneUnnamed).toBe("メガストーン");
});

test("役割に合わず外した通知は、持ち物の名前と側を含み、側ごとに異なる", () => {
  const attacker = itemRoleText.droppedNotice("テストぼうぎょだま", "attacker");
  const defender = itemRoleText.droppedNotice("テストぼうぎょだま", "defender");
  expect(attacker).toBe("テストぼうぎょだまは攻撃側では計算に影響しないため、持ち物を外しました");
  expect(defender).toBe("テストぼうぎょだまは防御側では計算に影響しないため、持ち物を外しました");
});
