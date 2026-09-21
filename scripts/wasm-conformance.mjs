#!/usr/bin/env node
// Go/WASM 結果一致テスト(P1-9、ADR-0011 §7)。
//
// 同じ入力ベクタを
//   (a) ネイティブ Go の engine/wasmapi(= engine/cmd/wasmexpect が生成する期待値)
//   (b) ブラウザと同じ engine.wasm を Node + wasm_exec.js で動かしたもの
// に通し、レスポンス JSON が**バイト一致**することを確かめる。
//
//   node scripts/wasm-conformance.mjs
//   node scripts/wasm-conformance.mjs --expected /tmp/expected.json --max-reverse-ms 1000
//
// 前提(engine.wasm / wasm_exec.js / node / go)が揃わないときは、スキップせず必ず失敗する。
// 「未実装ターゲットやスキップの正常終了を成功と数えない」(CLAUDE.md / docs/development-workflow.md)。

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith('--')) die(`不明な引数: ${a}`);
    const key = a.slice(2);
    const next = argv[i + 1];
    if (next === undefined || next.startsWith('--')) die(`--${key} に値がない`);
    out[key] = next;
    i++;
  }
  return out;
}

function die(msg) {
  console.error(`wasm-conformance: ${msg}`);
  process.exit(1);
}

function mustExist(p, what) {
  if (!fs.existsSync(p)) {
    die(`${what} が見つからない: ${p}\n  先に \`make wasm\` を実行すること(スキップはしない)`);
  }
  return p;
}

const args = parseArgs(process.argv.slice(2));
const vectorsPath = mustExist(args.vectors ?? path.join(REPO, 'engine/wasmapi/testdata/vectors.json'), '入力ベクタ');
const wasmPath = mustExist(args.wasm ?? path.join(REPO, 'web/public/engine.wasm'), 'engine.wasm');
const wasmExecPath = mustExist(args['wasm-exec'] ?? path.join(REPO, 'web/public/wasm_exec.js'), 'wasm_exec.js');
const maxReverseMs = Number(args['max-reverse-ms'] ?? 1000);
if (!Number.isFinite(maxReverseMs) || maxReverseMs <= 0) die('--max-reverse-ms が数値でない');

// --- 入力ベクタ -------------------------------------------------------------

const vectorDoc = JSON.parse(fs.readFileSync(vectorsPath, 'utf8'));
if (vectorDoc.schemaVersion !== 1) die(`ベクタの schemaVersion=${vectorDoc.schemaVersion} は未知`);
const vectors = vectorDoc.vectors ?? [];
if (vectors.length === 0) die('ベクタが空');

// --- 期待値(ネイティブ Go)--------------------------------------------------

let expectedPath = args.expected;
let tmpDir;
if (!expectedPath) {
  tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pokecalc-wasm-'));
  expectedPath = path.join(tmpDir, 'expected.json');
  try {
    execFileSync('go', ['run', './cmd/wasmexpect', '-vectors', vectorsPath, '-out', expectedPath], {
      cwd: path.join(REPO, 'engine'),
      stdio: ['ignore', 'inherit', 'inherit'],
    });
  } catch (e) {
    die(`ネイティブ Go の期待値生成に失敗した(engine/cmd/wasmexpect): ${e.message}`);
  }
}
const expected = JSON.parse(fs.readFileSync(mustExist(expectedPath, '期待値'), 'utf8'));

// 期待値そのものが未実装/エラー封筒なら、一致しても意味がないので先に落とす。
for (const v of vectors) {
  const e = expected[v.name];
  if (e === undefined) die(`期待値にベクタ ${v.name} が無い`);
  let parsed;
  try {
    parsed = JSON.parse(e);
  } catch {
    die(`期待値 ${v.name} が JSON ではない: ${e.slice(0, 120)}`);
  }
  if (parsed.result === undefined) {
    die(`期待値 ${v.name} が result を持たない(ネイティブ側が未実装かエラー): ${e.slice(0, 200)}`);
  }
}

// --- WASM の起動 ------------------------------------------------------------

new vm.Script(fs.readFileSync(wasmExecPath, 'utf8'), { filename: wasmExecPath }).runInThisContext();
if (typeof globalThis.Go !== 'function') die('wasm_exec.js が globalThis.Go を定義しなかった');

const go = new globalThis.Go();
let goExited = null;
const instantiateStart = performance.now();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(wasmPath), go.importObject);
const instantiateMs = performance.now() - instantiateStart;
go.run(instance).then(
  () => { goExited = '正常終了'; },
  (err) => { goExited = `異常終了: ${err && err.message ? err.message : err}`; },
);

// 登録完了を待つ(ブラウザでも同じ目印を使う)。
const readyDeadline = Date.now() + 5000;
while (globalThis.pokecalcReady !== true && Date.now() < readyDeadline && goExited === null) {
  await new Promise((r) => setTimeout(r, 5));
}
if (goExited !== null) die(`WASM プログラムが起動直後に終了した(${goExited})`);
if (globalThis.pokecalcReady !== true) die('globalThis.pokecalcReady が立たない(engine/cmd/wasm が未実装)');

