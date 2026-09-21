// PokeAPI(PokeAPI/pokeapi リポジトリの commit を固定)の CSV から日本語名だけを抽出する
// (ADR-0101 §3)。REST を種族・技・持ち物・特性ごとに約1,200回叩かず、
// commit 固定の CSV(data/v2/csv/*.csv)を raw で取得する(公平利用・版の固定)。
//
// 実行: node fetch-pokeapi.mjs
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const config = JSON.parse(readFileSync(`${root}data/importer/config.json`, 'utf8'));
const commit = config.sources?.pokeapi;
if (!commit || !/^[0-9a-f]{40}$/.test(commit)) {
  throw new Error(`config.json の sources.pokeapi が40桁の commit でない: ${commit}`);
}

const LANGUAGES = ['ja-Hrkt', 'ja']; // ADR-0101 §3: この2言語だけ出す(取り込む言語の絞り込みは config 側)
const cacheDir = `${root}data/generated/.cache/pokeapi/${commit}/`;
mkdirSync(cacheDir, { recursive: true });

// parseCSV は簡易 RFC4180 パーサ(ダブルクォートで囲まれたカンマ・改行に対応)。
function parseCSV(text) {
  const rows = [];
  let row = [];
  let field = '';
  let inQuotes = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuotes) {
      if (c === '"' && text[i + 1] === '"') { field += '"'; i++; }
      else if (c === '"') { inQuotes = false; }
      else { field += c; }
    } else if (c === '"') {
      inQuotes = true;
    } else if (c === ',') {
      row.push(field); field = '';
    } else if (c === '\n') {
      row.push(field); field = ''; rows.push(row); row = [];
    } else if (c !== '\r') {
      field += c;
    }
  }
  if (field.length > 0 || row.length > 0) { row.push(field); rows.push(row); }
  const header = rows.shift();
  return rows.filter((r) => r.length === header.length).map((r) => Object.fromEntries(header.map((h, i) => [h, r[i]])));
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ネットワークの礼儀(ADR-0101 §3): 逐次取得し、実際にネットワークへ出たときだけ
// リクエスト間に待ち時間を入れる(キャッシュ済みは待たない)。
async function fetchCSV(name) {
  const cachePath = `${cacheDir}${name}`;
  if (existsSync(cachePath)) {
    return parseCSV(readFileSync(cachePath, 'utf8'));
  }
  const url = `https://raw.githubusercontent.com/PokeAPI/pokeapi/${commit}/data/v2/csv/${name}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`PokeAPI CSV を取得できない: ${res.status} ${url}`);
  const text = await res.text();
  writeFileSync(cachePath, text);
  await sleep(300);
  return parseCSV(text);
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
