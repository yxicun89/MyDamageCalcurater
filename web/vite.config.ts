import { fileURLToPath } from "node:url";
import { defineConfig, loadEnv, searchForWorkspaceRoot, type UserConfig } from "vite";
import react from "@vitejs/plugin-react";

// タイプ相性表は P1-13 のデータ(testdata/golden/typechart.json)を正とし、複製せずに読む(ADR-0300 §3)。
const typeChartPath = fileURLToPath(new URL("../testdata/golden/typechart.json", import.meta.url));

/**
 * 開発サーバー・ビルド・テストに共通の設定。vitest の設定(vitest.config.ts / vitest.wasm.config.ts)はこれに足す
 * (既定の export は mode ごとの環境変数を読む関数なので、mergeConfig にはこちらを渡す)。
 */
export const baseConfig: UserConfig = {
  plugins: [react()],
  resolve: {
    alias: { "@typechart": typeChartPath },
  },
  server: {
    // web/ の外にあるのは相性表の1ファイルだけ。開発サーバーが読めるものをそれに限る。
    fs: { allow: [searchForWorkspaceRoot(process.cwd()), typeChartPath] },
  },
};

export default defineConfig(({ mode }) => {
  // API_PROXY_TARGET は開発サーバーのプロキシだけが読む Node 側の値。`VITE_` 接頭辞を付けないので
  // クライアントのバンドルには入らない。VITE_API_BASE_URL(ブラウザで読む基点 URL。api/config.ts)とは別(ADR-0301 §4)。
  const env = loadEnv(mode, process.cwd(), "");
  const proxyTarget = env.API_PROXY_TARGET ?? "";
  // 開発時、/api を calc-svc(または gateway)へ転送する(ADR-0301 §4)。未設定(空)なら転送しない
  // (VITE_API_BASE_URL の既定 "/" のまま、同じオリジンに /api があるものとして扱う)。
  if (proxyTarget === "") {
    return baseConfig;
  }
  return {
    ...baseConfig,
    server: { ...baseConfig.server, proxy: { "/api": { target: proxyTarget, changeOrigin: true } } },
  };
});
