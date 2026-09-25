# 手順書コマンドの解説

- 対象: `README.md`・`docs/verify-m1.md`・`docs/runbooks/{api,data,balance,speed,ios,ios-device-install}.md` のコードブロックに出る**全コマンド**(重複は 1 行にまとめ、「出現」欄に出現箇所を並べる)。
- 基準: `origin/main` 3379b03 取り込み後。行番号は同時点。
- 略記: **host** = 開発機(Mac)のシェル / **k3d** = クラスタ `pokecalc`(kubectl context `k3d-pokecalc`)/ **docker** = ホストの Docker。全 make ターゲットの定義は [make-targets.md](make-targets.md)、リソースとポートは [k8s-local.md](k8s-local.md)、DB は [db-mysql.md](db-mysql.md)。
- 全手順の先頭にある `cd "$(git rev-parse --show-toplevel)"` は、host のカレントをリポジトリルート(worktree のルート)へ移すだけ(副作用なし)。Makefile の相対パスがルート前提のため。

## 1. 環境・テスト・ビルド(host のみ。クラスタ・DB に触れない)

| コマンド | 出現 | 裏で走るもの | 場所 | 副作用 / 失敗時 |
|---|---|---|---|---|
| `git status --short --branch` `git diff` `git diff --cached` `git log -8 --oneline` | README | git の読み取り | host | なし |
| `make doctor` | verify §1、README | `scripts/doctor.sh`: `git go node jq docker k3d kubectl helm` の存在を確認、`docker info` でデーモン確認。不足は `brew install …` を表示 | host | なし。NG 行が出たら導入。`tiup xcodebuild codex tailscale claude` は任意(`--`) |
| `make web-install` | verify §1 | `cd web && npm ci --no-audit --no-fund` | host | `web/node_modules` を更新(ネットワーク) |
| `make web-e2e-install` | verify §1 | `web-deps` → `npx playwright install chromium` | host | chromium を取得(ネットワーク) |
| `make test` | README、verify §2、api §1、balance/speed §1 | 前提の合成: `test-engine`(`go test ./...` in engine)・`test-services`・`test-tools`・`test-scripts`(`argocd-bootstrap_test.sh`)・`balance-test` `speed-test` `judge-test` `web-test`(vitest) | host | 生成物なし。失敗したテスト名で該当モジュールを特定 |
| `make lint` | README、verify §2 | `gofmt -l`・`go vet`(engine/services/tools)、`scripts/*.sh` と `tools/importer/*.sh` の構文検査、`node --check`、`k8s-render`、`check-publishable`(+selftest)、各レーンの lint(`speed-lint judge-lint web-lint balance-lint`) | host | `kubectl kustomize` を使う(クラスタ非接触) |
| `make build` | README、verify §2、api/balance/speed | engine・services・tools の `go build ./...`、`speed-build judge-build web-build balance-build`(web は型検査+vite build+サイズ予算) | host | `web/dist` を生成 |
| `make check-publishable` | api §1、balance/speed §1 | `scripts/check-publishable.sh`: 絶対パス・秘密・追跡禁止ファイル・第三者データを検査 | host | 違反箇所を報告 |
| `make test-golden` | README、verify §2 | `cd engine && go test -tags golden ./... -run Golden`(`testdata/golden` の @smogon/calc 期待値と全件一致) | host | 差分は `known_diffs.yaml`(ADR 付き)だけ許容 |
| `make test-all-species` | README | `go test -tags allspecies ./... -run AllSpecies` | host | — |
| `make test-wasm` | verify §2 | `wasm`(`scripts/wasm.sh`)→ `node scripts/wasm-conformance.mjs`(ネイティブ Go の期待値と engine.wasm の JSON が**バイト一致**) | host | `web/public/{engine.wasm,wasm_exec.js}` を生成。前提欠落はスキップせず失敗 |
| `make web-test-wasm` | verify §2 | `wasm` → `npm run test:wasm`(vitest.wasm.config.ts。Web が組んだリクエストを本物の wasm に通す) | host | — |
| `make web-e2e` | verify §2 | `wasm` → `npm run e2e`(Playwright + chromium。`vite preview` を `127.0.0.1:4317` で一時起動。**オフライン = WASM 計算**) | host | 一時サーバは終了で消える |
| `make web-e2e-online` | verify §2 | `wasm` → `npm run e2e:online`(vite preview `:4318` + `go run ./calc/cmd/calc` を `127.0.0.1:18317` に起動。マスタは `web/scripts/export-example-master.mjs` が出す例データ。`API_PROXY_TARGET` で `/api` を calc-svc へ) | host | gateway・k3d は使わない |
| `make web-e2e-balance` | verify §2 | `wasm` → `npm run e2e:balance`(vite preview `:4320` + `go run ./balance/cmd/api` `:18318`。`BALANCE_PROXY_TARGET`) | host | — |
| `make web-e2e-container` | verify §2 | `web-docker-build`(`docker build -f web/Dockerfile -t pokecalc/web:local .`)→ `npm run e2e:container`(`docker rm -f` → `docker run --rm --pull never --read-only --tmpfs /tmp --user 101:101 --cap-drop ALL -p 127.0.0.1:4319:8080`) | host + docker | k8s の Deployment と同じ制約で nginx を検証(本番配信の確認)。イメージ作成 |
| `(cd engine && go vet ./... && go build ./...)` ほか 2 行 | README | `make lint`/`build` を使わずモジュール単位で確認 | host | — |

