import XCTest

/// お気に入り・計算履歴画面(ADR-0509・ADR-0501「お気に入り・計算履歴の受け入れ条件」)。
/// モックの挙動は環境変数で切り替える。
/// - `POKECALC_MOCK_FAVORITES`: 未設定=空のストア(追加・削除は動く)/ `list`(id 102「HB特化」9002-000・id 101 9003-000)/
///   `fail` / `unavailable` / `full`(100件)。
/// - `POKECALC_MOCK_FREQUENT_OPPONENTS`: 未設定=3件(9003-000・9001-000・マスタに無い 9999-000)/ `empty` / `fail`。
/// 識別子と架空の key を直接書く(UI テストは App のターゲットに依存しない)。実装前は identifier が無いので失敗してよい。
@MainActor
final class FavoritesScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// AX5(アクセシビリティ域の最大)。
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    /// タップ範囲の下限(pt。design.md・ADR-0501 の規約)。
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(
        favorites: String? = nil, opponents: String? = nil, contentSizeCategory: String? = nil
    ) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let favorites { app.launchEnvironment["POKECALC_MOCK_FAVORITES"] = favorites }
        if let opponents { app.launchEnvironment["POKECALC_MOCK_FREQUENT_OPPONENTS"] = opponents }
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

    private func goBack(_ app: XCUIApplication) {
        let back = app.navigationBars.buttons.element(boundBy: 0)
        XCTAssertTrue(back.waitForExistence(timeout: Self.existenceTimeout))
        back.tap()
    }

    private func openCalc(_ app: XCUIApplication) {
        let open = app.buttons["openCalcScreen"]
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 既定(空のストア + 履歴3件)

    /// 既定: お気に入りは空(案内文)、よく計算する相手は3件(サーバーの順)。マスタに無い key は「不明なポケモン」で残る。
    func testDefaultShowsEmptyFavoritesAndOpponentHistory() {
        let app = launch()
        openFavorites(app)

        XCTAssertTrue(element(app, "favoritesSection").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "favoritesEmpty").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "opponentHistorySection").exists)
        let first = element(app, "opponentHistoryRow-9003-000")
        let second = element(app, "opponentHistoryRow-9001-000")
        let unknown = element(app, "opponentHistoryRow-9999-000")
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(second.exists)
        XCTAssertTrue(unknown.exists, "名前を引けない相手も、件数があるので行を残す")
        XCTAssertTrue(unknown.label.contains("不明なポケモン"), "label: \(unknown.label)")
        XCTAssertLessThan(first.frame.minY, second.frame.minY, "サーバーの順(スコア降順)")
        XCTAssertTrue(first.label.contains("テストモンさん"), "label: \(first.label)")
        XCTAssertTrue(first.label.contains("3回"), "件数を出す: \(first.label)")
        XCTAssertTrue(element(app, "opponentHistoryPendingNote").exists, "生の履歴一覧は契約待ちの注記")
    }

    // MARK: - お気に入りの一覧・削除

    func testListShowsFavoritesNewestFirstAndRemoveDropsOnlyThatRow() {
        let app = launch(favorites: "list")
        openFavorites(app)
        let newer = element(app, "favoriteRow-102")
        let older = element(app, "favoriteRow-101")
        XCTAssertTrue(newer.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(older.exists)
        XCTAssertLessThan(newer.frame.minY, older.frame.minY, "更新の新しい順")
        XCTAssertTrue(newer.label.contains("HB特化"), "label: \(newer.label)")
        XCTAssertTrue(newer.label.contains("テストモンに"), "ラベルがあるときは種族名も添える: \(newer.label)")
        XCTAssertTrue(older.label.contains("テストモンさん"), "ラベルが無いときは種族名が題名: \(older.label)")
        XCTAssertFalse(element(app, "favoritesEmpty").exists)

        let remove = app.buttons["favoriteDeleteButton-102"]
        XCTAssertTrue(remove.exists)
        remove.tap()

        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: element(app, "favoriteRow-102"), handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertTrue(element(app, "favoriteRow-101").exists, "他の行は残る")
        XCTAssertFalse(element(app, "favoritesActionError").exists)
    }

    func testRemovingLastFavoriteShowsEmptyGuide() {
        let app = launch(favorites: "list")
        openFavorites(app)
        for id in ["102", "101"] {
            let remove = app.buttons["favoriteDeleteButton-\(id)"]
            XCTAssertTrue(remove.waitForExistence(timeout: Self.existenceTimeout))
            remove.tap()
            let gone = NSPredicate(format: "exists == false")
            expectation(for: gone, evaluatedWith: element(app, "favoriteRow-\(id)"), handler: nil)
            waitForExpectations(timeout: Self.existenceTimeout)
        }
        XCTAssertTrue(element(app, "favoritesEmpty").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 通信失敗・503(画面を壊さない。絶対ルール5)

    func testFavoritesFailureShowsGuidanceAndRetryWhileHistoryStillShows() {
        let app = launch(favorites: "fail")
        openFavorites(app)
        let error = element(app, "favoritesError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.contains("通信に失敗しました"), "label: \(error.label)")
        XCTAssertTrue(app.buttons["favoritesRetryButton"].exists)
        XCTAssertFalse(element(app, "favoritesEmpty").exists, "失敗を「空」と見せない")
        XCTAssertTrue(
            element(app, "opponentHistoryRow-9003-000").waitForExistence(timeout: Self.existenceTimeout),
            "お気に入りの失敗が履歴の表示を巻き込まない")
    }

    func testFavoritesUnavailableSaysCalculationStillWorks() {
        let app = launch(favorites: "unavailable")
        openFavorites(app)
        let error = element(app, "favoritesError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.contains("計算はそのまま使えます"), "label: \(error.label)")
        XCTAssertFalse(error.label.contains("store_unavailable"), "code を出さない")
    }

    func testOpponentHistoryEmptyAndFailureAreShownWithoutBreakingFavorites() {
        let empty = launch(favorites: "list", opponents: "empty")
        openFavorites(empty)
        XCTAssertTrue(element(empty, "opponentHistoryEmpty").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(empty, "favoriteRow-102").exists)
        empty.terminate()

        let failing = launch(favorites: "list", opponents: "fail")
        openFavorites(failing)
        XCTAssertTrue(element(failing, "opponentHistoryError").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(failing.buttons["opponentHistoryRetryButton"].exists)
        XCTAssertFalse(element(failing, "opponentHistoryEmpty").exists, "失敗を「空」と見せない")
        XCTAssertTrue(element(failing, "favoriteRow-102").exists, "履歴の失敗がお気に入りの表示を巻き込まない")
    }

    // MARK: - 計算画面からの追加

    /// 防御側を追加 → 追加済みの通知 → 一覧に1件。同じ内容をもう一度追加すると「すでに追加済み」で、一覧は増えない。
    func testPinDefenderFromCalcThenListShowsItAndPinningAgainIsAlreadyPinned() {
        let app = launch()
        openCalc(app)
        let pin = app.buttons["pinDefenderFavoriteButton"]
        XCTAssertTrue(pin.waitForExistence(timeout: Self.existenceTimeout))
        pin.tap()
        let status = element(app, "favoritePinStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(status.label.contains("お気に入りに追加しました"), "label: \(status.label)")
        XCTAssertTrue(element(app, "calcResultRow-none@-").exists, "追加しても計算の結果は消えない")

        pin.tap()
        let already = NSPredicate(format: "label CONTAINS %@", "すでにお気に入りに追加済み")
        expectation(for: already, evaluatedWith: status, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)

        goBack(app)
        openFavorites(app)
        let rows = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH 'favoriteRow-'"))
        XCTAssertTrue(rows.firstMatch.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(rows.count, 1, "同じ内容は2つにならない")
        XCTAssertFalse(element(app, "favoritesEmpty").exists)
    }

    func testPinAttackerButtonExistsAndWorks() {
        let app = launch()
        openCalc(app)
        let pin = app.buttons["pinAttackerFavoriteButton"]
        XCTAssertTrue(pin.waitForExistence(timeout: Self.existenceTimeout))
        pin.tap()
        let status = element(app, "favoritePinStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(status.label.contains("お気に入りに追加しました"), "label: \(status.label)")
    }

    /// 上限(100件)に達していて新しい内容なら、案内を出して計算は使える。
    func testPinWhenFullShowsLimitGuidanceAndCalcKeepsWorking() {
        let app = launch(favorites: "full")
        openCalc(app)
        let pin = app.buttons["pinDefenderFavoriteButton"]
        XCTAssertTrue(pin.waitForExistence(timeout: Self.existenceTimeout))
        pin.tap()
        let status = element(app, "favoritePinStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(status.label.contains("100件"), "label: \(status.label)")
        XCTAssertTrue(element(app, "calcResultRow-none@-").exists, "追加できなくても計算は成功する")
    }

    func testPinFailureAndUnavailableKeepCalcWorkingAndShowJapaneseGuidance() {
        for (scenario, expected) in [("fail", "通信に失敗しました"), ("unavailable", "計算はそのまま使えます")] {
            let app = launch(favorites: scenario)
            openCalc(app)
            let pin = app.buttons["pinDefenderFavoriteButton"]
            XCTAssertTrue(pin.waitForExistence(timeout: Self.existenceTimeout), scenario)
            pin.tap()
            let status = element(app, "favoritePinStatus")
            XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout), scenario)
            XCTAssertTrue(status.label.contains(expected), "\(scenario): \(status.label)")
            XCTAssertTrue(element(app, "calcResultRow-none@-").exists, "\(scenario): 計算は成功する")
            app.terminate()
        }
    }

    // MARK: - タップ範囲・AX5

    /// 外すボタンは 36pt 以上(既定の文字サイズ)。
    func testRemoveButtonMeetsMinimumTapSize() {
        let app = launch(favorites: "list")
        openFavorites(app)
        let remove = app.buttons["favoriteDeleteButton-102"]
        XCTAssertTrue(remove.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertGreaterThanOrEqual(remove.frame.height, Self.minTapSize)
        XCTAssertGreaterThanOrEqual(remove.frame.width, Self.minTapSize)
    }

    /// AX5: 一覧・履歴・外すボタン・再読み込みが横にはみ出さず、外すボタンを押せる。
    func testScreenNoHorizontalOverflowAtAX5AndRemoveStillWorks() {
        let app = launch(favorites: "list", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        let width = app.windows.firstMatch.frame.width
        let identifiers = [
            "favoritesSection", "favoriteRow-102", "favoriteRow-101", "favoriteDeleteButton-102",
            "opponentHistorySection", "opponentHistoryRow-9003-000", "opponentHistoryRow-9999-000",
            "opponentHistoryPendingNote",
        ]
        for identifier in identifiers {
            let target = element(app, identifier)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(identifier) が左にはみ出す")
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(identifier) が右にはみ出す")
        }
        let remove = app.buttons["favoriteDeleteButton-102"]
        XCTAssertGreaterThanOrEqual(remove.frame.height, Self.minTapSize)
        if !remove.isHittable { app.swipeUp() }
        remove.tap()
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: element(app, "favoriteRow-102"), handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    func testFailureStateNoHorizontalOverflowAtAX5() {
        let app = launch(favorites: "unavailable", opponents: "fail", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        let width = app.windows.firstMatch.frame.width
        for identifier in ["favoritesError", "favoritesRetryButton", "opponentHistoryError", "opponentHistoryRetryButton"] {
            let target = element(app, identifier)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, identifier)
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, identifier)
        }
    }
}
