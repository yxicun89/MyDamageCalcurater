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
	@if [ -f api/openapi.yaml ]; then \
		echo "gen-go: (P0-3 以降で oapi-codegen を実装)"; \
	else echo "gen-go: api/openapi.yaml が未作成"; fi

.PHONY: gen-ts
gen-ts: ## TypeScript 型を openapi.yaml から生成
	@if [ -f web/package.json ]; then \
		echo "gen-ts: (P4 以降で openapi-typescript を実装)"; \
	else echo "gen-ts: web 未作成"; fi

## --- テスト -----------------------------------------------------------
.PHONY: test
test: test-engine test-services ## 全ユニットテスト

.PHONY: test-engine
test-engine: ## engine のユニットテスト
	@cd engine && $(GO) test ./...

.PHONY: test-services
test-services: ## services のユニットテスト
	@cd services && $(GO) test ./...

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

.PHONY: import
import: ## マスタデータ取込
	@echo "import: (P2 で実装)"

.PHONY: assets
assets: ## 画像を WebP 2サイズに変換して MinIO へ
	@echo "assets: (M画像対応 で実装)"

## --- 補助 -------------------------------------------------------------
.PHONY: fmt
fmt: ## gofmt
	@gofmt -w engine services tools

.PHONY: tidy
tidy: ## go mod tidy(全モジュール)
	@cd engine && $(GO) mod tidy
	@cd services && $(GO) mod tidy
	@cd tools && $(GO) mod tidy
