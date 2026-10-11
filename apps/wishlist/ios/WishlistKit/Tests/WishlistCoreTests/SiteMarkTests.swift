import XCTest

@testable import WishlistCore

// AC-IOS-5-3e: サイトのマーク(web/src/lib/siteMark.ts の移植)。docs/phase5-spec.md 5-3「決めたこと」。
final class SiteMarkTests: XCTestCase {
    /// AC-IOS-5-3e-1: 通常はサイト名の先頭 1 文字
    func testMarkIsTheFirstCharacter() {
        XCTAssertEqual(SiteMark.mark(for: "メルカリ"), "メ")
        XCTAssertEqual(SiteMark.mark(for: "Amazon"), "A")
        XCTAssertEqual(SiteMark.mark(for: "駿河屋"), "駿")
        XCTAssertEqual(SiteMark.mark(for: "魂ウェブ"), "魂")
        XCTAssertEqual(SiteMark.mark(for: ""), "")
    }

    /// AC-IOS-5-3e-2: Yahoo!フリマ と Yahoo!ショッピング は別のマーク(どちらも「Y」にならない)
    func testYahooSitesHaveDistinctMarks() {
        let flea = SiteMark.mark(for: "Yahoo!フリマ")
        let shopping = SiteMark.mark(for: "Yahoo!ショッピング")
        XCTAssertEqual(flea, "フ")
        XCTAssertEqual(shopping, "シ")
        XCTAssertNotEqual(flea, shopping)
        XCTAssertNotEqual(SiteMark.colorHex(for: "Yahoo!フリマ"), SiteMark.colorHex(for: "Yahoo!ショッピング"))
    }

    /// AC-IOS-5-3e-3: 色は名前の表から引く。無い名前は muted
    func testColorTableAndFallback() {
        XCTAssertEqual(SiteMark.colorHex(for: "メルカリ"), "#ff4d6d")
        XCTAssertEqual(SiteMark.colorHex(for: "駿河屋"), "#e17055")
        XCTAssertEqual(SiteMark.colorHex(for: "Yahoo!フリマ"), "#ef233c")
        XCTAssertEqual(SiteMark.colorHex(for: "Yahoo!ショッピング"), "#2f9e44")
        XCTAssertEqual(SiteMark.colorHex(for: "あみあみ"), "#00a884")
        XCTAssertEqual(SiteMark.colorHex(for: "Amazon"), "#ff9f1c")
        XCTAssertEqual(SiteMark.colorHex(for: "魂ウェブ"), "#5b4bd6")
        XCTAssertEqual(SiteMark.colorHex(for: "その他"), SiteMark.fallbackColorHex)
        XCTAssertEqual(SiteMark.fallbackColorHex, "#56677d")
    }
}
