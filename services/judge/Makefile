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

# 生成物は Git に置かない(ADR-0807)。出力が無いか入力が新しいときだけ生成する(ルート Makefile の GEN_STALE)。
judge-gen:
	@if $(GEN_STALE) services/judge/internal/api/openapi.gen.go -- services/judge/api/openapi.yaml services/judge/internal/api/cfg.yaml services/go.mod; then \
		(cd services && $(GO) tool oapi-codegen -config judge/internal/api/cfg.yaml judge/api/openapi.yaml) && \
		echo "judge-gen: services/judge/internal/api/openapi.gen.go を生成"; \
	fi

judge-test: judge-gen
	@cd $(JUDGE_DIR) && GOWORK=off $(GO) test ./...

judge-lint: judge-gen
	@test -z "$$(gofmt -l $(JUDGE_DIR))" || { gofmt -l $(JUDGE_DIR); exit 1; }
	@cd $(JUDGE_DIR) && GOWORK=off $(GO) vet ./...

judge-build: judge-gen
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

# 公開用のイメージは手元の生成物の鮮度に頼らず、push の前に必ず作り直す(ADR-0807)
judge-docker-push:
	@$(MAKE) --no-print-directory judge-gen GEN_FORCE=1
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/publish-image.sh

# balance レーンのクラスタ内レジストリを共有する(ADR-0709 §1)。専用の registry-apply は持たない。
# 公開用のイメージは手元の生成物の鮮度に頼らず、push の前に必ず作り直す(ADR-0807)
judge-registry-push:
	@$(MAKE) --no-print-directory judge-gen GEN_FORCE=1
	@JUDGE_DIR=$(JUDGE_DIR) JUDGE_REGISTRY_PORT=$(JUDGE_REGISTRY_PORT) $(JUDGE_DIR)/scripts/local-registry-push.sh

judge-argocd-app:
	@JUDGE_DIR=$(JUDGE_DIR) $(JUDGE_DIR)/scripts/argocd-local-app.sh

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0700 §1。speed と同じ形)。
judge-docker-build: judge-gen
	@docker build -f $(JUDGE_DIR)/Dockerfile -t $(JUDGE_IMAGE) .

judge-k3d-deploy: judge-docker-build ## k3d へ judge を直接デプロイ(local overlay)
	@CLUSTER=$(CLUSTER) TAG_PATHS="$(JUDGE_DIR) engine" ./scripts/k3d-deploy-tagged.sh judge-k3d-deploy $(JUDGE_DIR)/deploy/k8s/overlays/local pokecalc/judge

judge-smoke:
	@JUDGE_URL=$(JUDGE_URL) API_URL=$(API_URL) $(JUDGE_DIR)/scripts/smoke.sh
