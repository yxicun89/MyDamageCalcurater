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
///
/// P6-15(ADR-0501「P6-15」): P6-14 の残り3点を追加する。
/// 1. AX5 で攻撃側プリセットのピル(`A振り(無補正)` 等)が「A振り…」と省略される不具合
///    (`CalcScreenView.presetSegmentedRow`・`ReverseScreenView.presetSegmentedRow`/`PresetPillButton`
///    〈`.attacker` 側の `KnownDefenderPreset` のピルも含む〉)。XCUITest は文字が省略記号で切れて
///    いるかどうかを直接読めないため、「アクセシビリティの文字サイズではピルを縦に積み、各ピルが
///    画面幅いっぱいに近い幅を持つ」という直し方(`cardsRow` と同じ縦積みパターン)が効いているかを
///    `assertPresetPillsStackVertically` で間接的に確かめる(pill の `minY` が互いに離れている・
///    幅がウィンドウ幅の半分を超える)。既定サイズでは今までどおり1行のままであることも
///    `assertPresetPillsSingleRow` で確かめる(回帰確認)。
/// 2. AX5 で計算画面の「詳細」(`calcConditionsToggle`)を開いた状態でも横にはみ出さないこと
///    (issue #274・ADR-0501「issue #274」6章の identifier を検査する)。
/// 3. 既定サイズで%表示(`calcResultPercent-*`)が `minimumScaleFactor` によって不要に縮んでいない
///    こと。`testCalcScreenResultPercentNotShrunkAtDefaultSize` のコメント参照。
@MainActor
final class LargeTextLayoutUITests: XCTestCase {
    private static let existenceTimeout: TimeInterval = 5
    /// AX5(アクセシビリティ域の最大)。`UICTContentSizeCategoryAccessibilityXXXL` が
    /// `UIContentSizeCategory.accessibilityExtraExtraExtraLarge` に対応する内部識別子。
    private static let ax5ContentSizeCategory = "UICTContentSizeCategoryAccessibilityXXXL"
    /// フォントのサブピクセル丸めを許容する程度の小さな余裕(pt)。
    private static let overflowTolerance: CGFloat = 1
    /// P6-15 (3): 縦向き/横向きでの%表示の高さ比較の許容誤差(pt)。回転に伴う描画の
    /// サブピクセル差を吸収する程度(`overflowTolerance` より少し広めに取る)。
    private static let percentHeightTolerance: CGFloat = 3

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

    // MARK: - P6-15 (1): プリセットのピルの縦積み(ADR-0501「P6-15」1章)

    /// アクセシビリティの文字サイズでピルが縦に積まれている(= 3等分の1行をやめている)ことを、
    /// 次の2点で間接的に確かめる。XCUITest は `Text` が省略記号(`…`)で切れているかどうかを
    /// 直接読めない(`.label` は元の文字列のままで、見た目の省略は反映されない)ため、
    /// 「積んだ結果、各ピルが画面幅いっぱいに近い幅を持つ」ことを省略が解消した代理指標として使う
    /// (P6-15 タスク指示: 「pick a robust, meaningful assertion and explain」)。
    /// 1. 3つのピルの `minY` が互いに大きく離れている(横1行になっていない)
    /// 2. 各ピルの幅がウィンドウ幅の半分より広い(3等分〈おおよそ1/3〉のままではない)
    private func assertPresetPillsStackVertically(_ app: XCUIApplication, identifiers: [String], file: StaticString = #filePath, line: UInt = #line) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)

        let elements = identifiers.map { element(app, $0) }
        for (identifier, el) in zip(identifiers, elements) {
            XCTAssertTrue(el.waitForExistence(timeout: Self.existenceTimeout), "ピルが見つからない: \(identifier)", file: file, line: line)
        }
        let frames = zip(identifiers, elements.map(\.frame))

        let sortedMinYs = frames.map(\.1.minY).sorted()
        for i in 1..<sortedMinYs.count {
            XCTAssertGreaterThan(
                sortedMinYs[i] - sortedMinYs[i - 1], 10,
                "AX5 でピルが縦に積まれていない(横1行のままに見える): minYs=\(sortedMinYs) identifiers=\(identifiers)",
                file: file, line: line
            )
        }
        for (identifier, frame) in frames {
            XCTAssertGreaterThan(
                frame.width, windowFrame.width / 2,
                "AX5 でピル \(identifier) の幅が画面の半分以下(3等分のまま = 省略が直っていない疑い): "
                    + "frame=\(frame) window=\(windowFrame)",
                file: file, line: line
            )
        }
    }

