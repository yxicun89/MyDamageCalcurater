# 環境変数・ConfigMap・Secret

- 基準: `origin/main` 3379b03。値そのもの(パスワード・DSN の実値)は書かない。設定は環境変数で渡し、ローカルとクラウドの差は Kustomize overlay で吸収する(CLAUDE.md 技術規約)。
- 関連: サービスの構成は [architecture.md](architecture.md)、ルートは [api-endpoints.md](api-endpoints.md)。

## 1. サービスが読む環境変数(実装側)

「必須」= 未設定だと起動に失敗する。「任意」= 未設定でも起動し、機能が 503/404 になる。

### gateway(`services/gateway/cmd/gateway/main.go`。名前 `:32-39`、`loadConfig` `:66`)

| 変数 | 必須 | 既定 | 意味 | 検証 |
|---|---|---|---|---|
| `GATEWAY_ADDR` | 任意 | `:8080` | 待ち受け | — |
| `GATEWAY_CALC_URL` | **必須** | — | calc-svc の基底 URL(`/api/calc*` の転送先) | http/https・ホストあり(`:142`) |
| `GATEWAY_POKEDEX_URL` | 任意 | 未設定 | pokedex-svc(`/api/pokedex/*`)。未設定 → 503 | 同上 |
| `GATEWAY_ASSETS_URL` | 任意 | 未設定 | 画像配信元(`/assets/*`)。未設定 → 404 | 同上 |
| `GATEWAY_WEB_URL` | 任意 | 未設定 | Web の静的配信元(予約パス以外の GET/HEAD)。ADR-0205 | 同上 + クエリ不可(`:158`) |
| `GATEWAY_CORS_ALLOWED_ORIGINS` | 任意 | 空(CORS ヘッダなし) | カンマ区切りの許可オリジン(完全一致) | `*` は起動エラー(`:171`) |
| `GATEWAY_UPSTREAM_TIMEOUT` | 任意 | `10s` | 上流の応答ヘッダ待ち上限 | 正、かつ `http.Server.WriteTimeout`(60s)未満(`:110-126`) |

### calc-svc(`services/calc/cmd/calc/main.go`。名前 `:37-41`、`loadConfig` `:80`)

| 変数 | 必須 | 既定 | 意味 |
|---|---|---|---|
| `CALC_ADDR` | 任意 | `:8080` | 待ち受け |
| `CALC_MASTER_URL` | どちらか**ちょうど 1 つ** | — | pokedex-svc の基底 URL。`GET /internal/pokedex/master` を起動後バックグラウンドで取得(再試行 500ms→30s 上限) |
| `CALC_MASTER_PATH` | 同上 | — | マスタ一式 JSON(`MasterExport` 形)を起動時に読む。`make dev`・テスト用。失敗は非ゼロ終了 |
| `CALC_TYPECHART_PATH` | 設定すると**起動エラー** | — | 廃止(相性表は MasterExport に含む。ADR-0204) |

### pokedex-svc・migrate・import

| 変数 | 読む場所 | 必須 | 既定 | 意味 |
|---|---|---|---|---|
| `POKEDEX_ADDR` | `services/pokedex/cmd/pokedex/main.go:28` | 任意 | `:8080` | 待ち受け(`serve`) |
| `POKEDEX_DATABASE_DSN` | `cmd/pokedex/main.go:29`(serve・export)、`cmd/migrate/main.go:30`、`cmd/import/main.go:116` | **必須**(import の `-dry-run` は不要) | — | go-sql-driver/mysql 形式。`parseTime=true` を強制付与。エラー文に DSN を出さない |
| `POKEDEX_TEST_DSN` | `services/pokedex/db/mysql_test.go:37`・`importer/mysql_test.go:37` | `make test-db` で必須 | — | DB テスト用(`-tags mysql`)。無ければ**失敗**(スキップしない) |

- CLI 引数(環境変数ではない): `pokedex-migrate up|down|version`(`down` は `-confirm <DB名>` 必須。Makefile では `CONFIRM_DESTROY`)、`pokedex serve|export -out <dir>`、`pokedex-import -data <dir> [-dry-run] [-force] [-typechart <file>] [-upstream <file>] [-upstream-max-age 24h]`(`-typechart` の既定は `<data>/../testdata/golden/typechart.json`。ADR-0118)(`cmd/import/main.go:67-`)。

