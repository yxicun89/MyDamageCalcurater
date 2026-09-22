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

        // メンバーを1体追加する(`addMemberButton` → 種族メニューの先頭を選ぶ)。
        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        let firstSpeciesOption = app.buttons[Self.firstMockSpeciesName]
        XCTAssertTrue(firstSpeciesOption.waitForExistence(timeout: Self.existenceTimeout))
        firstSpeciesOption.tap()

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
}
