import XCTest

@testable import PokeCalcDesign

#if canImport(UIKit)
    import UIKit
#endif

/// G-05(ADR-0536): Web が足したアイコン(close/open/history/help/up/down/minus。docs/design.md「アイコンの決まり」)
/// に対応する SF Symbols の割り当て。表は design.md から独立に書き写す(実装の写しにしない)。
final class PopSymbolNamesTests: XCTestCase {

    private static let expected: [(web: String, symbol: String, actual: String)] = [
        ("close", "xmark", PopSymbolNames.close),
        ("open", "chevron.down", PopSymbolNames.open),
        ("history", "clock.arrow.circlepath", PopSymbolNames.history),
        ("help", "questionmark.circle", PopSymbolNames.help),
        ("up", "arrow.up", PopSymbolNames.up),
        ("down", "arrow.down", PopSymbolNames.down),
        ("minus", "minus", PopSymbolNames.minus),
    ]

    func testAssignmentMatchesDesignDoc() {
        for entry in Self.expected {
            XCTAssertEqual(entry.actual, entry.symbol, "Web の \(entry.web)")
        }
    }

    func testNamesAreDistinctSoMeaningsDoNotCollide() {
        XCTAssertEqual(Set(Self.expected.map(\.actual)).count, Self.expected.count)
    }

    #if canImport(UIKit)
        func testEveryNameIsARealSFSymbol() {
            for entry in Self.expected {
                XCTAssertNotNil(UIImage(systemName: entry.actual), "SF Symbols に無い名前: \(entry.actual)(\(entry.web))")
            }
        }
    #endif
}
