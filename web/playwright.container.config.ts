// P4-11: Playwright の E2E(コンテナ)。`make web-docker-build` で作ったイメージ(nginx の静的配信。engine.wasm を含む)を
// docker run で起動し、オフライン(WASM)の spec をそのまま走らせる(vite preview ではなく本番の配信で動くことを確かめる)。
// container.spec.ts は nginx の配信設定(SPA のフォールバック・MIME・キャッシュ・gzip・/healthz)を HTTP で確かめる。
// engine.wasm はイメージの中で作るので、web/public/ の成果物(make wasm)は前提にしない。

import { defineConfig, devices } from "@playwright/test";
import { CONTAINER_PORT, containerCommand, webRoot } from "./e2e/support/serverConfig.ts";

const baseURL = `http://127.0.0.1:${CONTAINER_PORT}`;

export default defineConfig({
  testDir: "./e2e",
  // オンライン(API)の spec は calc-svc が要る。/api は gateway の持ち物で、Web のコンテナは転送しない。
  testIgnore: ["**/online.spec.ts", "**/balance.spec.ts"],
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium-container", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: containerCommand(CONTAINER_PORT),
    cwd: webRoot,
    url: `${baseURL}/healthz`,
    reuseExistingServer: false,
    timeout: 60_000,
    // docker CLI は SIGTERM をコンテナへ転送する(--rm で消える)。SIGKILL だとコンテナが残る。
    gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
  },
});
