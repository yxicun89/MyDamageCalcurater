// issue #111(パッケージ D18): importer の PVC の容量計測と保持/prune。
// 実行: node --test tools/importer/prune.test.mjs
// 実装前(spec-writer)は prune.mjs が無いため失敗する。架空の複数版ツリーだけを使い、実データは使わない。
//
// 保持方針(ADR-0104 追記で固定する):
//  - source(calc / showdown / pokeapi)ごとに「config.json が固定する現在版」+「直前の成功版」(= 既定 2 世代)を残す。
//    直前の成功版は成功台帳 `<generated>/.import-success.json`(recordSuccess が書く)で決める。
//    失敗 run が作っただけの版は台帳に載らないので「直前の成功版」の枠を奪わない。
//  - 削除対象は `.cache/<source>/<版>` と `<source>/<版>`(snapshot)のうち保持しない版のディレクトリだけ。
//  - reports/ は import-<UTC時刻>.json を新しい順に 52 件残す。`latest*` は常に残す。
//  - `upstream/`・`.import.lock`・成功台帳・`.partial-` を含む名前・ディレクトリでないもの・知らない top-level は触らない。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, symlinkSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import {
  checkReserve,
  InsufficientSpaceError,
  planPrune,
  pruneGenerated,
  recordSuccess,
} from './prune.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const SOURCES = ['calc', 'showdown', 'pokeapi'];
const CURRENT = { calc: 'c3', showdown: 's3', pokeapi: 'p3' };

const put = (root, rel, bytes = 10) => {
  const path = join(root, rel);
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, Buffer.alloc(bytes, 1));
};

const reportName = (i) => `import-2026${String(Math.floor(i / 28) + 1).padStart(2, '0')}${String((i % 28) + 1).padStart(2, '0')}T000000Z.json`;

// 版 1〜3(3 が現在)の cache・snapshot を作る。pokeapi/showdown は cache あり、calc は snapshot のみ。
const seed = ({ reports = 0 } = {}) => {
  const root = mkdtempSync(join(tmpdir(), 'importer-prune-'));
  for (const v of ['1', '2', '3']) {
    put(root, `calc/c${v}/snapshot.json`);
    put(root, `showdown/s${v}/snapshot.json`);
    put(root, `pokeapi/p${v}/snapshot.json`);
    put(root, `.cache/showdown/s${v}/src/dist/sim/dex.js`, 100);
    put(root, `.cache/showdown/s${v}/source.tar.gz`, 200);
    put(root, `.cache/pokeapi/p${v}/pokemon.csv`, 50);
  }
  put(root, 'upstream/latest.json');
  put(root, '.import.lock', 0);
  put(root, 'reports/latest.json');
  put(root, 'reports/latest-summary.txt');
  for (let i = 0; i < reports; i++) put(root, `reports/${reportName(i)}`);
  return root;
};

const success = (root, history) => {
  // history: 古い順の版の列。各 run の成功を記録する。
  for (const v of history) {
    recordSuccess({ generatedDir: root, versions: { calc: `c${v}`, showdown: `s${v}`, pokeapi: `p${v}` }, now: new Date() });
  }
};

const config = (sources = CURRENT) => ({ sources });
const exists = (root, rel) => existsSync(join(root, rel));
const done = (root) => rmSync(root, { recursive: true, force: true });

test('現在版+直前の成功版を残し、それより古い cache と snapshot だけを削除候補にする', () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.deepEqual([...plan.remove].sort(), [
    '.cache/pokeapi/p1',
    '.cache/showdown/s1',
    'calc/c1',
    'pokeapi/p1',
    'showdown/s1',
  ]);
  for (const kept of ['calc/c3', 'calc/c2', '.cache/showdown/s2', '.cache/showdown/s3', 'pokeapi/p2']) {
    assert.ok(!plan.remove.includes(kept), kept);
  }
  done(root);
});

test('planPrune は削除しない(列挙だけ)', () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  planPrune({ generatedDir: root, config: config() });
  assert.ok(exists(root, 'calc/c1/snapshot.json'));
  done(root);
});

