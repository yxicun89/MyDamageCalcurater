#!/usr/bin/env node
// PR2(ADR-0307): pokedexFixture.ts(純粋関数)を node:http で待ち受ける。`web-e2e-online` は
// pokedex-svc(MySQL 必須)を立てられないので、Web の架空の例データから公開 API の応答を返す
// 軽量サーバーを代わりに立て、vite preview の /api/pokedex をここへ転送する(POKEDEX_PROXY_TARGET)。
//
// 例データ・フィクスチャは TypeScript(拡張子省略の import・vite.config.ts の @typechart 別名)で
// 書かれているため、プレーンな node では import できない。scripts/export-example-master.mjs と同じ手口で、
// vite.config.ts の設定(alias・fs.allow)をそのまま使う開発サーバーをミドルウェアモードで起動し、
// ssrLoadModule でモジュールを変換・解決して読む(HTTP では listen しない。listen するのはこのファイル自身)。
//
// 使い方: node e2e/support/pokedexFixtureServer.mjs [ポート番号]
//   省略すると e2e/support/serverConfig.ts の POKEDEX_FIXTURE_PORT を使う。

import { createServer } from "node:http";
import { fileURLToPath } from "node:url";
import { createServer as createViteServer } from "vite";

const webRoot = fileURLToPath(new URL("../..", import.meta.url));

async function loadFixture() {
  const server = await createViteServer({
    root: webRoot,
    logLevel: "error",
    server: { middlewareMode: true },
  });
  // critic指摘: ssrLoadModule が失敗したとき vite サーバーを閉じ忘れないよう、export-example-master.mjs の
  // loadSnapshots と同じく try/finally にする。
  try {
    const { exampleMasterSource } = await server.ssrLoadModule("/src/master/exampleSource.ts");
    const { handlePokedexRequest } = await server.ssrLoadModule("/e2e/support/pokedexFixture.ts");
    const master = await exampleMasterSource.load();
    return { master, handlePokedexRequest };
  } finally {
    await server.close();
  }
}

/** node:http の IncomingMessage のヘッダ(小文字化済み)を FixtureRequest のヘッダ形に写す。 */
function headersOf(req) {
  const headers = {};
  for (const [name, value] of Object.entries(req.headers)) {
    if (value === undefined) {
      continue;
    }
    // 同名ヘッダが複数来ると node は配列にまとめる。フィクスチャの検証は文字列前提なので連結する
    // (実際の重複は invalid_header の対象だが、その判定はフィクスチャ側の UUID チェックに任せる)。
    headers[name.toLowerCase()] = Array.isArray(value) ? value.join(",") : value;
  }
  return headers;
}

async function main() {
  const { POKEDEX_FIXTURE_PORT } = await import("./serverConfig.ts");
  const port = Number(process.argv[2] ?? POKEDEX_FIXTURE_PORT);
  const { master, handlePokedexRequest } = await loadFixture();

  const httpServer = createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://127.0.0.1:${String(port)}`);
    const request = {
      method: (req.method ?? "GET").toUpperCase(),
      path: url.pathname,
      query: url.searchParams,
      headers: headersOf(req),
    };
    let response;
    try {
      response = handlePokedexRequest(master, request);
    } catch (error) {
      response = {
        status: 500,
        body: { code: "internal", message: error instanceof Error ? error.message : String(error) },
      };
    }
    const payload = JSON.stringify(response.body);
    res.writeHead(response.status, { "Content-Type": "application/json; charset=utf-8" });
    res.end(payload);
  });

  await new Promise((resolve) => {
    httpServer.listen(port, "127.0.0.1", resolve);
  });
  console.log(`pokedexFixtureServer: http://127.0.0.1:${String(port)} で待ち受け中`);
}

await main();
