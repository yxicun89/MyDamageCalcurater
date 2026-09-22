// P4-10: URL で画面を切り替える。本番ビルド(vite preview)を相手に、直接開く・タブで URL が変わる・
// ブラウザの戻る/進む・/ から /calc への置き換えを確かめる。/reverse を直接開けること自体が、preview が
// 未知のパスに index.html を返す(SPA のフォールバック)ことの確認を兼ねる。

import { expect, test, type Page } from "@playwright/test";

const TABLIST_NAME = "画面の切り替え";

function tab(page: Page, name: string) {
  return page.getByRole("tab", { name, exact: true });
}

async function waitForTabs(page: Page): Promise<void> {
  await expect(page.getByRole("tablist", { name: TABLIST_NAME })).toBeVisible();
}

function pathOf(page: Page): string {
  return new URL(page.url()).pathname;
}

test("/reverse を直接開くと逆算タブが選択され、逆算画面が出る(SPA のフォールバック)", async ({ page }) => {
  const response = await page.goto("/reverse");
  expect(response?.status()).toBe(200);
  await waitForTabs(page);
  await expect(tab(page, "逆算")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("combobox", { name: "自分のポケモン", exact: true })).toBeVisible();
  await expect(page).toHaveTitle("逆算 | pokecalc");
  expect(pathOf(page)).toBe("/reverse");
});

test("タブで URL が変わり、戻る・進むでタブが切り替わる", async ({ page }) => {
  await page.goto("/reverse");
  await waitForTabs(page);

  await tab(page, "計算").click();
  await expect(page).toHaveURL(/\/calc$/);
  await expect(tab(page, "計算")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true })).toBeVisible();
  await expect(page).toHaveTitle("計算 | pokecalc");

  await page.goBack();
  await expect(page).toHaveURL(/\/reverse$/);
  await expect(tab(page, "逆算")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("combobox", { name: "自分のポケモン", exact: true })).toBeVisible();

  await page.goForward();
  await expect(page).toHaveURL(/\/calc$/);
  await expect(tab(page, "計算")).toHaveAttribute("aria-selected", "true");
});

test("/ を開くと /calc に置き換わり、戻る履歴に / が残らない", async ({ page }) => {
  await page.goto("/calc");
  await waitForTabs(page);
  const lengthBefore = await page.evaluate(() => window.history.length);
  await page.goto("/");
  await waitForTabs(page);
  await expect(page).toHaveURL(/\/calc$/);
  await expect(tab(page, "計算")).toHaveAttribute("aria-selected", "true");

  // replaceState なので、/ を開いた分の1件だけ増える(pushState なら2件増える)。
  expect(await page.evaluate(() => window.history.length)).toBe(lengthBefore + 1);
});

test("未知のパスを開くと /calc に置き換わる", async ({ page }) => {
  await page.goto("/no-such-screen");
  await waitForTabs(page);
  await expect(page).toHaveURL(/\/calc$/);
  await expect(tab(page, "計算")).toHaveAttribute("aria-selected", "true");
});
