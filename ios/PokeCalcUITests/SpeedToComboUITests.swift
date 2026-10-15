import XCTest

/// I-ios-21(ADR-0542・ADR-0501「素早さ画面から連続技へ」AC-h1〜AC-h8): 素早さ画面の「この相手で連続技を計算」→ ダメージタブの計算画面の連続技。
/// `POKECALC_USE_MOCK=1` で起動する。素早さのモック(`MockSpeedService`)と計算のモック(`Resources/species.json`)は同じ番号の
/// 架空のポケモンを持つ(名前は別): 9003-000 = 素早さ「テストクサネコ」/ 計算「テストモンさん」、9004-000 = 「テストデンキネズミ」/「テストモンよん」。
/// 計算画面の既定は 攻撃側 9001-000・防御側 9002-000 なので、引き継ぐと両側が変わる。
/// 計算の攻撃側の既定は無振り・無補正の性格。モックの性格には「素早さ上昇・特攻下降」が無いので、今の振りは「準速」(SP 32・補正なし)で確かめる
/// (性格の置き換えの規則は `ComboViewModelHandoffTests`・`ComboViewModelTests` が固定する)。
///
/// 識別子(新規のみ。既存の識別子・読み上げ名は付け替えない):
///   素早さ画面: speedComboLink(想定のカードの直下・「これ以上」の上。自分と相手を選ぶと出る)
///   連続技の節: comboScarfIgnored(スカーフを指定していたときの 1 行)/ comboHandoffHelpButton(「説明」。引き継いで開いたときだけ)/
///              comboHandoffHelpLine-<0...>(「説明」の奥の箇条)
/// 直列(IOS_TEST_PARALLEL=0)で通る前提。AX5 の検査は遅いので `executionTimeAllowance = 600`(ADR-0520 の上限)。
@MainActor
final class SpeedToComboUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrollAttempts = 40
    private static let settleWait: TimeInterval = 0.2
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let selfID = "9003-000"
    private static let opponentID = "9004-000"
    private static let selfSpeedName = "テストクサネコ"
    private static let calcSelfName = "テストモンさん"
    private static let calcOpponentName = "テストモンよん"
    private static let defaultChips = ["uninvested", "neutral-max", "max", "max-scarf"]

    override func setUpWithError() throws {
        continueAfterFailure = false
        executionTimeAllowance = 600
    }

    // MARK: - 補助

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
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

    private func wait(_ target: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    private func waitFor(
        _ format: String, _ argument: CVarArg, _ target: XCUIElement, _ message: String,
        file: StaticString = #filePath, line: UInt = #line
    ) {
        let predicate = NSPredicate(format: format, argument)
        let result = XCTWaiter().wait(
            for: [XCTNSPredicateExpectation(predicate: predicate, object: target)], timeout: Self.existenceTimeout)
        XCTAssertEqual(result, .completed, "\(message)(\(format) \(argument)。今: label=\(target.label) value=\(String(describing: target.value)))", file: file, line: line)
    }

    private func waitValue(_ target: XCUIElement, _ value: String, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        wait(target, "要素が無い: \(target)", file: file, line: line)
        waitFor("value == %@", value, target, message, file: file, line: line)
    }

    /// ゆっくりスワイプして慣性を小さくする(既定の swipe は 1 回に画面 1 枚ぶん以上動き、小さな欄を飛び越える)。
    private func scrollStep(_ app: XCUIApplication, towardsBottom: Bool) {
        if towardsBottom { app.swipeUp(velocity: .slow) } else { app.swipeDown(velocity: .slow) }
        Thread.sleep(forTimeInterval: Self.settleWait)
    }

    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, file: StaticString = #filePath, line: UInt = #line) {
        wait(target, "要素が無い: \(target)", file: file, line: line)
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
        // 上端がナビゲーションバーの下に潜ると、タップがバーに当たる。引き戻してから返す(CalcAttackerStatsUITests と同じ対策)。
        settleBelowNavigationBar(app, target)
    }

    /// 慣性が止まるまで待ち、要素の上端がバーの下に潜っていたら、ゆっくり(velocity .slow)引き下げて見える位置に置く。
    private func settleBelowNavigationBar(_ app: XCUIApplication, _ target: XCUIElement) {
        RunLoop.current.run(until: Date().addingTimeInterval(0.8))
        let window = app.windows.firstMatch.frame
        let minTop = window.height * 0.2
        guard target.exists, target.frame.minY < minTop else { return }
        let delta = min(window.midY - target.frame.midY, window.height * 0.4)
        let start = app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.35))
        start.press(forDuration: 0.05, thenDragTo: start.withOffset(CGVector(dx: 0, dy: delta)), withVelocity: .slow, thenHoldForDuration: 0.3)
        RunLoop.current.run(until: Date().addingTimeInterval(0.8))
    }

    private func tap(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        scrollUntilHittable(app, target, file: file, line: line)
        target.tap()
    }

    private func pick(_ app: XCUIApplication, button: String, pokemonID: String) {
        tap(app, button)
        wait(element(app, "speedPokemonSheet"), "ポケモンのシートが開かない")
        let row = element(app, "speedPokemonRow-\(pokemonID)")
        wait(row, "speedPokemonRow-\(pokemonID) が無い")
        if !row.isHittable { row.swipeUp() }
        row.tap()
        XCTAssertTrue(element(app, "speedPokemonSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選んだらシートが閉じる")
    }

    private func pickBoth(_ app: XCUIApplication) {
        pick(app, button: "speedPokemonButton", pokemonID: Self.selfID)
        pick(app, button: "speedOpponentPokemonButton", pokemonID: Self.opponentID)
    }

    /// 素早さ画面で 自分・相手・今の振り(準速)・ランク +1・(任意で)スカーフ を入れる。
    private func fillSpeedScreen(_ app: XCUIApplication, scarf: Bool) {
        app.openScreen("openSpeedScreen")
        wait(element(app, "speedScreen"), "speedScreen が開かない")
        pickBoth(app)
        tap(app, "speedCurrentToggle")
        tap(app, "speedCurrentPreset-neutral-max")
        tap(app, "rankMeterPlus-speed")
        waitValue(element(app, "rankMeter-speed"), "+1", "素早さ画面のランク")
        if scarf {
            tap(app, "speedScarf")
            waitFor("isSelected == %@", NSNumber(value: true), element(app, "speedScarf"), "スカーフを選んだ")
        }
    }

    /// リンクを押して、ダメージタブの計算画面で連続技の節が開くまで待つ。
    private func tapComboLink(_ app: XCUIApplication) {
        tap(app, "speedComboLink")
        wait(element(app, "calcScreen"), "計算画面へ移らない")
        waitFor("isSelected == %@", NSNumber(value: true), app.tabButton(UITestNavigation.TabID.damage), "ダメージタブへ移る")
        wait(element(app, "comboAddPicker"), "連続技の節が開かない(追加の技が出ない)")
    }

    private func assertTapSize(_ target: XCUIElement, _ name: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(name) の幅", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(name) の高さ", file: file, line: line)
    }

    private func assertInsideWindow(_ app: XCUIApplication, _ identifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let target = element(app, identifier)
        scrollUntilHittable(app, target, file: file, line: line)
        let window = app.windows.firstMatch.frame
        let frame = target.frame
        XCTAssertGreaterThanOrEqual(frame.minX, window.minX - Self.overflowTolerance, "\(identifier) が左にはみ出す: \(frame)", file: file, line: line)
        XCTAssertLessThanOrEqual(frame.maxX, window.maxX + Self.overflowTolerance, "\(identifier) が右にはみ出す: \(frame)", file: file, line: line)
    }

    // MARK: - AC-h6: 素早さ画面のボタン

    /// 自分と相手を選ぶと、想定のカードの直下(「これ以上」の上)にボタンが出る。36pt 以上。
    func testLinkAppearsBelowTheCardsOnceBothPokemonArePicked() {
        let app = launch()
        app.openScreen("openSpeedScreen")
        wait(element(app, "speedScreen"), "speedScreen が開かない")
        XCTAssertFalse(element(app, "speedComboLink").exists, "未選択では出さない")
        pick(app, button: "speedPokemonButton", pokemonID: Self.selfID)
        XCTAssertFalse(element(app, "speedComboLink").exists, "相手が未選択では出さない")
        pick(app, button: "speedOpponentPokemonButton", pokemonID: Self.opponentID)

        let link = element(app, "speedComboLink")
        scrollUntilHittable(app, link)
        waitFor("label CONTAINS %@", "この相手で連続技を計算", link, "ボタンの名前")
        XCTAssertEqual(link.elementType, .button, "ボタンとして読み上げる")
        assertTapSize(link, "speedComboLink")
        for chip in Self.defaultChips {
            let card = element(app, "speedCard-\(chip)")
            wait(card, "想定 \(chip) のカードが無い")
            XCTAssertLessThanOrEqual(card.frame.maxY, link.frame.minY + Self.overflowTolerance, "ボタンは想定のカード(\(chip))の下")
        }
        let reachHint = element(app, "speedReachHint")
        wait(reachHint, "今の振りが無いときの「これ以上」の案内")
        XCTAssertLessThanOrEqual(link.frame.maxY, reachHint.frame.minY + Self.overflowTolerance, "ボタンは「これ以上」の上")
    }

    // MARK: - AC-h2〜AC-h5: 引き継いで連続技の節を開く

    /// 自分・相手・素早さ(SP・性格補正・ランク)が入り、節が開いて見える位置にある。スカーフは反映せず 1 行で知らせる。
    /// 引き継がない計算の入力(急所)は残る。
    func testLinkFillsTheComboSectionAndKeepsOtherCalcInputs() {
        let app = launch()
        app.openScreen("openCalcScreen")
        wait(element(app, "calcScreen"), "計算画面が開かない")
        tap(app, "calcConditionsToggle")
        tap(app, "calcCondition-critical")
        waitFor("isSelected == %@", NSNumber(value: true), element(app, "calcCondition-critical"), "前提: 急所を選んだ")

        fillSpeedScreen(app, scarf: true)
        tapComboLink(app)

        // 結果へ目が行く: スクロールせずに節の入力(追加の技)が見えている(フォーカスの移動と一緒に節へ送る)。
        waitFor("isHittable == %@", NSNumber(value: true), element(app, "comboAddPicker"), "節へ送られていない")
        XCTAssertFalse(element(app, "comboOpenButton").exists, "閉じた状態(「次の技」だけ)ではない")

        waitFor("label == %@", Self.calcSelfName, element(app, "attackerSpeciesPicker"), "攻撃側 = 素早さ画面の自分")
        waitFor("label == %@", Self.calcOpponentName, element(app, "defenderSpeciesPicker"), "防御側 = 素早さ画面の相手")
        waitValue(element(app, "comboSelfSpeedSP"), "32", "素早さの SP = 素早さ画面の今の振り")
        waitValue(element(app, "comboSelfSpeedNature"), "素早さ補正なし", "性格補正 = 素早さ画面の補正なし")
        waitValue(element(app, "rankMeter-comboSelfSpeed"), "+1", "素早さのランク = 素早さ画面のランク")

        let scarf = element(app, "comboScarfIgnored")
        scrollUntilHittable(app, scarf)
        waitFor("label CONTAINS %@", "こだわりスカーフ", scarf, "スカーフは反映できない旨の 1 行")

        let help = element(app, "comboHandoffHelpButton")
        scrollUntilHittable(app, help)
        assertTapSize(help, "comboHandoffHelpButton")
        XCTAssertFalse(element(app, "comboHandoffHelpLine-0").exists, "説明の箇条は最初は出ない")
        help.tap()
        wait(element(app, "comboHandoffHelpLine-0"), "説明の奥に引き継ぐもの")
        waitFor("label CONTAINS %@", "こだわりスカーフ", element(app, "comboHandoffHelpLine-1"), "説明の奥に引き継がないもの")

        let critical = element(app, "calcCondition-critical")
        scrollUntilHittable(app, critical)
        XCTAssertTrue(critical.isSelected, "引き継がない入力(急所)は残る")
    }

    /// タブを移っても入れ直さない(token が同じ)。節で変えた値・素早さ画面の入力はどちらも残る。もう一度押すと入れ直す。
    func testSwitchingTabsKeepsInputsAndPressingAgainReapplies() {
        let app = launch()
        fillSpeedScreen(app, scarf: true)
        tapComboLink(app)
        let meter = element(app, "rankMeter-comboSelfSpeed")
        waitValue(meter, "+1", "引き継いだランク")

        tap(app, "rankMeterPlus-comboSelfSpeed")
        waitValue(meter, "+2", "節でランクを変えた")
        XCTAssertTrue(element(app, "comboScarfIgnored").waitForNonExistence(timeout: Self.existenceTimeout), "自分の素早さを変えたらスカーフの 1 行は消える")

        app.selectTab(UITestNavigation.TabID.speed)
        wait(element(app, "speedScreen"), "素早さ画面に戻れない")
        waitValue(element(app, "rankMeter-speed"), "+1", "素早さ画面の入力は残る")
        waitFor("label CONTAINS %@", Self.selfSpeedName, element(app, "speedPokemonButton"), "素早さ画面の自分は残る")
        waitFor("isSelected == %@", NSNumber(value: true), element(app, "speedCurrentPreset-neutral-max"), "今の振りは残る")

        app.selectTab(UITestNavigation.TabID.damage)
        wait(element(app, "calcScreen"), "計算画面に戻れない")
        waitValue(meter, "+2", "タブを戻っただけでは入れ直さない(節で変えた値が残る)")
        wait(element(app, "comboAddPicker"), "節は開いたまま")
        waitFor("label == %@", Self.calcSelfName, element(app, "attackerSpeciesPicker"), "攻撃側は残る")

        app.selectTab(UITestNavigation.TabID.speed)
        tapComboLink(app)
        waitValue(meter, "+1", "もう一度押すと素早さ画面の値で入れ直す")
        wait(element(app, "comboScarfIgnored"), "スカーフの 1 行も出し直す")
    }

    /// 計算タブに画面を積んでいても(右上の「このアプリについて」など)、計算画面の根へ戻して節を見せる。
    func testLinkReturnsToTheCalcRootWhenSomethingIsPushed() {
        let app = launch()
        app.openScreen("openCalcScreen")
        wait(element(app, "calcScreen"), "計算画面が開かない")
        app.openScreen("openAboutScreen")
        wait(element(app, "aboutScreen"), "このアプリについてが開かない")

        fillSpeedScreen(app, scarf: false)
        tapComboLink(app)
        XCTAssertFalse(element(app, "aboutScreen").exists, "積んだ画面は外して計算画面の根を出す")
        XCTAssertFalse(element(app, "comboScarfIgnored").exists, "スカーフを指定していなければ 1 行は出さない")
    }

    // MARK: - AC-h7: 大きい文字(AX5)

    func testHandoffAtAX5HasNoHorizontalOverflow() {
        let app = launch(contentSizeCategory: Self.ax5ContentSizeCategory)
        app.openScreen("openSpeedScreen")
        wait(element(app, "speedScreen"), "speedScreen が開かない")
        pickBoth(app)
        tap(app, "speedScarf")
        let link = element(app, "speedComboLink")
        assertInsideWindow(app, "speedComboLink")
        assertTapSize(link, "speedComboLink")
        link.tap()
        wait(element(app, "comboAddPicker"), "連続技の節が開かない")

        for identifier in ["comboScarfIgnored", "comboHandoffHelpButton", "comboSelfSpeedSP", "comboSelfSpeedNature", "rankMeter-comboSelfSpeed"] {
            assertInsideWindow(app, identifier)
        }
        let help = element(app, "comboHandoffHelpButton")
        scrollUntilHittable(app, help)
        assertTapSize(help, "comboHandoffHelpButton")
        help.tap()
        wait(element(app, "comboHandoffHelpLine-0"), "AX5 でも説明が開く")
    }
}
