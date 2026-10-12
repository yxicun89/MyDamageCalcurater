import XCTest

/// G-05(ADR-0536。ADR-0501「G-05 このアプリについて・お気に入り・履歴」章): 「このアプリについて」「お気に入り・履歴」
/// 「お気に入りから読み込む」「お気に入りに追加」を、文で説明せず、アイコン・短い名前・押せる行で分からせる作りに直した結果を、
/// 画面の要素の有無とタップの大きさ(36pt 以上)・AX5 のはみ出しで確かめる。文面そのものは `G05AboutFavoritesTextTests` が固定する。
/// 識別子は実装前は無いので失敗してよい(新しい識別子は ADR の表)。モックの切り替えは `FavoritesScreenUITests` の冒頭と同じ。
@MainActor
final class AboutFavoritesG05UITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(
        favorites: String? = nil, opponents: String? = nil, calcHistory: String? = nil,
        contentSizeCategory: String? = nil
    ) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let favorites { app.launchEnvironment["POKECALC_MOCK_FAVORITES"] = favorites }
        if let opponents { app.launchEnvironment["POKECALC_MOCK_FREQUENT_OPPONENTS"] = opponents }
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
        app.openScreen("openFavoritesScreen")
        XCTAssertTrue(element(app, "favoritesScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func openAbout(_ app: XCUIApplication) {
        app.openScreen("openAboutScreen")
        XCTAssertTrue(element(app, "aboutScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 10 {
            app.swipeUp()
            attempts += 1
        }
    }

    private func assertTapTarget(_ target: XCUIElement, _ name: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(name) が無い", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(name) の高さ", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(name) の幅", file: file, line: line)
    }

    private func assertInsideWindow(_ app: XCUIApplication, _ identifiers: [String], file: StaticString = #filePath, line: UInt = #line) {
        let width = app.windows.firstMatch.frame.width
        for identifier in identifiers {
            let target = element(app, identifier)
            scrollTo(target, in: app)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier, file: file, line: line)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(identifier) が左にはみ出す", file: file, line: line)
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(identifier) が右にはみ出す", file: file, line: line)
        }
    }

    // MARK: - このアプリについて

    /// 3 つの節に見出し(アイコン + 名前)がある。非公式表示・データの出典・データの扱い。文面(非公式の注記・出典 4 件)は変わらない。
    func testAboutHasThreeSectionHeadingsAndKeepsBodyText() {
        let app = launch()
        openAbout(app)
        for heading in ["非公式表示", "データの出典", "データの扱い"] {
            let title = app.staticTexts[heading].firstMatch
            scrollTo(title, in: app)
            XCTAssertTrue(title.waitForExistence(timeout: Self.existenceTimeout), "見出し「\(heading)」が無い")
        }
        XCTAssertTrue(element(app, "aboutUnofficialNotice").exists)
        XCTAssertTrue(element(app, "aboutUnofficialNotice").label.contains("非公式"))
        for index in 0..<4 {
            XCTAssertTrue(element(app, "aboutDataSource-\(index)").exists, "出典 \(index)")
        }
        // データの扱いの説明は 1 文ずつの箇条(3 要素 × 2 文 = 6 本。識別子は従来の deviceDataExplanation-<n>)。
        for index in 0..<6 {
            XCTAssertTrue(element(app, "deviceDataExplanation-\(index)").exists, "説明 \(index)")
        }
    }

    /// 削除は危険ボタン(ごみ箱アイコン + 名前)で、確認は 2 段階のまま。確認の「削除する」とキャンセルも 36pt 以上。
    func testAboutDeleteConfirmationIsTwoStepAndTappable() {
        let app = launch()
        openAbout(app)
        let delete = app.buttons["deleteDeviceDataButton"]
        scrollTo(delete, in: app)
        assertTapTarget(delete, "削除ボタン")
        XCTAssertTrue(delete.label.contains("この端末のデータを削除"), "名前は従来のまま: \(delete.label)")
        delete.tap()

        let confirm = app.buttons["confirmDeleteDeviceDataButton"]
        let cancel = app.buttons["cancelDeleteDeviceDataButton"]
        XCTAssertTrue(confirm.waitForExistence(timeout: Self.existenceTimeout))
        assertTapTarget(confirm, "削除する")
        assertTapTarget(cancel, "キャンセル")
        XCTAssertTrue(element(app, "deviceDataConfirmHeading").exists, "確認の見出し(注意のアイコン + 文)")
        cancel.tap()
        XCTAssertFalse(confirm.waitForExistence(timeout: 1), "キャンセルで確認が閉じる")
        XCTAssertTrue(app.buttons["deleteDeviceDataButton"].waitForExistence(timeout: Self.existenceTimeout))
    }

    /// AX5: 3 つの節・出典・削除と確認が横にはみ出さず、押せる。
    func testAboutNoHorizontalOverflowAtAX5() {
        let app = launch(contentSizeCategory: Self.ax5ContentSizeCategory)
        openAbout(app)
        assertInsideWindow(
            app, ["aboutUnofficialNotice", "aboutDataSource-0", "aboutDataSource-3", "deviceDataSection", "deleteDeviceDataButton"])
        let delete = app.buttons["deleteDeviceDataButton"]
        scrollTo(delete, in: app)
        delete.tap()
        assertInsideWindow(app, ["confirmDeleteDeviceDataButton", "cancelDeleteDeviceDataButton", "deviceDataConfirmHeading"])
        assertTapTarget(app.buttons["confirmDeleteDeviceDataButton"], "削除する(AX5)")
    }

    // MARK: - お気に入り: 空の状態(アイコン・一言・行動のボタン)

    func testEmptyFavoritesIsOneLineWithCalcButton() {
        let app = launch(opponents: "empty", calcHistory: "empty")
        openFavorites(app)
        let empty = element(app, "favoritesEmpty")
        XCTAssertTrue(empty.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(empty.label, "お気に入りはまだありません", "一言だけ(追加のしかたの文は置かない)")
        let calc = app.buttons["favoritesEmptyCalcButton"]
        assertTapTarget(calc, "お気に入りの空の「計算へ」")
        XCTAssertEqual(element(app, "calcHistoryEmpty").label, "計算履歴はまだありません")
        XCTAssertEqual(element(app, "opponentHistoryEmpty").label, "計算した相手がまだありません")
        assertTapTarget(app.buttons["calcHistoryEmptyCalcButton"], "履歴の空の「計算へ」")

        calc.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout), "計算へ移る")
    }

    // MARK: - お気に入りの行

    /// 行は 1 つのボタン(行を押すと計算に使う)。名前・個体だけの印・件数が出る。
    func testFavoriteRowIsOneUseButtonWithCountAndLegacyBadge() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        let row = element(app, "favoriteRow-402")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        let use = app.buttons["favoriteUseButton-402"]
        assertTapTarget(use, "行の使うボタン")
        XCTAssertEqual(use.label, "「雨で攻撃」を計算に使う")
        XCTAssertGreaterThanOrEqual(use.frame.width, row.frame.width * 0.5, "行全体を押せる(ボタンは行の幅の半分以上)")
        let count = element(app, "favoritesCount")
        XCTAssertTrue(count.waitForExistence(timeout: Self.existenceTimeout), "件数の表示")
        XCTAssertEqual(count.label, "3/100件")
        XCTAssertEqual(element(app, "favoriteLegacyHint-400").label, "個体だけ(計算の条件なし)")
        XCTAssertFalse(element(app, "favoritesLimitNote").exists, "上限の文は置かず、件数の数値で示す")
        assertTapTarget(app.buttons["favoriteDeleteButton-402"], "削除")
        XCTAssertEqual(app.buttons["favoriteDeleteButton-402"].label, "「雨で攻撃」を削除")
    }

    /// 削除は 2 段階: 確認の文(取り消せない操作)→「削除する」。やめる、で行は残る。
    func testFavoriteDeleteIsTwoStepAndCancelKeepsRow() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        app.buttons["favoriteDeleteButton-402"].tap()
        let message = element(app, "favoriteDeleteConfirm-402")
        XCTAssertTrue(message.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(message.label.contains("よろしいですか?"), "label: \(message.label)")
        assertTapTarget(app.buttons["favoriteDeleteConfirmButton-402"], "削除する")
        let cancel = app.buttons["favoriteDeleteCancelButton-402"]
        assertTapTarget(cancel, "やめる")
        cancel.tap()
        XCTAssertFalse(message.waitForExistence(timeout: 1), "やめると確認が閉じる")
        XCTAssertTrue(element(app, "favoriteRow-402").exists, "行は残る")
    }

    // MARK: - 計算履歴

    /// 補足の文は「説明」ボタンの奥。最初は出ず、開くと出る。
    func testCalcHistoryNoteIsBehindHelpButton() {
        let app = launch()
        openFavorites(app)
        XCTAssertTrue(element(app, "calcHistoryRow-0-1790000000").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "calcHistoryNote").exists, "補足の文を出し続けない")
        let help = app.buttons["calcHistoryHelpButton"]
        scrollTo(help, in: app)
        assertTapTarget(help, "履歴の説明ボタン")
        XCTAssertTrue(help.label.contains("説明"), "label: \(help.label)")
        help.tap()
        let note = element(app, "calcHistoryNote")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(note.label.contains("少し遅れて"), "label: \(note.label)")
    }

    /// 行は 1 つのボタンで、名前・技・結果の%・日付が読める。読み込み中は回り続ける ProgressView を出さない。
    func testCalcHistoryRowKeepsLabelAndNoSpinnerAfterLoad() {
        let app = launch()
        openFavorites(app)
        let row = app.buttons["calcHistoryRow-0-1790000000"]
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(row.label.contains("→"), row.label)
        XCTAssertTrue(row.label.contains("%"), row.label)
        XCTAssertEqual(app.activityIndicators.count, 0, "回り続ける読み込み表示を置かない(静止したアイコン + 一言)")
    }

    // MARK: - お気に入りから読み込む(計算タブ)

    private func openLoadSheet(_ app: XCUIApplication, side: String) {
        app.openScreen("openCalcScreen")
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let entry = element(app, "\(side)FavoriteSourceButton")
        for _ in 0..<14 where !(entry.exists && entry.isHittable) { element(app, "calcScreen").swipeUp() }
        entry.tap()
        XCTAssertTrue(element(app, "favoriteLoadSheet").waitForExistence(timeout: Self.existenceTimeout))
    }

    func testLoadEntryAndSheetHeaderAreShortAndTappable() {
        let app = launch(favorites: "loadable")
        openLoadSheet(app, side: "attacker")
        // 閉じるはアイコンだけ。読み上げ名は用語集の「閉じる」、36pt 以上。
        let close = app.buttons["favoriteLoadClose"]
        assertTapTarget(close, "閉じる")
        XCTAssertEqual(close.label, "閉じる")
        // 注記は「説明」ボタンの奥。
        let help = app.buttons["favoriteLoadHelpButton"]
        assertTapTarget(help, "説明ボタン")
        XCTAssertFalse(element(app, "favoriteLoadNote").exists)
        // 行は 1 行 1 ボタン(画像かエンブレム + 名前 + 副の情報)。
        let row = app.buttons["favoriteLoadRow-304"]
        assertTapTarget(row, "行")
        close.tap()
        XCTAssertFalse(element(app, "favoriteLoadSheet").waitForExistence(timeout: 1))
        let entry = element(app, "attackerFavoriteSourceButton")
        XCTAssertEqual(entry.label, "攻撃側のお気に入り", "入口の名前は短い")
    }

    func testLoadSheetEmptyIsOneLineWithAction() {
        let app = launch(favorites: nil)
        openLoadSheet(app, side: "attacker")
        let empty = element(app, "favoriteLoadEmpty")
        XCTAssertTrue(empty.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(empty.label, "お気に入りはまだありません")
        XCTAssertEqual(app.activityIndicators.count, 0, "回り続ける読み込み表示を置かない")
    }

    // MARK: - 復元の案内

    /// 「…の計算を開きました」の 1 行 + 「説明」ボタン。限界の文は説明の奥。ボタンは 36pt 以上。
    func testRestoreNoticeKeepsOneLineAndHelpButton() {
        let app = launch(favorites: "calc")
        openFavorites(app)
        app.buttons["favoriteUseButton-402"].tap()
        let notice = element(app, "favoriteRestoreNotice")
        XCTAssertTrue(notice.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(notice.label.contains("「雨で攻撃」の計算を開きました"), notice.label)
        assertTapTarget(app.buttons["favoriteRestoreHelpButton"], "復元の説明ボタン")
        XCTAssertFalse(element(app, "favoriteRestoreLimitNote").exists)
    }

    // MARK: - 追加の結果

    /// 「お気に入りに追加」の結果は 1 行の状態(アイコン + 一言)。ボタンは 36pt 以上。
    func testPinButtonsAndStatusLine() {
        let app = launch()
        app.openScreen("openCalcScreen")
        let pin = app.buttons["pinDefenderFavoriteButton"]
        XCTAssertTrue(pin.waitForExistence(timeout: Self.existenceTimeout))
        scrollTo(pin, in: app)
        assertTapTarget(pin, "防御側を追加")
        pin.tap()
        let status = element(app, "favoritePinStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(status.label.contains("。\n"), "状態は 1 行")
    }

    // MARK: - AX5

    func testFavoritesScreenNewPartsNoHorizontalOverflowAtAX5() {
        let app = launch(favorites: "calc", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        assertInsideWindow(
            app,
            [
                "favoritesCount", "favoriteRow-402", "favoriteUseButton-402", "favoriteDeleteButton-402",
                "favoriteLegacyHint-400", "calcHistoryHelpButton",
            ])
        let delete = app.buttons["favoriteDeleteButton-402"]
        scrollTo(delete, in: app)
        delete.tap()
        assertInsideWindow(
            app, ["favoriteDeleteConfirm-402", "favoriteDeleteConfirmButton-402", "favoriteDeleteCancelButton-402"])
        assertTapTarget(app.buttons["favoriteDeleteConfirmButton-402"], "削除する(AX5)")
    }

    func testEmptyStatesNoHorizontalOverflowAtAX5() {
        let app = launch(opponents: "empty", calcHistory: "empty", contentSizeCategory: Self.ax5ContentSizeCategory)
        openFavorites(app)
        assertInsideWindow(
            app,
            [
                "favoritesEmpty", "favoritesEmptyCalcButton", "calcHistoryEmpty", "calcHistoryEmptyCalcButton",
                "opponentHistoryEmpty",
            ])
        assertTapTarget(app.buttons["favoritesEmptyCalcButton"], "計算へ(AX5)")
    }

    func testLoadSheetNoHorizontalOverflowAtAX5() {
        let app = launch(favorites: "loadable", contentSizeCategory: Self.ax5ContentSizeCategory)
        openLoadSheet(app, side: "attacker")
        assertInsideWindow(app, ["favoriteLoadSheet", "favoriteLoadHelpButton", "favoriteLoadRow-304", "favoriteLoadClose"])
        let help = app.buttons["favoriteLoadHelpButton"]
        help.tap()
        assertInsideWindow(app, ["favoriteLoadNote"])
        assertTapTarget(app.buttons["favoriteLoadClose"], "閉じる(AX5)")
    }
}
