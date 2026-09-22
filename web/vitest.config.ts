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
      // domPolyfills.ts を先に読み込む(react-dom の import 前に window.AnimationEvent を足す必要があるため。
      // 理由は src/test/domPolyfills.ts のコメントを参照)。
      setupFiles: ["src/test/domPolyfills.ts", "src/test/setup.ts"],
    },
  }),
);
