// P4-11: コンテナ(web/Dockerfile・web/nginx.conf)の配信設定を HTTP で確かめる。playwright.container.config.ts だけが走らせる
// (vite preview はこの設定を持たないので、playwright.config.ts では除外する)。
// ADR-0300 §1(URL で画面を切り替える → SPA のフォールバック)、ADR-0011 §6・§11(engine.wasm は application/wasm)。
// /api は gateway の持ち物で、Web のコンテナは転送も index.html での代用もしない(DECISIONS.md 2026-09-22)。

import { expect, test, type APIResponse } from "@playwright/test";

/** WebAssembly のバイナリの先頭4バイト(\0asm)。 */
const WASM_MAGIC = [0x00, 0x61, 0x73, 0x6d];

/** index.html にだけある印(アプリを差し込む要素)。404 を index.html で代用していないかの判定にも使う。 */
const APP_ROOT = '<div id="root"';

/** ハッシュ付きの /static/* を「長く」キャッシュさせるとみなす max-age の下限(30日)。 */
const LONG_MAX_AGE_SECONDS = 30 * 24 * 60 * 60;

function header(response: APIResponse, name: string): string {
  return response.headers()[name.toLowerCase()] ?? "";
}

/** Cache-Control が「使う前に毎回サーバーへ確かめる」指定か(no-cache、または max-age=0 と must-revalidate)。 */
function requiresRevalidation(cacheControl: string): boolean {
  const value = cacheControl.toLowerCase();
  return value.includes("no-cache") || (/max-age=0\b/.test(value) && value.includes("must-revalidate"));
}

function maxAgeOf(cacheControl: string): number {
  const match = /max-age=(\d+)/.exec(cacheControl.toLowerCase());
  return match?.[1] === undefined ? 0 : Number(match[1]);
}

test.describe("SPA のフォールバック", () => {
  for (const path of ["/", "/calc", "/reverse", "/reverse?x=1"]) {
    test(`${path} は index.html(text/html)を返し、毎回確かめさせる`, async ({ request }) => {
      const response = await request.get(path);
      expect(response.status()).toBe(200);
      expect(header(response, "content-type")).toContain("text/html");
      expect(await response.text()).toContain(APP_ROOT);
      // index.html はハッシュを持たない。古い版を掴み続けないように毎回確かめさせる。
      expect(requiresRevalidation(header(response, "cache-control"))).toBe(true);
    });
  }

  test("無い /static/* は index.html で代用せず 404 を返す(JS の代わりに HTML が返って壊れるのを防ぐ)", async ({
    request,
  }) => {
    const response = await request.get("/static/does-not-exist-0000.js");
    expect(response.status()).toBe(404);
    expect(await response.text()).not.toContain(APP_ROOT);
  });
});

test.describe("engine.wasm と wasm_exec.js", () => {
  test("engine.wasm は application/wasm で、本物の WebAssembly を返し、毎回確かめさせる", async ({
    request,
  }) => {
    const response = await request.get("/engine.wasm");
    expect(response.status()).toBe(200);
    expect(header(response, "content-type")).toBe("application/wasm");
    expect(requiresRevalidation(header(response, "cache-control"))).toBe(true);
    const body = await response.body();
    expect([...body.subarray(0, 4)]).toEqual(WASM_MAGIC);
    // engine.wasm は約 4.6MB(ADR-0011 §11)。空や別物のファイルでないことの目安。
    expect(body.length).toBeGreaterThan(1_000_000);
  });

  test("gzip を受け付けるクライアントには engine.wasm を gzip で返す", async ({ request }) => {
    const response = await request.get("/engine.wasm", { headers: { "Accept-Encoding": "gzip" } });
    expect(response.status()).toBe(200);
    expect(header(response, "content-encoding")).toBe("gzip");
  });

  test("wasm_exec.js は JavaScript として返し、毎回確かめさせる", async ({ request }) => {
    const response = await request.get("/wasm_exec.js");
    expect(response.status()).toBe(200);
    expect(header(response, "content-type")).toContain("javascript");
    expect(requiresRevalidation(header(response, "cache-control"))).toBe(true);
  });
});

test("ハッシュ付きの /static/*.js は immutable で長くキャッシュさせる", async ({ request }) => {
  const html = await (await request.get("/")).text();
  const match = /(?:src|href)="(\/static\/[^"]+\.js)"/.exec(html);
  expect(match?.[1], "index.html から /static/*.js が見つからない").toBeDefined();
  const response = await request.get(match?.[1] ?? "");
  expect(response.status()).toBe(200);
  expect(header(response, "content-type")).toContain("javascript");
  const cacheControl = header(response, "cache-control");
  expect(cacheControl).toContain("immutable");
  expect(maxAgeOf(cacheControl)).toBeGreaterThanOrEqual(LONG_MAX_AGE_SECONDS);
});

test("/healthz は 200 を返す(k8s の probe 用)", async ({ request }) => {
  const response = await request.get("/healthz");
  expect(response.status()).toBe(200);
});

test("/api/* は Web のコンテナでは配らない(gateway の持ち物。index.html で代用しない)", async ({
  request,
}) => {
  for (const path of ["/api/calc", "/api/v1/calc"]) {
    const response = await request.post(path, { data: {} });
    expect(response.status(), path).toBe(404);
    expect(await response.text(), path).not.toContain(APP_ROOT);
  }
  const get = await request.get("/api/calc");
  expect(get.status()).toBe(404);
  expect(await get.text()).not.toContain(APP_ROOT);
});
