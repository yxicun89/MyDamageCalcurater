import XCTest

/// P6-14: 最大の文字サイズ(アクセシビリティ域の最大、AX5 =
/// `.accessibility5` / `UICTContentSizeCategoryAccessibilityXXXL`)で計算画面・逆算画面・構築画面が
/// 横にはみ出して左端が切れる不具合(docs/plan.md P6-14。2026-09-23 のスクリーンショットで確認済み)。
///
/// `POKECALC_USE_MOCK=1` で起動してモックを強制する(他の XCUITest と同じ。ADR-0500 §5・§7)。
/// 文字サイズは起動引数 `-UIPreferredContentSizeCategoryName` で固定する(Apple 標準の
/// `UICTContentSizeCategory*` の内部識別子。シミュレータの設定を変えずに再現できる)。
/// AX5 で実際に切り替わったこと自体は `assertAX5TookEffect` で確かめる
/// (`CalcScreenView.cardsRow` / `ReverseScreenView.cardsRow` は `dynamicTypeSize >= .accessibility1`
/// で横並び→縦積みに切り替わる実装になっており、この切り替えが起きていれば起動引数が効いている
/// と分かる)。
///
/// このテストは「要素が画面の外に出ていないか」だけを見る(モックの数値・文言には依存しない。
/// ADR-0501「XCUITest で確かめること」と同じ粒度)。P6-14 の時点では実装を直していないので、
/// AX5 のケースは失敗してよい(spec-writer の役目は受け入れ条件とテストを先に書くこと)。
@MainActor
final class LargeTextLayoutUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// AX5(アクセシビリティ域の最大)。`UICTContentSizeCategoryAccessibilityXXXL` が
    /// `UIContentSizeCategory.accessibilityExtraExtraExtraLarge` に対応する内部識別子。
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    /// フォントのサブピクセル丸めを許容する程度の小さな余裕(pt)。
    private static let overflowTolerance: CGFloat = 1

    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    // MARK: - 起動

    private func launchWithMock(contentSizeCategory: String? = nil) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment["POKECALC_USE_MOCK"] = "1"
        if let contentSizeCategory {
            app.launchArguments += ["-UIPreferredContentSizeCategoryName", contentSizeCategory]
        }
        app.launch()
        return app
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any)[identifier]
    }

    /// 同じ identifier を持つ要素をすべて集める。`ResultRowView`(`CalcScreenResults.swift`)のように
    /// `glassCard()` のコンテナに `.accessibilityElement(children: .contain)` を付け忘れていると、
    /// 中の複数の `Text`(見出し・%幅・確定数)が同じ親の identifier に「飲まれ」て同一 identifier の
    /// 要素が複数見つかる(`CalcConditionsSection.swift` のコメントが説明している既知の SwiftUI の
    /// 挙動。実際に本テストの初回実行で `calcResultRow-none@-` が3件ヒットして `.frame` の解決に失敗した)。
    /// これは P6-14(横はみ出し)とは別の、識別子まわりの既存の抜け漏れなので、本テストは1件に
    /// 決め打ちせず全件の frame を見て、どれか1件でもはみ出していれば失敗として報告する。
    private func matchingElements(_ app: XCUIApplication, _ identifier: String) -> [XCUIElement] {
        let query = app.descendants(matching: .any).matching(identifier: identifier)
        let count = query.count
        guard count > 0 else { return [] }
        return (0..<count).map { query.element(boundBy: $0) }
    }

    /// `matchingElements` の前方一致版。構築のメンバー行は identifier に UUID の member id が入る
    /// (`memberSP-<id>-hp` など)ため、完全一致では拾えない。
    private func matchingElementsBeginningWith(_ app: XCUIApplication, prefix: String) -> [XCUIElement] {
        let query = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix))
        let count = query.count
        guard count > 0 else { return [] }
        return (0..<count).map { query.element(boundBy: $0) }
    }

    // MARK: - 共通チェック

    /// AX5 の起動引数が実際に効いていることを確かめる(`cardsRow` の縦積み分岐が発動していれば
    /// `dynamicTypeSize >= .accessibility1` になっている = 起動引数が反映されている)。
    private func assertAX5TookEffect(_ app: XCUIApplication, attackerIdentifier: String, defenderIdentifier: String, file: StaticString = #filePath, line: UInt = #line) {
        let attacker = element(app, attackerIdentifier)
        let defender = element(app, defenderIdentifier)
        XCTAssertTrue(attacker.waitForExistence(timeout: Self.existenceTimeout), "\(attackerIdentifier) が見つからない", file: file, line: line)
        XCTAssertTrue(defender.exists, "\(defenderIdentifier) が見つからない", file: file, line: line)
        // 縦積みなら defender の上端は attacker の下端以上になる(横並びだと defender.minY はおおよそ attacker.minY と同じ)。
        XCTAssertGreaterThanOrEqual(
            defender.frame.minY, attacker.frame.maxY - Self.overflowTolerance,
            "AX5 で \(attackerIdentifier)/\(defenderIdentifier) が縦積みになっていない"
                + "(attacker=\(attacker.frame) defender=\(defender.frame))。"
                + "-UIPreferredContentSizeCategoryName 起動引数が効いていない可能性",
            file: file, line: line
        )
    }

    /// 与えた identifier の要素すべてについて、ウィンドウの左右をはみ出していないかを確かめる。
    /// 途中で止めず全要素を集計してから1つの失敗メッセージにまとめる(どの要素がどれだけはみ出したか
    /// 実装者が一目で分かるように)。
    private func assertNoHorizontalOverflow(_ app: XCUIApplication, identifiers: [String], file: StaticString = #filePath, line: UInt = #line) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)

        var offenders: [String] = []
        for identifier in identifiers {
            // 存在確認は1件でもヒットすれば良い(`waitForExistence` は複数マッチでも解決できる)。
            XCTAssertTrue(
                element(app, identifier).waitForExistence(timeout: Self.existenceTimeout),
                "要素が見つからない: \(identifier)", file: file, line: line
            )
            let matches = matchingElements(app, identifier)
            for (index, el) in matches.enumerated() {
                guard el.exists else { continue }
                let frame = el.frame
                // 非表示(サイズ0)の要素は対象外。
                if frame.width == 0 && frame.height == 0 { continue }
                let leftCut = frame.minX < -Self.overflowTolerance
                let rightCut = frame.maxX > windowFrame.maxX + Self.overflowTolerance
                if leftCut || rightCut {
                    let suffix = matches.count > 1 ? "[\(index)/\(matches.count)] label=\(el.label)" : ""
                    offenders.append("\(identifier)\(suffix): frame=\(frame) window=\(windowFrame) leftCut=\(leftCut) rightCut=\(rightCut)")
                }
            }
        }

        XCTAssertTrue(
            offenders.isEmpty,
            "横にはみ出している要素があります(window=\(windowFrame)):\n" + offenders.joined(separator: "\n"),
            file: file, line: line
        )
    }

    /// `assertNoHorizontalOverflow` の前方一致版(`matchingElementsBeginningWith` を使う)。
    /// 構築のメンバー行のように identifier に member id(UUID)が入って完全一致できない要素を検査する。
    private func assertNoHorizontalOverflowForPrefixes(_ app: XCUIApplication, prefixes: [String], file: StaticString = #filePath, line: UInt = #line) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)

        var offenders: [String] = []
        for prefix in prefixes {
            let firstMatch = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
            XCTAssertTrue(firstMatch.waitForExistence(timeout: Self.existenceTimeout), "要素が見つからない(前方一致): \(prefix)", file: file, line: line)
            let matches = matchingElementsBeginningWith(app, prefix: prefix)
            for (index, el) in matches.enumerated() {
                guard el.exists else { continue }
                let frame = el.frame
                if frame.width == 0 && frame.height == 0 { continue }
                let leftCut = frame.minX < -Self.overflowTolerance
                let rightCut = frame.maxX > windowFrame.maxX + Self.overflowTolerance
                if leftCut || rightCut {
                    let suffix = matches.count > 1 ? "[\(index)/\(matches.count)] label=\(el.label)" : ""
                    offenders.append("\(prefix)*\(suffix): frame=\(frame) window=\(windowFrame) leftCut=\(leftCut) rightCut=\(rightCut)")
                }
            }
        }

        XCTAssertTrue(
            offenders.isEmpty,
            "横にはみ出している要素があります(window=\(windowFrame)):\n" + offenders.joined(separator: "\n"),
            file: file, line: line
        )
    }

    // MARK: - 計算画面

    private static let calcScreenIdentifiers = [
        "calcBackendModeBadge",
        "attackerCard",
        "swapSidesButton",
        "defenderCard",
        "attackerPreset-none",
        "attackerPreset-aFull",
        "attackerPreset-aMax",
        "attackerTeamSourceButton",
        "movePicker",
        "calcConditionsToggle",
        // モックの既定(物理技・比較対象なし)の先頭行(`CalcScreenUITests.defaultPhysicalRowIDs` の1つ目と同じ)。
        "calcResultRow-none@-",
    ]

    private func openCalcScreen(_ app: XCUIApplication) {
        let openButton = app.buttons["openCalcScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "calcScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    /// 既定の文字サイズでの回帰確認(既存 XCUITest と同じ前提: はみ出しは無いはず)。
    func testCalcScreenNoHorizontalOverflowAtDefaultSize() {
        let app = launchWithMock()
        openCalcScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.calcScreenIdentifiers)
    }

    /// P6-14 本体: AX5 でのはみ出し確認。現時点では失敗してよい(実装未修正)。
    func testCalcScreenNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openCalcScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "attackerCard", defenderIdentifier: "defenderCard")
        assertNoHorizontalOverflow(app, identifiers: Self.calcScreenIdentifiers)
    }

    // MARK: - 逆算画面

    private static let reverseScreenIdentifiers = [
        "reverseBackendModeBadge",
        "reverseSide-defender",
        "reverseSide-attacker",
        "reverseMyCard",
        "reverseOpponentCard",
        // 既定の側(defender = 与えたダメージ)。`ReverseScreenView.presetSegmentedRow` を参照。
        "reverseAttackerPreset-none",
        "reverseAttackerPreset-aFull",
        "reverseAttackerPreset-aMax",
        "reverseTeamSourceButton",
        "reverseMovePicker",
        "reverseAddObservationButton",
    ]

    private func openReverseScreen(_ app: XCUIApplication) {
        let openButton = app.buttons["openReverseScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "reverseScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    func testReverseScreenNoHorizontalOverflowAtDefaultSize() {
        let app = launchWithMock()
        openReverseScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.reverseScreenIdentifiers)
    }

    func testReverseScreenNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openReverseScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "reverseMyCard", defenderIdentifier: "reverseOpponentCard")
        assertNoHorizontalOverflow(app, identifiers: Self.reverseScreenIdentifiers)
    }

    /// 観測欄に値を入れて候補カード(`ReverseCandidateCardView`)を実際に描画した状態での AX5 検査
    /// (ADR-0501「P6-14」§5「未確認・要フォローアップ」。候補が出ていない起動直後だけでは
    /// `percentRangeText` の `Text` が描画されず原因パターンを再現できない)。
    /// `ReverseScreenUITests.testEnteringObservationShowsCandidatesAndPremise` と同じ操作
    /// (`reverseObservationField-0` に "12" を入力)で `reverseCandidateRow-neutral@-` を出す。
    func testReverseScreenWithCandidateNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openReverseScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "reverseMyCard", defenderIdentifier: "reverseOpponentCard")

        let field = element(app, "reverseObservationField-0")
        XCTAssertTrue(field.waitForExistence(timeout: Self.existenceTimeout))
        field.tap()
        field.typeText("12")
        XCTAssertTrue(element(app, "reverseCandidateRow-neutral@-").waitForExistence(timeout: Self.existenceTimeout))

        assertNoHorizontalOverflow(app, identifiers: Self.reverseScreenIdentifiers + ["reverseCandidateRow-neutral@-"])
    }

    // MARK: - 構築(一覧・編集)画面

    private static let teamListIdentifiers = [
        "teamListScreen",
        "createTeamButton",
    ]

    private static let teamEditIdentifiers = [
        "teamEditScreen",
        "teamNameField",
        "addMemberButton",
        "saveTeamButton",
    ]

    /// `RootView.makeTeamStore()` が `POKECALC_USE_MOCK=1` のとき起動のたびに専用 UserDefaults suite
    /// を空にするため(`TeamScreenUITests` のコメントと同じ)、他のテストの構築データと衝突しない。
    private func openTeamListScreen(_ app: XCUIApplication) {
        let openButton = app.buttons["openTeamListScreen"]
        XCTAssertTrue(openButton.waitForExistence(timeout: Self.existenceTimeout))
        openButton.tap()
        XCTAssertTrue(element(app, "teamListScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    /// 構築を1つ作って編集画面まで進む(`CalcScreenUITests.createTeamWithOneMember` の前半と同じ操作列。
    /// メンバーは追加せず、編集画面そのものの要素だけを見る)。
    private func createTeamAndOpenEditScreen(_ app: XCUIApplication) {
        let createButton = element(app, "createTeamButton")
        XCTAssertTrue(createButton.waitForExistence(timeout: Self.existenceTimeout))
        createButton.tap()
        let nameField = app.alerts.textFields.firstMatch
        XCTAssertTrue(nameField.waitForExistence(timeout: Self.existenceTimeout))
        nameField.tap()
        nameField.typeText("テストこうちくP6-14")
        app.alerts.buttons["作成"].tap()
        XCTAssertTrue(element(app, "teamEditScreen").waitForExistence(timeout: Self.existenceTimeout))
    }

    func testTeamScreensNoHorizontalOverflowAtDefaultSize() {
        let app = launchWithMock()
        openTeamListScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.teamListIdentifiers)
        createTeamAndOpenEditScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.teamEditIdentifiers)
    }

    func testTeamScreensNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openTeamListScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.teamListIdentifiers)
        createTeamAndOpenEditScreen(app)
        assertNoHorizontalOverflow(app, identifiers: Self.teamEditIdentifiers)
    }

    /// `TeamEditMemberCard` の識別子は member id(UUID)を含むため前方一致で検査する
    /// (`matchingElementsBeginningWith`)。
    private static let teamEditMemberCardPrefixes = [
        "memberCard-",
        "memberNickname-",
        "memberSpeciesPicker-",
        "memberItemPicker-",
        "memberAbilityPicker-",
        "memberNaturePicker-",
        "memberTeraPicker-",
        "memberMoveSlot-",
        "memberSP-",
    ]

    /// `ReverseScreenUITests`/`CalcScreenUITests` の `createTeamWithOneMember` と同じ種族選択
    /// (`Resources/species.json` の2番目)。`speciesSearchSheet` 経由で選ぶ(issue #68)。
    private static let mockMemberSpeciesName = "テストモンに"

    /// メンバーを1体追加した状態(`TeamEditMemberCard`)の AX5 検査(ADR-0501「P6-14」§5
    /// 「未確認・要フォローアップ」)。`spStepper`(`TeamEditMemberCard.swift` 258〜284行)の
    /// `.fixedSize()` のラベルなど、メンバーカードの中身を実際に描画した状態でだけ再現しうる
    /// はみ出しを見る(メンバー未追加の `testTeamScreensNoHorizontalOverflowAtAX5` では検査できない)。
    func testTeamEditScreenWithMemberNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openTeamListScreen(app)
        createTeamAndOpenEditScreen(app)

        let addMemberButton = element(app, "addMemberButton")
        XCTAssertTrue(addMemberButton.waitForExistence(timeout: Self.existenceTimeout))
        addMemberButton.tap()
        // issue #68: 種族は検索シート経由で選ぶ(`Menu` ではなくなった)。
        XCTAssertTrue(element(app, "speciesSearchSheet").waitForExistence(timeout: Self.existenceTimeout))
        let speciesOption = app.buttons[Self.mockMemberSpeciesName]
        XCTAssertTrue(speciesOption.waitForExistence(timeout: Self.existenceTimeout))
        speciesOption.tap()

        let memberCard = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "memberCard-")).firstMatch
        XCTAssertTrue(memberCard.waitForExistence(timeout: Self.existenceTimeout))

        assertNoHorizontalOverflow(app, identifiers: Self.teamEditIdentifiers)
        assertNoHorizontalOverflowForPrefixes(app, prefixes: Self.teamEditMemberCardPrefixes)
    }
}
