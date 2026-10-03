// API 契約(各 openapi.yaml)から Web の型(*.gen.ts)を生成する(ADR-0807)。
//
// 生成物は Git に置かない。package.json の pre* フック(dev・build・typecheck・lint・test・e2e)と
// ルートの `make gen-ts` がこのスクリプトを呼ぶ。出力が無いか、仕様・package-lock.json(生成器の版)・
// このスクリプト自身(生成の手順)が出力より新しいときだけ生成するので、何度呼んでも速い。強制は GEN_FORCE=1。
import { execFileSync } from "node:child_process";
import { existsSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = join(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = join(webRoot, "..");
const bin = (name) => join(webRoot, "node_modules", ".bin", name);

// [仕様(リポジトリのルートから), 出力(web/ から)]
const targets = [
  ["api/openapi.yaml", "src/api/openapi.gen.ts"],
  ["services/balance/api/openapi.yaml", "src/api/balance.gen.ts"],
  ["services/speed/api/openapi.yaml", "src/speed/speed.gen.ts"],
  ["services/judge/api/openapi.yaml", "src/judge/judge.gen.ts"],
];

const lockfile = join(webRoot, "package-lock.json");
const thisScript = fileURLToPath(import.meta.url);
const force = process.env.GEN_FORCE === "1";

function isStale(spec, out) {
  if (force || !existsSync(out)) return true;
  const outTime = statSync(out).mtimeMs;
  return [spec, lockfile, thisScript].some((input) => statSync(input).mtimeMs > outTime);
}

const generated = [];
for (const [specRel, outRel] of targets) {
  const spec = join(repoRoot, specRel);
  const out = join(webRoot, outRel);
  if (!existsSync(spec)) {
    console.error(`gen: 仕様 ${specRel} が無い`);
    process.exit(1);
  }
  if (!isStale(spec, out)) continue;
  execFileSync(bin("openapi-typescript"), [relative(webRoot, spec), "-o", outRel], {
    cwd: webRoot,
    stdio: ["ignore", "ignore", "inherit"],
  });
  generated.push(outRel);
}

if (generated.length > 0) {
  execFileSync(bin("prettier"), ["--write", ...generated], {
    cwd: webRoot,
    stdio: ["ignore", "ignore", "inherit"],
  });
  console.log(`gen: ${generated.join("・")} を生成`);
}
