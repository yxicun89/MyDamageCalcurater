// 個人利用のためにポケモン画像を取得し、data/generated/images/src/{key}.png に置く(ADR-0810)。
//
//   node tools/assets/fetch.mjs [--limit N] [--dry-run]
//
// 入力は取り込み済みマスタ: ① readmodel の pokemon-types.json(取り込み済みの種族 key 一覧)
// ② importer が取得した Showdown スナップショット(num・baseSpecies・forme・formeOrder。key の採番に使う)。
// 入手元は Pokémon Showdown の sprites(home → gen5 の順に試す)。URL は Showdown の種族データから組み立て、名前は書かない。
// 取得は直列(同時 1 本)で、1 リクエストごとに待つ。既にあるファイルは取り直さない(冪等)。失敗した key は一覧で報告するが終了コードは 0(画像なしでもエンブレムで動く)。
// 環境変数: ASSETS_SRC(出力先)・ASSETS_SHOWDOWN_SNAPSHOT・ASSETS_KEYS_FILE・ASSETS_SPRITES_BASE_URL。

import { mkdir, readFile, rename, stat, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = join(here, '..', '..')

// convert.mjs の DEFAULT_SRC_DIR と同じ場所(convert.mjs は sharp を読むので、取得だけなら依存させない)。
export const DEFAULT_SRC_DIR = join(repoRoot, 'data', 'generated', 'images', 'src')
export const DEFAULT_BASE_URL = 'https://play.pokemonshowdown.com/sprites'
/** 試す順。home が高画質(192px・透過)、gen5 は小さいが新しい姿が先に載ることがある。 */
export const SPRITE_SETS = ['home', 'gen5']
export const USER_AGENT = 'pokecalc-personal-assets/1.0 (personal use; low rate)'
/** 取得は直列(同時 1 本。並列にしない。ADR-0810)。各リクエストの後にこの時間待つ。 */
export const DEFAULT_DELAY_MS = 500
export const MAX_RETRIES = 2
const PNG_MAGIC = Buffer.from([0x89, 0x50, 0x4e, 0x47])

export const toID = (s) => String(s).toLowerCase().replace(/[^a-z0-9]/g, '')

/** Showdown の sprite 名: 基本種 id + '-' + フォーム id(例 charizard-megax・meowth-alola)。フォームなしは基本種 id。 */
export function spriteId(sp) {
  const base = toID(sp.baseSpecies || sp.name)
  const forme = toID(sp.forme || '')
  return forme ? `${base}-${forme}` : base
}

/** Showdown の種族一覧から {key → sprite 名} を作る。key = 図鑑番号4桁-フォルム3桁(formeOrder の位置。pokedex importer と同じ規則)。 */
export function buildKeyMap(species) {
  const byName = new Map(species.map((s) => [s.name, s]))
  const map = new Map()
  for (const sp of species) {
    const base = byName.get(sp.baseSpecies)
    if (!base) continue
    const order = base.formeOrder ?? []
    let form = order.indexOf(sp.name)
    if (form < 0 && order.length === 0 && sp.name === base.name && !sp.forme) form = 0
    if (form < 0 || !Number.isInteger(sp.num) || sp.num <= 0) continue
    const key = `${String(sp.num).padStart(4, '0')}-${String(form).padStart(3, '0')}`
    if (!map.has(key)) map.set(key, spriteId(sp))
  }
  return map
}

/** 取得対象 = 取り込み済みの key(昇順)のうち sprite 名を引けるもの。引けない key は unmapped に入れる。 */
export function buildTargets(keys, keyMap) {
  const targets = []
  const unmapped = []
  for (const key of [...new Set(keys)].sort()) {
    const id = keyMap.get(key)
    if (id) targets.push({ key, spriteId: id })
    else unmapped.push(key)
  }
  return { targets, unmapped }
}

export function candidateUrls(baseUrl, id) {
  return SPRITE_SETS.map((set) => `${baseUrl.replace(/\/$/, '')}/${set}/${id}.png`)
}

const defaultSleep = (ms) => new Promise((r) => setTimeout(r, ms))

/** 1 URL を取る。5xx・429・通信エラーは MAX_RETRIES 回まで待って再試行。404 などは再試行しない。 */
async function getPng(url, { fetchImpl, sleep, delayMs }) {
  let lastErr = 'unknown'
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    if (attempt > 0) await sleep(delayMs * 2 ** attempt * 3)
    try {
      const res = await fetchImpl(url, { headers: { 'User-Agent': USER_AGENT, Accept: 'image/png' } })
      if (res.status === 404) return { notFound: true }
      if (res.status === 429 || res.status >= 500) {
        lastErr = `http_${res.status}`
        continue
      }
      if (!res.ok) return { error: `http_${res.status}` }
      const buf = Buffer.from(await res.arrayBuffer())
      if (buf.length < 8 || !buf.subarray(0, 4).equals(PNG_MAGIC)) return { error: 'not_png' }
      return { buf }
    } catch (err) {
      lastErr = `network_${err?.code ?? 'error'}`
    }
  }
  return { error: lastErr }
}

