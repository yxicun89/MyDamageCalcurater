import XCTest

/// P6-20: 構築のテキスト書き出し・取り込み(ADR-0501「P6-20」)。`POKECALC_USE_MOCK=1` のモックで通す。
/// 書式は日本語名の Showdown 風(実 Showdown とは互換にしない。ADR-0502)。名前はモックの架空データ
/// (`Resources/*.json` の 9001〜9004)。文言は ADR-0501「P6-20」3章の表をそのまま書く
/// (UI テストは App のターゲットに依存しない)。実装前は identifier が無いので失敗してよい。
/// 注意: `Menu` の中の identifier は UIKit に渡らないので、入口・操作はすべて `Menu` の外の通常のボタンに置く。
@MainActor
final class TeamTextTransferUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let firstSpeciesName = "テストモンいち"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elements(_ app: XCUIApplication, beginningWith prefix: String) -> XCUIElementQuery {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix))
    }

    /// 構築を新規作成して編集画面を開き、先頭の種族のメンバーを1体追加する。
    private func launchEditorWithOneMember() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        app.buttons["openTeamListScreen"].tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        element(app, "createTeamButton").tap()
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: Self.existenceTimeout))
        nameField.tap()
        nameField.typeText("テキスト取り込みUI")
        app.alerts.buttons["作成"].tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        element(app, "addMemberButton").tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        app.buttons[Self.firstSpeciesName].tap()
        XCTAssertTrue(elements(app, beginningWith: "memberCard-").firstMatch.waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func openSheet(_ app: XCUIApplication) {
        let open = element(app, "teamTextTransferButton")
        XCTAssertTrue(open.waitForExistence(timeout: Self.existenceTimeout))
        open.tap()
        XCTAssertTrue(element(app, "teamTextSheet").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func typeAndAnalyze(_ app: XCUIApplication, _ text: String) {
        let editor = element(app, "importTextEditor")
        XCTAssertTrue(editor.waitForExistence(timeout: Self.existenceTimeout))
        editor.tap()
        editor.typeText(text)
        element(app, "analyzeImportTextButton").tap()
    }

    private func memberCardCount(_ app: XCUIApplication) -> Int {
        elements(app, beginningWith: "memberCard-").count
    }

    // MARK: 書き出し

    func testExportOneMemberShowsTextAndCopyAndShare() {
        let app = launchEditorWithOneMember()
        let exportButton = elements(app, beginningWith: "exportMemberTextButton-").firstMatch
        XCTAssertTrue(exportButton.waitForExistence(timeout: Self.existenceTimeout))
        exportButton.tap()
        XCTAssertTrue(element(app, "teamTextSheet").waitForExistence(timeout: Self.existenceTimeout))

        let text = element(app, "exportedText")
        XCTAssertTrue(text.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(text.label.hasPrefix(Self.firstSpeciesName), "先頭行は種族の日本語名: \(text.label)")
        XCTAssertTrue(text.label.contains("Nature: テストせいかく"), "性格の行がある: \(text.label)")

        XCTAssertTrue(element(app, "copyExportedTextButton").exists)
        XCTAssertTrue(element(app, "shareExportedTextLink").exists)
        element(app, "copyExportedTextButton").tap()
        XCTAssertEqual(element(app, "copiedNotice").label, "コピーしました。")
    }

    func testExportWholeTeamFromTheSheet() {
        let app = launchEditorWithOneMember()
        openSheet(app)
        element(app, "exportTeamTextButton").tap()
        let text = element(app, "exportedText")
        XCTAssertTrue(text.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(text.label.hasPrefix(Self.firstSpeciesName))
    }

    // MARK: 取り込み

    func testImportWithRejectedLineLetsTheUserAddOnlyTheValidMember() {
        let app = launchEditorWithOneMember()
        let before = memberCardCount(app)
        openSheet(app)
        typeAndAnalyze(app, "テストモンさん\nEVs: 252 SpA\n- テストわざとくしゅA")

        XCTAssertTrue(element(app, "importRejectedList").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "importRejectedList").label.contains("取り込めなかった行"), true)
        let line = element(app, "importRejectedLine-2")
        XCTAssertTrue(line.exists, "行番号つきで出る")
        XCTAssertTrue(line.label.contains("2行目"))
        XCTAssertTrue(line.label.contains("EVs: 252 SpA"))
        XCTAssertTrue(line.label.contains("努力値(EVs)・個体値(IVs)の形式には対応していません"))

        let confirm = element(app, "confirmImportValidButton")
        XCTAssertTrue(confirm.exists)
        XCTAssertEqual(confirm.label, "取り込める1体だけ追加")
        XCTAssertTrue(element(app, "cancelImportButton").exists, "全か無かにしない: やめる選択肢もある")
        XCTAssertEqual(memberCardCount(app), before, "確定するまで追加しない")
        confirm.tap()

        XCTAssertTrue(element(app, "importedNotice").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "importedNotice").label, "1体を追加しました。保存すると反映されます。")
        XCTAssertEqual(memberCardCount(app), before + 1)
    }

    func testImportWithoutRejectedLinesNeedsNoDecision() {
        let app = launchEditorWithOneMember()
        let before = memberCardCount(app)
        openSheet(app)
        typeAndAnalyze(app, "テストモンさん\nNature: テストせいかく特攻上昇")
        XCTAssertFalse(element(app, "importRejectedList").exists)
        let confirm = element(app, "confirmImportValidButton")
        XCTAssertTrue(confirm.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(confirm.label, "1体を追加")
        confirm.tap()
        XCTAssertTrue(element(app, "importedNotice").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(memberCardCount(app), before + 1)
    }

    func testNothingImportableShowsTheListAndNoConfirm() {
        let app = launchEditorWithOneMember()
        let before = memberCardCount(app)
        openSheet(app)
        typeAndAnalyze(app, "そんなポケ")
        XCTAssertTrue(element(app, "importRejectedLine-1").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "importRejectedLine-1").label.contains("ポケモンが見つかりません"))
        XCTAssertEqual(element(app, "importNothingNotice").label, "取り込めるポケモンがありません。")
        XCTAssertFalse(element(app, "confirmImportValidButton").exists)
        element(app, "closeTeamTextSheetButton").tap()
        XCTAssertEqual(memberCardCount(app), before, "何も追加されない")
    }

    func testCancelAfterReviewAddsNothing() {
        let app = launchEditorWithOneMember()
        let before = memberCardCount(app)
        openSheet(app)
        typeAndAnalyze(app, "テストモンさん\nEVs: 252 SpA")
        XCTAssertTrue(element(app, "cancelImportButton").waitForExistence(timeout: Self.existenceTimeout))
        element(app, "cancelImportButton").tap()
        XCTAssertFalse(element(app, "importRejectedList").exists)
        XCTAssertFalse(element(app, "confirmImportValidButton").exists)
        element(app, "closeTeamTextSheetButton").tap()
        XCTAssertEqual(memberCardCount(app), before)
    }

    func testEmptyInputShowsAGuideInsteadOfAnalyzing() {
        let app = launchEditorWithOneMember()
        openSheet(app)
        let analyze = element(app, "analyzeImportTextButton")
        XCTAssertTrue(analyze.waitForExistence(timeout: Self.existenceTimeout))
        analyze.tap()
        XCTAssertEqual(element(app, "importEmptyNotice").label, "テキストを貼り付けてください。")
        XCTAssertFalse(element(app, "importRejectedList").exists)
    }
}
