// P4-6: オンライン(API)の主な流れ(ADR-0301 §4・§6)。playwright.online.config.ts が
//   - calc-svc を Web の例データ(MasterExport)で起動し、vite preview の /api を転送する
//   - pokedex フィクスチャ(e2e/support/pokedexFixtureServer.mjs。PR2・ADR-0307)を起動し、
//     /api/pokedex を /api より前にそちらへ転送する(POKEDEX_PROXY_TARGET)
// 計算モードをオンラインに切り替えると、マスタは /api/pokedex/* から読み、計算は /api/calc/bulk で行い、
// engine.wasm は読まない。同じ画面操作がオフライン(WASM)と同じ行を出すこと(ADR-0301 §6 の結合)も確かめる。
//
// PR2 での変更(ADR-0307):
//   - 種族の選択は `<select>` ではなく検索欄(ADR-0304 A-4・A-10)なので selectMatchupBySearch を使う。
//   - 「持ち物の候補も比較」はオンラインでは押せない(公開 API に効果データが無く
//     ONLINE_MASTER_CAPABILITIES.effects が false。ADR-0304 A-1)。以前はこれをオンにして比較していたが
//     成立しないので、**disabled であることを確かめたうえで**既定の5行を比較する(検査は増やして減らさない)。

import { expect, test, type Page, type Request } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  KO_PATTERN,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  openApp,
  rowTexts,
  selectMatchup,
  selectMatchupBySearch,
} from "./support/calcPage.ts";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isEngineWasm(request: Request): boolean {
  return new URL(request.url()).pathname.endsWith("/engine.wasm");
}

async function selectMode(page: Page, name: "オンライン(API)" | "オフライン(WASM)"): Promise<void> {
  await chooseRadio(page, "計算モード", name);
}

test("オンラインのマスタは /api/pokedex/* から読む(フィクスチャへの振り分けが効いている)", async ({
  page,
}) => {
  await openApp(page);
  const items = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/pokedex/items");
  const natures = page.waitForResponse(
    (response) => new URL(response.url()).pathname === "/api/pokedex/natures",
  );
  await selectMode(page, "オンライン(API)");

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
  await selectMode(page, "オンライン(API)");

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

test("同じ画面操作で、オンライン(API)とオフライン(WASM)の結果の行が一致する", async ({ page }) => {
  await openApp(page);
  await selectMode(page, "オンライン(API)");
  // オンライン側の行が API から来たことを、このテストの中でも確かめる(bulk の応答が 200)。
  const bulkResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/calc/bulk" && response.request().method() === "POST",
  );
  await selectMatchupBySearch(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  expect((await bulkResponse).status()).toBe(200);
  // 公開 API に効果データが無いので、持ち物の候補比較はオンラインでは選べない(ADR-0304 A-1)。
  const compare = page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true });
  await expect(compare).toBeDisabled();
  await expect(compare).not.toBeChecked();
  const online = await rowTexts(calcRows(page), DEFAULT_ROW_COUNT);

  // 計算モードは localStorage に覚えるので、開き直してオフラインに切り替え、同じ操作をする。
  await openApp(page);
  await selectMode(page, "オフライン(WASM)");
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  // オフラインは効果データがあるので、こちらでは候補比較を選べる(オンラインとの違いの確認)。
  await expect(compare).toBeEnabled();
  await expect.poll(async () => rowTexts(calcRows(page), DEFAULT_ROW_COUNT)).toEqual(online);
});
