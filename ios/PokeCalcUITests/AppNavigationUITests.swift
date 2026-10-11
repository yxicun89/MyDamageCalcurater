import XCTest

/// ナビゲーション案 B(ADR-0531): 下部タブ 4 つ(ダメージ・構築・素早さ・記録)と、ダメージタブの上部の区切り
/// (計算・逆算・調整)。タブ・区切りを行き来しても入力が消えないこと、画面の中で積んだ状態が残ること、
/// 「このアプリについて」が各タブから開けること、構築タブからタイプバランスへ入れることを確かめる。
@MainActor
final class AppNavigationUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let allTabs = [
        UITestNavigation.TabID.damage, UITestNavigation.TabID.team, UITestNavigation.TabID.speed,
        UITestNavigation.TabID.records,
    ]
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    /// `Resources/species.json` の2番目(構築の下書きに入れる種族)。
    private static let secondMockSpeciesName = "テストモンに"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    private func waitSelected(_ element: XCUIElement, _ selected: Bool, _ message: String) {
        let predicate = NSPredicate(format: "isSelected == %@", NSNumber(value: selected))
        let expectation = XCTNSPredicateExpectation(predicate: predicate, object: element)
        XCTAssertEqual(XCTWaiter().wait(for: [expectation], timeout: Self.existenceTimeout), .completed, message)
    }

    // MARK: 骨組み

    /// 起動直後はホームではなく、ダメージタブの計算が開いている。タブのボタンは SwiftUI の `Tab` に付けた識別子で
    /// 引ける(決定記録の懸念「`Tab` の識別子がタブバーのボタンへ伝わるか」の検証)。ラベルは短い日本語。
    func testLaunchShowsDamageTabWithCalcAndTabBarIdentifiers() {
        let app = launch()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout), "起動直後は計算画面")
        let expectedLabels = ["ダメージ", "構築", "素早さ", "記録"]
        for (tabID, label) in zip(Self.allTabs, expectedLabels) {
            let tab = app.tabButton(tabID)
            XCTAssertTrue(tab.waitForExistence(timeout: Self.existenceTimeout), "タブ \(tabID) が識別子で引ける")
            XCTAssertEqual(tab.label, label, "タブ \(tabID) のラベル")
        }
        XCTAssertTrue(app.tabButton(UITestNavigation.TabID.damage).isSelected, "最初はダメージタブ")
        XCTAssertTrue(element(app, "tabSegments-damage").exists, "ダメージタブには区切りがある")
        for entry in ["openCalcScreen", "openReverseScreen", "openAdjustScreen"] {
            XCTAssertTrue(app.buttons[entry].exists, "区切り \(entry) が常に見えている")
            XCTAssertGreaterThanOrEqual(app.buttons[entry].frame.height, 36, "区切り \(entry) のタップの高さ")
        }
        XCTAssertTrue(app.buttons["openCalcScreen"].isSelected, "最初は計算が選ばれている")
        XCTAssertFalse(app.buttons["openJudgeScreen"].exists, "判定は既定で出さない")
    }

    /// 最大の文字(AX5)でも、区切りのボタンは画面の幅に収まり(はみ出さない)、押せる高さを保つ。タブも出る。
    func testSegmentsFitAtAX5() {
        let app = launch(contentSizeCategory: Self.ax5ContentSizeCategory)
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let window = app.windows.firstMatch.frame
        for entry in ["openCalcScreen", "openReverseScreen", "openAdjustScreen"] {
            let button = app.buttons[entry]
            XCTAssertTrue(button.waitForExistence(timeout: Self.existenceTimeout), entry)
            XCTAssertGreaterThanOrEqual(button.frame.minX, window.minX, "\(entry) が左へはみ出さない")
            XCTAssertLessThanOrEqual(button.frame.maxX, window.maxX, "\(entry) が右へはみ出さない")
            XCTAssertGreaterThanOrEqual(button.frame.height, 36, "\(entry) の押せる高さ")
            XCTAssertTrue(button.isHittable, "\(entry) が押せる")
        }
        for tabID in Self.allTabs {
            // タブバーのボタンは画面より後に支援技術へ出ることがある(17 Pro・並列実行で実測)ので、存在を待つ。
            XCTAssertTrue(
                app.tabButton(tabID).waitForExistence(timeout: Self.existenceTimeout), "AX5 でもタブ \(tabID) が出る")
        }
        app.openScreen("openAdjustScreen")
        XCTAssertTrue(element(app, "adjustScreen").waitForExistence(timeout: Self.existenceTimeout), "AX5 でも区切りで移れる")
    }

    // MARK: 入力の保持

    /// 計算で変えたプリセットが、別のタブへ移って戻っても残る。
    func testCalcInputSurvivesTabSwitch() {
        let app = launch()
        let preset = app.buttons["attackerPreset-aFull"]
        XCTAssertTrue(preset.waitForExistence(timeout: Self.existenceTimeout))
        preset.tap()
        waitSelected(preset, true, "プリセットを選んだ")

        app.selectTab(UITestNavigation.TabID.speed)
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout))
        app.selectTab(UITestNavigation.TabID.records)
        XCTAssertTrue(element(app, "favoritesScreen").waitForExistence(timeout: Self.existenceTimeout))
        app.selectTab(UITestNavigation.TabID.damage)

        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(app.buttons["attackerPreset-aFull"].isSelected, "タブを移って戻っても計算の入力が残る")
        XCTAssertFalse(app.buttons["attackerPreset-none"].isSelected)
    }

    /// 入力を変えた直後(計算の完了を待たず)にタブを移って戻っても、計算は取り消されず、新しい入力の結果が出ている
    /// (読み込み中のまま・エラーのままにならない。ADR-0531 §2)。モックは即時に返すので、途中の取り消しそのものは
    /// `RetainedScreenWorkTests` が確かめる。
    func testInputChangedJustBeforeTabSwitchShowsResultAfterReturning() {
        let app = launch()
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout))
        let preset = app.buttons["attackerPreset-aFull"]
        preset.tap()
        app.tabButton(UITestNavigation.TabID.speed).tap()
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout))
        app.selectTab(UITestNavigation.TabID.damage)

        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(preset.isSelected, "新しい入力が残る")
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout), "結果が出ている")
        XCTAssertTrue(
            element(app, "calcLoadingIndicator").waitForNonExistence(timeout: Self.existenceTimeout), "読み込み中のままにならない")
        XCTAssertFalse(element(app, "calcErrorMessage").exists, "取り消しをエラーにしない")
    }

    /// ダメージタブの区切り: 計算 → 逆算 → 計算 → 逆算 と切り替えても、それぞれの入力が残る。
    func testDamageSegmentsKeepEachInput() {
        let app = launch()
        let calcPreset = app.buttons["attackerPreset-aFull"]
        XCTAssertTrue(calcPreset.waitForExistence(timeout: Self.existenceTimeout))
        calcPreset.tap()
        waitSelected(calcPreset, true, "計算のプリセットを選んだ")

        app.openScreen("openReverseScreen")
        XCTAssertTrue(element(app, "reverseScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(app.buttons["openReverseScreen"].isSelected, "区切りの選択が移る")
        let addObservation = element(app, "reverseAddObservationButton")
        XCTAssertTrue(addObservation.waitForExistence(timeout: Self.existenceTimeout))
        addObservation.tap()
        XCTAssertTrue(element(app, "reverseObservationField-1").waitForExistence(timeout: Self.existenceTimeout))

        app.openScreen("openCalcScreen")
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(app.buttons["attackerPreset-aFull"].isSelected, "逆算から戻っても計算の入力が残る")
        // 隠れた逆算の画面が上に重なっていない(押した操作が計算に届く)。XCUITest は隠れた画面の要素も木に残すので、
        // 存在ではなく操作で確かめる。
        let otherPreset = app.buttons["attackerPreset-none"]
        otherPreset.tap()
        waitSelected(otherPreset, true, "計算のプリセットを押せる(隠れた逆算に遮られない)")
        calcPreset.tap()
        waitSelected(calcPreset, true, "計算のプリセットを選び直せる")

        app.openScreen("openReverseScreen")
        XCTAssertTrue(element(app, "reverseObservationField-1").waitForExistence(timeout: Self.existenceTimeout),
            "計算から戻っても逆算の入力(追加した観測欄)が残る")
    }

    /// 構築で作りかけた下書き(保存前・編集画面まで積んだ状態)が、素早さへ移って戻っても残る。
    func testTeamDraftSurvivesSwitchingToSpeedAndBack() {
        let app = launch()
        app.openScreen("openTeamListScreen")
        let create = element(app, "createTeamButton")
        XCTAssertTrue(create.waitForExistence(timeout: Self.existenceTimeout))
        create.tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        let slot = element(app, "slotSpeciesPicker-1")
        XCTAssertTrue(slot.waitForExistence(timeout: Self.existenceTimeout))
        slot.tap()
        let option = app.buttons[Self.secondMockSpeciesName]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").waitForExistence(timeout: Self.existenceTimeout))

        app.selectTab(UITestNavigation.TabID.speed)
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "teamEditScreen").exists)
        app.selectTab(UITestNavigation.TabID.team)

        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout),
            "構築タブに戻ると編集画面のまま(積んだ状態が残る)")
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").exists, "保存前の下書きが残る")
    }

    // MARK: 入口

    /// 「このアプリについて」は各タブの右上のアイコンから開ける(開いた画面はそのタブに積まれ、戻ると元の画面)。
    func testAboutOpensFromEveryTab() {
        let app = launch()
        let rootScreens = ["calcScreen", "teamListScreen", "speedScreen", "favoritesScreen"]
        for (tabID, rootScreen) in zip(Self.allTabs, rootScreens) {
            app.selectTab(tabID)
            XCTAssertTrue(element(app, rootScreen).waitForExistence(timeout: Self.existenceTimeout), "\(tabID) の根の画面")
            let about = app.buttons["openAboutScreen"]
            XCTAssertTrue(about.waitForExistence(timeout: Self.existenceTimeout), "\(tabID) に右上のアイコンがある")
            XCTAssertEqual(about.label, "このアプリについて")
            about.tap()
            XCTAssertTrue(element(app, "aboutScreen").waitForExistence(timeout: Self.existenceTimeout), "\(tabID) から開ける")
            app.navigationBars.buttons.element(boundBy: 0).tap()
            XCTAssertTrue(element(app, rootScreen).waitForExistence(timeout: Self.existenceTimeout), "\(tabID) へ戻る")
        }
    }

    /// タイプバランスは構築へ統合するまで、構築タブの一覧の行から開ける(機能を失わない)。
    func testBalanceIsReachableFromTeamTab() {
        let app = launch()
        app.openScreen("openBalanceScreen")
        XCTAssertTrue(element(app, "balanceScreen").waitForExistence(timeout: Self.existenceTimeout))
        app.navigationBars.buttons.element(boundBy: 0).tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
    }
}
