import XCTest

/// G-04: 素早さ画面の「自分の周り」パネル(ADR-0527)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ)。数値は検査しない(ADR-0501 と同じ粒度)。
/// 確かめるのは「常時表示(折りたたみなし)」「表をスクロールしても見え続ける(表の外にある)」「AX5 で横にはみ出さない」。
///
/// 識別子(ADR-0527。implementer はこの名前で付ける):
///   speedNeighborhood(全体)/ speedNeighborhoodBefore・speedNeighborhoodAfter(先に動く側・後に動く側のリスト)/
///   speedNeighborhoodSelf / speedNeighborhoodBeforeTotal・speedNeighborhoodAfterTotal(端の合計)/ speedNeighborhoodEmpty(自分未決定)
///
/// 【spec-writer の足場】View が未実装のため、このテストは失敗してよい。
/// AX5 のテストは LargeTextLayoutUITests とは独立に持つ(そちらの既存の検査は変えない)。
@MainActor
final class SpeedNeighborhoodUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let overflowTolerance: CGFloat = 1
    private static let scrollCount = 6

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launchSpeedScreen(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        let open = element(app, "openSpeedScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openSpeedScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout), "speedScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// 実数値 1(全行より遅い)を入れて、自分の位置を決める。
    private func enterRawValue(_ app: XCUIApplication) {
        let rawMode = element(app, "speedMode-raw")
        XCTAssertTrue(rawMode.waitForExistence(timeout: Self.existenceTimeout))
        rawMode.tap()
        let field = element(app, "speedRawValueField")
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText("1")
        XCTAssertTrue(element(app, "speedResultSpeed").waitForExistence(timeout: Self.existenceTimeout), "結果が出ない")
    }

    private func assertWithinWindow(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い", file: file, line: line)
        let window = app.windows.firstMatch.frame
        let frame = target.frame
        XCTAssertGreaterThanOrEqual(frame.minX, window.minX - Self.overflowTolerance, "\(identifier) が左にはみ出す", file: file, line: line)
        XCTAssertLessThanOrEqual(frame.maxX, window.maxX + Self.overflowTolerance, "\(identifier) が右にはみ出す", file: file, line: line)
    }

    /// 常時表示(折りたたみなし)で、自分の位置が決まると前後が出る。表を下へ何度スクロールしても、
    /// 画面内でパネルの中身が見え続ける(パネルは表のスクロール領域の外 = 表の LazyVStack に入っていない。ADR-0517 とは独立)。
    func testNeighborhoodIsAlwaysVisibleAndSurvivesTableScrolling() {
        let app = launchSpeedScreen()
        let panel = element(app, "speedNeighborhood")
        XCTAssertTrue(panel.waitForExistence(timeout: Self.existenceTimeout), "自分の周りのパネルが無い(折りたたみにしない)")
        XCTAssertTrue(element(app, "speedNeighborhoodEmpty").waitForExistence(timeout: Self.existenceTimeout), "自分未決定の案内が無い")

        enterRawValue(app)
        XCTAssertTrue(element(app, "speedNeighborhoodSelf").waitForExistence(timeout: Self.existenceTimeout), "自分の行が無い")
        XCTAssertTrue(element(app, "speedNeighborhoodBefore").exists, "先に動く側(通常の場は速い側)のリストが無い")
        XCTAssertTrue(element(app, "speedNeighborhoodBeforeTotal").exists, "端の合計行が無い")
        XCTAssertFalse(element(app, "speedNeighborhoodEmpty").exists, "自分が決まったら案内は消える")

        // 表を下へスクロールしても、パネルは画面の外に追い出されない(画面の縦のスクロールで外れるのは仕方ないので、
        // 表の領域に入ってからの数回で「存在し続ける」ことだけを確かめる。常時表示 = 木から消えない)。
        let table = element(app, "speedTable")
        XCTAssertTrue(table.waitForExistence(timeout: Self.existenceTimeout))
        for _ in 0..<Self.scrollCount {
            app.swipeUp(velocity: .slow)
            XCTAssertTrue(panel.exists, "表をスクロールしてもパネルが木から消える(表の遅延描画に入っている)")
        }
    }

    /// トリックルームでは見出しが「先に動く側/後に動く側」になる。
    func testNeighborhoodUsesBeforeAfterWordsInTrickRoom() {
        let app = launchSpeedScreen()
        enterRawValue(app)
        let trickRoom = element(app, "speedTrickRoom")
        XCTAssertTrue(trickRoom.waitForExistence(timeout: Self.existenceTimeout))
        trickRoom.tap()
        let after = element(app, "speedNeighborhoodAfter")
        XCTAssertTrue(after.waitForExistence(timeout: Self.existenceTimeout), "後に動く側のリストが無い")
        XCTAssertTrue(
            app.staticTexts["後に動く側"].exists || after.label.contains("後に動く側"), "見出しが「後に動く側」になっていない")
    }

    /// AX5(Dynamic Type 最大)でもパネルが横にはみ出さない(固定高にしない。折り返す)。LargeTextLayoutUITests とは独立。
    func testNeighborhoodNoHorizontalOverflowAtAX5() {
        let app = launchSpeedScreen(contentSizeCategory: Self.ax5ContentSizeCategory)
        enterRawValue(app)
        for identifier in ["speedNeighborhood", "speedNeighborhoodSelf", "speedNeighborhoodBefore", "speedNeighborhoodBeforeTotal"] {
            assertWithinWindow(app, identifier)
        }
    }
}
