// issue #288(ADR-0136): スナップショットの形が変わった(技に target が増えた)とき、古い取得物のまま止まらないこと。
// 実行: node --test tools/importer/fetch-snapshot-shape.test.mjs
// 取り込み(Go)は target が無い古いスナップショットを ErrInvalidInput で拒否する。CronJob は fetch → import なので、
// fetch が既存の snapshot.json を再利用せず、毎回取得元から作り直せば、古い形のまま止まることはない。
// ネットワーク・npm・実データは使わない(架空のキャッシュ済みソースと架空の Dex。IMPORTER_ROOT で差し替える)。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { calcMoveEntry } from './calc-move.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const sha = (s) => createHash('sha256').update(s).digest('hex');
const COMMIT = 'abad1deaabad1deaabad1deaabad1deaabad1dea';
const TREE_HASH = sha('架空のツリー');

// 架空の Dex(dist/sim/dex.js)。fetch-showdown.mjs が呼ぶ範囲だけを持つ。
const FAKE_DEX = `
const move = (id, target) => ({ id, name: id, type: 'Fire', category: 'Special', basePower: 90, accuracy: 100, pp: 15,
  priority: 0, isNonstandard: undefined, target });
const dex = {
  data: { Conditions: {} },
  species: { all: () => [], getLearnsetData: () => null },
  moves: { all: () => [move('testflame', 'allAdjacentFoes'), move('teststrike', 'normal')] },
  items: { all: () => [] },
  abilities: { all: () => [] },
  natures: { all: () => [] },
  conditions: { get: () => ({ exists: false }) },
};
module.exports = { Dex: { mod: () => dex } };
`;

// キャッシュ済み(検証済み)の Showdown ソースと、target を持たない古い snapshot.json を持つ架空のルート。
const setup = () => {
  const root = mkdtempSync(join(tmpdir(), 'snapshot-shape-'));
  mkdirSync(join(root, 'data', 'importer'), { recursive: true });
  writeFileSync(
    join(root, 'data', 'importer', 'config.json'),
    JSON.stringify({ schemaVersion: 1, sources: { showdown: COMMIT }, integrity: { showdown: { treeSha256: TREE_HASH } } }),
  );
  writeFileSync(
    join(root, 'data', 'importer', 'regulations.json'),
    JSON.stringify({ schemaVersion: 1, regulations: [{ id: 'fake', isDefault: true, showdownMod: 'fakemod' }] }),
  );
  const cache = join(root, 'data', 'generated', '.cache', 'showdown', COMMIT);
  mkdirSync(join(cache, 'src', 'dist', 'sim'), { recursive: true });
  const tarball = Buffer.from('架空のtarball');
  writeFileSync(join(cache, 'source.tar.gz'), tarball);
  writeFileSync(join(cache, 'meta.json'), JSON.stringify({ commit: COMMIT, sha256: sha(tarball) }));
  writeFileSync(join(cache, 'src', 'dist', 'sim', 'dex.js'), FAKE_DEX);
  writeFileSync(join(cache, 'src.tree-sha256'), TREE_HASH);

  const outDir = join(root, 'data', 'generated', 'showdown', COMMIT);
  mkdirSync(outDir, { recursive: true });
  const stale = { schemaVersion: 1, source: 'showdown', version: COMMIT, moves: [{ id: 'testflame' }] };
  writeFileSync(join(outDir, 'snapshot.json'), JSON.stringify(stale));
  return { root, snapshot: join(outDir, 'snapshot.json') };
};

test('showdown: キャッシュ済みのソースがあっても、古い形の snapshot.json を作り直し、全技に target を出す', () => {
  const { root, snapshot } = setup();
  const r = spawnSync('node', [join(here, 'fetch-showdown.mjs')], {
    env: { ...process.env, IMPORTER_ROOT: root },
    encoding: 'utf8',
    timeout: 30_000,
  });
  assert.equal(r.status, 0, r.stderr);
  const moves = JSON.parse(readFileSync(snapshot, 'utf8')).moves;
  assert.deepEqual(moves.map((m) => [m.id, m.target]), [['testflame', 'allAdjacentFoes'], ['teststrike', 'normal']]);
});

test('calc: 技の target は全体技にだけ持たせ、calc が省略した技は空文字で出す(キーを落とさない)', () => {
  assert.equal(calcMoveEntry({ name: 'A', type: 'Fire', category: 'Special', basePower: 90, target: 'allAdjacentFoes' }).target, 'allAdjacentFoes');
  const omitted = calcMoveEntry({ name: 'B', type: 'Normal', category: 'Physical', basePower: 40 });
  assert.equal(omitted.target, '');
  assert.ok('target' in omitted);
});

test('fetch-showdown・fetch-calc は既存の snapshot.json を読んで再利用しない(形が変わっても毎回作り直す)', () => {
  for (const f of ['fetch-showdown.mjs', 'fetch-calc.mjs']) {
    const src = readFileSync(join(here, f), 'utf8');
    assert.ok(!/readFileSync\([^)]*snapshot\.json|existsSync\([^)]*snapshot\.json/.test(src), `${f} が snapshot.json を再利用している`);
    assert.ok(/writeFileSync\([^)]*snapshot\.json/.test(src), `${f} が snapshot.json を書いていない`);
  }
});
