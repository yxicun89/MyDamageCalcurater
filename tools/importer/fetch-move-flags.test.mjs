// ADR-0178: 技のフラグの判定材料を取得物に出す(取得元の表現のまま。語彙への変換・照合は Go 側)。
// 実行: node --test tools/importer/fetch-move-flags.test.mjs
//
// 受け入れ条件(取得):
//   - fetch-showdown は技ごとに flags(Showdown の flags のうち真のキーの昇順。語彙に無いキーも含む)・
//     recoil([分子, 分母] か null)・hasCrashDamage(真偽)を必ず出す(フラグの無い技も flags: [] でキーを落とさない)。
//   - fetch-calc(calcMoveEntry)は flags(真のキーの昇順)・recoil・hasCrashDamage・secondaries(真偽)を必ず出す。
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
const COMMIT = 'f1a95f1a95f1a95f1a95f1a95f1a95f1a95f1a95';
const TREE_HASH = sha('架空のツリー(フラグ)');

// 架空の Dex。flags は Showdown と同じく「フラグ名 → 1」のオブジェクト(偽のキーは持たない)。
const FAKE_DEX = `
const move = (id, extra) => ({ id, name: id, type: 'Normal', category: 'Physical', basePower: 80, accuracy: 100, pp: 15,
  priority: 0, isNonstandard: undefined, target: 'normal', flags: {}, ...extra });
const dex = {
  data: { Conditions: {} },
  species: { all: () => [], getLearnsetData: () => null },
  moves: { all: () => [
    move('testpunch', { flags: { punch: 1, protect: 1, contact: 1, mirror: 1 }, recoil: [33, 100] }),
    move('testcrash', { flags: { contact: 1 }, hasCrashDamage: true }),
    move('testplain', {}),
  ] },
  items: { all: () => [] },
  abilities: { all: () => [] },
  natures: { all: () => [] },
  conditions: { get: () => ({ exists: false }) },
};
module.exports = { Dex: { mod: () => dex } };
`;

const setup = () => {
  const root = mkdtempSync(join(tmpdir(), 'move-flags-'));
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
  const tarball = Buffer.from('架空のtarball(フラグ)');
  writeFileSync(join(cache, 'source.tar.gz'), tarball);
  writeFileSync(join(cache, 'meta.json'), JSON.stringify({ commit: COMMIT, sha256: sha(tarball) }));
  writeFileSync(join(cache, 'src', 'dist', 'sim', 'dex.js'), FAKE_DEX);
  writeFileSync(join(cache, 'src.tree-sha256'), TREE_HASH);
  return { root, snapshot: join(root, 'data', 'generated', 'showdown', COMMIT, 'snapshot.json') };
};

test('showdown: 技ごとに flags(真のキーの昇順)・recoil・hasCrashDamage を必ず出す', () => {
  const { root, snapshot } = setup();
  const r = spawnSync('node', [join(here, 'fetch-showdown.mjs')], {
    env: { ...process.env, IMPORTER_ROOT: root },
    encoding: 'utf8',
    timeout: 30_000,
  });
  assert.equal(r.status, 0, r.stderr);
  const moves = JSON.parse(readFileSync(snapshot, 'utf8')).moves;
  const pick = (m) => ({ id: m.id, flags: m.flags, recoil: m.recoil, hasCrashDamage: m.hasCrashDamage });
  assert.deepEqual(moves.map(pick), [
    { id: 'testpunch', flags: ['contact', 'mirror', 'protect', 'punch'], recoil: [33, 100], hasCrashDamage: false },
    { id: 'testcrash', flags: ['contact'], recoil: null, hasCrashDamage: true },
    { id: 'testplain', flags: [], recoil: null, hasCrashDamage: false },
  ]);
});

test('calc: 技ごとに flags(真のキーの昇順)・recoil・hasCrashDamage・secondaries を必ず出す', () => {
  const full = calcMoveEntry({
    name: 'A', type: 'Normal', category: 'Physical', basePower: 120,
    flags: { punch: 1, contact: 1 }, recoil: [33, 100], hasCrashDamage: false, secondaries: true,
  });
  assert.deepEqual(
    { flags: full.flags, recoil: full.recoil, hasCrashDamage: full.hasCrashDamage, secondaries: full.secondaries },
    { flags: ['contact', 'punch'], recoil: [33, 100], hasCrashDamage: false, secondaries: true },
  );
  const crash = calcMoveEntry({ name: 'B', type: 'Fighting', category: 'Physical', basePower: 130, flags: { contact: 1 }, hasCrashDamage: true });
  assert.equal(crash.hasCrashDamage, true);
  // calc は未設定の項目を省略する。キーを落とさず空値で埋める(Go 側が「無い」と「省略」を区別する)。
  const omitted = calcMoveEntry({ name: 'C', type: 'Normal', category: 'Status', basePower: 0 });
  assert.deepEqual(
    { flags: omitted.flags, recoil: omitted.recoil, hasCrashDamage: omitted.hasCrashDamage, secondaries: omitted.secondaries },
    { flags: [], recoil: null, hasCrashDamage: false, secondaries: false },
  );
  for (const k of ['flags', 'recoil', 'hasCrashDamage', 'secondaries']) assert.ok(k in omitted, `${k} が無い`);
});
