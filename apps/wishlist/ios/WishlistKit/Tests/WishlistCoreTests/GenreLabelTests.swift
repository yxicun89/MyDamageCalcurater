import XCTest

@testable import WishlistCore

// phase6-spec 6-2: 選ばれているジャンルのボタンだけ、絵柄の隣に名前を出す(ユーザー決定 2026-10-12)。
// 出す/出さないの判断と、長い名前の扱いは View ではなくここ(GenreLabel)に置く。
final class GenreLabelTests: XCTestCase {
    func testSelectedShowsNameAndUnselectedShowsNothing() {
        XCTAssertEqual(GenreLabel.visibleName("S.H.Figuarts", selected: true), "S.H.Figuarts")
        XCTAssertNil(GenreLabel.visibleName("S.H.Figuarts", selected: false))
    }

    func testAllChipUsesTheSameRule() {
        XCTAssertEqual(GenreLabel.visibleName(GenreLabel.allName, selected: true), "すべて")
        XCTAssertNil(GenreLabel.visibleName(GenreLabel.allName, selected: false))
    }

    func testBlankNameShowsNothingEvenWhenSelected() {
        XCTAssertNil(GenreLabel.visibleName("", selected: true))
        XCTAssertNil(GenreLabel.visibleName("  ", selected: true))
    }

    func testNameIsTrimmedButNotShortened() {
        // 省略は View の lineLimit(1) + truncationMode(.tail) に任せる。文字列は切らない
        XCTAssertEqual(GenreLabel.visibleName(" METAL BUILD・ROBOT魂 ", selected: true), "METAL BUILD・ROBOT魂")
        XCTAssertEqual(GenreLabel.visibleName("デュエル・マスターズ", selected: true), "デュエル・マスターズ")
    }

    func testMaxNameWidthKeepsTwoPillsInA375WideScreen() {
        // 選択中のボタン = 絵柄(24)+余白+名前。名前の最大幅はここで決め、View はこの値を使う
        XCTAssertEqual(GenreLabel.maxNameWidth, 140)
    }
}
