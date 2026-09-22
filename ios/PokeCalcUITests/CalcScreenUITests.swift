import XCTest

/// ダメージ計算画面(P6-2a)の骨組みを確かめる。`POKECALC_USE_MOCK=1` で起動してモックを強制する。
/// README「XCUITest で確かめること」のとおり、行の文言の数値は検査しない(モックの数値に依存しない)。
@MainActor
final class CalcScreenUITests: XCTestCase {
    /// モックの物理技の既定5行(ADR-0009。プリセットは openapi `DefenderPreset` の raw value)。
    /// 持ち物の比較が無いときの行 id は `<preset>@-`(README の約束)。
    private static let defaultPhysicalRowIDs = ["none@-", "hp@-", "hb_boost@-", "hb@-", "hb_full@-"]
    /// `Resources/items.json` の唯一の架空持ち物(比較トグルの対象)。
    private static let mockItemID = "test-item-berry"
    private static let existenceTimeout: TimeInterval = 5

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別(Text/ボタン/コンテナ)を決め打ちせず、identifier だけで要素を探す。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
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
    /// 物理の既定5行が出る(README・ADR-0009)。
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
}
