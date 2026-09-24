// Showdown ソースのキャッシュ確保(issue #102)。中断しても次回自己回復するよう、
// tarball・展開先とも一時名で作り、検証してから公開名へ rename する。
//
// 完成の判定:
//   tarball = source.tar.gz が在り、meta.json に記録した sha256 と内容が一致する
//   展開先  = src/dist/sim/dex.js が在る(build 完了後にだけ src/ へ rename するため)
// どちらかが欠損・不整合なら、その部分だけを作り直す。正常な完成キャッシュは取得元へ再アクセスしない。
import { createHash, randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const sha256Of = (buf) => createHash('sha256').update(buf).digest('hex');

const readMeta = (metaPath) => {
  try {
    return JSON.parse(readFileSync(metaPath, 'utf8'));
  } catch {
    return null;
  }
};

const isTarballValid = (tarballPath, metaPath) => {
  if (!existsSync(tarballPath)) return false;
  const meta = readMeta(metaPath);
  if (!meta?.sha256) return false;
  return sha256Of(readFileSync(tarballPath)) === meta.sha256;
};

const rmrf = (path) => rmSync(path, { recursive: true, force: true });

// deps: { download(): Promise<Buffer>, extract(tarballPath, dir), install(dir), build(dir), log(msg) }
// 戻り値: 完成した展開先(`<cacheDir>/src/`)。
export async function ensureShowdownSource({ cacheDir, commit, url, deps }) {
  const { download, extract, install, build, log = () => {} } = deps;
  const tarballPath = join(cacheDir, 'source.tar.gz');
  const metaPath = join(cacheDir, 'meta.json');
  const srcDir = join(cacheDir, 'src');
  const distEntry = join(srcDir, 'dist', 'sim', 'dex.js');
  mkdirSync(cacheDir, { recursive: true });

  // 前回中断した一時物(*.partial-*)は再利用せず消す。
  for (const name of readdirSync(cacheDir)) {
    if (name.includes('.partial-')) rmrf(join(cacheDir, name));
  }

  if (isTarballValid(tarballPath, metaPath)) {
    log('fetch-showdown: キャッシュ済みの tarball を使う');
  } else {
    // 欠損・破損: tarball と、それに紐づく展開先をともに作り直す。
    rmrf(tarballPath);
    rmrf(metaPath);
    rmrf(srcDir);
    const buf = await download();
    const sha256 = sha256Of(buf);
    const suffix = `.partial-${randomBytes(4).toString('hex')}`;
    writeFileSync(`${tarballPath}${suffix}`, buf);
    renameSync(`${tarballPath}${suffix}`, tarballPath);
    // meta は tarball の後に公開する。間で中断すると meta 欠損 = 次回作り直しになる。
    writeFileSync(`${metaPath}${suffix}`, JSON.stringify({ commit, fetchedAt: new Date().toISOString(), sha256, url }, null, 2));
    renameSync(`${metaPath}${suffix}`, metaPath);
    log(`fetch-showdown: tarball を取得してキャッシュした(sha256=${sha256})`);
  }

  if (!existsSync(distEntry)) {
    rmrf(srcDir); // 空・build 途中の不完全な展開先
    const tmpDir = join(cacheDir, `src.partial-${randomBytes(4).toString('hex')}`);
    try {
      mkdirSync(tmpDir, { recursive: true });
      extract(tarballPath, tmpDir);
      install(tmpDir);
      build(tmpDir);
      if (!existsSync(join(tmpDir, 'dist', 'sim', 'dex.js'))) {
        throw new Error('Showdown の build 後に dist/sim/dex.js が無い');
      }
      renameSync(tmpDir, srcDir);
    } catch (err) {
      rmrf(tmpDir);
      throw err;
    }
  }
  return `${srcDir}/`;
}
