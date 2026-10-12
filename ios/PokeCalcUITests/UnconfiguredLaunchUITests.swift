import XCTest

/// 接続先が空のビルドを `POKECALC_USE_MOCK` 無しで起動したとき(I-ios-19。ADR-0534)。
/// 黙ってモックに落とさず、「サーバーの接続先が設定されていません」を出す。テスト用データの名前
/// (「テスト」で始まる)・「モックデータで動作中」は出さない(モックの表示は POKECALC_USE_MOCK=1 のときだけ。文言は今のまま)。
/// 前提: テストのビルド設定の `POKECALC_API_BASE_URL` が空(`ios/PokeCalc/Config/PokeCalc.xcconfig` の既定)。
@MainActor
final class UnconfiguredLaunchUITests: XCTestCase {

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    func testEmptyBaseURLShowsNotConfiguredInsteadOfMock() {
        let app = XCUIApplication()
        app.launch()
        let notice = app.staticTexts["backendModeBadge"]
        XCTAssertTrue(notice.waitForExistence(timeout: 5))
        XCTAssertTrue(notice.label.contains("サーバーの接続先が設定されていません"), "label: \(notice.label)")
        XCTAssertFalse(app.staticTexts["calcBackendModeBadge"].exists, "計算画面(モック)に入らない")
        XCTAssertFalse(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'モックデータ'")).firstMatch.exists)
        app.openScreen("openAboutScreen")
        XCTAssertTrue(app.descendants(matching: .any)["aboutScreen"].waitForExistence(timeout: 5), "「このアプリについて」は開ける")
    }

    func testMockOnlyWhenExplicitlyRequested() {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let badge = app.staticTexts["calcBackendModeBadge"]
        XCTAssertTrue(badge.waitForExistence(timeout: 5))
        XCTAssertTrue(badge.label.contains("モック"), "label: \(badge.label)")
    }
}
