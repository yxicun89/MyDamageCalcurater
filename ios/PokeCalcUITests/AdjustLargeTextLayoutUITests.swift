import XCTest

/// AJ7: 調整画面の最大の文字サイズ(AX5)での横はみ出し(ADR-0502 §8・P6-14 の規約)。
/// `LargeTextLayoutUITests` と同じ検査を調整画面に当てる(既存ファイルは並行する iOS レーンが触るので、
/// 補助関数を写して別ファイルに置く)。結果の行が実際に描画された状態(送信後)で確かめる。
@MainActor
final class AdjustLargeTextLayoutUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let resultTimeout: TimeInterval = 10
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let overflowTolerance: CGFloat = 1

    /// 送信前から見える要素。
    private static let inputIdentifiers = [
        "adjustBackendModeBadge", "adjustOwnCard", "adjustOwnSpeciesButton", "adjustOwnNaturePicker",
        "adjustOwnMovePicker", "adjustOwnLearnersButton",
        "adjustFixedSP-hp", "adjustFixedSP-atk", "adjustFixedSP-def", "adjustFixedSP-spa", "adjustFixedSP-spd",
        "adjustFixedSP-spe", "adjustFixedSPTotal", "adjustModeCard",
        "adjustMode-indices", "adjustMode-bulk", "adjustMode-offense", "adjustMode-minKo", "adjustMode-minSurvive",
        "adjustSubmitButton",
    ]
    /// 送信後(指数と 16n)に出る要素。
    private static let resultIdentifiers = [
        "adjustResultCard", "adjustStatsLine", "adjustFirepowerIndex", "adjustPhysicalBulk", "adjustSpecialBulk",
        "adjustIndexNote", "adjustHPCurrent", "adjustHPLine-0", "adjustHPLine-1", "adjustHPLine-2", "adjustHPLine-3",
    ]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        let open = element(app, "openAdjustScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// 自分を選んで送信し、結果の行を描画させる(空の状態だけで「直った」と判定しない。P6-14 AC3)。
    private func submitIndices(_ app: XCUIApplication) {
        element(app, "adjustOwnSpeciesButton").tap()
        let row = element(app, "speciesSearchResult-9001-000")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        row.tap()
        element(app, "adjustOwnNaturePicker").tap()
        let nature = app.buttons["テストせいかく無補正"].firstMatch
        XCTAssertTrue(nature.waitForExistence(timeout: Self.existenceTimeout))
        nature.tap()
        let submit = element(app, "adjustSubmitButton")
        scrollUntilHittable(app, submit)
        submit.tap()
        XCTAssertTrue(element(app, "adjustStatsLine").waitForExistence(timeout: Self.resultTimeout))
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, maxAttempts: Int = 12) {
        var attempts = 0
        while !target.isHittable && attempts < maxAttempts {
            element(app, "adjustScreen").swipeUp()
            attempts += 1
        }
    }

    /// 与えた identifier の要素(同じ identifier の全件)がウィンドウの左右をはみ出していないか。
    /// 画面外(縦方向)にある要素はスクロールして出してから測る。
    private func assertNoHorizontalOverflow(_ app: XCUIApplication, identifiers: [String], file: StaticString = #filePath, line: UInt = #line) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)
        var offenders: [String] = []
        for identifier in identifiers {
            let first = element(app, identifier)
            XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout), "要素が見つからない: \(identifier)", file: file, line: line)
            let query = app.descendants(matching: .any).matching(identifier: identifier)
            for index in 0..<query.count {
                let el = query.element(boundBy: index)
                guard el.exists else { continue }
                let frame = el.frame
                if frame.width == 0 && frame.height == 0 { continue }
                let leftCut = frame.minX < -Self.overflowTolerance
                let rightCut = frame.maxX > windowFrame.maxX + Self.overflowTolerance
                if leftCut || rightCut {
                    offenders.append("\(identifier)[\(index)] label=\(el.label): frame=\(frame)")
                }
            }
        }
        XCTAssertTrue(offenders.isEmpty, "横にはみ出している要素があります(window=\(windowFrame)):\n" + offenders.joined(separator: "\n"),
                      file: file, line: line)
    }

    func testAdjustScreenNoHorizontalOverflowAtDefaultSize() {
        let app = launch()
        assertNoHorizontalOverflow(app, identifiers: Self.inputIdentifiers)
        submitIndices(app)
        assertNoHorizontalOverflow(app, identifiers: Self.resultIdentifiers)
    }

    func testAdjustScreenNoHorizontalOverflowAtAX5() {
        let app = launch(contentSizeCategory: Self.ax5ContentSizeCategory)
        assertNoHorizontalOverflow(app, identifiers: Self.inputIdentifiers)
        submitIndices(app)
        assertNoHorizontalOverflow(app, identifiers: Self.resultIdentifiers)
    }

    /// AX5 でもモードの選択肢は縦に並ぶ(横並びの segmented にしない。5つは AX5 の幅に収まらない)。
    func testModeChoicesStackVerticallyAtAX5() {
        let app = launch(contentSizeCategory: Self.ax5ContentSizeCategory)
        let first = element(app, "adjustMode-indices")
        let second = element(app, "adjustMode-bulk")
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertGreaterThanOrEqual(second.frame.minY, first.frame.maxY - Self.overflowTolerance,
                                    "モードの選択肢が縦に並んでいない(first=\(first.frame) second=\(second.frame))")
    }
}
