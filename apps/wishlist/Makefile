# wishlist(apps/wishlist/CLAUDE.md)のターゲット。ルートの Makefile から include する(パスはリポジトリ直下基準)。
#   make wishlist-test / wishlist-lint / wishlist-build / wishlist-kustomize
# ルートの go.work には入れず GOWORK=off で単独モジュールとして扱う(docs/design.md W-01)。

GO ?= go
WISHLIST_DIR ?= apps/wishlist
WISHLIST_API_DIR ?= $(WISHLIST_DIR)/api

.PHONY: wishlist-test wishlist-lint wishlist-build wishlist-kustomize

# ルートの make test / lint / build に wishlist を含める(CI で回すため。ユーザー決定 2026-10-03)。
test: wishlist-test
lint: wishlist-lint
build: wishlist-build

wishlist-test:
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) test ./...

wishlist-lint: wishlist-kustomize
	@test -z "$$(gofmt -l $(WISHLIST_API_DIR))" || { gofmt -l $(WISHLIST_API_DIR); exit 1; }
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) vet ./...
	@sc="$$(cd tools && $(GO) tool -n staticcheck)" && cd $(WISHLIST_API_DIR) && GOWORK=off "$$sc" ./...
	@bash -n $(WISHLIST_DIR)/scripts/*.sh

wishlist-build:
	@cd $(WISHLIST_API_DIR) && GOWORK=off $(GO) build ./...

wishlist-kustomize:
	@kubectl kustomize $(WISHLIST_DIR)/deploy/k8s/overlays/local >/dev/null
