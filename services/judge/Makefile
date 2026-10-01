GO ?= go
CLUSTER ?= pokecalc
JUDGE_DIR ?= services/judge
JUDGE_IMAGE ?= pokecalc/judge:local
JUDGE_URL ?= http://localhost:8080
API_URL ?= http://localhost:8080
# クラスタ内レジストリは balance-registry namespace のものを共有する(ADR-0709 §1)。
# Mac 側の port-forward は balance の 5001・speed の 5002 と衝突しないよう 5003(ノード側は localhost:5000 のまま)。
JUDGE_REGISTRY_PORT ?= 5003

.PHONY: judge-gen judge-test judge-lint judge-build judge-kustomize judge-docker-build judge-k3d-deploy judge-smoke judge-gitops-template-check judge-gitops-check judge-registry-push judge-argocd-app judge-docker-push

# ルートの make test / lint / build に judge を含める(ADR-0700 §1。speed と同じ)。
# レシピを持たない前提条件の追加なので、ルートの既存ターゲットのレシピは変わらない。
test: judge-test
lint: judge-lint
build: judge-build

judge-gen:
	@cd services && $(GO) tool oapi-codegen -config judge/internal/api/cfg.yaml judge/api/openapi.yaml

judge-test:
	@cd $(JUDGE_DIR) && GOWORK=off $(GO) test ./...

judge-lint:
	@test -z "$$(gofmt -l $(JUDGE_DIR))" || { gofmt -l $(JUDGE_DIR); exit 1; }
	@cd $(JUDGE_DIR) && GOWORK=off $(GO) vet ./...

judge-build:
	@cd $(JUDGE_DIR) && GOWORK=off $(GO) build ./...

judge-kustomize:
	@kubectl kustomize $(JUDGE_DIR)/deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize $(JUDGE_DIR)/deploy/k8s/overlays/gitops >/dev/null
	@kubectl kustomize $(JUDGE_DIR)/deploy/argocd >/dev/null

# クラスタを変えない検査(placeholder のままでよい)。GitOps を適用する前は ready のほうで確かめる。
judge-gitops-template-check: judge-kustomize
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/check-gitops.sh template

judge-gitops-check: judge-kustomize
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/check-gitops.sh ready

judge-docker-push:
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/publish-image.sh

# balance レーンのクラスタ内レジストリを共有する(ADR-0709 §1)。専用の registry-apply は持たない。
judge-registry-push:
	@JUDGE_DIR=$(JUDGE_DIR) JUDGE_REGISTRY_PORT=$(JUDGE_REGISTRY_PORT) $(JUDGE_DIR)/scripts/local-registry-push.sh

judge-argocd-app:
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/argocd-local-app.sh

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0700 §1。speed と同じ形)。
judge-docker-build:
	@docker build -f $(JUDGE_DIR)/Dockerfile -t $(JUDGE_IMAGE) .

judge-k3d-deploy: judge-docker-build ## k3d へ judge を直接デプロイ(local overlay)
	@k3d image import $(JUDGE_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(JUDGE_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/judge
	@kubectl -n pokecalc rollout status deployment/judge --timeout=120s

judge-smoke:
	@JUDGE_URL=$(JUDGE_URL) API_URL=$(API_URL) $(JUDGE_DIR)/scripts/smoke.sh