## 2. k3d の作成とデプロイ(verify §3・§4・api §2〜4・data §1)

| コマンド | 出現 | 裏で走るもの | 場所 | つなぐもの / 副作用 / 失敗時 |
|---|---|---|---|---|
| `make up` | verify §3、api §2、data §1、ios-device §1 | `scripts/up.sh`。順に: ① `k3d cluster create --config deploy/k3d.yaml`(無ければ。`8080:80` を serverlb に公開)② context が `k3d-pokecalc` か検査 ③ `kubectl apply -f base/namespace.yaml` ④ Secret `mysql-auth` を無ければ乱数で作成 ⑤ `docker build --target migrate` / `server` → `k3d image import` ⑥ `kubectl delete job pokedex-migrate --ignore-not-found` ⑦ `kubectl apply -k deploy/k8s/overlays/local` ⑧ `rollout status statefulset/mysql`(180s)⑨ `wait job/pokedex-migrate --for=condition=complete`(300s)⑩ `docker build --target importer` → import ⑪ `data/local/name_ja_overrides.json` があれば ConfigMap 作成 | host → docker → k3d | **クラスタ作成+全 apply。** つなぐもの: pokedex/migrate/import → mysql:3306。calc・gateway・web の image は作らない(`:local` は次の 2 コマンドで import。[k8s-local.md §4](k8s-local.md))。失敗時: `kubectl -n pokecalc get pods,jobs`、`logs job/pokedex-migrate`、`docker ps`(OrbStack/Docker 起動) |
| `make api-k3d-deploy` | verify §4(`make deploy-latest` の中)、api §4 | `api-docker-build`(`docker build -f services/{calc,gateway}/Dockerfile -t pokecalc/{calc,gateway}:local .`)→ `k3d image import`(両 image)→ `kubectl apply -k deploy/k8s/overlays/local-api` → `rollout restart deployment/calc deployment/gateway` → `rollout status`(各 120s) | host → k3d(ns pokecalc) | calc・gateway だけを更新(他レーンの Job・mysql に触れない)。つなぐもの: gateway →(env)`http://calc`・`http://pokedex`・`http://web`、calc → `http://pokedex` |
| `make web-k3d-deploy` | verify §4(`make deploy-latest` の中) | context 検査 → `docker build -f web/Dockerfile -t pokecalc/web:local .` → `k3d image import` → `kubectl apply -k overlays/local-web` → `rollout restart deployment/web` → `rollout status`(120s) | host → k3d | web だけを更新。web Service:80 → Pod:8080 |
| `kubectl -n pokecalc get pods` | verify §3 | Pod 一覧の読み取り | host → k3d | 期待: `calc gateway web balance pokedex mysql-0` が `Running`(balance は `balance-k3d-deploy` 済みの場合) |
| `make import-fetch` | verify §3、data §2 | `cd tools/importer && npm ci && node fetch.mjs`(calc 0.12.0・Showdown・PokeAPI の**版固定**の取得) | host | `data/generated/`(Git 管理外)を書く。**外部ネットワーク** |
| `make import-dry-run` | verify §3、data §3 | `cd services && go run ./pokedex/cmd/import -data ../data -dry-run` | host | DB 非接触。最終行 `blockers: none` を確認(食い違いがあれば exit 3 でブロック報告)。ADR-0121 より前のスナップショットは `技 … に mechanism が無い` で exit 3 → `make import-fetch` で取り直す |
| `make import-k8s` | verify §3、api §3、data §4 | context 検査 → `kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-$(date +%Y%m%d%H%M%S)` | host → k3d | **Job 作成**。Job は `cronjob.sh`(flock→fetch→check-upstream→`pokedex-import`)で全置換投入(**k3d 内から外部ネットワークに出る**)。stdout に Job 名 |
| `make pokedex-export`(+ `kubectl port-forward svc/mysql 3306:3306`) | verify §3、speed §3 | `cd services && go run ./pokedex/cmd/pokedex export -out ../data/generated/readmodel`(`POKEDEX_DATABASE_DSN` は Secret `mysql-auth` の `pokedex-dsn` を `127.0.0.1` に付け替えたもの) | host → k3d(port-forward で mysql:3306) | `data/generated/readmodel/` に balance・speed 用の read model を書く |
| `make deploy-latest` | verify §4 | `scripts/k3d-deploy-latest.sh`: context 検査 → mysql へ一時 port-forward し Secret の4つの DSN で `make migrate-up`(プロビジョニング → up → importer の権限の付け直し。ADR-0125) → pokedex-importer イメージを build・import(CronJob・import-k8s が使う)→ pokedex(server)を build・import・rollout restart → `api-k3d-deploy` → `web-k3d-deploy` → `judge-k3d-deploy` → read model があれば `balance-k3d-deploy-readmodel`・`speed-k3d-deploy-readmodel`(無ければ非ゼロで終了) | host → k3d(ns pokecalc) | 7 Deployment のイメージをいまのチェックアウトの内容に入れ替える |
| `created=$(make import-k8s)` `job_name=$(echo "$created" \| grep -o 'pokedex-import-manual-[0-9]*' \| tail -1)` | api §3、data §4 | 出力から Job 名を取り出す(シェルの文字列処理) | host | — |
| `kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s` | api §3、data §4 | 完了待ち | host → k3d | 期待: `condition met`。失敗時は `kubectl logs job/$job_name`、exit code は [db-mysql.md §5](db-mysql.md) |
| `kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' \| base64 -d` → `kubectl -n pokecalc exec mysql-0 -- env MYSQL_PWD="$pw" mysql -u root -N -e "SELECT COUNT(*) FROM pokedex.species;"` → `unset pw` | data §5 | Secret から root pw を取り出し、`mysql-0` 内の mysql クライアントで件数を数える | host → k3d(mysql-0) | 読み取り専用。パスワードは環境変数で渡し、画面に出さない |
| `kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-race1`(と `race2`)`kubectl -n pokecalc wait --for=condition=complete job/…race1 job/…race2 --timeout=600s \|\| true` `kubectl -n pokecalc get jobs …race1 …race2` | data §6 | CronJob の Job テンプレートから手動 Job を **2 本同時に**作り、`cronjob.sh` の flock 排他(ADR-0109)を確かめる | host → k3d | Job 2 本作成。期待(data.md §6a): 2 本とも最終的に 1/1。ロックに負けた側は exit 1 で 1〜2 回再試行してから成功(`backoffLimit: 2`) |
| `kubectl -n pokecalc logs -l job-name=pokedex-import-manual-race{1,2} --all-containers --prefix \| grep -i 'ロック\|lock'` | data §6 | 両 Job のログからロック取得失敗のメッセージ(`別の import が実行中`)を探す | host → k3d | 読み取り |
| `kubectl -n pokecalc delete job pokedex-import-manual-race1 pokedex-import-manual-race2` | data §6 | 検証用 Job の削除(ここでは Job のみ。DB データは消えない) | host → k3d | Job・Pod を削除 |
| `k3d cluster stop pokecalc` | api §7、data §7 | クラスタのコンテナを停止 | host → docker | データは保持(PVC 残る)。`make down`(= `k3d cluster delete`)は別物・人間の確認が要る |

