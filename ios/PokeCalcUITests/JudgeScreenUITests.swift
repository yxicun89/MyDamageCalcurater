import XCTest

/// P6-25: 判定画面(ADR-0504。受け入れ条件は ADR-0501「P6-25 の受け入れ条件」)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ。ADR-0500 §5・§7)。判定のモックの挙動は環境変数
/// `POKECALC_MOCK_JUDGE`(`error` / `candidate-error` / `marks`)で切り替える。
///
/// 数値は、モックの固定の事実(ADR-0504 §8。`MockJudgeServiceTests` が単体で固定している)に頼る箇所だけで検査する:
/// 自分の素早さ 150・候補 i(0 始まり)の素早さ 100 + 25 × i・優先度は i = 1 だけ 1・自分の技の確定数 hits = i + 1(i 偶数は確定)。
/// それ以外は「導線・要素の有無・並び・選択状態・失敗が他に波及しないこと」を見る(ADR-0501「XCUITest で確かめること」と同じ粒度)。
///
/// 識別子(ADR-0504 §10。implementer はこの名前で付ける)。`<p>` は対象の接頭辞: 自分 = `judgeAttacker`、相手候補 n(1 始まり)= `judgeCandidate<n>`。
///   ルート: openJudgeScreen / 画面: judgeScreen / マスタの失敗: judgeMasterError
///   各体: <p>SpeciesButton(種族のシート `speciesSearchSheet`・`speciesSearchResult-<key>` を開く。既存の部品)/ <p>NatureButton・<p>AbilityButton・<p>ItemButton
///         (選択シート `judgeOptionSheet` を開く。行 `judgeOption-<id>`・未選択に戻す `judgeOptionNone`)/ <p>MoveButton(技のシート `moveSearchSheet`・`moveSearchResult-<id>`)/
///         <p>SPValue-<stat>・<p>SPIncrement-<stat>・<p>SPDecrement-<stat>(stat = hp|atk|def|spa|spd|spe)・<p>SPTotal /
///         <p>RankValue-<stat>・<p>RankIncrement-<stat>・<p>RankDecrement-<stat>(stat = atk|def|spa|spd|spe)/
///         <p>TeamSourceButton・<p>TeamEmptyMessage(既存の `TeamSourceMenuRow`。identifierPrefix = `<p>Team`)
///   候補: judgeAddCandidate / judgeRemoveCandidate-<n> / judgeCandidateLimitNotice
///   場の効果: judgeTrickRoom / judgeAttackerTailwind / judgeDefenderTailwind(isSelected で状態を出す)/ judgeDefenderTailwindNotice
///   送信: judgeSubmit / judgeValidationError / judgeLoading / judgeEmptyResult
///   結果: judgeResult / judgeRow-<i>(i = defenderIndex。0 始まり)/ judgeRowName-<i> / judgeRowSpeed-<i> / judgeRowPriority-<i> /
///         judgeRowSpeedComparison-<i> / judgeRowTurnOrder-<i> / judgeRowAttackerKo-<i> / judgeRowDefenderKo-<i> /
///         judgeRowAttackerKoUnreliable-<i> / judgeRowDefenderKoUnreliable-<i> / judgeRowAttackerNote-<i> / judgeRowDefenderNote-<i> /
///         judgeAttackerUnsupportedSummary / judgeDefenderUnsupportedSummary
///   失敗: judgeError / judgeErrorCandidate
///
/// P6-25 の spec 時点では View が未実装のため、このテストは失敗してよい。
@MainActor
final class JudgeScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 14
    private static let speciesOne = (key: "9001-000", name: "テストモンいち")
    private static let speciesTwo = (key: "9002-000", name: "テストモンに")
    private static let speciesThree = (key: "9003-000", name: "テストモンさん")
    private static let physicalMove = "test-move-physical-a"
    private static let specialMove = "test-move-special-b"
    private static let atkUpNature = (id: "test-nature-atk-up", name: "テストせいかく攻撃上昇")
    private static let neutralNatureName = "テストせいかく無補正"
    private static let memberNickname = "テストこたいP6-25"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動・補助

    private func launch(scenario: String? = nil, openAtLaunch: Bool = false) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        // 判定の入口は既定で非表示(F-07)。画面のテストは環境変数で出す。
        app.launchEnvironment["POKECALC_SHOW_JUDGE"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_JUDGE"] = scenario }
        if openAtLaunch { app.launchEnvironment["POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH"] = "1" }
        app.launch()
        return app
    }

    private func launchJudgeScreen(scenario: String? = nil) -> XCUIApplication {
        let app = launch(scenario: scenario)
        openJudgeScreen(app)
        return app
    }

    private func openJudgeScreen(_ app: XCUIApplication) {
        let open = element(app, "openJudgeScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openJudgeScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "judgeScreen").waitForExistence(timeout: Self.existenceTimeout), "judgeScreen が開かない")
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func wait(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
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

    private func label(_ app: XCUIApplication, _ identifier: String) -> String {
        let target = element(app, identifier)
        wait(target, "\(identifier) が無い")
        return target.label
    }

    private func pickSpecies(_ app: XCUIApplication, prefix: String, key: String) {
        tap(app, "\(prefix)SpeciesButton")
        wait(element(app, "speciesSearchSheet"), "種族のシートが開かない")
        let row = element(app, "speciesSearchResult-\(key)")
        wait(row, "speciesSearchResult-\(key) が無い")
        row.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選んだらシートが閉じる")
    }

    private func pickMove(_ app: XCUIApplication, prefix: String, id: String) {
        tap(app, "\(prefix)MoveButton")
        wait(element(app, "moveSearchSheet"), "技のシートが開かない")
        let row = element(app, "moveSearchResult-\(id)")
        wait(row, "moveSearchResult-\(id) が無い")
        row.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選んだらシートが閉じる")
    }

    /// 自分と候補1を送れる状態にする(性格は画面が既定で入れる)。
    private func fillAttackerAndFirstCandidate(_ app: XCUIApplication) {
        pickSpecies(app, prefix: "judgeAttacker", key: Self.speciesOne.key)
        pickMove(app, prefix: "judgeAttacker", id: Self.physicalMove)
        pickSpecies(app, prefix: "judgeCandidate1", key: Self.speciesTwo.key)
        pickMove(app, prefix: "judgeCandidate1", id: Self.specialMove)
    }

    private func addAndFillSecondCandidate(_ app: XCUIApplication) {
        tap(app, "judgeAddCandidate")
        wait(element(app, "judgeCandidate2SpeciesButton"), "候補2が増えない")
        pickSpecies(app, prefix: "judgeCandidate2", key: Self.speciesThree.key)
        pickMove(app, prefix: "judgeCandidate2", id: Self.physicalMove)
    }

    private func submit(_ app: XCUIApplication) {
        tap(app, "judgeSubmit")
    }

    // MARK: - 導線

    func testOpenJudgeScreenFromRoot() {
        let app = launchJudgeScreen()
        for identifier in ["judgeAttackerSpeciesButton", "judgeCandidate1SpeciesButton", "judgeSubmit", "judgeEmptyResult"] {
            wait(element(app, identifier), "\(identifier) が無い")
        }
        XCTAssertFalse(element(app, "judgeResult").exists, "押すまで結果は出ない")
    }

    func testOpensAtLaunchWithTheEnvironmentVariable() {
        let app = launch(openAtLaunch: true)
        wait(element(app, "judgeScreen"), "POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH=1 で判定画面が開かない")
    }

    // MARK: - 入力

    /// 性格は画面が既定(補正なしの最初)を入れる。選択シートで変えられ、シートは選ぶと閉じる。
    func testNatureDefaultsToNeutralAndCanBeChangedFromTheOptionSheet() {
        let app = launchJudgeScreen()
        let button = element(app, "judgeAttackerNatureButton")
        wait(button, "judgeAttackerNatureButton が無い")
        let neutralLabel = Self.neutralNatureName
        XCTAssertTrue(
            NSPredicate(format: "label CONTAINS %@", neutralLabel).evaluate(with: button), "既定の性格(補正なしの最初)が出ている: \(button.label)")
        button.tap()
        wait(element(app, "judgeOptionSheet"), "選択シートが開かない")
        let option = element(app, "judgeOption-\(Self.atkUpNature.id)")
        wait(option, "judgeOption-\(Self.atkUpNature.id) が無い")
        option.tap()
        XCTAssertTrue(element(app, "judgeOptionSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選ぶと閉じる")
        XCTAssertTrue(button.label.contains(Self.atkUpNature.name), "選んだ性格に変わる: \(button.label)")
    }

    /// 特性・持ち物は任意(未選択に戻せる)。
    func testItemCanBeChosenAndClearedFromTheOptionSheet() {
        let app = launchJudgeScreen()
        tap(app, "judgeAttackerItemButton")
        wait(element(app, "judgeOptionSheet"), "選択シートが開かない")
        let none = element(app, "judgeOptionNone")
        wait(none, "未選択に戻す行(judgeOptionNone)が無い")
        let berry = element(app, "judgeOption-test-item-berry")
        wait(berry, "持ち物の行が無い")
        berry.tap()
        XCTAssertTrue(element(app, "judgeAttackerItemButton").label.contains("テストどうぐきのみ"))
        tap(app, "judgeAttackerItemButton")
        wait(element(app, "judgeOptionNone"), "judgeOptionNone が無い")
        element(app, "judgeOptionNone").tap()
        XCTAssertFalse(element(app, "judgeAttackerItemButton").label.contains("テストどうぐきのみ"), "未選択に戻る")
    }

    func testSPStepperChangesTheValueAndTheTotalAndStopsAtZero() {
        let app = launchJudgeScreen()
        XCTAssertEqual(label(app, "judgeAttackerSPValue-spe"), "0")
        tap(app, "judgeAttackerSPIncrement-spe")
        tap(app, "judgeAttackerSPIncrement-spe")
        XCTAssertEqual(label(app, "judgeAttackerSPValue-spe"), "2")
        XCTAssertTrue(label(app, "judgeAttackerSPTotal").contains("2"), "合計も変わる")
        tap(app, "judgeAttackerSPDecrement-spe")
        tap(app, "judgeAttackerSPDecrement-spe")
        XCTAssertEqual(label(app, "judgeAttackerSPValue-spe"), "0")
        XCTAssertFalse(element(app, "judgeAttackerSPDecrement-spe").isEnabled, "0 より下には減らせない")
    }

    func testRankStepperIsLimitedToPlusMinusSix() {
        let app = launchJudgeScreen()
        for _ in 0..<6 { tap(app, "judgeAttackerRankIncrement-spe") }
        XCTAssertTrue(label(app, "judgeAttackerRankValue-spe").contains("6"), "+6 に届く: \(label(app, "judgeAttackerRankValue-spe"))")
        XCTAssertFalse(element(app, "judgeAttackerRankIncrement-spe").isEnabled, "+6 より上には増やせない")
    }

    // MARK: - 場の効果・候補の増減

    func testSpeedFieldTogglesShowTheirStateAndTheDefenderTailwindNoticeIsAlwaysThere() {
        let app = launchJudgeScreen()
        wait(element(app, "judgeDefenderTailwindNotice"), "相手側の追い風が全候補に共通である旨が出ている")
        for identifier in ["judgeTrickRoom", "judgeAttackerTailwind", "judgeDefenderTailwind"] {
            tap(app, identifier)
            expectSelected(element(app, identifier), true, "\(identifier) が選ばれる")
        }
        tap(app, "judgeTrickRoom")
        expectSelected(element(app, "judgeTrickRoom"), false, "もう一度で外れる")
    }

    func testAddingCandidatesStopsAtTheLimitAndRemovingKeepsAtLeastOne() {
        let app = launchJudgeScreen()
        XCTAssertFalse(element(app, "judgeRemoveCandidate-1").exists && element(app, "judgeRemoveCandidate-1").isEnabled, "候補が1件のときは削除できない")
        for n in 2...6 {
            tap(app, "judgeAddCandidate")
            wait(element(app, "judgeCandidate\(n)SpeciesButton"), "候補\(n)が増えない")
        }
        wait(element(app, "judgeCandidateLimitNotice"), "上限の説明が出る")
        XCTAssertFalse(element(app, "judgeAddCandidate").isEnabled, "上限では追加できない")
        XCTAssertFalse(element(app, "judgeCandidate7SpeciesButton").exists)

        tap(app, "judgeRemoveCandidate-6")
        XCTAssertTrue(element(app, "judgeCandidate6SpeciesButton").waitForNonExistence(timeout: Self.existenceTimeout), "候補6が消える")
        XCTAssertFalse(element(app, "judgeCandidateLimitNotice").exists)
        XCTAssertTrue(element(app, "judgeAddCandidate").isEnabled)
    }

    // MARK: - 送信と結果

    func testSubmittingWithoutInputShowsAValidationMessageAndNoResult() {
        let app = launchJudgeScreen()
        submit(app)
        let message = element(app, "judgeValidationError")
        wait(message, "検査のメッセージが出ない")
        XCTAssertTrue(message.label.contains("自分のポケモン"), "誰の入力かを示す: \(message.label)")
        XCTAssertFalse(element(app, "judgeResult").exists, "呼ばずに止める")
    }

    /// 自分 1 + 候補 1 で送ると、候補の行が出る。モックの固定の事実(素早さ 150 対 100・確定 1 発・倒せない)を見る。
    func testSubmitShowsTheCandidateRowWithValuesFromTheJudge() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        submit(app)
        wait(element(app, "judgeResult"), "結果が出ない")
        wait(element(app, "judgeRow-0"), "行 0 が無い")
        XCTAssertEqual(label(app, "judgeRowName-0"), Self.speciesTwo.name, "種族名は入力から引く")
        XCTAssertTrue(label(app, "judgeRowSpeed-0").contains("素早さ 150 対 100"), label(app, "judgeRowSpeed-0"))
        XCTAssertTrue(label(app, "judgeRowPriority-0").contains("優先度 0 対 0"))
        XCTAssertEqual(label(app, "judgeRowSpeedComparison-0"), "素早さで上回る")
        XCTAssertEqual(label(app, "judgeRowTurnOrder-0"), "自分が先に動く")
        XCTAssertEqual(label(app, "judgeRowAttackerKo-0"), "自分の技で相手を確定1発")
        XCTAssertEqual(label(app, "judgeRowDefenderKo-0"), "相手の技で自分が倒せない")
        XCTAssertFalse(element(app, "judgeEmptyResult").exists, "結果が出たら案内は消える")
    }

    /// 候補ごとに違う値が、その候補の行に出る(行の取り違えがない)。候補 2(i = 1)は優先度 1 で、素早さで抜いていても相手が先に動く。
    func testTwoCandidatesShowDifferentValuesInTheirOwnRows() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-1"), "行 1 が無い")
        XCTAssertEqual(label(app, "judgeRowName-0"), Self.speciesTwo.name)
        XCTAssertEqual(label(app, "judgeRowName-1"), Self.speciesThree.name)
        XCTAssertTrue(label(app, "judgeRowSpeed-1").contains("素早さ 150 対 125"), label(app, "judgeRowSpeed-1"))
        XCTAssertTrue(label(app, "judgeRowPriority-1").contains("優先度 0 対 1"))
        XCTAssertEqual(label(app, "judgeRowSpeedComparison-1"), "素早さで上回る")
        XCTAssertEqual(label(app, "judgeRowTurnOrder-1"), "相手が先に動く", "優先度が違えば素早さより優先度")
        XCTAssertEqual(label(app, "judgeRowAttackerKo-1"), "自分の技で相手を乱数2発(15.0%)")
        let top0 = element(app, "judgeRow-0").frame.minY
        let top1 = element(app, "judgeRow-1").frame.minY
        XCTAssertLessThan(top0, top1, "行は候補の順(defenderIndex の昇順)に並ぶ")
    }

    func testTrickRoomChangesTheNextResult() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        submit(app)
        wait(element(app, "judgeRowSpeedComparison-0"), "結果が出ない")
        XCTAssertEqual(label(app, "judgeRowSpeedComparison-0"), "素早さで上回る")
        tap(app, "judgeTrickRoom")
        submit(app)
        let predicate = NSPredicate(format: "label == %@", "素早さで下回る")
        let result = XCTWaiter().wait(
            for: [XCTNSPredicateExpectation(predicate: predicate, object: element(app, "judgeRowSpeedComparison-0"))],
            timeout: Self.existenceTimeout)
        XCTAssertEqual(result, .completed, "トリックルームでは遅い方が先に動く")
        XCTAssertTrue(label(app, "judgeRowSpeed-0").contains("素早さ 150 対 100"), "実数値そのものは変わらない")
    }

    // MARK: - 未対応の印(方向ごとに分ける。ADR-0708)

    /// `marks` シナリオ: 順方向は全行に共通 → 結果の上に1回。逆方向は 2 番目の候補の行だけ。添え書きは印のある行・方向だけ。
    func testUnsupportedMarksAreShownPerDirection() {
        let app = launchJudgeScreen(scenario: "marks")
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-1"), "結果が出ない")

        let forwardSummary = element(app, "judgeAttackerUnsupportedSummary")
        wait(forwardSummary, "順方向の共通の印は結果の上に1回")
        XCTAssertTrue(forwardSummary.label.contains("自分の技の確定数"), forwardSummary.label)
        XCTAssertFalse(element(app, "judgeDefenderUnsupportedSummary").exists, "逆方向は全行に共通ではないので結果の上には出ない")
        XCTAssertFalse(element(app, "judgeRowAttackerNote-0").exists, "共通の印は行に重ねない")

        let reverseNote = element(app, "judgeRowDefenderNote-1")
        wait(reverseNote, "逆方向の印は 2 番目の候補の行に出る")
        XCTAssertTrue(reverseNote.label.contains("相手の技の確定数"), reverseNote.label)
        XCTAssertFalse(element(app, "judgeRowDefenderNote-0").exists)

        for index in 0...1 {
            XCTAssertTrue(element(app, "judgeRowAttackerKoUnreliable-\(index)").exists, "順方向の確定数は、どの行でも確定として見せない旨が付く")
        }
        XCTAssertTrue(element(app, "judgeRowDefenderKoUnreliable-1").exists)
        XCTAssertFalse(element(app, "judgeRowDefenderKoUnreliable-0").exists, "順方向の印が逆方向の確定数を疑わしく見せない")
    }

    func testNoMarksMeansNoNoticesInTheNormalScenario() {
        let app = launchJudgeScreen()
        fillAttackerAndFirstCandidate(app)
        submit(app)
        wait(element(app, "judgeRow-0"), "結果が出ない")
        for identifier in [
            "judgeAttackerUnsupportedSummary", "judgeDefenderUnsupportedSummary", "judgeRowAttackerKoUnreliable-0",
            "judgeRowDefenderKoUnreliable-0", "judgeRowAttackerNote-0", "judgeRowDefenderNote-0",
        ] {
            XCTAssertFalse(element(app, identifier).exists, "\(identifier) が出ている")
        }
    }

    // MARK: - 失敗(絶対ルール5: 他の画面・入力に波及しない)

    func testJudgeFailureShowsAJapaneseMessageKeepsTheInputAndDoesNotBreakTheCalcScreen() {
        let app = launchJudgeScreen(scenario: "error")
        fillAttackerAndFirstCandidate(app)
        submit(app)
        let error = element(app, "judgeError")
        wait(error, "失敗が出ない")
        XCTAssertTrue(error.label.contains("判定に必要なサービスに接続できません"), error.label)
        XCTAssertFalse(error.label.contains("upstream"), "サーバーの英語は出さない")
        XCTAssertFalse(element(app, "judgeResult").exists)
        XCTAssertTrue(label(app, "judgeAttackerSpeciesButton").contains(Self.speciesOne.name), "入力は消えない(直して送り直せる)")

        app.navigationBars.buttons.element(boundBy: 0).tap()
        let openCalc = app.buttons["openCalcScreen"]
        wait(openCalc, "ルートに戻れない")
        openCalc.tap()
        wait(element(app, "calcScreen"), "判定が失敗しても計算画面は開く")
    }

    func testCandidateErrorNamesTheFailedCandidate() {
        let app = launchJudgeScreen(scenario: "candidate-error")
        fillAttackerAndFirstCandidate(app)
        addAndFillSecondCandidate(app)
        submit(app)
        wait(element(app, "judgeError"), "失敗が出ない")
        let hint = element(app, "judgeErrorCandidate")
        wait(hint, "どの候補かを示す")
        XCTAssertTrue(hint.label.contains("相手候補2"), hint.label)
    }

    // MARK: - 構築から呼び出す

    /// 構築ビルダーでメンバーを1体作る(`CalcScreenUITests.createTeamWithOneMember` と同じ操作列)。
    private func createTeamWithOneMember(_ app: XCUIApplication) {
        let openTeamList = app.buttons["openTeamListScreen"]
        wait(openTeamList, "openTeamListScreen が無い")
        openTeamList.tap()
        wait(element(app, "teamListScreen"), "構築一覧が開かない")
        element(app, "createTeamButton").tap()
        wait(element(app, "teamEditScreen"), "編集画面が開かない")
        let add = element(app, "slotSpeciesPicker-1")
        wait(add, "slotSpeciesPicker-1 が無い")
        add.tap()
        wait(element(app, "speciesSearchSheet"), "種族のシートが開かない")
        let option = app.buttons[Self.speciesTwo.name]
        wait(option, "種族が無い")
        option.tap()
        let nickname = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "memberNickname-")).firstMatch
        wait(nickname, "ニックネーム欄が無い")
        nickname.tap()
        nickname.typeText(Self.memberNickname)
        element(app, "saveTeamButton").tap()
        wait(element(app, "teamSavedNotice"), "保存できない")
        element(app, "backToListButton").tap()
        wait(element(app, "teamListScreen"), "保存して一覧に戻れない")
        app.navigationBars.buttons.element(boundBy: 0).tap()
    }

    /// 構築が無い間は入口が無効(案内つき)。構築を作ってから開くと、自分側にも候補側にもメンバーを呼び出せる。
    func testCallingTeamMembersIntoTheAttackerAndACandidate() {
        let app = launch()
        let open = app.buttons["openJudgeScreen"]
        wait(open, "openJudgeScreen が無い")
        open.tap()
        wait(element(app, "judgeScreen"), "judgeScreen が開かない")
        let emptyButton = element(app, "judgeAttackerTeamSourceButton")
        wait(emptyButton, "構築から呼び出す入口が無い")
        XCTAssertFalse(emptyButton.isEnabled, "構築が無いときは無効")
        wait(element(app, "judgeAttackerTeamEmptyMessage"), "案内が出る")

        app.navigationBars.buttons.element(boundBy: 0).tap()
        createTeamWithOneMember(app)
        openJudgeScreen(app)

        let attackerButton = element(app, "judgeAttackerTeamSourceButton")
        wait(attackerButton, "入口が無い")
        XCTAssertTrue(attackerButton.isEnabled, "構築ができたので選べる")
        attackerButton.tap()
        let attackerMember = app.buttons[Self.memberNickname]
        wait(attackerMember, "メンバーが一覧に出ない")
        attackerMember.tap()
        let attackerSpecies = element(app, "judgeAttackerSpeciesButton")
        let attackerPredicate = NSPredicate(format: "label CONTAINS %@", Self.speciesTwo.name)
        XCTAssertEqual(
            XCTWaiter().wait(for: [XCTNSPredicateExpectation(predicate: attackerPredicate, object: attackerSpecies)], timeout: Self.existenceTimeout),
            .completed, "呼び出した個体の種族に変わる")

        tap(app, "judgeCandidate1TeamSourceButton")
        let candidateMember = app.buttons[Self.memberNickname]
        wait(candidateMember, "候補側でもメンバーが一覧に出る")
        candidateMember.tap()
        let candidateSpecies = element(app, "judgeCandidate1SpeciesButton")
        let candidatePredicate = NSPredicate(format: "label CONTAINS %@", Self.speciesTwo.name)
        XCTAssertEqual(
            XCTWaiter().wait(for: [XCTNSPredicateExpectation(predicate: candidatePredicate, object: candidateSpecies)], timeout: Self.existenceTimeout),
            .completed, "候補側にも写る")
    }
}
