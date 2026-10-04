# pokecalc Makefile
# 各ターゲットは docs/plan/ の Phase 進行に合わせて実装を埋めていく。
# 未実装のターゲットは理由を表示して非0(終了コード 2)で終わる。成功と数えないため(issue #261・#294)。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# ツールのパスを固定(brew 導入分を確実に拾う)
export PATH := /opt/homebrew/bin:$(PATH)

GO ?= go
CLUSTER ?= pokecalc
GATEWAY_URL ?= http://localhost:8080

.PHONY: help
help: ## このヘルプを表示
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## --- 環境 -------------------------------------------------------------
.PHONY: doctor
doctor: ## 前提ツールの確認
	@./scripts/doctor.sh

## --- コード生成 -------------------------------------------------------
.PHONY: gen
gen: gen-go gen-sql gen-ts ## OpenAPI / sqlc のコード生成

.PHONY: gen-go
gen-go: ## Go サーバ/型を openapi.yaml から生成
	@cd services && $(GO) tool oapi-codegen -config internal/api/cfg.yaml ../api/openapi.yaml
	@echo "gen-go: services/internal/api/openapi.gen.go を生成"

.PHONY: gen-sql
gen-sql: ## pokedex の DB 行の型・クエリを sqlc から生成(ADR-0100 §1)
	@cd tools && $(GO) tool sqlc generate -f ../services/pokedex/db/sqlc.yaml
	@echo "gen-sql: services/pokedex/internal/store を生成"

.PHONY: gen-ts
gen-ts: ## TypeScript 型を openapi.yaml から生成(web/src/api/openapi.gen.ts・balance.gen.ts。要 make web-install)
	@test -x web/node_modules/.bin/openapi-typescript || { echo "gen-ts: web の依存が無い(先に make web-install)" >&2; exit 1; }
	@cd web && npx --no-install openapi-typescript ../api/openapi.yaml -o src/api/openapi.gen.ts >/dev/null && npx --no-install prettier --write src/api/openapi.gen.ts >/dev/null
	@cd web && npx --no-install openapi-typescript ../services/balance/api/openapi.yaml -o src/api/balance.gen.ts >/dev/null && npx --no-install prettier --write src/api/balance.gen.ts >/dev/null
	@echo "gen-ts: web/src/api/openapi.gen.ts・web/src/api/balance.gen.ts を生成"

## --- テスト -----------------------------------------------------------
.PHONY: test
test: test-engine test-golden test-services test-tools test-scripts ## 全ユニットテスト(実装済みGoモジュール・engine のゴールデン・ルート scripts/ のシェル)

.PHONY: test-engine
test-engine: ## engine のユニットテスト
	@cd engine && $(GO) test ./...

.PHONY: test-services
test-services: ## services のユニットテスト
	@cd services && $(GO) test ./...

.PHONY: test-tools
test-tools:
	@cd tools && $(GO) test ./...
	@node --test tools/importer/showdown-cache.test.mjs tools/importer/pokeapi-csv.test.mjs tools/importer/prune.test.mjs tools/importer/integrity.test.mjs tools/importer/fetch-integrity.test.mjs tools/importer/fetch-snapshot-shape.test.mjs tools/importer/fetch-showdown-megastone.test.mjs
	@[ -d tools/assets/node_modules ] || (cd tools/assets && npm ci --silent)
	@node --test tools/assets/convert.test.mjs

.PHONY: test-scripts
test-scripts: ## ルート scripts/ のシェルスクリプトのテスト(Argo CD 導入 ADR-0405・監視スタック導入 ADR-0406・計算API SLO ADR-0407・ルートの e2e ADR-0306・GitOps の AppProject/共通スクリプト/レジストリ ADR-0408・Makefile の help と未実装ターゲット。クラスタ・ネットワークに触らない)
	@./scripts/argocd-bootstrap_test.sh
	@./scripts/observability-bootstrap_test.sh
	@./scripts/observability-slo_test.sh
	@./scripts/e2e_test.sh
	@./scripts/ai-guard/bash-guard_test.sh
	@./scripts/pr-merge_test.sh
	@./scripts/gitops_test.sh
	@./scripts/make-targets_test.sh
	@./scripts/check-master-version_test.sh
	@./scripts/master-release_test.sh
	@./scripts/require-k3d-context_test.sh
	@./scripts/image-tag_test.sh
	@./scripts/k3d-deploy-tagged_test.sh
	@./scripts/up-secrets_test.sh
	@./scripts/images-k3d_test.sh
	@./scripts/k3d-m2-deploy_test.sh
	@./scripts/tidb-operator-bootstrap_test.sh
	@./scripts/test-db-docker_test.sh
	@./scripts/db-backup_test.sh
	@./scripts/db-restore_test.sh
	@./scripts/check-plan_test.sh
	@./scripts/db-backup-k3d_test.sh

