import XCTest

/// 計算画面の攻撃側「攻撃」「特攻」の SP 数値入力・性格補正と、技の選択肢のダメージ技への絞り込み
/// (ADR-0518。F-01 / I-ios-1・I-ios-5)。`POKECALC_USE_MOCK=1` で起動する。
///
/// モックはダメージを計算しないので、ここでは「ブロックの出方・強調」「入力と選択状態」「プリセットとの連動」
/// 「不正入力の案内と結果の行の出し入れ」「同じ向きの無効化」「変化技が選択肢に出ない」だけを見る。
/// 要求の中身(SP・性格の解決)は `CalcViewModelAttackerStatsTests` で固定している。
///
/// 画面の高さは機種で違う(iPhone 17e・iPhone 18 Pro・iPhone 17)。ブロックは技セレクタと「詳細」の間にあるので、
/// スクロールは「前方 → 届かなければ swipeDown で戻る」を持ち、タップは要素の上のほうを狙う
/// (過去に MegaItemLockUITests で、行の中央が画面の下端に当たって失敗した)。
///
/// 足場(spec-writer): 実装前なのでこのテストは失敗する(`TODO(implementer` の実装後に実行して確かめる)。
@MainActor
final class CalcAttackerStatsUITests: XCTestCase {
    /// モックの物理技の既定の先頭行(`CalcScreenUITests.defaultPhysicalRowIDs` の1つ目と同じ)。
    private static let firstRowID = "calcResultRow-none@-"
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrollAttempts = 6
    /// design.md のタップ範囲(pt)。
    private static let minimumTapSize: CGFloat = 36
    /// 既定の攻撃側(モックの1番目の種族)の learnset にある特殊技(`CalcScreenUITests` と同じ)。
    private static let specialMoveName = "テストわざとくしゅA"
    private static let specialMoveID = "test-move-special-a"
    /// モックの変化技(攻撃側の既定の種族の learnset にある)。選択肢に出ないことを確かめる。
    private static let statusMoveName = "テストわざへんかA"
    private static let statusMoveID = "test-move-status-a"
    private static let physicalMoveID = "test-move-physical-a"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 補助

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchCalcScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, Self.firstRowID).waitForExistence(timeout: Self.existenceTimeout),
                      "起動時の行が出る")
        return app
    }

    /// タップできる位置まで動かす。まず前方(swipeUp)、届かなければ swipeDown で戻る。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        let screen = element(app, "calcScreen")
        var attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            screen.swipeUp()
            attempts += 1
        }
        attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts * 2 {
            screen.swipeDown()
            attempts += 1
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)")
    }

    /// 要素の上のほうをタップする(行の中央が画面の下端に当たらないように)。
    private func tapUpperPart(_ target: XCUIElement) {
        target.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.25)).tap()
    }

    /// 数値キーボードを閉じる(キーボードのツールバーの「完了」。`AttackerStatLabels.keyboardDone`)。
    private func dismissKeyboard(_ app: XCUIApplication) {
        let done = app.buttons["calcKeyboardDone"]
        if done.waitForExistence(timeout: Self.existenceTimeout) {
            done.tap()
        }
    }

    /// SP 欄の中身を消して `text` を打つ(欄が空なら何も消さない)。打ち終えたらキーボードを閉じる。
    private func typeSP(_ app: XCUIApplication, stat: String, _ text: String) {
        let field = element(app, "attackerSPField-\(stat)")
        scrollUntilHittable(app, field)
        tapUpperPart(field)
        let current = field.value as? String ?? ""
        if !current.isEmpty {
            field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count))
        }
        if !text.isEmpty {
            field.typeText(text)
        }
        dismissKeyboard(app)
    }

    private func waitForPredicate(_ format: String, _ target: XCUIElement, _ value: Any? = nil) {
        let predicate = value.map { NSPredicate(format: format, argumentArray: [$0]) } ?? NSPredicate(format: format)
        expectation(for: predicate, evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    /// スクロール直後はタップが慣性で外れることがあるため、選択されなければ一度だけ押し直す(期待値は弱めない)。
    private func tapUntilSelected(_ app: XCUIApplication, _ target: XCUIElement) {
        scrollUntilHittable(app, target)
        tapUpperPart(target)
        if !target.waitForExistence(timeout: 1) || !NSPredicate(format: "isSelected == true").evaluate(with: target) {
            Thread.sleep(forTimeInterval: 1)
            if !target.isSelected {
                scrollUntilHittable(app, target)
                tapUpperPart(target)
            }
        }
        waitForPredicate("isSelected == true", target)
    }

    private func waitUntilGone(_ target: XCUIElement) {
        waitForPredicate("exists == false", target)
    }

    private func selectSpecialMove(_ app: XCUIApplication) {
        let movePicker = element(app, "movePicker")
        scrollUntilHittable(app, movePicker)
        tapUpperPart(movePicker)
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(Self.specialMoveName)
        let result = element(app, "moveSearchResult-\(Self.specialMoveID)")
        XCTAssertTrue(result.waitForExistence(timeout: Self.existenceTimeout))
        result.tap()
        XCTAssertFalse(element(app, "moveSearchSheet").exists, "選ぶとシートが閉じる")
    }

    // MARK: - ブロックの出方

    /// 「攻撃」「特攻」の2ブロックが常に出る。SP の既定は 0・補正は「補正なし」。物理技(既定)なので
    /// 「攻撃」の見出しだけに「(この技で使用)」が付く。どの入力もタップ範囲が 36pt 以上。
    func testBothBlocksAreShownWithDefaultsAndTheUsedSideIsMarkedInText() {
        let app = launchCalcScreen()
        scrollUntilHittable(app, element(app, "attackerSPField-atk"))

        XCTAssertTrue(element(app, "attackerStatBlock-atk").exists)
        XCTAssertTrue(element(app, "attackerStatBlock-spa").exists, "技が使わない側も常に出す")
        XCTAssertEqual(element(app, "attackerStatHeading-atk").label, "攻撃(この技で使用)")
        XCTAssertEqual(element(app, "attackerStatHeading-spa").label, "特攻", "使わない側には付けない")

        for stat in ["atk", "spa"] {
            XCTAssertEqual(element(app, "attackerSPField-\(stat)").value as? String, "0", "SP の既定は 0 (\(stat))")
            for choice in ["up", "neutral", "down"] {
                let button = element(app, "attackerNature-\(stat)-\(choice)")
                XCTAssertTrue(button.exists, "性格補正の選択肢が無い: \(stat) \(choice)")
                XCTAssertEqual(button.isSelected, choice == "neutral", "既定は補正なし: \(stat) \(choice)")
                XCTAssertGreaterThanOrEqual(button.frame.height, Self.minimumTapSize, "\(stat) \(choice) のタップ範囲")
            }
            XCTAssertGreaterThanOrEqual(
                element(app, "attackerSPField-\(stat)").frame.height, Self.minimumTapSize, "SP 欄のタップ範囲 (\(stat))")
            XCTAssertFalse(element(app, "attackerCustomMark-\(stat)").exists, "既定(無振り)はカスタムではない")
        }
        XCTAssertEqual(element(app, "attackerNature-atk-up").label, "上昇")
        XCTAssertEqual(element(app, "attackerNature-atk-neutral").label, "補正なし")
        XCTAssertEqual(element(app, "attackerNature-atk-down").label, "下降")
        XCTAssertEqual(element(app, "attackerSPField-atk").label, "攻撃のSP")
        XCTAssertEqual(element(app, "attackerSPField-spa").label, "特攻のSP")
    }

    /// 特殊技を選ぶと強調が「特攻」へ移る。両方のブロックの値は残る。
    func testSelectingSpecialMoveMovesTheEmphasisAndKeepsBothValues() {
        let app = launchCalcScreen()
        typeSP(app, stat: "atk", "20")
        XCTAssertEqual(element(app, "attackerSPField-atk").value as? String, "20")

        selectSpecialMove(app)
        scrollUntilHittable(app, element(app, "attackerSPField-spa"))
        waitForPredicate("label == %@", element(app, "attackerStatHeading-spa"), "特攻(この技で使用)")
        XCTAssertEqual(element(app, "attackerStatHeading-atk").label, "攻撃", "強調だけが移る")
        XCTAssertEqual(element(app, "attackerSPField-atk").value as? String, "20", "技を替えても攻撃の値は残る")
        XCTAssertEqual(element(app, "attackerSPField-spa").value as? String, "0")
        XCTAssertTrue(element(app, Self.firstRowID).waitForExistence(timeout: Self.existenceTimeout))
    }

    // MARK: - 入力とプリセットの連動

    /// SP を手で入れると、ピルの選択が外れて「カスタム」の印が出る。0 に戻すと印が消えてピルが選ばれた表示に戻る。
    /// 結果の行は出続ける(モックは数値を計算しない)。
    func testTypingSPMakesTheBlockCustomAndTypingZeroRestoresThePreset() {
        let app = launchCalcScreen()
        let nonePill = app.buttons["attackerPreset-none"]
        XCTAssertTrue(nonePill.isSelected, "既定は無振り")

        typeSP(app, stat: "atk", "20")
        XCTAssertTrue(element(app, "attackerCustomMark-atk").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "attackerCustomMark-atk").label, "カスタム")
        waitForPredicate("isSelected == false", nonePill)
        XCTAssertTrue(element(app, "attackerNature-atk-neutral").isSelected, "数値を変えても補正は変わらない")
        XCTAssertTrue(element(app, Self.firstRowID).waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "calcErrorMessage").exists)

        typeSP(app, stat: "atk", "0")
        waitUntilGone(element(app, "attackerCustomMark-atk"))
        waitForPredicate("isSelected == true", nonePill)
    }

    /// 「特化」のピルを押すと、使う側のブロックに 32・上昇が入る(もう一方は変わらない)。
    func testFullPresetFillsTheUsedBlockWithThirtyTwoAndUp() {
        let app = launchCalcScreen()
        let fullPill = app.buttons["attackerPreset-aFull"]
        XCTAssertTrue(fullPill.waitForExistence(timeout: Self.existenceTimeout))
        fullPill.tap()
        waitForPredicate("isSelected == true", fullPill)

        scrollUntilHittable(app, element(app, "attackerSPField-atk"))
        XCTAssertEqual(element(app, "attackerSPField-atk").value as? String, "32")
        XCTAssertTrue(element(app, "attackerNature-atk-up").isSelected)
        XCTAssertEqual(element(app, "attackerSPField-spa").value as? String, "0", "使わない側は変わらない")
        XCTAssertTrue(element(app, "attackerNature-spa-neutral").isSelected)
        XCTAssertFalse(element(app, "attackerCustomMark-atk").exists, "プリセットと一致するのでカスタムではない")
    }

    // MARK: - 性格補正と同じ向きの無効化

    /// 性格補正をタップすると選択が移り(数値は変わらない)、もう一方のブロックが上昇のときは
    /// こちらの「上昇」と「特化」のピルが選べなくなって理由が出る(黙って書き換えない)。
    func testNatureChoiceAndSameDirectionIsDisabledWithAReason() {
        let app = launchCalcScreen()
        let spaUp = element(app, "attackerNature-spa-up")
        scrollUntilHittable(app, spaUp)
        tapUpperPart(spaUp)
        waitForPredicate("isSelected == true", spaUp)
        XCTAssertTrue(element(app, "attackerNature-spa-neutral").exists)
        XCTAssertFalse(element(app, "attackerNature-spa-neutral").isSelected)
        XCTAssertEqual(element(app, "attackerSPField-spa").value as? String, "0", "補正を変えても数値は変わらない")

        XCTAssertFalse(element(app, "attackerNature-atk-up").isEnabled, "もう一方が上昇なら上昇は選べない")
        XCTAssertTrue(element(app, "attackerNature-atk-down").isEnabled)
        XCTAssertTrue(element(app, "attackerNature-atk-neutral").isEnabled)
        XCTAssertFalse(app.buttons["attackerPreset-aFull"].isEnabled, "特化(上昇)のピルも選べない")
        XCTAssertTrue(app.buttons["attackerPreset-aMax"].isEnabled)
        let reason = element(app, "attackerSameDirectionReason-atk")
        XCTAssertTrue(reason.exists, "理由の一文を添える")
        XCTAssertEqual(reason.label, "攻撃と特攻の両方を上昇、または両方を下降にすることはできません")
        XCTAssertTrue(element(app, "attackerNature-spa-up").isSelected, "もう一方の値を黙って書き換えない")

        // 下降は選べて、結果の行は出続ける。
        let atkDown = element(app, "attackerNature-atk-down")
        tapUntilSelected(app, atkDown)
        XCTAssertTrue(element(app, Self.firstRowID).waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "calcErrorMessage").exists)
    }

    // MARK: - 不正入力

    /// 33 のような範囲外は丸めず、入力の下に理由を出し、結果の行を出さない。直すと理由が消えて行が戻る。
    func testInvalidSPShowsAReasonHidesRowsAndRecoversWhenFixed() {
        let app = launchCalcScreen()
        typeSP(app, stat: "atk", "33")

        let error = element(app, "attackerSPError-atk")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(error.label, "攻撃のSPは0〜32の整数で入力してください")
        XCTAssertEqual(element(app, "attackerSPField-atk").value as? String, "33", "丸めずそのまま残す")
        waitUntilGone(element(app, Self.firstRowID))

        typeSP(app, stat: "atk", "32")
        waitUntilGone(error)
        XCTAssertTrue(element(app, Self.firstRowID).waitForExistence(timeout: Self.existenceTimeout), "直すと計算し直す")
    }

    /// 技が使わない側(物理技のときの特攻)の欄が不正でも計算しない(両方の SP を要求に載せるため)。
    func testInvalidSPOnTheUnusedSideAlsoHidesRows() {
        let app = launchCalcScreen()
        typeSP(app, stat: "spa", "40")
        XCTAssertTrue(element(app, "attackerSPError-spa").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "attackerSPError-spa").label, "特攻のSPは0〜32の整数で入力してください")
        waitUntilGone(element(app, Self.firstRowID))
    }

    // MARK: - 技の選択肢はダメージ技だけ

    /// 技の検索シートに変化技が出ない(モックの既定の攻撃側は変化技も覚えるが、選択肢には入らない)。
    /// 変化技の名前で検索しても出ない。ダメージ技は出る。
    func testMoveSheetListsOnlyDamagingMoves() {
        let app = launchCalcScreen()
        let movePicker = element(app, "movePicker")
        scrollUntilHittable(app, movePicker)
        tapUpperPart(movePicker)
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))

        XCTAssertTrue(
            element(app, "moveSearchResult-\(Self.physicalMoveID)").waitForExistence(timeout: Self.existenceTimeout),
            "ダメージ技は出る")
        XCTAssertTrue(element(app, "moveSearchResult-\(Self.specialMoveID)").exists)
        XCTAssertFalse(element(app, "moveSearchResult-\(Self.statusMoveID)").exists, "変化技は出ない")

        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(Self.statusMoveName)
        XCTAssertFalse(
            element(app, "moveSearchResult-\(Self.statusMoveID)").waitForExistence(timeout: 1),
            "変化技の名前で検索しても出ない")
    }
}