test('prune 後も現在版・直前の成功版・upstream・latest*・ロック・台帳が残る', async () => {
  const root = seed({ reports: 3 });
  success(root, ['1', '2', '3']);
  await pruneGenerated({ generatedDir: root, config: config() });
  for (const rel of [
    'calc/c3/snapshot.json', 'calc/c2/snapshot.json',
    'showdown/s3/snapshot.json', 'showdown/s2/snapshot.json',
    '.cache/showdown/s3/src/dist/sim/dex.js', '.cache/showdown/s2/source.tar.gz',
    '.cache/pokeapi/p3/pokemon.csv', '.cache/pokeapi/p2/pokemon.csv',
    'upstream/latest.json', 'reports/latest.json', 'reports/latest-summary.txt',
    '.import.lock', '.import-success.json',
  ]) {
    assert.ok(exists(root, rel), `${rel} は残る`);
  }
  for (const rel of ['calc/c1', 'showdown/s1', 'pokeapi/p1', '.cache/showdown/s1', '.cache/pokeapi/p1']) {
    assert.ok(!exists(root, rel), `${rel} は消える`);
  }
  done(root);
});

test('失敗 run だけが作った版(台帳に無い)は「直前の成功版」の枠を奪わない', () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  put(root, 'showdown/s9/snapshot.json'); // 失敗 run の残骸(config は s3 のまま)
  put(root, '.cache/showdown/s9/source.tar.gz');
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.ok(plan.remove.includes('showdown/s9'), '台帳に無い版は再生成できるので削除候補');
  assert.ok(!plan.remove.includes('showdown/s2'), '直前の成功版 s2 は残す');
  done(root);
});

test('config の現在版が台帳に未記録でも現在版は消さない(初回 run・台帳消失)', () => {
  const root = seed();
  // 台帳なし: 現在版だけは必ず残り、他は保持の根拠が無いので候補
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.ok(!plan.remove.some((p) => p.endsWith('/c3') || p.endsWith('/s3') || p.endsWith('/p3')));
  done(root);
});

test('現在版の snapshot が無い(新版の取得に失敗した)なら何も消さずに例外', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  rmSync(join(root, 'showdown/s3'), { recursive: true });
  await assert.rejects(() => pruneGenerated({ generatedDir: root, config: config() }), /現在版|snapshot/);
  assert.ok(exists(root, 'showdown/s1/snapshot.json'), '旧成功版を消さない');
  assert.ok(exists(root, 'calc/c1/snapshot.json'));
  done(root);
});

test('config の版名が相対パス・区切りを含むなら拒否する(パス脱出)', () => {
  const root = seed();
  assert.throws(() => planPrune({ generatedDir: root, config: config({ ...CURRENT, showdown: '../s3' }) }));
  assert.throws(() => planPrune({ generatedDir: root, config: config({ ...CURRENT, calc: 'a/b' }) }));
  done(root);
});

test('.partial- を含む名前(実行中の一時ディレクトリ)は削除しない', () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  put(root, '.cache/showdown/s3.partial-abcd1234/x');
  put(root, '.cache/showdown/s7.partial-abcd1234/x');
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.ok(!plan.remove.some((p) => p.includes('.partial-')));
  done(root);
});

test('知らない top-level・ディレクトリでないものには触らない', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  put(root, 'other/keep.txt');
  put(root, 'local-override/name_ja_overrides.json');
  put(root, 'calc/README.txt');
  await pruneGenerated({ generatedDir: root, config: config() });
  for (const rel of ['other/keep.txt', 'local-override/name_ja_overrides.json', 'calc/README.txt']) assert.ok(exists(root, rel), rel);
  done(root);
});

test('report は新しい順に 52 件残し、古いものを消す。latest* は常に残す', async () => {
  const root = seed({ reports: 60 });
  success(root, ['1', '2', '3']);
  await pruneGenerated({ generatedDir: root, config: config() });
  const left = readdirSync(join(root, 'reports')).filter((n) => /^import-.*\.json$/.test(n)).sort();
  assert.equal(left.length, 52);
  assert.equal(left[0], [...Array(60).keys()].map(reportName).sort()[8], '古い 8 件が消え、新しい 52 件が残る');
  assert.ok(exists(root, 'reports/latest.json'));
  assert.ok(exists(root, 'reports/latest-summary.txt'));
  done(root);
});

test('report が 52 件以下なら 1 件も消さない(境界)', async () => {
  const root = seed({ reports: 52 });
  success(root, ['1', '2', '3']);
  await pruneGenerated({ generatedDir: root, config: config() });
  assert.equal(readdirSync(join(root, 'reports')).filter((n) => /^import-/.test(n)).length, 52);
  done(root);
});

