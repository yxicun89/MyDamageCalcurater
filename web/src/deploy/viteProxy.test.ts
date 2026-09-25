// @vitest-environment node
// P4-12a(ADR-0303 §5): 開発サーバー・preview は /api/balance を BALANCE_PROXY_TARGET に転送する
// (/api の API_PROXY_TARGET と別。どちらも `VITE_` 接頭辞なし)。/api/balance は /api より先に照合させる
// (Vite のプロキシは定義順に前方一致で選ぶため、/api が先だと balance への要求が calc に行く)。
// PR2(ADR-0307 §4): 同じ理由で /api/pokedex(POKEDEX_PROXY_TARGET)も /api より先に置く
// (critic 指摘: この振り分けが単体テストで守られておらず、削除しても vitest が全緑のままだった)。

import type { UserConfig } from "vite";
import { afterEach, describe, expect, test } from "vitest";
import viteConfig from "../../vite.config";

const ENV_KEYS = ["API_PROXY_TARGET", "BALANCE_PROXY_TARGET", "POKEDEX_PROXY_TARGET"] as const;
const saved = Object.fromEntries(ENV_KEYS.map((key) => [key, process.env[key]]));

afterEach(() => {
  for (const key of ENV_KEYS) {
    const value = saved[key];
    if (value === undefined) {
      Reflect.deleteProperty(process.env, key);
    } else {
      process.env[key] = value;
    }
  }
});

function resolveConfig(env: Partial<Record<(typeof ENV_KEYS)[number], string>>): UserConfig {
  for (const key of ENV_KEYS) {
    const value = env[key];
    if (value === undefined) {
      Reflect.deleteProperty(process.env, key);
    } else {
      process.env[key] = value;
    }
  }
  if (typeof viteConfig !== "function") {
    throw new Error("vite.config.ts の既定の export は mode を受け取る関数");
  }
  return viteConfig({ mode: "production", command: "serve" });
}

function proxyOf(config: UserConfig): Record<string, unknown> {
  const proxy = config.server?.proxy;
  return proxy ?? {};
}

describe("vite.config.ts のプロキシ", () => {
  test("BALANCE_PROXY_TARGET を設定すると /api/balance をそこへ転送する", () => {
    const proxy = proxyOf(resolveConfig({ BALANCE_PROXY_TARGET: "http://127.0.0.1:18318" }));
    expect(proxy["/api/balance"]).toMatchObject({ target: "http://127.0.0.1:18318", changeOrigin: true });
  });

  test("両方を設定すると /api/balance を /api より先に置き、それぞれの転送先へ送る", () => {
    const proxy = proxyOf(
      resolveConfig({
        API_PROXY_TARGET: "http://127.0.0.1:18317",
        BALANCE_PROXY_TARGET: "http://127.0.0.1:18318",
      }),
    );
    expect(Object.keys(proxy)).toEqual(["/api/balance", "/api"]);
    expect(proxy["/api"]).toMatchObject({ target: "http://127.0.0.1:18317" });
    expect(proxy["/api/balance"]).toMatchObject({ target: "http://127.0.0.1:18318" });
  });

  // PR2(ADR-0307 §4、critic指摘): /api/pokedex も /api より先に置く(pokedexフィクスチャへの振り分け)。
  test("POKEDEX_PROXY_TARGET を設定すると /api/pokedex をそこへ転送する", () => {
    const proxy = proxyOf(resolveConfig({ POKEDEX_PROXY_TARGET: "http://127.0.0.1:18319" }));
    expect(proxy["/api/pokedex"]).toMatchObject({ target: "http://127.0.0.1:18319", changeOrigin: true });
  });

  test("3つとも設定すると /api/balance → /api/pokedex → /api の順に置き、それぞれの転送先へ送る", () => {
    const proxy = proxyOf(
      resolveConfig({
        API_PROXY_TARGET: "http://127.0.0.1:18317",
        BALANCE_PROXY_TARGET: "http://127.0.0.1:18318",
        POKEDEX_PROXY_TARGET: "http://127.0.0.1:18319",
      }),
    );
    expect(Object.keys(proxy)).toEqual(["/api/balance", "/api/pokedex", "/api"]);
    expect(proxy["/api"]).toMatchObject({ target: "http://127.0.0.1:18317" });
    expect(proxy["/api/balance"]).toMatchObject({ target: "http://127.0.0.1:18318" });
    expect(proxy["/api/pokedex"]).toMatchObject({ target: "http://127.0.0.1:18319" });
  });

  test("POKEDEX_PROXY_TARGET と API_PROXY_TARGET だけなら /api/pokedex を /api より先に置く", () => {
    const proxy = proxyOf(
      resolveConfig({
        API_PROXY_TARGET: "http://127.0.0.1:18317",
        POKEDEX_PROXY_TARGET: "http://127.0.0.1:18319",
      }),
    );
    expect(Object.keys(proxy)).toEqual(["/api/pokedex", "/api"]);
    expect(proxy["/api"]).toMatchObject({ target: "http://127.0.0.1:18317" });
    expect(proxy["/api/pokedex"]).toMatchObject({ target: "http://127.0.0.1:18319" });
  });

  test("API_PROXY_TARGET だけなら従来どおり /api だけを転送する", () => {
    const proxy = proxyOf(resolveConfig({ API_PROXY_TARGET: "http://127.0.0.1:18317" }));
    expect(Object.keys(proxy)).toEqual(["/api"]);
  });

  test("どちらも未設定ならプロキシを置かない", () => {
    expect(Object.keys(proxyOf(resolveConfig({})))).toEqual([]);
  });
});
