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
