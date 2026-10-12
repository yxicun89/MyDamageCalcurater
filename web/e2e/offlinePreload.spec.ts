// ADR-0358 段 3: 「このアプリについて」の「オフライン用にポケモンのデータを保存」。
// pokedex フィクスチャ(公開 API の形)に対して、頭文字ごとの一覧取得 → 直列の解決 → 端末(IndexedDB)への保存が通ること、
// 保存後にオフラインへ切り替えると「保存済み: ○匹」が出ることを確かめる。

import { expect, test } from "@playwright/test";
import { chooseRadio } from "./support/calcPage.ts";

test("オンラインで保存すると、オフラインの画面に保存済みの匹数が出る", async ({ page }) => {
  await page.goto("/about");
  const start = page.getByRole("button", { name: "オフライン用にポケモンのデータを保存", exact: true });
  await expect(start).toBeEnabled();
  await start.click();

  const status = page.getByRole("status").filter({ hasText: /保存済み: \d+匹。オフラインで使えます/ });
  await expect(status).toBeVisible();
  const saved = Number(/保存済み: (\d+)匹/.exec((await status.textContent()) ?? "")?.[1]);
  expect(saved).toBeGreaterThanOrEqual(3);
  await expect(start).toBeFocused();

  await chooseRadio(page, "計算する場所", "この端末(オフライン)");
  await expect(page.getByRole("button", { name: "オフライン用にポケモンのデータを保存" })).toBeDisabled();
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));
  await page.goto("/calc");
  await expect(page.getByRole("note", { name: "オフラインで使える範囲" })).toContainText(
    `保存済み: ${String(saved)}匹`,
  );
});
