// P4-6: 逆算の主な流れ(docs/test-strategy.md「E2E」: 観測ダメージ入力 → 候補リストが表示される)。
// 観測値は同じ対面の計算画面の結果から作る(例データや engine の式が変わっても、実在し得る値を入れるため)。
// 計算画面の2行目は engine の既定プリセット順で「H に 32 振り・B 無振り」(ADR-0009 §1 の hp)なので、
// 逆算の H32 前提(ADR-0010 §R1)と揃い、その範囲の整数%は B の SP 範囲の候補を生む。

import { expect, test, type Page } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  combobox,
  openApp,
  parsePercentRange,
  reverseRows,
  rowTexts,
  selectMatchup,
} from "./support/calcPage.ts";

/** 計算画面の「H32・B0」行(2行目)の表示%の範囲に入る整数%を返す。範囲に整数が無ければ失敗させる。 */
async function observablePercent(page: Page): Promise<number> {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const texts = await rowTexts(calcRows(page), DEFAULT_ROW_COUNT);
  const range = parsePercentRange(texts[1] ?? "");
  const percent = Math.ceil(range.min);
  expect(percent, `表示%の範囲 ${range.min}〜${range.max} に整数が無い`).toBeLessThanOrEqual(range.max);
  expect(percent).toBeGreaterThanOrEqual(1);
  return percent;
}

async function openReverseTab(page: Page): Promise<void> {
  await page.getByRole("tab", { name: "逆算", exact: true }).click();
  await expect(page.getByRole("tab", { name: "逆算", exact: true })).toHaveAttribute("aria-selected", "true");
}

test.beforeEach(async ({ page }) => {
  await openApp(page);
});

test("与えたダメージの観測%を入れると、H32 前提の注記と SP 範囲つきの候補が出る", async ({ page }) => {
  const percent = await observablePercent(page);
  await openReverseTab(page);

  // 既定の側は「与えたダメージ」(相手 = 防御側を推定する)。
  await expect(page.getByRole("radio", { name: "与えたダメージ", exact: true })).toBeChecked();
  await combobox(page, "自分のポケモン").selectOption({ label: SPECIES.fire.nameJa });
  await combobox(page, "相手のポケモン").selectOption({ label: SPECIES.water.nameJa });
  // 技は自分(攻撃側)の最初のダメージ技が自動で選ばれる(計算画面と同じ技)。
  await expect(combobox(page, "技").locator("option:checked")).toContainText("威力");

  await page.getByRole("textbox", { name: "観測1", exact: true }).fill(String(percent));

  const rows = reverseRows(page);
  await expect(rows.first()).toBeVisible();
  await expect(page.getByText("H32 を仮定", { exact: false })).toBeVisible();
  for (const text of await rows.allInnerTexts()) {
    // SP 範囲(「B 0〜7」「B 12」など。防御側の物理技なので B)と、候補の表示%の範囲を持つ。
    expect(text).toMatch(/B \d+(〜\d+)?/);
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
  // 観測値は実在の対面(H32・B0)から作ったので、近い候補ではない一致候補が少なくとも1つある。
  await expect(rows.filter({ hasNotText: "近い候補" })).not.toHaveCount(0);
});

test("観測を追加すると、候補は絞られるか同じ数のまま(増えない)", async ({ page }) => {
  const percent = await observablePercent(page);
  await openReverseTab(page);
  await combobox(page, "自分のポケモン").selectOption({ label: SPECIES.fire.nameJa });
  await combobox(page, "相手のポケモン").selectOption({ label: SPECIES.water.nameJa });
  await page.getByRole("textbox", { name: "観測1", exact: true }).fill(String(percent));

  const rows = reverseRows(page);
  // 候補は常に「性格2 × 持ち物」件返る(ADR-0010 §R1)ので、絞り込みは「観測を説明できる(近い候補でない)候補」の数で見る。
  const exactRows = rows.filter({ hasNotText: "近い候補" });
  await waitForReverseDone(page);
  const firstExact = await exactRows.count();

  await page.getByRole("button", { name: "観測を追加", exact: true }).click();
  await page.getByRole("textbox", { name: "観測2", exact: true }).fill(String(percent));

  // 入力を変えると古い候補を消して「計算中」になる(aria-busy)。再計算が終わってから数える。
  await waitForReverseDone(page);
  const secondExact = await exactRows.count();
  expect(secondExact).toBeGreaterThanOrEqual(1);
  expect(secondExact).toBeLessThanOrEqual(firstExact);
});

test("観測に範囲外の値を入れると、検証メッセージが出て候補を出さない", async ({ page }) => {
  await openReverseTab(page);
  await combobox(page, "自分のポケモン").selectOption({ label: SPECIES.fire.nameJa });
  await combobox(page, "相手のポケモン").selectOption({ label: SPECIES.water.nameJa });

  const observation = page.getByRole("textbox", { name: "観測1", exact: true });
  await observation.fill("101");
  await expect(observation).toHaveAttribute("aria-invalid", "true");
  // issue #304: aria-describedby は観測欄の説明(hint)と検証メッセージの両方を指すようになった
  // (与えたダメージ・%単位が既定なので、hint は「相手の HP が減った割合(%)」)。
  await expect(observation).toHaveAccessibleDescription(
    "相手の HP が減った割合(%) 1〜100 の整数で入力してください",
  );
  await expect(page.getByRole("list", { name: "推定結果", exact: true })).toHaveCount(0);
});

/** 逆算の再計算が終わる(計算中の表示が消え、推定結果の候補が出る)まで待つ。 */
async function waitForReverseDone(page: Page): Promise<void> {
  await expect(page.locator(".reverse-results[aria-busy='true']")).toHaveCount(0);
  await expect(reverseRows(page).first()).toBeVisible();
}
