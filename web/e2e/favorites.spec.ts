// P5-3c(ADR-0327): お気に入り(手動ピン留め)の主な流れ。record-svc は起動しない。
// `/api/record/favorites**` は spec の中の fake で受ける(page.route。保存先はこの配列だけ)。マスタは架空の例データ。
// 確かめること: 計算画面で攻撃側を選ぶ → 「攻撃側をお気に入りに追加」→ お気に入りタブの一覧に出る(件数 1/100件)→
// 削除(確認 → 削除する)→ 空の案内に戻る。POST の本文は speciesKey・Lv50・SP 合計 66 以下で、端末 ID ヘッダが付く。
// record が 503 でも計算タブは使える。
// I-web-8 = F-09(ADR-0333): 追加した本文には計算の入力(calc)が付き、お気に入りタブの「計算に使う」で計算タブに戻ると
// 入力が戻って結果がすぐ出る。

import { expect, test, type Page, type Route } from "@playwright/test";
import {
  SPECIES,
  calcRows,
  combobox,
  openApp,
  selectMatchup,
  selectSpeciesBySearch,
} from "./support/calcPage.ts";

interface FakeFavorite {
  id: string;
  label: string | null;
  individual: { speciesKey: string; level: number; sp: Record<string, number> };
  /** ADR-0228: 計算の入力全体(付けて作成したときだけ)。fake は受け取った形のまま返す。 */
  calc?: { attacker: { speciesKey: string }; defender: { speciesKey: string }; moveId: string };
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
        calc?: FakeFavorite["calc"] | null;
      };
      backend.postBodies.push(input);
      backend.postHeaders.push(request.headers());
      const now = "2026-10-04T09:00:00Z";
      const created: FakeFavorite = {
        id: String(sequence),
        label: input.label ?? null,
        individual: input.individual,
        ...(input.calc === undefined || input.calc === null ? {} : { calc: input.calc }),
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

/**
 * 一括計算(POST /api/calc/bulk)の最小の応答(1行)。この設定(playwright.config.ts)には calc-svc が無いので、
 * 計算の結果が出ることを確かめるため応答だけ差し替える(計算の中身ではなく、入力が戻って要求が送られることを見る)。
 */
const BULK_STUB = {
  defenderSpeciesKey: SPECIES.water.key,
  rows: [
    {
      preset: "none",
      presetLabel: "無振り",
      itemId: null,
      abilityId: "exampleabilitynone",
      abilityIds: ["exampleabilitynone"],
      defender: {
        sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
        nature: { plus: null, minus: null },
        natureId: null,
        stats: { hp: 175, atk: 100, def: 100, spa: 100, spd: 100, spe: 100 },
      },
      result: {
        rolls: [10, 11, 12],
        minDamage: 10,
        maxDamage: 12,
        minPercent: 5.7,
        maxPercent: 6.9,
        defenderHP: 175,
        effectiveness: 1,
        stab: false,
        category: "physical",
        ko: { hits: 9, guaranteed: true, displayChancePercent: 100 },
        unsupported: [],
      },
    },
  ],
};

test("計算画面で追加(calc 付き)→ 攻撃側を変える → お気に入りタブの「計算に使う」→ 計算タブで入力が戻り結果が出る", async ({
  page,
}) => {
  const backend = await installRecordBackend(page);
  const bulkBodies: { attacker: { speciesKey: string }; defenderSpeciesKey: string }[] = [];
  await page.route("**/api/calc/bulk", (route) => {
    bulkBodies.push(route.request().postDataJSON() as (typeof bulkBodies)[number]);
    return route.fulfill({ status: 200, json: BULK_STUB });
  });
  await openApp(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(calcRows(page).first()).toBeVisible();

  await page.getByRole("button", { name: "攻撃側をお気に入りに追加", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "お気に入りに追加しました" })).toBeVisible();
  const body = backend.postBodies[0] as { label: string; calc?: FakeFavorite["calc"] };
  expect(body.calc?.attacker.speciesKey).toBe(SPECIES.fire.key);
  expect(body.calc?.defender.speciesKey).toBe(SPECIES.water.key);
  expect(body.calc?.moveId).not.toBe("");
  expect(body.label.startsWith(`${SPECIES.fire.nameJa}→${SPECIES.water.nameJa}`)).toBe(true);

  // 戻したことが分かるよう、攻撃側を別の種族に変えておく
  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.water.nameJa);

  await page.getByRole("tab", { name: "お気に入り", exact: true }).click();
  const region = page.getByRole("region", { name: "お気に入り", exact: true });
  await region.getByRole("button", { name: `「${body.label}」を計算に使う`, exact: true }).click();

  await expect(page.getByRole("tab", { name: "計算", exact: true })).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveURL(/\/calc$/);
  await expect(combobox(page, "攻撃側のポケモン")).toHaveValue(SPECIES.fire.nameJa);
  await expect(combobox(page, "防御側のポケモン")).toHaveValue(SPECIES.water.nameJa);
  await expect(calcRows(page).first()).toBeVisible();
  // 戻した入力で一括計算が要求された(攻撃側は追加したときの種族)
  await expect.poll(() => bulkBodies.at(-1)?.attacker.speciesKey).toBe(SPECIES.fire.key);
  expect(bulkBodies.at(-1)?.defenderSpeciesKey).toBe(SPECIES.water.key);
});
