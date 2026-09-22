// P4-12a: Playwright の E2E(タイプバランス = balance API)。ADR-0303 §4・§5: Web の例データを balance の read model
// (pokemon-types・moves・abilities)にも書き出し(scripts/export-example-master.mjs が calc のスナップショットと同じ
// ディレクトリに balance-*.json を書く)、その出力で balance-svc を起動する。vite preview は /api/balance を
// balance-svc へ転送する(BALANCE_PROXY_TARGET。vite.config.ts)。要 Go(services/ の go.work で go run する)。
// 書き出し先は data/generated/(.gitignore 済み。ADR-0002)。

import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";
import {
  BALANCE_PORT,
  BALANCE_SVC_PORT,
  assertWasmArtifacts,
  previewCommand,
  repoRoot,
  webRoot,
} from "./e2e/support/serverConfig.ts";

// 本番ビルドは web/public/ の engine.wasm を写すので、balance の画面だけでも前提は同じ。
assertWasmArtifacts();

const baseURL = `http://127.0.0.1:${String(BALANCE_PORT)}`;
const balanceSvcURL = `http://127.0.0.1:${String(BALANCE_SVC_PORT)}`;
const outDir = join(repoRoot, "data", "generated", "e2e-balance");
const masterPath = join(outDir, "web-example-master.json");

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/balance.spec.ts"],
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium-balance", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // 例データを書き出してから balance-svc を起動する(書き出しに失敗したら起動しない)。
      command: `node ${join(webRoot, "scripts", "export-example-master.mjs")} ${masterPath} && go run ./balance/cmd/api`,
      cwd: join(repoRoot, "services"),
      url: `${balanceSvcURL}/healthz`,
      reuseExistingServer: false,
      timeout: 180_000,
      env: {
        PORT: String(BALANCE_SVC_PORT),
        BALANCE_POKEMON_TYPES_PATH: join(outDir, "balance-pokemon-types.json"),
        BALANCE_MOVES_PATH: join(outDir, "balance-moves.json"),
        BALANCE_ABILITIES_PATH: join(outDir, "balance-abilities.json"),
      },
    },
    {
      command: previewCommand(BALANCE_PORT),
      cwd: webRoot,
      url: baseURL,
      reuseExistingServer: false,
      timeout: 120_000,
      env: { BALANCE_PROXY_TARGET: balanceSvcURL },
    },
  ],
});