const api = globalThis.pokecalc;
for (const fn of ['calc', 'calcBulk', 'calcReverse']) {
  if (typeof api?.[fn] !== 'function') die(`globalThis.pokecalc.${fn} が登録されていない`);
}

// --- 照合 -------------------------------------------------------------------

const failures = [];
const timings = [];

function call(v) {
  const t0 = performance.now();
  const got = api[v.fn](JSON.stringify(v.request));
  const ms = performance.now() - t0;
  if (typeof got !== 'string') {
    failures.push(`${v.name}: 戻り値が文字列でない (${typeof got})`);
    return { got: '', ms };
  }
  timings.push({ name: v.name, fn: v.fn, ms });
  return { got, ms };
}

// 1周目: 期待値とバイト一致するか。
for (const v of vectors) {
  const { got } = call(v);
  const want = expected[v.name];
  if (got !== want) {
    failures.push(
      `${v.name}: Go と WASM の出力が違う\n    native: ${want.slice(0, 300)}\n    wasm  : ${got.slice(0, 300)}`,
    );
  }
}

// 2周目: 逆順に呼び直して、状態を持ち越していない(再入して同じ結果)ことを見る。
for (const v of [...vectors].reverse()) {
  const { got } = call(v);
  if (got !== expected[v.name]) {
    failures.push(`${v.name}: 2回目の呼び出しが1回目と違う(WASM が状態を持ち越している)`);
  }
}

// 異常系: 引数の型違い・壊れた JSON でも文字列の error 封筒を返し、Go の panic を JS へ漏らさない。
// 直後に正常ベクタを呼んで、Promise(go.run)が死んでいない=以後も計算できることを確かめる。
function checkErrorEnvelope(label, got, code) {
  if (typeof got !== 'string') {
    failures.push(`${label}: 戻り値が文字列でない (${typeof got})`);
    return;
  }
  let env;
  try {
    env = JSON.parse(got);
  } catch {
    failures.push(`${label}: 戻り値が JSON ではない: ${got.slice(0, 200)}`);
    return;
  }
  if (env?.result !== undefined || env?.error?.code !== code || typeof env.error.message !== 'string' || env.error.message === '') {
    failures.push(`${label}: error 封筒(code=${code})になっていない: ${got.slice(0, 200)}`);
  }
}
function safeCall(label, fn) {
  try {
    return fn();
  } catch (e) {
    failures.push(`${label}: JS へ例外が漏れた: ${e && e.message ? e.message : e}`);
    return undefined;
  }
}
checkErrorEnvelope('calc()(引数0個)', safeCall('calc()', () => api.calc()), 'invalid_json');
checkErrorEnvelope('calc(123)(数値)', safeCall('calc(123)', () => api.calc(123)), 'invalid_json');
checkErrorEnvelope('calc(null)', safeCall('calc(null)', () => api.calc(null)), 'invalid_json');
checkErrorEnvelope('calc("a","b")(引数2個)', safeCall('calc(a,b)', () => api.calc('a', 'b')), 'invalid_json');
checkErrorEnvelope('calcBulk({})(オブジェクト)', safeCall('calcBulk({})', () => api.calcBulk({})), 'invalid_json');
checkErrorEnvelope('calc("{")(壊れた JSON)', safeCall('calc("{")', () => api.calc('{')), 'invalid_json');
checkErrorEnvelope('calcReverse("{")(壊れた JSON)', safeCall('calcReverse("{")', () => api.calcReverse('{')), 'invalid_json');
{
  const v = vectors[0];
  const { got } = call(v);
  if (got !== expected[v.name]) {
    failures.push(`${v.name}: 異常系の呼び出しの後で結果が変わった(Promise が死んだ/状態が壊れた)`);
  }
}
// go.run の Promise の解決はマイクロタスク/イベントループ側なので、少し待ってから確認する。
await new Promise((r) => setTimeout(r, 50));
if (goExited !== null) failures.push(`異常系の呼び出し後に WASM プログラムが終了した(${goExited})`);

// 逆算の応答時間(ブラウザで対話的に使えること。ADR-0011 §9)。
const reverseWorst = timings
  .filter((t) => t.fn === 'calcReverse')
  .sort((a, b) => b.ms - a.ms)[0];
if (reverseWorst && reverseWorst.ms > maxReverseMs) {
  failures.push(`逆算が遅すぎる: ${reverseWorst.name} が ${reverseWorst.ms.toFixed(0)}ms(上限 ${maxReverseMs}ms)`);
}

// --- 結果 -------------------------------------------------------------------

const wasmBytes = fs.statSync(wasmPath).size;
console.log(`wasm-conformance: ${vectors.length} ベクタ × 2周`);
console.log(`  engine.wasm  : ${(wasmBytes / 1024 / 1024).toFixed(2)} MB`);
console.log(`  instantiate  : ${instantiateMs.toFixed(1)} ms`);
if (reverseWorst) console.log(`  逆算の最悪値 : ${reverseWorst.ms.toFixed(1)} ms (${reverseWorst.name})`);

if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });

if (failures.length > 0) {
  console.error(`\nwasm-conformance: ${failures.length} 件の不一致`);
  for (const f of failures) console.error(`  - ${f}`);
  process.exitCode = 1;
} else {
  console.log('wasm-conformance: Go とバイト一致 (OK)');
}
