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
}
