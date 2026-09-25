#!/usr/bin/env node
// P4-5(ADR-0301 §5、2026-09-25追記でADR-0204のMasterExportに追従): Web の架空の例データ
// (src/master/exampleSource.ts)を calc-svc のマスタ一式(api/openapi.yaml の MasterExport、
// src/master/exportSnapshot.ts の toCalcSnapshot)の JSON に書き出す。calc-svc をこの出力で起動すれば、
// Web の例データの ID がそのまま API に通る(オンラインの動作確認・P4-6 の E2E)。
// P4-12a(ADR-0303 §4): 同じ例データを balance-svc の read model(pokemon-types・moves・abilities。
// src/master/exportBalanceReadModel.ts)にも、calc のスナップショットと同じディレクトリに書き出す。
//
// 使い方: node scripts/export-example-master.mjs [出力先のパス]
//   省略すると data/generated/web-example-master.json(リポジトリルート基準)に書く。
//   balance の3ファイル(balance-pokemon-types.json・balance-moves.json・balance-abilities.json)は
//   同じディレクトリに書く。data/generated/ は .gitignore 済みで、生成物はコミットしない(ADR-0002)。
//
// 例データは TypeScript(拡張子省略の import・vite.config.ts の @typechart 別名)で書かれているため、
// プレーンな node では import できない。vite.config.ts の設定(alias・fs.allow)をそのまま使う開発サーバーを
// ミドルウェアモードで起動し、ssrLoadModule でモジュールを変換・解決して読む(HTTP では listen しない)。

import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const defaultOutPath = join(webRoot, "..", "data", "generated", "web-example-master.json");

async function loadSnapshots() {
  const server = await createServer({
    root: webRoot,
    logLevel: "error",
    server: { middlewareMode: true },
  });
  try {
    const { exampleMasterSource } = await server.ssrLoadModule("/src/master/exampleSource.ts");
    const { toCalcSnapshot } = await server.ssrLoadModule("/src/master/exportSnapshot.ts");
    const { toBalanceAbilities, toBalanceMoves, toBalancePokemonTypes } = await server.ssrLoadModule(
      "/src/master/exportBalanceReadModel.ts",
    );
    const master = await exampleMasterSource.load();
    return {
      calc: toCalcSnapshot(master),
      balancePokemonTypes: toBalancePokemonTypes(master),
      balanceMoves: toBalanceMoves(master),
      balanceAbilities: toBalanceAbilities(master),
    };
  } finally {
    await server.close();
  }
}

async function main() {
  const outPath = resolve(process.argv[2] ?? defaultOutPath);
  const outDir = dirname(outPath);
  const { calc, balancePokemonTypes, balanceMoves, balanceAbilities } = await loadSnapshots();
  await mkdir(outDir, { recursive: true });
  await writeFile(outPath, JSON.stringify(calc, null, 2));
  await writeFile(join(outDir, "balance-pokemon-types.json"), JSON.stringify(balancePokemonTypes, null, 2));
  await writeFile(join(outDir, "balance-moves.json"), JSON.stringify(balanceMoves, null, 2));
  await writeFile(join(outDir, "balance-abilities.json"), JSON.stringify(balanceAbilities, null, 2));
  console.log(`export-example-master: ${outPath} と balance の3ファイルに書き出した`);
}

await main();
