import XCTest

/// F-11(ADR-0525): 調整の「目標から振り方を決める」。`POKECALC_USE_MOCK=1`(ADR-0500 §5)で代表シナリオを通す。
/// 数値は `MockAdjustGoalsService` の決め打ちなので固定しない(行の存在と文の骨格だけ見る)。
/// 従来の調整(`AdjustScreenUITests`)は変えず、目標方式は追加として確かめる。
@MainActor
final class AdjustGoalsUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let resultTimeout: TimeInterval = 10

    private static let ownSpeciesKey = "9001-000"
    private static let opponentSpeciesKey = "9002-000"
    private static let neutralNatureName = "テストせいかく無補正"
    private static let physicalMoveId = "test-move-physical-a"
    private static let specialMoveId = "test-move-special-a"
    private static let opponentSpecialMoveId = "test-move-special-b"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func tap(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い", file: file, line: line)
        target.tap()
    }

    private func launchAdjustScreen(goalsScenario: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let goalsScenario { app.launchEnvironment["POKECALC_MOCK_ADJUST_GOALS"] = goalsScenario }
        app.launch()
        tap(app, "openAdjustScreen")
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func pickSpecies(_ app: XCUIApplication, button: String, key: String) {
        tap(app, button)
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "speciesSearchResult-\(key)")
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
    }

    /// 自分 = 9001・無補正。目標方式を選ぶ。
    private func selectOwnAndGoalsMode(_ app: XCUIApplication) {
        pickSpecies(app, button: "adjustOwnSpeciesButton", key: Self.ownSpeciesKey)
        tap(app, "adjustOwnNaturePicker")
        let nature = app.buttons[Self.neutralNatureName].firstMatch
        XCTAssertTrue(nature.waitForExistence(timeout: Self.existenceTimeout))
        nature.tap()
        tap(app, "adjustMode-goals")
    }

    /// 技の sheet で1つ選ぶ(選ぶと閉じる)。
    private func pickMove(_ app: XCUIApplication, button: String, moveId: String) {
        tap(app, button)
        XCTAssertTrue(element(app, "adjustGoalMoveSheet").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "adjustGoalMoveOption-\(moveId)")
        XCTAssertFalse(element(app, "adjustGoalMoveSheet").exists, "選ぶとシートが閉じる")
    }

    private func waitForLabel(_ target: XCUIElement, _ text: String, file: StaticString = #filePath, line: UInt = #line) {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label == %@", text), object: target)
        XCTAssertEqual(XCTWaiter().wait(for: [expectation], timeout: Self.existenceTimeout), .completed,
                       "「\(text)」にならない(実際: \(target.label))", file: file, line: line)
    }

    // MARK: - 入口

    /// 目標方式は「調整の内容」の先頭。選ぶと目標の領域が出て、従来の相手・目標の領域は出ない。目標が無い間は案内。
    func testGoalsModeAppearsFirstAndShowsEmptyNotice() {
        let app = launchAdjustScreen()
        XCTAssertTrue(element(app, "adjustMode-goals").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertLessThan(element(app, "adjustMode-goals").frame.minY, element(app, "adjustMode-indices").frame.minY, "先頭に置く")
        XCTAssertFalse(element(app, "adjustGoalsCard").exists, "選ぶまでは出さない")
        tap(app, "adjustMode-goals")
        XCTAssertTrue(element(app, "adjustMode-goals").isSelected)
        XCTAssertFalse(element(app, "adjustMode-indices").isSelected)
        XCTAssertTrue(element(app, "adjustGoalsCard").waitForExistence(timeout: Self.existenceTimeout))
        waitForLabel(element(app, "adjustNoGoals"), "目標を追加してください")
        XCTAssertFalse(element(app, "adjustOpponentCard").exists)
        XCTAssertFalse(element(app, "adjustGoalCard").exists, "従来の発数・確率の領域は出さない")
        tap(app, "adjustMode-minKo")
        XCTAssertFalse(element(app, "adjustGoalsCard").exists, "従来のモードに戻ると目標の領域は消える")
        XCTAssertTrue(element(app, "adjustOpponentCard").waitForExistence(timeout: Self.existenceTimeout))
    }

    func testSubmitWithoutGoalsShowsReason() {
        let app = launchAdjustScreen()
        selectOwnAndGoalsMode(app)
        tap(app, "adjustSubmitButton")
        waitForLabel(element(app, "adjustAlert"), "目標を追加してください")
        XCTAssertFalse(element(app, "adjustStatsLine").exists)
    }

    func testSubmitWithGoalMissingOpponentShowsReasonWithNumber() {
        let app = launchAdjustScreen()
        selectOwnAndGoalsMode(app)
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustSubmitButton")
        waitForLabel(element(app, "adjustAlert"), "目標 1: 相手のポケモンを選んでください")
    }

    // MARK: - 追加・種類・削除

    /// 素早さ + 倒す の2目標を足して送る。目標ごとの結果の行と、満たす振り方、指数も出る。外すと番号が詰まる。
    func testOutspeedAndKoGoalsShowOutcomesAndRemoveRenumbers() {
        let app = launchAdjustScreen()
        selectOwnAndGoalsMode(app)
        // 自分の技(倒す目標の自分の技の選択肢)は自分の learnset から。
        tap(app, "adjustAddGoalButton")
        XCTAssertTrue(element(app, "adjustGoal-1").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "adjustGoal-1-kind-outspeed").isSelected, "既定は素早さを上回る")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        XCTAssertTrue(element(app, "adjustGoal-1-preset-fastest").isSelected, "既定は最速")
        XCTAssertTrue(element(app, "adjustGoal-1-boostMoveButton").exists, "素早さには先に使う技の欄")

        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-2-kind-ko")
        XCTAssertTrue(element(app, "adjustGoal-2-kind-ko").isSelected)
        XCTAssertFalse(element(app, "adjustGoal-2-boostMoveButton").exists, "種類を変えると欄が変わる")
        pickSpecies(app, button: "adjustGoal-2-opponentButton", key: Self.opponentSpeciesKey)
        pickMove(app, button: "adjustGoal-2-ownMoveButton", moveId: Self.physicalMoveId)
        XCTAssertTrue(element(app, "adjustGoal-2-hits").exists)

        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertTrue(element(app, "adjustGoalOutcome-2").exists)
        XCTAssertTrue(element(app, "adjustGoalsPlanSP").exists)
        XCTAssertTrue(element(app, "adjustGoalsPlanTotal").exists)
        XCTAssertTrue(element(app, "adjustRemaining").exists)
        XCTAssertTrue(element(app, "adjustStatsLine").exists, "今の振り方の指数も同時に出す")
        XCTAssertFalse(element(app, "adjustGoalsInfeasible").exists)
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").label.hasPrefix("目標 1: "))
        XCTAssertTrue(element(app, "adjustGoalOutcome-2").label.hasPrefix("目標 2: "))

        tap(app, "adjustGoal-1-remove")
        XCTAssertTrue(element(app, "adjustGoal-1").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "adjustGoal-2").exists, "外すと番号を詰める")
        XCTAssertTrue(element(app, "adjustGoal-1-kind-ko").isSelected, "残ったのは元の2番目の目標")
    }

    /// 耐える目標: 相手の技を sheet で選び、発数と確率を変えられる。
    func testSurviveGoalMoveHitsAndChance() {
        let app = launchAdjustScreen()
        selectOwnAndGoalsMode(app)
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-1-kind-survive")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        XCTAssertTrue(element(app, "adjustGoal-1-preset-aFull").isSelected, "耐えるの既定は特化")
        pickMove(app, button: "adjustGoal-1-opponentMoveButton", moveId: Self.opponentSpecialMoveId)
        waitForLabel(element(app, "adjustGoal-1-hits"), "1発")
        tap(app, "adjustGoal-1-hitsPlus")
        tap(app, "adjustGoal-1-hitsPlus")
        waitForLabel(element(app, "adjustGoal-1-hits"), "3発")
        tap(app, "adjustGoal-1-hitsMinus")
        waitForLabel(element(app, "adjustGoal-1-hits"), "2発")
        tap(app, "adjustGoal-1-threshold-90")
        XCTAssertTrue(element(app, "adjustGoal-1-threshold-90").isSelected)
        XCTAssertFalse(element(app, "adjustGoal-1-threshold-100").isSelected)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").label.contains("2発耐えます"))
    }

    func testGoalLimitDisablesAddWithReason() {
        let app = launchAdjustScreen()
        selectOwnAndGoalsMode(app)
        for _ in 0..<6 { tap(app, "adjustAddGoalButton") }
        XCTAssertTrue(element(app, "adjustGoal-6").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "adjustAddGoalButton").isEnabled, "6 件で追加できない")
        waitForLabel(element(app, "adjustGoalLimitHint"), "目標は 6 つまでです")
        tap(app, "adjustGoal-6-remove")
        XCTAssertTrue(element(app, "adjustAddGoalButton").isEnabled, "外すとまた足せる")
        XCTAssertFalse(element(app, "adjustGoalLimitHint").exists)
    }

    // MARK: - 満たせない・機能なし

    func testInfeasibleResultShowsNearestPlanAndNotice() {
        let app = launchAdjustScreen(goalsScenario: "infeasible")
        selectOwnAndGoalsMode(app)
        tap(app, "adjustAddGoalButton")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").waitForExistence(timeout: Self.resultTimeout))
        waitForLabel(element(app, "adjustGoalsInfeasible"), "すべての目標は満たせませんでした")
        XCTAssertTrue(element(app, "adjustGoalOutcome-1").label.contains("先には動けません"))
    }

    /// サーバーが目標の操作を提供していない(404)とき: 案内を出し、従来の調整に戻る。画面は壊れず、従来の調整は使える。
    func testUnavailableGoalsFallsBackToClassicAdjustWithGuidance() {
        let app = launchAdjustScreen(goalsScenario: "unavailable")
        selectOwnAndGoalsMode(app)
        tap(app, "adjustAddGoalButton")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustSubmitButton")
        waitForLabel(element(app, "adjustAlert"), "目標から振り方を決める機能は今は使えません。ほかの調整の内容をお使いください")
        XCTAssertFalse(element(app, "adjustMode-goals").exists, "使えないと分かったら選択肢を出さない")
        XCTAssertFalse(element(app, "adjustGoalsCard").exists)
        XCTAssertTrue(element(app, "adjustMode-indices").isSelected, "従来の調整に戻る")
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustStatsLine").waitForExistence(timeout: Self.resultTimeout), "従来の調整は使える(絶対ルール5)")
        XCTAssertFalse(element(app, "adjustAlert").exists)
    }

    /// 一時的な失敗(503)では目標方式のまま、日本語のエラーを出す。入力は消えない。
    func testTransientFailureKeepsGoalsModeAndInputs() {
        let app = launchAdjustScreen(goalsScenario: "fail")
        selectOwnAndGoalsMode(app)
        tap(app, "adjustAddGoalButton")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustAlert").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertFalse(element(app, "adjustAlert").label.isEmpty)
        XCTAssertTrue(element(app, "adjustMode-goals").isSelected)
        XCTAssertTrue(element(app, "adjustGoal-1").exists, "入力は消さない")
    }
}