### balance / speed / judge(`PORT` は 3 サービス共通、既定 `8080`)

| サービス | 変数 | 読む場所 | 必須 | 意味 |
|---|---|---|---|---|
| balance | `PORT` | `cmd/api/main.go:27` | 任意 | 待ち受けポート |
| balance | `BALANCE_POKEMON_TYPES_PATH` | `cmd/api/config.go:9` | 任意 | ポケモンのタイプ read model。未設定 → analyze 等が 503。設定済みで読めない → 非ゼロ終了 |
| balance | `BALANCE_MOVES_PATH` | `config.go:30` | 任意 | 技 read model(coverage 等) |
| balance | `BALANCE_ABILITIES_PATH` | `config.go:50` | 任意 | 特性 read model |
| speed | `PORT` | `cmd/api/config.go:13` | 任意 | 待ち受けポート |
| speed | `SPEED_POKEMON_PATH` | `config.go:9` | 任意 | ポケモン read model。未設定 → 503 |
| judge | `PORT` | `cmd/api/config.go:13` | 任意 | 待ち受けポート |
| judge | `JUDGE_POKEDEX_BASE_URL` | `config.go:16` | 任意 | pokedex-svc の基底 URL。未設定 → judge API は 503 |
| judge | `JUDGE_CALC_BASE_URL` | `config.go:17` | 任意 | calc-svc の基底 URL。同上 |
| judge | `JUDGE_UPSTREAM_TIMEOUT` | `config.go:18-19` | 任意(既定 `3s`) | 上流呼び出しのタイムアウト(正の duration) |
| judge | `JUDGE_CHOICE_SCARF_ITEM_ID` | `config.go:21` | 任意 | こだわりスカーフの持ち物 ID の上書き(空なら `judge.DefaultChoiceScarfItemID`。ADR-0701 §3) |

### Web(ビルド・開発)

| 変数 | 読む場所 | 既定 | 意味 |
|---|---|---|---|
| `VITE_API_BASE_URL` | `web/src/api/config.ts:12`(`import.meta.env`) | 空 → `/`(同一オリジン) | API の基点 URL。末尾スラッシュ 1 つに正規化。Web・balance・speed・pokedex クライアントが共有 |
| `API_PROXY_TARGET` | `web/vite.config.ts:29` | 空(プロキシなし) | 開発サーバー・preview が `/api` を転送する先(`VITE_` 接頭辞なし = Node 側のみ) |
| `BALANCE_PROXY_TARGET` | `web/vite.config.ts:30` | 空 | 同 `/api/balance`(`/api` より先に照合) |
| `BASE_URL`(Vite 組み込み) | `web/src/app/routes.ts` ほか 11 箇所 | `/` | 画面パスの基点 |
| `CI` | `web/playwright*.config.ts`(4 ファイル) | 未設定 | `forbidOnly`・リトライ設定 |

### iOS

| 設定 | 定義 | 参照 |
|---|---|---|
| `POKECALC_API_BASE_URL`(xcconfig) | `ios/PokeCalc/Config/PokeCalc.xcconfig`(値は空) | `ios/PokeCalc-Info.plist:6` → `ios/PokeCalcKit/Sources/PokeCalcCore/AppConfiguration.swift:30`。空ならモック動作 |

## 2. マニフェストが注入する環境変数(注入側)

