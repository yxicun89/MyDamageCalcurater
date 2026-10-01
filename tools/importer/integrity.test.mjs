// issue #222・#301(D19): 取得物の内容ハッシュによる照合(ADR-0101 追記)。
// 実行: node --test tools/importer/integrity.test.mjs
// ツリー・CSV はすべて架空の内容で、ネットワーク・実データ・npm は使わない。
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { test } from 'node:test';
import {
  EXIT_NEEDS_HUMAN,
  IntegrityError,
  exitCodeFor,
  expectedPokeapiCsvSha256,
  expectedShowdownTreeSha256,
  hashFileTree,
  sha256Hex,
  verifyFileSha256,
  verifyTreeSha256,
} from './integrity.mjs';

const sha = (s) => createHash('sha256').update(s).digest('hex');
const HEX_A = 'a'.repeat(64);
const HEX_B = 'b'.repeat(64);

const makeTree = (files) => {
  const dir = mkdtempSync(join(tmpdir(), 'integrity-'));
  for (const [rel, body] of Object.entries(files)) {
    mkdirSync(dirname(join(dir, rel)), { recursive: true });
    writeFileSync(join(dir, rel), body);
  }
  return dir;
};

// 仕様(ADR-0101 追記): 通常ファイルごとに `<内容の sha256>  <ルートからの相対パス(/ 区切り)>\n` の行を作り、
// 相対パス全体の文字列(コードユニット順。ロケール依存にしない)で並べて連結し、その sha256 を取る。
// ディレクトリ・空ディレクトリ・実行ビット・時刻・所有者は見ない。
const expectedHash = (entries) =>
  sha(
    [...entries]
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([path, digest]) => `${digest}  ${path}\n`)
      .join(''),
  );

test('ツリーのハッシュは「内容の sha256 + パス」の昇順の一覧の sha256', () => {
  const dir = makeTree({ 'a.txt': 'x', 'b/c.txt': 'y' });
  assert.equal(hashFileTree(dir), expectedHash([['a.txt', sha('x')], ['b/c.txt', sha('y')]]));
  rmSync(dir, { recursive: true, force: true });
});

test('並びはディレクトリ走査順でなくパス全体の文字列順(a-b.txt は a/b.txt より前)', () => {
  const dir = makeTree({ 'a/b.txt': '1', 'a-b.txt': '2', 'a.txt': '3' });
  assert.equal(
    hashFileTree(dir),
    expectedHash([['a/b.txt', sha('1')], ['a-b.txt', sha('2')], ['a.txt', sha('3')]]),
  );
  rmSync(dir, { recursive: true, force: true });
});

test('作成順が違っても、同じパスと内容なら同じハッシュ', () => {
  const d1 = makeTree({ 'z.txt': 'z', 'a.txt': 'a' });
  const d2 = makeTree({ 'a.txt': 'a', 'z.txt': 'z' });
  assert.equal(hashFileTree(d1), hashFileTree(d2));
  rmSync(d1, { recursive: true, force: true });
  rmSync(d2, { recursive: true, force: true });
});

test('内容・パス・ファイルの有無が変わればハッシュが変わる', () => {
  const base = makeTree({ 'a.txt': 'a', 'd/b.txt': 'b' });
  const h = hashFileTree(base);
  for (const files of [
    { 'a.txt': 'A', 'd/b.txt': 'b' }, // 内容
    { 'a2.txt': 'a', 'd/b.txt': 'b' }, // パス
    { 'a.txt': 'a', 'd/b.txt': 'b', 'extra.txt': '' }, // 追加(空ファイルでも)
    { 'a.txt': 'a' }, // 欠落
  ]) {
    const d = makeTree(files);
    assert.notEqual(hashFileTree(d), h, JSON.stringify(Object.keys(files)));
    rmSync(d, { recursive: true, force: true });
  }
  rmSync(base, { recursive: true, force: true });
});

test('空ディレクトリ・実行ビットはハッシュに影響しない', () => {
  const d1 = makeTree({ 'a.txt': 'a' });
  const d2 = makeTree({ 'a.txt': 'a' });
  mkdirSync(join(d2, 'empty'));
  chmodSync(join(d2, 'a.txt'), 0o755);
  assert.equal(hashFileTree(d1), hashFileTree(d2));
  rmSync(d1, { recursive: true, force: true });
  rmSync(d2, { recursive: true, force: true });
});

