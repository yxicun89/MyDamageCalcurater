# ADR-0203: calc・gateway の k3d デプロイとスモーク

- 状態: 採用・実装済み(2026-09-22。P3-3。k3d の既存クラスタで `make api-k3d-deploy && make api-smoke` を確認: calc・bulk・reverse=200、missing_header・invalid_header=400、pokedex=503、balance=200。critic 指摘を受け、apply の分離を「Secret の有無で分岐」から「常に local-api だけ」に修正、smoke.sh の 000 連結バグと dev.sh の go run 孤児プロセスを修正)
- 日付: 2026-09-22
- 関連: ADR-0002(実マスタをコミットしない)、ADR-0012(サービス境界。`/api/balance` は balance の Ingress)、
  ADR-0015(相性表 JSON)、ADR-0200(calc-svc の契約・`/healthz`)、ADR-0202(gateway のルーティング・ヘッダ検証)、
  docs/test-strategy.md L4(契約)・L5(E2E)、docs/ai-shared/COORDINATION.md(共有ファイルの規約)、plan.md P3-3

## 背景

calc-svc(P3-1)と gateway(P3-2)はプロセスとしては動くが、k3d に載せる手段(イメージ・マニフェスト・Makefile)と、
載せた後に通しで確かめる手段(スモーク)が無い。契約テスト(L4)は calc-svc 単体と gateway 経由の一部があるが、
gateway 経由の代表的な 400 と pokedex 未設定の 503 が1つの表にまとまっていない。pokedex-svc(P2-3)はまだ無い。

## 決定

### 1. 契約テスト(L4)

`services/gateway/internal/httpapi/contract_test.go` の `TestRealCalcThroughGatewayMatchesContract`(上流は calc-svc の実物。
架空マスタ)を1つの表に拡張する: calc・bulk・reverse の3操作それぞれの成功、`missing_header`・`invalid_header`(gateway)、
`unknown_field`・`unknown_species`(calc-svc。gateway は書き換えない)、`unknown_move`(既存の行)、pokedex 未設定の
`GET /api/pokedex/natures` の 503 `upstream_unavailable`。kin-openapi で `api/openapi.yaml`(生成物に埋め込まれた仕様)に照らす。
既存の行・既存のテストは残す。`make test`(`cd services && go test ./...`)に含まれる。`api/openapi.yaml` は変えない。

### 2. イメージ

`services/calc/Dockerfile` と `services/gateway/Dockerfile`。ビルドコンテキストはリポジトリ直下(services が engine を
`../engine` の replace で参照するため。`services/pokedex/Dockerfile` と同じ)。ビルド段は pokedex と同じ
`golang:1.27.1-alpine@sha256:…`(`services/go.mod` の `go` の版と一致させ、digest で固定)、`CGO_ENABLED=0`・`-trimpath`。
最終段は `scratch`、`USER 65532:65532`(数値の非 root。`runAsNonRoot` は数値 UID でないと検証できない)、`ENTRYPOINT`。
イメージ名は `pokecalc/calc:local`・`pokecalc/gateway:local`(k3d に `k3d image import` で直接入れる。レジストリを介さない)。

### 3. Kustomize

- `deploy/k8s/base/calc/`・`deploy/k8s/base/gateway/`: Deployment と Service(名前はどちらもサービス名。Service は `http:80` →
  コンテナの `http`(8080))。readiness / liveness は `GET /healthz`。resources の requests / limits(cpu・memory)。
  `automountServiceAccountToken: false`、Pod の `runAsNonRoot: true`・`seccompProfile: RuntimeDefault`、コンテナの
  `readOnlyRootFilesystem: true`・`allowPrivilegeEscalation: false`・`capabilities.drop: [ALL]`。`imagePullPolicy: IfNotPresent`。
- gateway の Ingress(`deploy/k8s/base/gateway/`): `ingressClassName: traefik`、ホスト指定なし、path `/` Prefix → Service
  `gateway`。balance の `/api/balance` は traefik の規則の長さによる優先で balance の Ingress に届く(ADR-0012)。
- base の環境変数: gateway は `GATEWAY_CALC_URL=http://calc`(Service 名。クラウドでも同じ)だけ。`GATEWAY_POKEDEX_URL` は
  pokedex-svc ができるまで未設定(→ 503)、`GATEWAY_ASSETS_URL` は未設定(→ 404)。calc は base でマスタの場所を持たない
  (local 専用の ConfigMap を base から参照すると、クラウドの overlay が壊れるため)。
