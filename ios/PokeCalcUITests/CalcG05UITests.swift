import XCTest

/// G-05 第 5 弾(ADR-0541。ADR-0501「G-05 計算の作り直し」章): 計算画面を、文で説明せず、見出し・ポケモンカード・増減ボタン・チップ・
/// 大きい数字と確定数のバッジで分からせる作りに直した結果を、画面の要素の有無・位置の順・タップの大きさ(36pt 以上)・AX5 のはみ出しで確かめる。
/// 文面そのものは `G05CalcTextTests`、ソースの決まりは `G05CalcSourceRulesTests` が固定する。既存の `CalcScreenUITests` ほかは挙動と識別子・
/// 読み上げ名を守る(ここは新しい構造だけ)。新しい識別子は実装前は無いので失敗してよい(新しい識別子は ADR-0501 の表)。
/// `POKECALC_USE_MOCK=1` で起動する(モックの既定: 物理技・5 行・相性は等倍・防御側の持ち物の比較チップ `test-item-berry`)。
@MainActor
final class CalcG05UITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let firstRowID = "none@-"
    private static let mockItemID = "test-item-berry"

    override func setUpWithError() throws {
        continueAfterFailure = false
        // AX5 の検査は遅いスワイプで時間がかかる。上限の 600 秒まで許す(ADR-0520)。
        executionTimeAllowance = 600
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launchCalcScreen(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        app.openScreen("openCalcScreen")
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(
            element(app, "calcResultRow-\(Self.firstRowID)").waitForExistence(timeout: Self.existenceTimeout), "起動時の行が出る")
        return app
    }

    /// 画面を少しずつ上へ送って、要素が押せるようにする(下へ行き過ぎたら戻る)。
    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication, maxAttempts: Int = 40) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        var attempts = 0
        while !target.isHittable && attempts < maxAttempts {
            element(app, "calcScreen").swipeUp(velocity: .slow)
            attempts += 1
        }
        attempts = 0
        while !target.isHittable && attempts < maxAttempts {
            element(app, "calcScreen").swipeDown(velocity: .slow)
            attempts += 1
        }
        RunLoop.current.run(until: Date().addingTimeInterval(0.6))
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)")
    }

    private func assertTapTarget(_ target: XCUIElement, _ name: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(name) が無い", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(name) の高さ", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(name) の幅", file: file, line: line)
    }

    private func assertInsideWindow(_ app: XCUIApplication, _ targets: [XCUIElement], file: StaticString = #filePath, line: UInt = #line) {
        let width = app.windows.firstMatch.frame.width
        for target in targets {
            scrollTo(target, in: app)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(target.identifier) が左にはみ出す", file: file, line: line)
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(target.identifier) が右にはみ出す", file: file, line: line)
        }
    }

    private func openConditions(_ app: XCUIApplication) {
        let toggle = element(app, "calcConditionsToggle")
        scrollTo(toggle, in: app)
        if (toggle.value as? String) != "開いています" { toggle.tap() }
        XCTAssertTrue(element(app, "calcConditionsPanel").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func dismissKeyboard(_ app: XCUIApplication) {
        let done = app.buttons["calcKeyboardDone"]
        if done.waitForExistence(timeout: Self.existenceTimeout) { done.tap() }
    }

    // MARK: - 見出しとカード

    /// 「攻撃側」「防御側」の見出し(見出しの特性)がカードの上にあり、カードは従来の識別子のまま。回り続ける読み込み表示が無い。
    func testSideHeadingsSitAboveTheCards() {
        let app = launchCalcScreen()
        let attackerHeading = element(app, "calcAttackerHeading")
        let defenderHeading = element(app, "calcDefenderHeading")
        XCTAssertTrue(attackerHeading.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(attackerHeading.label, "攻撃側")
        XCTAssertEqual(defenderHeading.label, "防御側")
        let attackerCard = element(app, "attackerCard")
        let defenderCard = element(app, "defenderCard")
        XCTAssertLessThanOrEqual(attackerHeading.frame.maxY, attackerCard.frame.minY + Self.overflowTolerance, "見出しはカードの上")
        XCTAssertLessThanOrEqual(defenderHeading.frame.maxY, defenderCard.frame.minY + Self.overflowTolerance, "見出しはカードの上")
        XCTAssertEqual(app.activityIndicators.count, 0, "回り続ける読み込み表示を置かない(静止したアイコン + 一言)")
    }

    /// ポケモンの欄・持ち物の欄・特性の欄はカードの中にあり、36pt 以上。特性は「詳細」の中ではない。
    func testAbilityAndItemPickersLiveInsideTheCards() {
        let app = launchCalcScreen()
        let attackerCard = element(app, "attackerCard")
        let defenderCard = element(app, "defenderCard")
        let attackerSpecies = element(app, "attackerSpeciesPicker")
        let attackerItem = element(app, "attackerItemPicker")
        let attackerAbility = element(app, "calcAttackerAbilityPicker")
        let defenderSpecies = element(app, "defenderSpeciesPicker")
        let defenderAbility = element(app, "calcDefenderAbilityPicker")
        for (name, picker, card) in [
            ("攻撃側のポケモン", attackerSpecies, attackerCard), ("攻撃側の持ち物", attackerItem, attackerCard),
            ("攻撃側の特性", attackerAbility, attackerCard), ("防御側のポケモン", defenderSpecies, defenderCard),
            ("防御側の特性", defenderAbility, defenderCard),
        ] {
            assertTapTarget(picker, name)
            XCTAssertTrue(card.frame.insetBy(dx: -Self.overflowTolerance, dy: -Self.overflowTolerance).contains(picker.frame), "\(name)はカードの中")
        }
        // 「詳細」を開いても、特性の欄は増えない(カードの中の 1 つだけ。詳細の中には無い)。
        openConditions(app)
        XCTAssertEqual(
            app.descendants(matching: .any).matching(identifier: "calcAttackerAbilityPicker").count, 1, "特性の欄は 1 つだけ")
        XCTAssertEqual(
            app.descendants(matching: .any).matching(identifier: "calcDefenderAbilityPicker").count, 1, "特性の欄は 1 つだけ")
    }

    /// 攻守入れ替えはアイコンのボタン(読み上げ名は従来のまま)で 36pt 以上。押すと種族が入れ替わる。
    func testSwapIsAnIconButton() {
        let app = launchCalcScreen()
        let swap = app.buttons["swapSidesButton"]
        assertTapTarget(swap, "攻守を入れ替える")
        XCTAssertEqual(swap.label, "攻守を入れ替える", "読み上げ名は従来のまま")
        XCTAssertEqual(swap.staticTexts.count, 0, "アイコンだけ(見える文字を置かない)")
    }

    // MARK: - 技の欄

    /// 技の欄は 36pt 以上。読み上げは技名を含み、値に 威力・分類・相性 が入る(見える文の要約は無くなった)。
    func testMovePickerKeepsNameAndMovesSummaryToValue() {
        let app = launchCalcScreen()
        let picker = app.buttons["movePicker"]
        assertTapTarget(picker, "技")
        XCTAssertFalse(picker.label.isEmpty)
        let value = picker.value as? String ?? ""
        XCTAssertTrue(value.contains("威力"), "値に威力: \(value)")
        XCTAssertTrue(value.contains("等倍"), "値に相性(モックの既定は等倍): \(value)")
    }

    // MARK: - プリセット・攻撃/特攻のブロック

    /// プリセットのピル・性格補正の 3 択は 36pt 以上。選択の特性(isSelected)は従来のまま。
    func testPresetsAndNatureChoicesAreBigEnoughAndKeepSelection() {
        let app = launchCalcScreen()
        for id in ["attackerPreset-none", "attackerPreset-aFull", "attackerPreset-aMax"] {
            assertTapTarget(app.buttons[id], id)
        }
        XCTAssertTrue(app.buttons["attackerPreset-none"].isSelected)
        let neutral = element(app, "attackerNature-atk-neutral")
        scrollTo(neutral, in: app)
        for choice in ["up", "neutral", "down"] {
            assertTapTarget(element(app, "attackerNature-atk-\(choice)"), "攻撃の性格補正 \(choice)")
        }
        XCTAssertTrue(neutral.isSelected)
        XCTAssertEqual(neutral.label, "補正なし", "読み上げ名は従来のまま")
    }

    /// SP は − / + のボタンと数値欄。0 で − が、32 で + が押せなくなる。数字を打つと反映され、不正な値は誤りとして出る(従来どおり)。
    func testSPStepperButtons() {
        let app = launchCalcScreen()
        let field = element(app, "attackerSPField-atk")
        let down = app.buttons["attackerSPDecrease-atk"]
        let up = app.buttons["attackerSPIncrease-atk"]
        scrollTo(field, in: app)
        assertTapTarget(down, "攻撃のSPを減らす")
        assertTapTarget(up, "攻撃のSPを増やす")
        assertTapTarget(field, "攻撃のSP")
        XCTAssertEqual(down.label, "攻撃のSPを減らす")
        XCTAssertEqual(up.label, "攻撃のSPを増やす")
        XCTAssertEqual(field.value as? String, "0")
        XCTAssertFalse(down.isEnabled, "0 で − は止まる")
        XCTAssertTrue(up.isEnabled)

        up.tap()
        up.tap()
        XCTAssertEqual(field.value as? String, "2")
        down.tap()
        XCTAssertEqual(field.value as? String, "1")
        XCTAssertTrue(down.isEnabled)
    }

    func testSPStepperStopsAtThirtyTwoAndFixesOutOfRangeText() {
        let app = launchCalcScreen()
        let field = element(app, "attackerSPField-atk")
        let down = app.buttons["attackerSPDecrease-atk"]
        let up = app.buttons["attackerSPIncrease-atk"]
        scrollTo(field, in: app)
        field.tap()
        field.typeText("32")
        dismissKeyboard(app)
        XCTAssertFalse(up.isEnabled, "32 で + は止まる")
        XCTAssertTrue(down.isEnabled)

        field.tap()
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: 2) + "33")
        dismissKeyboard(app)
        XCTAssertTrue(element(app, "attackerSPError-atk").waitForExistence(timeout: Self.existenceTimeout), "不正な値は従来どおり誤りとして出る")
        XCTAssertFalse(up.isEnabled)
        XCTAssertTrue(down.isEnabled)
        down.tap()
        XCTAssertEqual(field.value as? String, "32", "− で範囲内に戻す")
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: element(app, "attackerSPError-atk"), handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    // MARK: - 結果の行

    /// 結果の行は 1 行目に「大きいダメージ幅」と「確定数のバッジ」、その下に調整・持ち物(小さく)。
    func testResultRowPutsPercentAndKOFirstAndPresetBelow() {
        let app = launchCalcScreen()
        let percent = element(app, "calcResultPercent-\(Self.firstRowID)")
        let ko = element(app, "calcResultKO-\(Self.firstRowID)")
        let preset = element(app, "calcResultPreset-\(Self.firstRowID)")
        XCTAssertTrue(percent.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(ko.exists)
        XCTAssertTrue(preset.exists, "調整・持ち物の小さい一行(新しい識別子)")
        XCTAssertFalse(percent.label.isEmpty)
        XCTAssertFalse(ko.label.isEmpty)
        XCTAssertLessThan(abs(percent.frame.midY - ko.frame.midY), max(percent.frame.height, ko.frame.height), "同じ行に並ぶ")
        XCTAssertGreaterThanOrEqual(preset.frame.minY, max(percent.frame.maxY, ko.frame.maxY) - Self.overflowTolerance, "調整は下")
        XCTAssertEqual(app.activityIndicators.count, 0)
    }

    /// 技の相性は結果の先頭に 1 つだけ(枠だけのバッジ)。各行には繰り返さない。結果の一覧の読み上げ名は「計算結果」。
    func testEffectivenessBadgeIsShownOnceAboveTheRows() {
        let app = launchCalcScreen()
        let badge = element(app, "calcEffectivenessBadge")
        XCTAssertTrue(badge.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(badge.label, "等倍")
        XCTAssertEqual(app.descendants(matching: .any).matching(identifier: "calcEffectivenessBadge").count, 1)
        let firstRow = element(app, "calcResultRow-\(Self.firstRowID)")
        XCTAssertLessThanOrEqual(badge.frame.maxY, firstRow.frame.minY + Self.overflowTolerance, "相性は行より上")
        XCTAssertTrue(element(app, "calcResults").exists, "結果の一覧のコンテナ")
        XCTAssertEqual(element(app, "calcResults").label, "計算結果")
    }

    // MARK: - 詳細・チップ

    /// 「詳細」の開閉は アイコン + 名前のボタン(読み上げの値は 開いています / 閉じています)。中身は急所・やけど・天候・フィールド・壁・ランク。
    func testConditionsToggleReportsOpenState() {
        let app = launchCalcScreen()
        let toggle = element(app, "calcConditionsToggle")
        scrollTo(toggle, in: app)
        assertTapTarget(toggle, "詳細")
        XCTAssertEqual(toggle.value as? String, "閉じています")
        toggle.tap()
        XCTAssertTrue(element(app, "calcConditionsPanel").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(toggle.value as? String, "開いています")
    }

    /// 条件のチップ(急所・やけど・天候・フィールド・壁)と持ち物の比較のチップは 36pt 以上で、横スクロールにせず折り返して全部が画面の幅に収まる。
    func testConditionChipsWrapAndAreBigEnough() {
        let app = launchCalcScreen()
        openConditions(app)
        var ids = ["calcCondition-critical", "calcCondition-burn"]
        ids += ["none", "sun", "rain", "sand", "snow"].map { "calcWeather-\($0)" }
        ids += ["none", "electric", "grassy", "psychic", "misty"].map { "calcTerrain-\($0)" }
        ids += ["reflect", "lightScreen", "auroraVeil"].map { "calcDefenderScreen-\($0)" }
        let width = app.windows.firstMatch.frame.width
        for id in ids {
            let chip = element(app, id)
            scrollTo(chip, in: app)
            assertTapTarget(chip, id)
            XCTAssertGreaterThanOrEqual(chip.frame.minX, -Self.overflowTolerance, "\(id) が左にはみ出す")
            XCTAssertLessThanOrEqual(chip.frame.maxX, width + Self.overflowTolerance, "\(id) は折り返して画面の幅に収まる(横スクロールにしない)")
        }
        let compare = element(app, "defenderItemToggle-\(Self.mockItemID)")
        scrollTo(compare, in: app)
        assertTapTarget(compare, "持ち物で比較のチップ")
        XCTAssertLessThanOrEqual(compare.frame.maxX, width + Self.overflowTolerance)
    }

    // MARK: - 連続技の節(機能・文言・識別子は不変。見た目の部品だけ)

    /// 開くボタンと手の上へ・下へは 36pt 以上、回り続ける読み込み表示が無い。
    func testComboControlsAreBigEnoughAndStatic() {
        let app = launchCalcScreen()
        let open = app.buttons["comboOpenButton"]
        scrollTo(open, in: app)
        assertTapTarget(open, "次の技")
        open.tap()
        let down = app.buttons["comboStepDown-0"]
        XCTAssertTrue(down.waitForExistence(timeout: Self.existenceTimeout))
        assertTapTarget(down, "手を下へ")
        assertTapTarget(app.buttons["comboStepUp-0"], "手を上へ")
        XCTAssertEqual(app.activityIndicators.count, 0, "連続技の計算中も静止した表示")
    }

    // MARK: - AX5

    /// AX5 で、見出し・カード・SP のボタン・条件のチップが横にはみ出さず、ボタンは 36pt 以上のまま。
    func testNewPartsDoNotOverflowAtAX5() {
        let app = launchCalcScreen(contentSizeCategory: Self.ax5ContentSizeCategory)
        assertInsideWindow(
            app,
            [
                element(app, "calcAttackerHeading"), element(app, "attackerCard"), element(app, "calcAttackerAbilityPicker"),
                element(app, "swapSidesButton"), element(app, "calcDefenderHeading"), element(app, "defenderCard"),
                element(app, "calcDefenderAbilityPicker"), element(app, "movePicker"),
            ])
        for id in ["attackerSPDecrease-atk", "attackerSPIncrease-atk"] {
            let button = app.buttons[id]
            scrollTo(button, in: app)
            assertTapTarget(button, id)
        }
        assertInsideWindow(
            app, [element(app, "attackerSPField-atk"), element(app, "attackerSPDecrease-atk"), element(app, "attackerSPIncrease-atk")])
        openConditions(app)
        assertInsideWindow(
            app,
            [element(app, "calcWeather-snow"), element(app, "calcTerrain-misty"), element(app, "calcDefenderScreen-auroraVeil")])
    }
}
