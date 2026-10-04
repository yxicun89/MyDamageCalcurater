// 利用者がブラウザで開く入口(k3d の localhost:8080)で、この端末(オフライン)とオンライン(API・実マスタ)の計算が
// 画面に結果を出すことを確かめる。バックエンドの疎通(api-smoke)だけでは画面の不具合(白画面 #268、
// 古い pokedex による技の一括取得 404)を見逃したため、画面の操作で確かめる。
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  PERCENT_RANGE_PATTERN,
  calcRows,
  chooseRadio,
  combobox,
  openApp,
} from "../e2e/support/calcPage.ts";
import { isCountedResponseFailure } from "../e2e/support/expectedFailures.ts";

const HEADERS = {
  "X-Device-Id": "00000000-0000-4000-8000-00000000e2e1",
  "X-Session-Id": "00000000-0000-4000-8000-00000000e2e2",
};

function trackFailures(page: Page): string[] {
  const failures: string[] = [];
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    // /images/manifest.json の 404 だけは正常(画像なし = エンブレム。ADR-0808)。許容は expectedFailures.ts に狭く置く。
    if (isCountedResponseFailure(response.status(), pathname)) {
      failures.push(`${response.status()} ${pathname}`);
    }
  });
  page.on("pageerror", (error) => failures.push(`pageerror ${error.message}`));
  return failures;
}

async function pickFirstCandidate(page: Page, name: string, prefix: string): Promise<void> {
  const field = combobox(page, name);
  const searched = page.waitForResponse(
    (response) => new URL(response.url()).pathname === "/api/pokedex/species" && response.status() === 200,
  );
  await field.pressSequentially(prefix);
  await searched;
  await expect(field).toHaveAttribute("aria-expanded", "true");
  await field.press("ArrowDown");
  await field.press("Enter");
}

/** 実マスタの先頭の種族の名前の先頭1文字(実マスタの名前をテストに書かない。CLAUDE.md)。 */
async function firstSpeciesPrefix(request: APIRequestContext): Promise<string> {
  const species = await request.get("/api/pokedex/species?limit=1", { headers: HEADERS });
  expect(species.status(), "pokedex が応答しない(make import-k8s 済みか)").toBe(200);
  const [first] = (await species.json()) as { nameJa: string }[];
  expect(first?.nameJa, "pokedex にマスタが入っていない(make import-k8s)").toBeTruthy();
  return (first?.nameJa ?? "").slice(0, 1);
}

/** オフライン(キャッシュ済みの種族だけが候補)で、検索欄の先頭の候補を選ぶ。 */
async function pickFirstCachedCandidate(page: Page, name: string, prefix: string): Promise<void> {
  const field = combobox(page, name);
  await expect(field).toHaveAttribute("aria-expanded", /^(true|false)$/);
  await field.pressSequentially(prefix);
  await expect(field).toHaveAttribute("aria-expanded", "true");
  await field.press("ArrowDown");
  await field.press("Enter");
}

// ADR-0313: 既定はオンライン。一度オンラインで引いた実マスタの種族を IndexedDB に保存し、この端末(オフライン)で計算する。
test("この端末(オフライン): オンラインで引いた実マスタの種族で、キャッシュから計算結果が出る", async ({
  page,
  request,
}) => {
  const prefix = await firstSpeciesPrefix(request);
  const failures = trackFailures(page);
  await openApp(page);
  await pickFirstCandidate(page, "攻撃側のポケモン", prefix);
  await pickFirstCandidate(page, "防御側のポケモン", prefix);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);

  await chooseRadio(page, "計算する場所", "この端末(オフライン)");
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await pickFirstCachedCandidate(page, "攻撃側のポケモン", prefix);
  await pickFirstCachedCandidate(page, "防御側のポケモン", prefix);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(calcRows(page).first()).toContainText(PERCENT_RANGE_PATTERN);
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(failures).toEqual([]);
});

test("サーバー(オンライン): 実マスタのポケモンを選ぶと計算結果が出る", async ({ page, request }) => {
  const prefix = await firstSpeciesPrefix(request);

  const failures = trackFailures(page);
  await openApp(page);
  await chooseRadio(page, "計算する場所", "サーバー(オンライン)");
  await pickFirstCandidate(page, "攻撃側のポケモン", prefix);
  await pickFirstCandidate(page, "防御側のポケモン", prefix);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(calcRows(page).first()).toContainText(PERCENT_RANGE_PATTERN);
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(failures).toEqual([]);
});
