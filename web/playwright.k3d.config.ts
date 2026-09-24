// k3d 上で動いている全体(ブラウザ → localhost:8080 → gateway → web / calc / pokedex)を、利用者と同じ入口から
// 実ブラウザで確かめる(docs/verify-m1.md §3)。サーバーは起動しない(`make up` と各デプロイ済みが前提)。
import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.K3D_URL ?? "http://localhost:8080";

export default defineConfig({
  testDir: "./e2e-k3d",
  fullyParallel: false,
  forbidOnly: process.env.CI !== undefined,
  retries: 0,
  reporter: "list",
  timeout: 60_000,
  use: { baseURL, trace: "retain-on-failure" },
  projects: [{ name: "chromium-k3d", use: { ...devices["Desktop Chrome"] } }],
});
