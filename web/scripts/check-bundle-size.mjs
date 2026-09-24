// Web の初期ロードの予算(docs/design.md「パフォーマンス予算」: JS ≤ 300KB gzip、WASM 除く)を
// vite build の成果物で確かめる。超えたら失敗する。
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const JS_BUDGET_GZIP_BYTES = 300 * 1024;
const distAssets = fileURLToPath(new URL("../dist/static/", import.meta.url));

function listJsFiles(dir) {
  return readdirSync(dir)
    .map((name) => join(dir, name))
    .filter((path) => statSync(path).isFile() && path.endsWith(".js"));
}

let files;
try {
  files = listJsFiles(distAssets);
} catch {
  console.error("check-bundle-size: dist/static が無い(先に vite build を実行する)");
  process.exit(1);
}
if (files.length === 0) {
  console.error("check-bundle-size: dist/static に JS が無い");
  process.exit(1);
}
const total = files.reduce((sum, path) => sum + gzipSync(readFileSync(path), { level: 9 }).length, 0);
console.log(`check-bundle-size: JS ${total} bytes (gzip) / 予算 ${JS_BUDGET_GZIP_BYTES} bytes`);
if (total > JS_BUDGET_GZIP_BYTES) {
  console.error("check-bundle-size: 予算超過");
  process.exit(1);
}
