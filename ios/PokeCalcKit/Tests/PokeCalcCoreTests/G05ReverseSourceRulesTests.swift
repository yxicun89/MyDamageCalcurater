import Foundation
import XCTest

/// G-05 第 3 弾(ADR-0539): 逆算画面の View ファイルが守る決まりのソース検査(docs/design.md「動きの決まり」「見やすさの決まり」
/// 「iOS の追い方(G-05)」)。`G05TeamSourceRulesTests`(第 2 弾)と同じ作り。UITest では見えない決まり(常時動く表示・行数の制限・
/// 色のリテラル・アイコンの割り当ての迂回・視差効果)と、作り直しの構造(部品の使い方)をファイルの文字列で確かめる。
/// 対象は `ios/PokeCalc/` のファイル名(無ければ失敗する。消したり移したら、この表も同じ変更で直す)。
/// 共有の種族ピッカーの見出し(`SpeciesHeaderMenuLabel`。計算・調整と共用)・ランクのメーターは変えないので対象に入れない。
final class G05ReverseSourceRulesTests: XCTestCase {

    private static let files = [
        "ReverseScreenView.swift", "ReverseScreenCards.swift", "ReverseScreenObservations.swift", "ReverseScreenResults.swift",
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
        _ pattern: String, _ reason: String, in names: [String] = G05ReverseSourceRulesTests.files,
        file: StaticString = #filePath, line: UInt = #line
    ) throws {
        for name in names {
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
    }

    /// アイコンは `PopSymbol` 経由(1 か所の割り当て)。SF Symbols の名前を View に直書きしない。
    func testSymbolsAreAssignedThroughPopSymbol() throws {
        try assertNoMatch(#"(systemName|systemImage|symbol):\s*""#, "SF Symbols の名前を直書きしない(PopSymbol に割り当てる)")
        try assertNoMatch(#"Image\(systemName:"#, "アイコンは PopIcon(PopSymbol.…)で出す")
        try assertNoMatch(
            #""(chevron|arrow|xmark|plus|minus|trash|pencil|exclamationmark|checkmark|questionmark)[A-Za-z0-9.]*""#,
            "SF Symbols の名前らしい文字列を置かない")
    }

    /// 押下の縮み・動き(候補が絞られる切り替え)は「視差効果を減らす」で 0 秒・なし。
    func testMotionRespectsReduceMotion() throws {
        for name in Self.files {
            let text = try code(name)
            if text.contains(".scaleEffect(") || text.contains(".animation(") || text.contains("withAnimation") {
                XCTAssertTrue(
                    text.contains("accessibilityReduceMotion") || text.contains("MotionToken"),
                    "\(name): 動きは視差効果を減らすで 0 秒(@Environment(\\.accessibilityReduceMotion) か MotionToken)")
            }
        }
        // 候補が絞られる切り替えは、操作したときだけ・視差効果を減らすでなし(既存の挙動を保つ)。
        try assertContains("ReverseScreenResults.swift", ["accessibilityReduceMotion", "reduceMotion"])
    }

    // MARK: - 自分・相手のカード

    /// 見出し(「自分」「相手」)の下にカード。見出しは PopHeading(アイコン + 名前。見出しの特性)。
    func testCardsHaveRegionHeadings() throws {
        try assertContains(
            "ReverseScreenCards.swift",
            ["PopHeading(", "ReverseLabels.myRegion", "ReverseLabels.opponentRegion", "reverseMyHeading", "reverseOpponentHeading"])
    }

    /// 持ち物の上限の注記は状態の通知(アイコン + 一言)。メガ固定の理由は従来どおり(文のまま)。
    func testCardNoticesAreIconNotices() throws {
        try assertContains("ReverseScreenCards.swift", ["PopNoticeView(kind: .info", "reverseMyItemLimitHint", "reverseMyItemLockReason"])
    }

    // MARK: - 画面本体

    func testScreenUsesPopPartsForPickerAndLoading() throws {
        try assertContains(
            "ReverseScreenView.swift",
            ["PopSymbol.picker", "PopNoticeView(kind: .loading", "ReverseLabels.loading", "reverseLoadingIndicator"])
        try assertNotContains("ReverseScreenView.swift", ["ProgressView"])
        // 相手の持ち物候補の上限の注記も状態の通知。
        try assertContains("ReverseScreenView.swift", ["PopNoticeView(kind: .info", "reverseItemCandidateLimitHint"])
    }

    /// ダメージの側の切り替えは今の区切り(識別子 `reverseSide-*`・選択中の特性・AX では縦積み)のまま。語は不変。
    func testSideSwitchKeepsSegmentedBehavior() throws {
        try assertContains("ReverseScreenView.swift", ["reverseSide-", "isSelected", "accessibility1"])
    }

    // MARK: - 観測欄

    func testObservationRowUsesUnitLabelHintAndIconButtons() throws {
        try assertContains(
            "ReverseScreenObservations.swift",
            [
                "ReverseLabels.unitLabel", "ReverseLabels.observationLabel", "ReverseLabels.observationHint",
                "PopIconButton(", "PopSymbol.minus", "PopSymbol.add", "reverseObservationHint", "reverseObservationRemove-",
                "reverseObservationUnit-", "reverseAddObservationButton",
            ])
        // 旧: 単位を「ダメージ」と出す・丸付きの直書きのアイコン。
        try assertNotContains("ReverseScreenObservations.swift", [#": "ダメージ""#, "minus.circle", "plus.circle"])
        // エラーと上限の注記はアイコン + 文(PopNoticeView)。識別子は不変。
        try assertContains(
            "ReverseScreenObservations.swift",
            ["PopNoticeView(kind: .error", "PopNoticeView(kind: .info", "reverseObservationError-", "reverseObservationLimitHint"])
    }

    // MARK: - 候補のカード・結果

    func testResultsRenderCandidateCardLikeWeb() throws {
        try assertContains(
            "ReverseScreenResults.swift",
            [
                "ReverseLabels.predictedPercent", "ReverseLabels.closeCandidate", "ReverseLabels.referenceRange",
                "ReverseLabels.hasNoExactCandidate", "ReverseLabels.noExactNotice", "BulkRowDisplay.barFraction",
                "accessibilityHidden(true)", "reverseNoExactNotice", "reverseCandidateNature-", "reverseCandidatePercent-",
                "reverseCandidateItem-", "reverseCandidateMatch-", "reverseCandidateRange-", "PopNoticeView(kind: .info",
            ])
        // 旧: 1 行目に「型 / 持ち物」を並べる・一致を caption の文で毎枚出す。
        try assertNotContains("ReverseScreenResults.swift", [#"\(candidate.natureClassLabel) / \(candidate.itemLabel)"#])
    }

    func testUnsupportedMarksKeepTheirIdentifiers() throws {
        try assertContains(
            "ReverseScreenResults.swift",
            ["reverseUnsupportedNotice", "reverseCandidateUnsupported-", "reverseCandidateAbility-", "reversePremise", "reverseExactCount"])
    }

    // MARK: - 検索シート(種族・技。計算・構築・調整と共用のため MasterSearchSheet.swift)

    /// 種族・技の検索欄の補足(空クエリの案内)は「説明」ボタンの奥。「一致なし」「多すぎる」は状態なので従来どおり出す。
    func testSearchSheetsKeepHintBehindHelpButtonAndCloseIsIcon() throws {
        try assertContains(
            "MasterSearchSheet.swift",
            [
                "PopHelpButton(", "CommonPartsText.help", "PopSymbol.help", "speciesSearchHelp", "moveSearchHelp", "speciesSearchNote", "moveSearchNote",
                "PopIconButton(", "PopSymbol.close", "CommonPartsText.close",
                "speciesSearchCancelButton", "moveSearchCancelButton", "speciesSearchHint", "moveSearchHint",
            ])
        try assertNotContains("MasterSearchSheet.swift", [#"Button("閉じる")"#, "ProgressView("])
    }
}
