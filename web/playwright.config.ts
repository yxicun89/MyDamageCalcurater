// P4-6: Playwright の E2E(オフライン = WASM、既定の計算モード)。docs/test-strategy.md「E2E(Playwright)」。
// 本番ビルド(vite build → vite preview)を相手にし、バックエンドは起動しない(WASM だけで計算できることを確かめる)。
// オンライン(API)は playwright.online.config.ts(calc-svc も起動する)。
// 自動化するブラウザは chromium だけ。Safari・実機の確認は人間の作業(plan.md のブロッカー)。

import { defineConfig, devices } from "@playwright/test";
import { OFFLINE_PORT, assertWasmArtifacts, previewCommand, webRoot } from "./e2e/support/serverConfig.ts";

assertWasmArtifacts();

const baseURL = `http://127.0.0.1:${OFFLINE_PORT}`;

export default defineConfig({
  testDir: "./e2e",
  // Playwright が走らせるのは *.spec.ts だけ。e2e/support/ には vitest が走らせる *.test.ts
  // (pokedex フィクスチャの契約テスト。ADR-0307)があり、Playwright の既定の testMatch では
  // それも拾ってしまうため明示する。
  testMatch: ["**/*.spec.ts"],
  // オンライン専用の spec は calc-svc が要るので、こちらでは走らせない。
  // container.spec.ts は nginx(web/nginx.conf)の配信設定を確かめるもので、preview では成り立たない
  // (playwright.container.config.ts だけが走らせる)。
  testIgnore: ["**/online.spec.ts", "**/container.spec.ts", "**/balance.spec.ts"],
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: previewCommand(OFFLINE_PORT),
    cwd: webRoot,
    url: baseURL,
    // 別の設定(オンライン)で動いているサーバーを取り違えないよう、既存のサーバーは使わない。
    reuseExistingServer: false,
    timeout: 120_000,
    // API への転送を持たない preview にする(オフラインの前提。開発者の環境変数を持ち込まない)。
    env: { API_PROXY_TARGET: "" },
  },
});
