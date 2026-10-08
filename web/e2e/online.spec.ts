// P4-6: サーバー(オンライン)の主な流れ(ADR-0301 §4・§6)。playwright.online.config.ts が
//   - calc-svc を Web の例データ(MasterExport)で起動し、vite preview の /api を転送する
//   - pokedex フィクスチャ(e2e/support/pokedexFixtureServer.mjs。PR2・ADR-0307)を起動し、
//     /api/pokedex を /api より前にそちらへ転送する(POKEDEX_PROXY_TARGET)
// 計算モードをオンラインに切り替えると、マスタは /api/pokedex/* から読み、計算は /api/calc/bulk で行い、
// engine.wasm は読まない。同じ画面操作がこの端末(オフライン)と同じ行を出すこと(ADR-0301 §6 の結合)も確かめる。
//
// PR2 での変更(ADR-0307):
//   - 種族の選択は `<select>` ではなく検索欄(ADR-0304 A-4・A-10)なので selectMatchupBySearch を使う。
//   - issue 211(ADR-0322): 公開 API が effect を返すので、オンラインでも「持ち物の候補も比較」を押せる。
//     オフライン(キャッシュ。effects: false)では押せないままなので、そちらは disabled を確かめる。

import { expect, test, type Page, type Request } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  KO_PATTERN,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  combobox,
  openApp,
  rowTexts,
  selectMatchupBySearch,
  selectSpeciesBySearch,
} from "./support/calcPage.ts";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isEngineWasm(request: Request): boolean {
  return new URL(request.url()).pathname.endsWith("/engine.wasm");
}

async function selectMode(page: Page, name: "サーバー(オンライン)" | "この端末(オフライン)"): Promise<void> {
  await chooseRadio(page, "計算する場所", name);
}

test("オンラインのマスタは /api/pokedex/* から読む(フィクスチャへの振り分けが効いている)", async ({
  page,
}) => {
  // ADR-0313: 既定がオンラインなので、マスタは開いた時点で読まれる。応答の待ち受けは開く前に仕掛ける。
  const items = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/pokedex/items");
  const natures = page.waitForResponse(
    (response) => new URL(response.url()).pathname === "/api/pokedex/natures",
  );
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");

  for (const response of [await items, await natures]) {
    expect(response.status(), `${response.url()} が 200 でない`).toBe(200);
    // 端末 ID・セッション ID を全リクエストに付ける(CLAUDE.md 技術規約、ADR-0301 §3)。
    const headers = response.request().headers();
    expect(headers["x-device-id"]).toMatch(UUID_PATTERN);
    expect(headers["x-session-id"]).toMatch(UUID_PATTERN);
  }
  // マスタが読めていれば失敗の案内は出ない。
  await expect(page.getByRole("alert")).toHaveCount(0);

  // 種族の検索も公開 API 経由(ADR-0304 §1・A-13)。
  const search = page.waitForResponse(
    (response) => new URL(response.url()).pathname === "/api/pokedex/species",
  );
  const detail = page.waitForResponse((response) =>
    new URL(response.url()).pathname.startsWith("/api/pokedex/species/"),
  );
  const moves = page.waitForResponse(
    (response) => new URL(response.url()).pathname === "/api/pokedex/moves/batch",
  );
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  expect((await search).status()).toBe(200);
  expect((await detail).status()).toBe(200);
  expect((await moves).status()).toBe(200);
});

test("オンラインでは /api/calc/bulk(200)で計算し、engine.wasm を取得しない", async ({ page }) => {
  const wasmRequests: string[] = [];
  page.on("request", (request) => {
    if (isEngineWasm(request)) {
      wasmRequests.push(request.url());
    }
  });
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");

  const bulkResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/calc/bulk" && response.request().method() === "POST",
  );
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const response = await bulkResponse;
  expect(response.status()).toBe(200);
  // 端末 ID・セッション ID を全リクエストに付ける(CLAUDE.md 技術規約、ADR-0301 §3)。
  const headers = response.request().headers();
  expect(headers["x-device-id"]).toMatch(UUID_PATTERN);
  expect(headers["x-session-id"]).toMatch(UUID_PATTERN);

  for (const text of await rowTexts(calcRows(page), DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
    expect(text).toMatch(KO_PATTERN);
  }
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(wasmRequests, "オンラインでは engine.wasm を読まない").toEqual([]);
});

