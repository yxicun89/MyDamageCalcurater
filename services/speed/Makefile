GO ?= go
CLUSTER ?= pokecalc
SPEED_DIR ?= services/speed
SPEED_IMAGE ?= pokecalc/speed:local
SPEED_URL ?= http://localhost:8080

.PHONY: speed-gen speed-test speed-lint speed-build speed-kustomize speed-docker-build speed-k3d-deploy speed-smoke

# ルートの make test / lint / build に speed を含める(ADR-0600 §2。balance と同じ)。
# レシピを持たない前提条件の追加なので、ルートの既存ターゲットのレシピは変わらない。
test: speed-test
lint: speed-lint
build: speed-build

speed-gen:
	@cd services && $(GO) tool oapi-codegen -config speed/internal/api/cfg.yaml speed/api/openapi.yaml

speed-test:
	@cd $(SPEED_DIR) && GOWORK=off $(GO) test ./...

speed-lint:
	@test -z "$$(gofmt -l $(SPEED_DIR))" || { gofmt -l $(SPEED_DIR); exit 1; }
	@cd $(SPEED_DIR) && GOWORK=off $(GO) vet ./...

speed-build:
	@cd $(SPEED_DIR) && GOWORK=off $(GO) build ./...

# GitOps の overlay と Argo CD Application は SP4 で足す(ADR-0600 §2)。
speed-kustomize:
	@kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local >/dev/null

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0600 §2)。
speed-docker-build:
	@docker build -f $(SPEED_DIR)/Dockerfile -t $(SPEED_IMAGE) .

speed-k3d-deploy: speed-docker-build ## k3d へ speed を直接デプロイ(local overlay。GitOps は SP4)
	@k3d image import $(SPEED_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(SPEED_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/speed
	@kubectl -n pokecalc rollout status deployment/speed --timeout=120s

speed-smoke:
	@SPEED_URL=$(SPEED_URL) $(SPEED_DIR)/scripts/smoke.sh
