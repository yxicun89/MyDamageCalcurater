import XCTest

/// P6-17(ADR-0501「P6-17」5章): 未対応の印(ADR-0123)の表示を確かめる。`POKECALC_USE_MOCK=1` で起動する。
///
/// モックは印をフィクスチャから決める(ADR-0501「P6-17」4章): `test-move-multi-hit`(多段技)を選ぶと全行に技の印、
/// `test-item-unsupported` を比較すると、その持ち物の行だけに防御側の持ち物の印が付く。起動のための切り替え
/// (環境変数)は増やさない。文言の数値は検査しない(ADR-0501「XCUITest で確かめること」)。
@MainActor
final class UnsupportedMarksUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// `Resources/moves.json` の多段技(`mechanisms: ["multi_hit"]`。先頭の種族の learnset にある)。
    private static let multiHitMoveName = "テストわざれんぞく"
    private static let multiHitMoveID = "test-move-multi-hit"
    /// `Resources/items.json` の `unsupportedEffect: true` の持ち物。
    private static let unsupportedItemName = "テストどうぐみたいおう"
    private static let unsupportedItemID = "test-item-unsupported"
    /// モックの物理技の既定5行(`CalcScreenUITests.defaultPhysicalRowIDs` と同じ)。
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func launch(openButton identifier: String, screen: String) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons[identifier]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, screen).waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 技の検索シート(`movePicker` / `reverseMovePicker` から開く共通のシート)で多段技を選ぶ。
    private func selectMultiHitMove(_ app: XCUIApplication, pickerIdentifier: String) {
        let picker = element(app, pickerIdentifier)
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        picker.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(Self.multiHitMoveName)
        let result = element(app, "moveSearchResult-\(Self.multiHitMoveID)")
        XCTAssertTrue(result.waitForExistence(timeout: Self.existenceTimeout))
        result.tap()
    }

    // MARK: - 計算画面

    /// 既定の技・持ち物では印が無く、注記は出ない(既存の画面のまま)。
    func testCalcScreenShowsNoNoticeByDefault() {
        let app = launch(openButton: "openCalcScreen", screen: "calcScreen")
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout))
        }
        XCTAssertFalse(element(app, "calcUnsupportedNotice").exists)
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertFalse(element(app, "calcResultUnsupported-\(rowID)").exists, rowID)
        }
    }

    /// 多段技を選ぶと、全行共通の技の印が結果の上に1回だけ出る(行には重ねない)。文言は技の名前(マスタ)と理由を含む。
    func testCalcScreenMultiHitMoveShowsNoticeOnceAboveResults() {
        let app = launch(openButton: "openCalcScreen", screen: "calcScreen")
        selectMultiHitMove(app, pickerIdentifier: "movePicker")

        let notice = element(app, "calcUnsupportedNotice")
        XCTAssertTrue(notice.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(notice.label.contains(Self.multiHitMoveName), notice.label)
        XCTAssertTrue(notice.label.contains("多段技"), notice.label)
        XCTAssertTrue(notice.label.contains("正確でない可能性"), notice.label)

        let firstRow = element(app, "calcResultRow-\(Self.defaultPhysicalRowIDs[0])")
        XCTAssertTrue(firstRow.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertLessThan(notice.frame.minY, firstRow.frame.minY, "注記は結果の行より上")
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertFalse(element(app, "calcResultUnsupported-\(rowID)").exists, "共通の印を行に重ねない: \(rowID)")
        }
    }

    /// 未対応の持ち物を比較すると、その持ち物の行だけに注記が出る(持ち物なしの行・結果の上には出ない)。
    func testCalcScreenUnsupportedItemComparisonShowsPerRowNote() {
        let app = launch(openButton: "openCalcScreen", screen: "calcScreen")
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout))
        }
        let toggle = element(app, "defenderItemToggle-\(Self.unsupportedItemID)")
        XCTAssertTrue(toggle.waitForExistence(timeout: Self.existenceTimeout))
        toggle.tap()

        let itemRowID = "none@\(Self.unsupportedItemID)"
        XCTAssertTrue(element(app, "calcResultRow-\(itemRowID)").waitForExistence(timeout: Self.existenceTimeout))
        let note = element(app, "calcResultUnsupported-\(itemRowID)")
        XCTAssertTrue(note.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(note.label.contains(Self.unsupportedItemName), note.label)
        XCTAssertTrue(note.label.contains("未対応"), note.label)
        XCTAssertFalse(element(app, "calcResultUnsupported-none@-").exists, "持ち物なしの行には出ない")
        XCTAssertFalse(element(app, "calcUnsupportedNotice").exists, "一部の行だけの印は結果の上に出さない")
    }

    // MARK: - 逆算画面

    /// 多段技で逆算すると、全候補共通の技の印が候補の上に1回だけ出る(候補カードには重ねない)。
    func testReverseScreenMultiHitMoveShowsNoticeAboveCandidates() {
        let app = launch(openButton: "openReverseScreen", screen: "reverseScreen")
        selectMultiHitMove(app, pickerIdentifier: "reverseMovePicker")

        let field = element(app, "reverseObservationField-0")
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText("12")

        let firstCandidate = element(app, "reverseCandidateRow-neutral@-")
        XCTAssertTrue(firstCandidate.waitForExistence(timeout: Self.existenceTimeout))
        let notice = element(app, "reverseUnsupportedNotice")
        XCTAssertTrue(notice.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(notice.label.contains(Self.multiHitMoveName), notice.label)
        XCTAssertTrue(notice.label.contains("多段技"), notice.label)
        XCTAssertLessThan(notice.frame.minY, firstCandidate.frame.minY, "注記は候補より上")
        XCTAssertFalse(element(app, "reverseCandidateUnsupported-neutral@-").exists, "共通の印を候補に重ねない")
    }
}
