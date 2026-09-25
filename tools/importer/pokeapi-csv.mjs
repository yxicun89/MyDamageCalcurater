// PokeAPI の CSV(data/v2/csv/*.csv)の簡易 RFC4180 パーサ(ダブルクォートで囲まれたカンマ・改行に対応)。
// fetch-pokeapi.mjs から使う。列数がヘッダと違う行(キャッシュの途中切れ・上流の形式変化)は黙って
// 捨てず、件数を出して失敗する(issue #311。捨てると名前が静かに欠けて英語名へのフォールバックになる)。

// parseCSV は text をヘッダの列名をキーにしたオブジェクトの配列にする。name はエラー表示用のファイル名。
export function parseCSV(text, name) {
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
  const bad = rows.filter((r) => r.length !== header.length);
  if (bad.length > 0) {
    throw new Error(`PokeAPI CSV ${name} に列数がヘッダ(${header.length} 列)と違う行が ${bad.length} 行ある(キャッシュの途中切れ・上流の形式変化を疑う)`);
  }
  return rows.map((r) => Object.fromEntries(header.map((h, i) => [h, r[i]])));
}
