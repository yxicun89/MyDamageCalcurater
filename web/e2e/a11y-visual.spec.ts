// F-12(I-web-6、ADR-0334): ポップ・カラフルな見た目の基盤を当てた画面の a11y と「動き」の回帰。
// - axe(WCAG 2.x A/AA。色のコントラストを含む)で、計算・素早さ・構築をライト/ダークの両方で 0 件にする
//   (計算はタイプ色のカード〈種族を選んだ状態〉も見る)。
// - 常時動くアニメーションが無い(無限に繰り返す Animation が document に無い)。
// - 「視差効果を減らす」(prefers-reduced-motion: reduce)では、ボタン・タブの transition が 0 秒になる。
// a11y.spec.ts(キーボード操作)・a11y-about.spec.ts(/about)は変えずに残す。見た目の差分の画像比較は入れない
// (不安定なため。画面の撮り方は ADR-0334「確認の手順」)。

import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { SPECIES, openApp, selectMatchup } from "./support/calcPage.ts";

const WCAG_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

/** 主要画面(今回の PR で共通部品を当てる画面)。tab の名前と、開いたことの目印。 */
const SCREENS = [
  { path: "/calc", tab: "計算" },
  { path: "/speed", tab: "素早さ" },
  { path: "/team", tab: "構築" },
] as const;

async function openScreen(page: Page, path: string, tab: string): Promise<void> {
  await page.goto(path);
  await expect(page.getByRole("tab", { name: tab, exact: true })).toHaveAttribute("aria-selected", "true");
}

async function expectNoAxeViolations(page: Page): Promise<void> {
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
  expect(results.violations).toEqual([]);
}

for (const scheme of ["light", "dark"] as const) {
  test.describe(`${scheme} テーマ`, () => {
    test.beforeEach(async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
    });

    for (const { path, tab } of SCREENS) {
      test(`${path} に axe の違反が無い`, async ({ page }) => {
        await openScreen(page, path, tab);
        await expectNoAxeViolations(page);
      });
    }

    test("タイプ色のカード(種族を選んだ計算画面)に axe の違反が無い", async ({ page }) => {
      await openApp(page);
      await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
      await expect(
        page.getByRole("region", { name: "攻撃側" }).getByRole("heading", { level: 3 }),
      ).toHaveText(SPECIES.fire.nameJa);
      await expectNoAxeViolations(page);
    });
  });
}

test("タイプ色のカード: 選んだ種族で攻撃側カードの --card-type が変わる", async ({ page }) => {
  await openApp(page);
  const card = page.getByRole("region", { name: "攻撃側" });
  await expect(card).toHaveClass(/\bui-card--typed\b/);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const value = await card.evaluate((element) =>
    (element as HTMLElement).style.getPropertyValue("--card-type"),
  );
  expect(value).toMatch(/^var\(--type-[a-z0-9-]+, var\(--brand-primary\)\)$/);
});

test("どの主要画面にも、無限に繰り返すアニメーションが無い(常時動くものを置かない)", async ({ page }) => {
  for (const { path, tab } of SCREENS) {
    await openScreen(page, path, tab);
    const infinite = await page.evaluate(
      () =>
        document
          .getAnimations()
          .filter((animation) => animation.effect?.getComputedTiming().iterations === Infinity).length,
    );
    expect(infinite, `${path} に無限のアニメーションがある`).toBe(0);
  }
});

test("視差効果を減らす設定では、ボタン・タブの transition が 0 秒", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openApp(page);
  const durations = await page.evaluate(() =>
    [...document.querySelectorAll(".ui-button, .ui-tab, .ui-chip")].map(
      (element) => getComputedStyle(element).transitionDuration,
    ),
  );
  expect(durations.length).toBeGreaterThan(0);
  for (const duration of durations) {
    expect(duration.split(",").every((part) => parseFloat(part) === 0)).toBe(true);
  }
});

test("通常の設定では、ボタン・タブに押下・選択の短い transition がある(操作のときだけ動く)", async ({
  page,
}) => {
  await openApp(page);
  const tab = page.getByRole("tab", { name: "計算", exact: true });
  const duration = await tab.evaluate((element) => getComputedStyle(element).transitionDuration);
  expect(duration.split(",").some((part) => parseFloat(part) > 0)).toBe(true);
});
