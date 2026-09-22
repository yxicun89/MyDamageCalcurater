// P4-6: アクセシビリティのスモーク(キーボード操作)。画面の切り替えタブは WAI-ARIA Authoring Practices の
// Tabs パターン(ロービング tabIndex・矢印キーで選択とフォーカスを移す automatic activation。App.tsx)。

import { expect, test } from "@playwright/test";
import { combobox, openApp } from "./support/calcPage.ts";

test("タブは矢印キー・Home・End で選択とフォーカスが移り、パネルの中身が入れ替わる", async ({ page }) => {
  await openApp(page);
  const calcTab = page.getByRole("tab", { name: "計算", exact: true });
  const reverseTab = page.getByRole("tab", { name: "逆算", exact: true });

  await calcTab.focus();
  await expect(calcTab).toHaveAttribute("aria-selected", "true");

  await page.keyboard.press("ArrowRight");
  await expect(reverseTab).toBeFocused();
  await expect(reverseTab).toHaveAttribute("aria-selected", "true");
  await expect(calcTab).toHaveAttribute("aria-selected", "false");
  await expect(page.getByRole("tabpanel", { name: "逆算", exact: true })).toBeVisible();
  await expect(combobox(page, "自分のポケモン")).toBeVisible();
  await expect(combobox(page, "攻撃側のポケモン")).toHaveCount(0);

  // 末尾から ArrowRight で先頭へ回り込む。
  await page.keyboard.press("ArrowRight");
  await expect(calcTab).toBeFocused();
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();

  await page.keyboard.press("End");
  await expect(reverseTab).toBeFocused();
  await page.keyboard.press("Home");
  await expect(calcTab).toBeFocused();
  await page.keyboard.press("ArrowLeft");
  await expect(reverseTab).toBeFocused();
  await expect(reverseTab).toHaveAttribute("aria-selected", "true");
});

test("Tab キーは選択中のタブにだけ止まる(ロービング tabIndex)", async ({ page }) => {
  await openApp(page);
  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("tabindex", "0");
  await expect(page.getByRole("tab", { name: "逆算", exact: true })).toHaveAttribute("tabindex", "-1");
});
