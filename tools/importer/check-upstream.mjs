// 上流(npm の @smogon/calc・smogon/pokemon-showdown・PokeAPI/pokeapi)の最新版を検出し、
// data/generated/upstream/latest.json に書く(ADR-0104 §4)。比較対象の source は
// data/importer/config.json の sources から読む。ネットワークに出るのはこのスクリプトだけ
// (Go の importer はこのファイルを読んで固定版と比べるだけで、実行時にネットワークへ出ない)。
//
// 週1回 tools/importer/cronjob.sh から呼ばれる。失敗しても取り込みは止めない
// (cronjob.sh 側で `|| ...` により警告にする)。config.json は書き換えない
// (版を上げるのは人の PR。ADR-0104 §1)。
//
// 実行: node check-upstream.mjs
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// リポジトリ名・ブランチ名は取得元の識別子(fetch-*.mjs と同じ流儀で、ここだけに置く。Go には書かない)。
const SHOWDOWN_REPO = 'smogon/pokemon-showdown';
const SHOWDOWN_BRANCH = 'master';
const POKEAPI_REPO = 'PokeAPI/pokeapi';
const POKEAPI_BRANCH = 'master';

const root = fileURLToPath(new URL('../../', import.meta.url));
const config = JSON.parse(readFileSync(`${root}data/importer/config.json`, 'utf8'));
const pinnedSources = Object.keys(config.sources ?? {}).sort();

// 1回の問い合わせの上限(応答が無いまま取り込みを遅らせない)。
const FETCH_TIMEOUT_MS = 30_000;

async function fetchLatestCalc() {
  const res = await fetch('https://registry.npmjs.org/@smogon/calc/latest', { signal: AbortSignal.timeout(FETCH_TIMEOUT_MS) });
  if (!res.ok) throw new Error(`npm registry から @smogon/calc の latest を取得できない: HTTP ${res.status}`);
  const body = await res.json();
  if (!body.version) throw new Error('npm registry の応答に version が無い');
  return body.version;
}

// GitHub API の commit(未認証。Accept: application/vnd.github.sha で40桁の16進だけを返させる。
// 未認証の上限 60 回/時に対して週1回・2回で十分少ない。トークンは使わない)。
async function fetchLatestCommit(repo, branch) {
  const res = await fetch(`https://api.github.com/repos/${repo}/commits/${branch}`, {
    headers: { Accept: 'application/vnd.github.sha' },
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!res.ok) throw new Error(`GitHub API から ${repo}@${branch} の commit を取得できない: HTTP ${res.status}`);
  const sha = (await res.text()).trim();
  if (!/^[0-9a-f]{40}$/.test(sha)) throw new Error(`${repo}@${branch} の応答が commit(40桁の16進)でない: ${sha}`);
  return sha;
}

// source ごとの検出手段。config.json に無い source(検出手段を持たない)は素通りし、
// Go 側で not-checked(unknown)として扱われる。
const fetchers = {
  calc: fetchLatestCalc,
  showdown: () => fetchLatestCommit(SHOWDOWN_REPO, SHOWDOWN_BRANCH),
  pokeapi: () => fetchLatestCommit(POKEAPI_REPO, POKEAPI_BRANCH),
};

const sources = {};
const errors = {};
for (const source of pinnedSources) {
  const fetcher = fetchers[source];
  if (!fetcher) continue;
  try {
    sources[source] = await fetcher();
  } catch (err) {
    errors[source] = String(err?.message ?? err);
    console.warn(`check-upstream: ${source} の検出に失敗: ${errors[source]}`);
  }
}

const result = {
  schemaVersion: 1,
  checkedAt: new Date().toISOString(),
  sources,
  errors,
};

// 一部・全部が取れなくても毎回書き直す(古い結果を残して「最新」と誤らせない)。
const outPath = `${root}data/generated/upstream/latest.json`;
mkdirSync(`${root}data/generated/upstream/`, { recursive: true });
writeFileSync(outPath, JSON.stringify(result, null, 2));
console.log(`check-upstream: ${outPath} を書いた(検出 ${Object.keys(sources).length} 件・失敗 ${Object.keys(errors).length} 件)`);
