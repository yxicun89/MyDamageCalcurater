import XCTest

/// 逆算画面(P6-2b)の骨組みを確かめる。`POKECALC_USE_MOCK=1` で起動してモックを強制する。
/// ADR-0501「XCUITest で確かめること」のとおり、候補の数値・範囲の文言は検査しない
/// (モックの数値に依存しない。モックは計算しないので観測を足しても範囲は絞られない)。
@MainActor
final class ReverseScreenUITests: XCTestCase {
    /// `Resources/items.json` の唯一の架空持ち物(相手の持ち物候補トグルの対象。`CalcScreenUITests` と同じ)。
    private static let mockItemID = "test-item-berry"
    private static let existenceTimeout: TimeInterval = 5
    /// `Resources/species.json` の2番目。逆算画面の既定の自分(先頭の種族)とは別の種族にして、
    /// 構築から呼び出したときに種族が実際に変わったことを検査できるようにする(P6-2d)。
    private static let secondMockSpeciesName = "テストモンに"
    private static let secondMockSpeciesKey = "9002-000"
    /// `Resources/species.json` の3番目(issue #68: 種族検索シートで打ってから選ぶ確認用)。
    private static let thirdMockSpeciesName = "テストモンさん"
    private static let thirdMockSpeciesKey = "9003-000"
    /// P6-2d のテスト用の構築ビルダーで付けるニックネーム(`CalcScreenUITests` と同じ理由:
    /// `Menu` の項目は accessibilityIdentifier が渡らないため、ラベルで一意に選べる値にする)。
    private static let mockMemberNickname = "テストこたいP6逆算"

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別を決め打ちせず、identifier だけで要素を探す(`CalcScreenUITests` と同じ)。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// id を含む動的な identifier(`memberCard-<id>` 等)を前方一致で探す(`CalcScreenUITests` と同じ)。
    private func elementBeginningWith(_ app: XCUIApplication, _ prefix: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
    }

    /// ルート画面の構築一覧を開き、メンバー1体だけの構築を1つ作って一覧まで戻る
    /// (`CalcScreenUITests.createTeamWithOneMember` と同じ操作列。P6-2d)。
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
        nameField.typeText("テストこうちくP6-2d逆算")
        app.alerts.buttons["作成"].tap()

        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        // issue #68: 種族は検索シート経由で選ぶ(`Menu` ではなくなった)。
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let speciesOption = app.buttons[Self.secondMockSpeciesName]
        XCTAssertTrue(speciesOption.waitForExistence(timeout: Self.existenceTimeout))
        speciesOption.tap()
        XCTAssertTrue(elementBeginningWith(app, "memberCard-").waitForExistence(timeout: Self.existenceTimeout))

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

