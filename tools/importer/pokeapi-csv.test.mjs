// issue #311: PokeAPI の CSV の列数違いの行を黙って捨てない。実行: node --test tools/importer/pokeapi-csv.test.mjs
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { collectNames, parseCSV } from './pokeapi-csv.mjs';

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

// issue #349: languages.csv の identifier は小文字の ja-hrkt。大文字小文字で取りこぼさない。
test('識別子が小文字(ja-hrkt)でも読み仮名の名前を ja-Hrkt のキーで集める', () => {
  const langs = new Map([['1', 'ja-hrkt'], ['2', 'ja'], ['3', 'en']]);
  const rows = [
    { local_language_id: '1', move_id: '10', name: 'かな' },
    { local_language_id: '2', move_id: '10', name: '漢字' },
    { local_language_id: '3', move_id: '10', name: 'English' },
    { local_language_id: '1', move_id: '11', name: 'だけかな' },
  ];
  const got = collectNames(rows, 'move_id', 'name', langs, ['ja-Hrkt', 'ja']);
  assert.deepEqual(got.get('10'), { 'ja-Hrkt': 'かな', ja: '漢字' });
  assert.deepEqual(got.get('11'), { 'ja-Hrkt': 'だけかな' });
});

// --- 姿の名前(ADR-0141) ---------------------------------------------------------------------

test('collectFormEntries は完全名と姿の名前を別々に出し、既定の姿は出さない', async () => {
  const { collectFormEntries } = await import('./pokeapi-csv.mjs');
  const langs = new Map([['1', 'ja-Hrkt'], ['2', 'en']]);
  const forms = parseCSV('id,identifier,form_identifier\n1,fake,\n2,fake-alola,alola\n3,fake-own,own\n4,other-x,x\n', 'f.csv');
  const formNames = parseCSV(
    'pokemon_form_id,local_language_id,form_name,pokemon_name\n2,1,アローラのすがた,\n2,2,Alolan Form,\n3,1,,フェイク（じぶん）\n1,1,ふつう,\n',
    'n.csv',
  );
  assert.deepEqual(collectFormEntries(forms, formNames, langs, ['ja-Hrkt', 'ja']), [
    { slug: 'fake-alola', names: { 'ja-Hrkt': '' }, formNames: { 'ja-Hrkt': 'アローラのすがた' } },
    { slug: 'fake-own', names: { 'ja-Hrkt': 'フェイク（じぶん）' }, formNames: { 'ja-Hrkt': '' } },
  ]);
});
