// P4-12a: タイプバランスの画面の主な流れ(ADR-0303)。playwright.balance.config.ts が balance-svc を Web の例データの
// read model で起動し、vite preview が /api/balance を転送する。倍率・集計は本物の balance-svc の計算。
// 例データ: テストほのお(9001-000・ほのお)・テストみず(9002-000・みず)、技 テストかえんパンチ(ほのお・物理)。

import { expect, test, type Locator, type Page } from "@playwright/test";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const FIRE = { key: "9001-000", nameJa: "テストほのお" } as const;
const WATER = { key: "9002-000", nameJa: "テストみず" } as const;
const FIRE_PUNCH = "テストかえんパンチ";

/** 倍率のセルの書式(「×2 弱点」「×1/2 耐性」など。docs/type-balance-design.md §10)。 */
const DEFENSE_LABEL = /^×(0|[1-9][0-9]*(\/[1-9][0-9]*)?) (弱点|等倍|耐性|無効)$/;

function member(page: Page, index: number): Locator {
  return page.getByRole("group", { name: `メンバー${String(index)}`, exact: true });
}

async function openBalance(page: Page): Promise<void> {
  await page.goto("/balance");
  await expect(page.getByRole("tab", { name: "タイプバランス", exact: true })).toHaveAttribute(
    "aria-selected",
    "true",
  );
}

/** 表の中で、行見出しが name の行のセル(見出し以外)。 */
function rowCells(table: Locator, name: string): Locator {
  return table
    .getByRole("row")
    .filter({ has: table.page().getByRole("rowheader", { name, exact: true }) })
    .getByRole("cell");
}

test("2体を選ぶと analyze(200)で防御相性の表(18タイプ・文字の倍率)とチームの集計を出す", async ({ page }) => {
  await openBalance(page);

  const analyzeResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/balance/v1/team-balance/analyze" &&
      response.request().method() === "POST",
  );
  await member(page, 1).getByRole("combobox", { name: "ポケモン", exact: true }).selectOption(FIRE.key);
  await page.getByRole("button", { name: "メンバーを追加", exact: true }).click();
  await member(page, 2).getByRole("combobox", { name: "ポケモン", exact: true }).selectOption(WATER.key);
  const response = await analyzeResponse;
  expect(response.status()).toBe(200);
  // 端末 ID・セッション ID を付ける(ADR-0303 §5)。
  const headers = response.request().headers();
  expect(headers["x-device-id"]).toMatch(UUID_PATTERN);
  expect(headers["x-session-id"]).toMatch(UUID_PATTERN);

  const defense = page.getByRole("table", { name: "防御相性", exact: true });
  await expect(rowCells(defense, WATER.nameJa)).toHaveCount(18);
  // 列見出しは「メンバー」+ 18タイプ。
  await expect(defense.getByRole("columnheader")).toHaveCount(19);
  const fireCells = rowCells(defense, FIRE.nameJa);
  await expect(fireCells).toHaveCount(18);
  for (const text of await fireCells.allTextContents()) {
    expect(text.trim()).toMatch(DEFENSE_LABEL);
  }
  // ほのおタイプは みず技が ×2 弱点・くさ技が ×1/2 耐性(列の位置は見出しから引く)。
  const headersText = (await defense.getByRole("columnheader").allTextContents()).map((text) => text.trim());
  const cellFor = (cells: Locator, typeName: string) => cells.nth(headersText.indexOf(typeName) - 1);
  await expect(cellFor(fireCells, "みず")).toHaveText("×2 弱点");
  await expect(cellFor(fireCells, "くさ")).toHaveText("×1/2 耐性");
  await expect(cellFor(rowCells(defense, WATER.nameJa), "でんき")).toHaveText("×2 弱点");

  // チームの集計: みず技は テストほのお が弱点・テストみず が耐性。
  const summary = page.getByRole("table", { name: "チームの集計", exact: true });
  await expect(rowCells(summary, "みず")).toHaveText(["1", "0", "1", "0", "0"]);
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("攻撃技を選ぶと coverage(200)で攻撃範囲の表を出す", async ({ page }) => {
  await openBalance(page);
  await member(page, 1).getByRole("combobox", { name: "ポケモン", exact: true }).selectOption(FIRE.key);

  const coverageResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/balance/v1/team-balance/coverage" &&
      response.request().method() === "POST",
  );
  await member(page, 1)
    .getByRole("combobox", { name: "技1", exact: true })
    .selectOption({ label: FIRE_PUNCH });
  expect((await coverageResponse).status()).toBe(200);

  const coverage = page.getByRole("table", { name: "攻撃範囲", exact: true });
  // ほのお技は くさに ×2 抜群(有効1・抜群1)、みずに ×1/2 いまひとつ(有効0・抜群0)。
  await expect(rowCells(coverage, "くさ")).toHaveText(["×2 抜群", "1", "1"]);
  await expect(rowCells(coverage, "みず")).toHaveText(["×1/2 いまひとつ", "0", "0"]);
  await expect(coverage.getByRole("rowheader")).toHaveCount(18);
  await expect(page.getByRole("alert")).toHaveCount(0);
});
