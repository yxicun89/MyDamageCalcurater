// CB-6 段階 B(ADR-0359): 計算画面の連続技(オフライン = WASM の calcCombo)。docs/plan/combo-calc.md AC-19。
// 手がメインだけのときは「次の技」だけ。足すと 2 手の結果(各手・合計・N 手で倒す・素早さ)が出て、防御側の振りを変えると合計が変わる。

import { expect, test } from "@playwright/test";
import { SPECIES, combobox, openAppOffline, selectMatchup } from "./support/calcPage.ts";

test.beforeEach(async ({ page }) => {
  await openAppOffline(page);
});

test("「次の技」で 2 手目を足すと、各手・合計・N 手で倒す・素早さが出る", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const section = page.getByRole("region", { name: "連続技", exact: true });
  await expect(section.getByRole("button", { name: "次の技", exact: true })).toBeVisible();
  await expect(section.getByRole("combobox", { name: "追加の技", exact: true })).toHaveCount(0);

  await section.getByRole("button", { name: "次の技", exact: true }).click();
  await section.getByRole("combobox", { name: "追加の技", exact: true }).click();
  await page.getByRole("listbox", { name: "追加の技", exact: true }).getByRole("option").nth(1).click();

  const steps = section.getByRole("list", { name: "連続技の各手の結果", exact: true }).getByRole("listitem");
  await expect(steps).toHaveCount(2);
  await expect(steps.first()).toContainText(/\d+\.\d〜\d+\.\d%/);
  await expect(section.getByText(/^合計 \d+\.\d〜\d+\.\d%$/)).toBeVisible();
  await expect(section.getByText(/^(2 手で倒す( \d+\.\d%|\(確定\))|倒せない)$/)).toBeVisible();
  await expect(section.getByText(/^素早さ \d+ → \d+$/)).toBeVisible();
  // 画面の上の単発の結果はそのまま残る。
  await expect(combobox(page, "技")).not.toHaveAttribute("data-value", "");
});

test("防御側の振りを変えると、合計のダメージが変わる(UC-5)", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const section = page.getByRole("region", { name: "連続技", exact: true });
  await section.getByRole("button", { name: "次の技", exact: true }).click();
  await section.getByRole("combobox", { name: "追加の技", exact: true }).click();
  await page.getByRole("listbox", { name: "追加の技", exact: true }).getByRole("option").nth(1).click();

  const total = section.getByText(/^合計 /);
  await expect(total).toBeVisible();
  const before = await total.innerText();
  const defender = section.getByRole("combobox", { name: "相手の振り", exact: true });
  await defender.selectOption({ index: 4 });
  await expect(total).not.toHaveText(before);
});
