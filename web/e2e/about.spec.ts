// issue #328(P6-18、Web 分。ADR-0314): 「このアプリについて」(/about)。本番ビルド(vite preview)で、
// フッターのリンクから開く・直接開ける(SPA のフォールバック)・非公式の注記とデータの出典4件・戻る導線・
// ブラウザの戻る/進む・他タブの入力が残ることを確かめる。文言の正は src/i18n/ja.ts(aboutText)で、
// ここはあえてリテラル(DECISIONS.md 2026-09-26「P6-18」の確定文言)で書く。

import { expect, test } from "@playwright/test";
import { SPECIES, combobox, openApp, selectSpeciesBySearch } from "./support/calcPage.ts";

const NOTICE =
  "このアプリは個人が私的に使うための非公式ツールです。" +
  "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。" +
  "ポケモン・Pokémon および関連する名称は各社の商標です。";

test("フッターのリンクから /about を開き、非公式の注記とデータの出典4件が出る", async ({ page }) => {
  await openApp(page);
  await page.getByRole("contentinfo").getByRole("link", { name: "このアプリについて", exact: true }).click();

  await expect(page).toHaveURL(/\/about$/);
  await expect(page).toHaveTitle("このアプリについて | pokecalc");
  await expect(
    page.getByRole("heading", { level: 2, name: "このアプリについて", exact: true }),
  ).toBeFocused();
  await expect(page.getByText(NOTICE, { exact: true })).toBeVisible();
  const items = page.getByRole("list", { name: "データの出典", exact: true }).getByRole("listitem");
  await expect(items).toHaveCount(4);
  await expect(items.nth(0)).toContainText("@smogon/calc(MIT License)");
  await expect(items.nth(1)).toContainText("Pokémon Showdown(MIT License)");
  await expect(items.nth(2)).toContainText("PokeAPI");
  await expect(items.nth(2)).not.toContainText("License");
  await expect(items.nth(3)).toContainText("Pokémon HOME・Pokémon Champions の公式情報");
  // タブには入らない。
  await expect(page.getByRole("tab", { name: "このアプリについて" })).toHaveCount(0);
});

test("/about を直接開ける(SPA のフォールバック)。URL は /calc に置き換わらない", async ({ page }) => {
  const response = await page.goto("/about");
  expect(response?.status()).toBe(200);
  await expect(page.getByText(NOTICE, { exact: true })).toBeVisible();
  expect(new URL(page.url()).pathname).toBe("/about");
});

test("「計算に戻る」と、ブラウザの戻る・進む。計算・逆算の入力は往復しても残る", async ({ page }) => {
  await openApp(page);
  const attacker = combobox(page, "攻撃側のポケモン");
  // ADR-0313: 既定がオンラインなので、種族は検索欄で選ぶ。
  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.fire.nameJa);
  const selected = await attacker.inputValue();
  await page.getByRole("tab", { name: "逆算", exact: true }).click();
  const observation = page.getByRole("textbox", { name: "観測1", exact: true });
  await observation.fill("45");

  await page.getByRole("contentinfo").getByRole("link", { name: "このアプリについて", exact: true }).click();
  await expect(page.getByText(NOTICE, { exact: true })).toBeVisible();
  // 隠れているだけで DOM には残る。
  await expect(page.locator('input[aria-label="攻撃側のポケモン"]')).toHaveCount(1);

  // ブラウザの戻る → 逆算(入力が残る)→ 進む → 情報ページ。
  await page.goBack();
  await expect(page).toHaveURL(/\/reverse$/);
  await expect(observation).toHaveValue("45");
  await page.goForward();
  await expect(page).toHaveURL(/\/about$/);
  await expect(page.getByText(NOTICE, { exact: true })).toBeVisible();

  // 戻る導線。
  await page.getByRole("link", { name: "計算に戻る", exact: true }).click();
  await expect(page).toHaveURL(/\/calc$/);
  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("aria-selected", "true");
  await expect(attacker).toHaveValue(selected);
});

test("キーボードだけで、フッターのリンクで開き「計算に戻る」で戻れる", async ({ page }) => {
  await openApp(page);
  await page.getByRole("contentinfo").getByRole("link", { name: "このアプリについて", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/about$/);

  await page.getByRole("link", { name: "計算に戻る", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/calc$/);
});

test("未知のパス(/about/extra)は /calc に置き換わる", async ({ page }) => {
  await page.goto("/about/extra");
  await expect(page).toHaveURL(/\/calc$/);
  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("aria-selected", "true");
});
