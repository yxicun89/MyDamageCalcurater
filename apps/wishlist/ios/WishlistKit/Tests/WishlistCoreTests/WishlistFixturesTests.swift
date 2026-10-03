import XCTest

@testable import WishlistCore

// AC-IOS-FIX-01: モック起動(XCUITest・プレビュー)用の架空データ。XCUITest がこの名前・並び・ID に依存するので固定する。
// サイトの URL は仕様 §5 の「確認済み」のもの(メルカリ・Amazon)だけ。他サイトは推測で入れない。
final class WishlistFixturesTests: XCTestCase {
    func testSites() {
        XCTAssertEqual(
            WishlistFixtures.sites,
            [
                Site(id: 1, name: "メルカリ", searchURLTemplate: "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc", fetchType: .headless),
                Site(id: 2, name: "Amazon", searchURLTemplate: "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank", fetchType: .linkOnly),
            ])
    }

    func testGenres() {
        XCTAssertEqual(
            WishlistFixtures.genres,
            [
                Genre(id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", sortOrder: 1, siteIDs: [1, 2]),
                Genre(id: 2, name: "デュエマ", queryTemplate: "{name} {option}", sortOrder: 2, siteIDs: [2, 1]),
            ])
    }

    func testItemsHaveTheNamesTheUITestsRelyOn() {
        let items = WishlistFixtures.items
        XCTAssertEqual(items.map(\.id).sorted(), [11, 12])
        let gris = items.first { $0.id == 12 }
        XCTAssertEqual(gris?.name, "グリス")
        XCTAssertEqual(gris?.genreID, 1)
        XCTAssertNil(gris?.optionText)
        let borushack = items.first { $0.id == 11 }
        XCTAssertEqual(borushack?.name, "ボルシャック")
        XCTAssertEqual(borushack?.optionText, "銀トレジャー")
        XCTAssertEqual(borushack?.genreID, 2)
    }

    func testReferentialIntegrityAndImagePaths() {
        let genreIDs = Set(WishlistFixtures.genres.map(\.id))
        let siteIDs = Set(WishlistFixtures.sites.map(\.id))
        for item in WishlistFixtures.items {
            XCTAssertTrue(genreIDs.contains(item.genreID), item.name)
            XCTAssertNotNil(
                item.imageURLPath.range(of: #"^images/[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp|gif)$"#, options: .regularExpression),
                "画像パスは images/<UUID v4>.<拡張子>: \(item.imageURLPath)")
        }
        for genre in WishlistFixtures.genres {
            XCTAssertTrue(Set(genre.siteIDs).isSubset(of: siteIDs), genre.name)
        }
    }

    /// 仕様 §12 フェーズ1の完了条件(「S.H.Figuarts グリス」をメルカリと Amazon で検索した状態で開ける)が、フィクスチャでも成り立つ。
    func testGrisLinksOpenMercariAndAmazonSearches() throws {
        let gris = try XCTUnwrap(WishlistFixtures.items.first { $0.id == 12 })
        let genre = WishlistFixtures.genres.first { $0.id == gris.genreID }
        let links = SiteLinks.resolve(item: gris, genre: genre, sites: WishlistFixtures.sites)
        XCTAssertEqual(links.map(\.site.name), ["メルカリ", "Amazon"])
        XCTAssertEqual(
            links.map(\.url.absoluteString),
            [
                "https://jp.mercari.com/search?keyword=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9&status=on_sale&sort=price&order=asc",
                "https://www.amazon.co.jp/s?k=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9&s=price-asc-rank",
            ])
    }
}