.PHONY: lint
lint: ## gofmt / go vet / shell・Node構文チェック
	@test -z "$$(gofmt -l engine services tools)" || { gofmt -l engine services tools; exit 1; }
	@cd engine && $(GO) vet ./...
	@cd engine && $(GO) vet -tags golden ./...
	@cd engine && $(GO) vet -tags allspecies ./...
	@cd services && $(GO) vet ./...
	@cd services && $(GO) vet -tags mysql ./pokedex/...
	@cd services && $(GO) vet -tags tidb ./record/... ./team/...
	@cd services && $(GO) vet -tags nats ./calc/...
	@cd tools && $(GO) vet ./...
	@$(MAKE) --no-print-directory staticcheck
	@scripts/check-conflict-markers.sh
	@scripts/check-plan.sh
	@for script in scripts/*.sh; do bash -n "$$script" || exit; done
	@for script in tools/importer/*.sh; do sh -n "$$script" || exit; done
	@node --check tools/golden/generate.mjs
	@node --check scripts/wasm-conformance.mjs
	@for script in tools/importer/*.mjs; do node --check "$$script" || exit; done
	@for script in tools/assets/*.mjs; do node --check "$$script" || exit; done
	@$(MAKE) --no-print-directory k8s-render
	@$(MAKE) --no-print-directory check-publishable
	@$(MAKE) --no-print-directory check-publishable-selftest

# staticcheck(tools/go.mod の tool ディレクティブで版固定。issue #331)。警告が1件でもあれば失敗する。
# ビルドタグ付きのファイルも解析する(vet と同じタグ)。balance・speed・judge は別モジュールのため GOWORK=off。
# engine はタグ無しだと golden 専用の定数を未使用と誤検知するため、タグ付きだけ解析する。
# 生成コードは staticcheck.conf の checks で外す(各モジュールのルートに置く)。
.PHONY: staticcheck
staticcheck: ## staticcheck(固定版)を全モジュール・全ビルドタグで実行
	@sc="$$(cd tools && $(GO) tool -n staticcheck)" && \
	cd engine && "$$sc" -tags golden ./... && "$$sc" -tags allspecies ./... && cd ../services && \
	"$$sc" ./... && "$$sc" -tags mysql ./pokedex/... && "$$sc" -tags tidb ./record/... ./team/... && "$$sc" -tags nats ./calc/... && cd ../tools && \
	"$$sc" ./... && cd ../services/balance && GOWORK=off "$$sc" ./... && cd ../speed && GOWORK=off "$$sc" ./... && cd ../judge && GOWORK=off "$$sc" ./...

.PHONY: build
build: ## 実装済みGoモジュールをビルド(Web/WASMは後続タスク)
	@cd engine && $(GO) build ./...
	@cd services && $(GO) build ./...
	@cd tools && $(GO) build ./...

.PHONY: golden-generate
golden-generate: ## 外部実装の期待値を再生成(package-lock.json どおりに npm ci してから。ネットワークが要る)
	@cd tools/golden && npm ci && npm run generate

.PHONY: test-golden
test-golden: ## engine のゴールデンテスト(@smogon/calc 照合)
	@cd engine && $(GO) test -tags golden ./... -run Golden

.PHONY: test-all-species
test-all-species: ## 全ポケモン網羅・性質テスト
	@cd engine && $(GO) test -tags allspecies ./... -run AllSpecies

## --- pokedex DB(migrate。ADR-0100 §5) ---------------------------------
.PHONY: migrate-up
migrate-up: ## pokedex の DB を最新版まで migrate する(POKEDEX_DATABASE_DSN が必須)
	@cd services && $(GO) run ./pokedex/cmd/migrate up

.PHONY: migrate-version
migrate-version: ## pokedex の migrate バージョンを表示する(POKEDEX_DATABASE_DSN が必須)
	@cd services && $(GO) run ./pokedex/cmd/migrate version

.PHONY: migrate-down
migrate-down: ## pokedex の DB を全て戻す(破壊的。CONFIRM_DESTROY=<DB名> が必須。人間の確認)
	@if [ -z "$(CONFIRM_DESTROY)" ]; then \
		echo "migrate-down: CONFIRM_DESTROY=<DB名> を指定すること(全テーブルを消す破壊的操作)。人間が確認すること" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) run ./pokedex/cmd/migrate down -confirm "$(CONFIRM_DESTROY)"

.PHONY: migrate-force
migrate-force: ## pokedex の dirty を解いて版を FORCE_VERSION にする(FORCE_VERSION=<版> CONFIRM_FORCE=<DB名> が必須。人間の確認。docs/runbooks/data.md)
	@if [ -z "$(FORCE_VERSION)" ] || [ -z "$(CONFIRM_FORCE)" ]; then \
		echo "migrate-force: FORCE_VERSION=<版> CONFIRM_FORCE=<DB名> を指定すること(migration の状態を書き換える操作)。手順は docs/runbooks/data.md。人間が確認すること" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) run ./pokedex/cmd/migrate force -version "$(FORCE_VERSION)" -confirm "$(CONFIRM_FORCE)"

## --- record/team DB(migrate。ADR-0211 §4・§5) ------------------------
.PHONY: migrate-up-record
migrate-up-record: ## record の DB を最新版まで migrate する(RECORD_DATABASE_DSN が必須)
	@cd services && $(GO) run ./record/cmd/migrate up

.PHONY: migrate-version-record
migrate-version-record: ## record の migrate バージョンを表示する(RECORD_DATABASE_DSN が必須)
	@cd services && $(GO) run ./record/cmd/migrate version

.PHONY: migrate-down-record
migrate-down-record: ## record の DB を全て戻す(破壊的。CONFIRM_DESTROY=<DB名> が必須。人間の確認)
	@if [ -z "$(CONFIRM_DESTROY)" ]; then \
		echo "migrate-down-record: CONFIRM_DESTROY=<DB名> を指定すること(全テーブルを消す破壊的操作)。人間が確認すること" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) run ./record/cmd/migrate down -confirm "$(CONFIRM_DESTROY)"

.PHONY: migrate-up-team
migrate-up-team: ## team の DB を最新版まで migrate する(TEAM_DATABASE_DSN が必須)
	@cd services && $(GO) run ./team/cmd/migrate up

.PHONY: migrate-version-team
migrate-version-team: ## team の migrate バージョンを表示する(TEAM_DATABASE_DSN が必須)
	@cd services && $(GO) run ./team/cmd/migrate version

.PHONY: migrate-down-team
migrate-down-team: ## team の DB を全て戻す(破壊的。CONFIRM_DESTROY=<DB名> が必須。人間の確認)
	@if [ -z "$(CONFIRM_DESTROY)" ]; then \
		echo "migrate-down-team: CONFIRM_DESTROY=<DB名> を指定すること(全テーブルを消す破壊的操作)。人間が確認すること" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) run ./team/cmd/migrate down -confirm "$(CONFIRM_DESTROY)"

.PHONY: test-db
test-db: ## pokedex(MySQL)・record/team(TiDB)のDBを使うテスト(POKEDEX_TEST_DSN・RECORD_TEST_DSN・TEAM_TEST_DSN が必須。make test には含めない。ADR-0211)
	@if [ -z "$(POKEDEX_TEST_DSN)" ]; then \
		echo "test-db: POKEDEX_TEST_DSN が設定されていない(スキップせず失敗する)" >&2; \
		exit 1; \
	fi
	@if [ -z "$(RECORD_TEST_DSN)" ]; then \
		echo "test-db: RECORD_TEST_DSN が設定されていない(スキップせず失敗する)" >&2; \
		exit 1; \
	fi
	@if [ -z "$(TEAM_TEST_DSN)" ]; then \
		echo "test-db: TEAM_TEST_DSN が設定されていない(スキップせず失敗する)" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) test -tags mysql -p 1 ./pokedex/...
	@cd services && $(GO) test -tags tidb -p 1 ./record/... ./team/...

.PHONY: test-db-docker
test-db-docker: ## test-db を Docker の使い捨て MySQL・TiDB で流す(終了時に消す。Docker が無ければ失敗。make test には含めない。issue #223)
	@./scripts/test-db-docker.sh

.PHONY: db-backup
db-backup: ## DB のバックアップ(MODE=full|journal KIND=pokedex|record|team。接続は DB_HOST/DB_PORT/DB_USER/DB_NAME と MYSQL_PWD。docs/runbooks/data.md。ADR-0225)
	@./scripts/db-backup.sh "$(or $(MODE),full)" "$(KIND)"

.PHONY: db-restore
db-restore: ## DB の復元(KIND=… GEN=<世代|latest>。上書きなので CONFIRM_RESTORE=<DB名> が必須。サービスを止めてから。ADR-0225)
	@./scripts/db-restore.sh "$(KIND)" "$(or $(GEN),latest)"

.PHONY: db-backup-k3d
db-backup-k3d: ## k3d の実 DB(record・team・pokedex)をバックアップ(KIND で絞れる。Secret はスクリプト内部で読み値を出さない。非破壊。ADR-0227)
	@./scripts/db-backup-k3d.sh $(KIND)

.PHONY: db-restore-drill-k3d
db-restore-drill-k3d: ## k3d の実 TiDB で復元訓練(別名 DB へ最新世代を復元→行数・墓石を照合→別名 DB を削除。稼働中の DB は上書きしない。ADR-0227)
	@./scripts/db-restore-drill-k3d.sh $(KIND)

.PHONY: test-db-backup
test-db-backup: ## バックアップ→復元の実 DB 往復テスト(Docker の使い捨て TiDB・MySQL。Docker が無ければ失敗。make test には含めない。ADR-0225)
	@./scripts/db-backup-restore_docker_test.sh

.PHONY: test-nats
test-nats: ## calc-svcのイベント発行を実NATSで検査する(CALC_TEST_NATS_URL が必須。make test には含めない。ADR-0212)
	@if [ -z "$(CALC_TEST_NATS_URL)" ]; then \
		echo "test-nats: CALC_TEST_NATS_URL が設定されていない(スキップせず失敗する)" >&2; \
		exit 1; \
	fi
	@cd services && $(GO) test -tags nats -p 1 ./calc/internal/events/...

.PHONY: db-local-up
db-local-up: ## make dev 用に docker で mysql:9.7.2 を 127.0.0.1:3306 に起動する(パスワードは .env)
	@./scripts/db-local-up.sh

.PHONY: tidb-local-up
tidb-local-up: ## make dev 用に tiup playground で TiDB v8.5.8 を 127.0.0.1:4000 に起動し record・team の DB を作る(ADR-0211 §2)
	@./scripts/tidb-local-up.sh

.PHONY: nats-local-up
nats-local-up: ## make dev 用に docker で NATS v2.15.0(JetStream 有効)を 127.0.0.1:4222 に起動する(ADR-0212 §2)
	@./scripts/nats-local-up.sh

## --- クラスタ / ローカル ---------------------------------------------
.PHONY: up
up: ## k3d クラスタ作成 + 全デプロイ
	@./scripts/up.sh

.PHONY: deploy-latest
deploy-latest: ## いまのチェックアウトで全サービスを作り直して k3d へ入れ替える(make up 済みが前提。動作確認の前に毎回)
	@./scripts/k3d-deploy-latest.sh

.PHONY: down
down: ## k3d クラスタ削除
	@k3d cluster delete $(CLUSTER) || true

.PHONY: dev
dev: ## k8s を使わずローカルで全サービス起動
	@./scripts/dev.sh

## --- e2e / iOS --------------------------------------------------------
.PHONY: e2e
e2e: ## 常時3件のPlaywright(k3d不要)+ k3d-<CLUSTER>検出時にスモーク+Playwright3件を追加(ADR-0306)
	@CLUSTER=$(CLUSTER) ./scripts/e2e.sh

# ios-test などの iOS のターゲットは ios/Makefile(末尾で include)

## --- ビルド / データ --------------------------------------------------
.PHONY: wasm
wasm: ## engine を WASM にビルドして web/public へ
	@./scripts/wasm.sh

.PHONY: test-wasm
test-wasm: wasm ## Go と WASM の結果一致テスト(Node。要 make wasm)
	@node scripts/wasm-conformance.mjs

.PHONY: import
import: ## マスタデータの変換・投入(POKEDEX_DATABASE_DSN が必須。取得は import-fetch)
	@cd services && $(GO) run ./pokedex/cmd/import -data ../data

.PHONY: import-dry-run
import-dry-run: ## マスタデータの変換・報告だけ行う(DB には触らない)
	@cd services && $(GO) run ./pokedex/cmd/import -data ../data -dry-run

.PHONY: import-fetch
import-fetch: ## 取得元(calc/Showdown/PokeAPI)から実データを取得する(ネットワークが要る。先に tools/importer で npm ci)
	@cd tools/importer && npm ci && node fetch.mjs

.PHONY: import-check-upstream
import-check-upstream: ## 上流(calc/Showdown/PokeAPI)の最新版を検出して報告する(ネットワークが要る。取り込みはしない)
	@cd tools/importer && npm ci && node check-upstream.mjs

.PHONY: check-master-version
check-master-version: ## calc・balance・speed が export した read model と同じ dataVersion で動いているか確かめる(読み取りだけ。要 k3d の context。ADR-0135)
	@./scripts/check-master-version.sh

.PHONY: master-release
master-release: ## マスタ更新を calc・balance・speed へ反映する(import Job 完了待ち→export 検証→変化なしなら終了→入れ替え・rollout→版一致→smoke。要 k3d の context。ADR-0135)
	@./scripts/master-release.sh

.PHONY: pokedex-export-k3d
pokedex-export-k3d: ## k3d の mysql から read model を data/generated/readmodel/ に書く(port-forward と DSN の取得を内部で行う。make up・import 済みが前提)
	@./scripts/pokedex-export-local.sh

.PHONY: pokedex-export
pokedex-export: ## balance/speed 向けの read model を6ファイル(4ファイル+type-chart.json・metadata.json)書く(POKEDEX_DATABASE_DSN が必須。出力先 data/generated/readmodel/)
	@cd services && $(GO) run ./pokedex/cmd/pokedex export -out ../data/generated/readmodel

.PHONY: import-k8s
import-k8s: ## k3d 上の CronJob pokedex-import を手動で1回流す(週1回の定期実行とは別に)
	@CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh import-k8s
	@kubectl -n pokecalc create job --from=cronjob/pokedex-import "pokedex-import-manual-$$(date +%Y%m%d%H%M%S)"

.PHONY: pokedex-registry-push
pokedex-registry-push: ## pokedex(server イメージ)をクラスタ内共有レジストリ balance-registry へ digest 固定で push する(タイプバランスレーン issue #237 の依頼。ADR-0018・ADR-0605 と同じ方式)
	@./scripts/pokedex-registry-push.sh

.PHONY: k8s-render
# 各レーンの overlay も描画する(issue #261・#321)。どれか1つでも描画できなければ lint を失敗させる。
# local/api・local/web・local/mysql・local/nats は Component なので、local・local-api・local-web の描画で確かめる。
k8s-render: k8s-render-kubectl api-kustomize web-kustomize balance-kustomize speed-kustomize judge-kustomize ## kustomize で全レーンの overlay(local・cloud・tidb・local-api・local-web・balance・speed・judge・observability・Argo CD の AppProject)が描画できることを確かめる(apply はしない)
	@kubectl kustomize deploy/k8s/overlays/cloud >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local/tidb >/dev/null
	@kubectl kustomize deploy/argocd >/dev/null
	@kubectl kustomize deploy/k8s/base/observability >/dev/null
	@if kubectl cluster-info --request-timeout=3s >/dev/null 2>&1; then \
		kubectl apply --dry-run=client --request-timeout=10s -f deploy/k8s/base/record/job-migrate.yaml -o yaml >/dev/null; \
		kubectl apply --dry-run=client --request-timeout=10s -f deploy/k8s/base/team/job-migrate.yaml -o yaml >/dev/null; \
		echo "k8s-render: 全レーンの overlay・record/team migrate Job の描画を確認"; \
	else \
		echo "k8s-render: 全レーンの overlay の描画を確認(クラスタ未起動のため record/team migrate Job の dry-run はスキップ)"; \
	fi

# kubectl が無いと各 *-kustomize が分かりにくいエラーで止まるため、先に理由を出して止める(issue #75)。
.PHONY: k8s-render-kubectl
k8s-render-kubectl:
	@command -v kubectl >/dev/null 2>&1 || { echo "k8s-render: kubectl が無いため overlay を描画できません(brew install kubectl。make doctor で確認)" >&2; exit 1; }

.PHONY: assets
assets: ## 手元の画像(data/generated/images/src)を WebP 2サイズ + manifest に変換する(画像なしでも成功。ADR-0807。ASSETS_SRC・ASSETS_OUT で場所を変更)
	@[ -d tools/assets/node_modules ] || (cd tools/assets && npm ci --silent)
	@cd tools/assets && node convert.mjs

.PHONY: images-k3d
images-k3d: ## make assets の出力(data/generated/images/dist)を k3d のノードへ置き、gateway の /images/* で配信する(make up 済み。画像が無ければ何もせず成功。ADR-0807)
	@./scripts/images-k3d.sh

## --- 公開前の検査 -----------------------------------------------------
.PHONY: check-publishable
check-publishable: ## 公開前の検査(絶対パス・秘密・追跡禁止ファイル・第三者データ。速い)
	@./scripts/check-publishable.sh

.PHONY: check-publishable-full
check-publishable-full: ## 上記 + Git 作者情報・make gen の差分(遅い)
	@./scripts/check-publishable.sh --full

.PHONY: check-publishable-selftest
check-publishable-selftest: ## 公開前の検査の自己テスト(違反を検出できることの確認)
	@./scripts/check-publishable.sh --self-test

## --- 補助 -------------------------------------------------------------
.PHONY: fmt
fmt: ## gofmt
	@gofmt -w engine services tools

.PHONY: tidy
tidy: ## go mod tidy(全モジュール)
	@cd engine && $(GO) mod tidy
	@cd services && $(GO) mod tidy
	@cd tools && $(GO) mod tidy
	@cd services/balance && GOWORK=off $(GO) mod tidy
	@cd services/speed && GOWORK=off $(GO) mod tidy
	@cd services/judge && GOWORK=off $(GO) mod tidy

.PHONY: deps-outdated
deps-outdated: ## 古くなった依存の一覧を表示する(ネットワーク使用。失敗しても一覧を出す。make test には含めない)
	# GOWORK=off: go.work があると workspace 全体(全モジュール合算)の一覧になってしまうため、
	# モジュール単体の一覧にする(services/balance の既存ターゲットと同じ考え方)。
	@echo "== Go: engine (go list -m -u all) =="
	@cd engine && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Go: services (go list -m -u all) =="
	@cd services && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Go: tools (go list -m -u all) =="
	@cd tools && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Go: services/balance (go list -m -u all) =="
	@cd services/balance && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Go: services/speed (go list -m -u all) =="
	@cd services/speed && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Go: services/judge (go list -m -u all) =="
	@cd services/judge && GOWORK=off $(GO) list -m -u all 2>&1 || true
	@echo "== Node: tools/golden (npm outdated) =="
	@if [ -f tools/golden/package.json ]; then cd tools/golden && (npm outdated || true); else echo "(tools/golden/package.json が無い)"; fi
	@echo "== Node: tools/importer (npm outdated) =="
	@if [ -f tools/importer/package.json ]; then cd tools/importer && (npm outdated || true); else echo "(tools/importer/package.json が無い。未作成)"; fi
	@echo "== Node: web (npm outdated) =="
	@if [ -f web/package.json ]; then cd web && (npm outdated || true); else echo "(web/package.json が無い。未作成)"; fi

include services/balance/Makefile
include services/speed/Makefile
include services/judge/Makefile
include services/gateway/Makefile
include web/Makefile
include ios/Makefile
include apps/wishlist/Makefile
