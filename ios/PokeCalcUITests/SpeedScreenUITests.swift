import XCTest

/// 素早さ画面の導線・想定のチップ・ピッカー・失敗の独立(P6-24 → I-speed-4c で新構成に整理。ADR-0503・ADR-0610 追記)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ)。素早さのモックの挙動は環境変数
/// `POKECALC_MOCK_SPEED`(`pokemon-error` / `all-error` ほか)で切り替える。
/// 画面の受け入れ条件(カード・今の振り・トリックルーム・「これ以上」・matchup の失敗)は `SpeedMatchupUITests`、
/// AX5 は `SpeedMatchupLargeTextUITests`。表・境界線・遅延描画・実数値入力・入力モードのテストは、
/// それらの部品を画面から外した(ADR-0610 §4。ADR-0517・0527 を置き換え)ため削除した。
@MainActor
final class SpeedScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 12
    private static let mockPokemonA = "9001-000"
    private static let mockPokemonB = "9002-000"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動・補助

    private func launch(scenario: String? = nil, openAtLaunch: Bool = false) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_SPEED"] = scenario }
        if openAtLaunch { app.launchEnvironment["POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH"] = "1" }
        app.launch()
        return app
    }

    private func launchSpeedScreen(scenario: String? = nil) -> XCUIApplication {
        let app = launch(scenario: scenario)
        app.openScreen("openSpeedScreen")
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout), "speedScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func wait(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    /// 遅延して描く表の段が出るまでスクロールする(見つかったら止める)。
    private func scrollUntilExists(_ app: XCUIApplication, prefix: String) {
        let first = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
        for _ in 0..<Self.maxScrolls where !first.exists {
            app.swipeUp()
        }
    }

    /// 画面外の要素をタップできる位置までスクロールする(画面は縦に長い単一の ScrollView)。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeUp()
        }
    }

    private func tap(_ app: XCUIApplication, _ identifier: String) {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        scrollUntilHittable(app, target)
        target.tap()
    }

    private func expectSelected(_ target: XCUIElement, _ selected: Bool, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        let predicate = NSPredicate(format: "isSelected == %@", NSNumber(value: selected))
        let result = XCTWaiter().wait(for: [XCTNSPredicateExpectation(predicate: predicate, object: target)], timeout: Self.existenceTimeout)
        XCTAssertEqual(result, .completed, message, file: file, line: line)
    }

    // MARK: - 導線

    func testOpensAtLaunchWithTheEnvironmentVariable() {
        let app = launch(openAtLaunch: true)
        wait(element(app, "speedScreen"), "POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH=1 で素早さ画面が開かない")
    }

    // MARK: - 想定のチップ

    /// 最後の1つは外せない(契約上 assumptions は1つ以上)。説明が出る。
    func testTheLastAssumptionCannotBeTurnedOff() {
        let app = launchSpeedScreen()
        for id in ["uninvested", "neutral-max", "max"] {
            tap(app, "speedAssumption-\(id)")
        }
        let last = element(app, "speedAssumption-max-scarf")
        wait(element(app, "speedAssumptionMinimumNotice"), "1つだけ残ったら説明が出る")
        expectSelected(last, true, "最後の1つは選ばれている")

        tap(app, "speedAssumption-max-scarf")
        expectSelected(last, true, "最後の1つは外せない")
    }

    // MARK: - ポケモンのピッカー

    func testPokemonPickerFiltersByName() {
        let app = launchSpeedScreen()
        tap(app, "speedPokemonButton")
        wait(element(app, "speedPokemonSheet"), "ポケモンのシートが開かない")
        wait(element(app, "speedPokemonRow-\(Self.mockPokemonA)"), "一覧が出る")
        XCTAssertTrue(element(app, "speedPokemonRow-\(Self.mockPokemonB)").exists)

        let field = element(app, "speedPokemonSearchField")
        wait(field, "検索欄が無い")
        field.tap()
        field.typeText("ミズ")
        wait(element(app, "speedPokemonRow-\(Self.mockPokemonB)"), "絞り込み後も一致する行は残る")
        XCTAssertTrue(
            element(app, "speedPokemonRow-\(Self.mockPokemonA)").waitForNonExistence(timeout: Self.existenceTimeout),
            "一致しない行は消える")
    }

    // MARK: - 自分のランク(メーター。ADR-0533)

    /// 13 段のメーター: 既定は「±0」・ラベル「素早さ」。+ で「+1」、両端で + が無効になる。
    func testRankMeterPlusChangesValueAndStopsAtTheEnds() {
        let app = launchSpeedScreen()
        let meter = element(app, "rankMeter-speed")
        scrollUntilHittable(app, meter)
        XCTAssertEqual(meter.value as? String, "±0")
        XCTAssertEqual(meter.label, "素早さ")
        tap(app, "rankMeterPlus-speed")
        expectation(for: NSPredicate(format: "value == %@", "+1"), evaluatedWith: meter, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        for _ in 0..<5 { tap(app, "rankMeterPlus-speed") }
        expectation(for: NSPredicate(format: "value == %@", "+6"), evaluatedWith: meter, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertFalse(element(app, "rankMeterPlus-speed").isEnabled, "+6 より上には増やせない")
    }

    // MARK: - 失敗の独立(絶対ルール5を含む)

    /// ポケモン一覧が失敗しても、画面は開いたままで失敗が表示され、他の入力は使える。
    func testPokemonFailureKeepsTheScreenAndTheOtherInputs() {
        let app = launchSpeedScreen(scenario: "pokemon-error")
        wait(element(app, "speedPokemonError"), "一覧の失敗が表示されない")
        tap(app, "speedScarf")
        expectSelected(element(app, "speedScarf"), true, "一覧が失敗していても他の入力は使える")
        tap(app, "speedTrickRoom")
        expectSelected(element(app, "speedTrickRoom"), true, "トリックルームも切り替えられる")
    }

    /// 絶対ルール5: 素早さが全面的に失敗しても、計算画面はこれまでどおり開く。
    func testCalcScreenStillWorksWhenSpeedFailsEntirely() {
        let app = launchSpeedScreen(scenario: "all-error")
        wait(element(app, "speedPokemonError"), "一覧の失敗が表示されない")
        // 素早さタブからダメージタブの計算へ移る(ADR-0531。以前はルートへ戻ってから開いた)。
        app.openScreen("openCalcScreen")
        wait(element(app, "calcScreen"), "素早さが失敗しても計算画面は開く")
    }
}
