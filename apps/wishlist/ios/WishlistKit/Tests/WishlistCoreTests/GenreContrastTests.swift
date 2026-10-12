import XCTest

@testable import WishlistCore

final class GenreContrastTests: XCTestCase {
    func testLightBackgroundsPreferDarkText() {
        for hex in ["#f2b705", "#ffffff", "#f59e0b", "#7cb518"] {
            XCTAssertTrue(GenreContrast.prefersDarkText(onHex: hex), hex)
        }
    }

    func testDarkBackgroundsPreferWhiteText() {
        for hex in ["#1d5fd1", "#e0393e", "#4f46e5", "#000000", "bad"] {
            XCTAssertFalse(GenreContrast.prefersDarkText(onHex: hex), hex)
        }
    }
    /// 設定の色見本 12 色(web/src/ui/icons.tsx の COLOR_SWATCHES・iOS の GenrePalette と同じ色)。
    func testSwatchesPickReadableText() {
        for hex in ["#f0642a", "#f59e0b", "#f2b705", "#7cb518", "#16a34a", "#0d9488", "#0ea5e9", "#e04f9a"] {
            XCTAssertTrue(GenreContrast.prefersDarkText(onHex: hex), hex)
        }
        for hex in ["#e0393e", "#1f6fe5", "#4f46e5", "#7c4dff"] {
            XCTAssertFalse(GenreContrast.prefersDarkText(onHex: hex), hex)
        }
    }
}
