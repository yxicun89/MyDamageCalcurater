import { defineConfig, mergeConfig } from "vitest/config";
import { baseConfig } from "./vite.config.ts";

// 画面とロジックの単体テスト。engine は fake に差し替える(本物の engine.wasm は vitest.wasm.config.ts)。
export default mergeConfig(
  baseConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      include: ["src/**/*.test.{ts,tsx}"],
      exclude: ["src/**/*.wasm.test.ts"],
      setupFiles: ["src/test/setup.ts"],
    },
  }),
);
