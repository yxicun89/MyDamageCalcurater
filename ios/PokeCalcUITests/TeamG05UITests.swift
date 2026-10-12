import XCTest

/// G-05 第 2 弾(ADR-0537。ADR-0501「G-05 構築の作り直し」章): 構築の一覧・編集を、文で説明せず、アイコン・短い名前・タイルで
/// 分からせる作りに直した結果を、画面の要素の有無とタップの大きさ(36pt 以上)・AX5 のはみ出しで確かめる。文面そのものは
/// `G05TeamTextTests` が固定する。既存の `TeamScreenUITests` は挙動と識別子・読み上げ名を守る(ここは新しい構造だけ)。
/// 新しい識別子は実装前は無いので失敗してよい(新しい識別子は ADR の表)。モックの切り替えは `TeamScreenUITests` と同じ。
@MainActor
final class TeamG05UITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    private static let minTapSize: CGFloat = 36
    private static let overflowTolerance: CGFloat = 1
    private static let firstSpecies = "テストモンいち"
    private static let secondSpecies = "テストモンに"
    /// `Resources/species.json` の 1 番目の learnset にある技(タイプが違う 4 つ)。
    private static let moveA = "test-move-physical-a"  // かくとう
    private static let moveB = "test-move-special-a"  // どく

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String, endingWith suffix: String? = nil) -> XCUIElement {
        let format = suffix == nil ? "identifier BEGINSWITH %@" : "identifier BEGINSWITH %@ AND identifier ENDSWITH %@"
        let predicate = suffix == nil ? NSPredicate(format: format, prefix) : NSPredicate(format: format, prefix, suffix!)
        return app.descendants(matching: .any).matching(predicate).firstMatch
    }

    private func launchTeamList(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        app.openScreen("openTeamListScreen")
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func launchEditor(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = launchTeamList(contentSizeCategory: contentSizeCategory)
        element(app, "createTeamButton").tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    private func scrollTo(_ target: XCUIElement, in app: XCUIApplication) {
        var attempts = 0
        while !target.isHittable && attempts < 12 {
            app.swipeUp()
            attempts += 1
        }
        // スワイプの慣性が止まってから押す(動いている最中の押下はスクロールの停止として扱われる)。
        RunLoop.current.run(until: Date().addingTimeInterval(0.8))
        // 慣性で行き過ぎて、要素が上のナビゲーションバーの下に潜ったら、画面の中央まで引き戻す。
        let window = app.windows.firstMatch.frame
        if target.exists, target.frame.midY < window.height * 0.25 {
            let delta = window.midY - target.frame.midY
            let start = app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.3))
            start.press(forDuration: 0.05, thenDragTo: start.withOffset(CGVector(dx: 0, dy: delta)))
            RunLoop.current.run(until: Date().addingTimeInterval(0.8))
        }
    }

    private func pickSpecies(_ app: XCUIApplication, slot: Int, name: String) {
        let picker = element(app, "slotSpeciesPicker-\(slot)")
        scrollTo(picker, in: app)
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout), "\(slot)体目の「ポケモン」欄")
        picker.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let option = app.buttons[name]
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout))
        option.tap()
        XCTAssertFalse(element(app, "speciesSearchSheet").waitForExistence(timeout: 1), "選ぶとシートが閉じる")
    }

    private func assertTapTarget(_ target: XCUIElement, _ name: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "\(name) が無い", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.height, Self.minTapSize, "\(name) の高さ", file: file, line: line)
        XCTAssertGreaterThanOrEqual(target.frame.width, Self.minTapSize, "\(name) の幅", file: file, line: line)
    }

    private func assertInsideWindow(_ app: XCUIApplication, _ targets: [XCUIElement], file: StaticString = #filePath, line: UInt = #line) {
        let width = app.windows.firstMatch.frame.width
        for target in targets {
            scrollTo(target, in: app)
            XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), target.identifier, file: file, line: line)
            XCTAssertGreaterThanOrEqual(target.frame.minX, -Self.overflowTolerance, "\(target.identifier) が左にはみ出す", file: file, line: line)
            XCTAssertLessThanOrEqual(target.frame.maxX, width + Self.overflowTolerance, "\(target.identifier) が右にはみ出す", file: file, line: line)
        }
    }

    /// ラベルの先頭の数値(例: 「合計 12/66(残り 54)」→ 12)。
    private func total(of summary: XCUIElement) -> Int? {
        let digits = summary.label.drop { !$0.isNumber }.prefix { $0.isNumber }
        return Int(digits)
    }

    // MARK: - 一覧

    /// 空の一覧は 1 行の一言(長い案内の文を出さない)で、回り続ける読み込み表示を置かない。
    func testEmptyListIsOneLineAndHasNoSpinner() {
        let app = launchTeamList()
        let empty = element(app, "teamListEmpty")
        XCTAssertTrue(empty.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(empty.label, "まだ構築がありません")
        XCTAssertEqual(app.activityIndicators.count, 0, "回り続ける読み込み表示を置かない(静止したアイコン + 一言)")
        assertTapTarget(app.buttons["createTeamButton"], "新しい構築")
    }

    /// カード: [開く]は アイコン + 名前、[削除]はアイコンだけ(読み上げ名は従来の「「名前」を削除」)。削除は 2 段階のまま。
    func testListCardOpenHasLabelAndDeleteIsIconOnly() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        pickSpecies(app, slot: 2, name: Self.secondSpecies)
        element(app, "saveTeamButton").tap()
        XCTAssertTrue(element(app, "teamSavedNotice").waitForExistence(timeout: Self.existenceTimeout))
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        let open = elementBeginningWith(app, "teamOpen-")
        assertTapTarget(open, "開く")
        XCTAssertTrue(open.label.contains("を開く"), "読み上げ名は従来のまま: \(open.label)")
        let delete = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'teamDelete-'")).firstMatch
        assertTapTarget(delete, "削除")
        XCTAssertEqual(delete.label, "「構築 1」を削除", "読み上げ名は従来のまま")
        XCTAssertEqual(delete.staticTexts.count, 0, "削除はアイコンだけ(見える文字を置かない)")
        XCTAssertEqual(elementBeginningWith(app, "teamIcons-").label, "\(Self.firstSpecies)、\(Self.secondSpecies)", "空きの枠は読み上げない")

        delete.tap()
        assertTapTarget(elementBeginningWith(app, "teamDeleteConfirm-"), "削除する")
        assertTapTarget(elementBeginningWith(app, "teamDeleteCancel-"), "やめる")
        XCTAssertTrue(elementBeginningWith(app, "teamDeleteConfirmNotice-").exists, "確認の文(取り消せない操作)")
    }

    // MARK: - 編集: 6 枠のタイルの格子

    /// 見出しの下に 6 枠のタイル。空きは「+ N体目を追加」、埋まった枠は名前のカード。空の枠の案内の文は無い。
    func testTileGridShowsSixSlotsAndEmptyTilesHaveAddLabel() {
        let app = launchEditor()
        XCTAssertTrue(element(app, "teamSlotGrid").waitForExistence(timeout: Self.existenceTimeout), "タイルの格子")
        XCTAssertEqual(element(app, "teamSlotGrid").label, "メンバー")
        for number in 1...6 {
            let tile = app.buttons["teamTile-\(number)"]
            XCTAssertTrue(tile.waitForExistence(timeout: Self.existenceTimeout), "\(number)体目のタイル")
            XCTAssertEqual(tile.label, "\(number)体目を追加")
            XCTAssertGreaterThanOrEqual(tile.frame.height, Self.minTapSize)
            XCTAssertGreaterThanOrEqual(tile.frame.width, Self.minTapSize)
        }
        for number in 1...6 {
            XCTAssertFalse(element(app, "slotEmptyHint-\(number)").exists, "空の枠の案内の文は置かない")
        }
        XCTAssertEqual(app.activityIndicators.count, 0, "読み込み中も回り続ける表示を置かない")
    }

    func testFilledTileShowsTheSpeciesName() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        scrollTo(app.buttons["teamTile-1"], in: app)
        XCTAssertEqual(app.buttons["teamTile-1"].label, Self.firstSpecies, "埋まった枠のカードの名前")
        XCTAssertEqual(app.buttons["teamTile-2"].label, "2体目を追加", "他の枠は空きのまま")
    }

    /// タイルを押すと、その枠までスクロールして見える(Web ではその枠の欄へフォーカスが移る)。
    func testTappingATileScrollsToThatSlot() {
        let app = launchEditor()
        let tile = app.buttons["teamTile-6"]
        XCTAssertTrue(tile.waitForExistence(timeout: Self.existenceTimeout))
        tile.tap()
        let picker = element(app, "slotSpeciesPicker-6")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(picker.isHittable, "6体目の「ポケモン」欄まで送られて押せる")
    }

    // MARK: - 編集: 上へ・下へ・外すはアイコンだけ

    func testSlotControlsAreIconOnlyAndKeepAccessibilityNames() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        for (identifier, label) in [("slotMoveUp-1", "1体目を上へ"), ("slotMoveDown-1", "1体目を下へ"), ("slotRemove-1", "1体目を外す")] {
            let button = app.buttons[identifier]
            assertTapTarget(button, label)
            XCTAssertEqual(button.label, label, "読み上げ名は従来のまま")
            XCTAssertEqual(button.staticTexts.count, 0, "\(label): アイコンだけ(見える文字を置かない)")
        }
        XCTAssertFalse(app.buttons["slotMoveUp-1"].isEnabled, "1体目は上へ動かせない")
    }

    // MARK: - 編集: メンバーカード

    func testMemberCardUsesShortWordsAndEmptyMoveSlots() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        let heading = elementBeginningWith(app, "memberMovesHeading-")
        XCTAssertTrue(heading.waitForExistence(timeout: Self.existenceTimeout), "技の見出し")
        XCTAssertEqual(heading.label, "技")
        let nickname = app.textFields.matching(NSPredicate(format: "identifier BEGINSWITH 'memberNickname-'")).firstMatch
        XCTAssertTrue(nickname.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(nickname.placeholderValue, "ニックネーム")
        let slot = elementBeginningWith(app, "memberMoveSlot-", endingWith: "-0")
        XCTAssertEqual(slot.label, "(なし)", "空の技の枠")
    }

    /// SP: − / + ボタンと数値欄。合計と残りは数値 + バー。0 と 32・合計 66 で止まる。
    func testSPHasButtonsFieldSummaryAndBar() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        let summary = elementBeginningWith(app, "memberSPSummary-")
        scrollTo(summary, in: app)
        XCTAssertTrue(summary.waitForExistence(timeout: Self.existenceTimeout), "合計と残り")
        XCTAssertEqual(summary.label, "合計 0/66(残り 66)")
        XCTAssertTrue(elementBeginningWith(app, "memberSPBar-").exists, "合計のバー(装飾)")

        let up = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'memberSPUp-' AND identifier ENDSWITH '-atk'")).firstMatch
        let down = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'memberSPDown-' AND identifier ENDSWITH '-atk'")).firstMatch
        let field = app.textFields.matching(NSPredicate(format: "identifier BEGINSWITH 'memberSPField-' AND identifier ENDSWITH '-atk'")).firstMatch
        scrollTo(up, in: app)
        assertTapTarget(up, "SP を増やす")
        assertTapTarget(down, "SP を減らす")
        assertTapTarget(field, "SP の数値欄")
        XCTAssertEqual(up.label, "こうげきの能力ポイントを増やす")
        XCTAssertEqual(down.label, "こうげきの能力ポイントを減らす")
        XCTAssertEqual(field.label, "こうげきの能力ポイント")
        XCTAssertFalse(down.isEnabled, "0 で止まる")

        up.tap()
        XCTAssertEqual(field.value as? String, "1")
        XCTAssertEqual(total(of: summary), 1)
        XCTAssertEqual(summary.label, "合計 1/66(残り 65)")
        down.tap()
        XCTAssertEqual(field.value as? String, "0")
        XCTAssertEqual(summary.label, "合計 0/66(残り 66)")
    }

    /// 数値欄に直接入れても 32 で止まり(+ が押せなくなる)、合計に反映される。キーボードは「完了」で閉じられる。
    func testSPFieldAcceptsTypingAndStopsAt32() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        let field = app.textFields.matching(NSPredicate(format: "identifier BEGINSWITH 'memberSPField-' AND identifier ENDSWITH '-atk'")).firstMatch
        scrollTo(field, in: app)
        field.tap()
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: 3) + "32")
        let done = app.keyboards.buttons["完了"].exists ? app.keyboards.buttons["完了"] : app.buttons["完了"]
        XCTAssertTrue(done.waitForExistence(timeout: Self.existenceTimeout), "数値キーボードに「完了」")
        done.tap()
        let up = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'memberSPUp-' AND identifier ENDSWITH '-atk'")).firstMatch
        XCTAssertFalse(up.isEnabled, "32 で止まる")
        XCTAssertEqual(elementBeginningWith(app, "memberSPSummary-").label, "合計 32/66(残り 34)")
    }

    // MARK: - 編集: 技ピッカー(Web の技ピッカーとそろえる)

    private func openMoveSheet(_ app: XCUIApplication, slot: Int) {
        let button = elementBeginningWith(app, "memberMoveSlot-", endingWith: "-\(slot)")
        scrollTo(button, in: app)
        button.tap()
        XCTAssertTrue(element(app, "moveSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
    }

    /// 先頭に「(なし)」の行、技はタイプ順の群、他の枠で選んだ技は消さず薄く押せない行、自分の枠の技は選べる。
    func testMovePickerHasNoneRowTypeGroupsAndDisabledDuplicates() {
        let app = launchEditor()
        pickSpecies(app, slot: 1, name: Self.firstSpecies)

        openMoveSheet(app, slot: 0)
        let none = app.buttons["moveSearchRemoveButton"]
        XCTAssertTrue(none.waitForExistence(timeout: Self.existenceTimeout), "先頭の「(なし)」の行")
        XCTAssertEqual(none.label, "(なし)")
        XCTAssertGreaterThanOrEqual(none.frame.height, Self.minTapSize)
        XCTAssertTrue(app.staticTexts["かくとう"].exists, "タイプ順の群の見出し(かくとう)")
        XCTAssertTrue(app.staticTexts["どく"].exists, "タイプ順の群の見出し(どく)")
        let first = app.buttons["moveSearchResult-\(Self.moveA)"]
        XCTAssertTrue(first.waitForExistence(timeout: Self.existenceTimeout))
        first.tap()
        XCTAssertFalse(element(app, "moveSearchSheet").waitForExistence(timeout: 1), "選ぶとシートが閉じる")

        openMoveSheet(app, slot: 1)
        let taken = app.buttons["moveSearchResult-\(Self.moveA)"]
        XCTAssertTrue(taken.waitForExistence(timeout: Self.existenceTimeout), "他の枠で選んだ技も一覧に残す")
        XCTAssertFalse(taken.isEnabled, "他の枠で選んだ技は薄く押せない")
        XCTAssertTrue(app.buttons["moveSearchResult-\(Self.moveB)"].isEnabled)
        app.buttons["moveSearchCancelButton"].tap()

        openMoveSheet(app, slot: 0)
        XCTAssertTrue(app.buttons["moveSearchResult-\(Self.moveA)"].isEnabled, "自分の枠の技は選べる")
        app.buttons["moveSearchRemoveButton"].tap()
        XCTAssertEqual(elementBeginningWith(app, "memberMoveSlot-", endingWith: "-0").label, "(なし)", "「(なし)」で枠が空く")
    }

    // MARK: - AX5

    func testListAndEditorNewPartsNoHorizontalOverflowAtAX5() {
        let app = launchEditor(contentSizeCategory: Self.ax5ContentSizeCategory)
        assertInsideWindow(app, [element(app, "teamSlotGrid"), app.buttons["teamTile-1"], app.buttons["teamTile-6"]])
        pickSpecies(app, slot: 1, name: Self.firstSpecies)
        assertInsideWindow(
            app,
            [
                app.buttons["slotMoveUp-1"], app.buttons["slotMoveDown-1"], app.buttons["slotRemove-1"],
                elementBeginningWith(app, "memberMovesHeading-"), elementBeginningWith(app, "memberSPSummary-"),
                elementBeginningWith(app, "memberSPDown-", endingWith: "-hp"), elementBeginningWith(app, "memberSPUp-", endingWith: "-spe"),
                elementBeginningWith(app, "memberSPField-", endingWith: "-spa"),
            ])
        assertTapTarget(app.buttons["slotRemove-1"], "外す(AX5)")
        assertTapTarget(elementBeginningWith(app, "memberSPUp-", endingWith: "-spe"), "SP を増やす(AX5)")

        element(app, "saveTeamButton").tap()
        XCTAssertTrue(element(app, "teamSavedNotice").waitForExistence(timeout: Self.existenceTimeout))
        element(app, "backToListButton").tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
        let delete = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'teamDelete-'")).firstMatch
        assertInsideWindow(app, [elementBeginningWith(app, "teamIcons-"), elementBeginningWith(app, "teamOpen-"), delete])
        assertTapTarget(delete, "削除(AX5)")
        delete.tap()
        assertInsideWindow(app, [elementBeginningWith(app, "teamDeleteConfirm-"), elementBeginningWith(app, "teamDeleteCancel-")])
    }

    func testEmptyListNoHorizontalOverflowAtAX5() {
        let app = launchTeamList(contentSizeCategory: Self.ax5ContentSizeCategory)
        assertInsideWindow(app, [element(app, "teamListEmpty"), app.buttons["createTeamButton"]])
    }
}
