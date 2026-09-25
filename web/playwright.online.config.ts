// P4-6: Playwright の E2E(オンライン = API)。ADR-0301 §5・§6・ADR-0204: Web の例データを calc-svc の
// マスタ一式(MasterExport。相性表も含む)に書き出し(scripts/export-example-master.mjs)、その出力で
// calc-svc を起動する。vite preview は /api を calc-svc へ転送する(API_PROXY_TARGET。vite.config.ts)。
// 要 Go(services/ を go run する)。書き出し先は data/generated/(.gitignore 済み。ADR-0002)。
// 相性表を別出しする廃止済みの環境変数(services/calc/cmd/calc/main.go 参照)は渡さない
// (設定されていると calc-svc が起動しない)。
//
// PR2(ADR-0307): 「オンライン」のマスタは calc-svc ではなく pokedex-svc の公開 API から読む
// (createOnlineMasterSource)。pokedex-svc(MySQL 必須)は `web-e2e-online` では立てられないので、
// Web の例データから公開 API の応答を返す軽量フィクスチャ(pokedexFixtureServer.mjs)を3つ目の
// webServer として起動し、vite preview の /api/pokedex をそちらへ転送する(POKEDEX_PROXY_TARGET)。

import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";
import {
  CALC_SVC_PORT,
  ONLINE_PORT,
  POKEDEX_FIXTURE_PORT,
  assertWasmArtifacts,
  previewCommand,
  repoRoot,
  webRoot,
} from "./e2e/support/serverConfig.ts";

assertWasmArtifacts();

const baseURL = `http://127.0.0.1:${ONLINE_PORT}`;
const calcSvcURL = `http://127.0.0.1:${CALC_SVC_PORT}`;
const pokedexFixtureURL = `http://127.0.0.1:${POKEDEX_FIXTURE_PORT}`;
const masterPath = join(repoRoot, "data", "generated", "e2e-web-example-master.json");

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/online.spec.ts"],
  fullyParallel: true,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium-online", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // 例データを書き出してから calc-svc を起動する(書き出しに失敗したら起動しない)。
      command: `node ${join(webRoot, "scripts", "export-example-master.mjs")} ${masterPath} && go run ./calc/cmd/calc`,
      cwd: join(repoRoot, "services"),
      url: `${calcSvcURL}/healthz`,
      reuseExistingServer: false,
      timeout: 180_000,
      env: {
        CALC_ADDR: `127.0.0.1:${CALC_SVC_PORT}`,
        CALC_MASTER_PATH: masterPath,
      },
    },
    {
      // pokedex フィクスチャ(ADR-0307)。calc-svc・vite preview と並行に起動してよい(依存が無い)。
      command: `node ${join(webRoot, "e2e", "support", "pokedexFixtureServer.mjs")} ${String(POKEDEX_FIXTURE_PORT)}`,
      cwd: webRoot,
      url: `${pokedexFixtureURL}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: previewCommand(ONLINE_PORT),
      cwd: webRoot,
      url: baseURL,
      reuseExistingServer: false,
      timeout: 120_000,
      env: { API_PROXY_TARGET: calcSvcURL, POKEDEX_PROXY_TARGET: pokedexFixtureURL },
    },
  ],
});
