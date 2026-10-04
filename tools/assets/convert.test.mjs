// P8-1(ADR-0807)の画像変換ツールのテスト。`node --test tools/assets/convert.test.mjs`。
//
// 入力はユーザーが手元に置く画像フォルダ。ここでは Docker・外部ネットワーク・実画像なしで走るよう、
// 小さな架空の PNG をテスト内で生成する(zlib だけで作る。画像ライブラリに依存しない)。
// 実装前は ./convert.mjs が無いので全件失敗する。

import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, relative } from 'node:path'
import { after, describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { deflateSync } from 'node:zlib'

import { DEFAULT_OUT_DIR, DEFAULT_SRC_DIR, DETAIL_MAX_BYTES, THUMB_MAX_BYTES, convertImages } from './convert.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = join(here, '..', '..')
const cli = join(here, 'convert.mjs')

// ---- 架空の PNG ----------------------------------------------------------------------------

const crcTable = Array.from({ length: 256 }, (_, n) => {
  let c = n
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
  return c >>> 0
})
function crc32(buf) {
  let c = 0xffffffff
  for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8)
  return (c ^ 0xffffffff) >>> 0
}
function chunk(type, data) {
  const len = Buffer.alloc(4)
  len.writeUInt32BE(data.length)
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data])
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(body))
  return Buffer.concat([len, body, crc])
}
/** なめらかなグラデーションの RGBA PNG(圧縮しやすい = 容量上限の検査に使える)。seed で色を変える。 */
function fakePng(width, height, seed = 0) {
  const raw = Buffer.alloc((width * 4 + 1) * height)
  for (let y = 0; y < height; y++) {
    const row = y * (width * 4 + 1)
    raw[row] = 0
    for (let x = 0; x < width; x++) {
      const o = row + 1 + x * 4
      raw[o] = (x * 255) / width
      raw[o + 1] = (y * 255) / height
      raw[o + 2] = (seed * 37) % 256
      raw[o + 3] = 255
    }
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(width, 0)
  ihdr.writeUInt32BE(height, 4)
  ihdr[8] = 8 // 8bit
  ihdr[9] = 6 // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

// ---- WebP の最小の読み取り(RIFF/WEBP の確認と幅・高さ) ----------------------------------------

function webpSize(buf) {
  assert.equal(buf.subarray(0, 4).toString('ascii'), 'RIFF', 'RIFF で始まる')
  assert.equal(buf.subarray(8, 12).toString('ascii'), 'WEBP', 'WEBP 形式')
  const fourcc = buf.subarray(12, 16).toString('ascii')
  if (fourcc === 'VP8X') return { width: 1 + buf.readUIntLE(24, 3), height: 1 + buf.readUIntLE(27, 3) }
  if (fourcc === 'VP8L') {
    const bits = buf.readUInt32LE(21)
    return { width: 1 + (bits & 0x3fff), height: 1 + ((bits >> 14) & 0x3fff) }
  }
  assert.equal(fourcc, 'VP8 ', '未知の WebP チャンク')
  return { width: buf.readUInt16LE(26) & 0x3fff, height: buf.readUInt16LE(28) & 0x3fff }
}

// ---- 作業ディレクトリ ----------------------------------------------------------------------

const made = []
function workdir() {
  const root = mkdtempSync(join(tmpdir(), 'assets-test-'))
  made.push(root)
  const src = join(root, 'src')
  mkdirSync(src)
  return { root, src, out: join(root, 'out') }
}
after(() => made.forEach((d) => rmSync(d, { recursive: true, force: true })))

function listFiles(dir) {
  const out = []
  const walk = (d) => {
    for (const e of readdirSync(d, { withFileTypes: true })) {
      const p = join(d, e.name)
      if (e.isDirectory()) walk(p)
      else out.push(relative(dir, p).split('\\').join('/'))
    }
  }
  walk(dir)
  return out.sort()
}
const readManifest = (out) => JSON.parse(readFileSync(join(out, 'manifest.json'), 'utf8'))
const quiet = { log: () => {} }

// ---- テスト --------------------------------------------------------------------------------

describe('convertImages: 変換(AC-A1・A2)', () => {
  it('キー名の画像を thumb(128)・detail(512)の WebP に変換し、manifest に載せる', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0445-000.png'), fakePng(1024, 1024))
    writeFileSync(join(src, '0006-001.png'), fakePng(400, 300, 1))

    const result = await convertImages({ srcDir: src, outDir: out, ...quiet })

    assert.deepEqual([...result.converted].sort(), ['0006-001', '0445-000'])
    assert.deepEqual(result.skipped, [])
    const m = readManifest(out)
    assert.equal(m.version, 1)
    assert.deepEqual(Object.keys(m.images), ['0006-001', '0445-000'], 'キーは昇順(差分が安定する)')

    for (const [key, { thumb, detail }] of Object.entries(m.images)) {
      assert.match(thumb, new RegExp(`^thumb/${key}\\.[0-9a-f]{8}\\.webp$`))
      assert.match(detail, new RegExp(`^detail/${key}\\.[0-9a-f]{8}\\.webp$`))
    }

    // 大きい画像: 長辺がちょうど 128 / 512。小さい画像: 拡大しない・縦横比を保つ。
    const big = m.images['0445-000']
    assert.deepEqual(webpSize(readFileSync(join(out, big.thumb))), { width: 128, height: 128 })
    assert.deepEqual(webpSize(readFileSync(join(out, big.detail))), { width: 512, height: 512 })
    const small = m.images['0006-001']
    assert.deepEqual(webpSize(readFileSync(join(out, small.thumb))), { width: 128, height: 96 })
    assert.deepEqual(webpSize(readFileSync(join(out, small.detail))), { width: 400, height: 300 })

    // 容量上限(thumb ≤20KB / detail ≤100KB)。
    assert.equal(THUMB_MAX_BYTES, 20 * 1024)
    assert.equal(DETAIL_MAX_BYTES, 100 * 1024)
    for (const { thumb, detail } of Object.values(m.images)) {
      assert.ok(statSync(join(out, thumb)).size <= THUMB_MAX_BYTES)
      assert.ok(statSync(join(out, detail)).size <= DETAIL_MAX_BYTES)
    }
    // 出力は manifest・thumb・detail の3種類だけ(余計なファイルを置かない)。
    const expected = ['manifest.json', ...Object.values(m.images).flatMap((v) => [v.detail, v.thumb])].sort()
    assert.deepEqual(listFiles(out), expected)
  })

  it('拡張子は png・jpg・jpeg・webp を受ける', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0001-000.png'), fakePng(64, 64))
    // jpg/webp の実バイト列は作れないので、変換不能として skipped に入ること(拡張子は拾われる)を確かめる。
    writeFileSync(join(src, '0002-000.jpg'), Buffer.from('not-a-jpeg'))
    writeFileSync(join(src, '0003-000.webp'), Buffer.from('not-a-webp'))
    const result = await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(result.converted, ['0001-000'])
    assert.deepEqual(result.skipped.map((s) => s.file).sort(), ['0002-000.jpg', '0003-000.webp'])
  })

  it('ファイル名の hash は内容で決まる: 同じ入力の再実行は byte 単位で同一、画像を変えると名前が変わる', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0445-000.png'), fakePng(300, 300, 1))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    const first = listFiles(out)
    const firstManifest = readFileSync(join(out, 'manifest.json'), 'utf8')
    const digest = (files) => createHash('sha256').update(files.map((f) => readFileSync(join(out, f))).join('|')).digest('hex')
    const firstDigest = digest(first)

    await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(listFiles(out), first)
    assert.equal(readFileSync(join(out, 'manifest.json'), 'utf8'), firstManifest, 'manifest が再実行で変わらない(時刻等を入れない)')
    assert.equal(digest(listFiles(out)), firstDigest)

    writeFileSync(join(src, '0445-000.png'), fakePng(300, 300, 2))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    const m = readManifest(out)
    assert.notEqual(JSON.stringify(m), JSON.stringify(JSON.parse(firstManifest)), '内容が違えば URL が変わる(長期キャッシュを破る)')
    const after = listFiles(out)
    assert.equal(after.filter((f) => f.startsWith('thumb/')).length, 1, '古い hash の thumb が残らない')
    assert.equal(after.filter((f) => f.startsWith('detail/')).length, 1, '古い hash の detail が残らない')
  })

  it('入力から消した画像は出力からも消える(manifest にも載らない)', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0001-000.png'), fakePng(64, 64))
    writeFileSync(join(src, '0002-000.png'), fakePng(64, 64, 1))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    rmSync(join(src, '0002-000.png'))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(Object.keys(readManifest(out).images), ['0001-000'])
    assert.ok(listFiles(out).every((f) => !f.includes('0002-000')))
  })

  it('入力ディレクトリの中身は変更しない', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0001-000.png'), fakePng(64, 64))
    const before = listFiles(src)
    const bytes = readFileSync(join(src, '0001-000.png'))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(listFiles(src), before)
    assert.deepEqual(readFileSync(join(src, '0001-000.png')), bytes)
  })

  it('manifest に絶対パス・入力ファイル名・時刻を含めない(公開しても手元の環境が出ない)', async () => {
    const { root, src, out } = workdir()
    writeFileSync(join(src, '0001-000.png'), fakePng(64, 64))
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    const text = readFileSync(join(out, 'manifest.json'), 'utf8')
    assert.ok(!text.includes(root), '絶対パスを含まない')
    assert.ok(!text.includes('.png'), '入力ファイル名を含まない')
    assert.deepEqual(Object.keys(JSON.parse(text)).sort(), ['images', 'version'])
  })
})

