import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

// 画面とロジックの単体テスト。engine は fake に差し替える(本物の engine.wasm は vitest.wasm.config.ts)。
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      include: ["src/**/*.test.{ts,tsx}"],
      exclude: ["src/**/*.wasm.test.ts"],
      setupFiles: ["src/test/setup.ts"],
    },
  }),
);
