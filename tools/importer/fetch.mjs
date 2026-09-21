// 取得元(calc / Showdown / PokeAPI)を順に取り込む(ADR-0101 §1・§3)。`make import-fetch` から呼ぶ。
// ネットワークの礼儀のため逐次実行し、リクエスト間に短い待ち時間を入れる
// (キャッシュ済みのものは各スクリプトが再取得しない)。
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

for (const script of ['./fetch-calc.mjs', './fetch-showdown.mjs', './fetch-pokeapi.mjs']) {
  console.log(`fetch: ${script} を実行`);
  await import(script);
  await sleep(500);
}
