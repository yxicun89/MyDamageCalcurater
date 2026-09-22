// Pokemon Showdown(champions mod。commit を config.json で固定)からの抽出(ADR-0101 §3)。
// 実行: node fetch-showdown.mjs
//
// 版の正は data/importer/config.json の sources.showdown(40桁の commit)だけ。
// Showdown のコードを実行して継承(inherit)を解決した実効値を取り出す(読むだけでは誤る)。
//
// 手順: codeload の tarball を data/generated/.cache/showdown/<commit>/ に取得しキャッシュする
// (同じ commit は再取得しない)→ 展開 → `npm ci && node build` → 生成された `Dex` を
// `Dex.mod(<mod>)` で使う。tar 展開は依存を増やさないため system の `tar` コマンドを使う。
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const config = JSON.parse(readFileSync(`${root}data/importer/config.json`, 'utf8'));
const commit = config.sources?.showdown;
if (!commit || !/^[0-9a-f]{40}$/.test(commit)) {
  throw new Error(`config.json の sources.showdown が40桁の commit でない: ${commit}`);
}

// mod は data/importer/regulations.json の既定レギュレーションの showdownMod だけを正とする
// (config.json 側に既定値を持たない。ADR-0101 §7)。既定が無ければ最初の定義を使う。
const regulations = JSON.parse(readFileSync(`${root}data/importer/regulations.json`, 'utf8')).regulations ?? [];
const regulation = regulations.find((r) => r.isDefault) ?? regulations[0];
const mod = regulation?.showdownMod;
if (!mod || /^PENDING/i.test(mod)) {
  throw new Error(`regulations.json の showdownMod が未固定(実際の mod 名を pin すること): ${mod}`);
}

const cacheDir = `${root}data/generated/.cache/showdown/${commit}/`;
const tarballPath = `${cacheDir}source.tar.gz`;
const extractDir = `${cacheDir}src/`;
mkdirSync(cacheDir, { recursive: true });

if (!existsSync(tarballPath)) {
  const url = `https://codeload.github.com/smogon/pokemon-showdown/tar.gz/${commit}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Showdown tarball を取得できない: ${res.status} ${url}`);
  const buf = Buffer.from(await res.arrayBuffer());
  writeFileSync(tarballPath, buf);
  const sha256 = createHash('sha256').update(buf).digest('hex');
  writeFileSync(`${cacheDir}meta.json`, JSON.stringify({ commit, fetchedAt: new Date().toISOString(), sha256, url }, null, 2));
  console.log(`fetch-showdown: tarball を取得してキャッシュした(sha256=${sha256})`);
} else {
  console.log('fetch-showdown: キャッシュ済みの tarball を使う');
}

if (!existsSync(extractDir)) {
  mkdirSync(extractDir, { recursive: true });
  execFileSync('tar', ['-xzf', tarballPath, '--strip-components=1', '-C', extractDir]);
  execFileSync('npm', ['ci', '--omit=dev'], { cwd: extractDir, stdio: 'inherit' });
  execFileSync('node', ['build'], { cwd: extractDir, stdio: 'inherit' });
}

// 固定した Showdown は CommonJS として build される。ESM からの dynamic import では
// named export が直接見える版と default に包まれる版があるため、両方を受ける。
const dexModule = await import(`${extractDir}dist/sim/dex.js`);
const dexExports = dexModule.default ?? dexModule['module.exports'] ?? dexModule;
const Dex = dexExports.Dex ?? dexExports.default;
if (!Dex?.mod) {
  throw new Error(`Showdown の Dex export を解決できない(exports=${Object.keys(dexModule).sort().join(',')})`);
}
const dex = Dex.mod(mod);

const toNonstandard = (thing) => (thing.isNonstandard ? thing.isNonstandard : null);
const hooks = (thing) => Object.keys(thing)
  .filter((key) => /^on[A-Z]/.test(key) && typeof thing[key] === 'function')
  .sort();

const species = [...dex.species.all()].map((s) => ({
  id: s.id,
  name: s.name,
  num: s.num,
  baseSpecies: s.baseSpecies,
  forme: s.forme ?? '',
  baseForme: s.baseForme ?? '',
  types: s.types,
  baseStats: s.baseStats,
  abilities: s.abilities,
  requiredItem: s.requiredItem ?? '',
  formeOrder: s.formeOrder ?? [],
  isNonstandard: toNonstandard(s),
  prevo: s.prevo ?? '',
}));

const moves = [...dex.moves.all()].map((m) => ({
  id: m.id,
  name: m.name,
  type: m.type,
  category: m.category,
  basePower: m.basePower,
  accuracy: m.accuracy === true ? 0 : m.accuracy,
  pp: m.pp,
  priority: m.priority,
  isNonstandard: toNonstandard(m),
}));

const items = [...dex.items.all()].map((i) => ({
  id: i.id,
  name: i.name,
  isNonstandard: toNonstandard(i),
  hooks: hooks(i),
}));
const abilities = [...dex.abilities.all()].map((a) => ({
  id: a.id,
  name: a.name,
  isNonstandard: toNonstandard(a),
  hooks: hooks(a),
}));

// 学習元の符号(例 "9M" 第9世代マシン・"7L12" 第7世代レベル12・"8E" 第8世代タマゴ技)の先頭の数字が
// 学習した世代。ADR-0103 §7: フォーマットの minSourceGen 以上の学習元が1つでもあれば学習可能なので、
// 技ごとに学習元の最大世代だけを残す(個々の学習元の一覧までは持たない)。
const maxSourceGen = (sources) => {
  const gens = sources.map((src) => Number(src.charAt(0))).filter((g) => Number.isInteger(g) && g > 0);
  return gens.length > 0 ? Math.max(...gens) : 0;
};

const learnsets = {};
// 進化前はレギュレーション外でも習得元になるため、mod の標準集合に絞らず全種族を走査する。
for (const s of dex.species.all()) {
  const entry = dex.species.getLearnsetData(s.id);
  if (entry?.learnset) {
    const byMove = {};
    for (const [moveId, sources] of Object.entries(entry.learnset)) {
      const gen = maxSourceGen(sources);
      if (gen > 0) byMove[moveId] = gen;
    }
    learnsets[s.id] = byMove;
  }
}

const snapshot = {
  schemaVersion: 1,
  source: 'showdown',
  version: commit,
  mod,
  species,
  moves,
  items,
  abilities,
  learnsets,
};

const outDir = `${root}data/generated/showdown/${commit}/`;
mkdirSync(outDir, { recursive: true });
writeFileSync(`${outDir}snapshot.json`, JSON.stringify(snapshot));
writeFileSync(`${outDir}meta.json`, JSON.stringify({ fetchedAt: new Date().toISOString(), commit, mod }, null, 2));
console.log(`fetch-showdown: ${outDir}snapshot.json を書いた(種族 ${species.length} 件・技 ${moves.length} 件)`);
