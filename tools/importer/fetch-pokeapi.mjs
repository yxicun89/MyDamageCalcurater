// PokeAPI(PokeAPI/pokeapi リポジトリの commit を固定)の CSV から日本語名だけを抽出する
// (ADR-0101 §3)。REST を種族・技・持ち物・特性ごとに約1,200回叩かず、
// commit 固定の CSV(data/v2/csv/*.csv)を raw で取得する(公平利用・版の固定)。
//
// 実行: node fetch-pokeapi.mjs
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { exitCodeFor, expectedPokeapiCsvSha256, verifyFileSha256 } from './integrity.mjs';
import { parseCSV } from './pokeapi-csv.mjs';

// IMPORTER_ROOT: テスト用にリポジトリのルートを差し替える。
const root = process.env.IMPORTER_ROOT ? `${process.env.IMPORTER_ROOT.replace(/\/$/, '')}/` : fileURLToPath(new URL('../../', import.meta.url));
const config = JSON.parse(readFileSync(`${root}data/importer/config.json`, 'utf8'));
const commit = config.sources?.pokeapi;
if (!commit || !/^[0-9a-f]{40}$/.test(commit)) {
  throw new Error(`config.json の sources.pokeapi が40桁の commit でない: ${commit}`);
}

// 各 CSV は取得(またはキャッシュ読み込み)の直後・解析の前に、期待ハッシュと照合する(D19)。
// 不一致・期待値なしは終了コード 3(fail closed)。
let expectedCsv;
try {
  expectedCsv = expectedPokeapiCsvSha256(config);
} catch (err) {
  console.error(`fetch-pokeapi: ${err.message}`);
  process.exit(exitCodeFor(err));
}
const verifyCSV = (name, content) => {
  try {
    verifyFileSha256({ name, content, expected: expectedCsv });
  } catch (err) {
    console.error(`fetch-pokeapi: ${err.message}`);
    process.exit(exitCodeFor(err));
  }
};

const LANGUAGES = ['ja-Hrkt', 'ja']; // ADR-0101 §3: この2言語だけ出す(取り込む言語の絞り込みは config 側)
const cacheDir = `${root}data/generated/.cache/pokeapi/${commit}/`;
mkdirSync(cacheDir, { recursive: true });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ネットワークの礼儀(ADR-0101 §3): 逐次取得し、実際にネットワークへ出たときだけ
// リクエスト間に待ち時間を入れる(キャッシュ済みは待たない)。
async function fetchCSV(name) {
  const cachePath = `${cacheDir}${name}`;
  if (existsSync(cachePath)) {
    const cached = readFileSync(cachePath);
    verifyCSV(name, cached);
    return parseCSV(cached.toString('utf8'), cachePath);
  }
  const url = `https://raw.githubusercontent.com/PokeAPI/pokeapi/${commit}/data/v2/csv/${name}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`PokeAPI CSV を取得できない: ${res.status} ${url}`);
  const body = Buffer.from(await res.arrayBuffer());
  verifyCSV(name, body); // 検証に通ったものだけキャッシュに書く
  const text = body.toString('utf8');
  writeFileSync(cachePath, body);
  await sleep(300);
  return parseCSV(text, url);
}

const languages = await fetchCSV('languages.csv');
const langIDToIdentifier = new Map(languages.map((l) => [l.id, l.identifier ?? l.iso639]));

// namesByOwner: ownerIdColumn の値ごとに、対象言語だけの {identifier: name} を集める。
function collectNames(rows, ownerCol, nameCol = 'name') {
  const byOwner = new Map();
  for (const r of rows) {
    const lang = langIDToIdentifier.get(r.local_language_id);
    if (!LANGUAGES.includes(lang)) continue;
    const owner = r[ownerCol];
    if (!byOwner.has(owner)) byOwner.set(owner, {});
    byOwner.get(owner)[lang] = r[nameCol];
  }
  return byOwner;
}

async function namedEntries(slugRows, nameRows, idCol, slugCol, ownerCol, nameCol = 'name') {
  const names = collectNames(nameRows, ownerCol, nameCol);
  return slugRows
    .map((r) => ({ slug: r[slugCol], names: names.get(r[idCol]) ?? {} }))
    .filter((e) => Object.keys(e.names).length > 0);
}

const pokemonSpecies = await fetchCSV('pokemon_species.csv');
const pokemonSpeciesNames = await fetchCSV('pokemon_species_names.csv');
const speciesEntries = await namedEntries(pokemonSpecies, pokemonSpeciesNames, 'id', 'identifier', 'pokemon_species_id');

const pokemonForms = await fetchCSV('pokemon_forms.csv');
const pokemonFormNames = await fetchCSV('pokemon_form_names.csv');
// pokemon_forms.csv の form_identifier は既定フォームだと空になる(identifier は既定でも
// 埋まっていることがあるので、既定フォームの判定には使えない)。既定でないフォームだけ出す。
// pokemon_form_names.csv には name 列が無く、フォーム名は pokemon_name 列に入っている。
const formEntries = await namedEntries(
  pokemonForms.filter((f) => f.form_identifier),
  pokemonFormNames,
  'id',
  'identifier',
  'pokemon_form_id',
  'pokemon_name',
);

const moves = await fetchCSV('moves.csv');
const moveNames = await fetchCSV('move_names.csv');
const moveEntries = await namedEntries(moves, moveNames, 'id', 'identifier', 'move_id');

const items = await fetchCSV('items.csv');
const itemNames = await fetchCSV('item_names.csv');
const itemEntries = await namedEntries(items, itemNames, 'id', 'identifier', 'item_id');

const abilities = await fetchCSV('abilities.csv');
const abilityNames = await fetchCSV('ability_names.csv');
const abilityEntries = await namedEntries(abilities, abilityNames, 'id', 'identifier', 'ability_id');

const types = await fetchCSV('types.csv');
const typeNames = await fetchCSV('type_names.csv');
const typeEntries = await namedEntries(types, typeNames, 'id', 'identifier', 'type_id');

// natures(ADR-0105 §4): 性格の日本語名。補正の正は Showdown で、ここは名前だけ。
const natures = await fetchCSV('natures.csv');
const natureNames = await fetchCSV('nature_names.csv');
const natureEntries = await namedEntries(natures, natureNames, 'id', 'identifier', 'nature_id');

const snapshot = {
  schemaVersion: 1,
  source: 'pokeapi',
  version: commit,
  species: speciesEntries,
  forms: formEntries,
  moves: moveEntries,
  items: itemEntries,
  abilities: abilityEntries,
  types: typeEntries,
  natures: natureEntries,
};

const raw = JSON.stringify(snapshot);
const outDir = `${root}data/generated/pokeapi/${commit}/`;
mkdirSync(outDir, { recursive: true });
writeFileSync(`${outDir}snapshot.json`, raw);
writeFileSync(
  `${outDir}meta.json`,
  JSON.stringify({ fetchedAt: new Date().toISOString(), commit, sha256: createHash('sha256').update(raw).digest('hex') }, null, 2),
);
console.log(`fetch-pokeapi: ${outDir}snapshot.json を書いた(種族名 ${speciesEntries.length} 件)`);
