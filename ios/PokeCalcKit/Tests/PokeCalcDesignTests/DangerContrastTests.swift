import XCTest

@testable import PokeCalcDesign

/// issue #99(アクセシビリティ): danger トークンが WCAG 2.2 SC 1.4.3(通常文字 4.5:1)を、
/// ライト・ダークどちらの背景(bg.base 単体・bg.glass を bg.base に重ねた見た目)に対しても満たすことを検査する。
/// `web/src/styles/contrast.test.ts` と同じ組み合わせ(独立した実装。coding-rules §2)。
final class DangerContrastTests: XCTestCase {

    func testLightThemeDangerAgainstBgBase() {
        assertMeetsWCAGNormalText(ColorToken.danger.light, ColorToken.bgBase.light, "ライト danger / bg.base")
    }

    func testLightThemeDangerAgainstBgGlassOverBgBase() {
        let bgGlass = compositeOver(ColorToken.bgGlass.light, over: ColorToken.bgBase.light)
        assertMeetsWCAGNormalText(ColorToken.danger.light, bgGlass, "ライト danger / bg.glass(bg.base に合成)")
    }

    func testDarkThemeDangerAgainstBgBase() {
        assertMeetsWCAGNormalText(ColorToken.danger.dark, ColorToken.bgBase.dark, "ダーク danger / bg.base")
    }

    func testDarkThemeDangerAgainstBgGlassOverBgBase() {
        let bgGlass = compositeOver(ColorToken.bgGlass.dark, over: ColorToken.bgBase.dark)
        assertMeetsWCAGNormalText(ColorToken.danger.dark, bgGlass, "ダーク danger / bg.glass(bg.base に合成)")
    }

    private func assertMeetsWCAGNormalText(_ foreground: RGBA, _ background: RGBA, _ label: String,
                                            file: StaticString = #filePath, line: UInt = #line) {
        let ratio = contrastRatio(foreground, background)
        XCTAssertGreaterThanOrEqual(ratio, wcagNormalTextMinContrast, "\(label): \(ratio)", file: file, line: line)
    }
}
