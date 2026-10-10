import XCTest

/// F-11(ADR-0525): 調整の目標方式の最大の文字サイズ(AX5)での横はみ出し。
/// `AdjustLargeTextLayoutUITests` と同じ検査を目標方式(3種類の目標カード・結果の行)に当てる(補助関数は写す)。
@MainActor
final class AdjustGoalsLargeTextUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let resultTimeout: TimeInterval = 10
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let overflowTolerance: CGFloat = 1

    /// 目標のカード(3種類)と追加ボタン。
    private static let inputIdentifiers = [
        "adjustMode-goals", "adjustGoalsCard", "adjustAddGoalButton",
        "adjustGoal-1", "adjustGoal-1-legend", "adjustGoal-1-kind", "adjustGoal-1-kind-outspeed", "adjustGoal-1-kind-survive",
        "adjustGoal-1-kind-ko", "adjustGoal-1-opponentButton", "adjustGoal-1-presets", "adjustGoal-1-preset-fastest",
        "adjustGoal-1-preset-neutral_max", "adjustGoal-1-preset-none", "adjustGoal-1-boostMoveButton", "adjustGoal-1-remove",
        "adjustGoal-2-opponentMoveButton", "adjustGoal-2-hitsAndChance", "adjustGoal-2-hitsMinus", "adjustGoal-2-hits",
        "adjustGoal-2-hitsPlus", "adjustGoal-2-threshold-100", "adjustGoal-2-threshold-90", "adjustGoal-2-threshold-75",
        "adjustGoal-2-threshold-50", "adjustGoal-3-ownMoveButton",
    ]
    private static let resultIdentifiers = [
        "adjustResultCard", "adjustGoalsPlanSP", "adjustGoalsPlanTotal", "adjustGoalsPlanStats", "adjustRemaining",
        "adjustGoalOutcome-1", "adjustGoalOutcome-2", "adjustGoalOutcome-3",
    ]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func tap(_ app: XCUIApplication, _ identifier: String) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い")
        scrollUntilHittable(app, target)
        target.tap()
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, maxAttempts: Int = 20) {
        var attempts = 0
        while !target.isHittable && attempts < maxAttempts {
            // 調整画面の中ではその ScrollView を、ルート画面ではアプリ全体を上へ送る。
            let screen = element(app, "adjustScreen")
            if screen.exists { screen.swipeUp() } else { app.swipeUp() }
            attempts += 1
        }
    }

    private func pickSpecies(_ app: XCUIApplication, button: String) {
        tap(app, button)
        let row = element(app, "speciesSearchResult-9002-000")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        row.tap()
    }

    private func pickMove(_ app: XCUIApplication, button: String, moveId: String) {
        tap(app, button)
        let option = element(app, "adjustGoalMoveOption-\(moveId)")
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()
    }

    /// 3種類の目標(素早さ・耐える・倒す)を足して全部入力し、送信して結果を出す。
    private func buildThreeGoalsAndSubmit(_ app: XCUIApplication) {
        tap(app, "openAdjustScreen")
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "adjustOwnSpeciesButton")
        let own = element(app, "speciesSearchResult-9001-000")
        XCTAssertTrue(own.waitForExistence(timeout: Self.existenceTimeout))
        own.tap()
        tap(app, "adjustOwnNaturePicker")
        let nature = app.buttons["テストせいかく無補正"].firstMatch
        XCTAssertTrue(nature.waitForExistence(timeout: Self.existenceTimeout))
        nature.tap()
        tap(app, "adjustMode-goals")

        tap(app, "adjustAddGoalButton")
        pickSpecies(app, button: "adjustGoal-1-opponentButton")
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-2-kind-survive")
        pickSpecies(app, button: "adjustGoal-2-opponentButton")
        pickMove(app, button: "adjustGoal-2-opponentMoveButton", moveId: "test-move-special-b")
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-3-kind-ko")
        pickSpecies(app, button: "adjustGoal-3-opponentButton")
        pickMove(app, button: "adjustGoal-3-ownMoveButton", moveId: "test-move-physical-a")
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustGoalOutcome-3").waitForExistence(timeout: Self.resultTimeout))
    }

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
                if frame.minX < -Self.overflowTolerance || frame.maxX > windowFrame.maxX + Self.overflowTolerance {
                    offenders.append("\(identifier)[\(index)] label=\(el.label): frame=\(frame)")
                }
            }
        }
        XCTAssertTrue(offenders.isEmpty, "横にはみ出している要素があります(window=\(windowFrame)):\n" + offenders.joined(separator: "\n"),
                      file: file, line: line)
    }

    private func run(contentSizeCategory: String?) {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        buildThreeGoalsAndSubmit(app)
        assertNoHorizontalOverflow(app, identifiers: Self.inputIdentifiers)
        assertNoHorizontalOverflow(app, identifiers: Self.resultIdentifiers)
    }

    func testGoalsNoHorizontalOverflowAtDefaultSize() {
        run(contentSizeCategory: nil)
    }

    func testGoalsNoHorizontalOverflowAtAX5() {
        run(contentSizeCategory: Self.ax5ContentSizeCategory)
    }
}
