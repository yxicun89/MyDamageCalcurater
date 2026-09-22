import XCTest

/// ダメージ計算画面(P6-2a)の骨組みを確かめる。`POKECALC_USE_MOCK=1` で起動してモックを強制する。
/// ADR-0501「XCUITest で確かめること」のとおり、行の文言の数値は検査しない(モックの数値に依存しない)。
@MainActor
final class CalcScreenUITests: XCTestCase {
    /// モックの物理技の既定5行(ADR-0009。プリセットは openapi `DefenderPreset` の raw value)。
    /// 持ち物の比較が無いときの行 id は `<preset>@-`(ADR-0501 の約束)。
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]
    /// `Resources/items.json` の唯一の架空持ち物(比較トグルの対象)。
    private static let mockItemID = "test-item-berry"
    private static let existenceTimeout: TimeInterval = 5
    /// `Resources/species.json` の2番目。計算画面の既定の攻撃側(先頭の種族)とは別の種族にして、
    /// 構築から呼び出したときに種族が実際に変わったことを検査できるようにする(P6-2d)。
    private static let secondMockSpeciesName = "テストモンに"
    /// P6-2d のテスト用の構築ビルダーで付けるニックネーム(画面上の他の文字列と重複しない値にして
    /// `Menu` の項目をラベルで一意に選べるようにする)。
    private static let mockMemberNickname = "テストこたいP6"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別(Text/ボタン/コンテナ)を決め打ちせず、identifier だけで要素を探す。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// id を含む動的な identifier(`attackerTeamMember-<teamID>-<memberID>` 等)を前方一致で探す。
    /// このテストは構築を1つしか作らないので、前方一致でも一意に定まる(`TeamScreenUITests` と同じ)。
    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    /// ルート画面の構築一覧を開き、メンバー1体だけの構築を1つ作って一覧まで戻る
    /// (`TeamScreenUITests.testCreateAddMemberSaveAndDeleteTeamFlow` と同じ操作列。P6-2d)。
    private func createTeamWithOneMember(_ app: XCUIApplication) {
        let openTeamList = app.buttons["openTeamListScreen"]
        XCTAssertTrue(openTeamList.waitForExistence(timeout: Self.existenceTimeout))
        openTeamList.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: Self.existenceTimeout))
        nameField.tap()
        nameField.typeText("テストこうちくP6-2d")
        app.alerts.buttons["作成"].tap()

        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        // 計算画面の既定の攻撃側(先頭の種族)とは別の種族を選ぶ(呼び出しで種族が変わったことを
        // 検査できるように)。
        let speciesOption = app.buttons[Self.secondMockSpeciesName]
        XCTAssertTrue(speciesOption.waitForExistence(timeout: Self.existenceTimeout))
        speciesOption.tap()
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").waitForExistence(timeout: Self.existenceTimeout))

        // ニックネームを付ける(`Menu` の項目は `accessibilityIdentifier` が実機の UIKit メニューへ
        // 渡らない制約があるため、あとでラベルの文字列だけで一意に選べるようにする。
        // ADR-0501「P6-2d」9章)。
        let nicknameField = elementBeginningWith(app, "memberNickname-")
        XCTAssertTrue(nicknameField.waitForExistence(timeout: Self.existenceTimeout))
        nicknameField.tap()
        nicknameField.typeText(Self.mockMemberNickname)

        let saveButton = element(app, "saveTeamButton")
        XCTAssertTrue(saveButton.waitForExistence(timeout: Self.existenceTimeout))
        saveButton.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))

        // ルートへ戻る(構築一覧の戻るボタン)。
        app.navigationBars.buttons.element(boundBy: 0).tap()
    }

    private func launchCalcScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 開いた直後に、モックの既定(攻撃側 = モックの種族1番目・技 = その最初のダメージ技)で
    /// 物理の既定5行が出る(ADR-0501・ADR-0009)。
    func testOpeningShowsDefaultPhysicalRows() {
        let app = launchCalcScreen()

        XCTAssertTrue(element(app, "calcBackendModeBadge").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "attackerCard").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "defenderCard").exists)

        // 批評 M3c: プリセットのチップがカード内に収まりきらず「無振り」が見切れていた。
        // 画面幅いっぱいの3等分の行にしたので、3つとも画面内でタップできること。
        XCTAssertTrue(app.buttons["attackerPreset-aFull"].waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(app.buttons["attackerPreset-aFull"].isHittable)
        XCTAssertTrue(app.buttons["attackerPreset-aMax"].isHittable)
        XCTAssertTrue(app.buttons["attackerPreset-none"].isHittable)

        for rowID in Self.defaultPhysicalRowIDs {
            let row = element(app, "calcResultRow-\(rowID)")
            XCTAssertTrue(row.waitForExistence(timeout: Self.existenceTimeout), "行が無い: \(rowID)")
        }
    }

    /// `swapSidesButton` をタップすると2枚のカードの種族名が入れ替わる。
    /// カードのヘッダー(エンブレム・名前・タイプ)は種族セレクタの `Menu` のラベルを兼ねる
    /// (批評 M3d)。`Menu` は中身を1つのボタンにまとめるので、種族名はボタン自体の
    /// `accessibilityLabel`(`SpeciesHeaderMenuLabel` 呼び出し元が明示的に設定)で読む。
    func testSwapSidesSwapsCardSpeciesNames() {
        let app = launchCalcScreen()

        let attackerPicker = element(app, "attackerSpeciesPicker")
        let defenderPicker = element(app, "defenderSpeciesPicker")
        XCTAssertTrue(attackerPicker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(defenderPicker.waitForExistence(timeout: Self.existenceTimeout))
        let attackerBefore = attackerPicker.label
        let defenderBefore = defenderPicker.label
        XCTAssertNotEqual(attackerBefore, defenderBefore, "入れ替えを検査できるよう、既定の攻撃側・防御側は別の種族のはず")

        element(app, "swapSidesButton").tap()

        // 入れ替えは非同期(計算のやり直しを待つ)なので、値が変わるまで待つ。
        let swapped = NSPredicate(format: "label == %@", defenderBefore)
        expectation(for: swapped, evaluatedWith: attackerPicker, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)

        XCTAssertEqual(attackerPicker.label, defenderBefore)
        XCTAssertEqual(defenderPicker.label, attackerBefore)
    }

    /// `defenderItemToggle-<持ち物 ID>` をタップすると行が 5 → 10 になる
    /// (プリセット × {持ち物なし, その持ち物})。
    func testTogglingDefenderItemDoublesRowCount() {
        let app = launchCalcScreen()

        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").waitForExistence(timeout: Self.existenceTimeout))
        }

        let toggle = element(app, "defenderItemToggle-\(Self.mockItemID)")
        XCTAssertTrue(toggle.waitForExistence(timeout: Self.existenceTimeout))
        toggle.tap()

        for rowID in Self.defaultPhysicalRowIDs {
            let withItemID = rowID.replacingOccurrences(of: "@-", with: "@\(Self.mockItemID)")
            let withItemRow = element(app, "calcResultRow-\(withItemID)")
            XCTAssertTrue(withItemRow.waitForExistence(timeout: Self.existenceTimeout), "持ち物ありの行が無い: \(rowID)")
        }
        for rowID in Self.defaultPhysicalRowIDs {
            XCTAssertTrue(element(app, "calcResultRow-\(rowID)").exists, "持ち物なしの行も残っているはず: \(rowID)")
        }
    }

    /// 「構築から選ぶ」の入口(P6-2d。ADR-0501「P6-2d」7章)。構築が無い起動直後は無効 + 案内文、
    /// 構築ビルダーで1体作ってから呼ぶと攻撃側カードの種族が変わりプリセットの選択が外れる、
    /// プリセットを選び直すとまた選択表示が戻る。数値は検査しない(モックは計算しない)。
    func testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack() {
        let app = launchCalcScreen()

        let teamButton = element(app, "attackerTeamSourceButton")
        XCTAssertTrue(teamButton.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(teamButton.isEnabled, "構築が無いときは無効")
        XCTAssertTrue(element(app, "attackerTeamEmptyMessage").waitForExistence(timeout: Self.existenceTimeout))

        // ルートへ戻り、構築を1つ作って計算画面を開き直す(`loadTeams()` は画面の起動時に走る)。
        // `POKECALC_USE_MOCK=1` は起動のたびに構築の保存先を空にするため、プロセスを再起動せず
        // 同じアプリのまま画面を行き来する(`RootView` のコメント参照)。
        app.navigationBars.buttons.element(boundBy: 0).tap()
        createTeamWithOneMember(app)
        let openCalcAgain = app.buttons["openCalcScreen"]
        XCTAssertTrue(openCalcAgain.waitForExistence(timeout: Self.existenceTimeout))
        openCalcAgain.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))

        let teamButtonAfter = element(app, "attackerTeamSourceButton")
        XCTAssertTrue(teamButtonAfter.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(teamButtonAfter.isEnabled, "構築ができたので選べる")
        XCTAssertFalse(element(app, "attackerTeamEmptyMessage").exists)

        let attackerPicker = element(app, "attackerSpeciesPicker")
        XCTAssertTrue(attackerPicker.waitForExistence(timeout: Self.existenceTimeout))
        let presetButton = app.buttons["attackerPreset-aFull"]
        XCTAssertTrue(presetButton.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(presetButton.isSelected, "起動直後は既定のプリセットが選ばれている")

        teamButtonAfter.tap()
        // `Section` で構築ごとに見出しを付けているので、開いた直後に全メンバーが並ぶ(9章)。
        // `Menu`/`Section` どちらも項目の accessibilityIdentifier が渡らないため、一意なニックネームで選ぶ
        // (ADR-0501「P6-2d」9章)。
        let memberButton = app.buttons[Self.mockMemberNickname]
        XCTAssertTrue(memberButton.waitForExistence(timeout: Self.existenceTimeout))
        memberButton.tap()

        // 呼び出しは非同期(計算のやり直しを待つ)。プリセットの選択表示が外れるまで待つ。
        let presetDeselected = NSPredicate(format: "isSelected == false")
        expectation(for: presetDeselected, evaluatedWith: presetButton, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertEqual(attackerPicker.label, Self.secondMockSpeciesName, "呼び出した個体の種族に変わる")

        // プリセットを選び直すと、ピルの選択表示が戻る(構築の選択は排他に外れる)。
        presetButton.tap()
        let presetSelectedAgain = NSPredicate(format: "isSelected == true")
        expectation(for: presetSelectedAgain, evaluatedWith: presetButton, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }
}
