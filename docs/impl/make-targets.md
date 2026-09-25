# Make ターゲット一覧

- 基準: `origin/main` 3379b03 取り込み後(docs/impl-guide)。行番号は各 Makefile の定義行。
- 読み方: 「前提」= Makefile の前提条件。「副作用」の**太字**はクラスタ・DB・レジストリ・破壊的操作。コマンド単位の実行順は [runbook-commands.md](runbook-commands.md)、リソースは [k8s-local.md](k8s-local.md)。
- ルート `Makefile` は末尾で 6 本を `include`(`Makefile:239-244`)。ターゲット名は接頭辞でレーン分離(`api-`=gateway/calc、`web-`、`balance-`、`speed-`、`judge-`、`ios-`)。

## CI(GitHub Actions。ADR-0119)

`.github/workflows/ci.yml` が PR と main への push で1ジョブ(ubuntu-latest、Secret 不要)を実行する。
下表の `test`/`lint`/`build` が Web・balance・speed・judge を合成済みなので、CI も go/web でジョブを
分けず `make test` → `make lint` → `make build` → `make test-golden` → `make test-wasm` を順に呼ぶ
(分けると Web 分が二重実行になる)。`kubectl`(`azure/setup-kubectl`、stable.txt 準拠のバージョンに固定)を
追加で入れ、`make lint` の `k8s-render` に加えて `api-kustomize`・`web-kustomize`・`balance-kustomize`・
`speed-kustomize`・`judge-kustomize`(各レーン専用 overlay)も描画確認する。最後に `make check-publishable`
を単独ステップとしても走らせる(`make lint` に含まれるが、ログで単独の合否として見せるため)。
Go は `go-version-file: go.work`、Node は `node-version-file: web/.node-version` を読み、版をワークフローに
二重に書かない。対象外(iOS・Playwright e2e・k3d への実 apply)は ci.yml 冒頭のコメントと ADR-0119 を参照。

## 共通の仕様

| 項目 | 内容 | 根拠 |
|---|---|---|
| `SHELL` | `/usr/bin/env bash` | `Makefile:5` |
| `PATH` | `/opt/homebrew/bin` を先頭に追加 | `Makefile:9` |
| 変数の既定 | `GO=go` `CLUSTER=pokecalc` `GATEWAY_URL=http://localhost:8080`(`GATEWAY_URL` は Makefile 内で未使用) | `Makefile:11-13` |
| 既定ゴール | `help` | `Makefile:6` |
| `test`/`lint`/`build` | 複数 Makefile が**前提条件だけ追記**(レシピを持たない)。結果は下表 | 各 Makefile 冒頭 |
| k3d 対象の安全弁 | `up.sh`・`import-k8s`・`web-k3d-deploy` は kubectl context が `k3d-$(CLUSTER)` でなければ中断 | `scripts/up.sh:21`、`Makefile:179`、`web/Makefile:78` |

### `test` / `lint` / `build` の合成結果(`make -pqRr` で確認)

| ターゲット | 前提条件(全 Makefile の合算) | 追加のレシピ |
|---|---|---|
| `test` | test-engine test-golden test-services test-tools test-scripts balance-test speed-test judge-test web-test | なし |
| `lint` | speed-lint judge-lint web-lint balance-lint | あり(gofmt・vet・構文検査・k8s-render・check-publishable・selftest。`Makefile:67-80`) |
| `build` | speed-build judge-build web-build balance-build | あり(engine・services・tools の go build) |
| `gen` | gen-go gen-sql gen-ts | なし |

注意: `api-` ターゲット(calc・gateway)は `test`/`lint`/`build` に前提を足さない(ユニットテストは `test-services` に含まれる。`services/gateway/Makefile:1-6`)。iOS も含まれない(`ios-test` は別)。

### 発見: `make help` に出ないターゲットが 15 件