    private func launchReverseScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openReverseScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "reverseScreen").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 観測欄(`.keyboardType(.numberPad)`)に `text` を入力する。フィールドをタップしてから
    /// 1文字ずつ打鍵する(テンキーはハードウェアキーボードのタイプと挙動が異なるため)。
    private func typeObservation(_ app: XCUIApplication, fieldIdentifier: String, text: String) {
        let field = element(app, fieldIdentifier)
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(text)
    }

    /// 開いた直後は候補が無く、観測欄が1つだけある。`reverseObservationField-0` に有効な値を入れると
    /// 候補(`neutral@-` / `plus@-`。モックの `echoReverseResult` は性格クラス × 持ち物なしの2件を返す)が出て、
    /// `reversePremise` が見える(モックは defender で H32)。
    func testEnteringObservationShowsCandidatesAndPremise() {
        let app = launchReverseScreen()

        XCTAssertTrue(element(app, "reverseBackendModeBadge").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "reverseObservationField-0").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "reverseCandidateRow-neutral@-").exists, "観測を入れる前は候補が無い")

        typeObservation(app, fieldIdentifier: "reverseObservationField-0", text: "12")

        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "reverseCandidateRow-plus@-").exists)
        XCTAssertTrue(element(app, "reversePremise").exists)
        XCTAssertTrue(element(app, "reverseExactCount").exists)
    }

    /// `reverseAddObservationButton` で観測欄が増える。範囲外(101)を入力すると `reverseObservationError-1`
    /// が出て、不正な行があるうちは候補が消える(ADR-0501「入力できない」規則は `abc` 側、テンキーなので
    /// 数字しか打てず、ここでは範囲外だけを確かめる)。
    func testAddingObservationAndInvalidValueClearsCandidates() {
        let app = launchReverseScreen()
        typeObservation(app, fieldIdentifier: "reverseObservationField-0", text: "12")
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout))

        let addButton = element(app, "reverseAddObservationButton")
        XCTAssertTrue(addButton.waitForExistence(timeout: Self.existenceTimeout))
        addButton.tap()
        XCTAssertTrue(element(app, "reverseObservationField-1").waitForExistence(timeout: Self.existenceTimeout))

        typeObservation(app, fieldIdentifier: "reverseObservationField-1", text: "101")
        XCTAssertTrue(element(app, "reverseObservationError-1").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(element(app, "reverseCandidateRow-neutral@-").exists, "不正な行があるうちは候補を出さない")
    }

    /// issue #110(ADR-0501「issue #110」9章): 観測欄が上限(16行)に達すると `reverseAddObservationButton` が
    /// 無効になり、理由 `reverseObservationLimitHint` が出る。上限の手前では理由は出ない。
    /// 行数は XCUITest から `RequestLimits` を参照できないので、`reverseObservationField-<n>` の最後の番号で数える
    /// (開いた直後の1行 + 15回の追加 = 16行。値の正は `RequestLimits.maxObservations` と XCTest)。
    func testAddObservationButtonIsDisabledWithReasonAtTheLimit() {
        let app = launchReverseScreen()
        let addButton = element(app, "reverseAddObservationButton")
        XCTAssertTrue(addButton.waitForExistence(timeout: Self.existenceTimeout))

        let maxObservations = 16
        for _ in 1..<maxObservations {
            XCTAssertFalse(element(app, "reverseObservationLimitHint").exists, "上限の手前では理由を出さない")
            XCTAssertTrue(addButton.isEnabled)
            addButton.tap()
        }
        XCTAssertTrue(element(app, "reverseObservationField-\(maxObservations - 1)").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "reverseObservationLimitHint").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(addButton.isEnabled, "上限に達したら追加ボタンは無効")
        XCTAssertFalse(element(app, "reverseObservationField-\(maxObservations)").exists)

        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "reverse-observation-limit"
        screenshot.lifetime = .keepAlways
        add(screenshot)
    }

    /// `reverseOpponentItemToggle-<モックの持ち物 ID>` をタップすると候補が 2 → 4 になる
    /// (性格クラス2 × {持ち物なし, その持ち物})。
    func testTogglingOpponentItemCandidateAddsRows() {
        let app = launchReverseScreen()
        typeObservation(app, fieldIdentifier: "reverseObservationField-0", text: "12")
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "reverseCandidateRow-plus@-").exists)

        let toggle = element(app, "reverseOpponentItemToggle-\(Self.mockItemID)")
        XCTAssertTrue(toggle.waitForExistence(timeout: Self.existenceTimeout))
        toggle.tap()

        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@\(Self.mockItemID)").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "reverseCandidateRow-plus@\(Self.mockItemID)").exists)
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").exists, "持ち物なしの候補も残っているはず")
        XCTAssertTrue(element(app, "reverseCandidateRow-plus@-").exists)
    }

    /// `reverseSide-attacker` をタップすると観測欄が空の1つに戻り、候補が消える(単位の意味が変わるため)。
    func testSwitchingSideResetsObservationsAndClearsCandidates() {
        let app = launchReverseScreen()
        typeObservation(app, fieldIdentifier: "reverseObservationField-0", text: "12")
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout))

        let attackerSide = element(app, "reverseSide-attacker")
        XCTAssertTrue(attackerSide.waitForExistence(timeout: Self.existenceTimeout))
        attackerSide.tap()

        let field = element(app, "reverseObservationField-0")
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        // 空の1行に戻る = 2番目の観測欄は無く、1番目のテキストも空になっている。
        XCTAssertFalse(element(app, "reverseObservationField-1").exists)
        // 空の `TextField` は `value` が "" と nil のどちらの実装もありうるので両方許容する。
        XCTAssertTrue((field.value as? String ?? "").isEmpty, "側を切り替えたら観測欄のテキストも空に戻す")
        XCTAssertFalse(element(app, "reverseCandidateRow-neutral@-").exists, "側を切り替えたら候補を消す")
    }

    /// 「構築から選ぶ」の入口(P6-2d。ADR-0501「P6-2d」7章)。構築が無い起動直後は無効 + 案内文、
    /// 構築ビルダーで1体作ってから呼ぶと自分のカードの種族が変わりプリセットの選択が外れる、
    /// プリセットを選び直すとまた選択表示が戻る。数値は検査しない(`CalcScreenUITests` と同じ粒度)。
    func testTeamSourceRowEmptyThenSelectingMemberClearsPresetAndPresetClearsBack() {
        let app = launchReverseScreen()

        let teamButton = element(app, "reverseTeamSourceButton")
        XCTAssertTrue(teamButton.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertFalse(teamButton.isEnabled, "構築が無いときは無効")
        XCTAssertTrue(element(app, "reverseTeamEmptyMessage").waitForExistence(timeout: Self.existenceTimeout))

        // ルートへ戻り、構築を1つ作って逆算画面を開き直す(`loadTeams()` は画面の起動時に走る。
        // `POKECALC_USE_MOCK=1` は起動のたびに構築の保存先を空にするため、プロセスを再起動せず
        // 同じアプリのまま画面を行き来する。`CalcScreenUITests` と同じ)。
        app.navigationBars.buttons.element(boundBy: 0).tap()
        createTeamWithOneMember(app)
        let openReverseAgain = app.buttons["openReverseScreen"]
        XCTAssertTrue(openReverseAgain.waitForExistence(timeout: Self.existenceTimeout))
        openReverseAgain.tap()
        XCTAssertTrue(element(app, "reverseScreen").waitForExistence(timeout: Self.existenceTimeout))

        let teamButtonAfter = element(app, "reverseTeamSourceButton")
        XCTAssertTrue(teamButtonAfter.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(teamButtonAfter.isEnabled, "構築ができたので選べる")
        XCTAssertFalse(element(app, "reverseTeamEmptyMessage").exists)

        // 既定の側は「与えたダメージ」(自分が攻撃側)。自分のカードは `reverseMySpeciesPicker`。
        let mySpeciesPicker = element(app, "reverseMySpeciesPicker")
        XCTAssertTrue(mySpeciesPicker.waitForExistence(timeout: Self.existenceTimeout))
        let presetButton = app.buttons["reverseAttackerPreset-aFull"]
        XCTAssertTrue(presetButton.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(presetButton.isSelected, "起動直後は既定のプリセットが選ばれている")

        teamButtonAfter.tap()
        // `Section` で構築ごとに見出しを付けているので、開いた直後に全メンバーが並ぶ(9章)。
        let memberButton = app.buttons[Self.mockMemberNickname]
        XCTAssertTrue(memberButton.waitForExistence(timeout: Self.existenceTimeout))
        memberButton.tap()

        // 呼び出しは非同期。プリセットの選択表示が外れるまで待つ。
        let presetDeselected = NSPredicate(format: "isSelected == false")
        expectation(for: presetDeselected, evaluatedWith: presetButton, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
        XCTAssertEqual(mySpeciesPicker.label, Self.secondMockSpeciesName, "呼び出した個体の種族に変わる")

        // プリセットを選び直すと、ピルの選択表示が戻る(構築の選択は排他に外れる)。
        presetButton.tap()
        let presetSelectedAgain = NSPredicate(format: "isSelected == true")
        expectation(for: presetSelectedAgain, evaluatedWith: presetButton, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    /// issue #68: `reverseOpponentSpeciesPicker` をタップすると `speciesSearchSheet` が出て、検索欄に
    /// 打つと候補が絞られ、1件タップするとシートが閉じて選択が反映される(`CalcScreenUITests` と同じ流れ)。
    func testOpponentSpeciesSearchSheetFiltersAndSelects() {
        let app = launchReverseScreen()

        let opponentPicker = element(app, "reverseOpponentSpeciesPicker")
        XCTAssertTrue(opponentPicker.waitForExistence(timeout: Self.existenceTimeout))
        opponentPicker.tap()

        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText(Self.thirdMockSpeciesName)

        let result = element(app, "speciesSearchResult-\(Self.thirdMockSpeciesKey)")
        XCTAssertTrue(result.waitForExistence(timeout: Self.existenceTimeout))
        // ラベルの文字列だけで探すと、シートの裏の自分側カードのヘッダー(既定の自分 = 2番目の種族。
        // ラベルが同じ文字列)を誤って拾う(`CalcScreenUITests` と同じ理由)。検索結果一覧に限定した
        // identifier で確かめる。
        XCTAssertFalse(
            element(app, "speciesSearchResult-\(Self.secondMockSpeciesKey)").exists, "絞り込まれて一致しない種族は消える"
        )
        result.tap()

        XCTAssertFalse(element(app, "speciesSearchSheet").exists, "選ぶとシートが閉じる")
        XCTAssertEqual(opponentPicker.label, Self.thirdMockSpeciesName)
    }
}
