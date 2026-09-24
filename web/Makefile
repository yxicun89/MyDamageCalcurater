# Web レーン(web/)のターゲット。ルートの Makefile から include する(ターゲット名は web- 接頭辞)。
# web-test / web-lint / web-build は、依存が無い・lockfile より古いときに自動で npm ci する(web-deps)。

WEB_DIR := web

# ルートの make test / make lint / make build に Web を含める(ユーザー決定 2026-09-22。DECISIONS.md)。
# 他のレーンの作業ディレクトリでも Web の失敗に気づけるように、依存が無ければ npm ci してから実行する。
test: web-test
lint: web-lint
build: web-build

# npm ci が書く node_modules/.package-lock.json を目印にし、package-lock.json の方が新しいときだけ入れ直す。
$(WEB_DIR)/node_modules/.package-lock.json: $(WEB_DIR)/package-lock.json
	@cd $(WEB_DIR) && npm ci --no-audit --no-fund

.PHONY: web-deps
web-deps: $(WEB_DIR)/node_modules/.package-lock.json

.PHONY: web-install
web-install: ## Web の依存を lockfile どおりに入れる(npm ci)
	@cd $(WEB_DIR) && npm ci --no-audit --no-fund

.PHONY: web-test
web-test: web-deps ## Web の単体・画面テスト(Vitest。engine は fake)
	@cd $(WEB_DIR) && npm test

.PHONY: web-test-wasm
web-test-wasm: wasm web-deps ## Web が組み立てたリクエストを本物の engine.wasm に通す結合テスト
	@cd $(WEB_DIR) && npm run test:wasm

.PHONY: web-lint
web-lint: web-deps ## Web の型検査・ESLint・Prettier
	@cd $(WEB_DIR) && npm run typecheck && npm run lint

.PHONY: web-build
web-build: web-deps ## Web をビルドし JS の配信サイズ予算を確かめる
	@cd $(WEB_DIR) && npm run build

.PHONY: web-dev
web-dev: wasm web-deps ## Web の開発サーバー(WASM で計算。バックエンド不要)
	@cd $(WEB_DIR) && npm run dev

.PHONY: web-e2e-install
web-e2e-install: web-deps ## E2E 用のブラウザ(chromium)を入れる(初回だけ。Playwright の版に合わせる)
	@cd $(WEB_DIR) && npx --no-install playwright install chromium

.PHONY: web-e2e
web-e2e: wasm web-deps ## Web の E2E(Playwright・chromium。オフライン = WASM。本番ビルドを preview で配る)
	@cd $(WEB_DIR) && npm run e2e

.PHONY: web-e2e-online
web-e2e-online: wasm web-deps ## Web の E2E(オンライン = API。例データで calc-svc も起動する。要 Go)
	@cd $(WEB_DIR) && npm run e2e:online

.PHONY: web-e2e-balance
web-e2e-balance: wasm web-deps ## Web の E2E(タイプバランス = balance API。例データで balance-svc も起動する。要 Go)
	@cd $(WEB_DIR) && npm run e2e:balance

# --- P4-11: コンテナ(k3d)---
# 既定の WEB_URL(web-k3d-smoke)は利用者がブラウザで開く入口(k3d の loadbalancer → gateway。localhost:8080)。
# Web だけを直接確かめるとき(診断用)は `make web-k3d-open` の port-forward 先 http://localhost:5173 を渡す。
WEB_URL ?= http://localhost:8080

.PHONY: web-docker-build
web-docker-build: ## Web のコンテナイメージ(nginx。engine.wasm を含む)を local タグでビルドする(コンテキストはリポジトリルート)
	@docker build -f web/Dockerfile -t pokecalc/web:local .

.PHONY: web-e2e-container
web-e2e-container: web-docker-build web-deps ## 作ったイメージを docker run で起動し、配信設定(SPA・MIME・キャッシュ・gzip)を E2E で確かめる
	@cd $(WEB_DIR) && npm run e2e:container

# クラスタ全体の立ち上げ(namespace・mysql・pokedex-migrate Job を含む)は `make up`(scripts/up.sh)が担う。
# `deploy/k8s/overlays/local`(共有)を丸ごと apply すると、他レーンの Job・DB を意図せず巻き込みうる
# (k3d クラスタは他レーンと共有。services/gateway/Makefile の api-k3d-deploy と同じ考え方)。Web レーンは
# 専用 overlay `deploy/k8s/overlays/local-web`(base/web と Component local/web だけ)で
# 自分の Deployment だけを apply する。
.PHONY: web-k3d-deploy
web-k3d-deploy: web-docker-build ## k3d(既存クラスタ。make up 済み)へ Web だけをデプロイする
	@test "$$(kubectl config current-context)" = "k3d-$(CLUSTER)" || { echo "web-k3d-deploy: kubectl のコンテキストが k3d-$(CLUSTER) ではない" >&2; exit 1; }
	@k3d image import pokecalc/web:local --cluster $(CLUSTER)
	@kubectl apply -k deploy/k8s/overlays/local-web
	@kubectl -n pokecalc rollout restart deployment/web
	@kubectl -n pokecalc rollout status deployment/web --timeout=120s

.PHONY: web-k3d-open
web-k3d-open: ## 診断用: gateway を通さず Web の Service へ port-forward する(localhost:5173。Ctrl-C で終了)
	@kubectl -n pokecalc port-forward svc/web 5173:80

.PHONY: web-k3d-smoke
web-k3d-smoke: ## k3d 上の Web をスモーク(既定 http://localhost:8080 = gateway 経由。WEB_URL で上書き)
	@WEB_URL=$(WEB_URL) web/scripts/k3d-smoke.sh

.PHONY: web-kustomize
web-kustomize: ## base・local・local-web overlay が描画できることを確かめる(apply しない)
	@kubectl kustomize deploy/k8s/base >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local >/dev/null
	@kubectl kustomize deploy/k8s/overlays/local-web >/dev/null