test("オンラインで「持ち物の候補も比較」をオンにすると、防御側の候補持ち物の行が増える(issue 211)", async ({
  page,
}) => {
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  const compare = page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true });
  await compare.check();
  await expect.poll(async () => calcRows(page).count()).toBeGreaterThan(DEFAULT_ROW_COUNT);
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("同じ画面操作で、サーバー(オンライン)とこの端末(オフライン)の結果の行が一致する", async ({ page }) => {
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");
  // オンライン側の行が API から来たことを、このテストの中でも確かめる(bulk の応答が 200)。
  const bulkResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/calc/bulk" && response.request().method() === "POST",
  );
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  expect((await bulkResponse).status()).toBe(200);
  // 公開 API が effect を返す(issue 211)ので、持ち物の候補比較はオンラインで選べる(既定はオフ)。
  const compare = page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true });
  await expect(compare).toBeEnabled();
  await expect(compare).not.toBeChecked();
  const online = await rowTexts(calcRows(page), DEFAULT_ROW_COUNT);

  // 計算モードは localStorage に覚えるので、開き直してオフラインに切り替え、同じ操作をする
  // (オンラインで取得したマスタ・種族は IndexedDB に保存済み。ADR-0313)。
  await openApp(page);
  await selectMode(page, "この端末(オフライン)");
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  // ADR-0313: オフラインのマスタはオンラインで取得したキャッシュなので、効果データが無く(effects: false)、
  // 候補比較はオフラインでも選べない(以前は架空の例データで選べた。実データ相当では同じ制約になる)。
  await expect(compare).toBeDisabled();
  await expect.poll(async () => rowTexts(calcRows(page), DEFAULT_ROW_COUNT)).toEqual(online);
});

// P5-5c(ADR-0317): よく計算する相手のチップ。record-svc はこの構成に無いので、
// gateway の /api/record/frequent-opponents を page.route で fake する(pokedex は本物のフィクスチャ)。
test("オンラインでは、よく計算する相手のチップを押すと防御側に反映される", async ({ page }) => {
  let recordCalls = 0;
  await page.route("**/api/record/frequent-opponents*", async (route) => {
    recordCalls += 1;
    const headers = route.request().headers();
    expect(headers["x-device-id"]).toMatch(UUID_PATTERN);
    expect(headers["x-session-id"]).toMatch(UUID_PATTERN);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        { speciesKey: SPECIES.water.key, score: 2.5, count: 3, lastCalculatedAt: "2026-10-01T00:00:00Z" },
      ]),
    });
  });
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");

  const group = page.getByRole("group", { name: "よく計算する相手", exact: true });
  const chip = group.getByRole("button", { name: SPECIES.water.nameJa, exact: true });
  await expect(chip).toBeVisible();
  await selectSpeciesBySearch(page, "攻撃側のポケモン", SPECIES.fire.nameJa);
  await chip.click();

  await expect(combobox(page, "防御側のポケモン")).toHaveValue(SPECIES.water.nameJa);
  for (const text of await rowTexts(calcRows(page), DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
  expect(recordCalls, "取得は画面表示時の1回だけ").toBe(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("オンラインで record が 503 でも、チップは出ず、計算は成功する", async ({ page }) => {
  await page.route("**/api/record/frequent-opponents*", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ code: "upstream_unavailable", message: "届きません" }),
    }),
  );
  await openApp(page);
  await selectMode(page, "サーバー(オンライン)");
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);

  for (const text of await rowTexts(calcRows(page), DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
  await expect(page.getByRole("group", { name: "よく計算する相手", exact: true })).toHaveCount(0);
  await expect(page.getByRole("alert")).toHaveCount(0);
});
