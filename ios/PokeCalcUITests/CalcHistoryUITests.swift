import XCTest

/// 計算履歴(お気に入り・履歴画面の「計算履歴」節。ADR-0230・ADR-0519)。
/// モックの挙動は環境変数 `POKECALC_MOCK_CALC_HISTORY` で切り替える。
/// - 未設定: 3件(新しい順。0: 9002-000 → 9003-000・`test-move-special-b`・41.2〜48.9%)/ `paged`(4件を1ページ2件)/
///   `paged-fail-more`(2ページ目だけ 503)/ `empty` / `fail` / `unavailable`。
/// 行の ID は「位置-発生時刻(秒)」。モックの時刻は固定(1790000000 から1時間ずつ古い)。
/// 識別子と架空の key を直接書く(UI テストは App のターゲットに依存しない)。
@MainActor
final class CalcHistoryUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1

    private static let row0 = "calcHistoryRow-0-1790000000"
    private static let row1 = "calcHistoryRow-1-1789996400"
    private static let row2 = "calcHistoryRow-2-1789992800"
    private static let row3 = "calcHistoryRow-3-1789989200"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(calcHistory: String? = nil, contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let calcHistory { app.launchEnvironment["POKECALC_MOCK_CALC_HISTORY"] = calcHistory }
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func openFavorites(_ app: XCUIApplication) {
        let open = app.buttons["openFavoritesScreen"]
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "favoritesScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 8 {
            app.swipeUp()
            attempts += 1
        }
    }

    private func waitForDisappearance(_ target: XCUIElement) {
        expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    // MARK: - 一覧

    /// 既定: 新しい順に3行。種族名(マスタから)・技名・%幅・日付が出て、「契約待ち」の注記は無く、続きの導線も無い。
    func testDefaultShowsRowsNewestFirstWithoutPendingNoteOrLoadMore() {
        let app = launch()
        openFavorites(app)
        let first = element(app, Self.row0)
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout))
        let second = element(app, Self.row1)
        XCTAssertTrue(second.exists)
        XCTAssertTrue(element(app, Self.row2).exists)
        XCTAssertLessThan(first.frame.minY, second.frame.minY, "新しい順")
        XCTAssertTrue(first.label.contains("テストモンに → テストモンさん"), "label: \(first.label)")
        XCTAssertTrue(first.label.contains("テストわざとくしゅB"), "技名: \(first.label)")
        XCTAssertTrue(first.label.contains("41.2\u{301C}48.9%"), "%幅: \(first.label)")
        XCTAssertFalse(element(app, "opponentHistoryPendingNote").exists, "「契約待ち」の注記は出さない")
        XCTAssertFalse(app.buttons["calcHistoryLoadMoreButton"].exists, "nextCursor が無ければ「もっと見る」を出さない")
        XCTAssertFalse(element(app, "calcHistoryEmpty").exists)
        XCTAssertTrue(element(app, "opponentHistorySection").exists, "「よく計算する相手」の節も残る")
        XCTAssertGreaterThanOrEqual(first.frame.height, Self.minTapSize)
    }

    func testEmptyShowsGuidance() {
        let app = launch(calcHistory: "empty")
        openFavorites(app)
        XCTAssertTrue(element(app, "calcHistoryEmpty").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, Self.row0).exists)
        XCTAssertFalse(app.buttons["calcHistoryLoadMoreButton"].exists)
    }

    // MARK: - もっと見る

    func testLoadMoreAppendsNextPageAndThenHidesButton() {
        let app = launch(calcHistory: "paged")
        openFavorites(app)
        XCTAssertTrue(element(app, Self.row0).waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, Self.row1).exists)
        XCTAssertFalse(element(app, Self.row2).exists, "最初のページは2行だけ")
        let more = app.buttons["calcHistoryLoadMoreButton"]
        XCTAssertTrue(more.exists)
        scrollTo(more, in: app)
        more.tap()

        XCTAssertTrue(element(app, Self.row2).waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, Self.row3).exists)
        XCTAssertTrue(element(app, Self.row0).exists, "取得済みの行は残る")
        waitForDisappearance(app.buttons["calcHistoryLoadMoreButton"])
    }

    /// 続きだけの失敗(503)は一覧の場所にだけ出し、取得済みの行を残し、「もっと見る」で再試行できる。
    func testLoadMoreFailureShowsErrorInPlaceAndKeepsRows() {
        let app = launch(calcHistory: "paged-fail-more")
        openFavorites(app)
        XCTAssertTrue(element(app, Self.row0).waitForExistence(timeout: Self.existenceTimeout))
        let more = app.buttons["calcHistoryLoadMoreButton"]
        scrollTo(more, in: app)
        more.tap()
        let error = element(app, "calcHistoryMoreError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.contains("計算はそのまま使えます"), "label: \(error.label)")
        XCTAssertTrue(element(app, Self.row0).exists)
        XCTAssertTrue(element(app, Self.row1).exists)
        XCTAssertTrue(app.buttons["calcHistoryLoadMoreButton"].exists, "再試行できる")
        XCTAssertTrue(element(app, "favoritesSection").exists, "画面全体は塞がない")
    }

    // MARK: - 行から計算画面へ

    /// 行をタップすると計算画面が開き、その行の攻撃側・防御側・技が復元されて計算結果が出る。
    func testTappingRowOpensCalcScreenWithRestoredInputsAndResult() {
        let app = launch()
        openFavorites(app)
        let row = element(app, Self.row0)
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        row.tap()

        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let attacker = element(app, "attackerSpeciesPicker")
        let defender = element(app, "defenderSpeciesPicker")
        XCTAssertTrue(attacker.waitForExistence(timeout: Self.existenceTimeout))
        let attackerRestored = NSPredicate(format: "label CONTAINS %@", "テストモンに")
        expectation(for: attackerRestored, evaluatedWith: attacker, handler: nil)
        let defenderRestored = NSPredicate(format: "label CONTAINS %@", "テストモンさん")
        expectation(for: defenderRestored, evaluatedWith: defender, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertTrue(element(app, "movePicker").label.contains("テストわざとくしゅB"), "label: \(element(app, "movePicker").label)")
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "計算を出し直す")
        XCTAssertFalse(element(app, "calcErrorMessage").exists)
    }

    // MARK: - 失敗(絶対ルール5: 画面全体・計算画面は塞がない)

    func testFailureShowsErrorAndRetryOnlyInHistoryAndCalcStillWorks() {
        for scenario in ["fail", "unavailable"] {
            let app = launch(calcHistory: scenario)
            openFavorites(app)
            let error = element(app, "calcHistoryError")
            XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout), scenario)
            if scenario == "unavailable" {
                XCTAssertTrue(error.label.contains("計算はそのまま使えます"), "label: \(error.label)")
                XCTAssertFalse(error.label.contains("store_unavailable"), "code を出さない")
            } else {
                XCTAssertTrue(error.label.contains("通信に失敗しました"), "label: \(error.label)")
            }
            XCTAssertTrue(app.buttons["calcHistoryRetryButton"].exists, scenario)
            XCTAssertFalse(element(app, "calcHistoryEmpty").exists, "失敗を「空」と見せない: \(scenario)")
            XCTAssertTrue(element(app, "favoritesSection").exists, scenario)
            XCTAssertTrue(
                element(app, "opponentHistoryRow-9003-000").waitForExistence(timeout: Self.existenceTimeout),
                "履歴の失敗が「よく計算する相手」を巻き込まない: \(scenario)")

            let back = app.navigationBars.buttons.element(boundBy: 0)
            back.tap()
            let openCalc = app.buttons["openCalcScreen"]
            XCTAssertTrue(openCalc.waitForExistence(timeout: Self.existenceTimeout))
            openCalc.tap()
            XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "\(scenario): 計算は成功する")
            app.terminate()
        }
    }

    // MARK: - AX5

    func testNoHorizontalOverflowAtAX5() {
        let app = launch(calcHistory: "paged", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        let width = app.windows.firstMatch.frame.width
        for identifier in ["calcHistorySection", Self.row0, Self.row1] {
            let target = element(app, identifier)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(identifier) が左にはみ出す")
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(identifier) が右にはみ出す")
        }
        let more = app.buttons["calcHistoryLoadMoreButton"]
        scrollTo(more, in: app)
        XCTAssertTrue(more.exists)
        XCTAssertGreaterThanOrEqual(more.frame.minX, -Self.overflowTolerance)
        XCTAssertLessThanOrEqual(more.frame.maxX, width + Self.overflowTolerance)
        XCTAssertGreaterThanOrEqual(more.frame.height, Self.minTapSize)
        more.tap()
        XCTAssertTrue(element(app, Self.row2).waitForExistence(timeout: Self.existenceTimeout))
    }
}
