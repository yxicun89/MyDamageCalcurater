// @smogon/calc(0.12.0 固定・Champions 世代 = Generations.get(0))からの抽出(ADR-0101 §3)。
// 実行: このディレクトリで `npm ci` の後 `node fetch-calc.mjs`。
//
// data/generated/calc/<version>/snapshot.json に、取得元の表現をなるべくそのまま書く
// (ID化・小文字化は Go 側で行う)。version は package.json の固定版と一致する。
import calc from '@smogon/calc';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';

const EXPECTED_VERSION = '0.12.0';
const { Generations } = calc;

const pkgVersion = JSON.parse(
  readFileSync(new URL('node_modules/@smogon/calc/package.json', import.meta.url)),
).version;
assert.equal(pkgVersion, EXPECTED_VERSION, `@smogon/calc の版が ${EXPECTED_VERSION} でない(package.json と揃える)`);

const gen = Generations.get(0); // Champions

const types = [...gen.types].map((t) => t.name);

const typeChart = {};
for (const t of gen.types) {
  const row = {};
  for (const [defName, mult] of Object.entries(t.effectiveness ?? {})) {
    if (mult === 1) continue; // 等倍の組は省略(無い組は等倍)
    row[defName] = Math.round(mult * 2);
  }
  typeChart[t.name] = row;
}

const species = [...gen.species].map((s) => ({
  name: s.name,
  types: s.types,
  baseStats: {
    hp: s.baseStats.hp, atk: s.baseStats.atk, def: s.baseStats.def,
    spa: s.baseStats.spa, spd: s.baseStats.spd, spe: s.baseStats.spe,
  },
}));

const moves = [...gen.moves].map((m) => ({
  name: m.name,
  type: m.type ?? '',
  category: m.category ?? '',
  basePower: m.basePower ?? 0,
  priority: m.priority ?? 0,
}));

const items = [...gen.items].map((i) => i.name);
const abilities = [...gen.abilities].map((a) => a.name);

const snapshot = {
  schemaVersion: 1,
  source: 'calc',
  version: pkgVersion,
  generation: 0,
  types,
  typeChart,
  species,
  moves,
  items,
  abilities,
};

const outDir = fileURLToPath(new URL(`../../data/generated/calc/${pkgVersion}/`, import.meta.url));
mkdirSync(outDir, { recursive: true });
writeFileSync(new URL('snapshot.json', `file://${outDir}`), JSON.stringify(snapshot));
writeFileSync(
  new URL('meta.json', `file://${outDir}`),
  JSON.stringify({ fetchedAt: new Date().toISOString(), source: 'npm package @smogon/calc' }, null, 2),
);
console.log(`fetch-calc: ${outDir}snapshot.json を書いた(種族 ${species.length} 件・技 ${moves.length} 件)`);