| ワークロード | 変数 = 値 | 定義 | 備考 |
|---|---|---|---|
| gateway | `GATEWAY_CALC_URL=http://calc`・`GATEWAY_POKEDEX_URL=http://pokedex`・`GOMEMLIMIT=56MiB` | `deploy/k8s/base/gateway/deployment.yaml` | Service 名で解決 |
| gateway(local) | `GATEWAY_CORS_ALLOWED_ORIGINS=http://localhost:5173`・`GATEWAY_WEB_URL=http://web` | `deploy/k8s/overlays/local/api/gateway-patch.yaml` | base には置かない(クラウドの Web の置き方が未定) |
| calc | `CALC_MASTER_URL=http://pokedex`・`GOMEMLIMIT=56MiB` | `deploy/k8s/base/calc/deployment.yaml` | |
| pokedex | `POKEDEX_DATABASE_DSN`(Secret `mysql-auth`/`pokedex-dsn`) | `deploy/k8s/base/pokedex/deployment.yaml:32` | `args: ["serve"]` |
| pokedex-migrate(Job) | `POKEDEX_DATABASE_DSN`(同上 `job-migrate.yaml:60`)。initContainer `wait-for-mysql`: `MYSQL_PWD`(Secret `mysql-root-password` `:42`) | `deploy/k8s/base/pokedex/job-migrate.yaml` | |
| pokedex-import(CronJob) | `HOME=/tmp`・`npm_config_cache=/tmp/npm-cache`・`POKEDEX_DATABASE_DSN`(Secret `:66`) | `deploy/k8s/base/pokedex/cronjob-import.yaml` | 読み取り専用ルートのため /tmp を使う |
| mysql | `MYSQL_ROOT_PASSWORD`(Secret `mysql-root-password` `:32`)・`MYSQL_DATABASE=pokedex`(初回起動時のみ有効) | `deploy/k8s/overlays/local/mysql/statefulset.yaml` | |
| balance | `PORT=8080` | `services/balance/deploy/k8s/base/deployment.yaml` | |
| balance(local) | `BALANCE_POKEMON_TYPES_PATH=/etc/balance/pokemon-types.json`・`BALANCE_MOVES_PATH=/etc/balance/moves.json`・`BALANCE_ABILITIES_PATH=/etc/balance/abilities.json` | `services/balance/deploy/k8s/overlays/local/deployment-*-patch.yaml`(3) | 架空データ |
| balance(local-readmodel) | 同 3 変数 = `/etc/balance/readmodel/{pokemon-types,moves,abilities}.json` | `…/overlays/local-readmodel/deployment-readmodel-patch.yaml` | pokedex export の実 read model |
| speed | `PORT=8080` | `services/speed/deploy/k8s/base/deployment.yaml` | |
| speed(local) | `SPEED_POKEMON_PATH=/etc/speed/pokemon.json` | `services/speed/deploy/k8s/overlays/local/deployment-pokemon-patch.yaml` | 架空データ |
| speed(local-readmodel) | `SPEED_POKEMON_PATH=/etc/speed/readmodel/speed-pokemon.json` | `…/overlays/local-readmodel/deployment-readmodel-patch.yaml` | |
| judge | `JUDGE_POKEDEX_BASE_URL=http://pokedex`・`JUDGE_CALC_BASE_URL=http://calc` | `services/judge/deploy/k8s/base/deployment.yaml` | |
| web | なし | `deploy/k8s/base/web/deployment.yaml` | `/tmp` は emptyDir |

## 3. ConfigMap(全件)

| 名前 | 定義・作成元 | 内容 | 参照(マウント先) | Git |
|---|---|---|---|---|
| `mysql-config` | `deploy/k8s/overlays/local/mysql/configmap.yaml` | `charset.cnf`(utf8mb4・`utf8mb4_0900_ai_ci`) | mysql StatefulSet `/etc/mysql/conf.d/charset.cnf`(subPath) | ○ |
| `balance-pokemon-types`・`balance-moves`・`balance-abilities` | `services/balance/deploy/k8s/overlays/local/kustomization.yaml:22-`(`configMapGenerator`。元 = 同ディレクトリの `*.example.json`) | 架空の read model | balance `/etc/balance/{pokemon-types,moves,abilities}.json`(subPath) | ○(架空) |
| `balance-readmodel` | `services/balance/scripts/k3d-deploy-readmodel.sh:28`(`kubectl create configmap`。`make balance-k3d-deploy-readmodel`) | `data/generated/readmodel/` の 3 ファイル(1MB 未満) | balance `/etc/balance/readmodel`(ディレクトリ) | ×(生成物) |
| `speed-pokemon` | `services/speed/deploy/k8s/overlays/local/kustomization.yaml:13`(`configMapGenerator`) | 架空のポケモン read model | speed `/etc/speed/pokemon.json`(subPath) | ○(架空) |
| `speed-readmodel` | `services/speed/scripts/k3d-deploy-readmodel.sh:30` | `speed-pokemon.json`(1 ファイル。1MB 未満) | speed `/etc/speed/readmodel` | ×(生成物) |
| `pokedex-name-overrides` | `scripts/up.sh:86-88`(`data/local/name_ja_overrides.json` があるときだけ作成/更新。無くても既存は消さない) | 日本語名の上書き(任意・Git 管理外の実データ) | pokedex-import CronJob `/app/data/local`(`optional: true`、read-only) | ×(実データ) |