    /// 既定の文字サイズでは今までどおり3等分の1行のままであること(回帰確認)。
    private func assertPresetPillsSingleRow(_ app: XCUIApplication, identifiers: [String], file: StaticString = #filePath, line: UInt = #line) {
        let windowFrame = app.windows.firstMatch.frame
        XCTAssertGreaterThan(windowFrame.width, 0, "ウィンドウの frame が取得できない", file: file, line: line)

        let elements = identifiers.map { element(app, $0) }
        for (identifier, el) in zip(identifiers, elements) {
            XCTAssertTrue(el.waitForExistence(timeout: Self.existenceTimeout), "ピルが見つからない: \(identifier)", file: file, line: line)
        }
        let frames = zip(identifiers, elements.map(\.frame))
        let minYs = frames.map(\.1.minY)
        XCTAssertLessThan(
            (minYs.max() ?? 0) - (minYs.min() ?? 0), Self.overflowTolerance * 2,
            "既定サイズでピルが横1行になっていない(回帰): minYs=\(minYs) identifiers=\(identifiers)",
            file: file, line: line
        )
        for (identifier, frame) in frames {
            XCTAssertLessThan(
                frame.width, windowFrame.width / 2,
                "既定サイズでピル \(identifier) の幅が画面の半分を超えている(3等分でなくなった回帰疑い): "
                    + "frame=\(frame) window=\(windowFrame)",
                file: file, line: line
            )
        }
    }

    // MARK: - P6-15 (2): AX5で「詳細」パネルを開いた状態のはみ出し(issue #274・ADR-0501「issue #274」6章)

    /// issue #274 6章の identifier 表のうち、rawValue が個体・種族に依存しない固定の要素
    /// (`calcConditionsPanel` 自体・急所/やけど・ランクの値と±・特性の `Menu`)。
    private static let calcConditionsPanelIdentifiers = [
        "calcConditionsPanel",
        "calcCondition-critical",
        "calcCondition-burn",
        "calcAttackerRankValue",
        "calcAttackerRankDecrement",
        "calcAttackerRankIncrement",
        "calcAttackerAbilityPicker",
    ]

    /// 天候・フィールド・防御側の壁(`calcWeather-*`/`calcTerrain-*`/`calcDefenderScreen-*`)は
    /// `ChipButton.swift` のコメントどおり **横スクロールの `ScrollView(.horizontal)` の中**にある
    /// (`CalcConditionsSection.weatherSection`/`terrainSection`/`defenderScreensSection`。1行に
    /// 全部並べる幅が無いための意図的な設計)。横スクロールの中身は「スクロールすれば見える」もので
    /// あり、`XCUIElement.frame` がウィンドウ幅を超えるのは不具合ではなく想定どおりの挙動
    /// (実際、最初の実行でこれらを `assertNoHorizontalOverflowForPrefixes` に含めたところ、
    /// 2つ目以降のチップが `rightCut=true` になり誤検知した。`itemComparisonToggles` の
    /// `defenderItemToggle-*` が既存の `calcScreenIdentifiers` に入っていないのと同じ理由で対象外にする)。
    /// このため、はみ出し検査ではなく「(少なくとも1件は)存在する」ことだけ確かめる。
    private static let calcConditionsPanelScrollableChipPrefixes = [
        "calcWeather-",
        "calcTerrain-",
        "calcDefenderScreen-",
    ]

