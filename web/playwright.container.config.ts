// P4-11: Playwright の E2E(コンテナ)。`make web-docker-build` で作ったイメージ(nginx の静的配信。engine.wasm を含む)を
// docker run で起動し、本番の配信(vite preview ではなく nginx)で動くことを確かめる。
// ADR-0313: 既定の計算モードがオンラインになり、コンテナは /api を配らない(404)ため、走らせるのは container.spec.ts だけにする
// (画面の振る舞いの spec は playwright.config.ts が見る)。CSP の下の WASM 計算は、container.spec.ts が /api/pokedex だけを
// pokedex フィクスチャへ転送し、オンラインで種族を引いてからオフラインで計算して確かめる。
// container.spec.ts は nginx の配信設定(SPA のフォールバック・MIME・キャッシュ・gzip・/healthz)を HTTP で確かめる。
// engine.wasm はイメージの中で作るので、web/public/ の成果物(make wasm)は前提にしない。

import { defineConfig, devices } from "@playwright/test";
import { join } from "node:path";
import {
  CONTAINER_PORT,
  CONTAINER_POKEDEX_FIXTURE_PORT,
  containerCommand,
  webRoot,
} from "./e2e/support/serverConfig.ts";

const baseURL = `http://127.0.0.1:${CONTAINER_PORT}`;

export default defineConfig({
  testDir: "./e2e",
  // Playwright が走らせるのは *.spec.ts だけ。e2e/support/ には vitest が走らせる *.test.ts
  // (pokedex フィクスチャの契約テスト。ADR-0307)があり、Playwright の既定の testMatch では
  // それも拾ってしまう(critic 指摘: 実際に make web-e2e-container が壊れることを確認済み)ため明示する。
  testMatch: ["**/container.spec.ts"],
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium-container", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // pokedex フィクスチャ(ADR-0307)。CSP テストが /api/pokedex をここへ転送する。
      command: `node ${join(webRoot, "e2e", "support", "pokedexFixtureServer.mjs")} ${String(CONTAINER_POKEDEX_FIXTURE_PORT)}`,
      cwd: webRoot,
      url: `http://127.0.0.1:${String(CONTAINER_POKEDEX_FIXTURE_PORT)}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: containerCommand(CONTAINER_PORT),
      cwd: webRoot,
      url: `${baseURL}/healthz`,
      reuseExistingServer: false,
      timeout: 60_000,
      // docker CLI は SIGTERM をコンテナへ転送する(--rm で消える)。SIGKILL だとコンテナが残る。
      gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
    },
  ],
});
