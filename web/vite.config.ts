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
  // /assets/ は gateway が画像配信用に予約している(ADR-0205・CLAUDE.md)。ビルド成果物を同じ接頭辞に置くと
  // gateway 経由(:8080)で JS/CSS が 404 になり白画面になるので、別の接頭辞に出す(ADR-0305)。
  build: { assetsDir: "static" },
  resolve: {
    alias: { "@typechart": typeChartPath },
  },
  server: {
    // web/ の外にあるのは相性表の1ファイルだけ。開発サーバーが読めるものをそれに限る。
    fs: { allow: [searchForWorkspaceRoot(process.cwd()), typeChartPath] },
  },
};

export default defineConfig(({ mode }) => {
  // API_PROXY_TARGET・BALANCE_PROXY_TARGET・POKEDEX_PROXY_TARGET は開発サーバーのプロキシだけが読む Node 側の値。
  // `VITE_` 接頭辞を付けないのでクライアントのバンドルには入らない。VITE_API_BASE_URL(ブラウザで読む基点 URL。
  // api/config.ts)とは別(ADR-0301 §4)。BALANCE_PROXY_TARGET は P4-12a(ADR-0303 §5)の balance API 用、
  // POKEDEX_PROXY_TARGET は PR2(ADR-0307)の pokedex フィクスチャ用で、どちらも /api とは別の転送先に送る。
  const env = loadEnv(mode, process.cwd(), "");
  const apiProxyTarget = env.API_PROXY_TARGET ?? "";
  const balanceProxyTarget = env.BALANCE_PROXY_TARGET ?? "";
  const pokedexProxyTarget = env.POKEDEX_PROXY_TARGET ?? "";
  // /api/balance・/api/pokedex は /api より前に置く(Vite のプロキシは定義順に前方一致で選ぶため、/api が先だと
  // balance・pokedex への要求が calc に行ってしまう)。
  const proxy: Record<string, { target: string; changeOrigin: boolean }> = {};
  if (balanceProxyTarget !== "") {
    proxy["/api/balance"] = { target: balanceProxyTarget, changeOrigin: true };
  }
  if (pokedexProxyTarget !== "") {
    proxy["/api/pokedex"] = { target: pokedexProxyTarget, changeOrigin: true };
  }
  if (apiProxyTarget !== "") {
    proxy["/api"] = { target: apiProxyTarget, changeOrigin: true };
  }
  if (Object.keys(proxy).length === 0) {
    return baseConfig;
  }
  return {
    ...baseConfig,
    server: { ...baseConfig.server, proxy },
  };
});
