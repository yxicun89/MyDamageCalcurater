import XCTest

/// 計算画面の「詳細」の「防御側のランク」(issue #274。ADR-0501「防御側のランクの受け入れ条件」)。
/// `POKECALC_USE_MOCK=1` で起動する。モックはダメージを計算しない(ランクで行の数値は変わらない)ので、
/// ここでは「詳細を開くと防御側のランクが B ±0 で出る」「上げ下げで表示が変わる」「上下限でボタンが無効になる」
/// 「操作しても結果の行が出続ける」「ボタンが 36pt 以上」「攻撃側のランクと混ざらない」だけを見る。
/// 要求の中身は `CalcViewModelDefenderRanksTests` / `APIPokeCalcServiceDefenderRanksTests` で固定している。
/// identifier は追加のみ: calcDefenderRankDecrement / calcDefenderRankValue / calcDefenderRankIncrement
/// (攻撃側の calcAttackerRank* と同じ流儀。既存の identifier は変えない)。
@MainActor
final class CalcDefenderRanksUITests: XCTestCase {
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrollAttempts = 6
    private static let minTapSize: CGFloat = 36
    private static let rankLimit = 6

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchCalcScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout),
                          "起動時の行が無い: \(rowID)")
        }
        return app
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        var attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            element(app, "calcScreen").swipeUp()
            attempts += 1
        }
        // 防御側のランクは「詳細」の末尾にあるので、上にある攻撃側のランクへ戻るときは上へスクロールする。
        attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            element(app, "calcScreen").swipeDown()
            attempts += 1
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)")
    }

    private func openConditions(_ app: XCUIApplication) {
        let toggle = element(app, "calcConditionsToggle")
        scrollUntilHittable(app, toggle)
        toggle.tap()
        XCTAssertTrue(element(app, "calcConditionsPanel").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func waitForLabel(_ target: XCUIElement, _ label: String) {
        expectation(for: NSPredicate(format: "label == %@", label), evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    private func assertRowsStillShown(_ app: XCUIApplication, _ context: String) {
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout),
                          "\(context) の後も行が出ている: \(rowID)")
        }
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "\(context) で計算が失敗していない")
    }

    /// 詳細を開くと、防御側のランクが「B ±0」で出る(物理技が既定)。閉じている間は出ない。
    func testDefenderRankIsHiddenWhenCollapsedAndShowsDefaultWhenOpened() {
        let app = launchCalcScreen()
        XCTAssertFalse(element(app, "calcDefenderRankValue").exists, "閉じている間は出さない")

        openConditions(app)
        let value = element(app, "calcDefenderRankValue")
        scrollUntilHittable(app, value)
        XCTAssertEqual(value.label, "B ±0")
        XCTAssertTrue(element(app, "calcDefenderRankIncrement").exists)
        XCTAssertTrue(element(app, "calcDefenderRankDecrement").exists)
        XCTAssertEqual(element(app, "calcDefenderRankIncrement").label, "防御側のランクを上げる")
        XCTAssertEqual(element(app, "calcDefenderRankDecrement").label, "防御側のランクを下げる")
    }

    /// 上げ下げで表示が変わり、結果の行は出続ける。攻撃側のランクは変わらない。
    func testDefenderRankStepperChangesValueAndKeepsRows() {
        let app = launchCalcScreen()
        openConditions(app)

        let increment = element(app, "calcDefenderRankIncrement")
        let decrement = element(app, "calcDefenderRankDecrement")
        let value = element(app, "calcDefenderRankValue")
        let attackerValue = element(app, "calcAttackerRankValue")
        scrollUntilHittable(app, increment)

        increment.tap()
        waitForLabel(value, "B +1")
        increment.tap()
        waitForLabel(value, "B +2")
        decrement.tap()
        waitForLabel(value, "B +1")
        decrement.tap()
        decrement.tap()
        waitForLabel(value, "B -1")

        XCTAssertEqual(attackerValue.label, "A ±0", "防御側のランクを変えても攻撃側のランクは変わらない")
        assertRowsStillShown(app, "防御側のランク")

        // 攻撃側を上げても防御側は変わらない。
        let attackerIncrement = element(app, "calcAttackerRankIncrement")
        scrollUntilHittable(app, attackerIncrement)
        attackerIncrement.tap()
        waitForLabel(attackerValue, "A +1")
        XCTAssertEqual(value.label, "B -1")
    }

    /// +6 で「上げる」、-6 で「下げる」が無効になる。
    func testDefenderRankStepperDisablesAtLimits() {
        let app = launchCalcScreen()
        openConditions(app)

        let increment = element(app, "calcDefenderRankIncrement")
        let decrement = element(app, "calcDefenderRankDecrement")
        let value = element(app, "calcDefenderRankValue")
        scrollUntilHittable(app, increment)

        XCTAssertTrue(increment.isEnabled)
        XCTAssertTrue(decrement.isEnabled)
        for step in 1...Self.rankLimit {
            increment.tap()
            waitForLabel(value, "B +\(step)")
        }
        XCTAssertFalse(increment.isEnabled, "+6 では上げられない")
        XCTAssertTrue(decrement.isEnabled)

        for _ in 1...(Self.rankLimit * 2) {
            decrement.tap()
        }
        waitForLabel(value, "B -6")
        XCTAssertFalse(decrement.isEnabled, "-6 では下げられない")
        XCTAssertTrue(increment.isEnabled)
        assertRowsStillShown(app, "上下限")
    }

    /// ステッパーのボタンは 36pt 以上(タップ範囲。既定の文字サイズ)。
    func testDefenderRankButtonsAreAtLeast36pt() {
        let app = launchCalcScreen()
        openConditions(app)

        let increment = element(app, "calcDefenderRankIncrement")
        let decrement = element(app, "calcDefenderRankDecrement")
        scrollUntilHittable(app, increment)
        for button in [increment, decrement] {
            XCTAssertGreaterThanOrEqual(button.frame.width, Self.minTapSize, button.identifier)
            XCTAssertGreaterThanOrEqual(button.frame.height, Self.minTapSize, button.identifier)
        }
    }
}
