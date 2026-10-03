GO ?= go
CLUSTER ?= pokecalc
SPEED_DIR ?= services/speed
SPEED_IMAGE ?= pokecalc/speed:local
SPEED_URL ?= http://localhost:8080
# pokedex export の出力先(Git 管理外。ADR-0603 §1)
SPEED_READMODEL_DIR ?= data/generated/readmodel
# クラスタ内レジストリは balance-registry namespace のものを共有する(ADR-0605 §1)。
# Mac 側の port-forward は balance の 5001 と衝突しないよう 5002(ノード側は localhost:5000 のまま)。
SPEED_REGISTRY_PORT ?= 5002

.PHONY: speed-gen speed-test speed-lint speed-build speed-kustomize speed-docker-build speed-docker-push speed-k3d-deploy speed-smoke speed-k3d-deploy-readmodel speed-smoke-readmodel speed-gitops-template-check speed-gitops-check speed-registry-push speed-argocd-app

# ルートの make test / lint / build に speed を含める(ADR-0600 §2。balance と同じ)。
# レシピを持たない前提条件の追加なので、ルートの既存ターゲットのレシピは変わらない。
test: speed-test
lint: speed-lint
build: speed-build

# 生成物は Git に置かない(ADR-0806)。出力が無いか入力が新しいときだけ生成する(ルート Makefile の GEN_STALE)。
speed-gen:
	@if $(GEN_STALE) services/speed/internal/api/openapi.gen.go -- services/speed/api/openapi.yaml services/speed/internal/api/cfg.yaml services/go.mod; then \
		(cd services && $(GO) tool oapi-codegen -config speed/internal/api/cfg.yaml speed/api/openapi.yaml) && \
		echo "speed-gen: services/speed/internal/api/openapi.gen.go を生成"; \
	fi

speed-test: speed-gen
	@cd $(SPEED_DIR) && GOWORK=off $(GO) test ./...

speed-lint: speed-gen
	@test -z "$$(gofmt -l $(SPEED_DIR))" || { gofmt -l $(SPEED_DIR); exit 1; }
	@cd $(SPEED_DIR) && GOWORK=off $(GO) vet ./...

speed-build: speed-gen
	@cd $(SPEED_DIR) && GOWORK=off $(GO) build ./...

speed-kustomize:
	@kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/local-readmodel >/dev/null
	@kubectl kustomize $(SPEED_DIR)/deploy/k8s/overlays/gitops >/dev/null
	@kubectl kustomize $(SPEED_DIR)/deploy/argocd >/dev/null

# クラスタを変えない検査(placeholder のままでよい)。GitOps を適用する前は ready のほうで確かめる。
speed-gitops-template-check: speed-kustomize
	@SERVICE=speed scripts/gitops/check-gitops.sh template

speed-gitops-check: speed-kustomize
	@SERVICE=speed scripts/gitops/check-gitops.sh ready

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0600 §2)。
speed-docker-build: speed-gen
	@docker build -f $(SPEED_DIR)/Dockerfile -t $(SPEED_IMAGE) .

# 公開用のイメージは手元の生成物の鮮度に頼らず、push の前に必ず作り直す(ADR-0806)
speed-docker-push:
	@$(MAKE) --no-print-directory speed-gen GEN_FORCE=1
	@SERVICE=speed scripts/gitops/publish-image.sh

# balance レーンのクラスタ内レジストリを共有する(ADR-0605 §1)。専用の registry-apply は持たない。
# 公開用のイメージは手元の生成物の鮮度に頼らず、push の前に必ず作り直す(ADR-0806)
speed-registry-push:
	@$(MAKE) --no-print-directory speed-gen GEN_FORCE=1
	@SERVICE=speed SPEED_REGISTRY_PORT=$(SPEED_REGISTRY_PORT) scripts/gitops/local-registry-push.sh

speed-argocd-app:
	@SERVICE=speed scripts/gitops/argocd-local-app.sh

speed-k3d-deploy: speed-docker-build ## k3d へ speed を直接デプロイ(local overlay。架空データの read model)
	@CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh speed-k3d-deploy
	@k3d image import $(SPEED_IMAGE) --cluster $(CLUSTER)
	@kubectl apply -k $(SPEED_DIR)/deploy/k8s/overlays/local
	@kubectl -n pokecalc rollout restart deployment/speed
	@kubectl -n pokecalc rollout status deployment/speed --timeout=120s

speed-smoke:
	@SPEED_URL=$(SPEED_URL) $(SPEED_DIR)/scripts/smoke.sh

# pokedex export の実データの read model を ConfigMap にして k3d の speed に読ませる(ADR-0603)
speed-k3d-deploy-readmodel: speed-gen
	@CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh speed-k3d-deploy-readmodel
	@SERVICE=speed SPEED_IMAGE=$(SPEED_IMAGE) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) CLUSTER=$(CLUSTER) scripts/gitops/k3d-deploy-readmodel.sh

speed-smoke-readmodel:
	@SPEED_URL=$(SPEED_URL) SPEED_READMODEL_DIR=$(SPEED_READMODEL_DIR) $(SPEED_DIR)/scripts/smoke-readmodel.sh
