import SwiftUI
import XCTest

@testable import PokeCalcDesign

/// docs/design.md「ポップ配色」(ADR-0334 / F-12)の値を、実装の写しではなく design.md の表から
/// **独立に**書き写して照合する(coding-rules §2)。design.md を変えたら、この表も同じ変更で直す。
/// Web の `web/src/styles/popPalette.test.ts` の `CONTRAST_PAIRS` と同じ組み合わせでコントラストも固定する。
final class PopPaletteTests: XCTestCase {

    /// design.md の表(トークン名は camelCase。値は #RRGGBB。shadow.color だけ黒の alpha)。
    private static let expected: [(name: String, light: String, dark: String)] = [
        ("brandPrimary", "1F5FD6", "7FA8FF"),
        ("onPrimary", "FFFFFF", "0E1015"),
        ("brandAccent", "FFCB05", "FFD84D"),
        ("onAccent", "14161A", "14161A"),
        ("success", "17743A", "5FD38A"),
        ("successSoft", "E2F5E8", "12301F"),
        ("warning", "9A5B00", "FFB547"),
        ("warningSoft", "FFF1D6", "33240B"),
        ("info", "0B6BA8", "5EC2F2"),
        ("infoSoft", "DCEFFB", "0C2A3A"),
        ("dangerSoft", "FDE3E4", "3A1416"),
        ("onDanger", "FFFFFF", "0E1015"),
        ("surfaceCard", "FFFFFF", "1A1D24"),
        ("bgGradientStart", "FFF6E0", "14131C"),
        ("bgGradientEnd", "E8F1FF", "0E1622"),
        ("tableHeader", "DCE7FB", "1C2638"),
        ("tableZebra", "EEF2F8", "151922"),
        ("tableHover", "E3ECFB", "1D2535"),
        ("focusRing", "1F5FD6", "7FA8FF"),
    ]

    private static func hex(_ value: String) -> RGBA {
        let number = Int(value, radix: 16)!
        return RGBA(red: (number >> 16) & 0xFF, green: (number >> 8) & 0xFF, blue: number & 0xFF, alpha: 1.0)
    }

    private func token(_ name: String) throws -> ColorPair {
        try XCTUnwrap(ColorToken.popPalette.first { $0.name == name }?.pair, name)
    }

    func testPopPaletteMatchesDesignDoc() throws {
        for row in Self.expected {
            let pair = try token(row.name)
            XCTAssertEqual(pair.light, Self.hex(row.light), "\(row.name) light")
            XCTAssertEqual(pair.dark, Self.hex(row.dark), "\(row.name) dark")
        }
        let shadow = try token("shadowColor")
        XCTAssertEqual(shadow.light, RGBA(red: 0, green: 0, blue: 0, alpha: 0.10))
        XCTAssertEqual(shadow.dark, RGBA(red: 0, green: 0, blue: 0, alpha: 0.40))
        XCTAssertEqual(ColorToken.popPalette.count, Self.expected.count + 1, "design.md に無い色を足さない")
    }

    func testPopPaletteNamesAvoidReservedPrefixesAndSuffixes() {
        for entry in ColorToken.popPalette {
            XCTAssertFalse(entry.name.hasSuffix("Ink"), entry.name)
            XCTAssertFalse(entry.name.hasPrefix("type"), entry.name)
        }
    }

    // MARK: - コントラスト(WCAG 2.2。通常文字 4.5、UI の境界・フォーカスの輪 3)

    private static let nonTextMinContrast = 3.0

    private func color(_ name: String, dark: Bool) throws -> RGBA {
        let pair: ColorPair
        switch name {
        case "textPrimary": pair = ColorToken.textPrimary
        case "textSecondary": pair = ColorToken.textSecondary
        case "bgBase": pair = ColorToken.bgBase
        case "bgGlass": pair = ColorToken.bgGlass
        case "danger": pair = ColorToken.danger
        default: pair = try token(name)
        }
        let value = dark ? pair.dark : pair.light
        guard value.alpha < 1 else { return value }
        // 半透明(bg.glass)は bg.base に重ねた見た目で検査する。
        return compositeOver(value, over: dark ? ColorToken.bgBase.dark : ColorToken.bgBase.light)
    }

