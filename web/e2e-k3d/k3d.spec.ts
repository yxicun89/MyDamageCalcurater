// 利用者がブラウザで開く入口(k3d の localhost:8080)で、オフライン(WASM)とオンライン(API・実マスタ)の計算が
// 画面に結果を出すことを確かめる。バックエンドの疎通(api-smoke)だけでは画面の不具合(白画面 #268、
// 古い pokedex による技の一括取得 404)を見逃したため、画面の操作で確かめる。
import { expect, test, type Page } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  combobox,
  openApp,
  selectMatchup,
} from "../e2e/support/calcPage.ts";

const HEADERS = {
  "X-Device-Id": "00000000-0000-4000-8000-00000000e2e1",
  "X-Session-Id": "00000000-0000-4000-8000-00000000e2e2",
};

function trackFailures(page: Page): string[] {
  const failures: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 400) {
      failures.push(`${response.status()} ${new URL(response.url()).pathname}`);
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

test("オフライン(WASM): 画面が描画され、計算結果が出る", async ({ page }) => {
  const failures = trackFailures(page);
  await openApp(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(calcRows(page).first()).toContainText(PERCENT_RANGE_PATTERN);
  expect(failures).toEqual([]);
});

test("オンライン(API): 実マスタのポケモンを選ぶと計算結果が出る", async ({ page, request }) => {
  // 実マスタの名前をテストに書かない(CLAUDE.md)。公開 API から先頭の種族を引き、その名前の先頭1文字で検索する。
  const species = await request.get("/api/pokedex/species?limit=1", { headers: HEADERS });
  expect(species.status(), "pokedex が応答しない(make import-k8s 済みか)").toBe(200);
  const [first] = (await species.json()) as { nameJa: string }[];
  expect(first?.nameJa, "pokedex にマスタが入っていない(make import-k8s)").toBeTruthy();
  const prefix = (first?.nameJa ?? "").slice(0, 1);

  const failures = trackFailures(page);
  await openApp(page);
  await chooseRadio(page, "計算モード", "オンライン(API)");
  await pickFirstCandidate(page, "攻撃側のポケモン", prefix);
  await pickFirstCandidate(page, "防御側のポケモン", prefix);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(calcRows(page).first()).toContainText(PERCENT_RANGE_PATTERN);
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(failures).toEqual([]);
});
