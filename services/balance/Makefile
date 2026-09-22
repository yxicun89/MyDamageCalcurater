GO ?= go
CLUSTER ?= pokecalc
BALANCE_DIR ?= services/balance
BALANCE_IMAGE ?= pokecalc/balance:local
BALANCE_URL ?= http://localhost:8080
# macOS の AirPlay 受信が 5000 を使うことがあるので、Mac 側の port-forward は 5001(ノード側は localhost:5000 のまま)
BALANCE_REGISTRY_PORT ?= 5001

.PHONY: balance-gen balance-test balance-lint balance-build balance-kustomize balance-docker-build balance-docker-push balance-k3d-deploy balance-smoke balance-gitops-template-check balance-gitops-check balance-sync-typechart balance-registry-apply balance-registry-push balance-argocd-app balance-k3d-deploy-readmodel balance-smoke-readmodel

# ルートの make test / lint / build に balance を含める(ユーザー決定 2026-09-21: テスト漏れを防ぐ)。
# レシピを持たない前提条件の追加なので、ルートの既存ターゲットのレシピは変わらない。
test: balance-test
lint: balance-lint
build: balance-build

balance-gen:
	@cd services && $(GO) tool oapi-codegen -config balance/internal/api/cfg.yaml balance/api/openapi.yaml

balance-test:
	@cd $(BALANCE_DIR) && GOWORK=off $(GO) test ./...

balance-lint:
	@test -z "$$(gofmt -l $(BALANCE_DIR))" || { gofmt -l $(BALANCE_DIR); exit 1; }
	@cd $(BALANCE_DIR) && GOWORK=off $(GO) vet ./...

balance-build:
	@cd $(BALANCE_DIR) && GOWORK=off $(GO) build ./...

balance-kustomize:
	@kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null
	@kubectl kustomize $(BALANCE_DIR)/deploy/k8s/overlays/gitops >/dev/null
	@kubectl kustomize $(BALANCE_DIR)/deploy/argocd >/dev/null

balance-gitops-template-check: balance-kustomize
	@BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/check-gitops.sh template

balance-gitops-check: balance-kustomize
	@BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/check-gitops.sh ready

balance-docker-build:
	@docker build -t $(BALANCE_IMAGE) $(BALANCE_DIR)

balance-docker-push:
	@BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/publish-image.sh

balance-k3d-deploy: balance-docker-build ## k3d へ balance を直接デプロイ
	@k3d image import $(BALANCE_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(BALANCE_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/balance
	@kubectl -n pokecalc rollout status deployment/balance --timeout=120s

balance-smoke:
	@BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke.sh

balance-sync-typechart:
	@cp testdata/golden/typechart.json $(BALANCE_DIR)/internal/master/data/typechart.json

balance-registry-apply:
	@kubectl apply -k $(BALANCE_DIR)/deploy/local-registry
	@kubectl -n balance-registry rollout status deployment/registry --timeout=120s

balance-registry-push:
	@BALANCE_DIR=$(BALANCE_DIR) BALANCE_REGISTRY_PORT=$(BALANCE_REGISTRY_PORT) $(BALANCE_DIR)/scripts/local-registry-push.sh

balance-argocd-app:
	@BALANCE_DIR=$(BALANCE_DIR) $(BALANCE_DIR)/scripts/argocd-local-app.sh

balance-k3d-deploy-readmodel:
	@BALANCE_DIR=$(BALANCE_DIR) BALANCE_IMAGE=$(BALANCE_IMAGE) CLUSTER=$(CLUSTER) $(BALANCE_DIR)/scripts/k3d-deploy-readmodel.sh

balance-smoke-readmodel:
	@BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke-readmodel.sh