`help` の抽出正規表現が `^[a-zA-Z_-]+:`(`Makefile:17`)で**数字を含む名前を拾わない**。次の 15 件は `##` の説明があるのに一覧に出ない:
`e2e` `import-k8s` `k8s-render` `api-k3d-deploy` `balance-k3d-deploy` `speed-k3d-deploy` `judge-k3d-deploy` `web-k3d-deploy` `web-k3d-open` `web-k3d-smoke` `web-e2e-install` `web-e2e` `web-e2e-online` `web-e2e-balance` `web-e2e-container`
(`web-k3d-open` が `make help` で見つからない原因。修正案: 正規表現を `[a-zA-Z0-9_-]+` にする。ルート Makefile はデータレーンの管轄。未修正)

## 全ターゲット

### `Makefile`(41 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `help` | 16 | — | `grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \| \ ⏎ awk 'BEGIN{FS=":.*?## "}{printf " \033[36m%-18s\033[0m %s\n", $$1, $$2}'` | なし(読み取り/検査) |
| `doctor` | 22 | — | `./scripts/doctor.sh` | なし(読み取り/検査) |
| `gen` | 27 | gen-go gen-sql gen-ts | (レシピなし) | なし(前提条件のみ) |
| `gen-go` | 30 | — | `cd services && $(GO) tool oapi-codegen -config internal/api/cfg.yaml ../api/openapi.yaml ⏎ echo "gen-go: services/internal/api/openapi.gen.go を生成"` | 生成物を書換 |
| `gen-sql` | 35 | — | `cd tools && $(GO) tool sqlc generate -f ../services/pokedex/db/sqlc.yaml ⏎ echo "gen-sql: services/pokedex/internal/store を生成"` | 生成物を書換 |
| `gen-ts` | 40 | — | `test -x web/node_modules/.bin/openapi-typescript \|\| { echo "gen-ts: web の依存が無い(先に make web-install)" >&2; exit 1; } ⏎ cd web && npx --no-install openapi-typescript ../…` | 生成物を書換 |
| `test` | 48 | test-engine test-golden test-services test-tools test-scripts | (レシピなし) | なし(前提条件のみ) |
| `test-engine` | 51 | — | `cd engine && $(GO) test ./...` | なし(読み取り/検査) |
| `test-services` | 55 | — | `cd services && $(GO) test ./...` | なし(読み取り/検査) |
| `test-tools` | 59 | — | `cd tools && $(GO) test ./...` | なし(読み取り/検査) |
| `test-scripts` | 63 | — | `./scripts/argocd-bootstrap_test.sh` | なし(PATH 上の偽 curl/kubectl で検査。クラスタ・ネットワーク非接触) |
| `lint` | 67 | — | `test -z "$$(gofmt -l engine services tools)" \|\| { gofmt -l engine services tools; exit 1; } ⏎ cd engine && $(GO) vet ./... ⏎ cd engine && $(GO) vet -tags golden ./... ⏎ cd engine && $(GO) vet -tags allspecies ./... ⏎ cd services && $(GO) vet ./... ⏎ cd tools …` | ファイル非変更。$(MAKE) で k8s-render・check-publishable(-selftest)も再帰実行 |
| `build` | 82 | — | `cd engine && $(GO) build ./... ⏎ cd services && $(GO) build ./... ⏎ cd tools && $(GO) build ./...` | なし(読み取り/検査) |
| `golden-generate` | 88 | — | `cd tools/golden && npm ci && npm run generate` | testdata/golden/ を再生成(@smogon/calc 0.12.0。lockfile どおりに npm ci してから。ネットワークが要る) |
| `test-golden` | 92 | — | `cd engine && $(GO) test -tags golden ./... -run Golden` | なし(読み取り/検査) |
| `test-all-species` | 96 | — | `cd engine && $(GO) test -tags allspecies ./... -run AllSpecies` | なし(読み取り/検査) |
| `migrate-up` | 101 | — | `cd services && $(GO) run ./pokedex/cmd/migrate up` | **DB 書込(migrate)** |
| `migrate-version` | 105 | — | `cd services && $(GO) run ./pokedex/cmd/migrate version` | なし(読み取り/検査) |
| `migrate-down` | 109 | — | `if [ -z "$(CONFIRM_DESTROY)" ]; then \ ⏎ echo "migrate-down: CONFIRM_DESTROY=<DB名> を指定すること(全テーブルを消す破壊的操作)。人間が確認すること" >&2; \ ⏎ exit 1; \ ⏎ fi ⏎ cd services && $(GO) run…` | **DB 破壊(全テーブル削除)**。CONFIRM_DESTROY=DB名 必須 |
| `test-db` | 117 | — | `if [ -z "$(POKEDEX_TEST_DSN)" ]; then \ ⏎ echo "test-db: POKEDEX_TEST_DSN が設定されていない(スキップせず失敗する)" >&2; \ ⏎ exit 1; \ ⏎ fi ⏎ cd services && $(GO) test -tags mysql -p 1 .…` | DB へ接続してテストが書込(要 POKEDEX_TEST_DSN。未設定は失敗) |
| `db-local-up` | 125 | — | `./scripts/db-local-up.sh` | docker で mysql:9.7.2 を 127.0.0.1:3306 に起動(既存なら start)。要 .env の MYSQL_ROOT_PASSWORD |
| `up` | 130 | — | `./scripts/up.sh` | **クラスタ作成+全 apply** |
| `down` | 134 | — | `k3d cluster delete $(CLUSTER) \|\| true` | **クラスタ削除** |
| `dev` | 138 | — | `./scripts/dev.sh` | calc/gateway をホストで常駐 |
| `e2e` | 143 | — | `./scripts/e2e.sh` | なし(**スタブ**: echo のみ。P4-6 未実装。scripts/e2e.sh:4) |
| `wasm` | 150 | — | `./scripts/wasm.sh` | web/public/engine.wasm・wasm_exec.js を生成(.gitignore 済み) |
| `test-wasm` | 154 | wasm | `node scripts/wasm-conformance.mjs` | web/public に wasm を作り、node で Go との一致を検査 |
| `import` | 158 | — | `cd services && $(GO) run ./pokedex/cmd/import -data ../data` | **DB 書込(全置換)** |
| `import-dry-run` | 162 | — | `cd services && $(GO) run ./pokedex/cmd/import -data ../data -dry-run` | なし(DB 非接触・ネットワークなし) |
| `import-fetch` | 166 | — | `cd tools/importer && npm ci && node fetch.mjs` | node_modules 更新(ネットワーク), 外部ネットワーク取得 |
| `import-check-upstream` | 170 | — | `cd tools/importer && npm ci && node check-upstream.mjs` | node_modules 更新(ネットワーク), 外部ネットワーク取得 |
| `pokedex-export` | 174 | — | `cd services && $(GO) run ./pokedex/cmd/pokedex export -out ../data/generated/readmodel` | DB を読み、data/generated/readmodel に4ファイルを書込 |
| `import-k8s` | 178 | — | `current_context="$$(kubectl config current-context)"; \ ⏎ if [ "$$current_context" != "k3d-$(CLUSTER)" ]; then \ ⏎ echo "import-k8s: 現在の kubectl context '$$current_con…` | **クラスタに Job 作成**(CronJob pokedex-import から。context 検査あり) |
| `k8s-render` | 187 | — | `kubectl kustomize deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/cloud >/dev/null ⏎ echo "k8s-render: local / cloud overlay の描画を確認"` | なし(kustomize 描画のみ) |
| `assets` | 193 | — | `echo "assets: (M画像対応 で実装)"` | なし(**スタブ**: echo のみ) |
| `check-publishable` | 198 | — | `./scripts/check-publishable.sh` | なし(検査のみ) |
| `check-publishable-full` | 202 | — | `./scripts/check-publishable.sh --full` | make gen を実行し生成物の差分も検査(scripts/check-publishable.sh:354) |
| `check-publishable-selftest` | 206 | — | `./scripts/check-publishable.sh --self-test` | なし(違反を仕込んで検出を確認) |
| `fmt` | 211 | — | `gofmt -w engine services tools` | ソース整形(書換) |
| `tidy` | 215 | — | `cd engine && $(GO) mod tidy ⏎ cd services && $(GO) mod tidy ⏎ cd tools && $(GO) mod tidy` | go.mod/go.sum 書換 |
| `deps-outdated` | 221 | — | `# GOWORK=off: go.work があると workspace 全体(全モジュール合算)の一覧になってしまうため、 ⏎ # モジュール単体の一覧にする(services/balance の既存ターゲットと同じ考え方)。 ⏎ echo "== Go: engine (go list -m -u all) ==" ⏎ cd e…` | 外部ネットワーク(依存の更新確認) |

