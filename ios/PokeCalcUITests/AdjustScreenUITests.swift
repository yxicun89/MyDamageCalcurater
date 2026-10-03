import XCTest

/// AJ7: 調整画面(ADR-0502 §8・§9)。`POKECALC_USE_MOCK=1` でモックを強制する(ADR-0500 §5)。
/// 主要な操作が画面でつながることだけを見る(数値は `MockAdjustService` の決め打ちなので固定しない)。
/// 文言は ADR-0502 §6 の確定文を直接書く(UI テストは App / PokeCalcCore のターゲットに依存しない)。
@MainActor
final class AdjustScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let resultTimeout: TimeInterval = 10

    /// モックの架空データ(`Resources/species.json`・`natures.json`・`moves.json`)。
    private static let ownSpeciesKey = "9001-000"
    private static let opponentSpeciesKey = "9002-000"
    private static let neutralNatureName = "テストせいかく無補正"
    private static let physicalMoveName = "テストわざぶつりA"
    private static let multiHitMoveName = "テストわざれんぞく"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    private func tap(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い", file: file, line: line)
        target.tap()
    }

    private func launchAdjustScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        tap(app, "openAdjustScreen")
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 種族の検索シート(計算・逆算と同じ `MasterSearchSheet`)で選ぶ。
    private func pickSpecies(_ app: XCUIApplication, button: String, key: String) {
        tap(app, button)
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "speciesSearchResult-\(key)")
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
    }

    /// `Menu` の Picker で項目を選ぶ(項目は見える文字のボタン)。
    private func pickMenuItem(_ app: XCUIApplication, picker: String, item: String) {
        tap(app, picker)
        let option = app.buttons[item].firstMatch
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout), "\(item) が選択肢に無い")
        option.tap()
    }

    private func selectOwn(_ app: XCUIApplication, moveName: String? = nil) {
        pickSpecies(app, button: "adjustOwnSpeciesButton", key: Self.ownSpeciesKey)
        pickMenuItem(app, picker: "adjustOwnNaturePicker", item: Self.neutralNatureName)
        if let moveName { pickMenuItem(app, picker: "adjustOwnMovePicker", item: moveName) }
    }

    private func waitForLabel(_ target: XCUIElement, _ text: String, file: StaticString = #filePath, line: UInt = #line) {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label == %@", text), object: target)
        XCTAssertEqual(XCTWaiter().wait(for: [expectation], timeout: Self.existenceTimeout), .completed,
                       "「\(text)」にならない(実際: \(target.label))", file: file, line: line)
    }

    // MARK: - 入口と初期状態

    /// ルートから開け、開いただけでは結果が出ない(送信ボタンでだけ呼ぶ)。モードは「指数と 16n を見る」で、
    /// 相手・目標の領域は出さない。
    func testOpensFromRootWithEmptyResultAndIndicesMode() {
        let app = launchAdjustScreen()
        XCTAssertTrue(element(app, "adjustBackendModeBadge").exists, "モックで動いていることを出す")
        for identifier in ["adjustOwnCard", "adjustModeCard", "adjustResultCard", "adjustSubmitButton"] {
            XCTAssertTrue(element(app, identifier).exists, "\(identifier) が無い")
        }
        waitForLabel(element(app, "adjustEmptyResult"), "「調整する」を押すと結果が出ます")
        XCTAssertTrue(element(app, "adjustMode-indices").isSelected, "既定は指数と 16n を見る")
        XCTAssertFalse(element(app, "adjustOpponentCard").exists)
        XCTAssertFalse(element(app, "adjustGoalCard").exists)
        XCTAssertFalse(element(app, "adjustStatsLine").exists)
    }

    // MARK: - 送信前の検査

    func testSubmitWithoutOwnShowsReason() {
        let app = launchAdjustScreen()
        tap(app, "adjustSubmitButton")
        waitForLabel(element(app, "adjustAlert"), "自分のポケモンと性格を選んでください")
        XCTAssertFalse(element(app, "adjustStatsLine").exists)
    }

    func testFixedSPOverLimitShowsReason() {
        let app = launchAdjustScreen()
        selectOwn(app)
        let field = element(app, "adjustFixedSP-hp")
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText("33")
        tap(app, "adjustSubmitButton")
        waitForLabel(element(app, "adjustAlert"), "能力ポイントは0〜32の整数で入力してください")
    }

    // MARK: - 結果

    /// 指数と 16n: 実数値・火力指数(技なしは案内)・耐久指数・HP の 16n の4行を出す。
    func testIndicesSubmitShowsIndicesAndHPLines() {
        let app = launchAdjustScreen()
        selectOwn(app)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustStatsLine").waitForExistence(timeout: Self.resultTimeout))
        waitForLabel(element(app, "adjustFirepowerIndex"), "技を選ぶと出します")
        for identifier in ["adjustPhysicalBulk", "adjustSpecialBulk", "adjustIndexNote", "adjustHPCurrent"] {
            XCTAssertTrue(element(app, identifier).exists, "\(identifier) が無い")
        }
        for index in 0..<4 {
            XCTAssertTrue(element(app, "adjustHPLine-\(index)").exists, "HP の 16n の \(index) 行目が無い")
        }
        XCTAssertFalse(element(app, "adjustEmptyResult").exists)
    }

    /// 倒せる最小の振り方: 相手と目標の領域が出て、送信すると最小 SP の行を出す。
    func testMinKoModeShowsOpponentGoalAndResult() {
        let app = launchAdjustScreen()
        selectOwn(app, moveName: Self.physicalMoveName)
        tap(app, "adjustMode-minKo")
        XCTAssertTrue(element(app, "adjustOpponentCard").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "adjustGoalCard").exists)
        XCTAssertTrue(element(app, "adjustOpponentDefenderPreset-none").exists, "相手が受ける側は防御側のプリセット")
        XCTAssertFalse(element(app, "adjustOpponentMovePicker").exists, "相手が受ける側では相手の技を出さない")
        pickSpecies(app, button: "adjustOpponentSpeciesButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustModeResult").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertTrue(element(app, "adjustStatsLine").exists, "指数も同時に出す")
    }

    /// 耐久に振る: 目標なしでは「目標を指定すると…」を出す。
    func testBulkModeWithoutGoalShowsPlanAndHint() {
        let app = launchAdjustScreen()
        selectOwn(app)
        tap(app, "adjustMode-bulk")
        XCTAssertTrue(element(app, "adjustBulkFocus-both").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "adjustBulkFocus-both").isSelected, "既定の基準は両方(見える形で選択済み)")
        XCTAssertFalse(element(app, "adjustCeiling-spe").exists, "耐久側は素早さを見ない")
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustMaxIndexPlan").waitForExistence(timeout: Self.resultTimeout))
        waitForLabel(element(app, "adjustMinSpNotRequested"), "目標を指定すると、目標を満たす最小の振り方も出します")
    }

    /// 未対応の印(連続技)を結果の上に1回出す。
    func testUnsupportedMarkNoticeIsShown() {
        let app = launchAdjustScreen()
        selectOwn(app, moveName: Self.multiHitMoveName)
        tap(app, "adjustMode-minKo")
        pickSpecies(app, button: "adjustOpponentSpeciesButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustUnsupportedNotice").waitForExistence(timeout: Self.resultTimeout))
    }

    // MARK: - 技を覚えるポケモン

    /// 技を選ぶまで押せず、押すと見出しと一覧を出す。
    func testLearnersPanelOpensFromOwnMove() {
        let app = launchAdjustScreen()
        pickSpecies(app, button: "adjustOwnSpeciesButton", key: Self.ownSpeciesKey)
        let button = element(app, "adjustOwnLearnersButton")
        XCTAssertTrue(button.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(button.isEnabled, "技を選ぶまで押せない")
        pickMenuItem(app, picker: "adjustOwnMovePicker", item: Self.physicalMoveName)
        XCTAssertTrue(button.isEnabled)
        button.tap()
        waitForLabel(element(app, "adjustLearnersHeading"), "\(Self.physicalMoveName)を覚えるポケモン")
        XCTAssertTrue(elementBeginningWith(app, "adjustLearnerRow-").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertFalse(element(app, "adjustLearnersMore").exists, "架空データは1ページに収まる")
        XCTAssertFalse(element(app, "adjustStatsLine").exists, "一覧を開いても調整は呼ばない")
    }
}
