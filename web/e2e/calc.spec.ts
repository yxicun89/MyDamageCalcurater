// P4-6: 計算画面の主な流れ(オフライン = WASM、既定の計算モード)。docs/test-strategy.md「E2E(Playwright)」:
// ポケモン選択 → 攻撃側/防御側選択 → 技選択 → 一括結果、攻守入れ替え・プリセット変更で結果が更新される。

import { expect, test } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  KO_PATTERN,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  combobox,
  openApp,
  parsePercentRange,
  rowTexts,
  selectMatchup,
} from "./support/calcPage.ts";

test.beforeEach(async ({ page }) => {
  await openApp(page);
});

test("攻撃側・防御側を選ぶと技が自動で選ばれ、既定の5行の一括結果が出る", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);

  // 技は攻撃側の最初のダメージ技(威力つき)が選ばれている。
  const move = combobox(page, "技");
  await expect(move).not.toHaveValue("");
  await expect(move.locator("option:checked")).toContainText("威力");

  const rows = calcRows(page);
  const texts = await rowTexts(rows, DEFAULT_ROW_COUNT);
  for (const text of texts) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
    expect(text).toMatch(KO_PATTERN);
  }
  // 各行にダメージバーがある。issue #306 でバーは装飾(aria-hidden)にしたので、
  // role ではなく testid で数え、支援技術に名前の無い meter が残っていないことも確かめる。
  await expect(rows.getByTestId("damage-bar")).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(page.getByRole("meter")).toHaveCount(0);
});

test("攻撃側の調整を A特化 に変えると、先頭行の最大%が増える", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const [before] = await rowTexts(rows, DEFAULT_ROW_COUNT);
  const beforeMax = parsePercentRange(before ?? "").max;

  await chooseRadio(page, "攻撃側の調整", "A特化");

  await expect
    .poll(async () => {
      const [after] = await rowTexts(rows, DEFAULT_ROW_COUNT);
      return parsePercentRange(after ?? "").max;
    })
    .toBeGreaterThan(beforeMax);
});

test("攻守入れ替えで攻撃側・防御側の名前が入れ替わり、結果が更新される", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const before = await rowTexts(rows, DEFAULT_ROW_COUNT);

  await page.getByRole("button", { name: "攻守入れ替え", exact: true }).click();

  await expect(combobox(page, "攻撃側のポケモン")).toHaveValue(SPECIES.water.key);
  await expect(combobox(page, "防御側のポケモン")).toHaveValue(SPECIES.fire.key);
  // issue #304: カードの見出し(h2)は「攻撃側」「防御側」のまま動かず、
  // 入れ替わるのはその中のポケモンの名前(h3。design.md「入力のラベル」の見出しの階層)。
  await expect(
    page.getByRole("region", { name: "攻撃側", exact: true }).getByRole("heading", { level: 2 }),
  ).toHaveText("攻撃側");
  await expect(
    page.getByRole("region", { name: "攻撃側", exact: true }).getByRole("heading", { level: 3 }),
  ).toHaveText(SPECIES.water.nameJa);
  await expect(
    page.getByRole("region", { name: "防御側", exact: true }).getByRole("heading", { level: 3 }),
  ).toHaveText(SPECIES.fire.nameJa);

  // 技は新しい攻撃側(テストみず)の learnset から選び直され、結果は入れ替え前と異なる。
  await expect(combobox(page, "技").locator("option:checked")).toContainText("威力");
  await expect.poll(async () => rowTexts(rows, DEFAULT_ROW_COUNT)).not.toEqual(before);
  for (const text of await rowTexts(rows, DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
});

test("持ち物の候補も比較 をオンにすると行が増える", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  await expect(rows).toHaveCount(DEFAULT_ROW_COUNT);

  await page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true }).check();

  await expect.poll(async () => rows.count()).toBeGreaterThan(DEFAULT_ROW_COUNT);
  // 増えた行も同じ書式で、持ち物の名前(例データの「テスト…」)か「持ち物なし」を持つ。
  for (const text of await rows.allInnerTexts()) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
    expect(text).toMatch(/持ち物なし|テスト/);
  }
});
