// issue #515・ADR-0320(docs/mega-evolution-spec.md §4-3): 計算画面で、メガ種族を選ぶと持ち物がメガストーンに固定される流れ。
// 既定の構成(オンラインのマスタ = pokedex フィクスチャ)で動く。計算(calc-svc)は使わず、入力欄の固定だけを確かめる。
// フィクスチャのメガ種族・メガストーンは架空(src/test/megaMaster.ts。実データは使わない)。

import { expect, test } from "@playwright/test";
import { MEGA, SPECIES, combobox, openApp, selectSpeciesBySearch } from "./support/calcPage.ts";

test("メガ種族を選ぶと持ち物がメガストーンに固定され、メガでない種族に変えると解除される", async ({
  page,
}) => {
  await openApp(page);
  await selectSpeciesBySearch(page, "攻撃側のポケモン", MEGA.fire.nameJa);

  const item = combobox(page, "攻撃側の持ち物");
  await expect(item).toBeDisabled();
  await expect(item.locator("option:checked")).toHaveText(MEGA.fire.stoneNameJa);
  // 理由は見える文言と、欄の説明(aria-describedby)の両方で伝わる。
  await expect(page.getByText(MEGA.lockedReason)).toBeVisible();
  await expect(item).toHaveAccessibleDescription(MEGA.lockedReason);

  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.fire.nameJa);
  await expect(item).toBeEnabled();
  await expect(item).toHaveValue("");
  await expect(page.getByText(MEGA.lockedReason)).toHaveCount(0);
  // メガストーンは単独の持ち物の選択肢に出ない。
  await expect(item.getByRole("option", { name: MEGA.fire.stoneNameJa })).toHaveCount(0);
});