test('回収 byte 数・削除した相対パス・残量だけをログへ出し、絶対パスは出さない', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  const lines = [];
  const res = await pruneGenerated({
    generatedDir: root,
    config: config(),
    log: (m) => lines.push(m),
    statfs: () => ({ totalBytes: 2000, freeBytes: 1500 }),
  });
  // calc/c1 10 + showdown/s1 10 + pokeapi/p1 10 + .cache/showdown/s1 (100+200) + .cache/pokeapi/p1 50 = 380
  assert.equal(res.reclaimedBytes, 380, '回収 byte 数');
  assert.deepEqual([...res.removed].sort(), [
    '.cache/pokeapi/p1', '.cache/showdown/s1', 'calc/c1', 'pokeapi/p1', 'showdown/s1',
  ]);
  const text = lines.join('\n');
  assert.ok(text.includes('.cache/showdown/s1'));
  assert.ok(text.includes(String(res.reclaimedBytes)));
  assert.ok(text.includes('1500'), '残量');
  assert.ok(!text.includes(root), '絶対パスを出さない');
  assert.ok(!text.includes('snapshot.json'), 'ファイル名・中身を出さない');
  done(root);
});

test('削除の途中で失敗したら例外で止まり、保持対象は残る。再実行で残りを消せる(冪等)', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  let n = 0;
  const flaky = (path) => {
    if (++n === 2) throw new Error('EIO(架空)');
    rmSync(path, { recursive: true, force: true });
  };
  await assert.rejects(() => pruneGenerated({ generatedDir: root, config: config(), rm: flaky }), /EIO/);
  for (const rel of ['calc/c3', 'calc/c2', 'showdown/s2', 'showdown/s3', 'pokeapi/p2', '.cache/showdown/s2']) assert.ok(exists(root, rel), rel);
  const again = await pruneGenerated({ generatedDir: root, config: config() });
  assert.ok(!exists(root, 'showdown/s1') && !exists(root, 'pokeapi/p1') && !exists(root, 'calc/c1'));
  assert.ok(again.removed.length >= 1);
  const third = await pruneGenerated({ generatedDir: root, config: config() });
  assert.deepEqual(third.removed, [], '3 回目は何も消さない');
  assert.equal(third.reclaimedBytes, 0);
  done(root);
});

test('版が進むと台帳の直前版が入れ替わる(s2→s3→s4 で s2 が残り s1 が消える)', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  put(root, 'calc/c4/snapshot.json');
  put(root, 'showdown/s4/snapshot.json');
  put(root, 'pokeapi/p4/snapshot.json');
  success(root, ['4']);
  await pruneGenerated({ generatedDir: root, config: config({ calc: 'c4', showdown: 's4', pokeapi: 'p4' }) });
  assert.ok(exists(root, 'showdown/s4') && exists(root, 'showdown/s3'));
  assert.ok(!exists(root, 'showdown/s2') && !exists(root, 'showdown/s1'));
  done(root);
});

test('recordSuccess は同じ版の再記録で重複させない', () => {
  const root = seed();
  success(root, ['3', '3', '3']);
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.ok(plan.remove.includes('showdown/s2'), '3 世代目の枠に同じ版が入って 2 を押し出したりしない');
  done(root);
});

// --- 容量の事前確認 -------------------------------------------------------------------

test('空きが予約容量以上なら通り、総量・空き量を返す', () => {
  const root = seed();
  const r = checkReserve({ generatedDir: root, reserveBytes: 500, statfs: () => ({ totalBytes: 2000, freeBytes: 500 }) });
  assert.equal(r.freeBytes, 500);
  assert.equal(r.totalBytes, 2000);
  done(root);
});

test('空きが予約容量を下回ると InsufficientSpaceError(専用メッセージ)', () => {
  const root = seed();
  assert.throws(
    () => checkReserve({ generatedDir: root, reserveBytes: 501, statfs: () => ({ totalBytes: 2000, freeBytes: 500 }) }),
    (e) => e instanceof InsufficientSpaceError && /importer-capacity/.test(e.message) && !e.message.includes(root),
  );
  done(root);
});

// --- CLI(DB・ネットワークに触らない) -----------------------------------------------

const cli = (args, env) => spawnSync('node', [join(here, 'prune.mjs'), ...args], {
  encoding: 'utf8',
  env: { ...process.env, ...env },
});

test('CLI check: 予約容量を満たせないと専用メッセージで終了コード 3(download・DB より前に止める契約)', () => {
  const root = seed();
  const r = cli(['check'], { IMPORT_GENERATED_DIR: root, IMPORT_RESERVE_BYTES: String(Number.MAX_SAFE_INTEGER) });
  assert.equal(r.status, 3);
  assert.match(r.stderr, /importer-capacity/);
  assert.ok(!r.stderr.includes(root), '絶対パスを出さない');
  done(root);
});

test('CLI check: 十分な空きなら 0 で、総量・使用量・空き量を出す', () => {
  const root = seed();
  const r = cli(['check'], { IMPORT_GENERATED_DIR: root, IMPORT_RESERVE_BYTES: '1' });
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /importer-capacity/);
  done(root);
});