describe('convertImages: 画像が無い・壊れている(AC-A3・A4)', () => {
  it('入力が空でも成功し、空の manifest を書く(画像無し = 全員エンブレム)', async () => {
    const { src, out } = workdir()
    const result = await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(result.converted, [])
    assert.deepEqual(result.skipped, [])
    assert.deepEqual(readManifest(out), { version: 1, images: {} })
  })

  it('入力ディレクトリが存在しなくても成功し、空の manifest を書く', async () => {
    const { root, out } = workdir()
    const result = await convertImages({ srcDir: join(root, 'does-not-exist'), outDir: out, ...quiet })
    assert.deepEqual(result.converted, [])
    assert.deepEqual(readManifest(out), { version: 1, images: {} })
  })

  it('不正なキー名・壊れた画像・0バイトはスキップして続行し、理由を返す。正しい画像は変換される', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0445-000.png'), fakePng(200, 200))
    writeFileSync(join(src, '445-0.png'), fakePng(64, 64)) // キー形式違反
    writeFileSync(join(src, 'pikachu.png'), fakePng(64, 64)) // キー形式違反
    writeFileSync(join(src, '0025-000.png'), Buffer.from('this is not an image')) // 壊れ
    writeFileSync(join(src, '0026-000.png'), fakePng(64, 64).subarray(0, 40)) // 途中で切れた PNG
    writeFileSync(join(src, '0027-000.png'), Buffer.alloc(0)) // 0バイト
    writeFileSync(join(src, '0028-000.txt'), 'text') // 対象外の拡張子(スキップ扱いでも無視でもよいが manifest に載らない)
    mkdirSync(join(src, '0029-000')) // ディレクトリは無視

    const result = await convertImages({ srcDir: src, outDir: out, ...quiet })

    assert.deepEqual(result.converted, ['0445-000'])
    const reasons = Object.fromEntries(result.skipped.map((s) => [s.file, s.reason]))
    assert.equal(reasons['445-0.png'], 'invalid_key')
    assert.equal(reasons['pikachu.png'], 'invalid_key')
    assert.equal(reasons['0025-000.png'], 'invalid_image')
    assert.equal(reasons['0026-000.png'], 'invalid_image')
    assert.equal(reasons['0027-000.png'], 'invalid_image')
    assert.deepEqual(Object.keys(readManifest(out).images), ['0445-000'])
    // スキップした画像の出力が部分的に残らない(manifest にあるキーだけがファイルを持つ)。
    const referenced = Object.values(readManifest(out).images).flatMap((v) => [v.thumb, v.detail])
    assert.deepEqual(listFiles(out).filter((f) => f !== 'manifest.json'), referenced.sort())
  })

  it('同じキーの入力が複数(0445-000.png と .jpg)なら、どちらも duplicate_key でスキップし、出力に残さない', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0445-000.png'), fakePng(64, 64))
    writeFileSync(join(src, '0445-000.jpg'), fakePng(64, 64, 1))
    writeFileSync(join(src, '0001-000.png'), fakePng(64, 64))
    const result = await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.deepEqual(result.converted, ['0001-000'])
    const reasons = Object.fromEntries(result.skipped.map((s) => [s.file, s.reason]))
    assert.equal(reasons['0445-000.png'], 'duplicate_key')
    assert.equal(reasons['0445-000.jpg'], 'duplicate_key')
    assert.ok(listFiles(out).every((f) => !f.includes('0445-000')))
  })

  it('thumb/ 配下の利用者ディレクトリは消さない', async () => {
    const { src, out } = workdir()
    mkdirSync(join(out, 'thumb', 'mine'), { recursive: true })
    writeFileSync(join(out, 'thumb', 'mine', 'keep.txt'), 'x')
    await convertImages({ srcDir: src, outDir: out, ...quiet })
    assert.ok(listFiles(out).includes('thumb/mine/keep.txt'))
  })

  it('スキップの警告を log に出す(黙って落とさない)', async () => {
    const { src, out } = workdir()
    writeFileSync(join(src, 'bad.png'), fakePng(8, 8))
    const lines = []
    await convertImages({ srcDir: src, outDir: out, log: (l) => lines.push(String(l)) })
    assert.ok(lines.some((l) => l.includes('bad.png')), `警告に対象ファイル名が出る: ${JSON.stringify(lines)}`)
  })
})

