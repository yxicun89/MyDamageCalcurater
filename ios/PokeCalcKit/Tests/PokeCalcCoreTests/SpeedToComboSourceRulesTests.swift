import Foundation
import XCTest

/// I-ios-21(ADR-0542・ADR-0501「素早さ画面から連続技へ」AC-h1・AC-h5・AC-h6・AC-h7): UITest では見えない配線と決まりのソース検査。
/// `G05CalcSourceRulesTests` と同じ作り(`ios/PokeCalc/` のファイルを文字列で読む。`//` で始まる行は外す)。
/// 計算画面・連続技の節のファイルの共通の決まり(ProgressView・lineLimit・色・SF Symbols の直書き・視差効果)は
/// `G05CalcSourceRulesTests` がそのまま見る。ここは引き継ぎで足す部分だけを見る。
final class SpeedToComboSourceRulesTests: XCTestCase {

    private static let comboFiles = ["ComboSectionView.swift", "ComboSectionParts.swift", "ComboResultView.swift"]
    private static let speedFiles = ["SpeedScreenView.swift", "SpeedMatchupOutput.swift"]

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
            .split(separator: "\n", omittingEmptySubsequences: false)
            .filter { !$0.trimmingCharacters(in: .whitespaces).hasPrefix("//") }
            .joined(separator: "\n")
    }

    private func joined(_ names: [String]) throws -> String {
        try names.map(source).joined(separator: "\n")
    }

    private func assertContains(
        _ label: String, _ text: String, _ needles: [String], file: StaticString = #filePath, line: UInt = #line
    ) {
        for needle in needles {
            XCTAssertTrue(text.contains(needle), "\(label) に \(needle) が無い", file: file, line: line)
        }
    }

    // MARK: - AC-h1: ルートの token の配線

    /// ルートが要求(token)を持ち、計算画面(タブの根)へ渡す。素早さ画面には「連続技を開く」口を渡す。
    func testRootWiresTheRequestThroughFeatureContext() throws {
        assertContains("RootView.swift", try source("RootView.swift"), ["navigation.openCombo(", "navigation.comboOpenRequest"])
        assertContains("Features/AppFeature.swift", try source("Features/AppFeature.swift"), ["comboOpenRequest", "openCombo"])
        assertContains("Features/CalcFeature.swift", try source("Features/CalcFeature.swift"), ["comboOpenRequest"])
        assertContains("Features/SpeedFeature.swift", try source("Features/SpeedFeature.swift"), ["onOpenCombo:"])
        assertContains("CalcScreenView.swift", try source("CalcScreenView.swift"), ["ComboOpenRequest", "openFromSpeed("])
    }

    // MARK: - AC-h5: フォーカスと読み上げ(操作したときだけ。常時の動きは無し)

    func testFocusAndAnnouncementAfterTheHandoff() throws {
        let combo = try joined(Self.comboFiles)
        assertContains(
            "連続技の節", combo,
            [
                "applyHandoff(", "appliedHandoffCount", "@AccessibilityFocusState", "AccessibilityNotification.Announcement(",
                "ComboText.handoffAnnouncement",
            ])
        assertContains("CalcScreenView.swift", try source("CalcScreenView.swift"), ["ScrollViewReader", ".scrollTo("])
    }

    /// 節へ送る動き(スクロール)は視差効果を減らすで 0 秒。
    func testScrollRespectsReduceMotion() throws {
        let calc = try source("CalcScreenView.swift")
        XCTAssertTrue(calc.contains("accessibilityReduceMotion"), "CalcScreenView は視差効果を減らすを読む")
        if calc.contains("withAnimation") {
            XCTAssertTrue(
                calc.range(of: #"reduceMotion\s*\?\s*nil"#, options: .regularExpression) != nil
                    || calc.contains("MotionToken"),
                "スクロールの動きは reduceMotion のとき nil(0 秒)にする")
        }
    }

    // MARK: - AC-h4: スカーフの 1 行と「説明」

    func testScarfNoticeAndHelpButton() throws {
        let combo = try joined(Self.comboFiles)
        assertContains(
            "連続技の節", combo,
            [
                "handoffScarfNoticeVisible", "ComboText.handoffScarfIgnored", "comboScarfIgnored", "PopNoticeView(kind: .info",
                "PopHelpButton(", "CommonPartsText.help", "PopSymbol.help", "comboHandoffHelpButton", "ComboText.handoffHelpLines",
            ])
    }

    // MARK: - AC-h6: 素早さ画面のボタン

    func testSpeedScreenLinkUsesTokensAndNoLineLimit() throws {
        let speed = try joined(Self.speedFiles)
        assertContains("素早さ画面", speed, ["speedComboLink", "SpeedMatchupLabels.comboLink", "PopLabel(", "PopSymbol."])
        XCTAssertNil(speed.range(of: #"\.lineLimit\(|\.minimumScaleFactor\("#, options: .regularExpression), "文字を切らない・縮めない")
        XCTAssertNil(speed.range(of: #"ProgressView\(|repeatForever|TimelineView"#, options: .regularExpression), "常時動く表示を置かない")
    }
}
