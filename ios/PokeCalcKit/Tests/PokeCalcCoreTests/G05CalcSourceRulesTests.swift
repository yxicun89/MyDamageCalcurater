import Foundation
import XCTest

/// G-05 第 5 弾(ADR-0541): 計算画面と連続技の節の View ファイルが守る決まりのソース検査(docs/design.md「動きの決まり」「見やすさの決まり」
/// 「iOS の追い方(G-05)」)。`G05TeamSourceRulesTests` と同じ作り。UITest では見えない決まり(常時動く表示・行数の制限・色のリテラル・
/// アイコンの割り当ての迂回・視差効果)と、作り直しの構造(部品の使い方)をファイルの文字列で確かめる。
/// 対象は `ios/PokeCalc/` のファイル名(無ければ失敗する。消したり移したら、この表も同じ変更で直す)。
/// ランクのメーター(`RankMeterView.swift`。ADR-0533)・検索シート(`MasterSearchSheet.swift`。逆算の弾 ADR-0539 が変える)は対象に入れない。
final class G05CalcSourceRulesTests: XCTestCase {

    private static let calcFiles = [
        "CalcScreenView.swift", "CalcScreenCards.swift", "CalcAttackerStatBlocksView.swift", "CalcConditionsSection.swift",
        "CalcScreenResults.swift", "CalcScreenStyleHelpers.swift",
    ]
    /// 連続技の節。機能・文言・識別子は変えない(ADR-0535)。見た目の部品の置き換えと規約の確認だけ。
    private static let comboFiles = ["ComboSectionView.swift", "ComboSectionParts.swift", "ComboResultView.swift"]
    private static var files: [String] { calcFiles + comboFiles }

    private func source(_ name: String) throws -> String {
        let ios = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent()
        let url = ios.appendingPathComponent("PokeCalc").appendingPathComponent(name)
        guard FileManager.default.fileExists(atPath: url.path) else {
            XCTFail("\(url.path) が無い")
            return ""
        }
        return try String(contentsOf: url, encoding: .utf8)
    }

    /// `//` で始まる行(説明のコメント)は検査から外す。
    private func code(_ name: String) throws -> String {
        try source(name).split(separator: "\n", omittingEmptySubsequences: false)
            .filter { !$0.trimmingCharacters(in: .whitespaces).hasPrefix("//") }
            .joined(separator: "\n")
    }

    private func assertNoMatch(
        _ pattern: String, _ reason: String, file: StaticString = #filePath, line: UInt = #line
    ) throws {
        for name in Self.files {
            let text = try code(name)
            XCTAssertNil(
                text.range(of: pattern, options: .regularExpression), "\(name): \(reason)(\(pattern))",
                file: file, line: line)
        }
    }

    private func assertContains(
        _ name: String, _ needles: [String], file: StaticString = #filePath, line: UInt = #line
    ) throws {
        let text = try code(name)
        for needle in needles {
            XCTAssertTrue(text.contains(needle), "\(name) に \(needle) が無い", file: file, line: line)
        }
    }

    private func assertNotContains(
        _ name: String, _ needles: [String], file: StaticString = #filePath, line: UInt = #line
    ) throws {
        let text = try code(name)
        for needle in needles {
            XCTAssertFalse(text.contains(needle), "\(name) に \(needle) が残っている", file: file, line: line)
        }
    }

    // MARK: - 全ファイル共通の決まり

