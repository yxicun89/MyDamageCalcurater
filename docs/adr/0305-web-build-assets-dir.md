# ADR-0305: Web のビルド成果物を `/static/` に出す(gateway の予約パス `/assets/` との衝突を解消)

- 状態: 採用(2026-09-25。issue #268)
- 関連: ADR-0205(gateway が `/api`・`/assets`・`/healthz`・`/internal` 以外を Web に転送)、ADR-0302(Web のコンテナ・nginx)、CLAUDE.md リポジトリ構成(gateway が `/assets` も配信)

## 背景

Vite の既定の `build.assetsDir` は `assets` で、`index.html` は `/assets/<hash>.js`・`/assets/<hash>.css` を読む。
gateway は `/assets/*` を画像配信用に予約し(ADR-0205。画像の上流は未設定なので 404)、Web に転送しない。
そのため gateway 経由(k3d の `http://localhost:8080`)では JS/CSS が 404 になり白画面になっていた(2026-09-22 の ADR-0205 導入以降)。
`make web-k3d-smoke`(Web 直の :5173)と `make api-smoke`(`/` の 200 だけを見る)はどちらもこれを検出できなかった。

## 決定

1. Web のビルド成果物は `/static/` に出す(`web/vite.config.ts` の `build.assetsDir: "static"`)。nginx のキャッシュ規則(`location ^~ /static/`)・バンドルサイズ検査・コンテナ E2E・k3d smoke を合わせて変える。
2. gateway の `/assets/`(画像)予約・CLAUDE.md の記述は変えない。
3. 回帰防止:
   - `web/src/deploy/assetsDir.test.ts`: `assetsDir` の先頭セグメントが gateway の `reservedFirstSegments`(`routing.go` から読む)に無いこと。
   - `web/scripts/k3d-smoke.sh`・`services/gateway/scripts/smoke.sh`: `index.html` が読む JS を実際に取得して 200 を確かめる(gateway 経由の確認は後者)。

## 理由

- 画像配信の接頭辞は CLAUDE.md・ADR-0205 で決まっており、gateway 側を変えると複数レーンの文書と契約に波及する。Web のビルド設定1行の方が影響が小さい。
- `/static/` は gateway の予約語と衝突せず、ADR-0205 のセグメント単位の判定でそのまま Web に届く。

## 影響

- 既に配布済みのブラウザキャッシュ: `index.html` は no-cache なので、次の読み込みから新しいパスを使う。
- 同じ衝突を別の予約語(`api`・`healthz`・`internal`)で起こさないことも 1. のテストが守る。
