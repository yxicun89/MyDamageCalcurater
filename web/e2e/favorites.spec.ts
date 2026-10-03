// P5-3c(ADR-0327): お気に入り(手動ピン留め)の主な流れ。record-svc は起動しない。
// `/api/record/favorites**` は spec の中の fake で受ける(page.route。保存先はこの配列だけ)。マスタは架空の例データ。
// 確かめること: 計算画面で攻撃側を選ぶ → 「攻撃側をお気に入りに追加」→ お気に入りタブの一覧に出る(件数 1/100件)→
// 削除(確認 → 削除する)→ 空の案内に戻る。POST の本文は speciesKey・Lv50・SP 合計 66 以下で、端末 ID ヘッダが付く。
// record が 503 でも計算タブは使える。

import { expect, test, type Page, type Route } from "@playwright/test";
import { SPECIES, openApp, selectSpeciesBySearch } from "./support/calcPage.ts";

interface FakeFavorite {
  id: string;
  label: string | null;
  individual: { speciesKey: string; level: number; sp: Record<string, number> };
  createdAt: string;
  updatedAt: string;
}

interface RecordBackend {
  readonly favorites: FakeFavorite[];
  readonly postBodies: unknown[];
  readonly postHeaders: Record<string, string>[];
}

const FAVORITES_PATH = /\/api\/record\/favorites(?:\/([^/?]+))?$/;

async function installRecordBackend(page: Page, options: { down?: boolean } = {}): Promise<RecordBackend> {
  const backend: RecordBackend = { favorites: [], postBodies: [], postHeaders: [] };
  let sequence = 0;
  await page.route(FAVORITES_PATH, async (route: Route) => {
    const request = route.request();
    const id = FAVORITES_PATH.exec(new URL(request.url()).pathname)?.[1];
    const json = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (options.down === true) {
      return json(503, { code: "upstream_unavailable", message: "記録サービスに届きません" });
    }
    if (request.method() === "GET" && id === undefined) {
      return json(200, backend.favorites);
    }
    if (request.method() === "POST") {
      sequence += 1;
      const input = request.postDataJSON() as {
        label?: string | null;
        individual: FakeFavorite["individual"];
      };
      backend.postBodies.push(input);
      backend.postHeaders.push(request.headers());
      const now = "2026-10-04T09:00:00Z";
      const created: FakeFavorite = {
        id: String(sequence),
        label: input.label ?? null,
        individual: input.individual,
        createdAt: now,
        updatedAt: now,
      };
      backend.favorites.unshift(created);
      return json(201, created);
    }
    if (request.method() === "DELETE" && id !== undefined) {
      const index = backend.favorites.findIndex((favorite) => favorite.id === id);
      if (index < 0) {
        return json(404, { code: "not_found", message: "お気に入りが見つかりません" });
      }
      backend.favorites.splice(index, 1);
      return route.fulfill({ status: 204 });
    }
    return json(405, { code: "method_not_allowed", message: "対応していない操作です" });
  });
  return backend;
}

test("計算画面で追加 → お気に入りタブの一覧に出る → 削除(2段階)で空に戻る", async ({ page }) => {
  const backend = await installRecordBackend(page);
  await openApp(page);
  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.fire.nameJa);

  await page.getByRole("button", { name: "攻撃側をお気に入りに追加", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "お気に入りに追加しました" })).toBeVisible();

  expect(backend.postBodies).toHaveLength(1);
  const body = backend.postBodies[0] as { label: string; individual: FakeFavorite["individual"] };
  expect(body.label).toBe(SPECIES.fire.nameJa);
  expect(body.individual.speciesKey).toBe(SPECIES.fire.key);
  expect(body.individual.level).toBe(50);
  expect(Object.values(body.individual.sp).reduce((sum, value) => sum + value, 0)).toBeLessThanOrEqual(66);
  expect(backend.postHeaders[0]?.["x-device-id"]).toMatch(/^[0-9a-f-]{36}$/);

  await page.getByRole("tab", { name: "お気に入り", exact: true }).click();
  const region = page.getByRole("region", { name: "お気に入り", exact: true });
  const list = region.getByRole("list", { name: "お気に入り一覧", exact: true });
  await expect(list).toContainText(SPECIES.fire.nameJa);
  await expect(region).toContainText("1/100件");

  await region.getByRole("button", { name: `「${SPECIES.fire.nameJa}」を削除`, exact: true }).click();
  await region.getByRole("button", { name: `「${SPECIES.fire.nameJa}」を削除する`, exact: true }).click();
  await expect(list).toHaveCount(0);
  await expect(region).toContainText("0/100件");
  expect(backend.favorites).toHaveLength(0);
});

test("record が 503 のとき、お気に入りタブは alert を出すが、計算タブは使える(計算は record に依存しない)", async ({
  page,
}) => {
  await installRecordBackend(page, { down: true });
  await openApp(page);
  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.fire.nameJa);
  await page.getByRole("tab", { name: "お気に入り", exact: true }).click();
  await expect(page.getByRole("alert").filter({ hasText: "記録サービスに届きません" })).toBeVisible();
  await page.getByRole("tab", { name: "計算", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true })).toHaveValue(
    SPECIES.fire.nameJa,
  );
});
