import XCTest

/// 防御側・相手の特性(issue #272。ADR-0501「P6-19」6章)。`POKECALC_USE_MOCK=1` で起動する。
///
/// モックはフィクスチャの `nullifiesMoveType` で「特性で結果が分かれる」状況を決め打ちに作る(ADR-0501「P6-19」5章):
/// `テストモンよん`(9004-000)は特性を2つ持ち、2つ目(`テストとくせいかくとうむこう`)がかくとう技を無効にする。
/// 既定の技(テストわざぶつりA)はかくとうなので、防御側(逆算は相手)を 9004-000 にすると行・候補が特性で分かれる。
/// 数値は検査しない(モックは計算しない)。ID・副題・選択の表示とリセットだけを見る。
/// 選択・要求の規則は `CalcViewModelDefenderAbilityTests`・`ReverseViewModelOpponentAbilityTests` で固定している。
@MainActor
final class AbilityPickerUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// 画面外の要素をタップできる位置までスクロールする最大回数(片方向あたり)。
    private static let maxScrollAttempts = 6

    private static let splitSpeciesName = "テストモンよん"
    private static let splitSpeciesKey = "9004-000"
    private static let plainSpeciesName = "テストモンに"
    private static let plainSpeciesKey = "9002-000"
    private static let deltaAbilityID = "test-ability-delta"
    private static let deltaAbilityName = "テストとくせいデルタ"
    private static let immuneAbilityID = "test-ability-fighting-immune"
    private static let immuneAbilityName = "テストとくせいかくとうむこう"
    private static let unspecifiedLabel = "指定なし"
    /// モックの物理技の既定5行(`CalcScreenUITests.defaultPhysicalRowIDs` と同じ)。
    private static let physicalPresets = ["none", "hp", "hb_boost", "hb", "hb_full"]

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// 上へ、届かなければ下へスクロールしてタップできる位置に出す。
    private func scrollUntilHittable(_ app: XCUIApplication, container: String, _ target: XCUIElement) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が無い: \(target)")
        var attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts {
            element(app, container).swipeUp()
            attempts += 1
        }
        attempts = 0
        while !target.isHittable && attempts < Self.maxScrollAttempts * 2 {
            element(app, container).swipeDown()
            attempts += 1
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)")
    }

    /// 種族の検索シートで `name` を打ち、`speciesSearchResult-<key>` を選ぶ(`CalcScreenUITests` と同じ流れ)。
    /// 検索欄の入力は画面ごとに保持される(8章)ので、同じ画面で2回目に選ぶときは `previousQuery` を渡して消してから打つ。
    private func chooseSpecies(_ app: XCUIApplication, picker: String, container: String, name: String, key: String,
                               previousQuery: String = "") {
        let pickerElement = element(app, picker)
        scrollUntilHittable(app, container: container, pickerElement)
        pickerElement.tap()
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let field = app.searchFields.firstMatch
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        if !previousQuery.isEmpty {
            field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: previousQuery.count))
        }
        field.typeText(name)
        let result = element(app, "speciesSearchResult-\(key)")
        XCTAssertTrue(result.waitForExistence(timeout: Self.existenceTimeout))
        result.tap()
    }

    /// `Menu` の特性の選択肢をラベルで選ぶ(`Menu` の項目には identifier が渡らないため。既存の XCUITest と同じ理由)。
    private func chooseAbility(_ app: XCUIApplication, picker: String, container: String, name: String) {
        let pickerElement = element(app, picker)
        scrollUntilHittable(app, container: container, pickerElement)
        pickerElement.tap()
        let option = app.buttons[name].firstMatch
        XCTAssertTrue(option.waitForExistence(timeout: Self.existenceTimeout), "選択肢が無い: \(name)")
        option.tap()
    }

    private func waitForValue(_ target: XCUIElement, _ value: String) {
        expectation(for: NSPredicate(format: "value == %@", value), evaluatedWith: target, handler: nil)
        waitForExpectations(timeout: Self.existenceTimeout)
    }

    // MARK: - 計算画面

    private func launchCalcScreen() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        app.launch()
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout))
        return app
    }

    /// 既定(防御側 テストモンに・特性1つ)では行の ID も副題も今までどおり。防御側を特性の2つある種族にすると
    /// 行が特性で分かれ、ID に特性が付き、各行に「特性: …」が出る。「詳細」の「防御側の特性」で1つに決めると
    /// 分かれなくなり、防御側の種族を変えると「指定なし」に戻る。
    func testDefenderAbilitySplitRowsPickerAndReset() {
        let app = launchCalcScreen()
        XCTAssertFalse(element(app, "calcResultAbility-none@-").exists, "特性で分かれない行には副題を出さない")

        chooseSpecies(app, picker: "defenderSpeciesPicker", container: "calcScreen",
                      name: Self.splitSpeciesName, key: Self.splitSpeciesKey)

        for preset in Self.physicalPresets {
            let deltaRow = "\(preset)@-@\(Self.deltaAbilityID)"
            let immuneRow = "\(preset)@-@\(Self.immuneAbilityID)"
            XCTAssertTrue(element(app, "calcResultRow-\(deltaRow)").waitForExistence(timeout: Self.existenceTimeout),
                          "特性で分かれた行: \(deltaRow)")
            XCTAssertTrue(element(app, "calcResultRow-\(immuneRow)").exists, "特性で分かれた行: \(immuneRow)")
        }
        XCTAssertFalse(element(app, "calcResultRow-none@-").exists, "分かれた行は特性つきの ID になる")
        XCTAssertEqual(element(app, "calcResultAbility-none@-@\(Self.deltaAbilityID)").label,
                       "特性: \(Self.deltaAbilityName)")
        XCTAssertEqual(element(app, "calcResultAbility-none@-@\(Self.immuneAbilityID)").label,
                       "特性: \(Self.immuneAbilityName)")

        let toggle = element(app, "calcConditionsToggle")
        scrollUntilHittable(app, container: "calcScreen", toggle)
        toggle.tap()
        XCTAssertTrue(element(app, "calcConditionsPanel").waitForExistence(timeout: Self.existenceTimeout))
        let picker = element(app, "calcDefenderAbilityPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout))
        XCTAssertEqual(picker.value as? String, Self.unspecifiedLabel, "既定は「指定なし」")
        XCTAssertTrue(element(app, "calcAttackerAbilityPicker").exists, "攻撃側の特性の選択は残る")

        chooseAbility(app, picker: "calcDefenderAbilityPicker", container: "calcScreen", name: Self.immuneAbilityName)
        waitForValue(picker, Self.immuneAbilityName)
        XCTAssertTrue(element(app, "calcResultRow-none@-").waitForExistence(timeout: Self.existenceTimeout),
                      "特性を1つに決めると行は分かれず、既存の ID に戻る")
        XCTAssertFalse(element(app, "calcResultRow-none@-@\(Self.deltaAbilityID)").exists)
        XCTAssertFalse(element(app, "calcResultAbility-none@-").exists, "分かれていなければ副題を出さない")

        chooseSpecies(app, picker: "defenderSpeciesPicker", container: "calcScreen",
                      name: Self.plainSpeciesName, key: Self.plainSpeciesKey, previousQuery: Self.splitSpeciesName)
        waitForValue(picker, Self.unspecifiedLabel)
    }

    /// 攻守入れ替えでも防御側の特性は「指定なし」に戻る。
    func testSwapResetsDefenderAbility() {
        let app = launchCalcScreen()
        chooseSpecies(app, picker: "defenderSpeciesPicker", container: "calcScreen",
                      name: Self.splitSpeciesName, key: Self.splitSpeciesKey)
        XCTAssertTrue(element(app, "calcResultRow-none@-@\(Self.deltaAbilityID)")
            .waitForExistence(timeout: Self.existenceTimeout))

        let toggle = element(app, "calcConditionsToggle")
        scrollUntilHittable(app, container: "calcScreen", toggle)
        toggle.tap()
        chooseAbility(app, picker: "calcDefenderAbilityPicker", container: "calcScreen", name: Self.deltaAbilityName)
        let picker = element(app, "calcDefenderAbilityPicker")
        waitForValue(picker, Self.deltaAbilityName)

        let swap = element(app, "swapSidesButton")
        scrollUntilHittable(app, container: "calcScreen", swap)
        swap.tap()
        waitForValue(picker, Self.unspecifiedLabel)
    }

    // MARK: - 逆算画面

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

    /// 与えたダメージで相手を特性の2つある種族にし、観測を入れると候補が特性で分かれる。「相手の特性」で1つに
    /// 決めると分かれなくなり、側を切り替えると「指定なし」に戻る。
    func testOpponentAbilitySplitCandidatesPickerAndReset() {
        let app = launchReverseScreen()
        let picker = element(app, "reverseOpponentAbilityPicker")
        XCTAssertTrue(picker.waitForExistence(timeout: Self.existenceTimeout), "相手の特性の選択は常に出す")
        XCTAssertEqual(picker.value as? String, Self.unspecifiedLabel)

        chooseSpecies(app, picker: "reverseOpponentSpeciesPicker", container: "reverseScreen",
                      name: Self.splitSpeciesName, key: Self.splitSpeciesKey)

        let field = element(app, "reverseObservationField-0")
        scrollUntilHittable(app, container: "reverseScreen", field)
        field.tap()
        field.typeText("30")
        app.buttons["完了"].firstMatch.tap()

        for natureClass in ["neutral", "plus"] {
            for ability in [Self.deltaAbilityID, Self.immuneAbilityID] {
                let id = "\(natureClass)@-@\(ability)"
                XCTAssertTrue(element(app, "reverseCandidateRow-\(id)").waitForExistence(timeout: Self.existenceTimeout),
                              "特性で分かれた候補: \(id)")
            }
        }
        XCTAssertEqual(element(app, "reverseCandidateAbility-neutral@-@\(Self.immuneAbilityID)").label,
                       "特性: \(Self.immuneAbilityName)")

        chooseAbility(app, picker: "reverseOpponentAbilityPicker", container: "reverseScreen", name: Self.deltaAbilityName)
        waitForValue(picker, Self.deltaAbilityName)
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout),
                      "特性を1つに決めると候補は分かれず、既存の ID に戻る")
        XCTAssertFalse(element(app, "reverseCandidateAbility-neutral@-").exists)

        let sideSwitch = element(app, "reverseSide-attacker")
        scrollUntilHittable(app, container: "reverseScreen", sideSwitch)
        sideSwitch.tap()
        waitForValue(picker, Self.unspecifiedLabel)
    }
}
