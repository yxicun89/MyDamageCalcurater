// P5-5d(ADR-0318 §7): 「この端末のデータを削除」(本番ビルド。バックエンドは起動せず page.route で両 DELETE を fake にする)。
// 情報ページで 削除ボタン → 確認ダイアログ(キャンセルが初期フォーカス)→ 削除する → record は partial→completed、
// team は completed → 「削除しました。」。両方の DELETE が端末 ID 付きで呼ばれ、partial で record が2回呼ばれる。
// 続けて、失敗(503)→ 再試行の流れと、キャンセル・Esc で何も呼ばないこと。文言は ADR-0209 §8 のリテラル。

import { expect, test, type Page } from "@playwright/test";
import { openApp } from "./support/calcPage.ts";

const RESULT = { purgedAt: "2026-10-02T01:00:00Z" };

async function openAbout(page: Page): Promise<void> {
  await openApp(page);
  await page.getByRole("contentinfo").getByRole("link", { name: "このアプリについて", exact: true }).click();
  await expect(page.getByRole("heading", { level: 3, name: "データの扱い", exact: true })).toBeVisible();
}

test("削除: 確認 → record は partial を繰り返して completed、team も completed → 「削除しました。」", async ({
  page,
}) => {
  const calls: { path: string; device: string | undefined }[] = [];
  let recordCount = 0;
  await page.route("**/api/record/device-data", async (route) => {
    calls.push({ path: "record", device: route.request().headers()["x-device-id"] });
    recordCount += 1;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ...RESULT,
        status: recordCount < 2 ? "partial" : "completed",
        deleted: { calcEvents: 1, aggregates: 0, favorites: 0 },
      }),
    });
  });
  await page.route("**/api/team/device-data", async (route) => {
    calls.push({ path: "team", device: route.request().headers()["x-device-id"] });
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ...RESULT, status: "completed", deleted: { teams: 0, teamMembers: 0 } }),
    });
  });

  await openAbout(page);
  await page.getByRole("button", { name: "この端末のデータを削除", exact: true }).click();
  const dialog = page.getByRole("alertdialog", { name: "この端末のデータを削除", exact: true });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText("履歴・お気に入り・構築をサーバーから削除します。元に戻せません。", { exact: true }),
  ).toBeVisible();
  await expect(dialog.getByRole("button", { name: "キャンセル", exact: true })).toBeFocused();
  expect(calls).toHaveLength(0);

  await dialog.getByRole("button", { name: "削除する", exact: true }).click();
  await expect(page.getByText("削除しました。", { exact: true })).toBeVisible();
  expect(calls.filter((call) => call.path === "record")).toHaveLength(2);
  expect(calls.filter((call) => call.path === "team")).toHaveLength(1);
  const deviceIds = new Set(calls.map((call) => call.device));
  expect(deviceIds.size).toBe(1);
  expect([...deviceIds][0]).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
});

test("失敗(record が 503)→ 「構築は削除済みです。」と再試行 → 成功。再試行は record だけ呼ぶ", async ({
  page,
}) => {
  let recordCount = 0;
  let teamCount = 0;
  await page.route("**/api/record/device-data", async (route) => {
    recordCount += 1;
    if (recordCount === 1) {
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ code: "upstream_unavailable", message: "x" }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ...RESULT,
        status: "completed",
        deleted: { calcEvents: 0, aggregates: 0, favorites: 0 },
      }),
    });
  });
  await page.route("**/api/team/device-data", async (route) => {
    teamCount += 1;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ...RESULT, status: "completed", deleted: { teams: 0, teamMembers: 0 } }),
    });
  });

  await openAbout(page);
  await page.getByRole("button", { name: "この端末のデータを削除", exact: true }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "削除する", exact: true }).click();

  const alert = page.getByRole("alert").filter({ hasText: "サーバーに届きませんでした。" });
  await expect(alert).toContainText("サーバーに届きませんでした。通信を確認してもう一度お試しください。");
  await expect(alert).toContainText("構築は削除済みです。");
  await expect(page.getByText("削除しました。", { exact: true })).toHaveCount(0);

  await page.getByRole("button", { name: "もう一度削除する", exact: true }).click();
  await expect(page.getByText("削除しました。", { exact: true })).toBeVisible();
  expect(recordCount).toBe(2);
  expect(teamCount).toBe(1);
});

test("キャンセル・Esc では何も呼ばず、フォーカスは削除ボタンへ戻る。API が不達でも計算画面へ戻れる", async ({
  page,
}) => {
  let called = 0;
  await page.route("**/api/*/device-data", async (route) => {
    called += 1;
    await route.abort("connectionrefused");
  });

  await openAbout(page);
  const button = page.getByRole("button", { name: "この端末のデータを削除", exact: true });
  await button.click();
  await page.getByRole("alertdialog").getByRole("button", { name: "キャンセル", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toHaveCount(0);
  await expect(button).toBeFocused();

  await button.click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("alertdialog")).toHaveCount(0);
  await expect(button).toBeFocused();
  expect(called).toBe(0);

  // 不達のまま削除 → 失敗を表示し、成功にしない。
  await button.click();
  await page.getByRole("alertdialog").getByRole("button", { name: "削除する", exact: true }).click();
  await expect(page.getByRole("alert").filter({ hasText: "サーバーに届きませんでした。" })).toBeVisible();
  await expect(page.getByText("削除しました。", { exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "計算に戻る", exact: true }).click();
  await expect(page).toHaveURL(/\/calc$/);
});
