import Foundation
import XCTest

/// G-05 第 4 弾(ADR-0540): 調整画面の View ファイルが守る決まりのソース検査(docs/design.md「動きの決まり」「見やすさの決まり」
/// 「iOS の追い方(G-05)」)。`G05TeamSourceRulesTests`(第 2 弾)と同じ作り。UITest では見えない決まり(常時動く表示・行数の制限・
/// 色のリテラル・アイコンの割り当ての迂回・視差効果)と、作り直しの構造(部品の使い方)をファイルの文字列で確かめる。
/// 対象は `ios/PokeCalc/` のファイル名(無ければ失敗する。消したり移したら、この表も同じ変更で直す)。
/// 技のシート(`MasterSearchSheet.swift`)とランクのメーターは、調整では変えない(閉じるのアイコン化・検索中の静止表示は第 3 弾=逆算の変更に従う)。
final class G05AdjustSourceRulesTests: XCTestCase {

    private static let files = [
        "AdjustScreenView.swift", "AdjustScreenCards.swift", "AdjustScreenGoals.swift", "AdjustScreenParts.swift",
        "AdjustScreenResults.swift",
    ]

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
                text.range(of: pattern, options: .regularExpression), "\(name): \(reason)(\(pattern))", file: file, line: line)
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

    private func assertMatches(
        _ name: String, _ pattern: String, _ reason: String, file: StaticString = #filePath, line: UInt = #line
    ) throws {
        XCTAssertNotNil(
            try code(name).range(of: pattern, options: .regularExpression), "\(name): \(reason)(\(pattern))", file: file, line: line)
    }

    // MARK: - 全ファイル共通の決まり

    func testNoAlwaysOnAnimation() throws {
        try assertNoMatch(#"ProgressView\("#, "回り続ける読み込み表示を置かない(PopNoticeView(.loading) の静止したアイコン + 一言)")
        try assertNoMatch(#"repeatForever|\.symbolEffect\(\.(pulse|variableColor|bounce|wiggle|breathe|rotate)"#, "常時動く演出を置かない")
        try assertNoMatch(#"TimelineView"#, "時間で動く表示を置かない")
    }

    func testNoLineLimitOrShrinking() throws {
        try assertNoMatch(#"\.lineLimit\("#, "lineLimit を付けない(全文を読める。AX5 でも欠けない)")
        try assertNoMatch(#"\.minimumScaleFactor\("#, "文字を縮めて収めない(目標の確率のピルも)")
    }

    func testColorsComeFromTokensOnly() throws {
        try assertNoMatch(#"Color\((red|white|black|hue|\.sRGB|\.display)"#, "色のリテラルを使わない")
        try assertNoMatch(
            #"#colorLiteral|UIColor\(|Color\.(red|blue|green|orange|yellow|gray|pink|purple|white|black)\b"#,
            "システム色・リテラルを使わない")
        try assertNoMatch(
            #"\.(foregroundStyle|foregroundColor|tint|background|fill|stroke)\(\s*\.(red|blue|green|orange|yellow|gray|pink|purple|white|black|primary|secondary|accentColor)\b"#,
            "システム色を直接指定しない")
    }

    /// アイコンは `PopSymbol` 経由(1 か所の割り当て)。SF Symbols の名前を View に直書きしない。
    func testSymbolsAreAssignedThroughPopSymbol() throws {
        try assertNoMatch(#"(systemName|systemImage|symbol):\s*""#, "SF Symbols の名前を直書きしない(PopSymbol に割り当てる)")
        try assertNoMatch(#"Image\(systemName:"#, "アイコンは PopIcon(PopSymbol.…)で出す")
        try assertNoMatch(
            #""(chevron|arrow|xmark|plus|minus|trash|pencil|exclamationmark|checkmark|questionmark|largecircle)[A-Za-z0-9.]*""#,
            "SF Symbols の名前らしい文字列を置かない")
        try assertNoMatch(#""circle(\.fill)?""#, "選択の丸(circle)を直書きしない")
    }

    /// 押下の縮み・動きは「視差効果を減らす」で 0 秒(`PillButtonStyle` と同じ作法)。
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

    // MARK: - 見出しのアイコン(Web の Icon と同じ割り当て)

    func testRegionHeadingsUseWebIcons() throws {
        // 自分 = 構築(team)、調整の内容 = 調整、相手 = 入れ替え、目標 = 判定(盾)、結果 = 計算、覚えるポケモン = 検索。
        try assertContains("AdjustScreenCards.swift", ["PopSymbol.team", "PopSymbol.adjust", "PopSymbol.swap", "PopSymbol.judge"])
        try assertContains("AdjustScreenGoals.swift", ["PopSymbol.judge"])
        try assertContains("AdjustScreenResults.swift", ["PopSymbol.calc", "PopSymbol.search"])
        // 全部の見出しが調整のアイコン(旧: AdjustCard が固定で PopSymbol.adjust)ではなくなる。
        try assertMatches("AdjustScreenParts.swift", #"struct AdjustCard[\s\S]*?systemImage:\s*String"#, "AdjustCard が見出しのアイコンを受け取る")
    }

    // MARK: - ポケモンカード(自分・相手・目標の相手)

    func testSpeciesRowsUsePokemonCard() throws {
        let card = try code("PopPokemonCard.swift")
        for needle in ["struct PopPokemonCard", "SpeciesImageView", "TypeBadgeView", "itemName"] {
            XCTAssertTrue(card.contains(needle), "PopPokemonCard.swift に \(needle) が無い")
        }
        try assertContains("AdjustScreenParts.swift", ["PopPokemonCard("])
        try assertContains("AdjustScreenCards.swift", ["ownItemMark", "opponentItemMark"])
        try assertContains("AdjustScreenGoals.swift", ["goalOpponentItemMark"])
        // 種族の欄の見た目を、計算・逆算と共有の `SpeciesHeaderMenuLabel`(lineLimit・縮小つき)に頼らない。
        try assertNotContains("AdjustScreenParts.swift", ["SpeciesHeaderMenuLabel"])
    }

    // MARK: - 増減ボタン(固定する能力ポイント・上限・発数)

    func testNumberFieldsHaveIconStepButtons() throws {
        try assertContains(
            "AdjustScreenParts.swift",
            [
                "PopIconButton(", "PopSymbol.minus", "PopSymbol.add", "AdjustNumberStep.", "AdjustText.stepperIncrease",
                "AdjustText.stepperDecrease", "keyboardType(.numberPad)",
            ])
        // 固定 SP の 6 欄に加え、上限は Menu の Picker(0...32 の 33 項目)をやめて増減ボタンにする。
        try assertNotContains("AdjustScreenCards.swift", ["ForEach(0...SPLimits.maxPerStat"])
        try assertContains("AdjustScreenCards.swift", ["AdjustText.ceilingLabel", "AdjustText.fixedSPLabel", "AdjustText.fixedSPTotal"])
        // 目標の発数の − / + は直書きのアイコンをやめて PopIconButton(読み上げ名・識別子は従来のまま)。
        try assertContains("AdjustScreenGoals.swift", ["PopIconButton(", "PopSymbol.minus", "PopSymbol.add", "AdjustText.hitsDecrease", "AdjustText.hitsIncrease"])
        try assertNotContains("AdjustScreenGoals.swift", ["func stepButton"])
    }

    // MARK: - 補足は「説明」ボタンの奥(1 カード 1 行まで)

    func testHintsMoveBehindHelpButtons() throws {
        try assertContains("AdjustScreenParts.swift", ["PopHelpButton(", "CommonPartsText.help", "PopSymbol.help"])
        // 固定 SP・最低素早さ・先に使う技の補足は説明ボタンの中(欄からの結びつけの accessibilityHint は残す)。
        for key in ["fixedSPHint", "minSpeedHint"] {
            let found = try Self.files.contains { try code($0).range(of: #"PopHelpButton\([\s\S]{0,400}"# + key, options: .regularExpression) != nil }
            XCTAssertTrue(found, "\(key) が説明ボタンの中にない")
        }
        try assertMatches("AdjustScreenGoals.swift", #"PopHelpButton\([\s\S]{0,400}goalBoostMoveHint"#, "先に使う技の補足が説明ボタンの中")
        try assertMatches("AdjustScreenResults.swift", #"PopHelpButton\([\s\S]{0,400}indexNote"#, "指数の注が説明ボタンの中")
        try assertMatches("AdjustScreenResults.swift", #"PopHelpButton\([\s\S]{0,400}minSpNotRequested"#, "最小の振り方の注が説明ボタンの中")
        // 旧: 常時出る caption の行。
        try assertNoMatch(
            #"AdjustResultLine\(text: AdjustText\.(indexNote|minSpNotRequested)|Text\(AdjustText\.goalBoostMoveHint\)"#, "補足を常時の文で出さない")
    }

    // MARK: - 状態の一言(案内の部品)

    func testStatesUseNoticeViews() throws {
        try assertContains("AdjustScreenView.swift", ["PopNoticeView(kind: .loading", "AdjustText.loadingNotice"])
        try assertContains(
            "AdjustScreenResults.swift",
            [
                "PopNoticeView(kind: .empty", "AdjustText.emptyResultNotice", "PopNoticeView(kind: .loading", "AdjustText.learnersLoading",
                "PopNoticeView(kind: .info", "PopNoticeView(kind: .error", "AdjustText.learnersEmpty",
            ])
        try assertContains("AdjustScreenGoals.swift", ["PopNoticeView(kind: .empty", "AdjustText.noGoalsNotice"])
        // 通信できないときの表示は変えない(ADR-0534): ErrorBannerView と「もう一度」。
        try assertContains("AdjustScreenView.swift", ["ErrorBannerView(", "LoadRetryButton("])
    }

    // MARK: - 結果: 結論のアイコンと横棒

    func testResultsShowVerdictIconsAndBars() throws {
        try assertContains(
            "AdjustScreenResults.swift", ["PopSymbol.done", "PopSymbol.close", "AdjustSPBar.fraction", "AdjustText.goalOutcomeLine"])
        // 結論の文(従来の文)は残す。アイコンは装飾(文が名前を持つ)。
        try assertContains("AdjustScreenResults.swift", ["AdjustText.koLine", "AdjustText.surviveLine", "AdjustText.goalLine"])
        try assertContains("AdjustScreenResults.swift", ["accessibilityHidden(true)"])
    }

    func testLearnersCloseIsIconOnly() throws {
        try assertContains("AdjustScreenResults.swift", ["PopIconButton(", "PopSymbol.close", "adjustLearnersClose", "AdjustText.learnersClose"])
        try assertNotContains("AdjustScreenResults.swift", ["textButton(AdjustText.learnersClose"])
    }

    // MARK: - 目標のカード

    func testGoalCardsUseKindIconsSegmentedKindAndRecentTiles() throws {
        try assertContains(
            "AdjustScreenGoals.swift",
            [
                "PopSymbol.speed", "PopSymbol.judge", "PopSymbol.calc", "PopPokemonCard(", "viewModel.recentOpponents",
                "AdjustText.goalRecentOpponents", "PopSymbol.add",
            ])
        // 種類は縦のラジオ(AdjustChoiceButton)でなく区切りボタン。選択中はチェック(PopSymbol.done)。識別子 adjustGoal-n-kind-<kind> は不変。
        try assertNotContains("AdjustScreenGoals.swift", ["AdjustChoiceButton("])
        try assertContains("AdjustScreenGoals.swift", ["-kind-", "PopSymbol.done"])
    }

    func testGoalMoveSheetHasIconCloseAndTypeBadgeRows() throws {
        try assertContains(
            "AdjustScreenGoals.swift",
            ["adjustGoalMoveSheetClose", "PopIconButton(", "PopSymbol.close", "TypeBadgeView(", "move.power", "adjustGoalMoveOption-"])
        try assertNotContains("AdjustScreenGoals.swift", ["Button(AdjustText.goalMoveSheetClose)"])
    }

    // MARK: - 選択肢(モード・耐久の基準・攻撃の分類)

    func testChoiceButtonsAreChipsWithoutRadioCircles() throws {
        try assertContains("AdjustScreenParts.swift", ["PopChipStyle", "PopSymbol.done"])
        try assertContains("AdjustScreenParts.swift", [".isSelected"])
    }

    // MARK: - 共通の部品

    func testSharedPartsExist() throws {
        try assertContains("PopComponents.swift", ["static let judge = PopSymbolNames.judge"])
        // 第 3 弾(逆算)の部品を再利用する(重複して作らない)。
        for part in ["struct PopHelpButton", "struct PopIconButton", "struct PopEmptyLabel", "struct PopNoticeView"] {
            XCTAssertTrue(try code("PopComponents.swift").contains(part), "PopComponents.swift に \(part) が無い")
        }
    }
}
