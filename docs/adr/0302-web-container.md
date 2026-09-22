# ADR-0302: Web をコンテナで配信する(nginx・k3d・gateway の後ろ)

- 状態: 採用(Web レーン、2026-09-22。P4-11。ユーザー決定: ローカルもコンテナで動かす・Web は gateway の後ろ)
- 関連: plan.md P4-11、ADR-0300 §1(URL で画面を切り替える = SPA のフォールバックが要る)、ADR-0301、ADR-0011 §6・§11(engine.wasm)、
  ADR-0202・ADR-0203(gateway・k3d の apply の分離)、DECISIONS.md 2026-09-22「Web の画面・コンテナ化・手順書の方針」

## 決定

1. **イメージ** `web/Dockerfile`(ビルドの context はリポジトリ直下): golang で engine.wasm と wasm_exec.js をイメージの中で作り、
   node で `vite build`、最終段は `nginxinc/nginx-unprivileged`(非 root の UID 101・8080 で待ち受け)。3段とも版と digest で固定する。
   ホストの `make wasm` に頼らないので、どの環境でも同じ成果物になる。
2. **nginx の配信規則** `web/nginx.conf`:
   - 未知のパスは `index.html` を返す(SPA のフォールバック。`/calc`・`/reverse` を直接開ける)。`/assets/` の下だけはフォールバックせず 404。
   - `.wasm` は `application/wasm`(`WebAssembly.instantiateStreaming` の条件)。gzip は wasm にも掛ける(4.6MB → 約1.3MB)。
   - ハッシュ付きの `/assets/*` は長期キャッシュ(immutable)。`index.html`・`engine.wasm`・`wasm_exec.js` はファイル名が変わらないので `no-cache`(毎回再検証)。
   - `/api` は配信しない(404)。API は gateway の担当で、nginx に proxy を持たない(経路を1本にする)。`/healthz` は probe 用。
3. **k8s**: `deploy/k8s/base/web`(Deployment と Service `web`:80 → 8080。非 root・読み取り専用のルート・/tmp は emptyDir)。
   ローカルは Component `overlays/local/web`(イメージのタグを local に)と、Web だけを apply する `overlays/local-web`(API レーンの local-api と同じ考え方。
   共有の k3d クラスタで他レーンのリソースに触らない)。`make web-k3d-deploy` がイメージの取り込み・apply・rollout を行う。
4. **入口**: gateway の後ろに置く(localhost:8080 だけで画面も API も使う)。gateway が `/api`・`/assets`・`/healthz` 以外を Service `web` に転送する
   `GATEWAY_WEB_URL` は API レーンが実装する。それまでは `make web-k3d-open`(port-forward で localhost:5173)で開き、オフラインで計算する。
5. **クラウド**: base に web を足したので、cloud overlay(`../../base` を読む)にも web が載る。クラウドへのデプロイは +α フェーズで、
   その時点で入口(gateway)の設定と合わせて見直す。

## 検証

- `make web-e2e-container`: ビルドしたイメージに対して、オフラインの E2E 一式と配信規則の HTTP テスト(`e2e/container.spec.ts`)。
- `make web-k3d-smoke`: k3d 上の Web に、`/healthz`・`/`・`/reverse`・`/engine.wasm`(MIME)・`/wasm_exec.js` を確かめる。
- `web/src/deploy/manifests.test.ts`(`make test`): digest の固定・セキュリティの設定・probe・Service・nginx の規則を文字列で確かめる(kubectl・docker 不要)。

## 却下

- **Web の nginx から /api を gateway に proxy する**: 入口が2つになり、gateway の後ろに置く決定と重なる。port-forward の間はオフラインで使う。
- **ホストの `make wasm` の成果物をイメージにコピーする**: `.dockerignore` が `*.wasm` を除外しており、ホストの Go の版に依存する。
