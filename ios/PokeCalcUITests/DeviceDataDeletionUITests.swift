import XCTest

/// P6-7(issue #103): 「この端末のデータを削除」(ADR-0501「P6-7」)。「このアプリについて」画面の
/// 「データの扱い」セクションに置く。モックの挙動は起動時の環境変数 `POKECALC_MOCK_DEVICE_DATA`
/// (`partial` / `fail-once`。`MockDeviceDataService.scenarioEnvironmentKey`)で切り替える。
/// 文言は ADR-0209 §8 の確定文(ADR-0501「P6-7」2章)を直接書く(`DeviceDataText` を共有しない。
/// UI テストは App のターゲットに依存しない)。実装前は identifier が無いので失敗してよい。
@MainActor
final class DeviceDataDeletionUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let completionTimeout: TimeInterval = 10

    private static let confirmMessage = "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。"
    private static let completedText = "削除しました。"
    private static let failureText = "サーバーに届きませんでした。通信を確認してもう一度お試しください。"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func launch(scenario: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_DEVICE_DATA"] = scenario }
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func openAbout(_ app: XCUIApplication) {
        let open = element(app, "openAboutScreen")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "aboutScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func tapDeleteAndConfirm(_ app: XCUIApplication) {
        let button = element(app, "deleteDeviceDataButton")
        XCTAssertTrue(button.waitForExistence(timeout: Self.existenceTimeout))
        button.tap()
        XCTAssertTrue(element(app, Self.confirmMessage).waitForExistence(timeout: Self.existenceTimeout), "確認文が出ない")
        let confirm = element(app, "confirmDeleteDeviceDataButton")
        XCTAssertTrue(confirm.waitForExistence(timeout: Self.existenceTimeout))
        confirm.tap()
    }

    private func waitForStatus(_ app: XCUIApplication, _ text: String) {
        let status = element(app, "deleteDeviceDataStatus")
        XCTAssertTrue(status.waitForExistence(timeout: Self.existenceTimeout), "状態表示が無い")
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label == %@", text), object: status)
        XCTAssertEqual(XCTWaiter().wait(for: [expectation], timeout: Self.completionTimeout), .completed, "状態が「\(text)」にならない(実際: \(status.label))")
    }

    /// 説明3文とボタンが見える(ADR-0501「P6-7」5章の AC 6)。
    func testSectionShowsExplanationAndDeleteButton() {
        let app = launch()
        openAbout(app)
        XCTAssertTrue(element(app, "deviceDataSection").waitForExistence(timeout: Self.existenceTimeout))
        for index in 0..<3 {
            XCTAssertTrue(element(app, "deviceDataExplanation-\(index)").exists, "説明 \(index) が無い")
        }
        XCTAssertTrue(element(app, "deleteDeviceDataButton").exists)
        XCTAssertFalse(element(app, "deleteDeviceDataStatus").exists, "操作前に状態表示は出さない")
    }

    /// 確認で取り消すと何も起きない(AC 7)。
    func testCancellingConfirmationLeavesNothingDeleted() {
        let app = launch()
        openAbout(app)
        let button = element(app, "deleteDeviceDataButton")
        XCTAssertTrue(button.waitForExistence(timeout: Self.existenceTimeout))
        button.tap()
        let cancel = element(app, "cancelDeleteDeviceDataButton")
        XCTAssertTrue(cancel.waitForExistence(timeout: Self.existenceTimeout))
        cancel.tap()
        XCTAssertFalse(element(app, "deleteDeviceDataStatus").exists)
        XCTAssertTrue(element(app, "deleteDeviceDataButton").exists)
    }

    /// partial が続いても自動で繰り返し、record と team の両方が completed になって「削除しました。」(AC 8)。
    func testPartialThenCompletedShowsCompleted() {
        let app = launch(scenario: "partial")
        openAbout(app)
        tapDeleteAndConfirm(app)
        waitForStatus(app, Self.completedText)
        XCTAssertFalse(element(app, "retryDeleteDeviceDataButton").exists, "完了後に再試行ボタンは出さない")
    }

    /// 失敗の文言と再試行ボタンが出て、再試行で完了する(AC 9)。
    func testFailureThenRetryCompletes() {
        let app = launch(scenario: "fail-once")
        openAbout(app)
        tapDeleteAndConfirm(app)
        let retry = element(app, "retryDeleteDeviceDataButton")
        XCTAssertTrue(retry.waitForExistence(timeout: Self.completionTimeout), "失敗後に再試行ボタンが出ない")
        let status = element(app, "deleteDeviceDataStatus")
        XCTAssertTrue(status.label.contains(Self.failureText), "失敗文言が無い: \(status.label)")
        XCTAssertFalse(status.label.contains(Self.completedText), "片方の失敗で完了を出さない")
        retry.tap()
        waitForStatus(app, Self.completedText)
    }

    /// 削除が失敗しても計算画面は動く(絶対ルール5。AC 10)。
    func testCalcScreenStillWorksAfterDeletionFailure() {
        let app = launch(scenario: "fail-once")
        openAbout(app)
        tapDeleteAndConfirm(app)
        XCTAssertTrue(element(app, "retryDeleteDeviceDataButton").waitForExistence(timeout: Self.completionTimeout))
        app.navigationBars.buttons.element(boundBy: 0).tap()
        let openCalc = app.buttons["openCalcScreen"]
        XCTAssertTrue(openCalc.waitForExistence(timeout: Self.existenceTimeout))
        openCalc.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        let rows = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "calcResultRow-"))
        XCTAssertTrue(rows.firstMatch.waitForExistence(timeout: Self.existenceTimeout), "計算結果の行が出ない")
    }
}
