import Foundation
import XCTest

/// G-05(ADR-0536): 作り直す 4 つの View ファイルが守る決まりのソース検査(docs/design.md「動きの決まり」「見やすさの決まり」
/// 「iOS の追い方(G-05)」)。View の見た目そのものは UITest が見る。ここは UITest では見えない決まり
/// (常時動く表示・行数の制限・色のリテラル・アイコンの割り当ての迂回・ProgressView の回転)をファイルの文字列で確かめる。
/// 対象は `ios/PokeCalc/` のファイル名(無ければ失敗する。消したり移したら、この表も同じ変更で直す)。
final class G05ScreenSourceRulesTests: XCTestCase {

    private static let files = [
        "AboutView.swift", "FavoritesScreenView.swift", "FavoriteLoadViews.swift", "FavoritePinSection.swift",
    ]

    private func source(_ name: String) throws -> String {
        // このファイル: ios/PokeCalcKit/Tests/PokeCalcCoreTests/<this>.swift → ios/PokeCalc/<name>
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

    /// 常時動く表示を置かない(回り続ける ProgressView・永久に繰り返す動き)。読み込み中は静止したアイコン + 一言。
    func testNoAlwaysOnAnimation() throws {
        try assertNoMatch(#"ProgressView\("#, "回り続ける読み込み表示を置かない(PopNoticeView(.loading) の静止したアイコン + 一言)")
        try assertNoMatch(#"repeatForever|\.symbolEffect\(\.(pulse|variableColor|bounce|wiggle|breathe|rotate)"#, "常時動く演出を置かない")
        try assertNoMatch(#"TimelineView"#, "時間で動く表示を置かない")
    }

    /// 行数で切らない(拡大した文字でも全文が読める。AX5 でも欠けない)。
    func testNoLineLimit() throws {
        try assertNoMatch(#"\.lineLimit\("#, "lineLimit を付けない")
        try assertNoMatch(#"\.minimumScaleFactor\("#, "文字を縮めて収めない")
    }

    /// 色はトークン(パレット K)だけ。リテラル・システム色を直接使わない。
    func testColorsComeFromTokensOnly() throws {
        try assertNoMatch(#"Color\((red|white|black|hue|\.sRGB|\.display)"#, "色のリテラルを使わない")
        try assertNoMatch(
            #"#colorLiteral|UIColor\(|Color\.(red|blue|green|orange|yellow|gray|pink|purple|white|black)\b"#,
            "システム色・リテラルを使わない")
        try assertNoMatch(
            #"\.(foregroundStyle|foregroundColor|tint|background|fill)\(\s*\.(red|blue|green|orange|yellow|gray|pink|purple|white|black|primary|secondary|accentColor)\b"#,
            "システム色を直接指定しない")
    }

    /// アイコンは `PopSymbol`(1 か所の割り当て)経由。SF Symbols の名前を View に直書きしない。
    func testSymbolsAreAssignedThroughPopSymbol() throws {
        try assertNoMatch(#"(systemName|systemImage):\s*""#, "SF Symbols の名前を直書きしない(PopSymbol に割り当てる)")
        try assertNoMatch(#"Image\(systemName:"#, "アイコンは PopIcon(PopSymbol.…)で出す")
    }

    /// 空の状態は `ContentUnavailableView`(アイコン・一言・行動のボタン)。
    func testEmptyStatesUseContentUnavailableView() throws {
        XCTAssertTrue(try code("FavoritesScreenView.swift").contains("ContentUnavailableView"), "お気に入り・履歴の空の状態")
        XCTAssertTrue(try code("FavoriteLoadViews.swift").contains("ContentUnavailableView"), "お気に入りを選ぶシートの空の状態")
    }

    /// 補足の文は「説明」ボタン(`CommonPartsText.help`)の奥へ。履歴・読み込みシート・復元の案内が使う。
    func testNotesLiveBehindTheHelpButton() throws {
        XCTAssertTrue(try code("FavoritesScreenView.swift").contains("CalcHistoryLabels.note"), "履歴の補足の文")
        XCTAssertTrue(try code("FavoriteLoadViews.swift").contains("FavoritesLabels.loadNote"), "読み込みシートの注記")
        XCTAssertTrue(try code("FavoriteLoadViews.swift").contains("FavoritesLabels.restoreLimitNote"), "復元の案内の限界の文")
        for name in ["FavoritesScreenView.swift", "FavoriteLoadViews.swift"] {
            XCTAssertTrue(try code(name).contains("PopSymbol.help"), "\(name): 説明ボタンは help のアイコン")
            XCTAssertTrue(try code(name).contains("CommonPartsText.help"), "\(name): 説明ボタンの名前は用語集の語")
        }
    }

    /// 「閉じる」はシートの閉じるボタン(アイコンだけ)で、読み上げ名は用語集の語。
    func testSheetCloseButtonIsIconWithGlossaryLabel() throws {
        let text = try code("FavoriteLoadViews.swift")
        XCTAssertTrue(text.contains("PopSymbol.close"))
        XCTAssertTrue(text.contains("CommonPartsText.close"))
    }

    /// 「このアプリについて」の節の見出しのアイコン: 非公式 = 注意、出典 = 情報、データの扱い = 丸の i(削除のごみ箱を見出しに使わない)。
    func testAboutSectionHeadingIcons() throws {
        let text = try code("AboutView.swift")
        XCTAssertTrue(text.contains("AboutText.unofficialHeading"))
        XCTAssertTrue(text.contains("PopSymbol.warning"), "非公式表示の見出しは注意のアイコン")
        XCTAssertTrue(text.contains("AboutText.dataSourcesHeading"))
        XCTAssertTrue(text.contains("PopSymbol.info"), "データの出典の見出しは情報のアイコン")
        XCTAssertNil(
            text.range(
                of: #"PopHeading\(title: DeviceDataText\.sectionTitle,\s*systemImage:\s*PopSymbol\.delete"#,
                options: .regularExpression),
            "データの扱いの見出しにごみ箱を使わない(丸の i)")
        XCTAssertNotNil(
            text.range(
                of: #"PopHeading\(title: DeviceDataText\.sectionTitle,\s*systemImage:\s*PopSymbol\.about"#,
                options: .regularExpression),
            "データの扱いの見出しは PopSymbol.about")
    }

    /// 触覚: 削除の確定は `.warning`(design.md「動きの決まり」。操作したときだけ)。
    func testDeleteUsesWarningHaptic() throws {
        XCTAssertTrue(try code("FavoritesScreenView.swift").contains(".sensoryFeedback(.warning"))
    }
}
