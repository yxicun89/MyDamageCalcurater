import XCTest

/// 判定の入口は既定で非表示(F-07。ユーザー決定 2026-10-04: 判定は目的を作り直す)。環境変数 `POKECALC_SHOW_JUDGE=1` で出る。
/// サービス・画面のコード・テストは残してある(判定画面のテストは `POKECALC_SHOW_JUDGE=1` を渡して動かす)。
@MainActor
final class JudgeHiddenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launchRoot(showJudge: Bool) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if showJudge { app.launchEnvironment["POKECALC_SHOW_JUDGE"] = "1" }
        app.launch()
        return app
    }

    /// 既定では、ほかの入口は出るが、判定の入口は出ない。
    func testJudgeEntryIsHiddenByDefault() {
        let app = launchRoot(showJudge: false)
        XCTAssertTrue(app.buttons["openCalcScreen"].waitForExistence(timeout: Self.existenceTimeout), "計算の入口は出る")
        XCTAssertTrue(app.buttons["openSpeedScreen"].exists, "素早さの入口は出る")
        XCTAssertFalse(app.buttons["openJudgeScreen"].exists, "判定の入口は既定で出さない")
    }

    /// 環境変数を渡すと、判定の入口が出る(再設計のときに戻せる)。
    func testJudgeEntryIsShownWithTheEnvironment() {
        let app = launchRoot(showJudge: true)
        XCTAssertTrue(app.buttons["openCalcScreen"].waitForExistence(timeout: Self.existenceTimeout))
        let judge = app.buttons["openJudgeScreen"]
        // ルートはスクロールすることがあるので、押せる位置まで探す。
        var attempts = 0
        while !judge.exists && attempts < 6 {
            app.swipeUp()
            attempts += 1
        }
        XCTAssertTrue(judge.exists, "POKECALC_SHOW_JUDGE=1 では判定の入口が出る")
    }
}