    /// AX5 は画面が縦に長くなるため、既定サイズ用の `CalcConditionsUITests.scrollUntilHittable`
    /// より多くスクロールが要る場合がある(上限を増やしただけの同じ考え方)。
    private func scrollUntilHittable(_ app: XCUIApplication, _ target: XCUIElement, containerIdentifier: String, maxAttempts: Int = 12, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(target.waitForExistence(timeout: Self.existenceTimeout), "要素が見つからない: \(target)", file: file, line: line)
        var attempts = 0
        while !target.isHittable && attempts < maxAttempts {
            element(app, containerIdentifier).swipeUp()
            attempts += 1
        }
        XCTAssertTrue(target.isHittable, "スクロールしてもタップできない: \(target)", file: file, line: line)
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

    /// `CalcScreenView.presetSegmentedRow` の3つのピル(`AttackerPreset.allCases` の順)。
    private static let calcAttackerPresetPillIdentifiers = [
        "attackerPreset-none", "attackerPreset-aFull", "attackerPreset-aMax",
    ]

    /// P6-15 (1) 本体: AX5 で攻撃側プリセットのピルが縦に積まれ、省略されにくい幅になっていること。
    /// 現時点(`presetSegmentedRow` が `dynamicTypeSize` を見ていない)では失敗してよい。
    func testCalcScreenAttackerPresetPillsStackVerticallyAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openCalcScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "attackerCard", defenderIdentifier: "defenderCard")
        assertPresetPillsStackVertically(app, identifiers: Self.calcAttackerPresetPillIdentifiers)
    }

    /// P6-15 (1) の回帰確認: 既定サイズでは今までどおり3等分の1行のまま。
    func testCalcScreenAttackerPresetPillsSingleRowAtDefaultSize() {
        let app = launchWithMock()
        openCalcScreen(app)
        assertPresetPillsSingleRow(app, identifiers: Self.calcAttackerPresetPillIdentifiers)
    }

    /// P6-15 (2): AX5 で計算画面の「詳細」(`calcConditionsToggle`)を開いた状態でも横にはみ出さないこと
    /// (issue #274・ADR-0501「issue #274」6章の identifier を検査する)。
    func testCalcScreenConditionsPanelNoHorizontalOverflowAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openCalcScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "attackerCard", defenderIdentifier: "defenderCard")

        let toggle = element(app, "calcConditionsToggle")
        scrollUntilHittable(app, toggle, containerIdentifier: "calcScreen")
        toggle.tap()
        XCTAssertTrue(element(app, "calcConditionsPanel").waitForExistence(timeout: Self.existenceTimeout))

        assertNoHorizontalOverflow(app, identifiers: Self.calcScreenIdentifiers + Self.calcConditionsPanelIdentifiers)
        // 横スクロールの中の天候・フィールド・防御側の壁は、存在(少なくとも1件描画されている)だけ確かめる
        // (`calcConditionsPanelScrollableChipPrefixes` のコメント参照。はみ出し検査の対象外)。
        for prefix in Self.calcConditionsPanelScrollableChipPrefixes {
            let firstMatch = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", prefix)).firstMatch
            XCTAssertTrue(firstMatch.waitForExistence(timeout: Self.existenceTimeout), "要素が見つからない(前方一致): \(prefix)")
        }
    }

    /// P6-15 (3): 既定サイズで%表示(`calcResultPercent-*`)が `minimumScaleFactor` によって
    /// 不要に縮んでいないこと。`ResultRowView.percentRangeTextView` は `.lineLimit(1) +
    /// .minimumScaleFactor(0.7)`(`CalcScreenMetrics.compactMinimumScaleFactor`)を使っている
    /// (ADR-0501「P6-14」6章)。既定サイズでは縮まないはずだが、レイアウトの変更で意図せず
    /// 縮んでしまっても `.label`(元の文字列のまま)からは気づけない。
    ///
    /// フォントの実測 pt 値をハードコードする代わりに、「横幅にまったく制約が無い横向き
    /// (landscape)」での同じ要素の高さを基準値として使う(P6-15 タスク指示「同じ文字列を、
    /// 確実に縮まない広い文脈で比較する」)。横向きでは十分な幅があるため `minimumScaleFactor` が
    /// 働く理由が無く、縦向き(既定)の高さがそれより明確に低ければ、既定サイズなのに縮小されている
    /// (回帰)と判定できる。iPhone は Info.plist(`INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone`)
    /// で横向きを許可しているので、この比較が成立する。
    func testCalcScreenResultPercentNotShrunkAtDefaultSize() {
        let app = launchWithMock()
        openCalcScreen(app)

        let identifier = "calcResultPercent-none@-"
        let percent = element(app, identifier)
        XCTAssertTrue(percent.waitForExistence(timeout: Self.existenceTimeout), "要素が見つからない: \(identifier)")
        let portraitWindowWidth = app.windows.firstMatch.frame.width
        let portraitHeight = percent.frame.height
        XCTAssertGreaterThan(portraitHeight, 0, "\(identifier) の高さが取得できない")

        addTeardownBlock { XCUIDevice.shared.orientation = .portrait }
        XCUIDevice.shared.orientation = .landscapeLeft
        waitForWindowWidthChange(app, from: portraitWindowWidth)

        let percentLandscape = element(app, identifier)
        XCTAssertTrue(percentLandscape.waitForExistence(timeout: Self.existenceTimeout), "横向きで要素が見つからない: \(identifier)")
        let landscapeHeight = percentLandscape.frame.height
        XCTAssertGreaterThan(landscapeHeight, 0, "横向きで \(identifier) の高さが取得できない")

        XCTAssertEqual(
            portraitHeight, landscapeHeight, accuracy: Self.percentHeightTolerance,
            "既定サイズ(縦向き)の%表示が、幅に制約の無い横向きより縮んでいる"
                + "(portrait=\(portraitHeight) landscape=\(landscapeHeight))。"
                + "minimumScaleFactor によって不要に縮小されている可能性がある"
        )
    }

    /// 端末の向きを変えた後、ウィンドウ幅が実際に変わる(=回転が反映された)まで待つ
    /// (アニメーション中の frame を読んで誤判定しないため)。
    private func waitForWindowWidthChange(_ app: XCUIApplication, from originalWidth: CGFloat, timeout: TimeInterval = 5) {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if abs(app.windows.firstMatch.frame.width - originalWidth) > 1 { return }
            usleep(100_000)
        }
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

    /// `ReverseScreenView.presetSegmentedRow`(`.defender` 側 = 起動直後の既定。`AttackerPreset`)の
    /// 3つのピル。
    private static let reverseAttackerPresetPillIdentifiers = [
        "reverseAttackerPreset-none", "reverseAttackerPreset-aFull", "reverseAttackerPreset-aMax",
    ]

    /// 同(`.attacker` 側。`KnownDefenderPreset.allCases` の順)の3つのピル。ラベルは
    /// `AttackerPreset` より短い(「HB振り」等)が、同じ `PresetPillButton` を使っているため
    /// 同じ直し方(縦積み)が適用されるはず(P6-15 タスク指示)。
    private static let reverseKnownDefenderPresetPillIdentifiers = [
        "reverseKnownDefenderPreset-none", "reverseKnownDefenderPreset-max", "reverseKnownDefenderPreset-full",
    ]

    /// P6-15 (1): 逆算画面「与えたダメージ」側(既定 = `.defender`)の `AttackerPreset` ピルが
    /// AX5 で縦に積まれること。現時点では失敗してよい。
    func testReverseScreenAttackerPresetPillsStackVerticallyAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openReverseScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "reverseMyCard", defenderIdentifier: "reverseOpponentCard")
        assertPresetPillsStackVertically(app, identifiers: Self.reverseAttackerPresetPillIdentifiers)
    }

    /// 同、既定サイズでの回帰確認。
    func testReverseScreenAttackerPresetPillsSingleRowAtDefaultSize() {
        let app = launchWithMock()
        openReverseScreen(app)
        assertPresetPillsSingleRow(app, identifiers: Self.reverseAttackerPresetPillIdentifiers)
    }

    /// P6-15 (1): 逆算画面「受けたダメージ」側(`.attacker`。`reverseSide-attacker` に切り替える)の
    /// `KnownDefenderPreset` ピルも同様に AX5 で縦に積まれること。
    func testReverseScreenKnownDefenderPresetPillsStackVerticallyAtAX5() {
        let app = launchWithMock(contentSizeCategory: Self.ax5ContentSizeCategory)
        openReverseScreen(app)
        assertAX5TookEffect(app, attackerIdentifier: "reverseMyCard", defenderIdentifier: "reverseOpponentCard")

        let attackerSide = element(app, "reverseSide-attacker")
        XCTAssertTrue(attackerSide.waitForExistence(timeout: Self.existenceTimeout))
        attackerSide.tap()

        assertPresetPillsStackVertically(app, identifiers: Self.reverseKnownDefenderPresetPillIdentifiers)
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