- read model の Deployment は `pokecalc.example/readmodel-hash` annotation で ConfigMap の内容が変わると Pod を作り直す(スクリプトが hash を差し込む)。

## 4. Secret(全件)

| 名前 | 作成元 | キー | 参照 |
|---|---|---|---|
| `mysql-auth`(namespace `pokecalc`) | `scripts/up.sh:33-44`(無いときだけ `openssl rand -hex 16` で作る。既存は上書きしない。**Git に置かない**。ADR-0100 §9) | `mysql-root-password`・`pokedex-dsn`(`root:<pw>@tcp(mysql:3306)/pokedex?parseTime=true` 形式) | mysql StatefulSet `MYSQL_ROOT_PASSWORD`(`statefulset.yaml:32-34`)、pokedex `POKEDEX_DATABASE_DSN`(`deployment.yaml:32-36`)、migrate Job(`job-migrate.yaml:42-46,60-64`)、import CronJob(`cronjob-import.yaml:66-70`) |
| `balance-registry`(imagePullSecret。**適用されない例のみ**) | `services/balance/deploy/k8s/overlays/gitops/private-registry-patch.example.yaml` | 未定義(Secret の実体は Git に無い) | balance Deployment の `imagePullSecrets`(この patch は kustomization に入っていない) |

- `kind: Secret` のマニフェストはリポジトリに存在しない(`grep '^kind: Secret'` = 0 件)。Argo CD 自身の Secret は導入時にクラスタ側で作られる(C 章)。
- ローカル(k8s 外)の MySQL: `make db-local-up` → `scripts/db-local-up.sh` が `.env`(`ENV_FILE` で変更可)の `MYSQL_ROOT_PASSWORD` を読む。空なら明示的に失敗。`.env.example` は値なし。

## 5. Makefile・シェルの上書き可能な変数

### Makefile(`?=` の既定値)

| 変数 | 既定 | 定義 | 用途 |
|---|---|---|---|
| `GO` | `go` | `Makefile:11`・`services/{balance,speed,judge}/Makefile:1` | Go コマンド |
| `CLUSTER` | `pokecalc` | `Makefile:12`・`services/{balance,speed,judge}/Makefile:2` | k3d クラスタ名(context は `k3d-$CLUSTER` を要求) |
| `GATEWAY_URL` | `http://localhost:8080` | `Makefile:13` | gateway の URL |
| `API_CLUSTER`・`API_URL` | `$(CLUSTER)`・`http://localhost:8080` | `services/gateway/Makefile:8-9` | `api-k3d-deploy`・`api-smoke` |
| `WEB_DIR`(`:=`)・`WEB_URL` | `web`・`http://localhost:8080`(gateway 経由。診断時は 5173) | `web/Makefile:4,61` | `web-k3d-smoke` の宛先 |
| `BALANCE_DIR`・`BALANCE_IMAGE`・`BALANCE_URL`・`BALANCE_REGISTRY_PORT` | `services/balance`・`pokecalc/balance:local`・`http://localhost:8080`・`5001` | `services/balance/Makefile:3-7` | balance の build・smoke・ローカルレジストリ |
| `SPEED_DIR`・`SPEED_IMAGE`・`SPEED_URL`・`SPEED_READMODEL_DIR`・`SPEED_REGISTRY_PORT` | `services/speed`・`pokecalc/speed:local`・`http://localhost:8080`・`data/generated/readmodel`・`5002` | `services/speed/Makefile:3-10` | 同上 |
| `JUDGE_DIR`・`JUDGE_IMAGE`・`JUDGE_URL` | `services/judge`・`pokecalc/judge:local`・`http://localhost:8080` | `services/judge/Makefile:3-5` | 同上 |
| `IOS_SIMULATOR`・`IOS_DESTINATION`・`IOS_SCREEN`・`IOS_APPEARANCE`・`IOS_CONTENT_SIZE` | `iPhone 18 Pro`・`platform=iOS Simulator,name=$(IOS_SIMULATOR)`・`root`・`light`・`large` | `ios/Makefile:4-9` | シミュレータ |
| `CONFIRM_DESTROY`(引数) | なし | `Makefile:migrate-down` | `migrate-down` に DB 名を必須で渡す(破壊的) |
| `POKEDEX_TEST_DSN`(引数/環境) | なし | `Makefile:117` | `make test-db` |

