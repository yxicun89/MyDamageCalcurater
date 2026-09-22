// P4-6: WASM モード(オフライン)ではバックエンドが無くても計算できる(docs/test-strategy.md「E2E」、ADR-0300 §2)。
// /api/** への通信を全て遮断した状態で計算し、engine.wasm は最初の計算まで取得しないこと(遅延読み込み)も確かめる。

import { expect, test, type Request } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  openApp,
  rowTexts,
  selectMatchup,
} from "./support/calcPage.ts";

function isEngineWasm(request: Request): boolean {
  return new URL(request.url()).pathname.endsWith("/engine.wasm");
}

function isApi(request: Request): boolean {
  return new URL(request.url()).pathname.startsWith("/api/");
}

test("/api を遮断しても計算でき、engine.wasm は最初の計算で初めて取得する", async ({ page }) => {
  const apiRequests: string[] = [];
  const wasmRequests: string[] = [];
  page.on("request", (request) => {
    if (isApi(request)) {
      apiRequests.push(request.url());
    }
    if (isEngineWasm(request)) {
      wasmRequests.push(request.url());
    }
  });
  // バックエンドが落ちている状態を再現する(届いた要求は全て通信エラーにする)。
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));

  await openApp(page);
  // 既定の計算モードはオフライン(WASM)。
  await expect(page.getByRole("radio", { name: "オフライン(WASM)", exact: true })).toBeChecked();
  await page.waitForLoadState("networkidle");
  expect(wasmRequests, "初回表示では engine.wasm を取得しない").toEqual([]);

  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  for (const text of await rowTexts(calcRows(page), DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
  await expect(page.getByRole("alert")).toHaveCount(0);

  expect(wasmRequests, "最初の計算で engine.wasm を1回だけ取得する").toHaveLength(1);
  expect(apiRequests, "オフラインでは /api を呼ばない").toEqual([]);

  // 2回目以降の計算で engine.wasm を取り直さない。
  const beforeSwap = await calcRows(page).allInnerTexts();
  await page.getByRole("button", { name: "攻守入れ替え", exact: true }).click();
  // 入れ替え後の計算が終わる(行の中身が変わる)のを待ってから確かめる(入れ替え前の行で満たされないように)。
  await expect.poll(async () => calcRows(page).allInnerTexts()).not.toEqual(beforeSwap);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  expect(wasmRequests).toHaveLength(1);
});
