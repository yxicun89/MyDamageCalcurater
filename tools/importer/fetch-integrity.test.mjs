// issue #222・#301(D19): Showdown の展開後の検証(ensureShowdownSource)と PokeAPI の CSV の照合(fetch-pokeapi.mjs)。
// 実行: node --test tools/importer/fetch-integrity.test.mjs
// ネットワーク・npm・実データは使わない(架空のツリー・架空の CSV。pokeapi は IMPORTER_ROOT でキャッシュ済みを読ませる)。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { IntegrityError } from './integrity.mjs';
import { ensureShowdownSource } from './showdown-cache.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const sha = (s) => createHash('sha256').update(s).digest('hex');
const COMMIT = '0123456789abcdef0123456789abcdef01234567';

// --- Showdown: 展開直後・install より前に検証する -----------------------------------------

const payload = Buffer.from('架空のtarball');
// 展開される架空のツリー(Showdown のソース相当)。ハッシュは integrity.test.mjs と同じ式で別に計算する。
const TREE = { 'package.json': '{}', 'sim/dex.ts': 'export {}' };
const TREE_HASH = sha(
  Object.entries(TREE)
    .sort(([a], [b]) => (a < b ? -1 : 1))
    .map(([p, body]) => `${sha(body)}  ${p}\n`)
    .join(''),
);

const setup = (tree = TREE) => {
  const cacheDir = mkdtempSync(join(tmpdir(), 'showdown-integrity-'));
  const log = [];
  const deps = {
    download: async () => { log.push('download'); return payload; },
    extract: (_tarball, dir) => {
      log.push('extract');
      for (const [rel, body] of Object.entries(tree)) {
        mkdirSync(dirname(join(dir, rel)), { recursive: true });
        writeFileSync(join(dir, rel), body);
      }
    },
    install: () => { log.push('install'); },
    build: (dir) => {
      log.push('build');
      mkdirSync(join(dir, 'dist', 'sim'), { recursive: true });
      writeFileSync(join(dir, 'dist', 'sim', 'dex.js'), '');
    },
  };
  const run = (expectedTreeSha256) =>
    ensureShowdownSource({ cacheDir, commit: COMMIT, url: 'u', expectedTreeSha256, deps });
  return { cacheDir, log, run, done: () => rmSync(cacheDir, { recursive: true, force: true }) };
};

test('一致: 展開 → 検証 → install → build の順に進み src/ を公開する', async () => {
  const t = setup();
  await t.run(TREE_HASH);
  assert.deepEqual(t.log, ['download', 'extract', 'install', 'build']);
  assert.ok(existsSync(join(t.cacheDir, 'src', 'dist', 'sim', 'dex.js')));
  t.done();
});

test('不一致: install・build(第三者のコードの実行)の前に IntegrityError(終了コード3)で止まり、src/ を残さない', async () => {
  const t = setup();
  await assert.rejects(t.run(sha('別の内容')), (err) => {
    assert.ok(err instanceof IntegrityError);
    assert.equal(err.exitCode, 3);
    assert.ok(err.message.includes(TREE_HASH), '実際のハッシュをメッセージに含める(人が config を更新できるように)');
    return true;
  });
  assert.deepEqual(t.log, ['download', 'extract']);
  assert.ok(!existsSync(join(t.cacheDir, 'src')));
  assert.deepEqual(readdirSync(t.cacheDir).filter((n) => n.includes('.partial-')), []);
  t.done();
});

test('期待値が形式不正でも install の前に止まる(fail closed)', async () => {
  const t = setup();
  await assert.rejects(t.run('not-a-hash'), IntegrityError);
  assert.ok(!t.log.includes('install'));
  t.done();
});

test('検証済みのキャッシュは再取得・再 build しない', async () => {
  const t = setup();
  await t.run(TREE_HASH);
  t.log.length = 0;
  await t.run(TREE_HASH);
  assert.deepEqual(t.log, []);
  t.done();
});

test('config の期待値が変わったら、キャッシュ済みでも再検証し、不一致なら src/ を使わせない', async () => {
  const t = setup();
  await t.run(TREE_HASH);
  t.log.length = 0;
  await assert.rejects(t.run(sha('改ざんを疑う別の期待値')), IntegrityError);
  assert.ok(!t.log.includes('install') && !t.log.includes('build'));
  assert.ok(!existsSync(join(t.cacheDir, 'src')), '期待値に合わない展開済みの src/ を残さない(import されない)');
  assert.ok(!t.log.includes('download'), 'tarball は有効なので再取得しない');
  // 期待値を元に戻せば、tarball から作り直して成功する。
  t.log.length = 0;
  await t.run(TREE_HASH);
  assert.deepEqual(t.log, ['extract', 'install', 'build']);
  t.done();
});

// --- Showdown: スクリプトが検証・--ignore-scripts を使うこと(静的) ---------------------------

test('fetch-showdown.mjs: npm ci は --ignore-scripts、期待ハッシュを config から読んで渡し、ログの組み立てに終了コードを使う', () => {
  const src = readFileSync(join(here, 'fetch-showdown.mjs'), 'utf8');
  const npmCi = src.split('\n').filter((l) => /'npm'/.test(l) && /'ci'/.test(l));
  assert.ok(npmCi.length >= 1, 'npm ci の呼び出しが無い');
  for (const l of npmCi) assert.match(l, /--ignore-scripts/, l);
  assert.match(src, /expectedShowdownTreeSha256\s*\(\s*config\s*\)/);
  assert.match(src, /expectedTreeSha256/);
  assert.match(src, /integrity\.mjs/);
});