    private static let textPairs: [(String, String)] = {
        var pairs: [(String, String)] = [
            ("onPrimary", "brandPrimary"), ("brandPrimary", "bgBase"), ("brandPrimary", "bgGlass"),
            ("brandPrimary", "surfaceCard"), ("onAccent", "brandAccent"),
            // iOS 固有: 背景のグラデーションの上に置く主色の文字(追加ボタンのラベル等)・カードの面・ゼブラの行の上。
            ("brandPrimary", "bgGradientStart"), ("brandPrimary", "bgGradientEnd"), ("brandPrimary", "tableZebra"),
            ("danger", "bgGradientStart"), ("danger", "bgGradientEnd"), ("danger", "tableZebra"),
            ("success", "bgBase"), ("success", "surfaceCard"), ("success", "successSoft"),
            ("warning", "bgBase"), ("warning", "surfaceCard"), ("warning", "warningSoft"),
            ("info", "bgBase"), ("info", "surfaceCard"), ("info", "infoSoft"),
            ("danger", "surfaceCard"), ("danger", "dangerSoft"), ("onDanger", "danger"),
            ("textPrimary", "successSoft"), ("textPrimary", "warningSoft"),
            ("textPrimary", "infoSoft"), ("textPrimary", "dangerSoft"),
        ]
        for background in ["bgGradientStart", "bgGradientEnd", "surfaceCard", "tableHeader", "tableZebra", "tableHover"] {
            pairs.append(("textPrimary", background))
            pairs.append(("textSecondary", background))
        }
        return pairs
    }()

    private static let nonTextPairs: [(String, String)] = [
        ("focusRing", "bgBase"), ("focusRing", "surfaceCard"),
        ("focusRing", "bgGradientStart"), ("focusRing", "bgGradientEnd"),
        ("brandPrimary", "bgGradientStart"), ("brandPrimary", "bgGradientEnd"),
    ]

    func testTextContrastPairsMeetWCAGNormalText() throws {
        for dark in [false, true] {
            for (foreground, background) in Self.textPairs {
                let ratio = contrastRatio(try color(foreground, dark: dark), try color(background, dark: dark))
                XCTAssertGreaterThanOrEqual(ratio, wcagNormalTextMinContrast,
                                            "\(dark ? "ダーク" : "ライト") \(foreground) / \(background): \(ratio)")
            }
        }
    }

    func testNonTextContrastPairsMeetWCAGNonText() throws {
        for dark in [false, true] {
            for (foreground, background) in Self.nonTextPairs {
                let ratio = contrastRatio(try color(foreground, dark: dark), try color(background, dark: dark))
                XCTAssertGreaterThanOrEqual(ratio, Self.nonTextMinContrast,
                                            "\(dark ? "ダーク" : "ライト") \(foreground) / \(background): \(ratio)")
            }
        }
    }

    // MARK: - 文字・影・動き

    func testTitleTypographyAndWeights() {
        XCTAssertEqual(TextStyleToken.title.size, 22)
        XCTAssertEqual(TextStyleToken.title.weight.cssValue, 800)
        XCTAssertEqual(TextStyleToken.heading.weight.cssValue, 700)
        XCTAssertEqual(FontWeightToken.strong.cssValue, 700)
        XCTAssertEqual(TextStyleToken.body.weight.cssValue, 400)
        XCTAssertGreaterThan(TextStyleToken.heading.weight.cssValue, TextStyleToken.body.weight.cssValue)
        XCTAssertGreaterThanOrEqual(TextStyleToken.title.weight.cssValue, TextStyleToken.heading.weight.cssValue)
    }

    func testShadowTokensMatchDesignDoc() {
        XCTAssertEqual(ShadowToken.card, ShadowToken(offsetY: 2, blur: 8))
        XCTAssertEqual(ShadowToken.raised, ShadowToken(offsetY: 6, blur: 16))
        XCTAssertEqual(ShadowToken.card.radius, 4)
        XCTAssertEqual(ShadowToken.raised.radius, 8)
    }

    func testPressMotionIsZeroWhenReducingMotion() {
        XCTAssertEqual(MotionToken.pressDuration, 0.15)
        XCTAssertEqual(MotionToken.pressDuration(reduceMotion: false), 0.15)
        XCTAssertEqual(MotionToken.pressDuration(reduceMotion: true), 0)
        XCTAssertEqual(MotionToken.pressScale, 0.96)
    }
}
