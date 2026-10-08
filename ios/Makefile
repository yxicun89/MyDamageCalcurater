# iOS レーンのターゲット(ADR-0500 §7)。ルートの Makefile から include する。
# パスはリポジトリのルートからの相対。ターゲット名は ios- 接頭辞。

IOS_SIMULATOR ?= iPhone 18 Pro
IOS_DESTINATION ?= platform=iOS Simulator,name=$(IOS_SIMULATOR)
# ios-sim-run の既定(手順書 docs/runbooks/ios.md)
IOS_SCREEN ?= root
IOS_APPEARANCE ?= light
IOS_CONTENT_SIZE ?= large

# Swift の生成物は Git に置かない(ADR-0807)。ios-gen は出力が無いか入力が新しい対象だけ生成するので、
# 下の ios-* はすべて前段に ios-gen を持つ。Xcode で直接開く前も make ios-gen を1回流す。
.PHONY: ios-gen
ios-gen: ## Swift の API クライアントを各 openapi.yaml から生成(Git 管理外。変更が無ければ何もしない。強制は GEN_FORCE=1)
	@./ios/scripts/openapi-gen.sh

.PHONY: ios-gen-check
ios-gen-check: ## 手元の Swift の生成物が各 openapi.yaml と一致するか(一時ディレクトリに生成して比べる)
	@./ios/scripts/openapi-gen.sh --check

.PHONY: ios-test
ios-test: ios-gen ios-lint ios-gen-check ios-check-request-limits ios-test-unit ios-test-ui ios-check-infoplist ## iOS シミュレータでテスト(XCTest + XCUITest + Info.plist 検査)

.PHONY: ios-lint
ios-lint: ios-gen ## ios/scripts の構文チェック(ルートの make lint は scripts/*.sh しか見ないため個別に持つ)
	@for script in ios/scripts/*.sh; do bash -n "$$script" || exit; done
	@bash ios/scripts/xcode-test-lock_test.sh

.PHONY: ios-check-request-limits
ios-check-request-limits: ios-gen ## RequestLimits(件数上限の写し)が api/openapi.yaml の maxItems と一致するか(issue #110)
	@./ios/scripts/check-request-limits.sh

.PHONY: ios-test-unit
ios-test-unit: ios-gen ## PokeCalcKit の XCTest をシミュレータで実行
	@cd ios/PokeCalcKit && ../scripts/run-xcode-tests.sh "ios-test-unit" \
		-scheme PokeCalcKit-Package -destination "$(IOS_DESTINATION)"

.PHONY: ios-test-ui
ios-test-ui: ios-gen ## アプリの XCUITest をシミュレータで実行(モック強制)
	@./ios/scripts/run-xcode-tests.sh "ios-test-ui" \
		-project ios/PokeCalc.xcodeproj -scheme PokeCalc -destination "$(IOS_DESTINATION)"

.PHONY: ios-check-infoplist
ios-check-infoplist: ios-gen ## POKECALC_API_BASE_URL を渡してビルドし、成果物の Info.plist に反映されるか確認(ADR-0500 §5)
	@./ios/scripts/check-infoplist.sh "$(IOS_DESTINATION)"

.PHONY: ios-swift-test
ios-swift-test: ios-gen ## PokeCalcKit の XCTest を macOS で実行(Xcode 無しでも可)
	@cd ios/PokeCalcKit && swift test

.PHONY: ios-sim-run
ios-sim-run: ios-gen ## アプリをシミュレータでモック起動(IOS_SCREEN=root|calc|reverse|team|adjust|speed|judge IOS_APPEARANCE IOS_CONTENT_SIZE)
	@./ios/scripts/sim-run.sh "$(IOS_SIMULATOR)" "$(IOS_SCREEN)" "$(IOS_APPEARANCE)" "$(IOS_CONTENT_SIZE)"