test('fetch.mjs・fetch-showdown.mjs・fetch-pokeapi.mjs は IntegrityError を終了コード3に写す', () => {
  for (const f of ['fetch.mjs', 'fetch-showdown.mjs', 'fetch-pokeapi.mjs']) {
    const src = readFileSync(join(here, f), 'utf8');
    assert.match(src, /exitCodeFor|IntegrityError/, `${f} が integrity.mjs の終了コード変換を使っていない`);
  }
});

// --- PokeAPI: CSV を取得(キャッシュ)直後・解析の前にバイト列を照合する --------------------------

// fetch-pokeapi.mjs が読む CSV。ヘッダだけ・最小の行の架空の内容。
const CSV = {
  'languages.csv': 'id,identifier\n1,ja\n',
  'pokemon_species.csv': 'id,identifier\n1,fakemon\n',
  'pokemon_species_names.csv': 'pokemon_species_id,local_language_id,name\n1,1,ふぇいくもん\n',
  'pokemon_forms.csv': 'id,identifier,form_identifier\n1,fakemon,\n',
  'pokemon_form_names.csv': 'pokemon_form_id,local_language_id,pokemon_name\n',
  'moves.csv': 'id,identifier\n1,fakestrike\n',
  'move_names.csv': 'move_id,local_language_id,name\n1,1,ふぇいくあたっく\n',
  'items.csv': 'id,identifier\n1,fakeitem\n',
  'item_names.csv': 'item_id,local_language_id,name\n1,1,ふぇいくどうぐ\n',
  'abilities.csv': 'id,identifier\n1,fakeability\n',
  'ability_names.csv': 'ability_id,local_language_id,name\n1,1,ふぇいくとくせい\n',
  'types.csv': 'id,identifier\n1,fake\n',
  'type_names.csv': 'type_id,local_language_id,name\n1,1,ふぇいく\n',
  'natures.csv': 'id,identifier\n1,fakenature\n',
  'nature_names.csv': 'nature_id,local_language_id,name\n1,1,ふぇいくせいかく\n',
};

const pokeapiRoot = (integrity, files = CSV) => {
  const root = mkdtempSync(join(tmpdir(), 'pokeapi-integrity-'));
  mkdirSync(join(root, 'data', 'importer'), { recursive: true });
  writeFileSync(
    join(root, 'data', 'importer', 'config.json'),
    JSON.stringify({ schemaVersion: 1, sources: { pokeapi: COMMIT }, ...(integrity ? { integrity } : {}) }),
  );
  const cache = join(root, 'data', 'generated', '.cache', 'pokeapi', COMMIT);
  mkdirSync(cache, { recursive: true });
  for (const [name, body] of Object.entries(files)) writeFileSync(join(cache, name), body);
  return root;
};

const csvHashes = (files = CSV) => Object.fromEntries(Object.entries(files).map(([n, b]) => [n, sha(b)]));

const runPokeapi = (root) =>
  spawnSync('node', [join(here, 'fetch-pokeapi.mjs')], {
    env: { ...process.env, IMPORTER_ROOT: root },
    encoding: 'utf8',
    timeout: 30_000,
  });

const snapshotPath = (root) => join(root, 'data', 'generated', 'pokeapi', COMMIT, 'snapshot.json');

test('pokeapi: 期待ハッシュと全 CSV が一致すれば従来どおり snapshot を書く', () => {
  const root = pokeapiRoot({ pokeapi: { csvSha256: csvHashes() } });
  const r = runPokeapi(root);
  assert.equal(r.status, 0, r.stderr);
  assert.ok(existsSync(snapshotPath(root)));
  rmSync(root, { recursive: true, force: true });
});

test('pokeapi: CSV が1つでも期待と違えば終了コード3で、snapshot を書かず、ファイル名を stderr に出す', () => {
  const hashes = csvHashes();
  hashes['moves.csv'] = sha('別の内容');
  const root = pokeapiRoot({ pokeapi: { csvSha256: hashes } });
  const r = runPokeapi(root);
  assert.equal(r.status, 3, r.stderr);
  assert.match(r.stderr, /moves\.csv/);
  assert.ok(!existsSync(snapshotPath(root)));
  rmSync(root, { recursive: true, force: true });
});

test('pokeapi: キャッシュ済みの CSV が書き換わっていても(PVC 上の改変)取得のたびに照合して止まる', () => {
  const root = pokeapiRoot({ pokeapi: { csvSha256: csvHashes() } });
  writeFileSync(join(root, 'data', 'generated', '.cache', 'pokeapi', COMMIT, 'items.csv'), 'id,identifier\n1,tampered\n');
  const r = runPokeapi(root);
  assert.equal(r.status, 3, r.stderr);
  assert.ok(!existsSync(snapshotPath(root)));
  rmSync(root, { recursive: true, force: true });
});

test('pokeapi: integrity が無い・表に無い CSV があるときも終了コード3(fail closed)', () => {
  for (const integrity of [undefined, { pokeapi: { csvSha256: { 'languages.csv': sha(CSV['languages.csv']) } } }]) {
    const root = pokeapiRoot(integrity);
    const r = runPokeapi(root);
    assert.equal(r.status, 3, r.stderr);
    assert.ok(!existsSync(snapshotPath(root)));
    rmSync(root, { recursive: true, force: true });
  }
});
