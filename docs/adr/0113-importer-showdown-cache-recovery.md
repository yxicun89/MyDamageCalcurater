# ADR-0113: Showdown 取得キャッシュを中断から自己回復させる(issue #102)

- 状態: 採用
- 日付: 2026-09-24
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #102、ADR-0101 §3、ADR-0104(CronJob・PVC キャッシュ)、ADR-0109(排他)

## 背景

`tools/importer/fetch-showdown.mjs` は `source.tar.gz` の存在と `src/` の存在だけで「取得・展開・build 済み」と
判断していた。展開・`npm ci`・build の途中で Pod が終了すると不完全な `src/` が PVC に残り、再試行も次週の
実行も `dist/sim/dex.js` の import で失敗し続け、人が PVC を掃除するまで復旧しなかった。

## 決定

キャッシュ確保を `tools/importer/showdown-cache.mjs`(`ensureShowdownSource`)へ切り出した。

1. tarball は一時名(`*.partial-<乱数>`)へ書いて rename する。`meta.json` に記録した sha256 と内容が
   一致しなければ不完全とみなし、tarball・meta・展開先をまとめて作り直す。meta は tarball の後に公開する
   (間で中断すれば meta 欠損 = 次回作り直し)。
2. 展開・`npm ci`・build は `src.partial-<乱数>/` で行い、`dist/sim/dex.js` を確認してから `src/` へ rename する。
   失敗時は一時ディレクトリを消す。`src/` が在っても `dist/sim/dex.js` が無ければ不完全として作り直す。
3. 起動時に前回の `*.partial-*` 残骸を消す(同時実行は ADR-0109 の flock で排除済み)。
4. 正常な完成キャッシュは取得元へ再アクセスしない(従来どおり)。

副作用のある手順(download/extract/install/build)は依存として注入し、隔離した一時ディレクトリで
`showdown-cache.test.mjs`(空の展開先・build 途中・壊れた tarball・meta 欠損・build 失敗・正常再利用)を検証する。
`make test-tools` から実行する。sha256 は自己整合の検査で、上流の正しさは commit 固定(config.json)が担う。