describe('既定の入出力先は Git に追跡されない(AC-A5。ADR-0002: 公式画像・変換物をコミットしない)', () => {
  it('既定の入力・出力は data/generated/images 以下', () => {
    assert.equal(relative(repoRoot, DEFAULT_SRC_DIR), join('data', 'generated', 'images', 'src'))
    assert.equal(relative(repoRoot, DEFAULT_OUT_DIR), join('data', 'generated', 'images', 'dist'))
  })

  it('git check-ignore が既定の入力・出力のファイルを無視対象と答える', () => {
    for (const dir of [DEFAULT_SRC_DIR, DEFAULT_OUT_DIR]) {
      const r = spawnSync('git', ['check-ignore', '-q', join(relative(repoRoot, dir), 'sample.png')], { cwd: repoRoot })
      assert.equal(r.status, 0, `${dir} 以下が .gitignore されていない`)
    }
  })

  it('Git に data/generated 以下の追跡ファイルが無い', () => {
    const r = spawnSync('git', ['ls-files', 'data/generated'], { cwd: repoRoot, encoding: 'utf8' })
    assert.equal(r.stdout.trim(), '')
  })
})

describe('CLI: node tools/assets/convert.mjs(AC-A6。make assets の中身)', () => {
  const run = (env) =>
    spawnSync(process.execPath, [cli], { cwd: repoRoot, env: { ...process.env, ...env }, encoding: 'utf8' })

  it('ASSETS_SRC・ASSETS_OUT を読み、成功は終了コード 0 とサマリーを出す', () => {
    const { src, out } = workdir()
    writeFileSync(join(src, '0445-000.png'), fakePng(200, 200))
    writeFileSync(join(src, 'bad.png'), fakePng(8, 8))
    const r = run({ ASSETS_SRC: src, ASSETS_OUT: out })
    assert.equal(r.status, 0, r.stderr)
    assert.deepEqual(Object.keys(readManifest(out).images), ['0445-000'])
    assert.match(r.stdout + r.stderr, /1/, 'converted 件数が出る')
    assert.match(r.stdout + r.stderr, /bad\.png/, 'スキップを報告する')
  })

  it('入力が無くても終了コード 0(現状の「画像なし」運用で make assets が壊れない)', () => {
    const { root, out } = workdir()
    const r = run({ ASSETS_SRC: join(root, 'none'), ASSETS_OUT: out })
    assert.equal(r.status, 0, r.stderr)
    assert.deepEqual(readManifest(out), { version: 1, images: {} })
  })
})
