# wishlist(apps/wishlist/CLAUDE.md)のターゲット。ルートの Makefile から include する(パスはリポジトリ直下基準)。
#   make wishlist-test / wishlist-lint / wishlist-build / wishlist-kustomize / wishlist-gen / wishlist-test-mysql
#   make wishlist-docker-build / wishlist-k3d-deploy
# ルートの go.work には入れず GOWORK=off で単独モジュールとして扱う(docs/design.md W-01)。

GO ?= go
WISHLIST_DIR ?= apps/wishlist
WISHLIST_API_DIR ?= $(WISHLIST_DIR)/api
WISHLIST_WEB_DIR ?= $(WISHLIST_DIR)/web

CLUSTER ?= pokecalc
WISHLIST_API_IMAGE ?= wishlist/api:local

.PHONY: wishlist-web-deps wishlist-web-test wishlist-web-lint wishlist-web-build wishlist-web-gen wishlist-test wishlist-lint wishlist-build wishlist-kustomize wishlist-gen wishlist-test-mysql wishlist-docker-build wishlist-k3d-deploy

# ルートの make test / lint / build に wishlist を含める(CI で回すため。ユーザー決定 2026-10-03)。
test: wishlist-test
lint: wishlist-lint
build: wishlist-build

# npm ci が書く node_modules/.package-lock.json を目印にし、package-lock.json の方が新しいときだけ入れ直す(web/Makefile と同じ方式)。
$(WISHLIST_WEB_DIR)/node_modules/.package-lock.json: $(WISHLIST_WEB_DIR)/package-lock.json
	@cd $(WISHLIST_WEB_DIR) && npm ci --no-audit --no-fund

wishlist-web-deps: $(WISHLIST_WEB_DIR)/node_modules/.package-lock.json

wishlist-web-test: wishlist-web-deps
	@cd $(WISHLIST_WEB_DIR) && npm test

wishlist-web-lint: wishlist-web-deps
	@cd $(WISHLIST_WEB_DIR) && npm run typecheck && npm run lint

wishlist-web-build: wishlist-web-deps
	@cd $(WISHLIST_WEB_DIR) && npm run build

# api/openapi.yaml から web/src/api/schema.d.ts を生成する(生成物はコミットする)。
wishlist-web-gen: wishlist-web-deps
	@cd $(WISHLIST_WEB_DIR) && npm run gen

wishlist-test: wishlist-web-test
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) test ./...

wishlist-lint: wishlist-kustomize wishlist-web-lint
	@test -z "$$(gofmt -l $(WISHLIST_API_DIR))" || { gofmt -l $(WISHLIST_API_DIR); exit 1; }
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) vet ./...
	@sc="$$(cd tools && $(GO) tool -n staticcheck)" && cd $(WISHLIST_API_DIR) && GOWORK=off "$$sc" ./...
	@bash -n $(WISHLIST_DIR)/scripts/*.sh

wishlist-build: wishlist-web-build
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) build ./...

wishlist-kustomize:
	@kubectl kustomize $(WISHLIST_DIR)/deploy/k8s/overlays/local >/dev/null

# OpenAPI(oapi-codegen)と sqlc の生成。既存の services・tools モジュールの go tool を変更せずに使う(docs/design.md W-01)。
# 生成物(internal/api/openapi.gen.go・internal/store/*.go)はコミットする(W-02)。
wishlist-gen: wishlist-web-gen
	@cd services && GOWORK=off $(GO) tool oapi-codegen -config ../$(WISHLIST_API_DIR)/internal/api/cfg.yaml ../$(WISHLIST_API_DIR)/openapi.yaml
	@cd tools && GOWORK=off $(GO) tool sqlc generate -f ../$(WISHLIST_API_DIR)/internal/store/sqlc.yaml
	@echo "wishlist-gen: $(WISHLIST_API_DIR)/internal/api/openapi.gen.go・internal/store を生成"

# MySQL 実装の契約テスト(-tags mysql)。WISHLIST_TEST_DSN が無ければ docker の mysql:9.7.2 を使い捨てで起動する。
wishlist-test-mysql:
	@$(WISHLIST_DIR)/scripts/test-mysql.sh

wishlist-docker-build: ## api のイメージを作る(wishlist/api:local)
	@docker build -t $(WISHLIST_API_IMAGE) $(WISHLIST_API_DIR)

# k3d(make up 済み)へ api を入れて overlays/local を適用する。migrate の Job は spec が不変なので消してから作り直す。
# web のイメージ(wishlist/web:local)は Web の実装まで無く、web の Pod は ImagePullBackOff のまま(既知)。
wishlist-k3d-deploy: wishlist-docker-build
	@CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh wishlist-k3d-deploy
	@k3d image import $(WISHLIST_API_IMAGE) -c $(CLUSTER)
	@kubectl -n wishlist delete job wishlist-migrate --ignore-not-found
	@kubectl apply -k $(WISHLIST_DIR)/deploy/k8s/overlays/local
