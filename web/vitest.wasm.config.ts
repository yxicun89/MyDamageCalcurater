import { defineConfig, mergeConfig } from "vitest/config";
import { baseConfig } from "./vite.config.ts";

// 本物の engine.wasm(make wasm の成果物)に Web が組み立てたリクエストを通す結合テスト。
// 前提(engine.wasm / wasm_exec.js)が無ければスキップではなく失敗する(ADR-0300 §8)。
export default mergeConfig(
  baseConfig,
  defineConfig({
    test: {
      environment: "node",
      include: ["src/**/*.wasm.test.ts"],
      testTimeout: 30_000,
    },
  }),
);
