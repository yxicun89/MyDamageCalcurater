import XCTest

/// P6-26: タイプバランス画面の XCUITest(`BalanceScreenUITests`・`LargeTextLayoutUITests`)で共有する操作。
/// モックの構築を作る手順(構築ビルダー画面の操作列)と、モックのタイプバランスの固定の事実(ADR-0505 §8)を置く。
@MainActor
enum BalanceUITestSupport {
    static let existenceTimeout: TimeInterval = 5

    /// 構築に入れるメンバー(モックのマスタの種族名と、先頭の技スロットに入れる技 ID。技を入れない = nil)。
    struct Member {
        let speciesName: String
        let moveID: String?
    }

    /// 技を 1 つ持つメンバー(モックの種族「テストモンいち」。学習する技 `test-move-physical-a`)。
    static let armedMember = Member(speciesName: "テストモンいち", moveID: "test-move-physical-a")
    /// 技を持たないメンバー(モックの種族「テストモンに」)。
    static let unarmedMember = Member(speciesName: "テストモンに", moveID: nil)

    static func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    static func launch(scenario: String? = nil, openAtLaunch: Bool = false, extraArguments: [String] = []) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let scenario { app.launchEnvironment["POKECALC_MOCK_BALANCE"] = scenario }
        if openAtLaunch { app.launchEnvironment["POKECALC_OPEN_BALANCE_SCREEN_AT_LAUNCH"] = "1" }
        app.launchArguments += extraArguments
        app.launch()
        return app
    }

    /// ルートの `openBalanceScreen` からタイプバランス画面を開く。
    static func openBalanceScreen(_ app: XCUIApplication) {
        let open = element(app, "openBalanceScreen")
        XCTAssertTrue(open.waitForExistence(timeout: existenceTimeout), "ルート画面に openBalanceScreen が無い")
        open.tap()
        XCTAssertTrue(element(app, "balanceScreen").waitForExistence(timeout: existenceTimeout), "balanceScreen が開かない")
    }

    /// ルート画面から構築ビルダーで構築を 1 つ作り、ルート画面に戻る(`JudgeScreenUITests.createTeamWithOneMember` と同じ操作列)。
    static func createTeam(_ app: XCUIApplication, name: String, members: [Member]) {
        let openTeamList = app.buttons["openTeamListScreen"]
        XCTAssertTrue(openTeamList.waitForExistence(timeout: existenceTimeout), "openTeamListScreen が無い")
        openTeamList.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: existenceTimeout), "構築一覧が開かない")
        element(app, "createTeamButton").tap()
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: existenceTimeout), "名前の入力欄が無い")
        nameField.tap()
        nameField.typeText(name)
        app.alerts.buttons["作成"].tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: existenceTimeout), "編集画面が開かない")
        for (index, member) in members.enumerated() {
            let add = element(app, "addMemberButton")
            XCTAssertTrue(add.waitForExistence(timeout: existenceTimeout), "addMemberButton が無い")
            add.tap()
            XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: existenceTimeout), "種族のシートが開かない")
            let option = app.buttons[member.speciesName]
            XCTAssertTrue(option.waitForExistence(timeout: existenceTimeout), "種族 \(member.speciesName) が無い")
            option.tap()
            if let moveID = member.moveID {
                // 先頭の技スロット(`memberMoveSlot-<memberID>-0`)を index 番目のメンバーぶん選ぶ。
                let slots = app.descendants(matching: .any).matching(
                    NSPredicate(format: "identifier BEGINSWITH %@ AND identifier ENDSWITH %@", "memberMoveSlot-", "-0"))
                let slot = slots.element(boundBy: index)
                XCTAssertTrue(slot.waitForExistence(timeout: existenceTimeout), "技スロットが無い")
                slot.tap()
                XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: existenceTimeout), "技のシートが開かない")
                let row = element(app, "moveSearchResult-\(moveID)")
                XCTAssertTrue(row.waitForExistence(timeout: existenceTimeout), "moveSearchResult-\(moveID) が無い")
                row.tap()
            }
        }
        element(app, "saveTeamButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: existenceTimeout), "保存して一覧に戻れない")
        app.navigationBars.buttons.element(boundBy: 0).tap()
    }
}
