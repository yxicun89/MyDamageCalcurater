import XCTest

/// 構築編集の中の「タイプバランス」(ADR-0421・ADR-0422・ADR-0532。別画面は廃止)。
///
/// `POKECALC_USE_MOCK=1` に加え、結果を出す検査は `POKECALC_MOCK_BALANCE=fixture`(UI テスト専用の架空の応答。
/// `MockBalanceService`)で起動する。指定しないモック構成は従来どおり接続先なし(`balance_unavailable`)。
/// 架空の応答の形(ほのお=×2 弱点・候補 1 の該当ポケモン 10 体で先頭 4 体はモックのマスタにいる)は MockBalanceServiceTests が固定する。
@MainActor
final class TeamBalanceUITests: XCTestCase {
    /// 種族 250ms・バランス 300ms・おすすめ 1 秒の待ちを含めても余裕のある上限。
    private static let existenceTimeout: TimeInterval = 10
    private static let firstMockSpeciesName = "テストモンいち"
    /// おすすめの候補 1 の先頭のタイル(モックのマスタの「テストモンに」)。
    private static let firstTileID = "balancePickTile-1-9002-000"
    private static let firstTileName = "テストモンに"
    /// 候補 1 の 10 体目(「もっと見る」を押すまで出ない)。
    private static let tenthTileID = "balancePickTile-1-9106-000"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elements(_ app: XCUIApplication, beginningWith prefix: String) -> XCUIElementQuery {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix))
    }

    private func launchEditor(fixture: Bool) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if fixture { app.launchEnvironment["POKECALC_MOCK_BALANCE"] = "fixture" }
        app.launch()
        app.openScreen("openTeamListScreen")
        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 空の枠 `slot`(1 始まり)の種族を、検索シートで `name` に決める。
    private func pickSpecies(_ app: XCUIApplication, slot: Int, name: String) {
        let picker = element(app, "slotSpeciesPicker-\(slot)")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout), "\(slot)体目の「ポケモン」欄")
        scrollUntilHittable(app, picker)
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[name]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForNonExistence(timeout: Self.existenceTimeout), "選ぶとシートが閉じる")
    }

    /// 構築編集のスクロールの中で `target` が押せる位置に来るまでスクロールする(下へ進んで届かなければ上へ戻る)。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, maxSwipes: Int = 25) {
        let screen = element(app, "teamEditScreen")
        for _ in 0..<maxSwipes where !(target.exists && target.isHittable) {
            screen.swipeUp()
        }
        for _ in 0..<maxSwipes where !(target.exists && target.isHittable) {
            screen.swipeDown()
        }
        XCTAssertTrue(target.isHittable, "\(target) に届かない")
    }

    private func memberCardCount(_ app: XCUIApplication) -> Int {
        elements(app, beginningWith: "memberCard-").count
    }

    private func waitForMemberCards(_ app: XCUIApplication, _ count: Int) {
        let predicate = NSPredicate { _, _ in self.memberCardCount(app) == count }
        let expectation = XCTNSPredicateExpectation(predicate: predicate, object: nil)
        XCTAssertEqual(XCTWaiter().wait(for: [expectation], timeout: Self.existenceTimeout), .completed, "メンバーが \(count) 体になる")
    }

    // MARK: - 結果

    /// 1 体選ぶだけで(保存しなくても)節に防御相性と集計が出る。選ぶ前は案内の一言だけ。
    func testOneSpeciesShowsResultsInsideTheEditor() {
        let app = launchEditor(fixture: true)
        XCTAssertTrue(element(app, "teamBalanceSection").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "teamBalanceEmpty").exists, "選ぶ前は案内")
        XCTAssertEqual(element(app, "teamBalanceEmpty").label, "ポケモンを選ぶと、タイプの強み・弱みがここに出ます")
        XCTAssertFalse(element(app, "balanceTeamSummary").exists, "選ぶ前は結果を出さない")

        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)

        XCTAssertTrue(element(app, "balanceTeamSummary").waitForExistence(timeout: Self.existenceTimeout), "チームの集計")
        XCTAssertTrue(element(app, "balanceMemberDefense-1").exists, "1 体目の防御相性")
        let weakRow = app.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS %@", "×2 弱点")).firstMatch
        XCTAssertTrue(weakRow.exists, "倍率は文字でも出す(色だけにしない)")
        XCTAssertFalse(element(app, "teamBalanceEmpty").exists)
        XCTAssertTrue(element(app, "teamUnsavedNotice").exists, "保存しないまま結果が出る")
    }

    // MARK: - おすすめのタイル

    /// タイルを押すと構築の最初の空き枠(2 体目)に入る。
    func testTappingATileFillsTheFirstEmptySlot() {
        let app = launchEditor(fixture: true)
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        let tile = element(app, Self.firstTileID)
        XCTAssertTrue(tile.waitForExistence(timeout: Self.existenceTimeout), "おすすめの該当ポケモンのタイル")
        XCTAssertEqual(tile.label, Self.firstTileName)
        XCTAssertTrue(tile.isEnabled)

        scrollUntilHittable(app, tile)
        tile.tap()

        waitForMemberCards(app, 2)
        XCTAssertFalse(element(app, "slotSpeciesPicker-2").exists, "2 体目の枠が埋まる")
        XCTAssertTrue(element(app, "slotSpeciesPicker-3").exists, "3 体目は空のまま")
        let picked = elements(app, beginningWith: "memberSpeciesPicker-").matching(NSPredicate(format: "label == %@", Self.firstTileName))
        XCTAssertEqual(picked.count, 1, "入れたのはタイルのポケモン")
    }

    /// 6 枠が埋まるとタイルは押せず、理由を一言出す。
    func testTilesCannotBePickedWhenAllSlotsAreFilled() {
        let app = launchEditor(fixture: true)
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        let tile = element(app, Self.firstTileID)
        XCTAssertTrue(tile.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "balancePickNoEmptySlot-1").exists, "空きがあるうちは理由を出さない")

        for count in 2...6 {
            scrollUntilHittable(app, tile)
            tile.tap()
            waitForMemberCards(app, count)
        }

        XCTAssertTrue(tile.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(tile.isEnabled, "満杯なら押せない")
        let reason = element(app, "balancePickNoEmptySlot-1")
        XCTAssertTrue(reason.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(reason.label, "空き枠がありません。枠を外すと入れられます")
        XCTAssertEqual(memberCardCount(app), 6, "7 体目は入らない")
    }

    /// 1 候補のタイルは 8 個まで。残りは「もっと見る」で出る。
    func testMoreButtonRevealsTheRestOfTheTiles() {
        let app = launchEditor(fixture: true)
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        let more = element(app, "balancePickMore-1")
        XCTAssertTrue(more.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(elements(app, beginningWith: "balancePickTile-1-").count, 8, "最初は 8 個")
        XCTAssertFalse(element(app, Self.tenthTileID).exists)

        scrollUntilHittable(app, more)
        more.tap()

        XCTAssertTrue(element(app, Self.tenthTileID).waitForExistence(timeout: Self.existenceTimeout), "残りが出る")
        XCTAssertEqual(elements(app, beginningWith: "balancePickTile-1-").count, 10)
    }

    // MARK: - balance が使えないとき

    /// 接続先が無くても構築の編集・保存は止まらない。原因は節の中だけに出し、再計算のボタンがある。
    func testBalanceUnavailableKeepsEditingAndSaving() {
        let app = launchEditor(fixture: false)
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)

        let error = element(app, "balanceAnalysisError")
        XCTAssertTrue(error.waitForExistence(timeout: Self.existenceTimeout), "節の中にエラー")
        XCTAssertEqual(error.label, "タイプバランスのサーバーに接続できません")
        XCTAssertTrue(element(app, "teamBalanceRetry").exists, "再計算のボタン")
        XCTAssertFalse(element(app, "teamEditErrorMessage").exists, "構築の側にはエラーを出さない")

        pickSpecies(app, slot: 2, name: "テストモンに")
        waitForMemberCards(app, 2)
        element(app, "saveTeamButton").tap()
        XCTAssertTrue(element(app, "teamSavedNotice").waitForExistence(timeout: Self.existenceTimeout), "保存できる")
    }
}
