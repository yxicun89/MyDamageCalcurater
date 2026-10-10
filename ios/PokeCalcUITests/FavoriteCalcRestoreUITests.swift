import XCTest

/// F-09: お気に入りから計算の入力を復元して結果をすぐ出す(ADR-0524)。
/// `POKECALC_MOCK_FAVORITES=calc`: id 402「雨で攻撃」(9003 → 9001・とくしゅA・雨・計算つき)/
/// 401「技が消えた」(計算つき・マスタに無い技)/ 400「旧お気に入り」(calc なし・9003)。
/// 計算画面でのお気に入り追加(calc つき)→ 一覧 → 開く、までを通しで確かめる。
@MainActor
final class FavoriteCalcRestoreUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(favorites: String? = nil, contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let favorites { app.launchEnvironment["POKECALC_MOCK_FAVORITES"] = favorites }
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

    private func waitForLabel(_ target: XCUIElement, contains text: String) {
        expectation(for: NSPredicate(format: "label CONTAINS %@", text), evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 8 {
            app.swipeUp()
            attempts += 1
        }
    }

    // MARK: - 一覧

    func testRowsWithCalcHaveUseButtonAndLegacyRowKeepsOldHint() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        XCTAssertTrue(element(app, "favoriteRow-402").waitForExistence(timeout: Self.existenceTimeout))
        let use = app.buttons["favoriteUseButton-402"]
        XCTAssertTrue(use.exists)
        XCTAssertEqual(use.label, "「雨で攻撃」を計算に使う")
        XCTAssertTrue(app.buttons["favoriteUseButton-401"].exists)
        XCTAssertFalse(app.buttons["favoriteUseButton-400"].exists, "calc の無い旧お気に入りに「計算に使う」は出さない")
        XCTAssertTrue(element(app, "favoriteLegacyHint-400").exists)
        XCTAssertTrue(app.buttons["favoriteDeleteButton-402"].exists, "外す導線は残る")
    }

    // MARK: - 復元

    func testUseButtonOpensCalcScreenWithRestoredInputsAndResult() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        let use = app.buttons["favoriteUseButton-402"]
        XCTAssertTrue(use.waitForExistence(timeout: Self.existenceTimeout))
        use.tap()

        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let notice = element(app, "favoriteRestoreNotice")
        XCTAssertTrue(notice.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(notice.label.contains("「雨で攻撃」の計算を開きました"), "label: \(notice.label)")
        XCTAssertTrue(notice.label.contains("防御側の性格・能力ポイント・持ち物は戻しません"), "復元しない範囲を明記: \(notice.label)")
        waitForLabel(element(app, "attackerSpeciesPicker"), contains: "テストモンさん")
        waitForLabel(element(app, "defenderSpeciesPicker"), contains: "テストモンいち")
        waitForLabel(element(app, "movePicker"), contains: "テストわざとくしゅA")
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "結果がすぐ出る")
        XCTAssertFalse(element(app, "calcErrorMessage").exists)
    }

    func testUnresolvableMoveShowsCalcFailureAndNoRestoreNotice() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        let use = app.buttons["favoriteUseButton-401"]
        XCTAssertTrue(use.waitForExistence(timeout: Self.existenceTimeout))
        use.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "calcErrorMessage").waitForExistence(timeout: Self.existenceTimeout), "計算の失敗として表示する")
        XCTAssertFalse(element(app, "favoriteRestoreNotice").exists, "失敗したら「開きました」を出さない")
        waitForLabel(element(app, "attackerSpeciesPicker"), contains: "テストモンいち")
    }

    // MARK: - 追加(calc つき)→ 開く

    func testPinningFromCalcScreenKeepsCalcAndRowOpensIt() {
        let app = launch()
        let openCalc = app.buttons["openCalcScreen"]
        XCTAssertTrue(openCalc.waitForExistence(timeout: Self.existenceTimeout))
        openCalc.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let pin = app.buttons["pinAttackerFavoriteButton"]
        scrollTo(pin, in: app)
        pin.tap()
        let status = element(app, "favoritePinStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(status.label.contains("お気に入りに追加しました"), "label: \(status.label)")
        goBack(app)

        openFavorites(app)
        let use = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH %@", "favoriteUseButton-")).firstMatch
        XCTAssertTrue(use.waitForExistence(timeout: Self.existenceTimeout), "calc つきで保存されたので「計算に使う」が出る")
        XCTAssertTrue(use.label.contains("→"), "見出しは「攻撃側→防御側(技)」: \(use.label)")
        use.tap()
        XCTAssertTrue(element(app, "favoriteRestoreNotice").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - AX5

    func testAX5NoHorizontalOverflowAndTapSizes() {
        let app = launch(favorites: "calc", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        let window = app.windows.firstMatch.frame
        for identifier in ["favoriteRow-402", "favoriteUseButton-402", "favoriteDeleteButton-402", "favoriteLegacyHint-400"] {
            let target = element(app, identifier)
            scrollTo(target, in: app)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier)
            let frame = target.frame
            XCTAssertGreaterThanOrEqual(frame.minX, -1, "\(identifier) が左に切れている \(frame)")
            XCTAssertLessThanOrEqual(frame.maxX, window.maxX + 1, "\(identifier) が右に切れている \(frame)")
            if identifier.hasPrefix("favoriteUseButton") || identifier.hasPrefix("favoriteDeleteButton") {
                XCTAssertGreaterThanOrEqual(frame.height, Self.minTapSize, "\(identifier) のタップ領域")
            }
        }
        let use = app.buttons["favoriteUseButton-402"]
        scrollTo(use, in: app)
        use.tap()
        XCTAssertTrue(element(app, "favoriteRestoreNotice").waitForExistence(timeout: Self.existenceTimeout))
        let notice = element(app, "favoriteRestoreNotice").frame
        XCTAssertGreaterThanOrEqual(notice.minX, -1)
        XCTAssertLessThanOrEqual(notice.maxX, window.maxX + 1, "復元の案内が右に切れている \(notice)")
    }
}
