# API レーン(calc-svc・gateway)のターゲット(ADR-0203 §4)。すべて api- 接頭辞
# (レーン間でターゲット名を衝突させない。docs/ai-shared/COORDINATION.md)。
# ルートの Makefile からは `include services/gateway/Makefile` の1行だけで読む。
#
# API のユニットテストは既存の `test-services`(ルート Makefile の `make test`)に含まれるので、
# services/balance のような `test: api-test` の前提条件は足さない(ADR-0203 §4)。

API_CLUSTER ?= $(CLUSTER)
API_URL ?= http://localhost:8080

.PHONY: api-docker-build api-k3d-deploy api-smoke api-kustomize

api-docker-build: ## calc・gateway のイメージを local タグでビルドする
	@docker build -f services/calc/Dockerfile -t pokecalc/calc:local .
	@docker build -f services/gateway/Dockerfile -t pokecalc/gateway:local .

# `deploy/k8s/overlays/local`(共有)は pokedex-migrate Job・mysql も含む。その Job が前提にする
# `mysql-auth` Secret はデータレーンが `make up`(scripts/up.sh)で作るもので、API レーンは他レーンの
# Secret を代わりに作らない(CLAUDE.md「サービスは自分のDBにだけ触る」)。Secret が無いクラスタでは
# `deploy/k8s/overlays/local-api`(calc・gateway だけ)を適用する(ADR-0203 追記)。
api-k3d-deploy: api-docker-build ## k3d(既存クラスタ)へ calc・gateway をデプロイする
	@k3d image import pokecalc/calc:local pokecalc/gateway:local --cluster $(API_CLUSTER)
	@if kubectl -n pokecalc get secret mysql-auth >/dev/null 2>&1; then \
		echo "api-k3d-deploy: mysql-auth がある(make up 済み)。deploy/k8s/overlays/local を適用します"; \
		kubectl apply -k deploy/k8s/overlays/local; \
	else \
		echo "api-k3d-deploy: mysql-auth が無い(他レーン未デプロイ)。deploy/k8s/overlays/local-api だけ適用します"; \
		kubectl apply -k deploy/k8s/overlays/local-api; \
	fi
	@kubectl -n pokecalc rollout restart deployment/calc deployment/gateway
	@kubectl -n pokecalc rollout status deployment/calc --timeout=120s
	@kubectl -n pokecalc rollout status deployment/gateway --timeout=120s

api-smoke: ## gateway 経由のスモーク(既定 http://localhost:8080。API_URL で上書き)
	@API_URL=$(API_URL) services/gateway/scripts/smoke.sh

api-kustomize: ## base・local overlay が描画できることを確かめる(apply しない)
	@kubectl kustomize deploy/k8s/base >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local-api >/dev/null
