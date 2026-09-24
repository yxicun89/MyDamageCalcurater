// issue #102: キャッシュの中断回復。実行: node --test tools/importer/showdown-cache.test.mjs
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { ensureShowdownSource } from './showdown-cache.mjs';

const payload = Buffer.from('架空のtarball');
const sha = createHash('sha256').update(payload).digest('hex');

const setup = () => {
  const cacheDir = mkdtempSync(join(tmpdir(), 'showdown-cache-'));
  const calls = { download: 0, extract: 0, install: 0, build: 0 };
  const deps = {
    download: async () => { calls.download++; return payload; },
    extract: () => { calls.extract++; },
    install: () => { calls.install++; },
    build: (dir) => {
      calls.build++;
      mkdirSync(join(dir, 'dist', 'sim'), { recursive: true });
      writeFileSync(join(dir, 'dist', 'sim', 'dex.js'), '');
    },
  };
  const run = () => ensureShowdownSource({ cacheDir, commit: 'c', url: 'u', deps });
  return { cacheDir, calls, deps, run, done: () => rmSync(cacheDir, { recursive: true, force: true }) };
};

const seedTarball = (cacheDir, body = payload, metaSha = sha) => {
  writeFileSync(join(cacheDir, 'source.tar.gz'), body);
  writeFileSync(join(cacheDir, 'meta.json'), JSON.stringify({ sha256: metaSha }));
};

test('初回は取得・展開・build を行い src/ を公開する', async () => {
  const t = setup();
  await t.run();
  assert.deepEqual(t.calls, { download: 1, extract: 1, install: 1, build: 1 });
  assert.ok(existsSync(join(t.cacheDir, 'src', 'dist', 'sim', 'dex.js')));
  assert.deepEqual(readdirSync(t.cacheDir).filter((n) => n.includes('.partial-')), []);
  t.done();
});

test('正常な完成キャッシュは何もしない(再取得しない)', async () => {
  const t = setup();
  await t.run();
  t.calls.download = t.calls.extract = t.calls.install = t.calls.build = 0;
  await t.run();
  assert.deepEqual(t.calls, { download: 0, extract: 0, install: 0, build: 0 });
  t.done();
});

test('空の展開ディレクトリから自己回復する(tarball は再取得しない)', async () => {
  const t = setup();
  seedTarball(t.cacheDir);
  mkdirSync(join(t.cacheDir, 'src'));
  await t.run();
  assert.equal(t.calls.download, 0);
  assert.equal(t.calls.build, 1);
  assert.ok(existsSync(join(t.cacheDir, 'src', 'dist', 'sim', 'dex.js')));
  t.done();
});

test('build 途中の残骸(*.partial-*)は捨てて作り直す', async () => {
  const t = setup();
  seedTarball(t.cacheDir);
  const leftover = join(t.cacheDir, 'src.partial-dead');
  mkdirSync(join(leftover, 'node_modules'), { recursive: true });
  await t.run();
  assert.ok(!existsSync(leftover));
  assert.equal(t.calls.build, 1);
  t.done();
});

test('壊れた tarball(ハッシュ不一致)は再取得して展開先も作り直す', async () => {
  const t = setup();
  seedTarball(t.cacheDir, Buffer.from('壊れた'));
  mkdirSync(join(t.cacheDir, 'src', 'dist', 'sim'), { recursive: true });
  writeFileSync(join(t.cacheDir, 'src', 'dist', 'sim', 'dex.js'), 'old');
  await t.run();
  assert.equal(t.calls.download, 1);
  assert.equal(t.calls.build, 1);
  t.done();
});

test('meta.json 欠損は再取得する', async () => {
  const t = setup();
  writeFileSync(join(t.cacheDir, 'source.tar.gz'), payload);
  await t.run();
  assert.equal(t.calls.download, 1);
  t.done();
});

test('build 失敗時は不完全な src/ を残さない(次回再試行できる)', async () => {
  const t = setup();
  t.deps.build = () => { throw new Error('boom'); };
  await assert.rejects(t.run(), /boom/);
  assert.ok(!existsSync(join(t.cacheDir, 'src')));
  assert.deepEqual(readdirSync(t.cacheDir).filter((n) => n.includes('.partial-') && n.startsWith('src')), []);
  t.done();
});
