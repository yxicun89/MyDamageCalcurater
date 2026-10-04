import XCTest

/// 判定画面の「素早さの反映/無視」の表示と状態異常の選択(ADR-0512。受け入れ条件は ADR-0501「判定の素早さ反映/無視と状態異常の受け入れ条件」)。
///
/// `POKECALC_USE_MOCK=1` + `POKECALC_MOCK_JUDGE=speed-notes` で起動する。モックの固定の事実(`MockJudgeServiceSpeedNotesTests` が単体で固定):
///   自分の Applied = 全行 [rank, tailwind](自分の状態異常がまひなら末尾に paralysis)/自分の Ignored = 候補2(i = 1)の行だけ [abilityId] /
///   候補の Applied = 候補1(i = 0)だけ [choiceScarf](その候補の状態異常がまひなら末尾に paralysis)/候補の Ignored = 候補2(i = 1)だけ [itemId, fieldWeather]。
/// 既定のシナリオ(normal)では4欄とも空 → 何も出ない。
///
/// 識別子(ADR-0512。implementer はこの名前で付ける。既存の identifier は変えない):
///   状態異常の選択ボタン: judgeAttackerStatusButton / judgeCandidate<n>StatusButton(既存の NatureButton と同じ作り。選択シート `judgeOptionSheet` を開く。
///     行は `judgeOption-<値>`(値 = none|burn|paralysis|poison|badly_poison|sleep|freeze の7つ。「なし」も実の選択肢なので `judgeOptionNone` は出さない)。
///     選んだらシートが閉じ、ボタンの label に選んだ状態異常の名前が入る。既定は「なし」)
///   結果の行(空なら出さない): judgeRowAttackerSpeedApplied-<i> / judgeRowDefenderSpeedApplied-<i> / judgeRowAttackerSpeedIgnored-<i> / judgeRowDefenderSpeedIgnored-<i>
///     (i = defenderIndex。0 始まり。1つの Text に 1 識別子)
///
/// 画面の高さは機種で違う(iPhone 17e と iPhone 18 Pro の両方で通ること)。スクロールは前方に進めたあと、届かなければ下(上方向)へ戻る。
@MainActor
final class JudgeSpeedNotesUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 14
    private static let speciesOne = "9001-000"
    private static let speciesTwo = "9002-000"
    private static let speciesThree = "9003-000"
    private static let physicalMove = "test-move-physical-a"
    private static let specialMove = "test-move-special-b"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動・補助

    private func launchJudgeScreen(scenario: String? = "speed-notes") -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_JUDGE"] = scenario }
        app.launch()
        let open = element(app, "openJudgeScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openJudgeScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "judgeScreen").waitForExistence(timeout: Self.existenceTimeout), "judgeScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func wait(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    /// 画面外の要素をタップできる位置までスクロールする。前方(swipeUp)に進めて届かなければ、通り越した場合のために下(swipeDown)へ戻る。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) { app.swipeUp() }
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) { app.swipeDown() }
    }

    private func tap(_ app: XCUIApplication, _ identifier: String) {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        scrollUntilHittable(app, target)
        target.tap()
    }

    private func label(_ app: XCUIApplication, _ identifier: String) -> String {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        return target.label
    }

    private func pick(_ app: XCUIApplication, button: String, sheet: String, row: String) {
        tap(app, button)
        wait(element(app, sheet), "\(sheet) が開かない")
        let target = element(app, row)
        wait(target, "\(row) が無い")
        target.tap()
        XCTAssertTrue(element(app, sheet).waitForNonExistence(timeout: Self.existenceTimeout), "選んだら\(sheet)が閉じる")
    }

    private func chooseStatus(_ app: XCUIApplication, prefix: String, value: String) {
        pick(app, button: "\(prefix)StatusButton", sheet: "judgeOptionSheet", row: "judgeOption-\(value)")
    }

    private func fillAttackerAndFirstCandidate(_ app: XCUIApplication) {
        pick(app, button: "judgeAttackerSpeciesButton", sheet: "speciesSearchSheet", row: "speciesSearchResult-\(Self.speciesOne)")
        pick(app, button: "judgeAttackerMoveButton", sheet: "moveSearchSheet", row: "moveSearchResult-\(Self.physicalMove)")
        pick(app, button: "judgeCandidate1SpeciesButton", sheet: "speciesSearchSheet", row: "speciesSearchResult-\(Self.speciesTwo)")
        pick(app, button: "judgeCandidate1MoveButton", sheet: "moveSearchSheet", row: "moveSearchResult-\(Self.specialMove)")
    }

    private func addAndFillSecondCandidate(_ app: XCUIApplication) {
        tap(app, "judgeAddCandidate")
        wait(element(app, "judgeCandidate2SpeciesButton"), "候補2が増えない")
        pick(app, button: "judgeCandidate2SpeciesButton", sheet: "speciesSearchSheet", row: "speciesSearchResult-\(Self.speciesThree)")
        pick(app, button: "judgeCandidate2MoveButton", sheet: "moveSearchSheet", row: "moveSearchResult-\(Self.physicalMove)")
    }

    private func submit(_ app: XCUIApplication) {
        tap(app, "judgeSubmit")
    }

    private func submitTwoCandidates(_ app: XCUIApplication) {
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-1"), "結果が出ない")
    }

    // MARK: - 状態異常の選択

    /// 既定は「なし」。選択シートには7値(なし を含む)が並び、「未選択に戻す」行は無い。選ぶとシートが閉じてボタンに名前が出る。
    func testStatusDefaultsToNoneAndCanBeChosenFromTheOptionSheet() {
        let app = launchJudgeScreen()
        let button = element(app, "judgeAttackerStatusButton")
        wait(button, "judgeAttackerStatusButton が無い")
        scrollUntilHittable(app, button)
        XCTAssertTrue(button.label.contains("なし"), "既定は なし: \(button.label)")
        button.tap()
        wait(element(app, "judgeOptionSheet"), "選択シートが開かない")
        for value in ["none", "burn", "paralysis", "poison", "badly_poison", "sleep", "freeze"] {
            wait(element(app, "judgeOption-\(value)"), "judgeOption-\(value) が無い")
        }
        XCTAssertFalse(element(app, "judgeOptionNone").exists, "なし が実の選択肢なので、未選択に戻す行は出さない")
        element(app, "judgeOption-badly_poison").tap()
        XCTAssertTrue(element(app, "judgeOptionSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選ぶと閉じる")
        XCTAssertTrue(button.label.contains("もうどく"), "選んだ状態異常に変わる: \(button.label)")
        tap(app, "judgeAttackerStatusButton")
        wait(element(app, "judgeOption-none"), "judgeOption-none が無い")
        element(app, "judgeOption-none").tap()
        XCTAssertTrue(button.label.contains("なし"), "なし に戻せる: \(button.label)")
    }

    /// 自分と候補の選択は独立(自分をまひにしても候補は なし のまま)。
    func testStatusIsChosenPerSideAndPerCandidate() {
        let app = launchJudgeScreen()
        chooseStatus(app, prefix: "judgeAttacker", value: "burn")
        tap(app, "judgeAddCandidate")
        wait(element(app, "judgeCandidate2StatusButton"), "候補2の状態異常のボタンが無い")
        chooseStatus(app, prefix: "judgeCandidate2", value: "sleep")
        XCTAssertTrue(label(app, "judgeAttackerStatusButton").contains("やけど"))
        XCTAssertTrue(label(app, "judgeCandidate1StatusButton").contains("なし"), "他の候補に波及しない")
        XCTAssertTrue(label(app, "judgeCandidate2StatusButton").contains("ねむり"))
    }

    // MARK: - 素早さの反映/無視の表示

    /// 自分側と相手候補側を取り違えず、候補ごとの行に出る。空の欄は出さない。文言は Web と同じ。
    func testSpeedNotesAreShownPerSideAndPerRowAndEmptyOnesAreHidden() {
        let app = launchJudgeScreen()
        submitTwoCandidates(app)

        XCTAssertEqual(label(app, "judgeRowAttackerSpeedApplied-0"), "自分の素早さに反映: ランク補正・追い風")
        XCTAssertEqual(label(app, "judgeRowDefenderSpeedApplied-0"), "相手の素早さに反映: こだわりスカーフ")
        XCTAssertFalse(element(app, "judgeRowAttackerSpeedIgnored-0").exists, "候補1は自分の無視が空 → 出さない")
        XCTAssertFalse(element(app, "judgeRowDefenderSpeedIgnored-0").exists, "候補1は相手の無視が空 → 出さない")

        XCTAssertEqual(label(app, "judgeRowAttackerSpeedApplied-1"), "自分の素早さに反映: ランク補正・追い風", "自分側はどの行でも同じ")
        XCTAssertFalse(element(app, "judgeRowDefenderSpeedApplied-1").exists, "候補2は相手の反映が空 → 出さない")
        XCTAssertEqual(label(app, "judgeRowAttackerSpeedIgnored-1"), "自分の素早さに特性は反映していません")
        XCTAssertEqual(label(app, "judgeRowDefenderSpeedIgnored-1"), "相手の素早さに持ち物・天候は反映していません")
    }

    /// 既定のシナリオ(4欄とも空)では、反映・無視の文は1つも出ない(既存の結果の表示を変えない)。
    func testNoSpeedNotesInTheNormalScenario() {
        let app = launchJudgeScreen(scenario: nil)
        fillAttackerAndFirstCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-0"), "結果が出ない")
        for identifier in [
            "judgeRowAttackerSpeedApplied-0", "judgeRowDefenderSpeedApplied-0", "judgeRowAttackerSpeedIgnored-0",
            "judgeRowDefenderSpeedIgnored-0",
        ] {
            XCTAssertFalse(element(app, identifier).exists, "\(identifier) が出ている")
        }
        XCTAssertTrue(label(app, "judgeRowSpeed-0").contains("素早さ 150 対 100"), "他の表示は変わらない")
    }

    /// 反映・無視の文を足しても、素早さ・確定数など既存の行の表示は変わらない。
    func testSpeedNotesDoNotChangeTheOtherRowValues() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-0"), "結果が出ない")
        XCTAssertTrue(label(app, "judgeRowSpeed-0").contains("素早さ 150 対 100"))
        XCTAssertEqual(label(app, "judgeRowSpeedComparison-0"), "素早さで上回る")
        XCTAssertEqual(label(app, "judgeRowAttackerKo-0"), "自分の技で相手を確定1発")
        XCTAssertEqual(label(app, "judgeRowDefenderKo-0"), "相手の技で自分が倒せない")
    }

    // MARK: - 状態異常 → 要求 → 表示

    /// 候補1をまひにして送ると、候補1の行の「相手の素早さに反映」に まひ が足される。自分側・他の行には出ない。
    func testCandidateParalysisReachesThatRowsDefenderSideOnly() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        chooseStatus(app, prefix: "judgeCandidate1", value: "paralysis")
        submit(app)
        wait(element(app, "judgeRow-1"), "結果が出ない")
        XCTAssertEqual(label(app, "judgeRowDefenderSpeedApplied-0"), "相手の素早さに反映: こだわりスカーフ・まひ")
        XCTAssertEqual(label(app, "judgeRowAttackerSpeedApplied-0"), "自分の素早さに反映: ランク補正・追い風", "候補のまひは自分側に出ない")
        XCTAssertFalse(element(app, "judgeRowDefenderSpeedApplied-1").exists, "まひにしていない候補2の行には出ない")
        XCTAssertTrue(label(app, "judgeCandidate1StatusButton").contains("まひ"), "送信しても入力は残る")
    }

    /// 自分をまひにして送ると、全行の「自分の素早さに反映」に まひ が足される。相手側には出ない。
    func testAttackerParalysisReachesTheAttackerSideOfEveryRow() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        chooseStatus(app, prefix: "judgeAttacker", value: "paralysis")
        submit(app)
        wait(element(app, "judgeRow-1"), "結果が出ない")
        for index in 0...1 {
            XCTAssertEqual(label(app, "judgeRowAttackerSpeedApplied-\(index)"), "自分の素早さに反映: ランク補正・追い風・まひ")
        }
        XCTAssertEqual(label(app, "judgeRowDefenderSpeedApplied-0"), "相手の素早さに反映: こだわりスカーフ", "自分のまひは相手側に出ない")
    }

    /// まひ以外の状態異常はモックでは何も足さない(judge も素早さに反映するのはまひだけ)。選べて送れる。
    func testNonParalysisStatusIsSentButAddsNoSpeedNote() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        chooseStatus(app, prefix: "judgeAttacker", value: "burn")
        submit(app)
        wait(element(app, "judgeRow-0"), "結果が出ない")
        XCTAssertEqual(label(app, "judgeRowAttackerSpeedApplied-0"), "自分の素早さに反映: ランク補正・追い風")
        XCTAssertFalse(element(app, "judgeError").exists)
    }
}
