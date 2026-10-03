# wishlist(apps/wishlist/CLAUDE.md)のターゲット。ルートの Makefile から include する(パスはリポジトリ直下基準)。
#   make wishlist-test / wishlist-lint / wishlist-build / wishlist-kustomize / wishlist-gen / wishlist-test-mysql
#   make wishlist-docker-build / wishlist-web-docker-build / wishlist-k3d-deploy
#   make wishlist-ios-gen / wishlist-ios-gen-check / wishlist-ios-sync-testdata / wishlist-ios-test / wishlist-ios-xcode-test(iOS。CI の wishlist-test には含めない)
# ルートの go.work には入れず GOWORK=off で単独モジュールとして扱う(docs/design.md W-01)。

GO ?= go
WISHLIST_DIR ?= apps/wishlist
WISHLIST_API_DIR ?= $(WISHLIST_DIR)/api
WISHLIST_WEB_DIR ?= $(WISHLIST_DIR)/web
WISHLIST_IOS_DIR ?= $(WISHLIST_DIR)/ios
WISHLIST_IOS_SIMULATOR ?= iPhone 18 Pro

CLUSTER ?= pokecalc
WISHLIST_API_IMAGE ?= wishlist/api:local
WISHLIST_WEB_IMAGE ?= wishlist/web:local

.PHONY: wishlist-web-deps wishlist-web-test wishlist-web-lint wishlist-web-build wishlist-web-gen wishlist-test wishlist-lint wishlist-build wishlist-kustomize wishlist-gen wishlist-test-mysql wishlist-docker-build wishlist-web-docker-build wishlist-k3d-deploy wishlist-ios-gen wishlist-ios-gen-check wishlist-ios-sync-testdata wishlist-ios-sync-check wishlist-ios-lint wishlist-ios-test wishlist-ios-xcode-test

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
	@bash -n $(WISHLIST_IOS_DIR)/scripts/*.sh

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

wishlist-web-docker-build: ## web のイメージを作る(wishlist/web:local)
	@docker build -t $(WISHLIST_WEB_IMAGE) $(WISHLIST_WEB_DIR)

# k3d(make up 済み)へ api と web を入れて overlays/local を適用する。migrate の Job は spec が不変なので消してから作り直す。
wishlist-k3d-deploy: wishlist-docker-build wishlist-web-docker-build
	@CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh wishlist-k3d-deploy
	@k3d image import $(WISHLIST_API_IMAGE) $(WISHLIST_WEB_IMAGE) -c $(CLUSTER)
	@kubectl -n wishlist delete job wishlist-migrate --ignore-not-found
	@kubectl apply -k $(WISHLIST_DIR)/deploy/k8s/overlays/local

# ---- iOS(SwiftUI。apps/wishlist/ios。ダメ計の ios-* と同じ構成)----
# CI は Linux なので wishlist-test / wishlist-lint(構文だけ)/ wishlist-build には iOS を含めない(ダメ計 iOS と同じ扱い)。
# Xcode が無い環境は wishlist-ios-test(macOS の swift test)までは動く。xcodebuild が要るのは wishlist-ios-xcode-test だけ。

# api/openapi.yaml から WishlistKit/Sources/WishlistAPI/Generated を生成する(生成物はコミットする)。
wishlist-ios-gen:
	@$(WISHLIST_IOS_DIR)/scripts/openapi-gen.sh

# コミット済みの生成物が openapi.yaml と一致するか。
wishlist-ios-gen-check:
	@$(WISHLIST_IOS_DIR)/scripts/openapi-gen.sh --check

# testdata/query-cases.json を Swift のテストリソースへコピーする(シンボリックリンクにしない)。
wishlist-ios-sync-testdata:
	@$(WISHLIST_IOS_DIR)/scripts/sync-testdata.sh

wishlist-ios-sync-check:
	@$(WISHLIST_IOS_DIR)/scripts/sync-testdata.sh --check

wishlist-ios-lint:
	@for script in $(WISHLIST_IOS_DIR)/scripts/*.sh; do bash -n "$$script" || exit; done

# WishlistKit のテストを macOS で実行する(シミュレータ不要)。
wishlist-ios-test: wishlist-ios-sync-check wishlist-ios-lint
	@. $(WISHLIST_IOS_DIR)/scripts/xcode-env.sh && cd $(WISHLIST_IOS_DIR)/WishlistKit && swift test

# xcodebuild(Kit のテスト + アプリの XCUITest)。Xcode が無ければ「Xcode が要る」と表示して終了コード 2(成功扱いにしない)。
wishlist-ios-xcode-test:
	@$(WISHLIST_IOS_DIR)/scripts/xcode-test.sh "$(WISHLIST_IOS_SIMULATOR)"
