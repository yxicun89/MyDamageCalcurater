import XCTest

/// P6-18(issue #328): ルート画面から「このアプリについて」画面を開けること、非公式の注記と
/// データの出典一覧が見えることを確かめる(ADR-0501「P6-18」4章・5章)。文言の数値(具体的な
/// 日本語の全文)は検査しない(P6-14/P6-17 と同じ粒度。`AboutTextTests` が文言そのものを固定する)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ。ADR-0500 §5・§7)。
/// `openAboutScreen`/`aboutScreen`/`aboutUnofficialNotice`/`aboutDataSource-<index>` は spec 時点では
/// 未実装のため、このテストは失敗してよい(「この時点では失敗してよい」)。
@MainActor
final class AboutScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launchWithMock() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func openAboutScreen(_ app: XCUIApplication) {
        let openButton = element(app, "openAboutScreen")
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout), "ルート画面に openAboutScreen が無い")
        openButton.tap()
        XCTAssertTrue(element(app, "aboutScreen").waitForExistence(timeout: Self.existenceTimeout), "aboutScreen が開かない")
    }

    /// ルート画面から About 画面を開ける(ADR-0501「P6-18」4章の3)。
    func testOpenAboutScreenFromRoot() {
        let app = launchWithMock()
        openAboutScreen(app)
    }

    /// 非公式の注記と、4件のデータ出典がすべて見える(ADR-0501「P6-18」4章の4)。
    func testAboutScreenShowsNoticeAndAllDataSources() {
        let app = launchWithMock()
        openAboutScreen(app)

        XCTAssertTrue(
            element(app, "aboutUnofficialNotice").waitForExistence(timeout: Self.existenceTimeout),
            "非公式の注記(aboutUnofficialNotice)が見えない"
        )

        for index in 0..<4 {
            let identifier = "aboutDataSource-\(index)"
            XCTAssertTrue(
                element(app, identifier).waitForExistence(timeout: Self.existenceTimeout),
                "データの出典 \(identifier) が見えない"
            )
        }
    }
}
