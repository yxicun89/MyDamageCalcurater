import XCTest

/// 逆算画面(P6-2b)の骨組みを確かめる。`POKECALC_USE_MOCK=1` で起動してモックを強制する。
/// README「XCUITest で確かめること」のとおり、候補の数値・範囲の文言は検査しない
/// (モックの数値に依存しない。モックは計算しないので観測を足しても範囲は絞られない)。
@MainActor
final class ReverseScreenUITests: XCTestCase {
    /// `Resources/items.json` の唯一の架空持ち物(相手の持ち物候補トグルの対象。`CalcScreenUITests` と同じ)。
    private static let mockItemID = "test-item-berry"
    private static let existenceTimeout: TimeInterval = 5

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    /// SwiftUI の View 種別を決め打ちせず、identifier だけで要素を探す(`CalcScreenUITests` と同じ)。
    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
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
    /// が出て、不正な行があるうちは候補が消える(README「入力できない」規則は `abc` 側、テンキーなので
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
}
