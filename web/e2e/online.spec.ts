// P4-6: オンライン(API)の主な流れ(ADR-0301 §4・§6)。playwright.online.config.ts が calc-svc を Web の例データで起動し、
// vite preview が /api を転送する。計算モードをオンラインに切り替えると /api/calc/bulk で計算し、engine.wasm は読まない。
// 同じ画面操作がオフライン(WASM)と同じ行を出すこと(ADR-0301 §6 の結合)も確かめる。

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
} from "./support/calcPage.ts";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isEngineWasm(request: Request): boolean {
  return new URL(request.url()).pathname.endsWith("/engine.wasm");
}

async function selectMode(page: Page, name: "オンライン(API)" | "オフライン(WASM)"): Promise<void> {
  await chooseRadio(page, "計算モード", name);
}

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
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
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
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  expect((await bulkResponse).status()).toBe(200);
  await page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true }).check();
  const rows = calcRows(page);
  await expect.poll(async () => rows.count()).toBeGreaterThan(DEFAULT_ROW_COUNT);
  const online = await rows.allInnerTexts();

  // 計算モードは localStorage に覚えるので、開き直してオフラインに切り替え、同じ操作をする。
  await openApp(page);
  await selectMode(page, "オフライン(WASM)");
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true }).check();
  await expect.poll(async () => rows.allInnerTexts()).toEqual(online);
});
