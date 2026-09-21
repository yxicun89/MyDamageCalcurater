import { fileURLToPath } from "node:url";
import { defineConfig, searchForWorkspaceRoot } from "vite";
import react from "@vitejs/plugin-react";

// タイプ相性表は P1-13 のデータ(testdata/golden/typechart.json)を正とし、複製せずに読む(ADR-0300 §3)。
const typeChartPath = fileURLToPath(new URL("../testdata/golden/typechart.json", import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@typechart": typeChartPath },
  },
  server: {
    // web/ の外にあるのは相性表の1ファイルだけ。開発サーバーが読めるものをそれに限る。
    fs: { allow: [searchForWorkspaceRoot(process.cwd()), typeChartPath] },
  },
});
