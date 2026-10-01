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

// issue #218(ADR-0308): タブを往復しても各画面の入力が消えない。
// 実ブラウザでだけ確かめられること:
//   - 非選択の画面が本当に DOM に残っていること(`page.locator` の toHaveCount は見た目に関係なく
//     DOM を数える。一方 getByRole は hidden の要素を除くので、支援技術から見えないことも同時に分かる)
//   - 実際の戻る/進む(page.goBack / goForward。jsdom での popstate の模倣ではない)
// 実装後に見るのは「逆算タブにいる間も攻撃側の select が DOM に1つあり、role では取れない」ことと、
// 「計算タブに戻ると選んだ値がそのまま残っている」こと。
test("タブを往復しても計算画面の入力が残り、非選択の間も DOM から消えない", async ({ page }) => {
  await page.goto("/calc");
  await waitForTabs(page);
  const attacker = page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true });
  // 先頭の option はプレースホルダ(hidden)なので、その次(最初の種族)を選ぶ。
  await attacker.selectOption({ index: 1 });
  const selected = await attacker.inputValue();
  expect(selected).not.toBe("");

  await tab(page, "逆算").click();
  await expect(page.getByRole("combobox", { name: "自分のポケモン", exact: true })).toBeVisible();
  // 隠れているだけで DOM には残る(= unmount されていない)。支援技術からは見えない。
  await expect(page.locator('select[aria-label="攻撃側のポケモン"]')).toHaveCount(1);
  await expect(attacker).toBeHidden();

  await tab(page, "計算").click();
  await expect(attacker).toHaveValue(selected);
});

test("戻る・進むでタブが切り替わっても、それぞれの画面の入力が残る", async ({ page }) => {
  await page.goto("/calc");
  await waitForTabs(page);
  const attacker = page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true });
  await attacker.selectOption({ index: 1 });
  const selected = await attacker.inputValue();

  await tab(page, "逆算").click();
  const observation = page.getByRole("textbox", { name: "観測1", exact: true });
  await observation.fill("45");

  await page.goBack();
  await expect(page).toHaveURL(/\/calc$/);
  await expect(attacker).toHaveValue(selected);

  await page.goForward();
  await expect(page).toHaveURL(/\/reverse$/);
  await expect(observation).toHaveValue("45");
});

// SP3(ADR-0604 §2): 素早さ比較のタブ。/speed を直接開ける(SPA のフォールバック)。
// speed-svc はこの構成では動いていないので、API は失敗するが画面(右の自分の入力)は出る(ADR-0604 §4)。
test("/speed を直接開くと素早さタブが選択され、自分のポケモンの入力が出る", async ({ page }) => {
  const response = await page.goto("/speed");
  expect(response?.status()).toBe(200);
  await waitForTabs(page);
  await expect(tab(page, "素早さ")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("region", { name: "自分のポケモン", exact: true })).toBeVisible();
  await expect(page).toHaveTitle("素早さ | pokecalc");
  expect(pathOf(page)).toBe("/speed");
});
