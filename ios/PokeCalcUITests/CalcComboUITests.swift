import XCTest

/// 計算画面の「連続技」(CB-8。ADR-0535・ADR-0501「連続技」AC-i1・AC-i7・AC-i10・AC-i13)。
/// `POKECALC_USE_MOCK=1` で起動する。モックは計算せず `Resources/combo-results.json` の架空の決め打ちを返すので、
/// 文言の数値はそのフィクスチャの値(合計 91.4〜108.6%・2 手で倒す 51.6%・素早さ 120 → 180・耐える 87.5%)。
/// 要求の中身は `ComboViewModelTests`・`APIPokeCalcServiceComboTests` が固定する。ここは画面の出し分け・操作・大きさだけを見る。
@MainActor
final class CalcComboUITests: XCTestCase {
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrollAttempts = 40
    /// スワイプのあと、慣性が止まるのを待つ秒数。
    private static let settleWait: TimeInterval = 0.2
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    /// モックの架空の技(`Resources/moves.json`)。変化技 A は確率 100% の「攻撃 +1・素早さ +1」を持つ。
    private static let boostMoveID = "test-move-status-a"
    private static let opponentMoveID = "test-move-special-b"
    private static let waveDash = "\u{301C}"

    override func setUpWithError() throws {
        continueAfterFailure = false
        // AX5 の検査は遅いスワイプで 260 秒前後かかり、300 秒(run-xcode-tests.sh の既定)に近い。上限の 600 秒まで許す(ADR-0520)。
        executionTimeAllowance = 600
    }