- `deploy/k8s/base/kustomization.yaml`(共有)の resources に `calc` と `gateway` を1行ずつ追加するだけ。
- **local overlay 専用の Component** `deploy/k8s/overlays/local/api/`(`kind: Component`)。local overlay の
  `kustomization.yaml` に `components: [api]` を追加する(resources では base の Deployment に patch を当てられないため Component)。
  Component が持つもの:
  - `configMapGenerator`: 例のマスタ(`services/calc/testdata/master.example.json`。架空)と相性表(`testdata/golden/typechart.json`。
    数値と英語 ID のみ)のコピーを Component の直下に置いて読ませる(Kustomize の load restrictor は overlay の外を読めない)。
    元ファイルが正で、コピーとのバイト一致を Go テストで固定する(services/balance の前例)。
  - calc の Deployment への strategic merge patch(`patches[].path`): ConfigMap を読み取り専用でマウントし、
    `CALC_MASTER_PATH`・`CALC_TYPECHART_PATH` にコンテナ内の絶対パスを設定する。
  - gateway の Deployment への patch: `GATEWAY_CORS_ALLOWED_ORIGINS=http://localhost:5173`(Vite の既定)。
  - `images`: `pokecalc/calc`・`pokecalc/gateway` を `local` タグにする(base のタグが `local` なら不要)。
- マニフェストは `kubectl` 無しで静的に検査する(`services/gateway/deploytest`。gopkg.in/yaml.v3)。検査は patch の合成を
  「コンテナを name で突き合わせ、env は name で上書き・volumeMounts と volumes は追加」だけ再現するので、patch はその範囲で書く
  (JSON 6902 patch・インラインの patch は使わない)。描画できることは `make api-kustomize` で確かめる。

### 4. Makefile

`services/gateway/Makefile`(API レーンのターゲット。すべて `api-` 接頭辞)。ルートの `Makefile` には
`include services/gateway/Makefile` の1行だけを追加する(COORDINATION.md の共有ファイルの規約)。

| ターゲット | 中身 |
|---|---|
| `api-docker-build` | 2イメージを `docker build -f services/<svc>/Dockerfile -t pokecalc/<svc>:local .` |
| `api-k3d-deploy` | `api-docker-build` → `k3d image import`(2イメージ)→ `kubectl apply -k`(下の「apply の分離」)→ rollout restart / `rollout status`(calc・gateway) |
| `api-smoke` | `API_URL=… services/gateway/scripts/smoke.sh` |
| `api-kustomize` | `kubectl kustomize` で `deploy/k8s/base`・`deploy/k8s/overlays/local`・`deploy/k8s/overlays/local-api` が描画できること |

API のテストは既存の `test-services` で `make test` に入っているので、balance のような `test: api-test` の前提条件は足さない。

**apply の分離(実装時の追記。critic 指摘で修正)**: クラスタ全体の立ち上げ(namespace・mysql・pokedex-migrate Job を含む)は
`make up`(scripts/up.sh)が担う。共有の `deploy/k8s/overlays/local` を `api-k3d-deploy` からも丸ごと apply すると、
pokedex-migrate Job の再実行(`spec.template` は immutable なので既存 Job と食い違うと apply が失敗する)や mysql の
意図しない上書きを引き起こしうる(k3d クラスタは他レーンと共有)。そこで API 専用の overlay `deploy/k8s/overlays/local-api`
(`base/calc`・`base/gateway` と Component `local/api` だけ)を置き、`api-k3d-deploy` は**常に**こちらだけを適用する
(`mysql-auth` Secret の有無で分岐しない。他レーンのリソースには一切触らない)。全体のデプロイと API だけのデプロイを
両立できるよう `api-kustomize` は `deploy/k8s/base`・`deploy/k8s/overlays/local`・`deploy/k8s/overlays/local-api` の
3つとも描画できることを確かめる(apply はしない。描画は副作用が無い)。

### 5. スモーク(L5 の API 部分)

`services/gateway/scripts/smoke.sh`(POSIX sh、`set -eu`、curl だけ。jq は使わない。レスポンスは1行の JSON なので grep / sed):