test('シンボリックリンクは辿らず、リンク先の文字列を内容として数える', () => {
  const outside = makeTree({ 'secret.txt': 'one' });
  const dir = makeTree({ 'a.txt': 'a' });
  symlinkSync(join(outside, 'secret.txt'), join(dir, 'link'));
  const h1 = hashFileTree(dir);
  // リンク先の中身が変わってもツリーのハッシュは変わらない(ツリーの外は見ない)。
  writeFileSync(join(outside, 'secret.txt'), 'two');
  assert.equal(hashFileTree(dir), h1);
  assert.equal(
    h1,
    expectedHash([['a.txt', sha('a')], ['link', sha(`symlink:${join(outside, 'secret.txt')}`)]]),
  );
  rmSync(outside, { recursive: true, force: true });
  rmSync(dir, { recursive: true, force: true });
});

test('sha256Hex は 16進小文字64桁', () => {
  assert.equal(sha256Hex('abc'), sha('abc'));
  assert.equal(sha256Hex(Buffer.from('abc')), sha('abc'));
});

test('verifyTreeSha256: 一致なら何も起きない・不一致は終了コード3の IntegrityError(期待値と実際を含む)', () => {
  verifyTreeSha256({ label: 'showdown', actual: HEX_A, expected: HEX_A });
  assert.throws(
    () => verifyTreeSha256({ label: 'showdown', actual: HEX_A, expected: HEX_B }),
    (err) => {
      assert.ok(err instanceof IntegrityError);
      assert.equal(err.exitCode, 3);
      assert.match(err.message, /showdown/);
      assert.ok(err.message.includes(HEX_A) && err.message.includes(HEX_B));
      return true;
    },
  );
});

test('期待値が無い・形が不正なら fail closed(同じ IntegrityError)', () => {
  for (const expected of [undefined, null, '', 'abc', 'A'.repeat(64), 'z'.repeat(64)]) {
    assert.throws(
      () => verifyTreeSha256({ label: 'showdown', actual: HEX_A, expected }),
      IntegrityError,
      String(expected),
    );
  }
});

test('verifyFileSha256: CSV などの内容(バイト列)を期待ハッシュの表で照合する', () => {
  const content = Buffer.from('id,identifier\n1,ja\n');
  const expected = { 'languages.csv': sha(content) };
  verifyFileSha256({ name: 'languages.csv', content, expected });
  assert.throws(
    () => verifyFileSha256({ name: 'languages.csv', content: Buffer.from('id,identifier\n1,en\n'), expected }),
    (err) => err instanceof IntegrityError && err.exitCode === 3 && /languages\.csv/.test(err.message),
  );
  // 表に無いファイルは通さない(新しい CSV を黙って信頼しない)。
  assert.throws(() => verifyFileSha256({ name: 'moves.csv', content, expected }), IntegrityError);
  assert.throws(() => verifyFileSha256({ name: 'languages.csv', content, expected: undefined }), IntegrityError);
});

test('config の integrity を読む: 無ければ fail closed', () => {
  const config = {
    integrity: { showdown: { treeSha256: HEX_A }, pokeapi: { csvSha256: { 'moves.csv': HEX_B } } },
  };
  assert.equal(expectedShowdownTreeSha256(config), HEX_A);
  assert.deepEqual(expectedPokeapiCsvSha256(config), { 'moves.csv': HEX_B });
  for (const bad of [{}, { integrity: {} }, { integrity: { showdown: {} } }, { integrity: { showdown: { treeSha256: 'xyz' } } }]) {
    assert.throws(() => expectedShowdownTreeSha256(bad), IntegrityError);
  }
  for (const bad of [{}, { integrity: {} }, { integrity: { pokeapi: { csvSha256: {} } } }, { integrity: { pokeapi: { csvSha256: { 'a.csv': 'xyz' } } } }]) {
    assert.throws(() => expectedPokeapiCsvSha256(bad), IntegrityError);
  }
});

test('終了コード: IntegrityError は 3(人間対応)、それ以外は 1(再試行で直りうる)', () => {
  assert.equal(EXIT_NEEDS_HUMAN, 3);
  assert.equal(exitCodeFor(new IntegrityError('x')), 3);
  assert.equal(exitCodeFor(new Error('network')), 1);
  assert.equal(exitCodeFor('文字列'), 1);
});
