// P8-1c(ADR-0325): ポケモン画像の表示(vite preview + pokedex フィクスチャ。playwright.config.ts が走らせる)。
// 画像の配信(gateway の /images)はここでは持たないので、page.route で manifest と WebP を偽装する。
// 契約(ADR-0808): manifest は /images/manifest.json、画像 URL は /images/ + 相対パス。無ければタイプ色エンブレム。
// 画像なしで従来どおり動くこと(AC-X)は既存の spec 全体が担保し、ここでは「manifest が使えない」場合を明示する。
//   - preview・nginx(コンテナ)は /images/* を SPA のフォールバック(index.html の 200)で返すので、manifest として
//     読めず(JSON でない)エンブレムのままになり、コンソールエラーも出ない(container.spec.ts の CSP テストを壊さない)。
//   - gateway が JSON の 404 を返す環境では、ブラウザが「Failed to load resource」をコンソールに出すことがある
//     (fetch の失敗はアプリ側で握りつぶすが、ブラウザ自身のネットワークログは消せない)。この spec の 404 のテストは
//     それを除いて「アプリ起因のエラー(pageerror・alert)が無いこと」を確かめる。

import { expect, test, type Page } from "@playwright/test";
import { SPECIES, openAppOffline, selectMatchup } from "./support/calcPage.ts";

// 1x1 の有効な WebP(<img> が読み込みに成功したと判定できる最小の画像)。
const TINY_WEBP = Buffer.from("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==", "base64");

const MANIFEST = {
  version: 1,
  images: {
    [SPECIES.fire.key]: {
      thumb: `thumb/${SPECIES.fire.key}.aaaaaaaa.webp`,
      detail: `detail/${SPECIES.fire.key}.bbbbbbbb.webp`,
    },
  },
};

async function fakeImages(page: Page): Promise<void> {
  await page.route("**/images/manifest.json", (route) => route.fulfill({ status: 200, json: MANIFEST }));
  await page.route("**/images/**/*.webp", (route) =>
    route.fulfill({ status: 200, contentType: "image/webp", body: TINY_WEBP }),
  );
}

test("manifest にある種族のカードには画像(<img>)が出て、無い側はエンブレムのまま", async ({ page }) => {
  await fakeImages(page);
  await openAppOffline(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);

  const attacker = page.getByRole("region", { name: "攻撃側", exact: true });
  const defender = page.getByRole("region", { name: "防御側", exact: true });
  const img = attacker.locator("img");
  await expect(img).toHaveCount(1);
  await expect(img).toHaveAttribute("src", `/images/thumb/${SPECIES.fire.key}.aaaaaaaa.webp`);
  await expect(img).toHaveAttribute("loading", "lazy");
  await expect(img).toHaveAttribute("alt", "");
  // 実際に読み込めている(壊れていれば onError でエンブレムに戻る)。
  await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBeGreaterThan(0);
  await expect(attacker.getByTestId("type-emblem")).toHaveCount(0);
  await expect(defender.locator("img")).toHaveCount(0);
  await expect(defender.getByTestId("type-emblem")).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("manifest が 404 のままでも、エンブレムで従来どおり動き、alert もページエラーも出ない", async ({
  page,
}) => {
  const pageErrors: string[] = [];
  const consoleErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("console", (message) => {
    // ブラウザ自身の「Failed to load resource」(404 の取得ログ)はアプリでは消せないので数えない。
    if (message.type() === "error" && !message.text().startsWith("Failed to load resource")) {
      consoleErrors.push(message.text());
    }
  });
  await page.route("**/images/manifest.json", (route) =>
    route.fulfill({ status: 404, json: { error: "not_found" } }),
  );
  await openAppOffline(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);

  for (const name of ["攻撃側", "防御側"]) {
    const card = page.getByRole("region", { name, exact: true });
    await expect(card.getByTestId("type-emblem")).toHaveCount(1);
    await expect(card.locator("img")).toHaveCount(0);
  }
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(pageErrors).toEqual([]);
  expect(consoleErrors).toEqual([]);
});

test("manifest が HTML(SPA のフォールバック)でもコンソールエラー0・エンブレム(preview・コンテナの実際の挙動)", async ({
  page,
}) => {
  const problems: string[] = [];
  page.on("pageerror", (error) => problems.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    // openAppOffline の暖機(オンラインで1回計算する)で preview が返す /api/calc の 404 は画像と無関係なので数えない。
    if (message.type() === "error" && !message.location().url.includes("/api/")) {
      problems.push(`console: ${message.text()}`);
    }
  });
  // 何も偽装しない: preview は /images/manifest.json に index.html(200)を返す。
  await openAppOffline(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(
    page.getByRole("region", { name: "攻撃側", exact: true }).getByTestId("type-emblem"),
  ).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(problems).toEqual([]);
});
