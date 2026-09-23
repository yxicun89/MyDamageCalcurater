import XCTest

/// 構築ビルダー(P6-2c)の骨組みを確かめる。`POKECALC_USE_MOCK=1` で起動してモックを強制する
/// (ADR-0501「P6-2c」5章)。一覧・編集の識別子は ADR の契約どおりだが、構築・メンバーの id は
/// 実行時に生成される UUID なので `BEGINSWITH` の述語で辿る。数値の正しさではなく操作が
/// つながることだけを見る(CalcScreenUITests / ReverseScreenUITests と同じ粒度)。
@MainActor
final class TeamScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// `Resources/species.json` の先頭(モックの種族一覧の並びはフィクスチャの並び順のまま。
    /// `MockPokeCalcService.matchingByPrefix` は空クエリなら並べ替えない)。
    private static let firstMockSpeciesName = "テストモンいち"
    /// `Resources/species.json` の3番目(issue #68: 種族検索シートで打ってから選ぶ確認用。
    /// メンバー追加時の既定=先頭とは別の種族にして、変更したことを検査できるようにする)。
    private static let thirdMockSpeciesName = "テストモンさん"
    private static let thirdMockSpeciesKey = "9003-000"
    /// `Resources/moves.json` のうち、3番目の種族(`thirdMockSpeciesKey`)の learnset にある技
    /// (issue #68: 技検索シートで打ってから技スロットに入れる確認用)。
    private static let searchableMoveName = "テストわざとくしゅB"
    private static let searchableMoveID = "test-move-special-b"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別を決め打ちせず、identifier だけで要素を探す(他画面の XCUITest と同じ)。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// id を含む動的な identifier(`teamRow-<id>` 等)を前方一致で探す。このテストは構築を1つしか
    /// 作らないので、前方一致でも一意に定まる。
    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    private func launchTeamList() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openTeamListScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 「一覧を開く→新規作成→名前を付けて保存→一覧に出る→開いてメンバーを1体追加して保存→
    /// 一覧から削除できる」の一連(ADR-0501「P6-2c」5章)。`RootView.makeTeamStore()` が
    /// `POKECALC_USE_MOCK=1` のとき起動のたびに専用 UserDefaults suite を空にするため、
    /// 開いた直後は必ず0件から始まる。
    func testCreateAddMemberSaveAndDeleteTeamFlow() {
        let app = launchTeamList()
        XCTAssertTrue(element(app, "teamListEmpty").waitForExistence(timeout: Self.existenceTimeout), "開いた直後は0件")

        // 新規作成: `createTeamButton` → アラートに名前を入れて「作成」。
        let teamName = "テストパーティUI"
        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()

        // アラートの `TextField` は UIKit の `UIAlertController` が作る `UITextField` に写像され、
        // SwiftUI 側の `.accessibilityIdentifier` が橋渡しされない(実装時に確認した制約。
        // ADR-0501「P6-2c」「### 7 確認事項」)。アラートに入力欄は1つしか無いので `firstMatch` で辿る。
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: Self.existenceTimeout))
        nameField.tap()
        nameField.typeText(teamName)
        app.alerts.buttons["作成"].tap()

        // 保存されて編集画面へ遷移する。
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(elementBeginningWith(app, "memberCard-").exists, "追加する前はメンバーが無い")

        // メンバーを1体追加する(`addMemberButton` → issue #68: 検索シートで種族を選ぶ)。
        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let firstSpeciesOption = app.buttons[Self.firstMockSpeciesName]
        XCTAssertTrue(firstSpeciesOption.waitForExistence(timeout: Self.existenceTimeout))
        firstSpeciesOption.tap()
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")

        let memberCard = elementBeginningWith(app, "memberCard-")
        XCTAssertTrue(memberCard.waitForExistence(timeout: Self.existenceTimeout), "追加したメンバーのカードが出る")

        // 保存すると一覧に戻る。
        let saveButton = element(app, "saveTeamButton")
        XCTAssertTrue(saveButton.waitForExistence(timeout: Self.existenceTimeout))
        saveButton.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        let row = elementBeginningWith(app, "teamRow-")
        XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "保存した構築が一覧に出る")
        XCTAssertEqual(row.label, teamName)

        // 削除すると一覧から消え、空の案内に戻る。
        let deleteButton = elementBeginningWith(app, "teamDelete-")
        XCTAssertTrue(deleteButton.waitForExistence(timeout: Self.existenceTimeout))
        deleteButton.tap()
        XCTAssertTrue(element(app, "teamListEmpty").waitForExistence(timeout: Self.existenceTimeout), "削除すると0件に戻る")
    }

    /// issue #68: メンバーの種族セレクタ(`memberSpeciesPicker-<id>`)・技スロット
    /// (`memberMoveSlot-<id>-<index>`)もどちらも検索シート経由(`Menu` ではなくなった)。
    /// 検索欄に打って絞り込み、1件タップするとシートが閉じて選択が反映されることを確かめる。
    func testMemberSpeciesAndMoveSlotSearchSheetsFilterAndSelect() {
        let app = launchTeamList()
        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: Self.existenceTimeout))
        nameField.tap()
        nameField.typeText("テスト検索シートUI")
        app.alerts.buttons["作成"].tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))

        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let firstSpeciesOption = app.buttons[Self.firstMockSpeciesName]
        XCTAssertTrue(firstSpeciesOption.waitForExistence(timeout: Self.existenceTimeout))
        firstSpeciesOption.tap()
        let memberCard = elementBeginningWith(app, "memberCard-")
        XCTAssertTrue(memberCard.waitForExistence(timeout: Self.existenceTimeout))

        // メンバーの種族を検索シートで3番目の種族に変える(打って絞り込んでから選ぶ)。
        let memberSpeciesPicker = elementBeginningWith(app, "memberSpeciesPicker-")
        XCTAssertTrue(memberSpeciesPicker.waitForExistence(timeout: Self.existenceTimeout))
        memberSpeciesPicker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let speciesField = app.searchFields.firstMatch
        XCTAssertTrue(speciesField.waitForExistence(timeout: Self.existenceTimeout))
        speciesField.tap()
        speciesField.typeText(Self.thirdMockSpeciesName)
        let speciesResult = element(app, "speciesSearchResult-\(Self.thirdMockSpeciesKey)")
        XCTAssertTrue(speciesResult.waitForExistence(timeout: Self.existenceTimeout))
        speciesResult.tap()
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
        XCTAssertEqual(memberSpeciesPicker.label, Self.thirdMockSpeciesName, "選択がメンバーのヘッダーに反映される")

        // 技スロット(先頭)を検索シートで埋める(打って絞り込んでから選ぶ)。
        let moveSlot = elementBeginningWith(app, "memberMoveSlot-")
        XCTAssertTrue(moveSlot.waitForExistence(timeout: Self.existenceTimeout))
        moveSlot.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let moveField = app.searchFields.firstMatch
        XCTAssertTrue(moveField.waitForExistence(timeout: Self.existenceTimeout))
        moveField.tap()
        moveField.typeText(Self.searchableMoveName)
        let moveResult = element(app, "moveSearchResult-\(Self.searchableMoveID)")
        XCTAssertTrue(moveResult.waitForExistence(timeout: Self.existenceTimeout))
        moveResult.tap()
        XCTAssertFalse(element(app, "moveSearchSheet").exists, "選ぶとシートが閉じる")
        XCTAssertEqual(moveSlot.label, Self.searchableMoveName, "選択が技スロットに反映される")
    }
}
