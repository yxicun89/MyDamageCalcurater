GO ?= go
CLUSTER ?= pokecalc
JUDGE_DIR ?= services/judge
JUDGE_IMAGE ?= pokecalc/judge:local
JUDGE_URL ?= http://localhost:8080

.PHONY: judge-gen judge-test judge-lint judge-build judge-kustomize judge-docker-build judge-k3d-deploy judge-smoke

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

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0700 §1。speed と同じ形)。
judge-docker-build:
	@docker build -f $(JUDGE_DIR)/Dockerfile -t $(JUDGE_IMAGE) .

judge-k3d-deploy: judge-docker-build ## k3d へ judge を直接デプロイ(local overlay)
	@k3d image import $(JUDGE_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(JUDGE_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/judge
	@kubectl -n pokecalc rollout status deployment/judge --timeout=120s

judge-smoke:
	@JUDGE_URL=$(JUDGE_URL) $(JUDGE_DIR)/scripts/smoke.sh
