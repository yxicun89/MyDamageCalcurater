# pokecalc Makefile
# 各ターゲットは docs/plan.md の Phase 進行に合わせて実装を埋めていく。
# 未実装のターゲットは理由を表示して正常終了する(ビルドを壊さない)。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# ツールのパスを固定(brew 導入分を確実に拾う)
export PATH := /opt/homebrew/bin:$(PATH)

GO ?= go
CLUSTER ?= pokecalc
GATEWAY_URL ?= http://localhost:8080

.PHONY: help
help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## --- 環境 -------------------------------------------------------------
.PHONY: doctor
doctor: ## 前提ツールの確認
	@./scripts/doctor.sh

## --- コード生成 -------------------------------------------------------
.PHONY: gen
gen: gen-go gen-ts ## OpenAPI / sqlc のコード生成

.PHONY: gen-go
gen-go: ## Go サーバ/型を openapi.yaml から生成
	@cd services && $(GO) tool oapi-codegen -config internal/api/cfg.yaml ../api/openapi.yaml
	@echo "gen-go: services/internal/api/openapi.gen.go を生成"

.PHONY: gen-ts
gen-ts: ## TypeScript 型を openapi.yaml から生成
	@if [ -f web/package.json ]; then \
		echo "gen-ts: (P4 以降で openapi-typescript を実装)"; \
	else echo "gen-ts: web 未作成"; fi

## --- テスト -----------------------------------------------------------
.PHONY: test
test: test-engine test-services test-tools ## 全ユニットテスト(実装済みGoモジュール)

.PHONY: test-engine
test-engine: ## engine のユニットテスト
	@cd engine && $(GO) test ./...

.PHONY: test-services
test-services: ## services のユニットテスト
	@cd services && $(GO) test ./...

.PHONY: test-tools
test-tools:
	@cd tools && $(GO) test ./...

.PHONY: lint
lint: ## gofmt / go vet / shell・Node構文チェック
	@test -z "$$(gofmt -l engine services tools)" || { gofmt -l engine services tools; exit 1; }
	@cd engine && $(GO) vet ./...
	@cd services && $(GO) vet ./...
	@cd tools && $(GO) vet ./...
	@for script in scripts/*.sh; do bash -n "$$script" || exit; done
	@node --check tools/golden/generate.mjs
	@node --check scripts/wasm-conformance.mjs
	@$(MAKE) --no-print-directory check-publishable

.PHONY: build
build: ## 実装済みGoモジュールをビルド(Web/WASMは後続タスク)
	@cd engine && $(GO) build ./...
	@cd services && $(GO) build ./...
	@cd tools && $(GO) build ./...

.PHONY: golden-generate
golden-generate: ## npm ci後に外部実装の期待値を再生成
	@cd tools/golden && npm run generate

.PHONY: test-golden
test-golden: ## engine のゴールデンテスト(@smogon/calc 照合)
	@cd engine && $(GO) test -tags golden ./... -run Golden

.PHONY: test-all-species
test-all-species: ## 全ポケモン網羅・性質テスト
	@cd engine && $(GO) test -tags allspecies ./... -run AllSpecies

## --- クラスタ / ローカル ---------------------------------------------
.PHONY: up
up: ## k3d クラスタ作成 + 全デプロイ
	@./scripts/up.sh

.PHONY: down
down: ## k3d クラスタ削除
	@k3d cluster delete $(CLUSTER) || true

.PHONY: dev
dev: ## k8s を使わずローカルで全サービス起動
	@./scripts/dev.sh

## --- e2e / iOS --------------------------------------------------------
.PHONY: e2e
e2e: ## k3d 上のスモーク + Playwright
	@./scripts/e2e.sh

.PHONY: ios-test
ios-test: ## iOS シミュレータでテスト
	@echo "ios-test: (M3 で実装)"

## --- ビルド / データ --------------------------------------------------
.PHONY: wasm
wasm: ## engine を WASM にビルドして web/public へ
	@./scripts/wasm.sh

.PHONY: test-wasm
test-wasm: wasm ## Go と WASM の結果一致テスト(Node。要 make wasm)
	@node scripts/wasm-conformance.mjs

.PHONY: import
import: ## マスタデータ取込
	@echo "import: (P2 で実装)"

.PHONY: assets
assets: ## 画像を WebP 2サイズに変換して MinIO へ
	@echo "assets: (M画像対応 で実装)"

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

include services/balance/Makefile
include web/Makefile