    func testNoAlwaysOnAnimation() throws {
        try assertNoMatch(#"ProgressView\("#, "回り続ける読み込み表示を置かない(PopNoticeView(.loading) の静止したアイコン + 一言)")
        try assertNoMatch(#"repeatForever|\.symbolEffect\(\.(pulse|variableColor|bounce|wiggle|breathe|rotate)"#, "常時動く演出を置かない")
        try assertNoMatch(#"TimelineView"#, "時間で動く表示を置かない")
    }

    func testNoLineLimit() throws {
        try assertNoMatch(#"\.lineLimit\("#, "lineLimit を付けない(全文を読める。AX5 でも欠けない)")
        try assertNoMatch(#"\.minimumScaleFactor\("#, "文字を縮めて収めない")
    }

    func testColorsComeFromTokensOnly() throws {
        try assertNoMatch(#"Color\((red|white|black|hue|\.sRGB|\.display)"#, "色のリテラルを使わない")
        try assertNoMatch(
            #"#colorLiteral|UIColor\(|Color\.(red|blue|green|orange|yellow|gray|pink|purple|white|black)\b"#,
            "システム色・リテラルを使わない")
        try assertNoMatch(
            #"\.(foregroundStyle|foregroundColor|tint|background|fill|stroke)\(\s*\.(red|blue|green|orange|yellow|gray|pink|purple|white|black|primary|secondary|accentColor)\b"#,
            "システム色を直接指定しない")
        try assertNoMatch(
            #"\?\?\s*\.(red|blue|green|orange|yellow|gray|pink|purple|white|black)\b"#,
            "トークンの代わりにシステム色へ落とさない(ColorToken.onPrimary など)")
    }

    /// アイコンは `PopSymbol`(1 か所の割り当て)経由。SF Symbols の名前を View に直書きしない。
    func testSymbolsAreAssignedThroughPopSymbol() throws {
        try assertNoMatch(#"(systemName|systemImage|symbol):\s*""#, "SF Symbols の名前を直書きしない(PopSymbol に割り当てる)")
        try assertNoMatch(#"Image\(systemName:"#, "アイコンは PopIcon(PopSymbol.…)で出す")
        try assertNoMatch(
            #""(chevron|arrow|xmark|plus|minus|trash|pencil|exclamationmark|checkmark|questionmark|circle)[A-Za-z0-9.]*""#,
            "SF Symbols の名前らしい文字列を置かない")
    }

    /// 押下の縮み・動きは「視差効果を減らす」で 0 秒。
    func testMotionRespectsReduceMotion() throws {
        for name in Self.files {
            let text = try code(name)
            if text.contains(".scaleEffect(") || text.contains(".animation(") || text.contains("withAnimation") {
                XCTAssertTrue(
                    text.contains("accessibilityReduceMotion") || text.contains("MotionToken"),
                    "\(name): 動きは視差効果を減らすで 0 秒(@Environment(\\.accessibilityReduceMotion) か MotionToken)")
            }
        }
    }

    // MARK: - 攻撃側・防御側のカード

    /// 見出し(「攻撃側」「防御側」)はカードの外・上。`cardsRow` に置く(カードの入れ替えの動き〈`.id(speciesKey)`〉に巻き込まない)。
    func testSideHeadingsAboveTheCards() throws {
        try assertContains(
            "CalcScreenView.swift",
            [
                "PopHeading(", "CalcScreenText.attackerHeading", "CalcScreenText.defenderHeading", "calcAttackerHeading", "calcDefenderHeading",
                "PopSymbol.damage", "PopSymbol.judge",
            ])
    }

    /// カードの顔は調整の弾で新設する `PopPokemonCard`(画像かエンブレム + 名前 + タイプバッジ + 持ち物の印)。
    /// 共有の `SpeciesHeaderMenuLabel`(逆算・構築が使う)は残し、矢印の直書きと lineLimit だけ直す(逆算の弾から送られた宿題)。
    func testCardsUsePokemonCardWithItemMark() throws {
        try assertContains(
            "CalcScreenCards.swift",
            ["PopPokemonCard(", "attackerItemMark", "defenderItemMark", "struct SpeciesHeaderMenuLabel", "PopSymbol.picker"])
        try assertNotContains("CalcScreenCards.swift", ["SpeciesHeaderMenuLabel(species: species)"])
    }

    /// 特性の欄は「詳細」の中ではなくカードの中(Web のカードは ポケモン・持ち物・特性 の順)。識別子は不変。
    func testAbilityPickersLiveInTheCards() throws {
        try assertContains("CalcScreenCards.swift", ["calcAttackerAbilityPicker", "calcDefenderAbilityPicker"])
        try assertNotContains(
            "CalcConditionsSection.swift", ["calcAttackerAbilityPicker", "calcDefenderAbilityPicker", "abilitySection", "defenderAbilitySection"])
    }

    /// 持ち物の上限の注記は 状態の一言(アイコン + 文字)。識別子は不変。
    func testItemLimitHintIsANotice() throws {
        try assertContains("CalcScreenCards.swift", ["PopNoticeView(kind: .info", "attackerItemLimitHint"])
    }

    // MARK: - 攻守入れ替え・技の欄・プリセット・読み込み中

    func testSwapIsAnIconButtonAndLoadingIsStatic() throws {
        try assertContains(
            "CalcScreenView.swift",
            [
                "PopIconButton(", "PopSymbol.swap", "swapSidesButton", "PopNoticeView(kind: .loading", "CalcScreenText.loading",
                "calcLoadingIndicator",
            ])
        try assertNotContains("CalcScreenView.swift", ["ProgressView(", "arrow.left.arrow.right"])
    }

    /// 技の欄は Web のトリガーと同じ並び(タイプバッジ + 技名 + 分類のアイコン + 威力)。要約の文(「威力37 / 物理 / ばつぐん(×2)」)は
    /// 見える文字にせず読み上げの値へ、相性は結果の先頭の枠だけのバッジへ移す。識別子 `movePicker` は不変。
    func testMoveSelectorShowsBadgeNameCategoryAndPower() throws {
        try assertContains(
            "CalcScreenView.swift",
            ["TypeBadgeView(", "PopSymbol.moveCategory", "PopSymbol.picker", "accessibilityValue(viewModel.moveSummaryText)", "movePicker"])
        try assertNotContains("CalcScreenView.swift", ["Text(viewModel.moveSummaryText)", "chevron.up.chevron.down"])
    }

    /// 選択中の印は色だけに頼らない(塗り + チェック)。プリセットのピルにもチェック。
    func testPresetPillsShowACheckWhenSelected() throws {
        try assertContains("CalcScreenView.swift", ["PopSymbol.done", "attackerPreset-"])
    }

    // MARK: - 攻撃・特攻のブロック

    func testStatBlocksHaveSPButtonsAndNotices() throws {
        try assertContains(
            "CalcAttackerStatBlocksView.swift",
            [
                "PopIconButton(", "PopSymbol.minus", "PopSymbol.add", "AttackerSPStep.canDecrease", "AttackerSPStep.canIncrease",
                "AttackerSPStep.stepped", "AttackerStatLabels.spDecrease", "AttackerStatLabels.spIncrease",
                "attackerSPDecrease-", "attackerSPIncrease-", "attackerSPField-", "keyboardType(.numberPad)",
            ])
        // 性格補正の 3 択は選択中に塗り + チェック。SP の誤りと各案内は アイコン + 一言(識別子は文に付けて不変)。
        try assertContains(
            "CalcAttackerStatBlocksView.swift",
            [
                "PopSymbol.done", "PopNoticeView(kind: .error", "PopNoticeView(kind: .info", "attackerSPError-", "attackerStatTeamNotice",
                "calcStatusMoveNotice", "calcNoDamagingMovesNotice", "attackerNature-",
            ])
    }

    // MARK: - 結果

    func testResultRowHasBigNumbersFilledKOBadgeAndSmallSupplements() throws {
        try assertContains(
            "CalcScreenResults.swift",
            [
                "TextStyleToken.resultPercent", "ColorToken.brandPrimary.color", "ColorToken.onPrimary.color", "calcResultKO-",
                "calcResultPercent-", "calcResultPreset-", "calcResultAbility-",
            ])
    }

    func testEffectivenessIsAnOutlinedBadgeOnceAboveTheRows() throws {
        try assertContains(
            "CalcScreenResults.swift", ["calcEffectivenessBadge", "EffectivenessLabel.text", "viewModel.moveEffectiveness"])
        try assertContains("CalcScreenResults.swift", ["CalcScreenText.resultsListLabel", "calcResults"])
    }

    func testResultNoticesAreStateNotices() throws {
        try assertContains(
            "CalcScreenResults.swift",
            ["PopNoticeView(", "calcUnsupportedNotice", "calcItemVariantLimitHint", "defenderItemCompareLockReason"])
    }

    /// チップは選択中に塗り + チェック、36pt 以上、折り返す並び(横スクロールにしない)。
    func testChipsShowACheckAndWrap() throws {
        try assertContains(
            "CalcScreenResults.swift",
            ["struct ChipButton", "PopSymbol.done", "minHeight: CalcScreenMetrics.minimumTapSide", "ComboWrapLayout"])
        try assertNotContains("CalcScreenResults.swift", ["ScrollView(.horizontal"])
    }

    // MARK: - 詳細

    func testConditionsToggleIsIconAndNameAndChipsWrap() throws {
        try assertContains(
            "CalcConditionsSection.swift",
            [
                "PopSymbol.open", "calcConditionsToggle", "accessibilityValue", "CalcScreenText.disclosureOpened",
                "CalcScreenText.disclosureClosed", "ComboWrapLayout", "RankMeterView(",
            ])
        try assertNotContains("CalcConditionsSection.swift", ["ScrollView(.horizontal", "chevron.up", "chevron.down"])
    }

    // MARK: - 連続技の節(機能・文言・識別子は不変。見た目の部品だけ)

    func testComboSectionUsesSharedPopParts() throws {
        try assertContains(
            "ComboSectionView.swift",
            ["PopIconButton(", "PopSymbol.up", "PopSymbol.down", "PopSymbol.close", "PopSymbol.add", "PopSymbol.picker", "PopSymbol.open"])
        try assertNotContains("ComboSectionView.swift", ["ComboIconButton("])
        try assertNotContains("ComboSectionParts.swift", ["struct ComboIconButton"])
        try assertContains("ComboSectionParts.swift", ["PopIcon(", "PopSymbol.done", "PopSymbol.up", "PopSymbol.down", "struct ComboWrapLayout"])
        try assertContains("ComboResultView.swift", ["PopNoticeView(kind: .loading", "comboLoading", "ComboText.loading", "comboError"])
    }

    // MARK: - 共通の部品(足りない分だけ足す)

    func testPopSymbolGainsMoveCategory() throws {
        try assertContains("PopComponents.swift", ["static func moveCategory", "figure.boxing", "sparkles", "circle.dashed"])
    }
}