async function exists(path) {
  try {
    return (await stat(path)).size > 0
  } catch {
    return false
  }
}

/**
 * targets を srcDir/{key}.png へ直列に取得する(並列にしない)。既存は skip。各リクエストの後に delayMs 待つ。
 * @returns {{fetched:string[], skipped:string[], failed:{key:string,reason:string}[], planned:string[]}}
 */
export async function fetchImages(targets, opts) {
  const { srcDir, baseUrl = DEFAULT_BASE_URL, delayMs = DEFAULT_DELAY_MS, limit = Infinity, dryRun = false, fetchImpl = fetch, sleep = defaultSleep } = opts
  const result = { fetched: [], skipped: [], failed: [], planned: [] }
  const todo = []
  for (const t of targets) {
    if (await exists(join(srcDir, `${t.key}.png`))) result.skipped.push(t.key)
    else if (todo.length < limit) todo.push(t)
  }
  if (dryRun) {
    result.planned = todo.map((t) => `${t.key} ${candidateUrls(baseUrl, t.spriteId)[0]}`)
    return result
  }
  if (todo.length > 0) await mkdir(srcDir, { recursive: true })
  for (const t of todo) {
    let outcome = { error: 'not_found' }
    for (const url of candidateUrls(baseUrl, t.spriteId)) {
      const r = await getPng(url, { fetchImpl, sleep, delayMs })
      await sleep(delayMs)
      if (r.buf) {
        outcome = r
        break
      }
      if (r.error) outcome = r // 404 だけなら次の入手元へ。他のエラーも次を試し、最後の理由を残す
    }
    if (outcome.buf) {
      const tmp = join(srcDir, `.${t.key}.png.tmp`)
      await writeFile(tmp, outcome.buf)
      await rename(tmp, join(srcDir, `${t.key}.png`))
      result.fetched.push(t.key)
    } else {
      result.failed.push({ key: t.key, reason: outcome.error ?? 'not_found' })
    }
  }
  result.fetched.sort()
  result.failed.sort((a, b) => (a.key < b.key ? -1 : 1))
  return result
}

async function readJson(path, hint) {
  try {
    return JSON.parse(await readFile(path, 'utf8'))
  } catch (err) {
    throw new Error(`${path} を読めない(${err.code ?? err.message})。${hint}`)
  }
}

export async function loadInputs(env = process.env) {
  const generated = join(repoRoot, 'data', 'generated')
  const keysFile = env.ASSETS_KEYS_FILE || join(generated, 'readmodel', 'pokemon-types.json')
  let snapshot = env.ASSETS_SHOWDOWN_SNAPSHOT
  if (!snapshot) {
    const cfg = await readJson(join(repoRoot, 'data', 'importer', 'config.json'), '')
    snapshot = join(generated, 'showdown', cfg.sources.showdown, 'snapshot.json')
  }
  const hint = '先にマスタの取得と取り込み(docs/runbooks/data.md)を済ませる'
  const keys = (await readJson(keysFile, hint)).pokemon.map((p) => p.pokemonId)
  const species = (await readJson(snapshot, hint)).species
  return { keys, species }
}

function parseArgs(argv) {
  const opts = { limit: Infinity, dryRun: false }
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--dry-run') opts.dryRun = true
    else if (argv[i] === '--limit') opts.limit = Number(argv[++i])
    else throw new Error(`不明な引数: ${argv[i]}`)
  }
  if (!(opts.limit > 0)) throw new Error('--limit は 1 以上の数')
  return opts
}

async function main() {
  const args = parseArgs(process.argv.slice(2))
  const { keys, species } = await loadInputs()
  const { targets, unmapped } = buildTargets(keys, buildKeyMap(species))
  const srcDir = process.env.ASSETS_SRC || DEFAULT_SRC_DIR
  const res = await fetchImages(targets, { srcDir, baseUrl: process.env.ASSETS_SPRITES_BASE_URL || DEFAULT_BASE_URL, ...args })
  for (const line of res.planned) console.log(`dry-run: ${line}`)
  for (const f of res.failed) console.warn(`assets-fetch: 失敗 ${f.key} (${f.reason})`)
  for (const k of unmapped) console.warn(`assets-fetch: 入手元の名前を引けない ${k}`)
  console.log(`assets-fetch: 対象 ${targets.length} 件・取得 ${res.fetched.length}・既存 ${res.skipped.length}・失敗 ${res.failed.length}・名前なし ${unmapped.length}${args.dryRun ? '(dry-run: 取得していない)' : ''}`)
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  main().catch((err) => {
    console.error(`assets-fetch: ${err.message}`)
    process.exit(1)
  })
}