test('CLI prune: 成功を台帳へ記録してから prune し、旧版を消して 0 で終わる', () => {
  const root = seed();
  success(root, ['1', '2']);
  const cfg = join(root, '..', `cfg-${process.pid}.json`);
  writeFileSync(cfg, JSON.stringify({ schemaVersion: 1, sources: CURRENT }));
  const r = cli(['prune'], { IMPORT_GENERATED_DIR: root, IMPORT_CONFIG_FILE: cfg });
  assert.equal(r.status, 0, r.stderr);
  assert.ok(!exists(root, 'showdown/s1'));
  assert.ok(exists(root, 'showdown/s2') && exists(root, 'showdown/s3'));
  assert.ok(!r.stdout.includes(root));
  rmSync(cfg, { force: true });
  done(root);
});

test('CLI prune: 現在版の snapshot が無ければ非 0 で何も消さない', () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  rmSync(join(root, 'calc/c3'), { recursive: true });
  const cfg = join(root, '..', `cfg2-${process.pid}.json`);
  writeFileSync(cfg, JSON.stringify({ schemaVersion: 1, sources: CURRENT }));
  const r = cli(['prune'], { IMPORT_GENERATED_DIR: root, IMPORT_CONFIG_FILE: cfg });
  assert.notEqual(r.status, 0);
  assert.ok(exists(root, 'calc/c1/snapshot.json'));
  rmSync(cfg, { force: true });
  done(root);
});

test('CLI check: IMPORT_RESERVE_BYTES が整数でなければ終了コード 2', () => {
  const root = seed();
  for (const v of ['abc', '', '-1', '1.5']) {
    const r = cli(['check'], { IMPORT_GENERATED_DIR: root, IMPORT_RESERVE_BYTES: v });
    assert.equal(r.status, 2, `${JSON.stringify(v)}: ${r.stderr}`);
    assert.match(r.stderr, /IMPORT_RESERVE_BYTES/);
  }
  done(root);
});

test('CLI: fs エラー(存在しない IMPORT_GENERATED_DIR)でも stderr に絶対パスを出さない', () => {
  const missing = join(tmpdir(), `importer-prune-missing-${process.pid}`, 'nested');
  const r = cli(['check'], { IMPORT_GENERATED_DIR: missing, IMPORT_RESERVE_BYTES: '1' });
  assert.notEqual(r.status, 0);
  assert.ok(!r.stderr.includes(missing) && !r.stderr.includes(tmpdir()), r.stderr);
  assert.match(r.stderr, /ENOENT/);
});

test('壊れた JSON・配列の台帳は無いものとして扱い、現在版だけ残す', () => {
  for (const body of ['{not json', '{"sources":["s1","s2"]}', '[]']) {
    const root = seed();
    writeFileSync(join(root, '.import-success.json'), body);
    const plan = planPrune({ generatedDir: root, config: config() });
    assert.ok(!plan.remove.some((p) => p.endsWith('/c3') || p.endsWith('/s3') || p.endsWith('/p3')));
    assert.ok(plan.remove.includes('showdown/s2'), '台帳が読めなければ直前版の根拠が無い');
    done(root);
  }
});

test('symlink の版ディレクトリ・symlink の base は消さず、辿らない', async () => {
  const root = seed();
  success(root, ['1', '2', '3']);
  const outside = mkdtempSync(join(tmpdir(), 'importer-prune-outside-'));
  put(outside, 'keep.txt');
  symlinkSync(outside, join(root, 'showdown/s8'));
  rmSync(join(root, '.cache/pokeapi'), { recursive: true });
  symlinkSync(outside, join(root, '.cache/pokeapi'));
  const plan = planPrune({ generatedDir: root, config: config() });
  assert.ok(!plan.remove.includes('showdown/s8'));
  assert.ok(!plan.remove.some((p) => p.startsWith('.cache/pokeapi')));
  await pruneGenerated({ generatedDir: root, config: config() });
  assert.ok(exists(outside, 'keep.txt'));
  done(root);
  done(outside);
});

test('reports/ の import- で始まらないファイル・ディレクトリは古くても残す', async () => {
  const root = seed({ reports: 60 });
  success(root, ['1', '2', '3']);
  put(root, 'reports/notes.json');
  put(root, 'reports/import-old.d/x.json');
  put(root, 'reports/import-20200101T000000Z.json.bak');
  await pruneGenerated({ generatedDir: root, config: config() });
  for (const rel of ['reports/notes.json', 'reports/import-old.d/x.json', 'reports/import-20200101T000000Z.json.bak']) assert.ok(exists(root, rel), rel);
  done(root);
});
