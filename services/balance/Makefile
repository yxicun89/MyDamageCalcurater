GO ?= go
CLUSTER ?= pokecalc
BALANCE_DIR ?= services/balance
BALANCE_IMAGE ?= pokecalc/balance:local
BALANCE_URL ?= http://localhost:8080

.PHONY: balance-gen balance-test balance-lint balance-build balance-kustomize balance-docker-build balance-k3d-deploy balance-smoke

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

balance-docker-build:
	@docker build -t $(BALANCE_IMAGE) $(BALANCE_DIR)

balance-k3d-deploy: balance-docker-build
	@k3d image import $(BALANCE_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(BALANCE_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/balance
	@kubectl -n pokecalc rollout status deployment/balance --timeout=120s

balance-smoke:
	@BALANCE_URL=$(BALANCE_URL) $(BALANCE_DIR)/scripts/smoke.sh