### シェルスクリプト(`${VAR:-既定}`)

| スクリプト | 変数 = 既定 |
|---|---|
| `scripts/up.sh:7-10` | `CLUSTER=pokecalc`・`POKEDEX_MIGRATE_IMAGE=pokecalc/pokedex-migrate:0.1.0`・`POKEDEX_IMPORTER_IMAGE=pokecalc/pokedex-importer:0.1.0`・`POKEDEX_SERVER_IMAGE=pokecalc/pokedex:0.1.0` |
| `scripts/dev.sh:25-26` | `DEV_CALC_PORT=8081`・`DEV_GATEWAY_PORT=8080`(k3d と衝突するときは変える) |
| `scripts/db-local-up.sh:8,16,18` | `ENV_FILE=.env`・`POKEDEX_MYSQL_CONTAINER=pokecalc-mysql-local`・`MYSQL_ROOT_PASSWORD`(空なら失敗) |
| `services/gateway/scripts/smoke.sh:22-25` | `API_URL=http://localhost:8080`・`API_SMOKE_RETRIES=30`・`API_SMOKE_BALANCE=auto`(`on`/`off`)・`API_SMOKE_NAMESPACE=pokecalc` |
| `web/scripts/k3d-smoke.sh:15,17` | `WEB_URL=http://localhost:8080`・`WEB_SMOKE_RETRIES=30` |
| `services/balance/scripts/*.sh` | `BALANCE_DIR=services/balance`・`BALANCE_URL=http://localhost:8080`・`BALANCE_READMODEL_DIR=data/generated/readmodel`・`BALANCE_IMAGE=pokecalc/balance:local`・`CLUSTER=pokecalc`・`BALANCE_REGISTRY_PORT=5001`・`BALANCE_RELEASE_IMAGE`(空)・`BALANCE_PLATFORMS=linux/amd64,linux/arm64`・`BALANCE_GITOPS_REPO_URL`(既定 = ファイル内の値 / `git remote get-url origin`) |
| `services/speed/scripts/*.sh` | `SPEED_DIR=services/speed`・`SPEED_URL=http://localhost:8080`・`SPEED_READMODEL_DIR=data/generated/readmodel`・`SPEED_IMAGE=pokecalc/speed:local`・`CLUSTER=pokecalc`・`SPEED_REGISTRY_PORT=5002`・`SPEED_RELEASE_IMAGE`(空)・`SPEED_PLATFORMS=linux/amd64,linux/arm64`・`SPEED_GITOPS_REPO_URL`(同上) |
| `services/judge/scripts/smoke.sh:5` | `JUDGE_URL=http://localhost:8080` |
| `tools/importer/cronjob.sh:12,17,18` | `HOME=/tmp`・`IMPORT_APP_DIR=/app`・`IMPORT_LOCK_FILE=$IMPORT_APP_DIR/data/generated/.import.lock`(テスト契約。名前を変えない。ADR-0109) |
| `ios/scripts/xcode-env.sh:8` | `DEVELOPER_DIR`(CommandLineTools を指していたら Xcode に切り替え) |
| `scripts/argocd-bootstrap.sh:12-26`(定数 `readonly`。環境変数ではない) | `ARGOCD_INSTALL_COMMIT`・`EXPECTED_INSTALL_YAML_SHA256`・`ARGOCD_IMAGE_DIGEST`・`DEX_IMAGE_DIGEST`・`REDIS_IMAGE_DIGEST`・各 REPO(版を上げるときはここだけ更新。C 章) |
| `scripts/check-publishable.sh`(内部定数) | `IMPORTER_TESTDATA_DIR` ほか(検査対象の定数。上書き用ではない)、`SELFTEST_TMP`(自己テストの一時 dir) |

