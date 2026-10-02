import XCTest

/// P6-23: 種族ピッカーの「よく使う相手」(ADR-0501「P6-23」)。モックの挙動は起動時の環境変数
/// `POKECALC_MOCK_FREQUENT_OPPONENTS`(`empty` / `fail`。`MockFrequentOpponentsService.scenarioEnvironmentKey`)で切り替える。
/// 既定のモックは 9003-000(テストモンさん)・9001-000(テストモンいち)・9999-000(マスタに無い)をこの順で返す。
/// 識別子と架空の key を直接書く(UI テストは App のターゲットに依存しない)。実装前は identifier が無いので失敗してよい。
@MainActor
final class FrequentOpponentsUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let firstKey = "9003-000"
    private static let firstName = "テストモンさん"
    private static let secondKey = "9001-000"
    private static let unresolvableKey = "9999-000"
    private static let section = "frequentOpponentsSection"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(scenario: String? = nil, open screen: String) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_FREQUENT_OPPONENTS"] = scenario }
        app.launch()
        let open = app.buttons["open\(screen)Screen"]
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "\(screen.prefix(1).lowercased() + screen.dropFirst())Screen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func openSheet(_ app: XCUIApplication, picker: String) {
        let button = element(app, picker)
        XCTAssertTrue(button.waitForExistence(timeout: Self.existenceTimeout))
        button.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
    }

    /// 計算画面: 防御側のピッカーを開くと、空クエリのとき先頭に「よく使う」が出る(解決できない key は出ない)。
    func testDefenderPickerShowsFrequentOpponentsInServerOrder() {
        let app = launch(open: "Calc")
        openSheet(app, picker: "defenderSpeciesPicker")

        XCTAssertTrue(element(app, Self.section).waitForExistence(timeout: Self.existenceTimeout))
        let first = element(app, "frequentOpponentRow-\(Self.firstKey)")
        let second = element(app, "frequentOpponentRow-\(Self.secondKey)")
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(second.exists)
        XCTAssertFalse(element(app, "frequentOpponentRow-\(Self.unresolvableKey)").exists, "解決できない key は黙って省く")
        XCTAssertLessThan(first.frame.minY, second.frame.minY, "サーバーの順(スコア降順)")
        XCTAssertTrue(element(app, "speciesSearchResult-\(Self.firstKey)").exists, "通常の一覧もそのまま出る")
    }

    /// 選ぶと通常の検索結果の選択と同じに反映される(シートが閉じ、防御側が変わる)。
    func testSelectingFrequentOpponentSetsDefenderAndClosesSheet() {
        let app = launch(open: "Calc")
        let picker = element(app, "defenderSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertNotEqual(picker.label, Self.firstName, "前提: 既定の防御側は別の種族")
        openSheet(app, picker: "defenderSpeciesPicker")

        let row = element(app, "frequentOpponentRow-\(Self.firstKey)")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout))
        row.tap()

        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
        let changed = NSPredicate(format: "label == %@", Self.firstName)
        expectation(for: changed, evaluatedWith: picker, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    /// 検索語を打つと「よく使う」は消え(検索結果に紛れさせない)、消すと戻る。
    func testSectionHidesWhileSearchingAndReturnsWhenQueryCleared() {
        let app = launch(open: "Calc")
        openSheet(app, picker: "defenderSpeciesPicker")
        XCTAssertTrue(element(app, Self.section).waitForExistence(timeout: Self.existenceTimeout))

        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(Self.firstName)
        XCTAssertTrue(element(app, "speciesSearchResult-\(Self.firstKey)").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, Self.section).exists, "検索中は出さない")
        XCTAssertFalse(element(app, "frequentOpponentRow-\(Self.firstKey)").exists)

        // システムの検索欄の「Clear text」ボタンは field の子として見つからないので、削除キーで消す。
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: Self.firstName.count))
        XCTAssertTrue(element(app, Self.section).waitForExistence(timeout: Self.existenceTimeout))
    }

    /// 攻撃側のピッカーには出さない(ADR-0501「P6-23」2章。頻度は防御側=相手の記録)。
    func testAttackerPickerHasNoFrequentOpponents() {
        let app = launch(open: "Calc")
        openSheet(app, picker: "attackerSpeciesPicker")
        XCTAssertTrue(element(app, "speciesSearchResult-\(Self.firstKey)").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, Self.section).exists)
    }

    /// 逆算画面: 相手のピッカーには出し、自分のピッカーには出さない。
    func testReverseOpponentPickerShowsFrequentOpponentsButMineDoesNot() {
        let app = launch(open: "Reverse")
        openSheet(app, picker: "reverseOpponentSpeciesPicker")
        XCTAssertTrue(element(app, Self.section).waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "frequentOpponentRow-\(Self.firstKey)").exists)
        element(app, "speciesSearchCancelButton").tap()
        XCTAssertFalse(element(app, "speciesSearchSheet").waitForExistence(timeout: 1))

        openSheet(app, picker: "reverseMySpeciesPicker")
        XCTAssertTrue(element(app, "speciesSearchResult-\(Self.firstKey)").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, Self.section).exists)
    }

    /// 空配列・取得失敗のとき、セクションは出ず、検索 UI は普通に使え、計算も成功する(絶対ルール5)。
    func testEmptyAndFailureShowNoSectionAndKeepSearchAndCalcWorking() {
        for scenario in ["empty", "fail"] {
            let app = launch(scenario: scenario, open: "Calc")
            XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "\(scenario): 計算は成功する")
            openSheet(app, picker: "defenderSpeciesPicker")
            XCTAssertTrue(element(app, "speciesSearchResult-\(Self.firstKey)").waitForExistence(timeout: Self.existenceTimeout), scenario)
            XCTAssertFalse(element(app, Self.section).exists, "\(scenario): セクションを出さない")
            XCTAssertFalse(element(app, "frequentOpponentRow-\(Self.firstKey)").exists, scenario)
            app.terminate()
        }
    }
}