    // MARK: - 補助

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchCalcScreen(environment: [String: String] = [:], contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        for (key, value) in environment { app.launchEnvironment[key] = value }
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        app.openScreen("openCalcScreen")
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(
                element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout), "起動時の行が無い: \(rowID)")
        }
        return app
    }

    /// 画面を上下に少しずつ動かす。既定の `swipeUp()` は勢い(慣性)で 1 回に画面 1 枚ぶん以上動き、AX5 の長い画面で
    /// 44pt ほどの欄を飛び越えることがある。ゆっくりスワイプして慣性を小さくし、動きが止まるのを待つ。
    private func scrollStep(_ app: XCUIApplication, towardsBottom: Bool) {
        let screen = element(app, "calcScreen")
        if towardsBottom {
            screen.swipeUp(velocity: .slow)
        } else {
            screen.swipeDown(velocity: .slow)
        }
        Thread.sleep(forTimeInterval: Self.settleWait)
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)", file: file, line: line)
        var attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            scrollStep(app, towardsBottom: true)
            attempts += 1
        }
        attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            scrollStep(app, towardsBottom: false)
            attempts += 1
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)", file: file, line: line)
    }

    private func tap(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        scrollUntilHittable(app, target, file: file, line: line)
        target.tap()
    }

    private func openCombo(_ app: XCUIApplication) {
        tap(app, "comboOpenButton")
        XCTAssertTrue(element(app, "comboAddPicker").waitForExistence(timeout: Self.existenceTimeout), "広げると追加の技が出る")
    }

    private func addStep(_ app: XCUIApplication, moveID: String) {
        tap(app, "comboAddPicker")
        let row = element(app, "comboAddMove-\(moveID)")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "追加の技の候補に \(moveID) が無い")
        if !row.isHittable { row.swipeUp() }
        row.tap()
    }

    private func selectOpponentMove(_ app: XCUIApplication, moveID: String) {
        tap(app, "comboOpponentMovePicker")
        let row = element(app, "comboOpponentMove-\(moveID)")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "相手の技の候補に \(moveID) が無い")
        if !row.isHittable { row.swipeUp() }
        row.tap()
    }

    private func waitForLabel(_ target: XCUIElement, _ label: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)", file: file, line: line)
        expectation(for: NSPredicate(format: "label == %@", label), evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    private func assertTapSize(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        scrollUntilHittable(app, target, file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(identifier) の幅", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(identifier) の高さ", file: file, line: line)
    }

    // MARK: - AC-i1: 開閉

    func testCollapsedSectionShowsOnlyTheNextMoveButtonAndOpensAndCloses() {
        let app = launchCalcScreen()
        let open = element(app, "comboOpenButton")
        scrollUntilHittable(app, open)
        XCTAssertEqual(open.label, "次の技")
        XCTAssertFalse(element(app, "comboResult").exists, "手がメインだけのときは計算しない")
        XCTAssertFalse(element(app, "comboAddPicker").exists)

        openCombo(app)
        XCTAssertTrue(element(app, "comboStep-0").exists, "1手目 = 上の技欄の技")
        XCTAssertFalse(element(app, "comboStepRemove-0").exists, "メインは外せない")
        XCTAssertTrue(element(app, "comboDefenderPicker").exists)
        XCTAssertTrue(element(app, "comboOpponentMovePicker").exists)
        XCTAssertTrue(element(app, "comboOpponentSpeedToggle").exists)
        XCTAssertTrue(element(app, "comboSelfSpeedSP").exists)
        XCTAssertTrue(element(app, "rankMeter-comboSelfSpeed").exists, "自分の素早さのランクはランクのメーター")
        XCTAssertTrue(element(app, "comboSpeedFieldToggle").exists)
        XCTAssertFalse(element(app, "comboTrickRoom").exists, "素早さの場は折りたたみ")
        XCTAssertTrue(element(app, "comboNote").exists)
        XCTAssertFalse(element(app, "comboResult").exists, "開いただけでは計算しない")

        tap(app, "comboCloseButton")
        XCTAssertTrue(element(app, "comboOpenButton").waitForExistence(timeout: Self.existenceTimeout), "閉じると「次の技」に戻る")
    }

    // MARK: - AC-i7: 結果

    func testAddingAStepShowsTheResult() {
        let app = launchCalcScreen()
        openCombo(app)
        addStep(app, moveID: Self.boostMoveID)

        XCTAssertTrue(element(app, "comboStep-1").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "comboResult").waitForExistence(timeout: Self.existenceTimeout))
        waitForLabel(element(app, "comboTotal"), "合計 91.4\(Self.waveDash)108.6%")
        waitForLabel(element(app, "comboKO"), "2 手で倒す 51.6%")
        waitForLabel(element(app, "comboSpeedSummary"), "素早さ 120 → 180")
        waitForLabel(element(app, "comboSpeedLine"), "179 以下を抜く / 180 は同速 / 181 以上は相手が先")
        XCTAssertTrue(element(app, "comboResultStep-1").label.contains("ダメージなし"), "変化技の手")
        XCTAssertTrue(element(app, "comboResultStep-1").label.contains("素早さ +1"), "ランクのチップ")
        XCTAssertTrue(element(app, "comboTotalBar").exists)
        XCTAssertFalse(element(app, "comboCloseButton").exists, "手が 2 つ以上なら閉じない")
        XCTAssertTrue(element(app, "comboSpeedExample").waitForExistence(timeout: Self.existenceTimeout), "相手の素早さが分からないときは例")

        // 外すと計算をやめ、閉じられる状態に戻る。
        tap(app, "comboStepRemove-1")
        XCTAssertTrue(element(app, "comboCloseButton").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "comboResult").exists)
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").exists, "単発の結果は出続ける: \(rowID)")
        }
    }

    func testOpponentMoveShowsSurvival() {
        let app = launchCalcScreen()
        openCombo(app)
        selectOpponentMove(app, moveID: Self.opponentMoveID)
        XCTAssertTrue(element(app, "comboResult").waitForExistence(timeout: Self.existenceTimeout), "相手の技だけでも計算する")
        waitForLabel(element(app, "comboSurvive"), "耐える 87.5%")
        waitForLabel(element(app, "comboActions"), "相手の行動 1\(Self.waveDash)2回")
        XCTAssertTrue(element(app, "comboPerHit").exists)
    }

    func testOpponentSpeedFromDefenderShowsTheValueInsteadOfTheLine() {
        let app = launchCalcScreen()
        openCombo(app)
        addStep(app, moveID: Self.boostMoveID)
        XCTAssertTrue(element(app, "comboSpeedLine").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "comboOpponentSpeedToggle")
        waitForLabel(element(app, "comboOpponentSpeed"), "相手の素早さ 150")
        XCTAssertFalse(element(app, "comboSpeedLine").exists)
        XCTAssertFalse(element(app, "comboSpeedExample").exists)
    }

    // MARK: - AC-i10: 失敗ともう一度

    func testFailureStaysInsideTheSectionAndRetryRecovers() {
        let app = launchCalcScreen(environment: ["POKECALC_MOCK_COMBO": "fail-once"])
        openCombo(app)
        addStep(app, moveID: Self.boostMoveID)

        let error = element(app, "comboError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(error.label.hasPrefix("連続技を計算できませんでした"), error.label)
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "計算画面のエラーは出さない(絶対ルール 5)")
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").exists, "単発の結果は壊れない: \(rowID)")
        }
        assertTapSize(app, "comboRetry")
        tap(app, "comboRetry")
        XCTAssertTrue(element(app, "comboResult").waitForExistence(timeout: Self.existenceTimeout), "もう一度で計算できる")
        XCTAssertFalse(element(app, "comboError").exists)
    }

    // MARK: - AC-i13: 大きさ

    func testControlsAreAtLeast36pt() {
        let app = launchCalcScreen()
        assertTapSize(app, "comboOpenButton")
        openCombo(app)
        addStep(app, moveID: Self.boostMoveID)
        XCTAssertTrue(element(app, "comboStep-1").waitForExistence(timeout: Self.existenceTimeout))
        for identifier in [
            "comboAddPicker", "comboStepUp-1", "comboStepDown-0", "comboStepRemove-1", "comboStepCritical-0",
            "comboOpponentMovePicker", "comboDefenderPicker", "comboOpponentSpeedToggle", "comboSpeedFieldToggle",
        ] {
            assertTapSize(app, identifier)
        }
        XCTAssertFalse(element(app, "comboStepCritical-1").exists, "変化技の手には急所を出さない")
        XCTAssertFalse(element(app, "comboStepUp-0").isEnabled, "先頭の手の上へは押せない")
        XCTAssertFalse(element(app, "comboStepDown-1").isEnabled, "最後の手の下へは押せない")
    }

    func testComboSectionHasNoHorizontalOverflowAtAX5() {
        let app = launchCalcScreen(contentSizeCategory: Self.ax5ContentSizeCategory)
        openCombo(app)
        addStep(app, moveID: Self.boostMoveID)
        XCTAssertTrue(element(app, "comboResult").waitForExistence(timeout: Self.existenceTimeout))
        tap(app, "comboSpeedFieldToggle")
        XCTAssertTrue(element(app, "comboTrickRoom").waitForExistence(timeout: Self.existenceTimeout))

        let window = app.windows.firstMatch.frame
        for identifier in [
            "comboSection", "comboStep-0", "comboStep-1", "comboStepUp-1", "comboStepRemove-1", "comboAddPicker",
            "comboDefenderPicker", "comboOpponentMovePicker", "comboOpponentSpeedToggle", "comboSelfSpeedSP",
            "rankMeter-comboSelfSpeed", "comboTrickRoom", "comboSelfTailwind", "comboOpponentTailwind",
            "comboResult", "comboTotal", "comboKO", "comboSpeedSummary", "comboSpeedLine", "comboNote",
        ] {
            let target = element(app, identifier)
            scrollUntilHittable(app, target)
            let frame = target.frame
            XCTAssertGreaterThanOrEqual(frame.minX, window.minX - Self.overflowTolerance, "\(identifier) が左にはみ出す: \(frame)")
            XCTAssertLessThanOrEqual(frame.maxX, window.maxX + Self.overflowTolerance, "\(identifier) が右にはみ出す: \(frame)")
        }
        assertTapSize(app, "comboStepRemove-1")
        assertTapSize(app, "comboTrickRoom")
    }
}
