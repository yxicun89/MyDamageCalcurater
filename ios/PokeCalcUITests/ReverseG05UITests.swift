import XCTest

/// G-05 第 3 弾(ADR-0539。ADR-0501「G-05 逆算の作り直し」章): 逆算画面を、文で説明せず、見出し・アイコン・バッジ・数値で
/// 分からせる作りに直した結果を、画面の要素の有無とタップの大きさ(36pt 以上)・AX5 のはみ出しで確かめる。文面そのものは
/// `G05ReverseTextTests` が固定する。既存の `ReverseScreenUITests` は挙動と識別子・読み上げ名を守る(ここは新しい構造だけ)。
/// 新しい識別子は実装前は無いので失敗してよい(新しい識別子は ADR の表)。モックの切り替えは `ReverseScreenUITests` と同じ。
/// モックの候補は `neutral@-`・`plus@-`(性格クラス × 持ち物なし)の2件(観測を入れると出る)。
@MainActor
final class ReverseG05UITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let candidateID = "neutral@-"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchReverse(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        app.openScreen("openReverseScreen")
        XCTAssertTrue(element(app, "reverseScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func assertTapTarget(_ target: XCUIElement, _ name: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(name) が無い", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(name) の高さ", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(name) の幅", file: file, line: line)
    }

    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 12 {
            app.swipeUp()
            attempts += 1
        }
        RunLoop.current.run(until: Date().addingTimeInterval(0.8))
    }

    /// 観測欄に打つ(前の欄のキーボードは「完了」で閉じてから。`ReverseScreenUITests.typeObservation` と同じ)。
    private func typeObservation(_ app: XCUIApplication, _ identifier: String, _ text: String) {
        let field = element(app, identifier)
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        let done = app.buttons["完了"]
        if app.keyboards.firstMatch.exists, done.exists {
            done.tap()
            XCTAssertTrue(app.keyboards.firstMatch.waitForNonExistence(timeout: Self.existenceTimeout))
        }
        field.tap()
        field.typeText(text)
    }

    private func showCandidates(_ app: XCUIApplication) {
        typeObservation(app, "reverseObservationField-0", "12")
        XCTAssertTrue(element(app, "reverseCandidateRow-\(Self.candidateID)").waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 自分・相手: 見出しの下にカード

    /// 見出し(「自分」「相手」)がカードの上にある。カードの識別子と、AX5 での縦積みは既存のまま。
    func testRegionHeadingsSitAboveTheirCards() {
        let app = launchReverse()
        for (heading, card, label) in [("reverseMyHeading", "reverseMyCard", "自分"), ("reverseOpponentHeading", "reverseOpponentCard", "相手")] {
            let headingElement = element(app, heading)
            XCTAssertTrue(headingElement.waitForExistence(timeout: Self.existenceTimeout), heading)
            XCTAssertEqual(headingElement.label, label)
            let cardElement = element(app, card)
            XCTAssertTrue(cardElement.exists, card)
            XCTAssertLessThanOrEqual(headingElement.frame.maxY, cardElement.frame.minY + Self.overflowTolerance, "\(heading) はカードの上")
        }
    }

    /// カードの中は 名前・タイプバッジ・持ち物の欄だけ。持ち物の欄(`reverseMyItemPicker`)は従来の名前のまま 36pt 以上。
    func testCardPickersAreTappable() {
        let app = launchReverse()
        assertTapTarget(element(app, "reverseMySpeciesPicker"), "自分のポケモン")
        assertTapTarget(element(app, "reverseOpponentSpeciesPicker"), "相手のポケモン")
        assertTapTarget(element(app, "reverseMyItemPicker"), "自分の持ち物")
        assertTapTarget(element(app, "reverseMovePicker"), "技")
    }

    // MARK: - 区切りボタン・チップ

    func testSideSwitchAndPresetsAreTappableAndKeepSelection() {
        let app = launchReverse()
        let defender = element(app, "reverseSide-defender")
        assertTapTarget(defender, "与えたダメージ")
        assertTapTarget(element(app, "reverseSide-attacker"), "受けたダメージ")
        XCTAssertTrue(defender.isSelected, "既定は与えたダメージ")
        for id in ["reverseAttackerPreset-none", "reverseAttackerPreset-aFull", "reverseAttackerPreset-aMax"] {
            assertTapTarget(app.buttons[id], id)
        }
    }

    // MARK: - 観測欄: 見える名前・単位・単位の 1 行・アイコンのボタン

    func testObservationRowShowsLabelUnitAndOneLineHint() {
        let app = launchReverse()
        XCTAssertEqual(element(app, "reverseObservationLabel-0").label, "ダメージ1", "見えるラベルは短く残す")
        XCTAssertEqual(element(app, "reverseObservationUnit-0").label, "%", "与えたダメージは %")
        XCTAssertEqual(element(app, "reverseObservationHint").label, "相手の HP が減った割合(%)", "単位の 1 行(1 カード 1 行まで)")
        XCTAssertEqual(element(app, "reverseObservationField-0").label, "ダメージ1(%)", "欄の読み上げ名は不変")

        element(app, "reverseSide-attacker").tap()
        XCTAssertTrue(element(app, "reverseObservationUnit-0").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "reverseObservationUnit-0").label, "HP", "旧: 「ダメージ」")
        XCTAssertEqual(element(app, "reverseObservationHint").label, "自分の HP が減った実数値(HP)")
        XCTAssertEqual(element(app, "reverseObservationField-0").label, "ダメージ1(HP)")
    }

    /// 削除はアイコンだけ(`staticTexts` が 0)で読み上げ名は不変、追加はアイコン + 名前。どちらも 36pt 以上。
    func testObservationButtonsAreIconRemoveAndLabeledAdd() {
        let app = launchReverse()
        let remove = app.buttons["reverseObservationRemove-0"]
        assertTapTarget(remove, "ダメージを削除")
        XCTAssertEqual(remove.label, "このダメージを削除")
        XCTAssertEqual(remove.staticTexts.count, 0, "アイコンだけ")
        let add = app.buttons["reverseAddObservationButton"]
        scrollTo(add, in: app)
        assertTapTarget(add, "ダメージを追加")
        XCTAssertTrue(add.label.contains("ダメージを追加"), add.label)
    }

    /// 入力エラーは アイコン + 文(従来の識別子)。上限の注記も同じ作り(識別子は不変)。
    func testObservationErrorStaysAndIsReadable() {
        let app = launchReverse()
        typeObservation(app, "reverseObservationField-0", "101")
        let error = element(app, "reverseObservationError-0")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(error.label.isEmpty)
        XCTAssertFalse(element(app, "reverseCandidateRow-\(Self.candidateID)").exists, "不正な行があるうちは候補を出さない(不変)")
    }

    // MARK: - 候補のカード

    func testCandidateCardShowsNatureBadgeLargePercentAndSmallDetails() {
        let app = launchReverse()
        showCandidates(app)
        let row = element(app, "reverseCandidateRow-\(Self.candidateID)")
        // 1 行目は型の名前だけ(旧: 「補正なし / 持ち物なし」)。持ち物は下に小さく。
        XCTAssertEqual(element(app, "reverseCandidateNature-\(Self.candidateID)").label, "補正なし")
        XCTAssertTrue(element(app, "reverseCandidateItem-\(Self.candidateID)").exists)
        XCTAssertFalse(element(app, "reverseCandidateItem-\(Self.candidateID)").label.contains("補正なし"))
        // 右に予測%の大きい数字(短いラベル「予測」つき)。範囲は SP の範囲として従来の識別子で残る。
        let percent = element(app, "reverseCandidatePercent-\(Self.candidateID)")
        XCTAssertTrue(percent.exists)
        XCTAssertTrue(percent.label.contains("%"), percent.label)
        XCTAssertTrue(row.staticTexts["予測"].exists, "短いラベル「予測」")
        XCTAssertTrue(element(app, "reverseCandidateRange-\(Self.candidateID)").exists)
        // 一致した候補には「ほぼ合う候補」「参考」を出さない(そのバッジは一致しない候補だけ)。
        XCTAssertFalse(row.staticTexts["ほぼ合う候補"].exists)
        XCTAssertFalse(row.staticTexts["参考"].exists)
        XCTAssertFalse(element(app, "reverseNoExactNotice").exists, "一致する候補があるので案内は出さない")
    }

    func testResultSummaryIsShortAndPremiseIsOneLine() {
        let app = launchReverse()
        showCandidates(app)
        let count = element(app, "reverseExactCount")
        XCTAssertTrue(count.exists)
        XCTAssertTrue(count.label.hasPrefix("一致 "), count.label)
        XCTAssertFalse(count.label.contains("入力したダメージと一致"), "旧: 長い文")
        XCTAssertEqual(element(app, "reversePremise").label, "H32 を仮定した結果です")
    }

    // MARK: - 回り続ける表示を置かない

    func testNoSpinnerOnReverseScreen() {
        let app = launchReverse()
        showCandidates(app)
        XCTAssertEqual(app.activityIndicators.count, 0, "回り続ける読み込み表示を置かない(静止したアイコン + 一言)")
    }

    // MARK: - 種族・技の検索シート(補足は「説明」ボタンの奥・閉じるはアイコン)

    func testSpeciesSheetHintBehindHelpAndCloseIsIcon() {
        let app = launchReverse()
        element(app, "reverseOpponentSpeciesPicker").tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))

        let close = app.buttons["speciesSearchCancelButton"]
        assertTapTarget(close, "閉じる")
        XCTAssertEqual(close.label, "閉じる")
        XCTAssertEqual(close.staticTexts.count, 0, "アイコンだけ")

        let help = app.buttons["speciesSearchHelp"]
        assertTapTarget(help, "説明ボタン")
        XCTAssertTrue(help.label.contains("説明"), help.label)
        XCTAssertFalse(element(app, "speciesSearchHint").exists, "検索欄の補足の文を出し続けない")
        XCTAssertFalse(element(app, "speciesSearchNote").exists)
        XCTAssertEqual(app.activityIndicators.count, 0, "検索中も回り続ける表示を置かない")

        help.tap()
        let note = element(app, "speciesSearchNote")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(note.label.contains("名前の先頭"), note.label)
    }

    /// 状態(一致する候補がない)は従来どおり出る。閉じるで戻る。
    func testSpeciesSheetNoMatchStaysAndCloseWorks() {
        let app = launchReverse()
        element(app, "reverseOpponentSpeciesPicker").tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText("zzzz")
        let hint = element(app, "speciesSearchHint")
        XCTAssertTrue(hint.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(hint.label, "一致する候補がありません")
        app.buttons["speciesSearchCancelButton"].tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForNonExistence(timeout: Self.existenceTimeout))
    }

    func testMoveSheetHintBehindHelpAndCloseIsIcon() {
        let app = launchReverse()
        let picker = element(app, "reverseMovePicker")
        scrollTo(picker, in: app)
        picker.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let close = app.buttons["moveSearchCancelButton"]
        assertTapTarget(close, "閉じる")
        XCTAssertEqual(close.label, "閉じる")
        XCTAssertEqual(close.staticTexts.count, 0, "アイコンだけ")
        assertTapTarget(app.buttons["moveSearchHelp"], "説明ボタン")
        XCTAssertFalse(element(app, "moveSearchHint").exists)
        close.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForNonExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - AX5

    /// 新しい部品(見出し・単位・単位の 1 行・アイコンのボタン・候補のカード)を含め、AX5 で横にはみ出さず、タップが 36pt 以上。
    func testNewPartsFitAtAX5() {
        let app = launchReverse(contentSizeCategory: Self.ax5ContentSizeCategory)
        let width = app.windows.firstMatch.frame.width
        showCandidates(app)
        let identifiers = [
            "reverseMyHeading", "reverseOpponentHeading", "reverseObservationLabel-0", "reverseObservationUnit-0",
            "reverseObservationHint", "reverseObservationRemove-0", "reverseAddObservationButton",
            "reverseCandidateRow-\(Self.candidateID)", "reverseCandidateNature-\(Self.candidateID)",
            "reverseCandidatePercent-\(Self.candidateID)", "reverseCandidateRange-\(Self.candidateID)",
        ]
        for identifier in identifiers {
            let target = element(app, identifier)
            scrollTo(target, in: app)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), identifier)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(identifier) が左にはみ出す")
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(identifier) が右にはみ出す")
        }
        assertTapTarget(app.buttons["reverseObservationRemove-0"], "AX5 の削除")
        assertTapTarget(app.buttons["reverseAddObservationButton"], "AX5 の追加")
    }

    /// AX5 の「ダメージの側」は縦積みのまま(各ボタンが全文を見せ、36pt 以上)。
    func testSideSwitchStaysReadableAtAX5() {
        let app = launchReverse(contentSizeCategory: Self.ax5ContentSizeCategory)
        let defender = element(app, "reverseSide-defender")
        let attacker = element(app, "reverseSide-attacker")
        assertTapTarget(defender, "与えたダメージ(AX5)")
        assertTapTarget(attacker, "受けたダメージ(AX5)")
        XCTAssertGreaterThanOrEqual(attacker.frame.minY, defender.frame.maxY - Self.overflowTolerance, "AX5 では縦に積む")
    }
}