- `API_URL`(既定 `http://localhost:8080`)。gateway 経由で `POST /api/calc`(例のマスタの ID、UUID の端末ID・セッションID)→ 200・
  `rolls` が16個・`category` あり、`/api/calc/bulk` → 200・`rows` が1行以上、`/api/calc/reverse` → 200・`candidates` が1件以上。
- ヘッダ無し → 400 `missing_header`、セッションID が UUID でない → 400 `invalid_header`。
- `GET /api/pokedex/natures` → 503 `upstream_unavailable`。**pokedex-svc を deploy/k8s に入れたら 200 の確認に変え**、
  `cmd/gateway` の `TestManifestGatewayLocalConfig`(`GATEWAY_POKEDEX_URL` 未設定の検査)も一緒に変える。
- `/api/balance/healthz` が balance に届くこと(gateway の `/` が奪わない)は任意。`API_SMOKE_BALANCE=auto`(既定。kubectl で
  `balance` の Ingress があるときだけ見る)/ `on` / `off`。
- ロールアウト直後の 000 / 404 / 502 / 503 は最初の1件だけ `API_SMOKE_RETRIES` 回(既定 30、1秒間隔)再試行する。
  接続拒否は curl 自身が `000` を書き出す(`-w '%{http_code}'`)ので、`status=$(curl ... || printf '000')` のように
  失敗時にさらに `000` を連結してはいけない(`000000` になり `case` の判定から漏れて再試行されなくなる。critic 指摘で修正。
  `status=$(curl ...) || true` で受けてから空なら `000` を補う)。
  **ADR-0205 で更新**: 再試行の対象は `000` / `502`(Traefik がまだ終了中の Pod に振り分けて返す状態)だけにし、
  `404` / `503` は pokedex 未設定・Web 未デプロイ等の意味のある最終状態でもありうるので再試行しない。また
  最初の1件(`POST /api/calc`)だけでなく、スクリプト内のすべてのリクエストに再試行を適用する。
- 失敗したら内容・ステータス・本文を出して非ゼロで終わる。
- スクリプト自体は Go テスト(`services/gateway/deploytest/smoke_test.go`)で、同じプロセスに起動した calc-svc の実物と gateway に
  向けて流し、正しい構成で成功・calc に届かない構成と pokedex が答える構成で失敗することを確かめる(確認の空振りを防ぐ)。
  curl が無い環境ではこの3件だけ skip する。

### 6. `scripts/dev.sh`(P3 の占位を API レーンが実装する)

`scripts/` は共有だが、`dev.sh` は「P3 で calc-svc / gateway のローカル起動を実装」と書かれた占位なので API レーンが実装する。
calc-svc(例のマスタ・`testdata/golden/typechart.json`)と gateway(`GATEWAY_CALC_URL=http://127.0.0.1:<calc のポート>`、
CORS は `http://localhost:5173`)を `go run` で起動し、Ctrl-C(`trap`)で両方を止める。ポートは `DEV_CALC_PORT`(既定 8081)・
`DEV_GATEWAY_PORT`(既定 8080。k3d の loadbalancer と同じなので、クラスタを動かしている間は変える)。`make dev` から呼ばれる。
確認は `make dev` の後に `API_URL=http://localhost:<DEV_GATEWAY_PORT> API_SMOKE_BALANCE=off services/gateway/scripts/smoke.sh`。

### 7. `scripts/e2e.sh` は変えない

P4-6(Web レーン)の占位。k3d のスモークは `make api-smoke`、`e2e.sh` は P4-6 で Playwright と合わせて `api-smoke` を呼ぶ。

## 受け入れ条件

