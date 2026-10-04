// Web の初期ロードの予算(docs/design.md「パフォーマンス予算」: JS ≤ 300KB・CSS ≤ 30KB gzip、WASM 除く)を
// vite build の成果物で確かめる。超えたら失敗する。
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const JS_BUDGET_GZIP_BYTES = 300 * 1024;
const CSS_BUDGET_GZIP_BYTES = 30 * 1024;
const distAssets = fileURLToPath(new URL("../dist/static/", import.meta.url));

function listFiles(dir) {
  return readdirSync(dir)
    .map((name) => join(dir, name))
    .filter((path) => statSync(path).isFile());
}

function gzipTotal(files) {
  return files.reduce((sum, path) => sum + gzipSync(readFileSync(path), { level: 9 }).length, 0);
}

let jsFiles;
let cssFiles;
try {
  const files = listFiles(distAssets);
  jsFiles = files.filter((path) => path.endsWith(".js"));
  cssFiles = files.filter((path) => path.endsWith(".css"));
} catch {
  console.error("check-bundle-size: dist/static が無い(先に vite build を実行する)");
  process.exit(1);
}
if (jsFiles.length === 0) {
  console.error("check-bundle-size: dist/static に JS が無い");
  process.exit(1);
}
if (cssFiles.length === 0) {
  console.error("check-bundle-size: dist/static に CSS が無い");
  process.exit(1);
}

let failed = false;
for (const [label, files, budget] of [
  ["JS", jsFiles, JS_BUDGET_GZIP_BYTES],
  ["CSS", cssFiles, CSS_BUDGET_GZIP_BYTES],
]) {
  const total = gzipTotal(files);
  console.log(`check-bundle-size: ${label} ${total} bytes (gzip) / 予算 ${budget} bytes`);
  if (total > budget) {
    console.error(`check-bundle-size: ${label} が予算超過`);
    failed = true;
  }
}
if (failed) {
  process.exit(1);
}
