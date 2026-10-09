// ADR-0143 §4: 種族の重さ(weightkg)を取得物に出す(取得元の数値のまま。hg への変換・照合は Go 側)。
// 実行: node --test tools/importer/fetch-species-weight.test.mjs
//
// 受け入れ条件(取得):
//   - fetch-showdown は種族ごとに weightkg(Showdown の数値のまま)を必ず出す。
//   - fetch-calc の種族1件の形(calc-species.mjs の calcSpeciesEntry。fetch-calc.mjs から分けた純粋な関数)は
//     name・types・baseStats・weightkg(calc の数値のまま)を出す。weightkg が無い・正でない種族は例外で止める(黙って 0 にしない)。
// ネットワーク・npm・実データは使わない(架空のキャッシュ済みソースと架空の Dex。IMPORTER_ROOT で差し替える)。
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { calcSpeciesEntry } from './calc-species.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const sha = (s) => createHash('sha256').update(s).digest('hex');
const COMMIT = 'a8e1a8e1a8e1a8e1a8e1a8e1a8e1a8e1a8e1a8e1';
const TREE_HASH = sha('架空のツリー(重さ)');

// 架空の Dex。種族は Showdown の Species と同じ項目を持つ(weightkg は kg の数値)。
const FAKE_DEX = `
const species = (id, weightkg) => ({ id, name: id, num: 9001, baseSpecies: id, forme: '', baseForme: '', types: ['Normal'],
  baseStats: { hp: 80, atk: 80, def: 80, spa: 80, spd: 80, spe: 80 }, abilities: { 0: 'Test' }, requiredItem: '',
  formeOrder: [], isNonstandard: undefined, prevo: '', weightkg });
const dex = {
  data: { Conditions: {} },
  species: { all: () => [species('testlight', 0.1), species('testheavy', 999.9), species('testmid', 6.9)], getLearnsetData: () => null },
  moves: { all: () => [] },
  items: { all: () => [] },
  abilities: { all: () => [] },
  natures: { all: () => [] },
  conditions: { get: () => ({ exists: false }) },
};
module.exports = { Dex: { mod: () => dex } };
`;

const setup = () => {
  const root = mkdtempSync(join(tmpdir(), 'species-weight-'));
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
  const tarball = Buffer.from('架空のtarball(重さ)');
  writeFileSync(join(cache, 'source.tar.gz'), tarball);
  writeFileSync(join(cache, 'meta.json'), JSON.stringify({ commit: COMMIT, sha256: sha(tarball) }));
  writeFileSync(join(cache, 'src', 'dist', 'sim', 'dex.js'), FAKE_DEX);
  writeFileSync(join(cache, 'src.tree-sha256'), TREE_HASH);
  return { root, snapshot: join(root, 'data', 'generated', 'showdown', COMMIT, 'snapshot.json') };
};

test('showdown: 種族ごとに weightkg を取得元の数値のまま出す', () => {
  const { root, snapshot } = setup();
  const r = spawnSync('node', [join(here, 'fetch-showdown.mjs')], {
    env: { ...process.env, IMPORTER_ROOT: root },
    encoding: 'utf8',
    timeout: 30_000,
  });
  assert.equal(r.status, 0, r.stderr);
  const species = JSON.parse(readFileSync(snapshot, 'utf8')).species;
  assert.deepEqual(
    species.map((s) => ({ id: s.id, weightkg: s.weightkg })),
    [
      { id: 'testlight', weightkg: 0.1 },
      { id: 'testheavy', weightkg: 999.9 },
      { id: 'testmid', weightkg: 6.9 },
    ],
  );
});

test('calc: 種族1件の形に weightkg を出す・無い/正でない値は止める', () => {
  const entry = calcSpeciesEntry({
    name: 'Testmon', types: ['Fire'], weightkg: 90.5,
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 90 }, nfe: true,
  });
  assert.deepEqual(entry, {
    name: 'Testmon', types: ['Fire'],
    baseStats: { hp: 80, atk: 100, def: 70, spa: 60, spd: 70, spe: 90 },
    weightkg: 90.5,
  });
  for (const bad of [undefined, 0, -1, Number.NaN]) {
    assert.throws(() => calcSpeciesEntry({ name: 'Bad', types: ['Normal'], weightkg: bad,
      baseStats: { hp: 1, atk: 1, def: 1, spa: 1, spd: 1, spe: 1 } }), /weightkg/);
  }
});
