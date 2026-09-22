# iOS レーンのターゲット(ADR-0500 §7)。ルートの Makefile から include する。
# パスはリポジトリのルートからの相対。ターゲット名は ios- 接頭辞。

IOS_SIMULATOR ?= iPhone 18 Pro
IOS_DESTINATION ?= platform=iOS Simulator,name=$(IOS_SIMULATOR)
# ios-sim-run の既定(手順書 docs/runbooks/ios.md)
IOS_SCREEN ?= root
IOS_APPEARANCE ?= light
IOS_CONTENT_SIZE ?= large

.PHONY: ios-gen
ios-gen: ## Swift の API クライアントを openapi.yaml から生成(ios/)
	@./ios/scripts/openapi-gen.sh

.PHONY: ios-gen-check
ios-gen-check: ## Swift の生成物が openapi.yaml と一致するか
	@./ios/scripts/openapi-gen.sh --check

.PHONY: ios-test
ios-test: ios-lint ios-gen-check ios-test-unit ios-test-ui ios-check-infoplist ## iOS シミュレータでテスト(XCTest + XCUITest + Info.plist 検査)

.PHONY: ios-lint
ios-lint: ## ios/scripts の構文チェック(ルートの make lint は scripts/*.sh しか見ないため個別に持つ)
	@for script in ios/scripts/*.sh; do bash -n "$$script" || exit; done

.PHONY: ios-test-unit
ios-test-unit: ## PokeCalcKit の XCTest をシミュレータで実行
	@cd ios/PokeCalcKit && ../scripts/run-xcode-tests.sh "ios-test-unit" \
		-scheme PokeCalcKit-Package -destination "$(IOS_DESTINATION)"

.PHONY: ios-test-ui
ios-test-ui: ## アプリの XCUITest をシミュレータで実行(モック強制)
	@./ios/scripts/run-xcode-tests.sh "ios-test-ui" \
		-project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination "$(IOS_DESTINATION)"

.PHONY: ios-check-infoplist
ios-check-infoplist: ## POKECALC_API_BASE_URL を渡してビルドし、成果物の Info.plist に反映されるか確認(ADR-0500 §5)
	@./ios/scripts/check-infoplist.sh "$(IOS_DESTINATION)"

.PHONY: ios-swift-test
ios-swift-test: ## PokeCalcKit の XCTest を macOS で実行(Xcode 無しでも可)
	@cd ios/PokeCalcKit && swift test

.PHONY: ios-sim-run
ios-sim-run: ## アプリをシミュレータでモック起動(IOS_SCREEN=root|calc|reverse|team IOS_APPEARANCE IOS_CONTENT_SIZE)
	@./ios/scripts/sim-run.sh "$(IOS_SIMULATOR)" "$(IOS_SCREEN)" "$(IOS_APPEARANCE)" "$(IOS_CONTENT_SIZE)"
