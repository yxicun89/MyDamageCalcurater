import XCTest

/// ルート画面の骨組み(P6-1。ADR-0531 で下部タブ)を確認する。`POKECALC_USE_MOCK=1` で起動してモックを強制する
/// (ADR-0500 §5・§7)。計算画面そのものは P6-2 なので、ここでは遷移がつながることだけを見る。
/// `XCUIApplication` / `XCUIElement` は MainActor 隔離(Swift 6 の厳格な並行性チェック)なので、
/// テストクラス自体を `@MainActor` にする。
@MainActor
final class PokeCalcUITests: XCTestCase {

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launchWithMock() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        return app
    }

    /// 起動直後に、モックで動いていることの表示が見える。ADR-0531 でホーム(状態バッジ `backendModeBadge` のあった画面)を
    /// 廃止し、起動直後はダメージタブの計算になったので、計算画面の同じ表示(`calcBackendModeBadge`)で確かめる。
    func testBackendModeBadgeIsVisibleAtLaunch() {
        let app = launchWithMock()
        let badge = app.staticTexts["calcBackendModeBadge"]
        XCTAssertTrue(badge.waitForExistence(timeout: 5))
        XCTAssertTrue(badge.label.contains("モック"), "label: \(badge.label)")
    }

    func testOpenCalcScreenNavigatesToCalcScreen() {
        let app = launchWithMock()
        app.openScreen("openCalcScreen")
        // 計算画面のルートは P6-2a でプレースホルダの `Text` から実画面(`ScrollView`)に変わった。
        // 要素の種類を決め打ちしないよう `.any` で探す(P6-2a の画面の中身は CalcScreenUITests)。
        XCTAssertTrue(app.descendants(matching: .any)["calcScreen"].waitForExistence(timeout: 5))
    }
}