## 6. 件数の突き合わせ

| 区分 | 件数 | 突き合わせ |
|---|---|---|
| サービス実装が読む環境変数(名前の異なるもの) | gateway 7・calc 4(廃止 1 含む)・pokedex 3(`POKEDEX_ADDR`・`POKEDEX_DATABASE_DSN`・`POKEDEX_TEST_DSN`)・balance 4(`PORT` 含む)・speed 2・judge 5 | 各 `main.go`/`config.go` の定数と一致 |
| 上記のうちマニフェストが注入する名前 | `GATEWAY_CALC_URL`・`GATEWAY_POKEDEX_URL`・`GATEWAY_CORS_ALLOWED_ORIGINS`・`GATEWAY_WEB_URL`・`CALC_MASTER_URL`・`POKEDEX_DATABASE_DSN`・`PORT`・`BALANCE_*_PATH`×3・`SPEED_POKEMON_PATH`・`JUDGE_POKEDEX_BASE_URL`・`JUDGE_CALC_BASE_URL`(+ `GOMEMLIMIT`・`HOME`・`npm_config_cache`・`MYSQL_*`) | §2 |
| 未注入(既定値で動く) | `GATEWAY_ADDR`・`GATEWAY_ASSETS_URL`・`GATEWAY_UPSTREAM_TIMEOUT`・`CALC_ADDR`・`CALC_MASTER_PATH`・`POKEDEX_ADDR`・`JUDGE_UPSTREAM_TIMEOUT`・`JUDGE_CHOICE_SCARF_ITEM_ID` | — |
| ConfigMap | 6 種(生成 3+1 を含めると名前 8) | §3。実クラスタの存在確認は未実施 |
| Secret | 実体 1(`mysql-auth`)+ 例 1 | `grep '^kind: Secret'` は 0 件(`up.sh` が作成) |

## カバレッジ

- 読んだ: gateway `main.go` 全体、calc `main.go` 全体、pokedex `cmd/pokedex/main.go`・`cmd/import/main.go`(`:60-130`)・`cmd/migrate/main.go`(引数部)、balance `cmd/api/{main,config}.go`(冒頭)、speed・judge の `config.go`、`web/vite.config.ts`・`web/src/api/config.ts`・playwright の `CI` 参照、`deploy/**` と `services/{balance,speed,judge}/deploy/**` の全 YAML、`scripts/{up,dev,db-local-up}.sh`・`tools/importer/cronjob.sh`・各 smoke/deploy スクリプトの `${VAR:-}` 行、全 Makefile の `?=`/`$(VAR)` 集計、`.env.example`。
- 機械抽出: `grep -rhoE '\$\{[A-Z_]+:[-=]'`・`grep -oh '\$\([A-Z_]+\)'`・`os.Getenv|LookupEnv`(Go 8 ファイル: balance/speed/judge/calc/pokedex×3/gateway = 全件掲載)・`import.meta.env|process.env`。
- 読めていない・未確認: 各スクリプトの本文(変数の意味は名前と周辺コメントから。`scripts/e2e.sh`・`scripts/wasm.sh`(`ROOT`・`OUT_DIR` は内部変数)・`scripts/check-publishable.sh`・`scripts/argocd-bootstrap.sh` の全変数は網羅していない)、`tools/importer/*.mjs`・`tools/golden/generate.mjs` の環境変数(`process.env` 参照は grep で 0 件)、`ios/scripts/*` の全変数、`web/e2e/support/serverConfig.ts` のポート定数(環境変数ではなく定数)、実クラスタの ConfigMap/Secret の存在(`kubectl get` 未実施)、`docker-compose`・CI 設定(リポジトリに無い)。
- 未実装: cloud overlay は Secret・MySQL・image 配布経路が未整備(`deploy/k8s/overlays/cloud/cronjob-import-suspend-patch.yaml` のコメント)。
