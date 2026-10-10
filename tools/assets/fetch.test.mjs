// 画像取得ツール(ADR-0810)のテスト。`node --test tools/assets/fetch.test.mjs`。
// 架空の種族データと偽の fetch を使い、ネットワークに出ない。

import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { after, describe, it } from 'node:test'

import { DEFAULT_BASE_URL, USER_AGENT, buildKeyMap, buildTargets, candidateUrls, fetchImages, spriteId } from './fetch.mjs'

const tmp = mkdtempSync(join(tmpdir(), 'assets-fetch-'))
after(() => rmSync(tmp, { recursive: true, force: true }))

const PNG = Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), Buffer.from('fake')])
const species = [
  { name: 'Foo', baseSpecies: 'Foo', forme: '', num: 7, formeOrder: ['Foo', 'Foo-Mega-X', 'Foo-Alola'] },
  { name: 'Foo-Mega-X', baseSpecies: 'Foo', forme: 'Mega-X', num: 7 },
  { name: 'Foo-Alola', baseSpecies: 'Foo', forme: 'Alola', num: 7 },
  { name: 'Mr. Bar', baseSpecies: 'Mr. Bar', forme: '', num: 12 },
  { name: 'Orphan', baseSpecies: 'Missing', forme: 'X', num: 99 },
]

const res = (status, body = PNG) => ({ status, ok: status >= 200 && status < 300, arrayBuffer: async () => body })
const fast = { delayMs: 0, sleep: async () => {} }

describe('キーと URL の組み立て', () => {
  it('spriteId は基本種 id + フォーム id', () => {
    assert.equal(spriteId(species[1]), 'foo-megax')
    assert.equal(spriteId(species[3]), 'mrbar')
  })
  it('key は図鑑番号4桁-formeOrder の位置3桁。基本種の省略は 000', () => {
    const m = buildKeyMap(species)
    assert.equal(m.get('0007-000'), 'foo')
    assert.equal(m.get('0007-001'), 'foo-megax')
    assert.equal(m.get('0007-002'), 'foo-alola')
    assert.equal(m.get('0012-000'), 'mrbar')
    assert.equal(m.has('0099-000'), false)
  })
  it('対象は昇順・重複なし。引けない key は unmapped', () => {
    const { targets, unmapped } = buildTargets(['0012-000', '0007-001', '0007-001', '0500-000'], buildKeyMap(species))
    assert.deepEqual(targets.map((t) => t.key), ['0007-001', '0012-000'])
    assert.deepEqual(unmapped, ['0500-000'])
  })
  it('候補 URL は home → gen5(末尾スラッシュを許す)', () => {
    assert.deepEqual(candidateUrls('https://x.test/s/', 'foo'), ['https://x.test/s/home/foo.png', 'https://x.test/s/gen5/foo.png'])
  })
})

describe('fetchImages', () => {
  const targets = [{ key: '0007-000', spriteId: 'foo' }, { key: '0007-001', spriteId: 'foo-megax' }, { key: '0012-000', spriteId: 'mrbar' }]

  it('取得して {key}.png に書く・既存は取り直さない(冪等)・User-Agent を付ける', async () => {
    const dir = join(tmp, 'a')
    const urls = []
    let ua
    const fetchImpl = async (url, init) => {
      urls.push(url)
      ua = init.headers['User-Agent']
      return res(200)
    }
    const r1 = await fetchImages(targets, { srcDir: dir, fetchImpl, ...fast })
    assert.deepEqual(r1.fetched, ['0007-000', '0007-001', '0012-000'])
    assert.equal(ua, USER_AGENT)
    assert.deepEqual(readdirSync(dir).sort(), ['0007-000.png', '0007-001.png', '0012-000.png'])
    assert.deepEqual(readFileSync(join(dir, '0007-000.png')), PNG)
    const n = urls.length
    const r2 = await fetchImages(targets, { srcDir: dir, fetchImpl, ...fast })
    assert.deepEqual(r2.skipped, ['0007-000', '0007-001', '0012-000'])
    assert.equal(urls.length, n)
  })

  it('home が 404 なら gen5 を試す。両方 404 は not_found で報告し、他の key は続ける', async () => {
    const dir = join(tmp, 'b')
    const fetchImpl = async (url) => {
      if (url.includes('mrbar')) return res(404)
      if (url.includes('/home/foo-megax')) return res(404)
      return res(200)
    }
    const r = await fetchImages(targets, { srcDir: dir, fetchImpl, ...fast })
    assert.deepEqual(r.fetched, ['0007-000', '0007-001'])
    assert.deepEqual(r.failed, [{ key: '0012-000', reason: 'not_found' }])
  })

  it('PNG でない応答は失敗(ファイルを残さない)。5xx は再試行して諦める', async () => {
    const dir = join(tmp, 'c')
    let calls = 0
    const fetchImpl = async (url) => {
      calls++
      return url.includes('foo-megax') ? res(503) : res(200, Buffer.from('<html>'))
    }
    const r = await fetchImages(targets.slice(0, 2), { srcDir: dir, fetchImpl, ...fast })
    assert.deepEqual(r.fetched, [])
    assert.deepEqual(r.failed.map((f) => [f.key, f.reason]), [['0007-000', 'not_png'], ['0007-001', 'http_503']])
    assert.ok(calls > 4)
    assert.deepEqual(readdirSync(dir), [])
  })

  it('--limit は未取得のうち先頭 N 件。dry-run は取得せず計画だけ', async () => {
    const dir = join(tmp, 'd')
    mkdirSync(dir)
    writeFileSync(join(dir, '0007-000.png'), PNG)
    let calls = 0
    const fetchImpl = async () => (calls++, res(200))
    const dry = await fetchImages(targets, { srcDir: dir, limit: 1, dryRun: true, fetchImpl, ...fast })
    assert.equal(calls, 0)
    assert.deepEqual(dry.planned, [`0007-001 ${DEFAULT_BASE_URL}/home/foo-megax.png`])
    const r = await fetchImages(targets, { srcDir: dir, limit: 1, fetchImpl, ...fast })
    assert.deepEqual(r.fetched, ['0007-001'])
    assert.equal(calls, 1)
  })

  it('直列: 同時に 1 本まで・リクエストごとに delayMs 待つ', async () => {
    const dir = join(tmp, 'e')
    let active = 0
    let maxActive = 0
    const sleeps = []
    const fetchImpl = async () => {
      active++
      maxActive = Math.max(maxActive, active)
      await new Promise((r) => setImmediate(r))
      active--
      return res(200)
    }
    await fetchImages(targets, { srcDir: dir, fetchImpl, delayMs: 123, sleep: async (ms) => void sleeps.push(ms) })
    assert.equal(maxActive, 1)
    assert.equal(sleeps.filter((ms) => ms === 123).length, 3)
  })
})