| AC | 内容 | 担当テスト / 確認 |
|---|---|---|
| AC-S1 | gateway 経由の calc・bulk・reverse の成功、missing_header・invalid_header・unknown_field・unknown_species(と unknown_move)、pokedex 未設定の 503 が api/openapi.yaml に合う(1つの表) | `gateway/internal/httpapi.TestRealCalcThroughGatewayMatchesContract` |
| AC-S2 | Dockerfile: コンテキストはリポジトリ直下、`golang:<services/go.mod の版>-alpine@sha256:<64桁>`、`CGO_ENABLED=0`、最終段 scratch、数値の非 root USER、ENTRYPOINT | `gateway/deploytest.TestAPIDockerfiles` |
| AC-S3 | base の Deployment / Service(名前・ラベル・イメージ名・ポート http・`/healthz` の probe・resources・非 root・readOnlyRootFilesystem・drop ALL・seccomp・Service 80)、待ち受けアドレスが containerPort と一致、gateway の Ingress(traefik・`/` Prefix・ホストなし → gateway)、base の resources に calc・gateway が1行ずつ | `cmd/calc.TestManifestCalcWorkload` / `cmd/gateway.TestManifestGatewayWorkload` / `TestManifestGatewayIngress` / `deploytest.TestBaseKustomizationListsAPIServices` |
| AC-S4 | local overlay は Component `api` を読む。calc のマスタと相性表は Component の直下のコピーから configMapGenerator で作った ConfigMap の読み取り専用マウントから来て、コピーは元ファイルとバイト一致し、そのまま起動できる。base はそれを参照しない。gateway は base で起動でき、local では calc=http://calc・pokedex/assets 未設定・CORS は `http://localhost:5173` だけ。イメージは `<repo>:local`。`deploy/k8s/overlays/local-api` は namespace `pokecalc`、resources は `../../base/calc`・`../../base/gateway` だけ、components は `../local/api` だけ(他レーンの resources を持たない) | `cmd/calc.TestManifestCalcBaseHasNoLocalData` / `TestManifestCalcLocalDataFromOverlayCopies` / `TestManifestCalcLocalImage` / `cmd/gateway.TestManifestGatewayBaseConfig` / `TestManifestGatewayLocalConfig` / `TestManifestGatewayLocalImage` / `deploytest.TestLocalOverlayUsesAPIComponent` / `TestLocalAPIOverlayIsScopedToAPIServices` |
| AC-S5 | `services/gateway/Makefile` に `api-docker-build`・`api-k3d-deploy`・`api-smoke`・`api-kustomize`(.PHONY、`api-` 接頭辞のみ)。`api-k3d-deploy` は `deploy/k8s/overlays/local-api` だけを apply し、共有の `deploy/k8s/overlays/local` は apply しない。ルートの Makefile は `include services/gateway/Makefile` の1行。`make api-kustomize` が成功する | `deploytest.TestAPIMakefile` + 手動 `make api-kustomize` |
| AC-S6 | smoke.sh は POSIX sh・`set -eu`・実行可能。正しい構成で成功し、壊れた構成で非ゼロ。接続拒否(ロールアウト直後)は `000` として再試行され、二重に連結されない。`000`/`502` はスクリプト内のすべてのリクエストで再試行し、それ以外(パスごとに1回だけ 502 を返す前段でも)は成功、502 が続く前段では失敗する。全体のデプロイは `make up`、API レーンは自分の2つの Deployment(calc・gateway)だけを `deploy/k8s/overlays/local-api` で apply する(`make api-k3d-deploy && make api-smoke` が成功する) | `deploytest.TestSmokeScriptIsPOSIXShell` / `TestSmokeScriptPassesAgainstGatewayAndCalc` / `TestSmokeScriptFailsOnBrokenStack` / `TestSmokeScriptRetriesThroughGatewayNotYetListening` / `TestSmokeScriptRetriesThroughTransientBadGateway` / `TestSmokeScriptFailsOnPersistentBadGateway` + 手動(k3d) |
| AC-S7 | `scripts/dev.sh` が calc-svc と gateway を例のマスタで起動し、Ctrl-C・SIGTERM で両方止まる(`go run` は子プロセスへシグナルを転送しないため、先に `go build` した実バイナリを直接起動する)。`make dev` の上で smoke.sh が成功する | `deploytest.TestDevScript`(静的)+ 手動 |
| AC-S8 | `make test`・`make lint`(check-publishable を含む)が成功。`api/openapi.yaml`・`scripts/e2e.sh`・他レーンの範囲は変えない | `make test` / `make lint` / `git diff --stat` |

## 結果

- k3d 上で calc と gateway が動き、`make api-smoke` で通しの確認ができる。pokedex-svc が入ったときに変える場所
  (smoke.sh の 503・`TestManifestGatewayLocalConfig`)が明示される。
- マニフェストの静的検査は Kustomize の描画を完全には再現しない(patch の書き方を §3 の範囲に限る)。描画そのものは
  `make api-kustomize` と k3d での apply が確かめる。
- 例のマスタと相性表のコピーが Component の下に増える(バイト一致をテストで固定するので、元ファイルを変えたらコピーし直す)。
