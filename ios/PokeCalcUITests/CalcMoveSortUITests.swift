import XCTest

/// G-01: 技ピッカー(検索シート)はタイプ順だけで並ぶ(切り替えは無い。ADR-0527。F-02 / ADR-0523 を更新)。
/// モックの既定の攻撃側(9001)のダメージ技は、タイプ順 = れんぞく(ノーマル)→ ぶつりA(かくとう)→ とくしゅA(どく)。
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
        return app
    }

    /// 上から下への並び(minY の昇順)の identifier を返す。
    private func order(_ app: XCUIApplication, _ identifiers: [String]) -> [String] {
        for id in identifiers {
            XCTAssertTrue(element(app, id).waitForExistence(timeout: Self.existenceTimeout), "\(id) が無い")
        }
        return identifiers.sorted { element(app, $0).frame.minY < element(app, $1).frame.minY }
    }

    func testMovesAreTypeOrderedWithHeadingsAndNoSortControls() {
        let app = openMoveSheet()
        let all = [Self.physical, Self.special, Self.multi]
        XCTAssertEqual(order(app, all), [Self.multi, Self.physical, Self.special], "タイプ順(ノーマル→かくとう→どく)")
        for heading in ["ノーマル", "かくとう", "どく"] {
            XCTAssertTrue(app.staticTexts[heading].waitForExistence(timeout: Self.existenceTimeout), "見出し \(heading)")
        }
        XCTAssertFalse(element(app, "moveSortPicker").exists, "並びの切り替えは無い")
    }

    func testSelectingFromTypeOrderedListStillWorks() {
        let app = openMoveSheet()
        element(app, Self.special).tap()
        XCTAssertFalse(element(app, "moveSearchSheet").exists, "選ぶとシートが閉じる")
        let picker = element(app, "movePicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(picker.label.contains("テストわざとくしゅA"), "タイプ順で選んだ技が入る: \(picker.label)")
    }

    func testRowsFitScreenAndTapTargetsAtAX5() {
        let app = openMoveSheet(contentSizeCategory: Self.ax5ContentSizeCategory)
        let window = app.windows.firstMatch.frame
        for id in [Self.physical, Self.special, Self.multi] {
            let row = element(app, id)
            XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
            XCTAssertGreaterThanOrEqual(row.frame.minX, -1, "\(id) が左に切れている \(row.frame)")
            XCTAssertLessThanOrEqual(row.frame.maxX, window.maxX + 1, "\(id) が右に切れている \(row.frame)")
            XCTAssertGreaterThanOrEqual(row.frame.height, Self.minimumTapSide, "\(id) のタップ領域が小さい")
        }
    }
}
