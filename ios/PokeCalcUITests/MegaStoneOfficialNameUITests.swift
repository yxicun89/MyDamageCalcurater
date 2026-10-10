import XCTest

/// メガストーンの正式名称(ADR-0509 追記 §6')。`POKECALC_USE_MOCK=1` でモックを強制する。
///
/// モックのメガ種族「テストメガモンいち」(基本種「テストモンいち」)のストーン `test-item-mega-stone` の nameJa は
/// 「テストどうぐメガいし」(日本語の文字を含む正式名称。モックの nameJa は「テスト」始まりの架空名に固定されるため、
/// 英語名のストーンはモックに置けない。英語名のフォールバックは単体テスト)。固定中の持ち物欄・結果の行には
/// 「テストモンいちのメガストーン」ではなく「テストどうぐメガいし」が出る。
@MainActor
final class MegaStoneOfficialNameUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let officialMegaSpeciesName = "テストメガモンいち"
    private static let officialStoneName = "テストどうぐメガいし"
    private static let composedStoneName = "テストモンいち専用のメガストーン"
    private static let lockedReason = "メガシンカするので、持ち物はメガストーンに決まっています"

    override func setUp() {
        continueAfterFailure = false
    }

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
        return app
    }

    private func selectAttacker(_ app: XCUIApplication, _ name: String) {
        let picker = element(app, "attackerSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[name]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout), "種族が無い: \(name)")
        option.tap()
    }

    /// 正式名称のストーンを持つメガ種族を選ぶと、持ち物欄(固定・操作不可)に正式名称が出て、組み立てた名前は出ない。
    func testOfficialStoneNameIsShownWhenLocked() {
        let app = launchCalcScreen()

        selectAttacker(app, Self.officialMegaSpeciesName)

        let itemPicker = element(app, "attackerItemPicker")
        XCTAssertTrue(itemPicker.waitForExistence(timeout: Self.existenceTimeout))
        expectation(for: NSPredicate(format: "label CONTAINS %@", Self.officialStoneName), evaluatedWith: itemPicker)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertFalse(itemPicker.isEnabled, "固定中は操作できない")
        XCTAssertEqual(element(app, "attackerItemLockReason").label, Self.lockedReason)
        let composed = app.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS %@", Self.composedStoneName))
        XCTAssertEqual(composed.count, 0, "正式名称があるときは「{基本種名}専用のメガストーン」を出さない")
    }

    /// 持ち物の選択肢には、正式名称のメガストーンも出ない(役割で外す。ADR-0509 §2)。
    func testOfficialStoneIsNotAnOption() {
        let app = launchCalcScreen()

        let itemPicker = element(app, "attackerItemPicker")
        XCTAssertTrue(itemPicker.waitForExistence(timeout: Self.existenceTimeout))
        itemPicker.tap()
        XCTAssertTrue(app.buttons["テストどうぐきのみ"].waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(app.buttons[Self.officialStoneName].exists)
    }

    /// 防御側がメガのとき、結果の行(持ち物名)にも正式名称が出る。
    func testOfficialStoneNameIsShownInResultRows() {
        let app = launchCalcScreen()

        let picker = element(app, "defenderSpeciesPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[Self.officialMegaSpeciesName]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()

        let named = app.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS %@", Self.officialStoneName))
        expectation(for: NSPredicate(format: "count > 0"), evaluatedWith: named)
        waitForExpectations(timeout: Self.existenceTimeout)
    }
}
