// 取得物の内容ハッシュによる照合(issue #222・#301。ADR-0101 追記 2026-10-01)。
// 第三者のコードを実行する前・解析する前に、config.json の integrity と照合する。
// 不一致・期待値なし・形式不正は fail closed(IntegrityError。終了コード 3 = 人間対応)。
import { createHash } from 'node:crypto';
import { lstatSync, readdirSync, readFileSync, readlinkSync } from 'node:fs';
import { join } from 'node:path';

export const EXIT_NEEDS_HUMAN = 3;

const HEX64 = /^[0-9a-f]{64}$/;

export class IntegrityError extends Error {
  constructor(message) {
    super(message);
    this.name = 'IntegrityError';
    this.exitCode = EXIT_NEEDS_HUMAN;
  }
}

export const exitCodeFor = (err) => (err instanceof IntegrityError ? err.exitCode : 1);

export const sha256Hex = (data) => createHash('sha256').update(data).digest('hex');

// 通常ファイルごとに `<内容の sha256>  <相対パス>\n` を作り、パス文字列順に連結して sha256 を取る。
// シンボリックリンクは辿らず、sha256("symlink:" + リンク先の文字列) を内容として数える。
export function hashFileTree(root) {
  const entries = [];
  const walk = (dir, prefix) => {
    for (const name of readdirSync(dir)) {
      const abs = join(dir, name);
      const rel = prefix === '' ? name : `${prefix}/${name}`;
      const st = lstatSync(abs);
      if (st.isSymbolicLink()) entries.push([rel, sha256Hex(`symlink:${readlinkSync(abs)}`)]);
      else if (st.isDirectory()) walk(abs, rel);
      else if (st.isFile()) entries.push([rel, sha256Hex(readFileSync(abs))]);
    }
  };
  walk(root, '');
  entries.sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  return sha256Hex(entries.map(([p, d]) => `${d}  ${p}\n`).join(''));
}

const requireHex = (label, expected) => {
  if (typeof expected !== 'string' || !HEX64.test(expected)) {
    throw new IntegrityError(`${label}: 期待ハッシュが無い・形式不正(64桁の16進小文字)。config.json の integrity を確認する: ${JSON.stringify(expected)}`);
  }
};

export function verifyTreeSha256({ label, actual, expected }) {
  requireHex(label, expected);
  if (actual !== expected) {
    throw new IntegrityError(`${label}: 内容ハッシュが一致しない。期待=${expected} 実際=${actual}。内容を確かめたうえで config.json の integrity を更新する PR を出す`);
  }
}

export function verifyFileSha256({ name, content, expected }) {
  const want = expected?.[name];
  requireHex(name, want);
  const actual = sha256Hex(content);
  if (actual !== want) {
    throw new IntegrityError(`${name}: 内容ハッシュが一致しない。期待=${want} 実際=${actual}。内容を確かめたうえで config.json の integrity を更新する PR を出す`);
  }
}

export function expectedShowdownTreeSha256(config) {
  const v = config?.integrity?.showdown?.treeSha256;
  requireHex('integrity.showdown.treeSha256', v);
  return v;
}

export function expectedPokeapiCsvSha256(config) {
  const table = config?.integrity?.pokeapi?.csvSha256;
  if (!table || typeof table !== 'object' || Object.keys(table).length === 0) {
    throw new IntegrityError('integrity.pokeapi.csvSha256 が無い。config.json を確認する');
  }
  for (const [name, v] of Object.entries(table)) requireHex(`integrity.pokeapi.csvSha256[${name}]`, v);
  return table;
}