### `services/gateway/Makefile`(4 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `api-docker-build` | 13 | — | `docker build -f services/calc/Dockerfile -t pokecalc/calc:local . ⏎ docker build -f services/gateway/Dockerfile -t pokecalc/gateway:local .` | docker イメージ作成 |
| `api-k3d-deploy` | 24 | api-docker-build | `k3d image import pokecalc/calc:local pokecalc/gateway:local --cluster $(API_CLUSTER) ⏎ kubectl apply -k deploy/k8s/overlays/local-api ⏎ kubectl -n pokecalc rollout res…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `api-smoke` | 31 | — | `API_URL=$(API_URL) services/gateway/scripts/smoke.sh` | なし(HTTP のみ。DB は gateway 経由で読むだけ。balance の Ingress 有無を kubectl で確認) |
| `api-kustomize` | 34 | — | `kubectl kustomize deploy/k8s/base >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local-api >/dev/null` | なし(kustomize 描画のみ) |

### `web/Makefile`(21 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `test` | 8 | web-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | 9 | web-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | 10 | web-build | (レシピなし) | なし(前提条件の追記のみ) |
| `$(WEB_DIR)/node_modules/.package-lock.json` | 13 | $(WEB_DIR)/package-lock.json | `cd $(WEB_DIR) && npm ci --no-audit --no-fund` | node_modules 更新(ネットワーク) |
| `web-deps` | 17 | $(WEB_DIR)/node_modules/.package-lock.json | (レシピなし) | なし(前提条件のみ) |
| `web-install` | 20 | — | `cd $(WEB_DIR) && npm ci --no-audit --no-fund` | node_modules 更新(ネットワーク) |
| `web-test` | 24 | web-deps | `cd $(WEB_DIR) && npm test` | なし(読み取り/検査) |
| `web-test-wasm` | 28 | wasm web-deps | `cd $(WEB_DIR) && npm run test:wasm` | なし(読み取り/検査) |
| `web-lint` | 32 | web-deps | `cd $(WEB_DIR) && npm run typecheck && npm run lint` | なし(読み取り/検査) |
| `web-build` | 36 | web-deps | `cd $(WEB_DIR) && npm run build` | web/dist を生成(型検査+vite build+サイズ予算) |
| `web-dev` | 40 | wasm web-deps | `cd $(WEB_DIR) && npm run dev` | vite dev サーバ常駐(既定 5173)。wasm ビルドも実行 |
| `web-e2e-install` | 44 | web-deps | `cd $(WEB_DIR) && npx --no-install playwright install chromium` | chromium を取得(ネットワーク) |
| `web-e2e` | 48 | wasm web-deps | `cd $(WEB_DIR) && npm run e2e` | chromium と vite preview を一時起動(WASM でオフライン) |
| `web-e2e-online` | 52 | wasm web-deps | `cd $(WEB_DIR) && npm run e2e:online` | chromium・vite preview・calc-svc(go run)を一時起動 |
| `web-e2e-balance` | 56 | wasm web-deps | `cd $(WEB_DIR) && npm run e2e:balance` | chromium・vite preview・balance-svc(go run)を一時起動 |
| `web-docker-build` | 64 | — | `docker build -f web/Dockerfile -t pokecalc/web:local .` | docker イメージ作成 |
| `web-e2e-container` | 68 | web-docker-build web-deps | `cd $(WEB_DIR) && npm run e2e:container` | docker run で web イメージを一時起動+chromium |
| `web-k3d-deploy` | 77 | web-docker-build | `test "$$(kubectl config current-context)" = "k3d-$(CLUSTER)" \|\| { echo "web-k3d-deploy: kubectl のコンテキストが k3d-$(CLUSTER) ではない" >&2; exit 1; } ⏎ k3d image import pokecal…` | docker イメージ作成, k3d ノードへ image import, **クラスタへ apply**(overlays/local-web), Pod 再起動。context が k3d-<CLUSTER> でなければ中断 |
| `web-k3d-open` | 85 | — | `kubectl -n pokecalc port-forward svc/web 5173:80` | port-forward 常駐 |
| `web-k3d-smoke` | 89 | — | `WEB_URL=$(WEB_URL) web/scripts/k3d-smoke.sh` | なし(HTTP のみ) |
| `web-kustomize` | 93 | — | `kubectl kustomize deploy/k8s/base >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local-web >/dev/null` | なし(kustomize 描画のみ) |

### `services/balance/Makefile`(20 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `test` | 13 | balance-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | 14 | balance-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | 15 | balance-build | (レシピなし) | なし(前提条件の追記のみ) |
| `balance-gen` | 17 | — | `cd services && $(GO) tool oapi-codegen -config balance/internal/api/cfg.yaml balance/api/openapi.yaml` | 生成物を書換 |
| `balance-test` | 20 | — | `cd $(BALANCE_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `balance-lint` | 23 | — | `test -z "$$(gofmt -l $(BALANCE_DIR))" \|\| { gofmt -l $(BALANCE_DIR); exit 1; } ⏎ cd $(BALANCE_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `balance-build` | 27 | — | `cd $(BALANCE_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `balance-kustomize` | 30 | — | `kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null ⏎ kubectl kusto…` | なし(kustomize 描画のみ) |
| `balance-gitops-template-check` | 36 | balance-kustomize | `BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/check-gitops.sh template` | なし(kustomize 描画+検査。digest はプレースホルダ可) |
| `balance-gitops-check` | 39 | balance-kustomize | `BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/check-gitops.sh ready` | なし(kustomize 描画+検査。digest 確定を要求) |
| `balance-docker-build` | 42 | — | `docker build -t $(BALANCE_IMAGE) $(BALANCE_DIR)` | docker イメージ作成 |
| `balance-docker-push` | 45 | — | `BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/publish-image.sh` | **レジストリへ push**(docker buildx --push。BALANCE_RELEASE_IMAGE 必須) |
| `balance-k3d-deploy` | 48 | balance-docker-build | `k3d image import $(BALANCE_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(BALANCE_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/bal…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `balance-smoke` | 54 | — | `BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke.sh` | なし(HTTP のみ) |
| `balance-sync-typechart` | 57 | — | `cp testdata/golden/typechart.json $(BALANCE_DIR)/internal/master/data/typechart.json` | balance/internal/master/data/typechart.json を上書き |
| `balance-registry-apply` | 60 | — | `kubectl apply -k $(BALANCE_DIR)/deploy/local-registry ⏎ kubectl -n balance-registry rollout status deployment/registry --timeout=120s` | **クラスタへ apply** |
| `balance-registry-push` | 64 | — | `BALANCE_DIR=$(BALANCE_DIR) BALANCE_REGISTRY_PORT=$(BALANCE_REGISTRY_PORT) $(BALANCE_DIR)/scripts/local-registry-push.sh` | docker build/save, レジストリへ port-forward(5001)して crane push |
| `balance-argocd-app` | 67 | — | `BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/argocd-local-app.sh` | **Argo CD Application を apply** |
| `balance-k3d-deploy-readmodel` | 70 | — | `BALANCE_DIR=$(BALANCE_DIR) BALANCE_IMAGE=$(BALANCE_IMAGE) CLUSTER=$(CLUSTER) $(BALANCE_DIR)/scripts/k3d-deploy-readmodel.sh` | docker build, k3d image import, ConfigMap balance-readmodel 作成/更新, **apply**, Pod 再起動 |
| `balance-smoke-readmodel` | 73 | — | `BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke-readmodel.sh` | なし(HTTP のみ) |

### `services/speed/Makefile`(18 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `test` | 16 | speed-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | 17 | speed-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | 18 | speed-build | (レシピなし) | なし(前提条件の追記のみ) |
| `speed-gen` | 20 | — | `cd services && $(GO) tool oapi-codegen -config speed/internal/api/cfg.yaml speed/api/openapi.yaml` | 生成物を書換 |
| `speed-test` | 23 | — | `cd $(SPEED_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `speed-lint` | 26 | — | `test -z "$$(gofmt -l $(SPEED_DIR))" \|\| { gofmt -l $(SPEED_DIR); exit 1; } ⏎ cd $(SPEED_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `speed-build` | 30 | — | `cd $(SPEED_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `speed-kustomize` | 33 | — | `kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null ⏎ kubectl kustomize…` | なし(kustomize 描画のみ) |
| `speed-gitops-template-check` | 40 | speed-kustomize | `SPEED_DIR=$(SPEED_DIR) $(SPEED_DIR)/scripts/check-gitops.sh template` | なし(kustomize 描画+検査。digest はプレースホルダ可) |
| `speed-gitops-check` | 43 | speed-kustomize | `SPEED_DIR=$(SPEED_DIR) $(SPEED_DIR)/scripts/check-gitops.sh ready` | なし(kustomize 描画+検査。digest 確定を要求) |
| `speed-docker-build` | 47 | — | `docker build -f $(SPEED_DIR)/Dockerfile -t $(SPEED_IMAGE) .` | docker イメージ作成 |
| `speed-docker-push` | 50 | — | `SPEED_DIR=$(SPEED_DIR) $(SPEED_DIR)/scripts/publish-image.sh` | **レジストリへ push**(docker buildx --push。SPEED_RELEASE_IMAGE 必須) |
| `speed-registry-push` | 54 | — | `SPEED_DIR=$(SPEED_DIR) SPEED_REGISTRY_PORT=$(SPEED_REGISTRY_PORT) $(SPEED_DIR)/scripts/local-registry-push.sh` | docker build/save, レジストリへ port-forward(5002)して crane push |
| `speed-argocd-app` | 57 | — | `SPEED_DIR=$(SPEED_DIR) $(SPEED_DIR)/scripts/argocd-local-app.sh` | **Argo CD Application を apply** |
| `speed-k3d-deploy` | 60 | speed-docker-build | `k3d image import $(SPEED_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(SPEED_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/speed ⏎…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `speed-smoke` | 66 | — | `SPEED_URL=$(SPEED_URL) $(SPEED_DIR)/scripts/smoke.sh` | なし(HTTP のみ) |
| `speed-k3d-deploy-readmodel` | 70 | — | `SPEED_DIR=$(SPEED_DIR) SPEED_IMAGE=$(SPEED_IMAGE) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) CLUSTER=$(CLUSTER) $(SPEED_DIR)/scripts/k3d-deploy-readmodel.sh` | docker build, k3d image import, ConfigMap speed-readmodel 作成/更新, **apply**, Pod 再起動 |
| `speed-smoke-readmodel` | 73 | — | `SPEED_URL=$(SPEED_URL) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) $(SPEED_DIR)/scripts/smoke-readmodel.sh` | なし(HTTP のみ) |

### `services/judge/Makefile`(11 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `test` | 11 | judge-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | 12 | judge-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | 13 | judge-build | (レシピなし) | なし(前提条件の追記のみ) |
| `judge-gen` | 15 | — | `cd services && $(GO) tool oapi-codegen -config judge/internal/api/cfg.yaml judge/api/openapi.yaml` | 生成物を書換 |
| `judge-test` | 18 | — | `cd $(JUDGE_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `judge-lint` | 21 | — | `test -z "$$(gofmt -l $(JUDGE_DIR))" \|\| { gofmt -l $(JUDGE_DIR); exit 1; } ⏎ cd $(JUDGE_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `judge-build` | 25 | — | `cd $(JUDGE_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `judge-kustomize` | 28 | — | `kubectl kustomize $(JUDGE_DIR)/deploy/k8s/overlays/local >/dev/null` | なし(kustomize 描画のみ) |
| `judge-docker-build` | 32 | — | `docker build -f $(JUDGE_DIR)/Dockerfile -t $(JUDGE_IMAGE) .` | docker イメージ作成 |
| `judge-k3d-deploy` | 35 | judge-docker-build | `k3d image import $(JUDGE_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(JUDGE_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/judge ⏎…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `judge-smoke` | 41 | — | `JUDGE_URL=$(JUDGE_URL) $(JUDGE_DIR)/scripts/smoke.sh` | なし(HTTP のみ。healthz だけ。scripts/smoke.sh) |

### `ios/Makefile`(9 定義)

| ターゲット | 行 | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|---|
| `ios-gen` | 12 | — | `./ios/scripts/openapi-gen.sh` | ios/PokeCalcKit/Sources/PokeCalcAPI/Generated を書換 |
| `ios-gen-check` | 16 | — | `./ios/scripts/openapi-gen.sh --check` | なし(一時ディレクトリに生成して差分検査) |
| `ios-test` | 20 | ios-lint ios-gen-check ios-test-unit ios-test-ui ios-check-infoplist | (レシピなし) | なし(前提条件のみ) |
| `ios-lint` | 23 | — | `for script in ios/scripts/*.sh; do bash -n "$$script" \|\| exit; done` | なし |
| `ios-test-unit` | 27 | — | `cd ios/PokeCalcKit && ../scripts/run-xcode-tests.sh "ios-test-unit" \ ⏎ scheme PokeCalcKit-Package -destination "$(IOS_DESTINATION)"` | シミュレータ/xcodebuild |
| `ios-test-ui` | 32 | — | `./ios/scripts/run-xcode-tests.sh "ios-test-ui" \ ⏎ project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination "$(IOS_DESTINATION)"` | シミュレータ/xcodebuild |
| `ios-check-infoplist` | 37 | — | `./ios/scripts/check-infoplist.sh "$(IOS_DESTINATION)"` | xcodebuild build(シミュレータ向けビルド) |
| `ios-swift-test` | 41 | — | `cd ios/PokeCalcKit && swift test` | swift test(macOS。シミュレータ不要) |
| `ios-sim-run` | 45 | — | `./ios/scripts/sim-run.sh "$(IOS_SIMULATOR)" "$(IOS_SCREEN)" "$(IOS_APPEARANCE)" "$(IOS_CONTENT_SIZE)"` | シミュレータ起動, xcodebuild build, アプリのインストール・起動 |

## 件数の突き合わせ

| 項目 | 件数 |
|---|---|
| Makefile ごとの定義数(パース) | Makefile 41 / gateway 4 / web 21 / balance 20 / speed 18 / judge 11 / ios 9 = **124 定義** |
| 名前の重複(`test` `lint` `build` が 5 Makefile で定義) | 12 定義が重複 → 一意 **112 名** |
| うちファイル規則(`web/node_modules/.package-lock.json`) | 1 → 実ターゲット **111** |
| `.PHONY` 宣言の総数 | **111**(実ターゲットと一致) |
| `make -pqRr` の一意ターゲット名 | 112(ファイル規則の代わりに暗黙の `Makefile` 再生成規則を含む。差は上記 1 件のみ) |
| `##` 付き(説明あり) | 72(`make help` 表示 57 + 数字入り 15) |
| レシピなし(前提条件のみ) | 16 |

## カバレッジ

- 読んだ範囲: ルート・gateway・web・balance・speed・judge・ios の全 Makefile を全行。ルート `scripts/{up,dev,db-local-up,wasm,doctor,e2e}.sh`、`services/{balance,speed,judge}/scripts/*.sh` の冒頭・外部コマンド行、`ios/scripts/*.sh` の冒頭。
- 読めていない/浅い箇所: `scripts/check-publishable.sh` の検査項目の全体(冒頭と `--full` 部分のみ)、`services/*/scripts/smoke*.sh` と `check-gitops.sh` の判定ロジック本体、`tools/golden` の生成手順、`ios/scripts/*.sh` の内部。「副作用」はこれらの冒頭コメント・Makefile の記述に基づく。
- 未実装・スタブ: `e2e`(`scripts/e2e.sh` は echo のみ。P4-6)、`assets`(echo のみ)。
- 未検証: 実際の `make` 実行による副作用の確認はしていない(`make -pqRr` による静的な一覧のみ)。
