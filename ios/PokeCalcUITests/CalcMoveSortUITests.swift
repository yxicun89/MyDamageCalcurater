import XCTest

/// F-02: 技ピッカー(検索シート)の並びの切り替え(習得順・五十音順・タイプ順。ADR-0523)。
/// モックの既定の攻撃側(9001)のダメージ技は、習得順 = ぶつりA(かくとう)→ とくしゅA(どく)→ れんぞく(ノーマル)、
/// 五十音順 = とくしゅA → ぶつりA → れんぞく、タイプ順 = れんぞく(ノーマル)→ ぶつりA(かくとう)→ とくしゅA(どく)。
@MainActor
final class CalcMoveSortUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let physical = "moveSearchResult-test-move-physical-a"
    private static let special = "moveSearchResult-test-move-special-a"
    private static let multi = "moveSearchResult-test-move-multi-hit"
    private static let minimumTapSide: CGFloat = 36

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func openMoveSheet(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        let open = app.buttons["openCalcScreen"]
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        let picker = element(app, "movePicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "moveSortPicker").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 上から下への並び(minY の昇順)の identifier を返す。
    private func order(_ app: XCUIApplication, _ identifiers: [String]) -> [String] {
        for id in identifiers {
            XCTAssertTrue(element(app, id).waitForExistence(timeout: Self.existenceTimeout), "\(id) が無い")
        }
        return identifiers.sorted { element(app, $0).frame.minY < element(app, $1).frame.minY }
    }

    func testDefaultIsLearnsetThenKanaThenTypeWithGroupHeadings() {
        let app = openMoveSheet()
        let all = [Self.physical, Self.special, Self.multi]
        XCTAssertEqual(order(app, all), [Self.physical, Self.special, Self.multi], "既定は習得順")
        XCTAssertTrue(element(app, "moveSort-learnset").isSelected)

        element(app, "moveSort-kana").tap()
        XCTAssertEqual(order(app, all), [Self.special, Self.physical, Self.multi], "五十音順")
        XCTAssertTrue(element(app, "moveSort-kana").isSelected)
        XCTAssertFalse(app.staticTexts["ノーマル"].exists, "五十音順では見出しを出さない")

        element(app, "moveSort-type").tap()
        XCTAssertEqual(order(app, all), [Self.multi, Self.physical, Self.special], "タイプ順")
        for heading in ["ノーマル", "かくとう", "どく"] {
            XCTAssertTrue(app.staticTexts[heading].waitForExistence(timeout: Self.existenceTimeout), "見出し \(heading)")
        }

        element(app, "moveSort-learnset").tap()
        XCTAssertEqual(order(app, all), [Self.physical, Self.special, Self.multi], "習得順へ戻せる")
    }

    func testChangingOrderKeepsSelectedMoveAndSelectingStillWorks() {
        let app = openMoveSheet()
        element(app, "moveSort-type").tap()
        element(app, Self.special).tap()
        XCTAssertFalse(element(app, "moveSearchSheet").exists, "選ぶとシートが閉じる")
        let picker = element(app, "movePicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(picker.label.contains("テストわざとくしゅA"), "タイプ順で選んだ技が入る: \(picker.label)")
    }

    func testChipsFitScreenAndTapTargetsAtAX5() {
        let app = openMoveSheet(contentSizeCategory: Self.ax5ContentSizeCategory)
        let window = app.windows.firstMatch.frame
        for id in ["moveSort-learnset", "moveSort-kana", "moveSort-type"] {
            let chip = element(app, id)
            XCTAssertTrue(chip.waitForExistence(timeout: Self.existenceTimeout))
            XCTAssertGreaterThanOrEqual(chip.frame.minX, -1, "\(id) が左に切れている \(chip.frame)")
            XCTAssertLessThanOrEqual(chip.frame.maxX, window.maxX + 1, "\(id) が右に切れている \(chip.frame)")
            XCTAssertGreaterThanOrEqual(chip.frame.height, Self.minimumTapSide, "\(id) のタップ領域が小さい")
        }
        // AX5 では縦に積む(横並びだと3つが同じ高さに並ぶ)。
        XCTAssertGreaterThan(element(app, "moveSort-kana").frame.minY, element(app, "moveSort-learnset").frame.minY + 1)
    }
}
