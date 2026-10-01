// importer の PVC(data/generated)の容量確認と保持 prune(issue #111・ADR-0104 追記)。
//   node prune.mjs check   取得より前に、空きが予約容量以上か確かめる。足りなければ終了コード 3
//   node prune.mjs prune   成功を台帳へ記録してから、保持しない旧版と古い report を消す
// 環境変数: IMPORT_GENERATED_DIR(既定 data/generated)・IMPORT_CONFIG_FILE(既定 data/importer/config.json)・
//           IMPORT_RESERVE_BYTES(既定 DEFAULT_RESERVE_BYTES)
// ログには相対パス・byte 数だけを出す(絶対パス・取得物の中身は出さない)。
import { existsSync, lstatSync, readdirSync, readFileSync, renameSync, rmSync, statfsSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// 予約容量の既定値: Showdown の新版 1 つ分(tarball + 展開済み tree + npm ci --omit=dev の依存 + build 結果)は
// 実測で約 190 MiB。展開中は一時ディレクトリと公開名が一時的に併存するので、その約 2 倍の 400 MiB を空きの下限にする。
export const DEFAULT_RESERVE_BYTES = 400 * 1024 * 1024;
export const EXIT_CAPACITY = 3; // 容量不足(再試行しても直らない。runbook の手順で人が回復する)
const LEDGER = '.import-success.json';
const LEDGER_MAX = 8; // 台帳が覚える成功版の数(保持は keepGenerations 世代だけ使う)
const SOURCES = ['calc', 'showdown', 'pokeapi'];
const REPORT_RE = /^import-.*\.json$/;

export class InsufficientSpaceError extends Error {}
export const EXIT_USAGE = 2; // 使い方・環境変数の誤り
export class UsageError extends Error {}

const VERSION_RE = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
const assertVersion = (source, v) => {
  if (typeof v !== 'string' || !VERSION_RE.test(v) || v.includes('..')) {
    throw new Error(`config の ${source} の版名が不正(パスとして使えない)`);
  }
};

const readLedger = (generatedDir) => {
  try {
    const l = JSON.parse(readFileSync(join(generatedDir, LEDGER), 'utf8'));
    return l && typeof l.sources === 'object' && l.sources && !Array.isArray(l.sources) ? l.sources : {};
  } catch {
    return {};
  }
};

// 成功した run の版を台帳へ記録する(古い順。同じ版は末尾へ動かして重複させない)。
export const recordSuccess = ({ generatedDir, versions, now = new Date() }) => {
  const sources = readLedger(generatedDir);
  for (const s of SOURCES) {
    if (versions[s] === undefined) continue;
    assertVersion(s, versions[s]);
    const list = (Array.isArray(sources[s]) ? sources[s] : []).filter((v) => v !== versions[s]);
    sources[s] = [...list, versions[s]].slice(-LEDGER_MAX);
  }
  const path = join(generatedDir, LEDGER);
  const tmp = `${path}.partial-${process.pid}`;
  writeFileSync(tmp, JSON.stringify({ schemaVersion: 1, updatedAt: now.toISOString(), sources }, null, 2));
  renameSync(tmp, path);
};

const dirNames = (dir) => {
  try {
    if (lstatSync(dir).isSymbolicLink()) return []; // base 自体が symlink なら何も触らない
    return readdirSync(dir, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name);
  } catch {
    return [];
  }
};

// 削除候補を列挙する(消さない)。remove は generatedDir からの相対パス。
export const planPrune = ({ generatedDir, config, keepGenerations = 2, keepReports = 52 }) => {
  const ledger = readLedger(generatedDir);
  const remove = [];
  for (const s of SOURCES) {
    const current = config.sources?.[s];
    assertVersion(s, current);
    const previous = (Array.isArray(ledger[s]) ? ledger[s] : []).filter((v) => v !== current).reverse();
    const keep = new Set([current, ...previous.slice(0, Math.max(0, keepGenerations - 1))]);
    for (const base of [join('.cache', s), s]) {
      for (const name of dirNames(join(generatedDir, base))) {
        if (name.includes('.partial-') || keep.has(name)) continue;
        remove.push(join(base, name));
      }
    }
  }
  try {
    const reports = readdirSync(join(generatedDir, 'reports'), { withFileTypes: true })
      .filter((e) => e.isFile() && REPORT_RE.test(e.name))
      .map((e) => e.name)
      .sort()
      .reverse();
    for (const name of reports.slice(keepReports)) remove.push(join('reports', name));
  } catch {
    // reports/ が無ければ何もしない
  }
  return { remove: remove.sort() };
};

const sizeOf = (path) => {
  let st;
  try {
    st = lstatSync(path);
  } catch {
    return 0;
  }
  if (!st.isDirectory()) return st.size;
  return readdirSync(path).reduce((sum, n) => sum + sizeOf(join(path, n)), 0);
};

const defaultStatfs = (dir) => {
  const s = statfsSync(dir);
  return { totalBytes: s.bsize * s.blocks, freeBytes: s.bsize * s.bavail };
};

// 取得より前に、新版 1 つ分の予約容量が空いているか確かめる。
export const checkReserve = ({ generatedDir, reserveBytes = DEFAULT_RESERVE_BYTES, statfs = defaultStatfs, log = () => {} }) => {
  const { totalBytes, freeBytes } = statfs(generatedDir);
  const usedBytes = totalBytes - freeBytes;
  log(`importer-capacity: total=${totalBytes} used=${usedBytes} free=${freeBytes} reserve=${reserveBytes}`);
  for (const s of SOURCES) {
    for (const base of [join('.cache', s), s]) {
      for (const name of dirNames(join(generatedDir, base))) {
        log(`importer-capacity: usage ${join(base, name)} ${sizeOf(join(generatedDir, base, name))}`);
      }
    }
  }
  if (freeBytes < reserveBytes) {
    throw new InsufficientSpaceError(
      `importer-capacity: 空き ${freeBytes} byte が予約容量 ${reserveBytes} byte を下回る。取得・DB 更新の前に停止した(docs/runbooks/data.md の「importer の PVC の容量」)`,
    );
  }
  return { totalBytes, freeBytes, usedBytes };
};

const defaultRm = (path) => rmSync(path, { recursive: true, force: true });

// 現在版の snapshot が揃っているか確かめる(無ければ新版の取得失敗なので例外)。
export const assertCurrentSnapshots = ({ generatedDir, config }) => {
  for (const s of SOURCES) {
    assertVersion(s, config.sources?.[s]);
    if (!existsSync(join(generatedDir, s, config.sources[s], 'snapshot.json'))) {
      throw new Error(`現在版の snapshot が無い(${s}/${config.sources[s]})。何も消さずに止める`);
    }
  }
};

// 保持しないものを消す。現在版の snapshot が無ければ何も消さずに例外にする。
export const pruneGenerated = async ({
  generatedDir, config, log = () => {}, statfs = defaultStatfs, rm = defaultRm, keepGenerations = 2, keepReports = 52,
}) => {
  assertCurrentSnapshots({ generatedDir, config });
  const { remove } = planPrune({ generatedDir, config, keepGenerations, keepReports });
  const removed = [];
  let reclaimedBytes = 0;
  for (const rel of remove) {
    const abs = join(generatedDir, rel);
    const size = sizeOf(abs);
    await rm(abs);
    removed.push(rel);
    reclaimedBytes += size;
    log(`importer-prune: 削除 ${rel} ${size} byte`);
  }
  const { freeBytes } = await statfs(generatedDir);
  log(`importer-prune: 削除 ${removed.length} 件・回収 ${reclaimedBytes} byte・残量 ${freeBytes} byte`);
  return { removed, reclaimedBytes, freeBytes };
};

const main = async (cmd) => {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
  const generatedDir = process.env.IMPORT_GENERATED_DIR || join(root, 'data/generated');
  const log = (m) => console.log(m);
  if (cmd === 'check') {
    const raw = process.env.IMPORT_RESERVE_BYTES ?? String(DEFAULT_RESERVE_BYTES);
    const reserveBytes = /^\d+$/.test(raw) ? Number(raw) : NaN;
    if (!Number.isSafeInteger(reserveBytes)) {
      throw new UsageError('IMPORT_RESERVE_BYTES は 0 以上の整数(byte)で指定する');
    }
    checkReserve({ generatedDir, reserveBytes, log });
  } else if (cmd === 'prune') {
    const configFile = process.env.IMPORT_CONFIG_FILE || join(root, 'data/importer/config.json');
    const config = JSON.parse(readFileSync(configFile, 'utf8'));
    // fetch・reconcile・DB apply が成功した後にだけ呼ばれる(cronjob.sh)ので、ここで成功を記録する。
    assertCurrentSnapshots({ generatedDir, config }); // 失敗 run の版を台帳へ載せない
    recordSuccess({ generatedDir, versions: config.sources, now: new Date() });
    await pruneGenerated({ generatedDir, config, log });
  } else {
    throw new UsageError('使い方: node prune.mjs check|prune');
  }
};

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv[2]).catch((e) => {
    // fs のエラーは message に絶対パスを含むので、errno コードだけ出す。
    console.error(e?.code && e?.syscall ? `importer-prune: ファイル操作に失敗した(${e.code} ${e.syscall})` : e.message);
    process.exit(e instanceof InsufficientSpaceError ? EXIT_CAPACITY : e instanceof UsageError ? EXIT_USAGE : 1);
  });
}
