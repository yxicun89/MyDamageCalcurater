import XCTest

/// 計算画面の「詳細」(issue #274。ADR-0501「issue #274」5章)。`POKECALC_USE_MOCK=1` で起動する。
///
/// モックはダメージを計算しない(条件で行の数値は変わらない)ので、ここでは「折りたたみの既定・開閉」
/// 「各入力の選択状態(`isSelected`)と表示」「操作しても結果の行が出続ける(計算が失敗しない)」だけを見る。
/// 条件が要求に載ることは `CalcViewModelConditionsTests` で固定している。
@MainActor
final class CalcConditionsUITests: XCTestCase {
    /// モックの物理技の既定5行(`CalcScreenUITests.defaultPhysicalRowIDs` と同じ。ADR-0009)。
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]
    private static let existenceTimeout: TimeInterval = 5
    /// 画面外の要素をタップできる位置まで上へスクロールする最大回数。
    private static let maxScrollAttempts = 6

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

    /// 「詳細」は画面の中ほどにあるので、タップできるまで上へスクロールする。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        var attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            element(app, "calcScreen").swipeUp()
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

    private func assertRowsStillShown(_ app: XCUIApplication, _ context: String) {
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout),
                          "\(context) の後も行が出ている: \(rowID)")
        }
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "\(context) で計算が失敗していない")
    }

    /// 起動直後は閉じていて、開くと全入力が既定の選択状態で出る。閉じるとまた隠れる。
    func testConditionsAreCollapsedByDefaultAndShowDefaultsWhenOpened() {
        let app = launchCalcScreen()

        XCTAssertTrue(element(app, "calcConditionsToggle").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "calcConditionsPanel").exists, "既定は閉じている")
        XCTAssertFalse(element(app, "calcCondition-critical").exists, "閉じている間は入力を出さない")

        openConditions(app)

        XCTAssertFalse(element(app, "calcCondition-critical").isSelected, "急所の既定は off")
        XCTAssertFalse(element(app, "calcCondition-burn").isSelected, "やけどの既定は off")
        XCTAssertTrue(element(app, "calcWeather-none").isSelected, "天候の既定は なし")
        for weather in ["sun", "rain", "sand", "snow"] {
            XCTAssertFalse(element(app, "calcWeather-\(weather)").isSelected, weather)
        }
        XCTAssertTrue(element(app, "calcTerrain-none").isSelected, "フィールドの既定は なし")
        for terrain in ["electric", "grassy", "psychic", "misty"] {
            XCTAssertFalse(element(app, "calcTerrain-\(terrain)").isSelected, terrain)
        }
        for screen in ["reflect", "lightScreen", "auroraVeil"] {
            XCTAssertFalse(element(app, "calcDefenderScreen-\(screen)").isSelected, screen)
        }
        XCTAssertEqual(element(app, "calcAttackerRankValue").label, "A ±0", "既定の技は物理なので A")
        XCTAssertTrue(element(app, "calcAttackerRankIncrement").exists)
        XCTAssertTrue(element(app, "calcAttackerRankDecrement").exists)
        XCTAssertTrue(element(app, "calcAttackerAbilityPicker").exists)

        let toggle = element(app, "calcConditionsToggle")
        scrollUntilHittable(app, toggle)
        toggle.tap()
        let hidden = NSPredicate(format: "exists == false")
        expectation(for: hidden, evaluatedWith: element(app, "calcConditionsPanel"), handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    /// 急所・天候・壁・ランクを操作すると選択状態が変わり、結果の行は出続ける。
    func testTogglingConditionsUpdatesSelectionAndKeepsRows() {
        let app = launchCalcScreen()
        openConditions(app)

        let critical = element(app, "calcCondition-critical")
        scrollUntilHittable(app, critical)
        critical.tap()
        expectation(for: NSPredicate(format: "isSelected == true"), evaluatedWith: critical, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        assertRowsStillShown(app, "急所")

        let sun = element(app, "calcWeather-sun")
        scrollUntilHittable(app, sun)
        sun.tap()
        expectation(for: NSPredicate(format: "isSelected == true"), evaluatedWith: sun, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertFalse(element(app, "calcWeather-none").isSelected, "天候は1つだけ選ばれる")

        let reflect = element(app, "calcDefenderScreen-reflect")
        scrollUntilHittable(app, reflect)
        reflect.tap()
        expectation(for: NSPredicate(format: "isSelected == true"), evaluatedWith: reflect, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)

        let increment = element(app, "calcAttackerRankIncrement")
        scrollUntilHittable(app, increment)
        increment.tap()
        let rankValue = element(app, "calcAttackerRankValue")
        expectation(for: NSPredicate(format: "label == %@", "A +1"), evaluatedWith: rankValue, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)

        assertRowsStillShown(app, "天候・壁・ランク")
        XCTAssertTrue(critical.isSelected, "他の条件を変えても急所は残る")
    }
}
