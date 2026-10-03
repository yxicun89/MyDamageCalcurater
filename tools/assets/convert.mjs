// 手元のポケモン画像を WebP 2サイズ(thumb 長辺128・detail 長辺512)と manifest.json に変換する(ADR-0807)。
//
//   node tools/assets/convert.mjs        環境変数 ASSETS_SRC(入力)・ASSETS_OUT(出力)で場所を変えられる
//
// 入力は `{図鑑番号4桁}-{フォルム3桁}.{png|jpg|jpeg|webp}`。形式違反・壊れた画像は警告してスキップし、
// 残りを処理する(終了コード 0)。入力が無くても成功し、空の manifest を書く。出力は決定的(時刻・絶対パスなし)。

import { createHash } from 'node:crypto'
import { mkdir, readdir, readFile, rename, rm, writeFile } from 'node:fs/promises'
import { dirname, extname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

import sharp from 'sharp'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = join(here, '..', '..')

export const DEFAULT_SRC_DIR = join(repoRoot, 'data', 'generated', 'images', 'src')
export const DEFAULT_OUT_DIR = join(repoRoot, 'data', 'generated', 'images', 'dist')

export const THUMB_MAX_BYTES = 20 * 1024
export const DETAIL_MAX_BYTES = 100 * 1024

const SIZES = [
  { name: 'thumb', edge: 128, maxBytes: THUMB_MAX_BYTES },
  { name: 'detail', edge: 512, maxBytes: DETAIL_MAX_BYTES },
]
const KEY_PATTERN = /^\d{4}-\d{3}$/
const INPUT_EXTS = new Set(['.png', '.jpg', '.jpeg', '.webp'])
// 容量上限に収まるまで品質を下げて再エンコードする(上から順に試す)。
const QUALITIES = [85, 75, 65, 55, 45, 35, 25, 15]

/** 長辺 edge に収め(拡大しない・縦横比を保つ)、maxBytes 以下になる最高品質の WebP を返す。収まらなければ null。 */
async function encode(input, edge, maxBytes) {
  for (const quality of QUALITIES) {
    const out = await sharp(input)
      .rotate()
      .resize({ width: edge, height: edge, fit: 'inside', withoutEnlargement: true })
      .webp({ quality, effort: 4 })
      .toBuffer()
    if (out.length <= maxBytes) return out
  }
  return null
}

const hash8 = (buf) => createHash('sha256').update(buf).digest('hex').slice(0, 8)

async function listInputs(srcDir) {
  try {
    const entries = await readdir(srcDir, { withFileTypes: true })
    return entries.filter((e) => e.isFile()).map((e) => e.name).sort()
  } catch (err) {
    if (err.code === 'ENOENT') return []
    throw err
  }
}

/** thumb/・detail/ のうち、今回の manifest が参照しないファイルを消す(入力から消した画像・古い hash)。 */
async function removeStale(outDir, keep) {
  for (const { name } of SIZES) {
    let files = []
    try {
      files = await readdir(join(outDir, name))
    } catch (err) {
      if (err.code !== 'ENOENT') throw err
    }
    for (const f of files) {
      if (!keep.has(`${name}/${f}`)) await rm(join(outDir, name, f), { recursive: true, force: true })
    }
  }
}

/**
 * srcDir の画像を outDir へ変換する。入力は変更しない。
 * @returns {Promise<{converted: string[], skipped: {file: string, reason: string}[]}>}
 */
export async function convertImages({ srcDir = DEFAULT_SRC_DIR, outDir = DEFAULT_OUT_DIR, log = console.log } = {}) {
  const converted = []
  const skipped = []
  const images = {}
  const outputs = new Map() // 相対パス → バイト列

  const skip = (file, reason) => {
    skipped.push({ file, reason })
    log(`警告: ${file} をスキップした (${reason})`)
  }

  for (const file of await listInputs(srcDir)) {
    const ext = extname(file)
    if (!INPUT_EXTS.has(ext.toLowerCase())) continue
    const key = file.slice(0, -ext.length)
    if (!KEY_PATTERN.test(key)) {
      skip(file, 'invalid_key')
      continue
    }
    try {
      const input = await readFile(join(srcDir, file))
      if (input.length === 0) throw new Error('empty')
      const entry = {}
      const pending = []
      for (const { name, edge, maxBytes } of SIZES) {
        const buf = await encode(input, edge, maxBytes)
        if (!buf) throw new Error(`${name} が容量上限に収まらない`)
        const rel = `${name}/${key}.${hash8(buf)}.webp`
        entry[name] = rel
        pending.push([rel, buf])
      }
      for (const [rel, buf] of pending) outputs.set(rel, buf)
      images[key] = entry
      converted.push(key)
    } catch {
      skip(file, 'invalid_image')
    }
  }

  const sorted = Object.fromEntries(Object.keys(images).sort().map((k) => [k, images[k]]))
  const manifest = `${JSON.stringify({ version: 1, images: sorted }, null, 2)}\n`

  await mkdir(outDir, { recursive: true })
  await removeStale(outDir, new Set(outputs.keys()))
  for (const [rel, buf] of outputs) {
    await mkdir(join(outDir, dirname(rel)), { recursive: true })
    await writeFile(join(outDir, rel), buf)
  }
  // manifest は最後に原子的に置く(途中で止まっても、参照先が揃っている旧 manifest か新 manifest のどちらか)。
  const tmp = join(outDir, 'manifest.json.tmp')
  await writeFile(tmp, manifest)
  await rename(tmp, join(outDir, 'manifest.json'))

  converted.sort()
  return { converted, skipped }
}

async function main() {
  const srcDir = process.env.ASSETS_SRC || DEFAULT_SRC_DIR
  const outDir = process.env.ASSETS_OUT || DEFAULT_OUT_DIR
  const { converted, skipped } = await convertImages({ srcDir, outDir })
  console.log(`assets: 変換 ${converted.length} 件・スキップ ${skipped.length} 件 (入力 ${srcDir} → 出力 ${outDir})`)
  for (const s of skipped) console.log(`  skipped: ${s.file} (${s.reason})`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((err) => {
    console.error(err)
    process.exit(1)
  })
}
