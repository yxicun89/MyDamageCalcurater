// 取得元(calc / Showdown / PokeAPI)を順に取り込む(ADR-0101 §1・§3)。`make import-fetch` から呼ぶ。
// ネットワークの礼儀のため逐次実行し、リクエスト間に短い待ち時間を入れる
// (キャッシュ済みのものは各スクリプトが再取得しない)。
import { exitCodeFor } from './integrity.mjs';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

for (const script of ['./fetch-calc.mjs', './fetch-showdown.mjs', './fetch-pokeapi.mjs']) {
  console.log(`fetch: ${script} を実行`);
  try {
    await import(script);
  } catch (err) {
    // IntegrityError は終了コード 3(人間対応)、それ以外は 1。
    console.error(`fetch: ${script} が失敗: ${err.message}`);
    process.exit(exitCodeFor(err));
  }
  await sleep(500);
}
