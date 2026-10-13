import XCTest

/// G-05 第 4 弾(ADR-0540。ADR-0501「G-05 調整の作り直し」章): 調整画面を、文で説明せず、アイコン・短い名前・増減ボタン・
/// 区切りボタン・「説明」ボタンで分からせる作りに直した結果を、画面の要素の有無とタップの大きさ(36pt 以上)・AX5 のはみ出しで確かめる。
/// 文面そのものは `G05AdjustTextTests` が固定する。既存の `AdjustScreenUITests`・`AdjustGoalsUITests` は挙動と識別子・読み上げ名を守る
/// (ここは新しい構造だけ)。新しい識別子は実装前は無いので失敗してよい(新しい識別子は ADR の表)。モックは `AdjustScreenUITests` と同じ。
@MainActor
final class AdjustG05UITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let resultTimeout: TimeInterval = 10
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let ownSpeciesKey = "9001-000"
    private static let opponentSpeciesKey = "9002-000"
    private static let neutralNatureName = "テストせいかく無補正"
    private static let physicalMoveName = "テストわざぶつりA"
    private static let physicalMoveId = "test-move-physical-a"
    private static let stats = ["hp", "atk", "def", "spa", "spd", "spe"]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchAdjust(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        app.openScreen("openAdjustScreen")
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 見えるまでスクロールする。要素が画面より上にあるときは下へ戻す(画面の上から下の順に並べ替えなくてよいように)。
    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 20 {
            let screen = element(app, "adjustScreen")
            let container = screen.exists ? screen : app
            let window = app.windows.firstMatch.frame
            if target.exists && target.frame.midY < window.midY { container.swipeDown() } else { container.swipeUp() }
            attempts += 1
        }
        RunLoop.current.run(until: Date().addingTimeInterval(0.5))
    }

    private func tap(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い", file: file, line: line)
        scrollTo(target, in: app)
        target.tap()
    }

    private func pickSpecies(_ app: XCUIApplication, button: String, key: String) {
        tap(app, button)
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "speciesSearchResult-\(key)")
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
    }

    private func selectOwn(_ app: XCUIApplication) {
        pickSpecies(app, button: "adjustOwnSpeciesButton", key: Self.ownSpeciesKey)
        tap(app, "adjustOwnNaturePicker")
        let nature = app.buttons[Self.neutralNatureName].firstMatch
        XCTAssertTrue(nature.waitForExistence(timeout: Self.existenceTimeout))
        nature.tap()
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
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(identifier) が無い", file: file, line: line)
            scrollTo(target, in: app)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(identifier) が左にはみ出す", file: file, line: line)
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(identifier) が右にはみ出す", file: file, line: line)
        }
    }

    private func text(of target: XCUIElement) -> String { (target.value as? String) ?? "" }

    // MARK: - 状態の一言(回り続ける表示が無い)

    func testEmptyResultIsANoticeAndThereIsNoSpinner() {
        let app = launchAdjust()
        let empty = element(app, "adjustEmptyResult")
        XCTAssertTrue(empty.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(empty.label, "「調整する」を押すと結果が出ます", "空の状態の一言(語は不変)")
        XCTAssertEqual(app.activityIndicators.count, 0, "読み込み中も回り続ける表示を置かない(静止したアイコン + 一言)")
        assertTapTarget(app.buttons["adjustSubmitButton"], "調整する")
    }

    // MARK: - 固定する能力ポイント: − / + と数値欄、説明ボタン

    func testFixedSPFieldsHaveStepButtonsOfTapSize() {
        let app = launchAdjust()
        for stat in Self.stats {
            let down = element(app, "adjustFixedSPDown-\(stat)")
            let up = element(app, "adjustFixedSPUp-\(stat)")
            scrollTo(up, in: app)
            assertTapTarget(down, "\(stat) の −")
            assertTapTarget(up, "\(stat) の +")
            assertTapTarget(element(app, "adjustFixedSP-\(stat)"), "\(stat) の数値欄")
            XCTAssertTrue(down.label.hasSuffix("を減らす"), "読み上げ名は用語集の語: \(down.label)")
            XCTAssertTrue(up.label.hasSuffix("を増やす"), "読み上げ名は用語集の語: \(up.label)")
            XCTAssertEqual(down.staticTexts.count, 0, "アイコンだけ(見える文字を置かない)")
        }
        XCTAssertEqual(element(app, "adjustFixedSPUp-hp").label, "H の固定ポイントを増やす")
        XCTAssertEqual(element(app, "adjustFixedSPDown-hp").label, "H の固定ポイントを減らす")
    }

    func testFixedSPStepButtonsChangeValueAndStopAtTheEnds() {
        let app = launchAdjust()
        let field = element(app, "adjustFixedSP-hp")
        let up = element(app, "adjustFixedSPUp-hp")
        let down = element(app, "adjustFixedSPDown-hp")
        scrollTo(up, in: app)
        XCTAssertFalse(down.isEnabled, "0(空)で − は押せない")
        up.tap()
        XCTAssertEqual(text(of: field), "1")
        XCTAssertTrue(down.isEnabled)
        waitForLabel(element(app, "adjustFixedSPTotal"), "合計 1 / 66")
        down.tap()
        XCTAssertEqual(text(of: field), "0")
        field.tap()
        field.typeText("32")
        XCTAssertFalse(up.isEnabled, "32 で + は押せない")
        down.tap()
        XCTAssertEqual(text(of: field), "31")
    }

    /// 補足の文は最初は出ず、「説明」ボタンを押すと出る(欄の説明は常時の文にしない)。
    func testFixedSPHintIsBehindHelpButton() {
        let app = launchAdjust()
        let help = element(app, "adjustFixedSPHelpButton")
        scrollTo(help, in: app)
        assertTapTarget(help, "説明")
        XCTAssertFalse(element(app, "adjustFixedSPHint").exists, "最初は出ない")
        help.tap()
        let hint = element(app, "adjustFixedSPHint")
        XCTAssertTrue(hint.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(hint.label, "ここで決めた値より下には振りません。残りを調整に回します")
    }

    // MARK: - 調整の内容: 選択肢の見た目・上限の増減ボタン・最低素早さの説明

    func testModeChoicesAreChipsAndCeilingsAreSteppers() {
        let app = launchAdjust()
        tap(app, "adjustMode-bulk")
        XCTAssertTrue(element(app, "adjustMode-bulk").isSelected)
        assertTapTarget(element(app, "adjustMode-bulk"), "耐久に振る")
        for stat in ["hp", "def", "spd"] {
            let up = element(app, "adjustCeilingUp-\(stat)")
            scrollTo(up, in: app)
            assertTapTarget(up, "\(stat) の上限の +")
            assertTapTarget(element(app, "adjustCeilingDown-\(stat)"), "\(stat) の上限の −")
            XCTAssertEqual(text(of: element(app, "adjustCeiling-\(stat)")), "32", "上限の既定は 32")
        }
        XCTAssertFalse(element(app, "adjustCeilingUp-hp").isEnabled, "32 で + は押せない")
        element(app, "adjustCeilingDown-hp").tap()
        XCTAssertEqual(text(of: element(app, "adjustCeiling-hp")), "31")
        XCTAssertTrue(element(app, "adjustCeilingUp-hp").isEnabled)
        XCTAssertEqual(element(app, "adjustCeilingDown-def").label, "B の上限を減らす")
    }

    func testMinSpeedHintIsBehindHelpButton() {
        let app = launchAdjust()
        tap(app, "adjustMode-offense")
        let help = element(app, "adjustMinSpeedHelpButton")
        scrollTo(help, in: app)
        assertTapTarget(help, "説明")
        XCTAssertFalse(element(app, "adjustMinSpeedHint").exists, "最初は出ない")
        help.tap()
        XCTAssertTrue(element(app, "adjustMinSpeedHint").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 結果: 注は説明ボタンの奥

    func testIndexNoteIsBehindHelpButton() {
        let app = launchAdjust()
        selectOwn(app)
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustStatsLine").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertFalse(element(app, "adjustIndexNote").exists, "指数の注は最初は出ない")
        let help = element(app, "adjustIndexNoteHelpButton")
        scrollTo(help, in: app)
        assertTapTarget(help, "説明")
        help.tap()
        let note = element(app, "adjustIndexNote")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(note.label, "火力の目安には、タイプ一致だけを含めます(持ち物・特性・テラスタルは含めません)")
    }

    func testMinSpNotRequestedIsBehindHelpButton() {
        let app = launchAdjust()
        selectOwn(app)
        tap(app, "adjustMode-bulk")
        tap(app, "adjustSubmitButton")
        XCTAssertTrue(element(app, "adjustMaxIndexPlan").waitForExistence(timeout: Self.resultTimeout))
        XCTAssertFalse(element(app, "adjustMinSpNotRequested").exists, "最初は出ない")
        let help = element(app, "adjustMinSpNotRequestedHelpButton")
        scrollTo(help, in: app)
        assertTapTarget(help, "説明")
        help.tap()
        XCTAssertTrue(element(app, "adjustMinSpNotRequested").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 覚えるポケモン

    func testLearnersButtonsAreTapSizeAndCloseIsIconOnly() {
        let app = launchAdjust()
        pickSpecies(app, button: "adjustOwnSpeciesButton", key: Self.ownSpeciesKey)
        assertTapTarget(element(app, "adjustOwnLearnersButton"), "覚えるポケモン")
        tap(app, "adjustOwnMovePicker")
        let move = app.buttons[Self.physicalMoveName].firstMatch
        XCTAssertTrue(move.waitForExistence(timeout: Self.existenceTimeout))
        move.tap()
        tap(app, "adjustOwnLearnersButton")
        XCTAssertTrue(element(app, "adjustLearnersHeading").waitForExistence(timeout: Self.resultTimeout))
        let close = element(app, "adjustLearnersClose")
        scrollTo(close, in: app)
        assertTapTarget(close, "閉じる")
        XCTAssertEqual(close.label, "閉じる", "アイコンだけでも読み上げ名は共通の語")
        XCTAssertEqual(close.staticTexts.count, 0, "アイコンだけ(見える文字を置かない)")
        XCTAssertEqual(app.activityIndicators.count, 0, "読み込み中も回り続ける表示を置かない")
        close.tap()
        XCTAssertFalse(element(app, "adjustLearnersCard").exists)
    }

    // MARK: - 目標

    private func startGoals(_ app: XCUIApplication) {
        selectOwn(app)
        tap(app, "adjustMode-goals")
    }

    /// 空の一言(語は不変)・追加のタイル・種類の区切りボタン(アイコン + 短い名前。選択中は選択の特性)。
    func testGoalsEmptyNoticeAddTileAndKindSegments() {
        let app = launchAdjust()
        startGoals(app)
        XCTAssertEqual(element(app, "adjustNoGoals").label, "目標を追加してください")
        assertTapTarget(element(app, "adjustAddGoalButton"), "目標を追加")
        tap(app, "adjustAddGoalButton")
        for kind in ["outspeed", "survive", "ko"] {
            assertTapTarget(element(app, "adjustGoal-1-kind-\(kind)"), "\(kind) の区切りボタン")
        }
        XCTAssertTrue(element(app, "adjustGoal-1-kind-outspeed").isSelected)
        element(app, "adjustGoal-1-kind-ko").tap()
        XCTAssertTrue(element(app, "adjustGoal-1-kind-ko").isSelected)
        XCTAssertFalse(element(app, "adjustGoal-1-kind-outspeed").isSelected)
        XCTAssertFalse(element(app, "adjustNoGoals").exists, "目標があれば空の一言は出ない")
    }

    /// 先に使う技の補足は説明ボタンの奥(常時の文を置かない)。
    func testBoostMoveHintIsBehindHelpButton() {
        let app = launchAdjust()
        startGoals(app)
        tap(app, "adjustAddGoalButton")
        let help = element(app, "adjustGoal-1-boostMoveHelpButton")
        scrollTo(help, in: app)
        assertTapTarget(help, "説明")
        XCTAssertFalse(element(app, "adjustGoal-1-boostMoveHint").exists, "最初は出ない")
        help.tap()
        let hint = element(app, "adjustGoal-1-boostMoveHint")
        XCTAssertTrue(hint.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(hint.label, "ニトロチャージのように自分の素早さが上がる技を選ぶと、上がったあとの素早さで比べます")
    }

    /// 「選んだ相手から」: これまでに選んだ相手を絵つきのタイルで並べ、押すとその目標の相手になる。
    func testRecentOpponentTilesSelectTheOpponent() {
        let app = launchAdjust()
        startGoals(app)
        tap(app, "adjustAddGoalButton")
        XCTAssertFalse(element(app, "adjustGoal-1-recent").exists, "まだ誰も選んでいないときは出さない")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustAddGoalButton")
        let group = element(app, "adjustGoal-2-recent")
        scrollTo(group, in: app)
        XCTAssertTrue(group.waitForExistence(timeout: Self.existenceTimeout), "選んだ相手の列")
        XCTAssertEqual(group.label, "選んだ相手から")
        let tile = element(app, "adjustGoal-2-recent-\(Self.opponentSpeciesKey)")
        assertTapTarget(tile, "選んだ相手のタイル")
        tile.tap()
        XCTAssertEqual(
            element(app, "adjustGoal-2-opponentButton").value as? String,
            element(app, "adjustGoal-1-opponentButton").value as? String, "タイルを押すとその相手になる")
    }

    /// 発数の − / + は 36pt 以上のアイコンボタン(読み上げ名・識別子は従来のまま)。
    func testGoalHitsButtonsAreIconButtons() {
        let app = launchAdjust()
        startGoals(app)
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-1-kind-survive")
        let plus = element(app, "adjustGoal-1-hitsPlus")
        scrollTo(plus, in: app)
        assertTapTarget(plus, "発数の +")
        assertTapTarget(element(app, "adjustGoal-1-hitsMinus"), "発数の −")
        XCTAssertEqual(plus.label, "目標 1 の発数を増やす")
        XCTAssertEqual(element(app, "adjustGoal-1-hitsMinus").label, "目標 1 の発数を減らす")
        XCTAssertEqual(plus.staticTexts.count, 0, "アイコンだけ")
        assertTapTarget(element(app, "adjustGoal-1-remove"), "外す")
    }

    /// 技のシート: 閉じるはアイコンだけ(共通の語)、行は 36pt 以上(タイプバッジ + 技名 + 威力)。選ぶと閉じる(従来のまま)。
    func testGoalMoveSheetHasIconCloseAndTapSizeRows() {
        let app = launchAdjust()
        startGoals(app)
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-1-kind-ko")
        tap(app, "adjustGoal-1-ownMoveButton")
        XCTAssertTrue(element(app, "adjustGoalMoveSheet").waitForExistence(timeout: Self.existenceTimeout))
        let close = element(app, "adjustGoalMoveSheetClose")
        assertTapTarget(close, "閉じる")
        XCTAssertEqual(close.label, "閉じる")
        XCTAssertEqual(close.staticTexts.count, 0, "アイコンだけ")
        assertTapTarget(element(app, "adjustGoalMoveOption-none"), "未選択の行")
        assertTapTarget(element(app, "adjustGoalMoveOption-\(Self.physicalMoveId)"), "技の行")
        close.tap()
        XCTAssertFalse(element(app, "adjustGoalMoveSheet").waitForExistence(timeout: 1))
    }

    // MARK: - AX5: 新しい部品が横にはみ出さず、タップが 36pt 以上

    func testNewPartsFitAtAX5() {
        let app = launchAdjust(contentSizeCategory: Self.ax5ContentSizeCategory)
        var identifiers = ["adjustFixedSPHelpButton", "adjustOwnLearnersButton"]
        for stat in Self.stats {
            identifiers += ["adjustFixedSP-\(stat)", "adjustFixedSPDown-\(stat)", "adjustFixedSPUp-\(stat)"]
        }
        assertInsideWindow(app, identifiers)
        for stat in Self.stats {
            assertTapTarget(element(app, "adjustFixedSPUp-\(stat)"), "AX5 の + \(stat)")
            assertTapTarget(element(app, "adjustFixedSPDown-\(stat)"), "AX5 の − \(stat)")
        }
        tap(app, "adjustMode-bulk")
        assertInsideWindow(app, ["adjustCeiling-hp", "adjustCeilingDown-hp", "adjustCeilingUp-hp", "adjustCeilingUp-spd"])
        tap(app, "adjustMode-offense")
        assertInsideWindow(app, ["adjustMinSpeed", "adjustMinSpeedHelpButton"])
    }

    func testGoalPartsFitAtAX5() {
        let app = launchAdjust(contentSizeCategory: Self.ax5ContentSizeCategory)
        startGoals(app)
        tap(app, "adjustAddGoalButton")
        pickSpecies(app, button: "adjustGoal-1-opponentButton", key: Self.opponentSpeciesKey)
        tap(app, "adjustAddGoalButton")
        tap(app, "adjustGoal-2-kind-survive")
        assertInsideWindow(
            app,
            [
                "adjustAddGoalButton", "adjustGoal-1-kind-outspeed", "adjustGoal-1-kind-survive", "adjustGoal-1-kind-ko",
                "adjustGoal-1-boostMoveHelpButton", "adjustGoal-2-recent", "adjustGoal-2-recent-\(Self.opponentSpeciesKey)",
                "adjustGoal-2-hitsMinus", "adjustGoal-2-hitsPlus", "adjustGoal-2-hits",
            ])
        assertTapTarget(element(app, "adjustGoal-1-kind-ko"), "AX5 の区切りボタン")
        assertTapTarget(element(app, "adjustGoal-2-hitsPlus"), "AX5 の発数の +")
    }

    private func waitForLabel(_ target: XCUIElement, _ text: String, file: StaticString = #filePath, line: UInt = #line) {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label == %@", text), object: target)
        XCTAssertEqual(
            XCTWaiter().wait(for: [expectation], timeout: Self.existenceTimeout), .completed,
            "「\(text)」にならない(実際: \(target.label))", file: file, line: line)
    }
}
