// P4-6: アクセシビリティのスモーク(キーボード操作)。画面の切り替えタブは WAI-ARIA Authoring Practices の
// Tabs パターン(ロービング tabIndex・矢印キーで選択とフォーカスを移す automatic activation。App.tsx)。

import { expect, test } from "@playwright/test";
import { combobox, openApp } from "./support/calcPage.ts";

test("タブは矢印キー・Home・End で選択とフォーカスが移り、パネルの中身が入れ替わる", async ({ page }) => {
  await openApp(page);
  const calcTab = page.getByRole("tab", { name: "計算", exact: true });
  const reverseTab = page.getByRole("tab", { name: "逆算", exact: true });
  const balanceTab = page.getByRole("tab", { name: "タイプバランス", exact: true });
  const speedTab = page.getByRole("tab", { name: "素早さ", exact: true });
  // P5-5 PR-A1(ADR-0309 §1): タブの並びは 計算 → 逆算 → タイプバランス → 素早さ → 判定 → 構築
  // (6件、構築が末尾。JD5〈ADR-0705 §1〉の時点では判定が末尾だった)。
  const judgeTab = page.getByRole("tab", { name: "判定", exact: true });
  const teamTab = page.getByRole("tab", { name: "構築", exact: true });

  await calcTab.focus();
  await expect(calcTab).toHaveAttribute("aria-selected", "true");

  await page.keyboard.press("ArrowRight");
  await expect(reverseTab).toBeFocused();
  await expect(reverseTab).toHaveAttribute("aria-selected", "true");
  await expect(calcTab).toHaveAttribute("aria-selected", "false");
  await expect(page.getByRole("tabpanel", { name: "逆算", exact: true })).toBeVisible();
  await expect(combobox(page, "自分のポケモン")).toBeVisible();
  await expect(combobox(page, "攻撃側のポケモン")).toHaveCount(0);

  await page.keyboard.press("ArrowRight");
  await expect(balanceTab).toBeFocused();
  await expect(balanceTab).toHaveAttribute("aria-selected", "true");
  await expect(reverseTab).toHaveAttribute("aria-selected", "false");
  await expect(page.getByRole("group", { name: "メンバー1", exact: true })).toBeVisible();

  await page.keyboard.press("ArrowRight");
  await expect(speedTab).toBeFocused();
  await expect(speedTab).toHaveAttribute("aria-selected", "true");
  await expect(balanceTab).toHaveAttribute("aria-selected", "false");

  await page.keyboard.press("ArrowRight");
  await expect(judgeTab).toBeFocused();
  await expect(judgeTab).toHaveAttribute("aria-selected", "true");
  await expect(speedTab).toHaveAttribute("aria-selected", "false");

  // P5-5 PR-A1: 構築(末尾)。team-svc に届かなくても、新規作成の入力は使える(ADR-0309 §4)。
  await page.keyboard.press("ArrowRight");
  await expect(teamTab).toBeFocused();
  await expect(teamTab).toHaveAttribute("aria-selected", "true");
  await expect(judgeTab).toHaveAttribute("aria-selected", "false");
  await expect(page.getByRole("textbox", { name: "構築名", exact: true })).toBeVisible();

  // 末尾から ArrowRight で先頭へ回り込む。
  await page.keyboard.press("ArrowRight");
  await expect(calcTab).toBeFocused();
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();

  await page.keyboard.press("End");
  await expect(teamTab).toBeFocused();
  await page.keyboard.press("Home");
  await expect(calcTab).toBeFocused();
  // 先頭から ArrowLeft で末尾へ回り込む。
  await page.keyboard.press("ArrowLeft");
  await expect(teamTab).toBeFocused();
  await expect(teamTab).toHaveAttribute("aria-selected", "true");
});

test("Tab キーは選択中のタブにだけ止まる(ロービング tabIndex)", async ({ page }) => {
  await openApp(page);
  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("tabindex", "0");
  await expect(page.getByRole("tab", { name: "逆算", exact: true })).toHaveAttribute("tabindex", "-1");
});
