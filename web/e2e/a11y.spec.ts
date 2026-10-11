// P4-6: アクセシビリティのスモーク(キーボード操作)。画面の切り替えタブは WAI-ARIA Authoring Practices の
// Tabs パターン(ロービング tabIndex・矢印キーで選択とフォーカスを移す automatic activation。App.tsx)。

import { expect, test } from "@playwright/test";
import { combobox, openApp } from "./support/calcPage.ts";
// ADR-0313: このファイルはタブ操作(キーボード)だけを見る。既定のオンラインで開いて pokedex フィクスチャのマスタを読む。

test("タブは矢印キー・Home・End で選択とフォーカスが移り、パネルの中身が入れ替わる(上位 4 区分 + ダメージの下位 3)", async ({
  page,
}) => {
  await openApp(page);
  // ADR-0356: 上位のタブは ダメージ → 構築 → 素早さ → 記録。ダメージの中の下位のタブは 計算 → 逆算 → 調整。
  // (判定は ADR-0330 で非表示、タイプバランスは ADR-0421 で構築へ統合。巡回の対象外)
  const calcTab = page.getByRole("tab", { name: "計算", exact: true });
  const reverseTab = page.getByRole("tab", { name: "逆算", exact: true });
  const adjustTab = page.getByRole("tab", { name: "調整", exact: true });
  const damageTab = page.getByRole("tab", { name: "ダメージ", exact: true });
  const teamTab = page.getByRole("tab", { name: "構築", exact: true });
  const speedTab = page.getByRole("tab", { name: "素早さ", exact: true });
  const recordsTab = page.getByRole("tab", { name: "記録", exact: true });

  // 下位のタブ(ダメージの種類)。
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
  await expect(adjustTab).toBeFocused();
  await expect(adjustTab).toHaveAttribute("aria-selected", "true");
  // 末尾から ArrowRight で先頭へ回り込む。
  await page.keyboard.press("ArrowRight");
  await expect(calcTab).toBeFocused();
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();
  await page.keyboard.press("End");
  await expect(adjustTab).toBeFocused();
  await page.keyboard.press("Home");
  await expect(calcTab).toBeFocused();

  // 上位のタブ(区分)。
  await damageTab.focus();
  await expect(damageTab).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("ArrowRight");
  await expect(teamTab).toBeFocused();
  await expect(teamTab).toHaveAttribute("aria-selected", "true");
  await expect(damageTab).toHaveAttribute("aria-selected", "false");
  // 構築: team-svc に届かなくても、[新しい構築]は押せる(ADR-0309 §4)。
  await expect(page.getByRole("button", { name: "新しい構築", exact: true })).toBeVisible();

  await page.keyboard.press("ArrowRight");
  await expect(speedTab).toBeFocused();
  await expect(speedTab).toHaveAttribute("aria-selected", "true");
  await expect(teamTab).toHaveAttribute("aria-selected", "false");

  // 記録(お気に入り)。タブを選ぶだけで record API を呼んでよい(失敗は画面内の alert に留まる)。
  await page.keyboard.press("ArrowRight");
  await expect(recordsTab).toBeFocused();
  await expect(recordsTab).toHaveAttribute("aria-selected", "true");
  await expect(speedTab).toHaveAttribute("aria-selected", "false");

  // 末尾から ArrowRight で先頭へ回り込む。ダメージに戻ると、最後に見ていた画面(計算)が出る。
  await page.keyboard.press("ArrowRight");
  await expect(damageTab).toBeFocused();
  await expect(damageTab).toHaveAttribute("aria-selected", "true");
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();

  await page.keyboard.press("End");
  await expect(recordsTab).toBeFocused();
  await page.keyboard.press("Home");
  await expect(damageTab).toBeFocused();
  // 先頭から ArrowLeft で末尾へ回り込む。
  await page.keyboard.press("ArrowLeft");
  await expect(recordsTab).toBeFocused();
  await expect(recordsTab).toHaveAttribute("aria-selected", "true");
});

test("Tab キーは選択中のタブにだけ止まる(ロービング tabIndex)", async ({ page }) => {
  await openApp(page);
  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("tabindex", "0");
  await expect(page.getByRole("tab", { name: "逆算", exact: true })).toHaveAttribute("tabindex", "-1");
});
