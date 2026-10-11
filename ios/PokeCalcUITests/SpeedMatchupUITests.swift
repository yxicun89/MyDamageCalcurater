import XCTest

/// I-speed-4c: 素早さ画面を「相手を選んで上を取る最低ライン」に作り直す(ADR-0610 §4。iOS の画面の具体は同 ADR の追記)。
/// Web(I-speed-4b)と同じ体験: 入力(自分・相手・場)→ 出力 1(想定ごとのカード)・出力 2(「これ以上」)。表と周辺パネルは画面に出ない。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ)。モックの数値は簡易な式で、
/// ここで固定する数値は `MockSpeedMatchupTests` の手計算と同じもの(架空の 4 体。A = 種族値 100 / D = 130)。
/// 自分 = A(9001-000)、相手 = D(9004-000)。
///   無振り D = 150: 自分 A は 補正なし S31 以上・上昇 S18 以上で抜ける(上昇 S17 は同速)→ 見出し「準速で抜ける」
///   最速スカーフ D = 300: A はどうやっても届かない → 見出し「最速でも届かない」
///   自分の今の振りが 最速(SP 32・上昇)のとき A = 167: 無振り D(150)には先に動き、最速 D(200)には後に動く。
///   自分 167 のときの「これ以上」: 無振り 146 族まで抜ける・147 族と同速・148 族以上に抜かれる / 準速 114 族まで・115 族と同速・116 族以上。
///
/// 識別子(implementer はこの名前で付ける。ADR-0610 追記):
///   画面: speedScreen
///   自分: speedPokemonButton(ピッカーを開く)/ speedRankValue・speedRankIncrement・speedRankDecrement /
///         speedScarf / speedSelfTailwind / speedSelfParalysis /
///         speedCurrentToggle(今の振りを入れる)/ speedCurrentPreset-<uninvested|neutral-max|max>(入れたときだけ見える)/
///         speedSPValue・speedSPIncrement・speedSPDecrement / speedNature-<minus|neutral|plus>
///   相手: speedOpponentPokemonButton / speedAssumption-<PresetId>(6 つ。既定で uninvested・neutral-max・max・max-scarf が選択)/
///         speedOpponentTailwind / speedOpponentParalysis
///         speedAssumptionMinimumNotice(想定が 1 つだけ残ったとき)/ speedPokemonError(ポケモン一覧の失敗)
///   場:   speedTrickRoom
///   ピッカー(自分・相手で共通): speedPokemonSheet / speedPokemonSearchField / speedPokemonRow-<pokemonId> / speedPokemonNoMatch
///   出力 1: speedMatchupHint(未選択)/ speedMatchupLoading / speedMatchupError /
///           speedCard-<id> / speedCardHeadline-<id> / speedCardOpponentSpeed-<id> /
///           speedCardNature-<id>-<nature>(本文)/ speedCardTie-<id>-<nature>(同速があるときだけ)/ speedCardOrder-<id>(今の振りがあるときだけ)
///           <id> = PresetId(例 max-scarf)。下降の行(…-minus)はトリックルーム中だけ
///   出力 2: speedReachHint(今の振りが無い)/ speedReachLoading / speedReachError /
///           speedReach / speedReachRow-<PresetId> / speedReachSlower-<PresetId> / speedReachTie-<PresetId>(同速の族があるときだけ)/
///           speedReachFaster-<PresetId>
///   連続技: speedComboLink(連続技が計算画面に入るまで出ない)
///   画面に出ないもの: speedTable / speedTier-* / speedBoundary / speedFilter-* / speedMode-* / speedResult* /
///                    speedNeighborhood* / speedJumpToSelf / speedTablePosition
///   オン/オフの操作(チップ・スカーフ・追い風・まひ・トリックルーム・今の振り)は `isSelected` で状態を出す。
///
/// 既存の SpeedScreenUITests / SpeedNeighborhoodUITests / LargeTextLayoutUITests の素早さ表前提のテストは、
/// この画面に置き換わるとき implementer が整理する(書き換え案は ADR-0610 追記)。このファイルは新構成の受け入れ条件。
/// spec 時点では View が未実装のため、失敗してよい。
@MainActor
final class SpeedMatchupUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 12
    private static let mockSelf = "9001-000"
    private static let mockOpponent = "9004-000"
    private static let defaultChips = ["uninvested", "neutral-max", "max", "max-scarf"]
    private static let extraChips = ["max-plus1", "max-plus2"]
    /// 新しい画面に出ない旧構成の識別子(完全一致)。
    private static let removedIdentifiers = [
        "speedTable", "speedBoundary", "speedJumpToSelf", "speedTablePosition", "speedResult", "speedResultSpeed",
        "speedFilter-max", "speedMode-raw", "speedMode-custom", "speedNeighborhood", "speedNeighborhoodPanel",
    ]
    /// 新しい画面に出ない旧構成の識別子(前方一致)。
    private static let removedPrefixes = ["speedTier-", "speedFilter-", "speedMode-", "speedNeighborhood"]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動・補助

    private func launchSpeedScreen(scenario: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_SPEED"] = scenario }
        app.launch()
        let open = element(app, "openSpeedScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openSpeedScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout), "speedScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func anyElement(_ app: XCUIApplication, prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    private func wait(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    private func waitGone(_ element: XCUIElement, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForNonExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
    }

    /// 画面外の要素をタップできる位置までスクロールする(画面は縦に長い単一の ScrollView)。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement) {
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeUp()
        }
        for _ in 0..<Self.maxScrolls where !(target.exists && target.isHittable) {
            app.swipeDown()
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

    /// 要素のラベルに `text` が含まれるようになるまで待つ。
    private func expectLabel(_ target: XCUIElement, contains text: String, file: StaticString = #filePath, line: UInt = #line) {
        let predicate = NSPredicate(format: "label CONTAINS %@", text)
        let result = XCTWaiter().wait(for: [XCTNSPredicateExpectation(predicate: predicate, object: target)], timeout: Self.existenceTimeout)
        XCTAssertEqual(result, .completed, "ラベルに「\(text)」が無い: \(target.label)", file: file, line: line)
    }

    private func pick(_ app: XCUIApplication, button: String, pokemonID: String) {
        tap(app, button)
        wait(element(app, "speedPokemonSheet"), "ポケモンのシートが開かない")
        let row = element(app, "speedPokemonRow-\(pokemonID)")
        wait(row, "speedPokemonRow-\(pokemonID) が無い")
        row.tap()
        waitGone(element(app, "speedPokemonSheet"), "選んだらシートが閉じる")
    }

    private func pickBoth(_ app: XCUIApplication) {
        pick(app, button: "speedPokemonButton", pokemonID: Self.mockSelf)
        pick(app, button: "speedOpponentPokemonButton", pokemonID: Self.mockOpponent)
    }

    private func assertOldLayoutIsGone(_ app: XCUIApplication, file: StaticString = #filePath, line: UInt = #line) {
        for id in Self.removedIdentifiers {
            XCTAssertFalse(element(app, id).exists, "表・周辺パネル・旧入力の \(id) が画面に残っている", file: file, line: line)
        }
        for prefix in Self.removedPrefixes {
            XCTAssertFalse(anyElement(app, prefix: prefix).exists, "旧構成の \(prefix)* が画面に残っている", file: file, line: line)
        }
    }

    // MARK: - 1. 入力 → 想定ごとのカード(最低ライン)。表は無い

    func testPickingBothPokemonShowsOneCardPerAssumptionAndNoTable() {
        let app = launchSpeedScreen()

        // 入力: 自分・相手・場。想定のチップは既定で 4 つ選ばれ、+1 / +2 は追加できる。
        for id in ["speedPokemonButton", "speedOpponentPokemonButton", "speedScarf", "speedSelfTailwind", "speedSelfParalysis",
            "speedOpponentTailwind", "speedOpponentParalysis", "speedTrickRoom", "speedRankValue", "speedCurrentToggle"]
        {
            wait(element(app, id), "入力 \(id) が無い")
        }
        for chip in Self.defaultChips {
            expectSelected(element(app, "speedAssumption-\(chip)"), true, "想定 \(chip) は既定で選ばれる")
        }
        for chip in Self.extraChips {
            expectSelected(element(app, "speedAssumption-\(chip)"), false, "想定 \(chip) は既定では選ばれない")
        }
        wait(element(app, "speedMatchupHint"), "未選択のときは案内が出る")
        assertOldLayoutIsGone(app)
        XCTAssertFalse(element(app, "speedComboLink").exists, "連続技が未実装の間は連続技へのリンクを出さない")

        // 選ぶと想定ごとのカードが出る。
        pickBoth(app)
        for chip in Self.defaultChips {
            wait(element(app, "speedCard-\(chip)"), "想定 \(chip) のカードが出ない")
        }
        XCTAssertFalse(element(app, "speedMatchupHint").exists)
        expectLabel(element(app, "speedCardOpponentSpeed-uninvested"), contains: "150")
        expectLabel(element(app, "speedCardHeadline-uninvested"), contains: "準速で抜ける")
        expectLabel(element(app, "speedCardNature-uninvested-neutral"), contains: "S31以上で抜ける")
        expectLabel(element(app, "speedCardNature-uninvested-plus"), contains: "S18以上で抜ける")
        expectLabel(element(app, "speedCardTie-uninvested-plus"), contains: "S17で同速")
        expectLabel(element(app, "speedCardHeadline-max-scarf"), contains: "最速でも届かない")
        expectLabel(element(app, "speedCardNature-max-scarf-plus"), contains: "届かない")
        XCTAssertFalse(element(app, "speedCardNature-uninvested-minus").exists, "下降の行はトリックルーム中だけ")
        XCTAssertFalse(element(app, "speedCardOrder-uninvested").exists, "今の振りを入れるまで行動順は出ない")
        XCTAssertFalse(element(app, "speedReach").exists, "今の振りを入れるまで「これ以上」は出ない")
        wait(element(app, "speedReachHint"), "「これ以上」の案内が出る")
        assertOldLayoutIsGone(app)

        // 想定のチップ: 追加するとカードが増え、外すとカードが減る。
        tap(app, "speedAssumption-max-plus1")
        wait(element(app, "speedCard-max-plus1"), "追加した想定のカードが出る")
        tap(app, "speedAssumption-uninvested")
        waitGone(element(app, "speedCard-uninvested"), "外した想定のカードは消える")
        XCTAssertTrue(element(app, "speedCard-max").exists, "他のカードは残る")
    }

    // MARK: - 2. 今の振り・トリックルーム・「これ以上」

    func testCurrentInvestmentAndTrickRoomShowOrderAndReach() {
        let app = launchSpeedScreen()
        pickBoth(app)
        wait(element(app, "speedCard-uninvested"), "カードが出ない")

        // 今の振りを入れる(最速 = SP 32・上昇 → 自分は 167)。行動順のバッジと「これ以上」が出る。
        tap(app, "speedCurrentToggle")
        expectSelected(element(app, "speedCurrentToggle"), true, "今の振りが入る")
        tap(app, "speedCurrentPreset-max")
        expectSelected(element(app, "speedCurrentPreset-max"), true, "最速が選ばれる")
        expectLabel(element(app, "speedCardOrder-uninvested"), contains: "先に動く")
        expectLabel(element(app, "speedCardOrder-max"), contains: "後に動く")
        let reach = element(app, "speedReach")
        wait(reach, "今の振りを入れたら「これ以上」が出る")
        for preset in ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"] {
            wait(element(app, "speedReachRow-\(preset)"), "「これ以上」の行 \(preset) が無い(表の 6 プリセットぶん)")
        }
        expectLabel(element(app, "speedReachSlower-uninvested"), contains: "146族まで抜ける")
        expectLabel(element(app, "speedReachTie-uninvested"), contains: "147族と同速")
        expectLabel(element(app, "speedReachFaster-uninvested"), contains: "148族以上に抜かれる")
        expectLabel(element(app, "speedReachSlower-neutral-max"), contains: "114族まで抜ける")
        expectLabel(element(app, "speedReachFaster-neutral-max"), contains: "116族以上に抜かれる")
        assertOldLayoutIsGone(app)

        // トリックルーム: 下降の行が出て、見出し・バッジ・「これ以上」の言い換えが切り替わる。
        tap(app, "speedTrickRoom")
        expectSelected(element(app, "speedTrickRoom"), true, "トリックルームが選ばれる")
        wait(element(app, "speedCardNature-uninvested-minus"), "トリックルーム中は下降の行が出る")
        expectLabel(element(app, "speedCardHeadline-uninvested"), contains: "下を取れる")
        expectLabel(element(app, "speedCardNature-uninvested-neutral"), contains: "S29まで下を取れる")
        expectLabel(element(app, "speedCardOrder-uninvested"), contains: "後に動く")
        expectLabel(element(app, "speedReachSlower-uninvested"), contains: "146族までは後に動く")
        expectLabel(element(app, "speedReachFaster-uninvested"), contains: "148族以上には先に動ける")
        XCTAssertTrue(element(app, "speedReachRow-max").exists, "トリックルームでも「これ以上」は消えない")

        // 今の振りを外すと、バッジと「これ以上」が消える。
        tap(app, "speedCurrentToggle")
        waitGone(element(app, "speedReach"), "今の振りを外したら「これ以上」は消える")
        XCTAssertFalse(element(app, "speedCardOrder-uninvested").exists)
        XCTAssertTrue(element(app, "speedCard-uninvested").exists, "カードは残る")
    }

    // MARK: - 3. 失敗は入力を壊さず、他の画面に波及しない(絶対ルール 5)

    func testMatchupFailureKeepsInputsAndDoesNotBlockTheCalcScreen() {
        let app = launchSpeedScreen(scenario: "matchup-error")
        pickBoth(app)
        let error = element(app, "speedMatchupError")
        wait(error, "最低ラインの失敗が表示されない")
        expectLabel(error, contains: "素早さのサーバーに接続できません")
        XCTAssertFalse(anyElement(app, prefix: "speedCard-").exists, "失敗のときはカードを出さない")
        XCTAssertTrue(element(app, "speedAssumption-max").exists, "入力は残る")

        // 「これ以上」の失敗は別に表示され、最低ラインの失敗と独立している。
        tap(app, "speedCurrentToggle")
        tap(app, "speedCurrentPreset-max")
        wait(element(app, "speedReachError"), "「これ以上」の失敗が別に表示される")
        XCTAssertTrue(element(app, "speedMatchupError").exists)

        // 絶対ルール 5: 素早さが失敗しても、ルートから計算画面はこれまでどおり開く。
        app.navigationBars.buttons.firstMatch.tap()
        let calc = element(app, "openCalcScreen")
        wait(calc, "ルートに戻れない")
        calc.tap()
        wait(element(app, "calcScreen"), "素早さが失敗しても計算画面は開く")
    }
}
