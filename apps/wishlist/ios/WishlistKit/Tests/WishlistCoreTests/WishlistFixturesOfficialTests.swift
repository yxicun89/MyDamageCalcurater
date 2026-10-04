import XCTest

@testable import WishlistCore

// AC-IOS-OFF-08: `WISHLIST_USE_FAKE=official` のフィクスチャ(XCUITest が依存する。spec-writer が実装済みなので最初から通る)。
final class WishlistFixturesOfficialTests: XCTestCase {
    func testOfficialFixtures() async throws {
        let gris = try XCTUnwrap(WishlistFixtures.officialItems.first { $0.id == 12 })
        XCTAssertTrue(gris.watchOfficial)
        XCTAssertEqual(gris.sourceURL, "https://tamashii.example/item/12/")
        XCTAssertEqual(gris.officialStatus?.status, .preorder)
        XCTAssertEqual(gris.officialStatus?.evidence, ["予約受付中", "予約する"])
        XCTAssertEqual(PriceFormat.jstDate(WishlistFixtures.officialCheckedAt), "10/4")
        let borushack = try XCTUnwrap(WishlistFixtures.officialItems.first { $0.id == 11 })
        XCTAssertFalse(borushack.watchOfficial)
        XCTAssertEqual(WishlistFixtures.officialItems.map(\.id), WishlistFixtures.items.map(\.id))

        XCTAssertEqual(WishlistFixtures.officialGenres.first { $0.id == 1 }?.aliases, [["S.H.Figuarts", "SHフィギュアーツ"]])
        XCTAssertEqual(WishlistFixtures.officialGenres.first { $0.id == 2 }?.aliases, [])

        let service = WishlistFixtures.makeServiceWithOfficial()
        let items = try await service.listItems()
        XCTAssertEqual(items, WishlistFixtures.officialItems)
        let genres = try await service.listGenres()
        XCTAssertEqual(genres, WishlistFixtures.officialGenres)
    }
}
