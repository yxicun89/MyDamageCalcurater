// issue #607(ADR-0140): Showdown の持ち物の megaStone(基本種名 → メガ種族名)をスナップショットに出す。
// 実行: node --test tools/importer/fetch-showdown-megastone.test.mjs
// 取り込み(Go)は megaStone のキーが無い古いスナップショットを拒否するので、メガストーンでない持ち物も
// 空のオブジェクト {} で必ずキーを持たせる。ネットワーク・npm・実データは使わない(架空の Dex。IMPORTER_ROOT で差し替える)。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

const here = dirname(fileURLToPath(import.meta.url));
const sha = (s) => createHash('sha256').update(s).digest('hex');
const COMMIT = 'abad1deaabad1deaabad1deaabad1deaabad1dea';
const TREE_HASH = sha('架空のツリー');

// 架空の Dex。Showdown の Item は megaStone を持たないとき undefined(sim/dex-items.ts)。
const FAKE_DEX = `
const item = (id, megaStone) => ({ id, name: id, isNonstandard: undefined, megaStone });
const dex = {
  data: { Conditions: {} },
  species: { all: () => [], getLearnsetData: () => null },
  moves: { all: () => [] },
  items: { all: () => [
    item('testmonite', { Testmon: 'Testmon-Mega' }),
    item('testpairite', { Testpair: 'Testpair-M-Mega', 'Testpair-F': 'Testpair-F-Mega' }),
    item('testorb', undefined),
  ] },
  abilities: { all: () => [] },
  natures: { all: () => [] },
  conditions: { get: () => ({ exists: false }) },
};
module.exports = { Dex: { mod: () => dex } };
`;

const setup = () => {
  const root = mkdtempSync(join(tmpdir(), 'snapshot-megastone-'));
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
  return { root, snapshot: join(root, 'data', 'generated', 'showdown', COMMIT, 'snapshot.json') };
};

test('showdown: 持ち物ごとに megaStone を出す(ストーンでなければ {}。キーを落とさない)', () => {
  const { root, snapshot } = setup();
  const r = spawnSync('node', [join(here, 'fetch-showdown.mjs')], {
    env: { ...process.env, IMPORTER_ROOT: root },
    encoding: 'utf8',
    timeout: 30_000,
  });
  assert.equal(r.status, 0, r.stderr);
  const items = JSON.parse(readFileSync(snapshot, 'utf8')).items;
  const byId = Object.fromEntries(items.map((i) => [i.id, i]));
  assert.deepEqual(byId.testmonite.megaStone, { Testmon: 'Testmon-Mega' });
  assert.deepEqual(byId.testpairite.megaStone, { Testpair: 'Testpair-M-Mega', 'Testpair-F': 'Testpair-F-Mega' });
  assert.ok('megaStone' in byId.testorb, 'ストーンでない持ち物にも megaStone のキーを出す');
  assert.deepEqual(byId.testorb.megaStone, {});
});
