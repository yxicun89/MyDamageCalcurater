// issue #311: PokeAPI の CSV の列数違いの行を黙って捨てない。実行: node --test tools/importer/pokeapi-csv.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseCSV } from './pokeapi-csv.mjs';

test('ヘッダの列名で行をオブジェクトにする(引用符内のカンマ・改行・二重引用符)', () => {
  const rows = parseCSV('id,name\n1,"a,b"\n2,"x\ny"\n3,"q""q"\n', 'test.csv');
  assert.deepEqual(rows, [
    { id: '1', name: 'a,b' },
    { id: '2', name: 'x\ny' },
    { id: '3', name: 'q"q' },
  ]);
});

test('末尾に改行が無くても最後の行を読む・CRLF を受け付ける', () => {
  assert.deepEqual(parseCSV('id,name\r\n1,a\r\n2,b', 'test.csv'), [
    { id: '1', name: 'a' },
    { id: '2', name: 'b' },
  ]);
});

test('列数がヘッダと違う行があれば件数とファイル名を出して失敗する', () => {
  assert.throws(
    () => parseCSV('id,name\n1,a\n2\n3,c,extra\n4,d\n', 'moves.csv'),
    (err) => err instanceof Error && /moves\.csv/.test(err.message) && /2 行/.test(err.message),
  );
});

test('途中で切れたファイル(最後の行の列が足りない)も失敗する', () => {
  assert.throws(() => parseCSV('id,name,extra\n1,a,x\n2,b', 'cut.csv'), /cut\.csv/);
});
