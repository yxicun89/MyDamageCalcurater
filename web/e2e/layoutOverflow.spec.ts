// F-12 PR-2(I-web-12、ADR-0336 §5): 共通部品(カード・チップ・表・ボタン)を全画面に当てたあとも、375px 幅で横に溢れない。
// 計算・素早さ・構築(PR-1)と、逆算・タイプバランス・お気に入り・調整・このアプリについて(PR-2)を、ページの
// scrollWidth <= clientWidth(横スクロールが出ない)で見る。mobile.spec.ts(issue #98。個別の部品の位置)とは別に、
// 画面ごとの一括の回帰として置く。ライト・ダークの両方。要素はアクセシブルな名前で引き、クラスには頼らない。

import { expect, test, type Page } from "@playwright/test";
import { SPECIES, openApp, selectMatchup, selectReverseMatchup } from "./support/calcPage.ts";

const VIEWPORT = { width: 375, height: 800 } as const;

const SCREENS = [
  { path: "/calc", ready: { role: "tab", name: "計算" } },
  { path: "/speed", ready: { role: "tab", name: "素早さ" } },
  { path: "/team", ready: { role: "tab", name: "構築" } },
  { path: "/reverse", ready: { role: "tab", name: "逆算" } },
  { path: "/balance", ready: { role: "tab", name: "タイプバランス" } },
  { path: "/favorites", ready: { role: "tab", name: "お気に入り" } },
  { path: "/adjust", ready: { role: "tab", name: "調整" } },
] as const;

async function expectNoHorizontalOverflow(page: Page, label: string): Promise<void> {
  const { scrollWidth, clientWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(scrollWidth, `${label} が ${String(VIEWPORT.width)}px で横に溢れている`).toBeLessThanOrEqual(
    clientWidth,
  );
}

const TYPE_IDS = [
  "normal",
  "fire",
  "water",
  "electric",
  "grass",
  "ice",
  "fighting",
  "poison",
  "ground",
  "flying",
  "psychic",
  "bug",
  "rock",
  "ghost",
  "dragon",
  "dark",
  "steel",
  "fairy",
] as const;

/** 既定の設定(balance-svc・pokedex フィクスチャなし)でも結果の表まで出せるよう、必要な API を fake で返す。 */
async function routeBalanceFakes(page: Page): Promise<void> {
  const json = (body: unknown) => ({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
  const fire = { key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストほのお", types: ["fire"] };
  await page.route("**/api/pokedex/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/pokedex/species") {
      await route.fulfill(json([fire]));
    } else if (path.startsWith("/api/pokedex/species/")) {
      await route.fulfill(
        json({
          ...fire,
          baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 110 },
          abilities: [],
          learnset: [],
          isMega: false,
          requiredItemId: null,
        }),
      );
    } else {
      await route.fulfill(json([]));
    }
  });
  await page.route("**/api/balance/**", async (route) => {
    await route.fulfill(
      json({
        members: [
          {
            pokemonId: fire.key,
            types: ["fire"],
            defense: TYPE_IDS.map((attackType) => ({
              attackType,
              multiplier: "1",
              category: "neutral",
              source: "type",
              effect: "none",
            })),
          },
        ],
        teamSummary: TYPE_IDS.map((attackType) => ({
          attackType,
          weak: 0,
          quadWeak: 0,
          resist: 0,
          immune: 0,
          neutral: 1,
        })),
      }),
    );
  });
}

for (const scheme of ["light", "dark"] as const) {
  test.describe(`375px・${scheme} テーマ`, () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize(VIEWPORT);
      await page.emulateMedia({ colorScheme: scheme });
    });

    for (const { path, ready } of SCREENS) {
      test(`${path} が横に溢れない`, async ({ page }) => {
        await page.goto(path);
        await expect(page.getByRole(ready.role, { name: ready.name, exact: true })).toHaveAttribute(
          "aria-selected",
          "true",
        );
        await expectNoHorizontalOverflow(page, path);
      });
    }

    test("/about が横に溢れない(出典リスト・データの扱い・削除確認ダイアログ)", async ({ page }) => {
      await page.goto("/about");
      await expect(
        page.getByRole("heading", { level: 2, name: "このアプリについて", exact: true }),
      ).toBeVisible();
      await expectNoHorizontalOverflow(page, "/about");
      const deleteButton = page.getByRole("button", { name: "この端末のデータを削除" });
      if (await deleteButton.isVisible()) {
        await deleteButton.click();
        await expect(page.getByRole("alertdialog")).toBeVisible();
        await expectNoHorizontalOverflow(page, "/about(削除確認ダイアログ)");
      }
    });

    test("種族を選んだ計算・逆算(タイプ色のカード・結果の行)が横に溢れない", async ({ page }) => {
      await openApp(page);
      await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
      await expectNoHorizontalOverflow(page, "/calc(種族を選択)");
      await page.goto("/reverse");
      await selectReverseMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
      await expectNoHorizontalOverflow(page, "/reverse(種族を選択)");
    });

    test("タイプバランスの結果(防御相性の表は 18 列)が 375px で横に溢れない(表は包みの中でスクロール)", async ({
      page,
    }) => {
      await routeBalanceFakes(page);
      await page.goto("/balance");
      await expect(page.getByRole("tab", { name: "タイプバランス", exact: true })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      const member = page.getByRole("group", { name: "メンバー1", exact: true });
      await member.getByRole("combobox", { name: "ポケモン", exact: true }).fill("テストほのお");
      await member
        .getByRole("listbox", { name: "ポケモン", exact: true })
        .getByRole("option", { name: "テストほのお", exact: true })
        .click();
      const table = page.getByRole("table", { name: "防御相性", exact: true }).first();
      await expect(table).toBeVisible();
      await expectNoHorizontalOverflow(page, "/balance(結果の表)");
    });

    test("調整の各モード(耐久・攻撃・最小の振り方。モードのチップ・相手の領域)が横に溢れない", async ({
      page,
    }) => {
      await page.goto("/adjust");
      const group = page.getByRole("radiogroup", { name: "調整の内容", exact: true });
      for (const mode of [
        "耐久に振る",
        "攻撃と素早さに振る",
        "倒せるいちばん少ない振り方",
        "耐えられるいちばん少ない振り方",
      ]) {
        await group.getByText(mode, { exact: true }).click();
        await expect(group.getByRole("radio", { name: mode, exact: true })).toBeChecked();
        await expectNoHorizontalOverflow(page, `/adjust(${mode})`);
      }
    });
  });
}
