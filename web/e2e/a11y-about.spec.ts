// issue #328(P6-18、Web 分。ADR-0314): 情報ページ(/about)とフッターの a11y。axe(WCAG 2.x A/AA)で違反が0件であること。
// 依存: @axe-core/playwright(devDependencies に最新安定版を完全固定で追加する。implementer の作業。
// 未導入の間はこのファイルの import が失敗して赤になる)。
// a11y.spec.ts(既存のキーボード操作のスモーク)と分けたのは、新しい依存が無い間も既存 spec を壊さないため。

import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { openApp } from "./support/calcPage.ts";

const WCAG_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

test("/about に axe の違反が無い", async ({ page }) => {
  await page.goto("/about");
  await expect(
    page.getByRole("heading", { level: 2, name: "このアプリについて", exact: true }),
  ).toBeVisible();
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
  expect(results.violations).toEqual([]);
});

test("フッターを足した計算画面(/calc)にも axe の違反が無い(ランドマーク・リンクの名前・色のコントラスト)", async ({
  page,
}) => {
  await openApp(page);
  await expect(
    page.getByRole("contentinfo").getByRole("link", { name: "このアプリについて", exact: true }),
  ).toBeVisible();
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
  expect(results.violations).toEqual([]);
});

test("ダークモードの /about にも axe の違反が無い(色のコントラスト)", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/about");
  await expect(
    page.getByRole("heading", { level: 2, name: "このアプリについて", exact: true }),
  ).toBeVisible();
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
  expect(results.violations).toEqual([]);
});