## 3. 動作確認(smoke。verify §5)

| コマンド | 出現 | 裏で走るもの | 場所 | つなぐもの / 失敗時 |
|---|---|---|---|---|
| `make web-k3d-open` | (診断用。verify には無い) | `kubectl -n pokecalc port-forward svc/web 5173:80`(**前面で常駐**。別ターミナル。Ctrl-C で終了) | host → k3d(svc/web) | `localhost:5173` → web Service:80 → Pod:8080。gateway を通さず Web(nginx)だけを確かめるときに使う |
| `make web-k3d-smoke` | verify §5 | `web/scripts/k3d-smoke.sh`: `WEB_URL`(既定 `http://localhost:8080` = ブラウザで開く入口。k3d → Traefik → gateway → web)の `/healthz` を 1 秒間隔で最大 30 回待ち、その後 `/` `/reverse` `/engine.wasm` `/wasm_exec.js` `/static/no-such-file.js`(404 期待)`/api/no-such-endpoint`(404 期待)と、index.html が読む JS(200 期待。issue #268)を curl | host → 8080 | 読み取りのみ。Web だけを診断するときは `WEB_URL=http://localhost:5173`(`make web-k3d-open` の後) |
| `make api-smoke` | verify §5、api §5 | `services/gateway/scripts/smoke.sh`(`API_URL` 既定 `http://localhost:8080`)。順序と各項目は [verify-mapping.md(D)](verify-mapping.md) | host → 8080 | 8080 → Traefik → gateway。最終行に `calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=200 internal=404 balance=200 web=200`。**400・404 は異常系を確かめた結果で正常** |
| `make web-k3d-e2e` | verify §5 | `cd web && K3D_URL=$(WEB_URL) npm run e2e:k3d`(`playwright.k3d.config.ts`・`e2e-k3d/k3d.spec.ts`。サーバーは起動しない) | host の chromium → 8080 → gateway → web / calc / pokedex | 読み取りのみ。オフライン(例データ)とオンライン(実マスタ)で計算結果が画面に出ることを確かめる |
| `curl -s http://localhost:18080/healthz` | api §6 | `make dev` の gateway への疎通 | host | 期待 `{"status":"ok"}` |
| `DEV_GATEWAY_PORT=18080 DEV_CALC_PORT=18081 make dev` | api §6 | `scripts/dev.sh`: `go build` した calc・gateway を**ホストで**起動。calc は `services/calc/testdata/master.example.json`(架空データ)、gateway は `GATEWAY_CALC_URL=http://127.0.0.1:<calc>`・`GATEWAY_CORS_ALLOWED_ORIGINS=http://localhost:5173` | host のみ(k8s・DB を使わない) | Ctrl-C で両方停止。どちらかが落ちれば残りも止めて非ゼロ終了。k3d 稼働中は 8080 が衝突するため port を変える |

## 4. 画面の確認(verify §6(ブラウザ)・§7(iOS)。ブラウザ)

| 操作 | 場所 | 経路 |
|---|---|---|
| `http://localhost:8080` を開く(Chrome・Safari) | ブラウザ → host:8080 | serverlb → Traefik → Ingress `gateway`(`/`)→ gateway → `/`は `GATEWAY_WEB_URL=http://web` へ転送。`/api/*` は gateway が検証して calc・pokedex へ。`/api/balance` は Ingress が直接 balance へ |
| 計算タブ・逆算タブ・タイプバランスタブ(verify §6-1・§6-2) | 同上 | 計算・逆算は **ブラウザ内 WASM**(HTTP を使わない。ADR-0011)またはオンライン時に gateway の API。タイプバランスは `/api/balance/*` |
| `http://localhost:8080/reverse` を直接開く | 同上 | gateway → web の SPA フォールバック(`web/nginx.conf`) |

## 5. レーン別 runbook(balance / speed / judge)

| コマンド | 出現 | 裏で走るもの | 場所 | つなぐもの / 副作用 |
|---|---|---|---|---|
| `make balance-k3d-deploy` | balance §2・§9 | `balance-docker-build`(`docker build -t pokecalc/balance:local services/balance`)→ `k3d image import` → `kubectl apply -k services/balance/deploy/k8s/overlays/local`(例データの ConfigMap 3 つ)→ `rollout restart`/`status` | host → k3d | Ingress `/api/balance` → balance Service → Pod。gateway を通らない |
| `make balance-smoke` | balance §2・§9 | `services/balance/scripts/smoke.sh`(`BALANCE_URL` 既定 8080)。`/api/balance/healthz` と `v1/team-balance/{analyze,coverage,recommendations,threats}`・`v1/move-range/analyze` を叩く | host → 8080 | — |
| `make balance-k3d-deploy-readmodel` | balance §2b | `k3d-deploy-readmodel.sh`: `docker build` → `k3d image import` → `kubectl create configmap balance-readmodel … \| apply --server-side` → `apply -k overlays/local-readmodel` → `rollout restart`/`status` | host → k3d | **要 `make pokedex-export` 済み**(実データ由来の read model を ConfigMap に載せる) |
| `make balance-smoke-readmodel` | balance §2b | `smoke-readmodel.sh`: 先頭のポケモンで analyze と recommendations が 200 | host → 8080 | — |
| `make speed-k3d-deploy` | speed §2 | `docker build -f services/speed/Dockerfile -t pokecalc/speed:local .`(engine を含むためコンテキストはルート)→ import → `apply -k …/speed/deploy/k8s/overlays/local` → restart/status | host → k3d | Ingress `/api/speed` |
| `SPEED_URL=http://localhost:8080 make speed-smoke` | speed §2 | `speed/scripts/smoke.sh`: `/api/speed/healthz` `v1/pokemon` `v1/position` `v1/table?presets=max-scarf`(および `unknown` の異常系) | host → 8080 | — |
| `kubectl -n pokecalc port-forward svc/mysql 3306:3306 >/tmp/mysql-pf.log 2>&1 &` `PF_PID=$!` `sleep 2` | speed §3 | mysql への一時 port-forward(バックグラウンド) | host → k3d(svc/mysql) | ホスト 3306 → headless mysql:3306 |
| `export POKEDEX_DATABASE_DSN=$(kubectl … get secret mysql-auth -o jsonpath='{.data.pokedex-dsn}' \| base64 -d \| sed 's/@tcp(mysql:/@tcp(127.0.0.1:/')` | speed §3 | Secret の DSN のホスト部を 127.0.0.1 に書換え(port-forward 越しに繋ぐ) | host | 値は画面に出さない(環境変数のみ)。**Git の Secret のキー名前提**([k8s-local.md §9 #1](k8s-local.md)) |
| `make pokedex-export` | speed §3 | `go run ./pokedex/cmd/pokedex export -out ../data/generated/readmodel`(DB を読み read model 4 ファイル) | host → 3306 | `data/generated/readmodel/` に書込(Git 管理外) |
| `kill $PF_PID` | speed §3 | port-forward を停止 | host | — |
| `make speed-k3d-deploy-readmodel` / `make speed-smoke-readmodel` | speed §3 | balance と同型(ConfigMap `speed-readmodel`) | host → k3d | — |
| `make speed-gitops-template-check` | speed §4 | `speed-kustomize`(4 overlay を描画)→ `check-gitops.sh template` | host | クラスタ非接触。gitops overlay の digest プレースホルダを許す検査 |
| `judge`(`make judge-k3d-deploy` `judge-smoke`) | (手順書に記載なし。Makefile のみ) | `judge/scripts/smoke.sh`: `/api/judge/healthz` が 200・`"status":"ok"`(30 回再試行) | host → 8080 | JD0 時点で healthz のみ |

## 6. Argo CD / GitOps 系(詳細は `gitops-argocd.md`(C))

| コマンド | 出現 | 裏で走るもの | 場所 | 副作用 |
|---|---|---|---|---|
| `./scripts/argocd-bootstrap.sh` | balance §3、speed §5 | 固定コミット SHA の `install.yaml` を取得 → SHA-256 検証 → 3 image を digest 参照に書換 → `kubectl apply -n argocd --server-side -f -`(ADR-0405) | host → k3d(ns argocd) | **Argo CD を導入**(外部ネットワーク) |
| `read -rs PAT` → `kubectl -n argocd create secret generic repo-pokecalc`(型・URL・ユーザー・認証値を `--from-literal` で渡す。引数の全文は docs/runbooks/balance.md §4)→ `kubectl -n argocd label secret repo-pokecalc` でラベル `argocd.argoproj.io/secret-type` に値 `repository` を付与 | balance §4、speed §6 | Git リポジトリの認証情報を Argo CD に登録(PAT は入力のみ・履歴に残さない) | host → k3d | **Secret 作成**(人間の作業。値は文書に書かない) |
| `make balance-registry-apply` | balance §5 | `kubectl apply -k services/balance/deploy/local-registry` → `rollout status deployment/registry` | host → k3d(ns balance-registry) | **クラスタ内レジストリを作成**(hostPort 5000) |
| `make balance-argocd-app` / `make speed-argocd-app` | balance §5、speed §7 | `argocd-local-app.sh`: `kubectl kustomize …/deploy/argocd` の `repoURL` を `git remote get-url origin` で埋めて `kubectl apply` | host → k3d(ns argocd) | **Application を作成**(Git 上の repoURL は `git.example.invalid` のプレースホルダ) |
| `digest=$(make -s balance-registry-push …)` / `speed-registry-push` | balance §6、speed §8 | `docker build` → `docker save` → レジストリへ port-forward(balance 5001・speed 5002)→ `crane push --insecure` → digest を出力 | host → docker → registry | image を push |
| `sed -i '' "s/digest: .*/digest: ${digest}/" …/overlays/gitops/kustomization.yaml` + `git diff …` | balance §6、speed §8 | gitops overlay の image digest を書換 | host(Git 作業ツリー) | **ファイルを書換**(commit・PR は別途) |
| `kubectl -n argocd annotate application pokecalc-{balance,speed} argocd.argoproj.io/refresh=normal --overwrite` | balance §7、speed §9 | Application の再取得を要求 | host → k3d | — |
| `kubectl config set-context --current --namespace=argocd` → `argocd --core app sync pokecalc-{balance,speed} --timeout 180` → `kubectl config set-context --current --namespace=default` | balance §7、speed §9 | argocd CLI を `--core`(サーバ無し。kubectl context の namespace 経由)で同期 | host → k3d | **同期 = クラスタ反映**。namespace を一時的に書換え、最後に `default` へ戻す |
| `kubectl -n pokecalc rollout status deployment/{balance,speed} --timeout=120s` `kubectl -n pokecalc get deploy … -o jsonpath='{…image}'` `grep digest …/kustomization.yaml` | balance §8、speed §10 | 反映結果の確認(digest 参照になっているか) | host → k3d | 読み取り |
| `for i in $(seq 1 15); do code=$(curl … /api/{balance,speed}/healthz); [ "$code" = 200 ] && break; sleep 2; done; echo "health=$code"` | balance §8、speed §10 | healthz を最大 15 回ポーリング | host → 8080 | — |

## 7. iOS

| コマンド | 出現 | 裏で走るもの | 場所 | つなぐもの / 副作用 |
|---|---|---|---|---|
| `make ios-test \| grep '^ios-'` | ios §1 | `ios-lint`(`ios/scripts/*.sh` 構文)→ `ios-gen-check`(`openapi-gen.sh --check`: 一時ディレクトリに生成し差分検査)→ `ios-test-unit`(`cd ios/PokeCalcKit && run-xcode-tests.sh … -scheme PokeCalcKit-Package`)→ `ios-test-ui`(`-project ios/PokeCalc.xcodeproj -scheme PokeCalc`。モック強制)→ `ios-check-infoplist`(`xcodebuild build` で Info.plist に `PokeCalcAPIBaseURL` が入るか) | host(Xcode・シミュレータ) | k3d・gateway 不要(モック)。`run-xcode-tests.sh` はスキップ・空実行も失敗にする |
| `make ios-sim-run IOS_SCREEN=root\|calc\|reverse\|team [IOS_APPEARANCE=… IOS_CONTENT_SIZE=…]` | ios §2〜 | `sim-run.sh`: `xcrun simctl boot`/`bootstatus` → `xcodebuild build` → インストール・モック起動 | host(シミュレータ) | 画面を直接開く環境変数 `POKECALC_OPEN_*_AT_LAUNCH`(`RootView.swift`) |
| `tailscale serve https / http://localhost:8080` | ios-device §2 | Tailscale の HTTPS 入口を host:8080(k3d serverlb → gateway)へ転送 | host(tailnet) | **端末から gateway を tailnet 経由で公開**(人間の作業)。`tailscale serve off` で解除 |
| `make ios-check-infoplist` | ios-device §3 | 上記の xcodebuild 検査 | host | `POKECALC_API_BASE_URL` は `https:/$()/<host>`(`//` がコメント扱いになるため。ADR-0500 §5) |

## カバレッジ

- 読んだ範囲: 上記 8 文書の全コードブロック(README 6、verify-m1 全、api・data・balance・speed・ios・ios-device の全ブロックを機械抽出。抽出結果の行 = 表の行に全件対応)、`scripts/{up,dev,db-local-up,wasm,doctor,e2e}.sh`、`web/scripts/k3d-smoke.sh` 全行、`services/gateway/scripts/smoke.sh` 全行、`web/playwright*.ts`・`web/e2e/support/serverConfig.ts` の起動コマンド、`services/{balance,speed}/scripts/*.sh` と `ios/scripts/*.sh` の冒頭・外部コマンド行。
- 読めていない箇所: `scripts/check-publishable.sh` の全検査項目、`services/{balance,speed}/scripts/smoke*.sh` の判定の細部、`check-gitops.sh` の判定、`ios/scripts/*.sh` の内部、`tools/importer/fetch*.mjs`・`check-upstream.mjs`、`argocd-bootstrap.sh` の後半(冒頭・固定値のみ確認)。
- 実行して確認したもの: `web-k3d-smoke`(既定 8080。2026-09-25、ADR-0305 の後)。それ以外は静的に読んだ内容で、実行はしていない。
- 未実装・スタブ: `make e2e`(`scripts/e2e.sh` は echo のみ。P4-6)、`make assets`(echo のみ)。judge は `healthz` のみ(JD0)。
- 手順書に記載が無いが存在するコマンド: `judge-k3d-deploy`/`judge-smoke`(§5 に記載)、`make down`(クラスタ削除。人間の確認)、`migrate-*`(db-mysql.md)。
