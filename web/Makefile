# Web レーン(web/)のターゲット。ルートの Makefile から include する(ターゲット名は web- 接頭辞)。
# web/node_modules が無いときは web-install(npm ci)を先に実行する。

WEB_DIR := web

.PHONY: web-install
web-install: ## Web の依存を lockfile どおりに入れる(npm ci)
	@cd $(WEB_DIR) && npm ci --no-audit --no-fund

.PHONY: web-test
web-test: ## Web の単体・画面テスト(Vitest。engine は fake)
	@cd $(WEB_DIR) && npm test

.PHONY: web-test-wasm
web-test-wasm: wasm ## Web が組み立てたリクエストを本物の engine.wasm に通す結合テスト
	@cd $(WEB_DIR) && npm run test:wasm

.PHONY: web-lint
web-lint: ## Web の型検査・ESLint・Prettier
	@cd $(WEB_DIR) && npm run typecheck && npm run lint

.PHONY: web-build
web-build: ## Web をビルドし JS の配信サイズ予算を確かめる
	@cd $(WEB_DIR) && npm run build

.PHONY: web-dev
web-dev: wasm ## Web の開発サーバー(WASM で計算。バックエンド不要)
	@cd $(WEB_DIR) && npm run dev
