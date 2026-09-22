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

# クラスタ全体の立ち上げ(namespace・mysql・pokedex-migrate Job を含む)は `make up`
# (scripts/up.sh)が担う。`deploy/k8s/overlays/local`(共有)を丸ごと apply すると、
# pokedex-migrate Job の再実行(spec.template は immutable なので既存 Job と食い違うと
# 失敗する)や mysql の意図しない上書きを引き起こしうる(k3d クラスタは他レーンと共有)。
# API レーンはここで自分の2つの Deployment(calc・gateway)だけを、専用の overlay
# `deploy/k8s/overlays/local-api`(base/calc・base/gateway と Component local/api だけ)で
# apply する。他レーンのリソースには一切触らない(ADR-0203 追記「apply の分離」)。
api-k3d-deploy: api-docker-build ## k3d(既存クラスタ。make up 済み)へ calc・gateway だけをデプロイする
	@k3d image import pokecalc/calc:local pokecalc/gateway:local --cluster $(API_CLUSTER)
	@kubectl apply -k deploy/k8s/overlays/local-api
	@kubectl -n pokecalc rollout restart deployment/calc deployment/gateway
	@kubectl -n pokecalc rollout status deployment/calc --timeout=120s
	@kubectl -n pokecalc rollout status deployment/gateway --timeout=120s

api-smoke: ## gateway 経由のスモーク(既定 http://localhost:8080。API_URL で上書き)
	@API_URL=$(API_URL) services/gateway/scripts/smoke.sh

api-kustomize: ## base・local overlay が描画できることを確かめる(apply しない)
	@kubectl kustomize deploy/k8s/base >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local-api >/dev/null
