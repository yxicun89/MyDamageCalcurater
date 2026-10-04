import XCTest

@testable import WishlistCore

// AC-IOS-OFF-07: 保存済みのキャッシュ(フェーズ4 より前の JSON)を読める。新しい項目は既定値になり、書き戻しても失われない。
// Domain.swift の Decodable(spec-writer が実装済み)なので最初から通る。
final class OfficialCacheCompatibilityTests: XCTestCase {
    func testOldItemAndGenreJSONDecodeWithDefaults() throws {
        let oldItem = try JSONEncoder().encode(OldItem(id: 12, genreID: 1, name: "グリス", imageURLPath: "images/a.png", sortOrder: 0, siteOverrides: [], createdAt: Date(timeIntervalSince1970: 0), updatedAt: Date(timeIntervalSince1970: 0)))
        let item = try JSONDecoder().decode(Item.self, from: oldItem)
        XCTAssertFalse(item.watchOfficial)
        XCTAssertNil(item.officialStatus)

        let oldGenre = try JSONEncoder().encode(OldGenre(id: 1, name: "S.H.Figuarts", queryTemplate: "{name}", sortOrder: 1, siteIDs: [1]))
        XCTAssertEqual(try JSONDecoder().decode(Genre.self, from: oldGenre).aliases, [])
    }

    func testRoundTripKeepsNewFields() throws {
        var item = T.gris
        item.watchOfficial = true
        item.officialStatus = OfficialStatus(status: .preorder, evidence: ["予約する"], checkedAt: Date(timeIntervalSince1970: 1_791_050_400))
        XCTAssertEqual(try JSONDecoder().decode(Item.self, from: JSONEncoder().encode(item)), item)
        var genre = T.figuarts
        genre.aliases = [["HG", "ハイグレード"]]
        XCTAssertEqual(try JSONDecoder().decode(Genre.self, from: JSONEncoder().encode(genre)), genre)
    }

    private struct OldItem: Encodable {
        var id: Int
        var genreID: Int
        var name: String
        var imageURLPath: String
        var sortOrder: Int
        var siteOverrides: [SiteOverride]
        var createdAt: Date
        var updatedAt: Date
    }

    private struct OldGenre: Encodable {
        var id: Int
        var name: String
        var queryTemplate: String
        var sortOrder: Int
        var siteIDs: [Int]
    }
}
