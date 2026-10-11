import XCTest

/// 構築(P6-2c。F-08 で作り直し: ADR-0522)。`POKECALC_USE_MOCK=1` で起動してモックを強制する
/// (ADR-0501「P6-2c」5章)。構築・メンバーの id は実行時に生成される UUID なので `BEGINSWITH` の述語で辿る。
/// 数値の正しさではなく操作がつながることだけを見る(CalcScreenUITests / ReverseScreenUITests と同じ粒度)。
/// 文言は Web と同じ語(「新しい構築」「N体目」「一覧に戻る」「保存していない変更があります」「構築 N」)。
@MainActor
final class TeamScreenUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// `Resources/species.json` の先頭(モックの種族一覧の並びはフィクスチャの並び順のまま)。
    private static let firstMockSpeciesName = "テストモンいち"
    private static let secondMockSpeciesName = "テストモンに"
    /// `Resources/species.json` の3番目(種族検索シートで打ってから選ぶ確認用)。
    private static let thirdMockSpeciesName = "テストモンさん"
    private static let thirdMockSpeciesKey = "9003-000"
    /// 3番目の種族の learnset にある技(技検索シートで打ってから技スロットに入れる確認用)。
    private static let searchableMoveName = "テストわざとくしゅB"
    private static let searchableMoveID = "test-move-special-b"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別を決め打ちせず、identifier だけで要素を探す(他画面の XCUITest と同じ)。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elements(_ app: XCUIApplication, beginningWith prefix: String) -> XCUIElementQuery {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix))
    }

    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        elements(app, beginningWith: prefix).firstMatch
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

    /// [新しい構築]を押して、できた構築の編集画面を開く。
    private func createTeam(_ app: XCUIApplication) {
        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    private func launchEditor() -> XCUIApplication {
        let app = launchTeamList()
        createTeam(app)
        return app
    }

    /// 空の枠 `slot`(1 始まり)の種族を、検索シートで `name` に決める。
    private func pickSpecies(_ app: XCUIApplication, slot: Int, name: String) {
        let picker = element(app, "slotSpeciesPicker-\(slot)")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout), "\(slot)体目の「ポケモン」欄")
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[name]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()
        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
    }

    private func save(_ app: XCUIApplication) {
        element(app, "saveTeamButton").tap()
        XCTAssertTrue(element(app, "teamSavedNotice").waitForExistence(timeout: Self.existenceTimeout), "保存しました")
    }

    // MARK: 6 つの枠

    /// [新しい構築]で名前の入力なしに構築ができ、すぐ編集画面が開く。6 つの枠が最初から並び、空の枠は
    /// 「ポケモン」の欄と案内だけ(技・持ち物などは出ない)。
    func testNewTeamOpensEditorWithSixEmptySlots() {
        let app = launchTeamList()
        XCTAssertTrue(element(app, "teamListEmpty").waitForExistence(timeout: Self.existenceTimeout), "開いた直後は0件")
        XCTAssertTrue(element(app, "teamListEmpty").label.contains("新しい構築"), "次にすることを書いた案内")
        XCTAssertFalse(element(app, "teamNameField").exists, "構築名の入力欄は無い")
        createTeam(app)

        XCTAssertFalse(element(app, "teamNameField").exists, "編集画面にも構築名の入力欄は無い")
        for number in 1...6 {
            XCTAssertTrue(element(app, "teamSlot-\(number)").exists, "\(number)体目の枠")
            XCTAssertTrue(element(app, "slotSpeciesPicker-\(number)").exists, "\(number)体目の「ポケモン」欄")
            XCTAssertEqual(
                element(app, "slotEmptyHint-\(number)").label, "ポケモンを選ぶと、技・持ち物・特性などを決められます")
        }
        XCTAssertFalse(elementBeginningWith(app, "memberCard-").exists, "空の枠に技・持ち物などは出ない")
        XCTAssertFalse(elementBeginningWith(app, "memberMoveSlot-").exists)
        XCTAssertFalse(element(app, "slotMoveUp-1").exists, "空の枠に入れ替え・外すは出ない")
        XCTAssertFalse(element(app, "teamUnsavedNotice").exists, "開いた直後は未保存の印が出ない")
        XCTAssertFalse(element(app, "addMemberButton").exists, "[メンバーを追加]は無い")
    }

    /// 種族を選ぶとその枠だけが展開し、技(4つ)・持ち物・特性・性格・SP の入力欄が出る。
    func testChoosingSpeciesShowsTheInputsForThatSlotOnly() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)

        XCTAssertTrue(elementBeginningWith(app, "memberCard-").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(elements(app, beginningWith: "memberMoveSlot-").count, 4, "技は4つ")
        XCTAssertTrue(elementBeginningWith(app, "memberItemPicker-").exists)
        XCTAssertTrue(elementBeginningWith(app, "memberAbilityPicker-").exists)
        XCTAssertTrue(elementBeginningWith(app, "memberNaturePicker-").exists)
        XCTAssertTrue(elementBeginningWith(app, "memberSP-").exists)
        XCTAssertTrue(element(app, "slotMoveDown-1").exists)
        XCTAssertTrue(element(app, "slotRemove-1").exists)
        XCTAssertTrue(element(app, "slotSpeciesPicker-2").exists, "他の枠は空のまま")
        XCTAssertTrue(element(app, "teamUnsavedNotice").exists, "変更したので未保存の印が出る")
        XCTAssertEqual(element(app, "teamUnsavedNotice").label, "保存していない変更があります")
    }

    /// 3番目の種族を検索シートで打って選び、技スロットも検索シートで埋める(issue #68 の流れ)。
    func testSpeciesAndMoveSlotSearchSheetsFilterAndSelect() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)

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

    // MARK: 上へ・下へ・外す

    /// [1体目を下へ]で 2 体目と入れ替わる。1体目の上・6体目の下は無効。
    func testMoveDownAndUpSwapsNeighbouringSlots() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        pickSpecies(app, slot: 2, name: Self.secondMockSpeciesName)

        let pickers = elements(app, beginningWith: "memberSpeciesPicker-")
        XCTAssertEqual(pickers.count, 2)
        func order() -> [String] {
            let sorted = (0..<pickers.count).map { pickers.element(boundBy: $0) }.sorted { $0.frame.minY < $1.frame.minY }
            return sorted.map(\.label)
        }
        XCTAssertEqual(order(), [Self.firstMockSpeciesName, Self.secondMockSpeciesName])
        XCTAssertFalse(element(app, "slotMoveUp-1").isEnabled, "1体目は上へ動かせない")

        element(app, "slotMoveDown-1").tap()
        XCTAssertEqual(order(), [Self.secondMockSpeciesName, Self.firstMockSpeciesName], "入れ替わる")

        element(app, "slotMoveUp-2").tap()
        XCTAssertEqual(order(), [Self.firstMockSpeciesName, Self.secondMockSpeciesName], "戻る")
    }

    /// [N体目を外す]でその枠だけが空に戻り、他の枠は動かない。
    func testRemovingASlotEmptiesOnlyThatSlot() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        pickSpecies(app, slot: 2, name: Self.secondMockSpeciesName)

        element(app, "slotRemove-1").tap()
        XCTAssertTrue(element(app, "slotSpeciesPicker-1").waitForExistence(timeout: Self.existenceTimeout), "1体目が空に戻る")
        XCTAssertEqual(elements(app, beginningWith: "memberCard-").count, 1)
        XCTAssertEqual(elementBeginningWith(app, "memberSpeciesPicker-").label, Self.secondMockSpeciesName, "2体目は動かない")
        XCTAssertFalse(element(app, "slotSpeciesPicker-2").exists, "2体目は埋まったまま")
    }

    // MARK: 保存・一覧に戻る

    /// 保存は明示。保存すると未保存の印が「保存しました」に変わり、編集画面に残る。
    func testExplicitSaveClearsTheUnsavedNotice() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        XCTAssertTrue(element(app, "teamUnsavedNotice").exists)
        save(app)
        XCTAssertFalse(element(app, "teamUnsavedNotice").exists)
        XCTAssertTrue(element(app, "teamEditScreen").exists, "保存しても編集画面に残る")
    }

    /// 未保存で[一覧に戻る]を押すと 2 段階: 確認が出て、[編集を続ける]で閉じ、[保存せずに戻る]で戻る。
    func testLeavingWithUnsavedChangesAsksFirst() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)

        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamLeaveConfirmNotice").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(element(app, "teamLeaveConfirmNotice").label, "保存していない変更があります。保存せずに一覧に戻りますか")
        XCTAssertTrue(element(app, "teamEditScreen").exists, "まだ戻っていない")
        XCTAssertEqual(element(app, "teamLeaveDiscardButton").label, "保存せずに戻る")

        element(app, "teamLeaveCancelButton").tap()
        XCTAssertFalse(element(app, "teamLeaveConfirmNotice").exists, "編集を続ける")
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").exists, "下書きは残っている")

        element(app, "backToListButton").tap()
        element(app, "teamLeaveDiscardButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        let count = elementBeginningWith(app, "teamCount-")
        XCTAssertTrue(count.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(count.label, "0/6体", "保存していない変更は捨てられた")
    }

    /// 左端から右へのスワイプ(システムの戻り)で未保存の編集が確認なしに消えない。
    func testEdgeSwipeBackDoesNotDiscardUnsavedChanges() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        let window = app.windows.firstMatch
        let start = window.coordinate(withNormalizedOffset: CGVector(dx: 0.0, dy: 0.5))
        let end = window.coordinate(withNormalizedOffset: CGVector(dx: 0.8, dy: 0.5))
        start.press(forDuration: 0.1, thenDragTo: end)
        XCTAssertTrue(element(app, "teamEditScreen").exists, "編集画面に残る")
        XCTAssertFalse(element(app, "teamListScreen").exists)
        XCTAssertTrue(element(app, "teamUnsavedNotice").exists, "未保存の印が残る")
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").exists)
    }

    /// 未保存の変更が無ければ、確認なしで一覧に戻る。
    func testLeavingWithoutChangesGoesStraightBack() {
        let app = launchEditor()
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "teamLeaveConfirmNotice").exists)
    }

    // MARK: 一覧のカード

    /// 保存した構築が一覧のカードに出る: 表示名「構築 N」・アイコン・n/6体・最終更新・[開く]。開き直すと枠が埋まっている。
    func testSavedTeamAppearsAsACardAndReopens() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstMockSpeciesName)
        pickSpecies(app, slot: 2, name: Self.secondMockSpeciesName)
        save(app)
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        let name = elementBeginningWith(app, "teamName-")
        XCTAssertTrue(name.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(name.label, "構築 1")
        XCTAssertFalse(app.staticTexts["名称未設定"].exists, "既定名の文字は画面に出さない")
        XCTAssertEqual(elementBeginningWith(app, "teamCount-").label, "2/6体")
        XCTAssertTrue(elementBeginningWith(app, "teamUpdated-").label.hasPrefix("最終更新: "))
        XCTAssertTrue(elementBeginningWith(app, "teamIcons-").exists, "6体までのアイコン列")
        XCTAssertTrue(elementBeginningWith(app, "teamCard-").exists)

        elementBeginningWith(app, "teamOpen-").tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(elements(app, beginningWith: "memberCard-").count, 2, "保存した2体が1・2体目に入っている")
        XCTAssertTrue(element(app, "slotSpeciesPicker-3").exists, "残りは空の枠")
        XCTAssertFalse(element(app, "teamUnsavedNotice").exists, "開いた直後は未保存の印が出ない")
    }

    /// 構築の表示名は作成の古い順に「構築 1」「構築 2」。
    func testTeamsAreNumberedInCreationOrder() {
        let app = launchEditor()
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        createTeam(app)
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        let names = elements(app, beginningWith: "teamName-")
        XCTAssertTrue(names.firstMatch.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(
            Set((0..<names.count).map { names.element(boundBy: $0).label }), ["構築 1", "構築 2"])
    }

    /// 削除は 2 段階: 押すと確認が出て、[やめる]で残り、[削除する]で消えて空の案内に戻る。
    func testDeleteAsksForConfirmationFirst() {
        let app = launchEditor()
        element(app, "backToListButton").tap()
        let deleteButton = elementBeginningWith(app, "teamDelete-")
        XCTAssertTrue(deleteButton.waitForExistence(timeout: Self.existenceTimeout))
        deleteButton.tap()
        XCTAssertTrue(elementBeginningWith(app, "teamDeleteConfirmNotice-").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(elementBeginningWith(app, "teamCard-").exists, "確認の間は消えない")

        elementBeginningWith(app, "teamDeleteCancel-").tap()
        XCTAssertFalse(elementBeginningWith(app, "teamDeleteConfirmNotice-").exists)
        XCTAssertTrue(elementBeginningWith(app, "teamCard-").exists, "やめると残る")

        elementBeginningWith(app, "teamDelete-").tap()
        let confirm = elementBeginningWith(app, "teamDeleteConfirm-")
        XCTAssertTrue(confirm.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(confirm.label, "削除する")
        confirm.tap()
        XCTAssertTrue(element(app, "teamListEmpty").waitForExistence(timeout: Self.existenceTimeout), "削除すると0件に戻る")
    }
}
