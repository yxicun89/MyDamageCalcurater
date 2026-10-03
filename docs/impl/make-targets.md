# Make ターゲット一覧

- 基準: 2026-10-01 の `origin/main`(PR #443 まで)。行番号は書かない(陳腐化するため)。ターゲット名は `Makefile` で `grep -n '^<名前>:' Makefile` すれば探せる。件数は `make help | wc -l` で数える。
- 読み方: 「前提」= Makefile の前提条件。「副作用」の**太字**はクラスタ・DB・レジストリ・破壊的操作。コマンド単位の実行順は [runbook-commands.md](runbook-commands.md)、リソースは [k8s-local.md](k8s-local.md)。
- ルート `Makefile` は末尾で 6 本を `include` する(balance・speed・judge・gateway・web・ios)。ターゲット名は接頭辞でレーン分離(`api-`=gateway/calc、`web-`、`balance-`、`speed-`、`judge-`、`ios-`)。
- 未実装のターゲット(`make assets`)は、理由を出して終了コード 2 で終わる(成功と数えない。issue #261・#294)。

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
| `SHELL` | `/usr/bin/env bash` | `Makefile` 冒頭 |
| `PATH` | `/opt/homebrew/bin` を先頭に追加 | `Makefile` 冒頭 |
| 変数の既定 | `GO=go` `CLUSTER=pokecalc` `GATEWAY_URL=http://localhost:8080`(`GATEWAY_URL` は Makefile 内で未使用) | `Makefile` 冒頭 |
| 既定ゴール | `help` | `Makefile` 冒頭 |
| `help` の出し方 | 全 Makefile(`include` 先を含む)の `## ` 付きターゲットを、左列にターゲット名で出す。数字入りの名前も出る | `scripts/make-targets_test.sh`(`make test-scripts`)が固定 |
| `test`/`lint`/`build` | 複数 Makefile が**前提条件だけ追記**(レシピを持たない)。結果は下表 | 各 Makefile 冒頭 |
| k3d 対象の安全弁 | kubectl の context が `k3d-$(CLUSTER)` でなければ中断するのは、`up`(`scripts/up.sh`)・`import-k8s`・`deploy-latest`・`pokedex-registry-push`(いずれも `scripts/require-k3d-context.sh`)と `web-k3d-deploy`(`web/Makefile` の自前の検査)。`api-k3d-deploy`・`balance-k3d-deploy`・`speed-k3d-deploy`・`judge-k3d-deploy` はまだ検査しない(各レーンへ依頼済み。docs/ai-shared/DECISIONS.md 2026-10-01) | `scripts/require-k3d-context.sh`、`scripts/require-k3d-context_test.sh` |
| イメージのタグ | ローカルでビルドするイメージのタグは `scripts/image-tag.sh`(git の短いコミット+未コミットなら `-dirty`)で出せる | `scripts/image-tag.sh`、`scripts/image-tag_test.sh` |

### `test` / `lint` / `build` の合成結果(`make -pqRr` で確認)

| ターゲット | 前提条件(全 Makefile の合算) | 追加のレシピ |
|---|---|---|
| `test` | test-engine test-golden test-services test-tools test-scripts balance-test speed-test judge-test web-test | なし |
| `lint` | speed-lint judge-lint web-lint balance-lint | あり(gofmt・vet(タグ付きも)・shell/Node の構文検査・`k8s-render`・`check-publishable`・`check-publishable-selftest`) |
| `build` | speed-build judge-build web-build balance-build | あり(engine・services・tools の go build) |
| `gen` | gen-go gen-sql gen-ts balance-gen speed-gen judge-gen | なし |

注意: `api-` ターゲット(calc・gateway)は `test`/`lint`/`build` に前提を足さない(ユニットテストは `test-services` に含まれる。`services/gateway/Makefile:1-6`)。iOS も含まれない(`ios-test` は別)。

## 全ターゲット

### `Makefile`(ルート)

| ターゲット | 前提 | 実行内容 | 副作用 |
|---|---|---|---|
| `help` | — | 全 Makefile の `## ` 付きターゲットを一覧表示 | なし |
| `doctor` | — | `scripts/doctor.sh` | なし(読み取り/検査) |
| `gen` | gen-go gen-sql gen-ts balance-gen speed-gen judge-gen | (レシピなし)。生成物は Git に置かない(ADR-0171)。各 gen-* は出力が無いか入力が新しいときだけ生成(`scripts/ensure-gen.sh stale`)。強制は `GEN_FORCE=1` | なし(前提条件のみ) |
| `gen-go` | — | oapi-codegen で `api/openapi.yaml` から Go サーバ/型を生成 | 生成物を書換(Git 管理外) |
| `gen-sql` | — | sqlc で pokedex の DB 行の型・クエリを生成 | 生成物を書換(Git 管理外) |
| `gen-ts` | web-deps | `cd web && npm run gen`(openapi-typescript で web の4つの `*.gen.ts` を生成。Web の pre* フックと同じ) | 生成物を書換(Git 管理外) |
| `gen-clean` | — | `scripts/ensure-gen.sh list` の生成物を削除(iOS の生成物は消さない) | 生成物を削除 |
| `gen-go-all` | gen-go gen-sql balance-gen speed-gen judge-gen | (レシピなし)。Go をビルドするターゲット(`build`・`lint`・`test-services`・`staticcheck`・`test-db*`・`migrate-*`・`import*`・`up`・`dev`・`e2e` 等)の前提 | なし(前提条件のみ) |
| `test` | test-engine test-golden test-services test-tools test-scripts | (レシピなし) | なし(前提条件のみ) |
| `test-engine` | — | `cd engine && go test ./...` | なし |
| `test-services` | — | `cd services && go test ./...` | なし |
| `test-tools` | — | `cd tools && go test ./...` と importer の `node --test`(showdown-cache・pokeapi-csv・prune) | なし |
| `test-scripts` | — | ルート `scripts/` のテスト(argocd-bootstrap・observability-bootstrap・observability-slo・e2e・gitops・make-targets・require-k3d-context・image-tag・up-secrets・test-db-docker) | なし(偽 kubectl・偽 make で検査。クラスタ・ネットワーク非接触) |
| `lint` | — | gofmt・go vet(`golden`・`allspecies`・`mysql`・`tidb`・`nats` タグも)・shell と Node の構文検査、`k8s-render`・`check-publishable`・`check-publishable-selftest` を再帰実行 | ファイル非変更 |
| `build` | — | engine・services・tools の `go build` | なし |
| `golden-generate` | — | `cd tools/golden && npm ci && npm run generate` | testdata/golden/ を再生成(@smogon/calc 0.12.0。ネットワークが要る) |
| `test-golden` | — | `cd engine && go test -tags golden ./... -run Golden` | なし |
| `test-all-species` | — | `cd engine && go test -tags allspecies ./... -run AllSpecies` | なし |
| `migrate-up` | — | pokedex の `cmd/migrate up`(`POKEDEX_PROVISION_DSN` などがあればプロビジョニングも) | **DB 書込(migrate)** |
| `migrate-version` | — | `cmd/migrate version` | なし |
| `migrate-down` | — | `cmd/migrate down -confirm`(`CONFIRM_DESTROY=<DB名>` 必須) | **DB 破壊(全テーブル削除)**。人間の確認 |
| `migrate-force` | — | `cmd/migrate force -version -confirm`(`FORCE_VERSION`・`CONFIRM_FORCE=<DB名>` 必須) | **DB 書込(`schema_migrations` の版と dirty)**。人間の確認 |
| `migrate-up-record` `migrate-version-record` `migrate-down-record` | — | record の DB(TiDB)に対する同じ操作(`RECORD_DATABASE_DSN` が必須。down は `CONFIRM_DESTROY`) | up・down は DB 書込 |
| `migrate-up-team` `migrate-version-team` `migrate-down-team` | — | team の DB(TiDB)に対する同じ操作(`TEAM_DATABASE_DSN` が必須) | up・down は DB 書込 |
| `test-db` | — | `POKEDEX_TEST_DSN`・`RECORD_TEST_DSN`・`TEAM_TEST_DSN` が必須(未設定は**失敗**)。`-tags mysql` の pokedex と `-tags tidb` の record/team を流す | DB へ接続してテストが書込 |
| `test-db-docker` | — | `scripts/test-db-docker.sh`: Docker で使い捨ての MySQL・TiDB を起動して `test-db` を流し、終わったら(失敗・中断でも)消す。Docker が無ければ失敗 | docker コンテナ・ネットワークの作成と削除(使い捨てのみ)。`make test` には含まれない |
| `test-nats` | — | `CALC_TEST_NATS_URL` が必須(未設定は失敗)。calc のイベント発行を実 NATS で検査 | NATS へ接続 |
| `db-local-up` | — | `scripts/db-local-up.sh` | docker で mysql:9.7.2 を 127.0.0.1:3306 に起動(既存なら start)。要 .env の MYSQL_ROOT_PASSWORD |
| `tidb-local-up` | — | `scripts/tidb-local-up.sh` | tiup playground で TiDB を 127.0.0.1:4000 に起動し record・team の DB を作る |
| `nats-local-up` | — | `scripts/nats-local-up.sh` | docker で NATS を 127.0.0.1:4222 に起動 |
| `up` | — | `scripts/up.sh`(context が `k3d-$(CLUSTER)` でなければ中断。Secret は `kubectl create`) | **クラスタ作成+全 apply**、Secret 作成、イメージ build と import |
| `deploy-latest` | — | `scripts/k3d-deploy-latest.sh`: migrate-up → pokedex・importer・calc・gateway・web・judge・balance・speed の入れ替え | **クラスタへ apply・Pod 再起動・DB migrate**(`make up` 済みが前提。context 検査あり)。read model が無いと balance・speed を飛ばして非0 |
| `down` | — | `k3d cluster delete $(CLUSTER)` | **クラスタ削除**(PVC・Secret も消える。人間の確認) |
| `dev` | — | `scripts/dev.sh` | calc/gateway をホストで常駐 |
| `e2e` | — | `scripts/e2e.sh`: web-e2e・web-e2e-online・web-e2e-balance を常に実行し、kubectl の context が `k3d-$(CLUSTER)` のときだけ api-smoke・web-k3d-smoke・web-k3d-e2e も実行(ADR-0306)。`E2E_REQUIRE_K3D=1` で k3d が無ければ失敗 | chromium・vite preview・go run を一時起動 |
| `wasm` | — | `scripts/wasm.sh` | web/public/engine.wasm・wasm_exec.js を生成(.gitignore 済み) |
| `test-wasm` | wasm | `node scripts/wasm-conformance.mjs` | web/public に wasm を作り、node で Go との一致を検査 |
| `import` | — | `cd services && go run ./pokedex/cmd/import -data ../data` | **DB 書込(全置換)** |
| `import-dry-run` | — | 同上に `-dry-run` | なし(DB 非接触・ネットワークなし)。`data/generated/reports/` に報告を書く |
| `import-fetch` | — | `cd tools/importer && npm ci && node fetch.mjs` | node_modules 更新(ネットワーク)、外部ネットワーク取得 |
| `import-check-upstream` | — | `cd tools/importer && npm ci && node check-upstream.mjs` | node_modules 更新(ネットワーク)、外部ネットワーク取得 |
| `pokedex-export` | — | `cd services && go run ./pokedex/cmd/pokedex export -out ../data/generated/readmodel`(要 `POKEDEX_DATABASE_DSN`) | DB を読み、`data/generated/readmodel` に6ファイル(4ファイル+`type-chart.json`・`metadata.json`。ADR-0128)を書込 |
| `import-k8s` | — | `scripts/require-k3d-context.sh` の後、CronJob `pokedex-import` から手動 Job `pokedex-import-manual-<時刻>` を作る(完了は待たない) | **クラスタに Job 作成**(context 検査あり) |
| `pokedex-registry-push` | — | `scripts/pokedex-registry-push.sh` | docker build(--target server)/save、balance-registry へ port-forward(5003)して crane push(context 検査あり。**クラスタ内レジストリへ push**) |
| `k8s-render` | k8s-render-kubectl api-kustomize web-kustomize balance-kustomize speed-kustomize judge-kustomize | cloud・local/tidb・`deploy/argocd` も `kubectl kustomize` で描画。クラスタがあれば record/team の migrate Job を `--dry-run=client` で確認 | なし(描画のみ) |
| `k8s-render-kubectl` | — | kubectl が無ければ理由を出して失敗(`k8s-render` の前提) | なし |
| `assets` | — | **未実装**。理由を出して終了コード 2 で終わる(画像の配信は計画外。issue #286) | なし |
| `check-publishable` | — | `scripts/check-publishable.sh`(絶対パス・秘密・追跡禁止ファイル・第三者データ。ADR-0130) | なし(検査のみ) |
| `check-publishable-full` | — | `scripts/check-publishable.sh --full` | `make gen` が成功し生成物が揃うこと・Git 作者情報も検査(遅い) |
| `check-publishable-selftest` | — | `scripts/check-publishable.sh --self-test` | なし(違反を仕込んで検出を確認) |
| `fmt` | — | `gofmt -w engine services tools` | ソース整形(書換) |
| `tidy` | — | 全 Go モジュール(engine・services・tools・balance・speed・judge)の `go mod tidy` | go.mod/go.sum 書換 |
| `deps-outdated` | — | 全モジュールの更新可能な依存を一覧(`GOWORK=off`) | 外部ネットワーク(依存の更新確認) |

### `services/gateway/Makefile`(4 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `api-docker-build` | — | `docker build -f services/calc/Dockerfile -t pokecalc/calc:local . ⏎ docker build -f services/gateway/Dockerfile -t pokecalc/gateway:local .` | docker イメージ作成 |
| `api-k3d-deploy` | api-docker-build | `k3d image import pokecalc/calc:local pokecalc/gateway:local --cluster $(API_CLUSTER) ⏎ kubectl apply -k deploy/k8s/overlays/local-api ⏎ kubectl -n pokecalc rollout res…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `api-smoke` | — | `API_URL=$(API_URL) services/gateway/scripts/smoke.sh` | なし(HTTP のみ。DB は gateway 経由で読むだけ。balance の Ingress 有無を kubectl で確認) |
| `api-kustomize` | — | `kubectl kustomize deploy/k8s/base >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local-api >/dev/null` | なし(kustomize 描画のみ) |

### `web/Makefile`(21 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `test` | web-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | web-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | web-build | (レシピなし) | なし(前提条件の追記のみ) |
| `$(WEB_DIR)/node_modules/.package-lock.json` | $(WEB_DIR)/package-lock.json | `cd $(WEB_DIR) && npm ci --no-audit --no-fund` | node_modules 更新(ネットワーク) |
| `web-deps` | $(WEB_DIR)/node_modules/.package-lock.json | (レシピなし) | なし(前提条件のみ) |
| `web-install` | — | `cd $(WEB_DIR) && npm ci --no-audit --no-fund` | node_modules 更新(ネットワーク) |
| `web-test` | web-deps | `cd $(WEB_DIR) && npm test` | なし(読み取り/検査) |
| `web-test-wasm` | wasm web-deps | `cd $(WEB_DIR) && npm run test:wasm` | なし(読み取り/検査) |
| `web-lint` | web-deps | `cd $(WEB_DIR) && npm run typecheck && npm run lint` | なし(読み取り/検査) |
| `web-build` | web-deps | `cd $(WEB_DIR) && npm run build` | web/dist を生成(型検査+vite build+サイズ予算) |
| `web-dev` | wasm web-deps | `cd $(WEB_DIR) && npm run dev` | vite dev サーバ常駐(既定 5173)。wasm ビルドも実行 |
| `web-e2e-install` | web-deps | `cd $(WEB_DIR) && npx --no-install playwright install chromium` | chromium を取得(ネットワーク) |
| `web-e2e` | wasm web-deps | `cd $(WEB_DIR) && npm run e2e` | chromium と vite preview を一時起動(WASM でオフライン) |
| `web-e2e-online` | wasm web-deps | `cd $(WEB_DIR) && npm run e2e:online` | chromium・vite preview・calc-svc(go run)を一時起動 |
| `web-e2e-balance` | wasm web-deps | `cd $(WEB_DIR) && npm run e2e:balance` | chromium・vite preview・balance-svc(go run)を一時起動 |
| `web-docker-build` | — | `docker build -f web/Dockerfile -t pokecalc/web:local .` | docker イメージ作成 |
| `web-e2e-container` | web-docker-build web-deps | `cd $(WEB_DIR) && npm run e2e:container` | docker run で web イメージを一時起動+chromium |
| `web-k3d-deploy` | web-docker-build | `test "$$(kubectl config current-context)" = "k3d-$(CLUSTER)" \|\| { echo "web-k3d-deploy: kubectl のコンテキストが k3d-$(CLUSTER) ではない" >&2; exit 1; } ⏎ k3d image import pokecal…` | docker イメージ作成, k3d ノードへ image import, **クラスタへ apply**(overlays/local-web), Pod 再起動。context が k3d-<CLUSTER> でなければ中断 |
| `web-k3d-open` | — | `kubectl -n pokecalc port-forward svc/web 5173:80` | port-forward 常駐 |
| `web-k3d-smoke` | — | `WEB_URL=$(WEB_URL) web/scripts/k3d-smoke.sh` | なし(HTTP のみ) |
| `web-kustomize` | — | `kubectl kustomize deploy/k8s/base >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize deploy/k8s/overlays/local-web >/dev/null` | なし(kustomize 描画のみ) |

### `services/balance/Makefile`(20 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `test` | balance-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | balance-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | balance-build | (レシピなし) | なし(前提条件の追記のみ) |
| `balance-gen` | — | 出力が無いか入力が新しいとき `cd services && $(GO) tool oapi-codegen -config balance/internal/api/cfg.yaml balance/api/openapi.yaml`(`balance-test`・`-lint`・`-build`・`-docker-*`・`-registry-push` の前提) | 生成物を書換(Git 管理外) |
| `balance-test` | — | `cd $(BALANCE_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `balance-lint` | — | `test -z "$$(gofmt -l $(BALANCE_DIR))" \|\| { gofmt -l $(BALANCE_DIR); exit 1; } ⏎ cd $(BALANCE_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `balance-build` | — | `cd $(BALANCE_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `balance-kustomize` | — | `kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null ⏎ kubectl kusto…` | なし(kustomize 描画のみ) |
| `balance-gitops-template-check` | balance-kustomize | `SERVICE=balance scripts/gitops/check-gitops.sh template` | なし(kustomize 描画+検査。digest はプレースホルダ可) |
| `balance-gitops-check` | balance-kustomize | `SERVICE=balance scripts/gitops/check-gitops.sh ready` | なし(kustomize 描画+検査。digest 確定を要求) |
| `balance-docker-build` | — | `docker build -t $(BALANCE_IMAGE) $(BALANCE_DIR)` | docker イメージ作成 |
| `balance-docker-push` | — | `SERVICE=balance scripts/gitops/publish-image.sh` | **レジストリへ push**(docker buildx --push。BALANCE_RELEASE_IMAGE 必須) |
| `balance-k3d-deploy` | balance-docker-build | `k3d image import $(BALANCE_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(BALANCE_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/bal…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `balance-smoke` | — | `BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke.sh` | なし(HTTP のみ) |
| `balance-sync-typechart` | — | `cp testdata/golden/typechart.json $(BALANCE_DIR)/internal/master/data/typechart.json` | balance/internal/master/data/typechart.json を上書き |
| `balance-registry-apply` | — | `kubectl apply -k $(BALANCE_DIR)/deploy/local-registry ⏎ kubectl -n balance-registry rollout status deployment/registry --timeout=120s` | **クラスタへ apply**(PVC `registry-data` を含む。ADR-0408 §3) |
| `balance-registry-push` | — | `SERVICE=balance BALANCE_REGISTRY_PORT=$(BALANCE_REGISTRY_PORT) scripts/gitops/local-registry-push.sh` | docker build/save, レジストリへ port-forward(5001)して crane push |
| `balance-argocd-app` | — | `SERVICE=balance scripts/gitops/argocd-local-app.sh` | **AppProject `pokecalc` と Argo CD Application を apply**(AppProject が先。ADR-0408 §1・§2) |
| `balance-k3d-deploy-readmodel` | — | `SERVICE=balance BALANCE_IMAGE=$(BALANCE_IMAGE) CLUSTER=$(CLUSTER) scripts/gitops/k3d-deploy-readmodel.sh` | docker build, k3d image import, ConfigMap balance-readmodel 作成/更新, **apply**, Pod 再起動 |
| `balance-smoke-readmodel` | — | `BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke-readmodel.sh` | なし(HTTP のみ) |

### `services/speed/Makefile`(18 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `test` | speed-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | speed-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | speed-build | (レシピなし) | なし(前提条件の追記のみ) |
| `speed-gen` | — | 出力が無いか入力が新しいとき `cd services && $(GO) tool oapi-codegen -config speed/internal/api/cfg.yaml speed/api/openapi.yaml`(`speed-test`・`-lint`・`-build`・`-docker-*`・`-registry-push` の前提) | 生成物を書換(Git 管理外) |
| `speed-test` | — | `cd $(SPEED_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `speed-lint` | — | `test -z "$$(gofmt -l $(SPEED_DIR))" \|\| { gofmt -l $(SPEED_DIR); exit 1; } ⏎ cd $(SPEED_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `speed-build` | — | `cd $(SPEED_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `speed-kustomize` | — | `kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local >/dev/null ⏎ kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null ⏎ kubectl kustomize…` | なし(kustomize 描画のみ) |
| `speed-gitops-template-check` | speed-kustomize | `SERVICE=speed scripts/gitops/check-gitops.sh template` | なし(kustomize 描画+検査。digest はプレースホルダ可) |
| `speed-gitops-check` | speed-kustomize | `SERVICE=speed scripts/gitops/check-gitops.sh ready` | なし(kustomize 描画+検査。digest 確定を要求) |
| `speed-docker-build` | — | `docker build -f $(SPEED_DIR)/Dockerfile -t $(SPEED_IMAGE) .` | docker イメージ作成 |
| `speed-docker-push` | — | `SERVICE=speed scripts/gitops/publish-image.sh` | **レジストリへ push**(docker buildx --push。SPEED_RELEASE_IMAGE 必須) |
| `speed-registry-push` | — | `SERVICE=speed SPEED_REGISTRY_PORT=$(SPEED_REGISTRY_PORT) scripts/gitops/local-registry-push.sh` | docker build/save, レジストリへ port-forward(5002)して crane push |
| `speed-argocd-app` | — | `SERVICE=speed scripts/gitops/argocd-local-app.sh` | **AppProject `pokecalc` と Argo CD Application を apply**(AppProject が先。ADR-0408 §1・§2) |
| `speed-k3d-deploy` | speed-docker-build | `k3d image import $(SPEED_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(SPEED_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/speed ⏎…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `speed-smoke` | — | `SPEED_URL=$(SPEED_URL) $(SPEED_DIR)/scripts/smoke.sh` | なし(HTTP のみ) |
| `speed-k3d-deploy-readmodel` | — | `SERVICE=speed SPEED_IMAGE=$(SPEED_IMAGE) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) CLUSTER=$(CLUSTER) scripts/gitops/k3d-deploy-readmodel.sh` | docker build, k3d image import, ConfigMap speed-readmodel 作成/更新, **apply**, Pod 再起動 |
| `speed-smoke-readmodel` | — | `SPEED_URL=$(SPEED_URL) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) $(SPEED_DIR)/scripts/smoke-readmodel.sh` | なし(HTTP のみ) |

### `services/judge/Makefile`(11 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `test` | judge-test | (レシピなし) | なし(前提条件の追記のみ) |
| `lint` | judge-lint | (レシピなし) | なし(前提条件の追記のみ) |
| `build` | judge-build | (レシピなし) | なし(前提条件の追記のみ) |
| `judge-gen` | — | 出力が無いか入力が新しいとき `cd services && $(GO) tool oapi-codegen -config judge/internal/api/cfg.yaml judge/api/openapi.yaml`(`judge-test`・`-lint`・`-build`・`-docker-*`・`-registry-push` の前提) | 生成物を書換(Git 管理外) |
| `judge-test` | — | `cd $(JUDGE_DIR) && GOWORK=off $(GO) test ./...` | なし(読み取り/検査) |
| `judge-lint` | — | `test -z "$$(gofmt -l $(JUDGE_DIR))" \|\| { gofmt -l $(JUDGE_DIR); exit 1; } ⏎ cd $(JUDGE_DIR) && GOWORK=off $(GO) vet ./...` | なし |
| `judge-build` | — | `cd $(JUDGE_DIR) && GOWORK=off $(GO) build ./...` | なし(読み取り/検査) |
| `judge-kustomize` | — | `kubectl kustomize $(JUDGE_DIR)/deploy/k8s/overlays/local >/dev/null` | なし(kustomize 描画のみ) |
| `judge-docker-build` | — | `docker build -f $(JUDGE_DIR)/Dockerfile -t $(JUDGE_IMAGE) .` | docker イメージ作成 |
| `judge-k3d-deploy` | judge-docker-build | `k3d image import $(JUDGE_IMAGE) --cluster $(CLUSTER) ⏎ kubectl apply -k $(JUDGE_DIR)/deploy/k8s/overlays/local ⏎ kubectl -n pokecalc rollout restart deployment/judge ⏎…` | k3d ノードへ image import, **クラスタへ apply**, Pod 再起動 |
| `judge-smoke` | — | `JUDGE_URL=$(JUDGE_URL) $(JUDGE_DIR)/scripts/smoke.sh` | なし(HTTP のみ。healthz だけ。scripts/smoke.sh) |

### `ios/Makefile`(9 定義)

| ターゲット | 前提 | 実行内容(レシピ要約) | 副作用 |
|---|---|---|---|
| `ios-gen` | — | `./ios/scripts/openapi-gen.sh` | ios/PokeCalcKit/Sources/PokeCalcAPI/Generated を書換 |
| `ios-gen-check` | — | `./ios/scripts/openapi-gen.sh --check` | なし(一時ディレクトリに生成して差分検査) |
| `ios-test` | ios-lint ios-gen-check ios-test-unit ios-test-ui ios-check-infoplist | (レシピなし) | なし(前提条件のみ) |
| `ios-lint` | — | `for script in ios/scripts/*.sh; do bash -n "$$script" \|\| exit; done` | なし |
| `ios-test-unit` | — | `cd ios/PokeCalcKit && ../scripts/run-xcode-tests.sh "ios-test-unit" \ ⏎ scheme PokeCalcKit-Package -destination "$(IOS_DESTINATION)"` | シミュレータ/xcodebuild |
| `ios-test-ui` | — | `./ios/scripts/run-xcode-tests.sh "ios-test-ui" \ ⏎ project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination "$(IOS_DESTINATION)"` | シミュレータ/xcodebuild |
| `ios-check-infoplist` | — | `./ios/scripts/check-infoplist.sh "$(IOS_DESTINATION)"` | xcodebuild build(シミュレータ向けビルド) |
| `ios-swift-test` | — | `cd ios/PokeCalcKit && swift test` | swift test(macOS。シミュレータ不要) |
| `ios-sim-run` | — | `./ios/scripts/sim-run.sh "$(IOS_SIMULATOR)" "$(IOS_SCREEN)" "$(IOS_APPEARANCE)" "$(IOS_CONTENT_SIZE)"` | シミュレータ起動, xcodebuild build, アプリのインストール・起動 |

## 数え方

- 全ターゲット名: `make help`(説明つきだけ)。説明の無いものも含めるなら `make -pqRr | grep -E '^[a-zA-Z0-9_-]+:' | sort -u`。
- 実行せずにレシピを見る: `make -n <ターゲット>`(副作用のあるものは実行しない)。

## カバレッジ

- 読んだ範囲: ルート・gateway・web・balance・speed・judge・ios の Makefile、ルート `scripts/{up,k3d-deploy-latest,require-k3d-context,image-tag,e2e,test-db-docker}.sh`。ルートの表は 2026-10-01 に Makefile から作り直した。レーン別の表(gateway 以降)は、行番号の列を外した以外は 2026-09-24 の内容のまま。
- 読めていない/浅い箇所: `scripts/check-publishable.sh` の検査項目の全体、`services/*/scripts/smoke*.sh` と `check-gitops.sh` の判定ロジック本体、`ios/scripts/*.sh` の内部。「副作用」はこれらの冒頭コメント・Makefile の記述に基づく。
- 未実装・スタブ: `assets`(終了コード 2)。
- 未検証: 実際の `make` 実行による副作用の確認はしていない(Makefile の静的な読み取りと、`make help`・`make -n` による確認のみ)。
