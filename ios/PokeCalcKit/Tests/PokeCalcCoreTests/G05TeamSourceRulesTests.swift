import Foundation
import XCTest

/// G-05 第 2 弾(ADR-0537): 構築の一覧・編集の View ファイルが守る決まりのソース検査(docs/design.md「動きの決まり」「見やすさの決まり」
/// 「iOS の追い方(G-05)」)。`G05ScreenSourceRulesTests`(第 1 弾)と同じ作り。UITest では見えない決まり(常時動く表示・行数の制限・
/// 色のリテラル・アイコンの割り当ての迂回・視差効果)と、作り直しの構造(部品の使い方)をファイルの文字列で確かめる。
/// 対象は `ios/PokeCalc/` のファイル名(無ければ失敗する。消したり移したら、この表も同じ変更で直す)。
/// `TeamBalanceSection.swift`(タイプバランスの節。ADR-0532)とランクのメーターは変えないので対象に入れない。
final class G05TeamSourceRulesTests: XCTestCase {

    private static let files = [
        "TeamListView.swift", "TeamEditView.swift", "TeamEditMemberCard.swift", "TeamSlotViews.swift",
        "TeamSlotGridView.swift", "TeamSourceMenuRow.swift", "TeamTextControls.swift",
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

    // MARK: - 一覧

    func testListUsesContentUnavailableViewAndStaticLoading() throws {
        try assertContains(
            "TeamListView.swift",
            ["ContentUnavailableView", "PopSymbol.team", "TeamLabels.emptyList", "PopNoticeView(kind: .loading", "TeamLabels.loading"])
    }

    func testListCardShowsAllSixSlotsAndIconOnlyDelete() throws {
        try assertContains(
            "TeamListView.swift", ["TeamLimits.maxMembers", "PopIconButton(", "PopSymbol.delete", "TeamLabels.deleteHint"])
        // 削除の確定は取り消せない操作なので警告の触覚(design.md「動きの決まり」)。
        try assertContains("TeamListView.swift", [".sensoryFeedback(.warning"])
        // 旧: 文字つきの削除ボタン(PopLabel(title: TeamLabels.deleteButton …))。
        try assertNotContains("TeamListView.swift", ["PopLabel(title: TeamLabels.deleteButton"])
    }

    // MARK: - 編集の画面

    func testEditHasTileGridAndStaticStates() throws {
        try assertContains(
            "TeamEditView.swift",
            ["TeamSlotGridView(", "ScrollViewReader", "PopSymbol.back", "PopNoticeView(kind: .loading", "TeamLabels.loading"])
        // 未保存・保存済みは アイコン + 一言(旧: caption の文字だけ)。
        try assertContains("TeamEditView.swift", ["PopSymbol.warning", "PopSymbol.done", "TeamLabels.unsavedNotice", "TeamLabels.savedNotice"])
    }

    func testTileGridUsesGridWithImagesAndAddTile() throws {
        try assertContains(
            "TeamSlotGridView.swift",
            [
                "LazyVGrid", "TeamLabels.slotsGroup", "TeamLabels.slotAdd", "SpeciesImageView", "TypeBadgeView", "PopSymbol.add",
                "TeamLimits.maxMembers",
            ])
    }

    func testSlotControlsAreIconOnlyButtonsWithOldNames() throws {
        try assertContains(
            "TeamSlotViews.swift",
            [
                "PopIconButton(", "PopSymbol.up", "PopSymbol.down", "PopSymbol.delete", "TeamLabels.moveUp", "TeamLabels.moveDown",
                "TeamLabels.remove",
            ])
        // 空の枠の案内の文は無くなった(タイルの「+」と「ポケモン」欄が示す)。
        try assertNotContains("TeamSlotViews.swift", ["emptySlotHint", "slotEmptyHint", #"title: "上へ""#, #"title: "下へ""#, #"title: "外す""#])
    }

    // MARK: - メンバーカード

    func testMemberCardUsesWebWordsAndSPButtons() throws {
        try assertContains(
            "TeamEditMemberCard.swift",
            [
                "TeamLabels.movesHeading", "TeamLabels.moveNone", "TeamLabels.nicknameField", "TeamLabels.spSummary",
                "TeamLabels.spIncrease", "TeamLabels.spDecrease", "TeamLabels.spField", "TeamSPControl.canIncrease",
                "TeamSPControl.canDecrease", "TeamSPControl.fillFraction", "PopIconButton(", "PopSymbol.minus", "PopSymbol.add",
                "keyboardType(.numberPad)",
            ])
        try assertNotContains(
            "TeamEditMemberCard.swift",
            ["Stepper(", #""技を選択""#, #""ニックネーム(任意)""#, #"Text("わざ")"#, #"Text("能力ポイント(合計"#])
    }

    /// 技の選択は計算・逆算と同じタイプ順の群(G-01)。他の枠の技は消さず、薄く押せない行にする(Web の技ピッカーと同じ)。
    func testMoveSheetFollowsWebMovePicker() throws {
        try assertContains("TeamEditMemberCard.swift", ["groupsByType: true", "disabledIDs:", "noneLabel: TeamLabels.moveNone"])
        try assertNotContains("TeamEditMemberCard.swift", ["remainingOptions"])
        try assertContains("MasterSearchSheet.swift", ["disabledIDs", "noneLabel"])
        try assertNotContains("MasterSearchSheet.swift", [#"Text("外す")"#])
    }

    // MARK: - 「構築から選ぶ」の入口

    func testSourceMenuRowHasNoLineLimitAndShowsEmptyAsNotice() throws {
        try assertContains("TeamSourceMenuRow.swift", ["PopNoticeView(kind: .empty", "PopSymbol.picker"])
        try assertNotContains("TeamSourceMenuRow.swift", ["lineLimit", "minimumScaleFactor"])
    }

    // MARK: - 共通の部品(第 1 弾の部品を再利用し、足りない分だけ足す)

    func testPopComponentsGainSymbolsAndEmptyIcon() throws {
        try assertContains("PopComponents.swift", ["static let back =", "static let picker ="])
        let text = try code("PopComponents.swift")
        XCTAssertNotNil(
            text.range(of: #"struct PopEmptyLabel[\s\S]*?systemImage"#, options: .regularExpression),
            "PopEmptyLabel が空の状態のアイコンを渡せる(構築は構築のアイコン)")
    }
}
