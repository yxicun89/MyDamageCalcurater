// 画像取得ツール(ADR-0810)のテスト。`node --test tools/assets/fetch.test.mjs`。
// 架空の種族データと偽の fetch を使い、ネットワークに出ない。

import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { after, describe, it } from 'node:test'

import { DEFAULT_GITHUB_BASE_URL, GITHUB_SETS, SPRITE_SETS, USER_AGENT, buildKeyMap, buildTargets, candidateUrls, fetchImages, flatten, githubId, spriteId } from './fetch.mjs'

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
    assert.equal(m.get('0007-000').spriteId, 'foo')
    assert.equal(m.get('0007-001').spriteId, 'foo-megax')
    assert.equal(m.get('0007-002').spriteId, 'foo-alola')
    assert.equal(m.get('0012-000').spriteId, 'mrbar')
    assert.equal(m.has('0099-000'), false)
  })
  it('対象は昇順・重複なし。引けない key は unmapped', () => {
    const { targets, unmapped } = buildTargets(['0012-000', '0007-001', '0007-001', '0500-000'], buildKeyMap(species))
    assert.deepEqual(targets.map((t) => t.key), ['0007-001', '0012-000'])
    assert.deepEqual(unmapped, ['0500-000'])
  })
  it('smogon/sprites のファイル名: 語の間は _、端の記号は消す、フォームは -o', () => {
    assert.equal(flatten('Mr. Bar'), 'mr_bar')
    assert.equal(flatten('Mega-X'), 'mega_x')
    assert.equal(flatten('Farfetch’d'), 'farfetchd')
    assert.equal(flatten('Flabébé'), 'flabebe')
    assert.equal(githubId(species[1]), 'sfoo-omega_x')
    assert.equal(githubId(species[3]), 'smr_bar')
    assert.equal(githubId(species[0]), 'sfoo')
  })
  it('候補 URL は GitHub raw(champions → dex)→ Showdown(home → gen5)。末尾スラッシュを許す', () => {
    const urls = candidateUrls({ github: 'https://g.test/src/', showdown: 'https://x.test/s/' }, { spriteId: 'foo', githubId: 'sfoo' })
    const [g0, g1] = GITHUB_SETS
    const [p0, p1] = SPRITE_SETS
    assert.deepEqual(urls, [`https://g.test/src/${g0}/sfoo.png`, `https://g.test/src/${g1}/sfoo.png`, `https://x.test/s/${p0}/foo.png`, `https://x.test/s/${p1}/foo.png`])
  })
})

describe('fetchImages', () => {
  const targets = [
    { key: '0007-000', spriteId: 'foo', githubId: 'sfoo' },
    { key: '0007-001', spriteId: 'foo-megax', githubId: 'sfoo-omega_x' },
    { key: '0012-000', spriteId: 'mrbar', githubId: 'smr_bar' },
  ]
  const [GH0] = GITHUB_SETS

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

  it('主が 404 なら次の入手元を試す。全部 404 は not_found で報告し、他の key は続ける', async () => {
    const dir = join(tmp, 'b')
    const fetchImpl = async (url) => {
      if (url.includes('mr') && url.includes('bar')) return res(404)
      if (url.includes(`/${GH0}/`) && url.includes('omega_x')) return res(404)
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

  it('503 のあと別の入手元が 404 でも、失敗の理由は 503 のまま(not_found で上書きしない)', async () => {
    const dir = join(tmp, 'f')
    const fetchImpl = async (url) => (url.includes(`/${GH0}/`) ? res(503) : res(404))
    const r = await fetchImages(targets.slice(0, 1), { srcDir: dir, fetchImpl, ...fast })
    assert.deepEqual(r.failed, [{ key: '0007-000', reason: 'http_503' }])
  })

  it('--limit は未取得のうち先頭 N 件。dry-run は取得せず計画だけ', async () => {
    const dir = join(tmp, 'd')
    mkdirSync(dir)
    writeFileSync(join(dir, '0007-000.png'), PNG)
    let calls = 0
    const fetchImpl = async () => (calls++, res(200))
    const dry = await fetchImages(targets, { srcDir: dir, limit: 1, dryRun: true, fetchImpl, ...fast })
    assert.equal(calls, 0)
    assert.deepEqual(dry.planned, [`0007-001 ${DEFAULT_GITHUB_BASE_URL}/${GH0}/sfoo-omega_x.png`])
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
