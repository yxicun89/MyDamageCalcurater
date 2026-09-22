GO ?= go
CLUSTER ?= pokecalc
JUDGE_DIR ?= services/judge
JUDGE_IMAGE ?= pokecalc/judge:local
JUDGE_URL ?= http://localhost:8080

# Kustomize・Docker・smoke のターゲット(judge-kustomize / judge-docker-build / judge-k3d-deploy /
# judge-smoke)は、deploy/k8s と Dockerfile ができた段階で足す(ADR-0700 §1。speed の Makefile と同じ形)。
.PHONY: judge-gen judge-test judge-lint judge-build

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
