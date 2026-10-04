import XCTest

/// 持ち物の役割の絞り込みとメガ種族の持ち物固定(ADR-0509)。`POKECALC_USE_MOCK=1` でモックを強制する。
///
/// モック(`Resources/items.json`・`species.json`。ADR-0509 §9):
/// - 攻撃側専用 `test-item-attack-only`・防御側専用 `test-item-defense-only`・役割なし `test-item-no-role`・
///   メガストーン `test-item-mega-stone`(`nameJa`「テストどうぐメガいし」は画面のどこにも出てはならない)
/// - メガ種族「テストメガモンいち」(基本種「テストモンいち」)
@MainActor
final class MegaItemLockUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let megaSpeciesName = "テストメガモンいち"
    private static let nonMegaSpeciesName = "テストモンさん"
    private static let lockedStoneName = "テストモンいちのメガストーン"
    private static let stoneMasterName = "テストどうぐメガいし"
    private static let lockedReason = "メガシンカ: メガストーンを持ちます"
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"

    override func setUpWithError() throws {
        continueAfterFailure = false
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
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func selectAttacker(_ app: XCUIApplication, _ name: String) {
        let picker = element(app, "attackerSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[name]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout), "種族が無い: \(name)")
        // 最大の文字サイズでは行が高く(約183pt)、画面の大きい機種で結果の行の下端が画面からはみ出す。
        // 中央をタップすると画面の下端(ホームインジケータ付近)に当たって効かないので、行の上のほうをタップする。
        option.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.15)).tap()
    }

    /// 画面のどの要素のラベルにも、メガストーンのマスタの名前が出ていない。
    private func assertStoneMasterNameIsHidden(_ app: XCUIApplication, file: StaticString = #filePath, line: UInt = #line) {
        let leaked = app.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS %@", Self.stoneMasterName))
        XCTAssertEqual(leaked.count, 0, "メガストーンの nameJa が画面に出ている", file: file, line: line)
    }

    /// 攻撃側の持ち物の選択肢は攻撃側の役割の持ち物だけ。比較のトグルは防御側の役割の持ち物だけ。メガストーンはどこにも出ない。
    func testItemOptionsAreFilteredByRole() {
        let app = launchCalcScreen()

        XCTAssertTrue(element(app, "defenderItemToggle-test-item-defense-only").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "defenderItemToggle-test-item-berry").exists)
        XCTAssertFalse(element(app, "defenderItemToggle-test-item-attack-only").exists, "攻撃側専用は比較に出ない")
        XCTAssertFalse(element(app, "defenderItemToggle-test-item-no-role").exists)
        XCTAssertFalse(element(app, "defenderItemToggle-test-item-mega-stone").exists)

        let itemPicker = element(app, "attackerItemPicker")
        XCTAssertTrue(itemPicker.waitForExistence(timeout: Self.existenceTimeout))
        itemPicker.tap()
        XCTAssertTrue(app.buttons["テストどうぐこうげき"].waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(app.buttons["テストどうぐきのみ"].exists)
        XCTAssertFalse(app.buttons["テストどうぐぼうぎょ"].exists, "防御側専用は攻撃側に出ない")
        XCTAssertFalse(app.buttons["テストどうぐこうかなし"].exists)
        XCTAssertFalse(app.buttons[Self.stoneMasterName].exists)
    }

    /// メガ種族を選ぶと持ち物はメガストーンに固定(操作不可・日本語の名前・理由の文)。メガ以外に変えると解除して未選択。
    func testMegaAttackerLocksAndUnlocksItem() {
        let app = launchCalcScreen()

        selectAttacker(app, Self.megaSpeciesName)

        let itemPicker = element(app, "attackerItemPicker")
        XCTAssertTrue(itemPicker.waitForExistence(timeout: Self.existenceTimeout))
        let lockedLabel = NSPredicate(format: "label CONTAINS %@", Self.lockedStoneName)
        expectation(for: lockedLabel, evaluatedWith: itemPicker)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertFalse(itemPicker.isEnabled, "固定中は操作できない")
        let reason = element(app, "attackerItemLockReason")
        XCTAssertTrue(reason.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(reason.label, Self.lockedReason)
        assertStoneMasterNameIsHidden(app)

        selectAttacker(app, Self.nonMegaSpeciesName)

        expectation(for: NSPredicate(format: "isEnabled == true"), evaluatedWith: itemPicker)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertTrue(itemPicker.label.contains("持ち物なし"), "メガストーンを残さず未選択に戻る: \(itemPicker.label)")
        XCTAssertFalse(element(app, "attackerItemLockReason").exists)
    }

    /// AX5 でも固定の理由の文が画面の横幅に収まる(折り返して、はみ出さない)。
    func testLockReasonFitsAtAX5() {
        let app = launchCalcScreen(contentSizeCategory: Self.ax5ContentSizeCategory)

        selectAttacker(app, Self.megaSpeciesName)

        let reason = element(app, "attackerItemLockReason")
        XCTAssertTrue(reason.waitForExistence(timeout: Self.existenceTimeout))
        let window = app.windows.firstMatch.frame
        XCTAssertGreaterThanOrEqual(reason.frame.minX, window.minX - 1, "左にはみ出している")
        XCTAssertLessThanOrEqual(reason.frame.maxX, window.maxX + 1, "右にはみ出している")
    }
}
