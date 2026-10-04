import XCTest

/// P6-24: 素早さ比較画面(ADR-0503。受け入れ条件は ADR-0501「P6-24 の受け入れ条件」)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ。ADR-0500 §5・§7)。素早さのモックの
/// 挙動は環境変数 `POKECALC_MOCK_SPEED`(`table-error` / `position-error` / `pokemon-error` / `all-error`)で切り替える。
///
/// 数値(実数値・同速の行数)は検査しない(ADR-0501「XCUITest で確かめること」と同じ粒度)。確かめるのは
/// 「導線・要素の有無・並び・選択状態・失敗が他に波及しないこと」。例外として、モックの固定の事実
/// (ADR-0503 §8: 4 体 × 6 行 = 24 行)に頼るのは「実数値 1 は全行より遅い」ケースの `speedResultFaster` だけ。
///
/// 識別子(ADR-0503 §9。implementer はこの名前で付ける):
///   ルート: openSpeedScreen / 画面: speedScreen
///   自分: speedMode-<preset|custom|raw> / speedPokemonButton(ポケモンのシートを開く)/ speedPreset-<uninvested|neutral-max|max> /
///         speedScarf / speedSelfTailwind / speedParalysis / speedNature-<minus|neutral|plus> /
///         speedSPValue・speedSPIncrement・speedSPDecrement / speedRankValue・speedRankIncrement・speedRankDecrement /
///         speedRawValueField / speedRawValueError
///   ピッカー(シート): speedPokemonSheet / speedPokemonSearchField / speedPokemonRow-<pokemonId> / speedPokemonNoMatch
///   結果: speedResult / speedResultSpeed / speedResultFaster / speedResultSlower / speedResultTieEntry-<index> / speedResultNoTie /
///         speedResultMovesBefore / speedResultMovesAfter / speedPositionLoading
///   表: speedTable / speedFilter-<PresetId> / speedFilterMinimumNotice / speedTableTailwind / speedTrickRoom /
///       speedTier-<speed> / speedTierTie-<speed>(同速の段だけ)/ speedTierSelf-<speed>(自分と同じ段だけ)/ speedBoundary
///   失敗: speedTableError / speedPositionError / speedPokemonError
///   オン/オフの操作(フィルター・スカーフ・追い風・まひ・トリックルーム・ピル)は `isSelected` で状態を出す(計算画面の
///   `calcCondition-*` と同じ流儀)。
///
/// P6-24 の spec 時点では View が未実装のため、このテストは失敗してよい。
@MainActor
final class SpeedScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let maxScrolls = 12
    private static let allPresetIDs = ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"]
    private static let mockPokemonA = "9001-000"
    private static let mockPokemonB = "9002-000"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動・補助

    private func launch(scenario: String? = nil, openAtLaunch: Bool = false) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_SPEED"] = scenario }
        if openAtLaunch { app.launchEnvironment["POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH"] = "1" }
        app.launch()
        return app
    }

    private func launchSpeedScreen(scenario: String? = nil) -> XCUIApplication {
        let app = launch(scenario: scenario)
        let open = element(app, "openSpeedScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openSpeedScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "speedScreen").waitForExistence(timeout: Self.existenceTimeout), "speedScreen が開かない")
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elements(_ app: XCUIApplication, prefix: String) -> [XCUIElement] {
        let query = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix))
        return (0..<query.count).map { query.element(boundBy: $0) }
    }

    /// `prefix` で始まる identifier の要素が1つでも現れるまで待つ(speedTier-<speed> のように値が可変のもの用)。
    private func waitForAny(_ app: XCUIApplication, prefix: String, _ message: String, file: StaticString = #filePath, line: UInt = #line) {
        let first = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout), message, file: file, line: line)
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

    private func pickPokemon(_ app: XCUIApplication, _ pokemonID: String) {
        tap(app, "speedPokemonButton")
        wait(element(app, "speedPokemonSheet"), "ポケモンのシートが開かない")
        let row = element(app, "speedPokemonRow-\(pokemonID)")
        wait(row, "speedPokemonRow-\(pokemonID) が無い")
        row.tap()
        XCTAssertTrue(element(app, "speedPokemonSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選んだらシートが閉じる")
    }

    private func enterRawValue(_ app: XCUIApplication, _ text: String) {
        tap(app, "speedMode-raw")
        let field = element(app, "speedRawValueField")
        wait(field, "speedRawValueField が無い")
        scrollUntilHittable(app, field)
        field.tap()
        field.typeText(text)
    }

    private func tierSpeeds(_ app: XCUIApplication) -> [(speed: Int, minY: CGFloat)] {
        elements(app, prefix: "speedTier-")
            .compactMap { el in
                Int(el.identifier.dropFirst("speedTier-".count)).map { (speed: $0, minY: el.frame.minY) }
            }
    }

    // MARK: - 導線

    func testOpenSpeedScreenFromRootShowsTheTable() {
        let app = launchSpeedScreen()
        wait(element(app, "speedTable"), "表の領域が無い")
        waitForAny(app, prefix: "speedTier-", "表の段(speedTier-*)が出る")
        for id in Self.allPresetIDs {
            XCTAssertTrue(element(app, "speedFilter-\(id)").exists, "絞り込み speedFilter-\(id) が無い")
        }
    }

    func testOpensAtLaunchWithTheEnvironmentVariable() {
        let app = launch(openAtLaunch: true)
        wait(element(app, "speedScreen"), "POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH=1 で素早さ画面が開かない")
    }

    // MARK: - 絞り込み・場の状態

    /// 最後の1つは外せない(契約上 presets は1つ以上)。説明が出て、表は出続ける。
    func testTheLastFilterCannotBeTurnedOff() {
        let app = launchSpeedScreen()
        wait(element(app, "speedTable"), "表の領域が無い")
        for id in Self.allPresetIDs.dropLast() {
            tap(app, "speedFilter-\(id)")
        }
        let last = element(app, "speedFilter-\(Self.allPresetIDs[Self.allPresetIDs.count - 1])")
        wait(element(app, "speedFilterMinimumNotice"), "1つだけ残ったら説明が出る")
        expectSelected(last, true, "最後の1つは選ばれている")

        tap(app, "speedFilter-\(Self.allPresetIDs[Self.allPresetIDs.count - 1])")
        expectSelected(last, true, "最後の1つは外せない")
        waitForAny(app, prefix: "speedTier-", "表は出続ける")
    }

    /// トリックルーム中は表の並びが遅い順になる(段の speed が上から昇順)。
    func testTrickRoomReversesTheTableOrder() {
        let app = launchSpeedScreen()
        wait(element(app, "speedTable"), "表の領域が無い")
        waitForAny(app, prefix: "speedTier-", "表の段が無い")
        let normal = tierSpeeds(app).sorted { $0.minY < $1.minY }.map(\.speed)
        XCTAssertEqual(normal, normal.sorted(by: >), "既定は速い順")

        tap(app, "speedTrickRoom")
        expectSelected(element(app, "speedTrickRoom"), true, "トリックルームが選ばれる")
        let deadline = Date().addingTimeInterval(Self.existenceTimeout)
        var reversed = false
        while Date() < deadline, !reversed {
            let speeds = tierSpeeds(app).sorted { $0.minY < $1.minY }.map(\.speed)
            reversed = speeds.count > 1 && speeds == speeds.sorted()
            if !reversed { RunLoop.current.run(until: Date().addingTimeInterval(0.1)) }
        }
        XCTAssertTrue(reversed, "トリックルーム中は遅い順(昇順)")
    }

    // MARK: - 自分のポケモン(raw)

    /// 実数値 1 は表のどの行より遅い: 境界線は表の末尾(すべての段の下)に出て、自分と同じ段は無い。
    func testRawValueBelowEveryRowDrawsTheBoundaryAtTheBottom() {
        let app = launchSpeedScreen()
        enterRawValue(app, "1")
        wait(element(app, "speedResultSpeed"), "結果の実数値が出ない")
        let faster = element(app, "speedResultFaster")
        wait(faster, "speedResultFaster が無い")
        XCTAssertTrue(faster.label.contains("24"), "モックの表は 24 行(ADR-0503 §8)。実際: \(faster.label)")
        XCTAssertTrue(element(app, "speedResultSlower").exists)
        XCTAssertTrue(element(app, "speedResultNoTie").exists, "同速なし")

        // 表は遅延描画(I-speed-2)なので、末尾の境界線は表の下までスクロールしてから現れる。
        let boundary = element(app, "speedBoundary")
        for _ in 0..<(Self.maxScrolls * 2) where !boundary.exists { app.swipeUp() }
        wait(boundary, "境界線(speedBoundary)が出ない")
        XCTAssertTrue(elements(app, prefix: "speedTierSelf-").isEmpty, "自分と同じ段は無い")
        let lowestTierBottom = tierSpeeds(app).map(\.minY).max() ?? 0
        XCTAssertGreaterThan(boundary.frame.minY, lowestTierBottom, "実数値 1 の境界は、どの段より下")
    }

    func testInvalidRawValueShowsAMessageAndNoResult() {
        let app = launchSpeedScreen()
        enterRawValue(app, "0")
        wait(element(app, "speedRawValueError"), "範囲外のメッセージが出ない")
        XCTAssertFalse(element(app, "speedResultSpeed").exists, "範囲外は送らない(結果は出ない)")
    }

    // MARK: - 遅延描画と自分の位置(I-speed-2 / F-06。ADR-0517)

    /// 表は画面付近の段だけを作る(全段を一度に生成しない)。総数は見出しの読み上げ「全N段」から読む。
    func testTableRendersOnlyNearbyTiersAndReadsOutTheTotal() {
        let app = launchSpeedScreen()
        let position = element(app, "speedTablePosition")
        scrollUntilHittable(app, position)
        wait(position, "総段数の読み上げ(speedTablePosition)が無い")
        let digits = position.label.drop { !$0.isNumber }.prefix { $0.isNumber }
        let total = Int(digits) ?? 0
        XCTAssertGreaterThan(total, 0, "総段数が読めない: \(position.label)")
        waitForAny(app, prefix: "speedTier-", "表の段が出る")
        XCTAssertLessThan(tierSpeeds(app).count, total, "遅延描画: 全 \(total) 段を一度に作らない")
    }

    /// 自分の行が画面外のとき「自分の位置へ」が出て、押すと自分の行(境界線)が見える。
    func testJumpToSelfBringsTheSelfRowIntoView() {
        let app = launchSpeedScreen()
        enterRawValue(app, "1")
        wait(element(app, "speedResultSpeed"), "結果の実数値が出ない")
        // 表の見出しに着くまで少しずつ送る(送りすぎて自分の行が見えると、ボタンは出ない)。
        let jump = element(app, "speedJumpToSelf")
        for _ in 0..<Self.maxScrolls where !(jump.exists && jump.isHittable) {
            app.swipeUp(velocity: .slow)
        }
        wait(jump, "自分の行が画面外なのに speedJumpToSelf が無い")
        jump.tap()
        let boundary = element(app, "speedBoundary")
        wait(boundary, "移動後に自分の位置(speedBoundary)が描かれる")
        XCTAssertTrue(boundary.isHittable, "移動後は自分の位置が画面内にある")
    }

    // MARK: - 自分のポケモン(preset)

    /// 最速 + スカーフは、同じポケモンの表の行(最速スカーフ・最速+1)と同速になる(ADR-0503 §8)。
    func testPresetMaxWithScarfShowsATieAndHighlightsTheSameSpeedTier() {
        let app = launchSpeedScreen()
        pickPokemon(app, Self.mockPokemonA)
        tap(app, "speedPreset-max")
        tap(app, "speedScarf")
        expectSelected(element(app, "speedScarf"), true, "スカーフが選ばれる")

        wait(element(app, "speedResultSpeed"), "結果の実数値が出ない")
        wait(element(app, "speedResultTieEntry-0"), "同速の一覧が出る")
        XCTAssertFalse(element(app, "speedResultNoTie").exists)
        XCTAssertFalse(element(app, "speedBoundary").exists, "同じ実数値の段があるときは境界線を引かない")
        waitForAny(app, prefix: "speedTierSelf-", "自分と同じ段が強調される")
        XCTAssertFalse(elements(app, prefix: "speedTierTie-").isEmpty, "同速の段にはバッジが出る")
    }

    func testCustomModeShowsItsControlsAndAResult() {
        let app = launchSpeedScreen()
        tap(app, "speedMode-custom")
        for id in ["speedSPValue", "speedSPIncrement", "speedSPDecrement", "speedRankValue", "speedRankIncrement", "speedRankDecrement"] {
            wait(element(app, id), "\(id) が無い")
        }
        for nature in ["minus", "neutral", "plus"] {
            XCTAssertTrue(element(app, "speedNature-\(nature)").exists, "speedNature-\(nature) が無い")
        }
        XCTAssertFalse(element(app, "speedResultSpeed").exists, "ポケモン未選択では結果は出ない")
        pickPokemon(app, Self.mockPokemonB)
        tap(app, "speedSPIncrement")
        tap(app, "speedNature-plus")
        expectSelected(element(app, "speedNature-plus"), true, "性格補正が選ばれる")
        wait(element(app, "speedResultSpeed"), "カスタムでも結果が出る")
    }

    // MARK: - ポケモンのピッカー

    func testPokemonPickerFiltersByName() {
        let app = launchSpeedScreen()
        tap(app, "speedPokemonButton")
        wait(element(app, "speedPokemonSheet"), "ポケモンのシートが開かない")
        wait(element(app, "speedPokemonRow-\(Self.mockPokemonA)"), "一覧が出る")
        XCTAssertTrue(element(app, "speedPokemonRow-\(Self.mockPokemonB)").exists)

        let field = element(app, "speedPokemonSearchField")
        wait(field, "検索欄が無い")
        field.tap()
        field.typeText("ミズ")
        wait(element(app, "speedPokemonRow-\(Self.mockPokemonB)"), "絞り込み後も一致する行は残る")
        XCTAssertTrue(
            element(app, "speedPokemonRow-\(Self.mockPokemonA)").waitForNonExistence(timeout: Self.existenceTimeout),
            "一致しない行は消える")
    }

    // MARK: - 失敗の独立(絶対ルール5を含む)

    /// 表だけ失敗しても、自分のポケモンの結果は出る。
    func testTableFailureDoesNotBlockThePosition() {
        let app = launchSpeedScreen(scenario: "table-error")
        wait(element(app, "speedTableError"), "表の失敗が表示されない")
        enterRawValue(app, "100")
        wait(element(app, "speedResultSpeed"), "表が失敗していても位置は出る")
        XCTAssertFalse(element(app, "speedPositionError").exists)
    }

    /// 位置だけ失敗しても、表は出続ける。
    func testPositionFailureKeepsTheTable() {
        let app = launchSpeedScreen(scenario: "position-error")
        enterRawValue(app, "100")
        wait(element(app, "speedPositionError"), "位置の失敗が表示されない")
        XCTAssertFalse(element(app, "speedTableError").exists)
        XCTAssertFalse(elements(app, prefix: "speedTier-").isEmpty, "表は出続ける")
        XCTAssertFalse(element(app, "speedBoundary").exists, "位置が失敗したら境界線は引かない")
    }

    /// ポケモン一覧が失敗しても、実数値入力と表は使える。
    func testPokemonFailureStillAllowsRawInput() {
        let app = launchSpeedScreen(scenario: "pokemon-error")
        wait(element(app, "speedPokemonError"), "一覧の失敗が表示されない")
        enterRawValue(app, "100")
        wait(element(app, "speedResultSpeed"), "一覧が失敗していても raw は使える")
        XCTAssertFalse(elements(app, prefix: "speedTier-").isEmpty)
    }

    /// 絶対ルール5: 素早さが全面的に失敗しても、ルートから計算画面はこれまでどおり開く。
    func testCalcScreenStillWorksWhenSpeedFailsEntirely() {
        let app = launchSpeedScreen(scenario: "all-error")
        wait(element(app, "speedTableError"), "表の失敗が表示されない")
        app.navigationBars.buttons.firstMatch.tap()
        let calc = element(app, "openCalcScreen")
        wait(calc, "ルートに戻れない")
        calc.tap()
        wait(element(app, "calcScreen"), "素早さが失敗しても計算画面は開く")
    }
}
