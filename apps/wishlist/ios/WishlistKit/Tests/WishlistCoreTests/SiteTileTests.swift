import XCTest

@testable import WishlistCore

// 詳細のサイトのタイル（phase5-spec 5-2「詳細」: マークと価格、最安の強調、出品なしは薄く、失敗）。
@MainActor
final class SiteTileTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_790_985_600)

    private func rows() -> [SiteRow] {
        T.sites.map { SiteRow(link: SiteLink(site: $0, query: "q", url: URL(string: "https://x.example/?q=q")!)) }
    }

    private func estimate(_ id: Int, low: Int?, status: EstimateStatus = .ok, fetchedAt: Date? = nil) -> SiteEstimate {
        SiteEstimate(siteID: id, low: low, mid: low, count: 1, suspiciousCount: 0, status: status, fetchedAt: fetchedAt ?? now)
    }

    func testPriceMarkAndBest() {
        let tiles = SiteTile.make(
            rows: rows(), estimates: ItemEstimates(itemID: 1, sites: [estimate(1, low: 3000), estimate(2, low: 4500)]), now: now)
        XCTAssertEqual(tiles.map(\.priceText), ["¥3,000", "¥4,500", nil])
        XCTAssertEqual(tiles.map(\.isBest), [true, false, false])
        XCTAssertEqual(tiles[0].mark, "メ")
        XCTAssertEqual(tiles[0].colorHex, "#ff4d6d")
        XCTAssertEqual(tiles[2].colorHex, SiteMark.fallbackColorHex)
    }

    func testNoResultIsDimWithNoPriceAndNeverBest() {
        let tiles = SiteTile.make(
            rows: rows(), estimates: ItemEstimates(itemID: 1, sites: [estimate(1, low: nil, status: .noResult), estimate(2, low: 900)]), now: now)
        XCTAssertEqual(tiles[0].priceText, "なし")
        XCTAssertTrue(tiles[0].isDim)
        XCTAssertFalse(tiles[0].isBest)
        XCTAssertTrue(tiles[1].isBest)
    }

    func testFailedKeepsThePreviousValueWithAnAgeNote() {
        let old = now.addingTimeInterval(-2 * 86_400)
        let tiles = SiteTile.make(
            rows: rows(), estimates: ItemEstimates(itemID: 1, sites: [estimate(1, low: 2000, status: .failed, fetchedAt: old), estimate(2, low: nil, status: .failed)]),
            now: now)
        XCTAssertEqual(tiles[0].priceText, "¥2,000")
        XCTAssertEqual(tiles[0].ageNote, "最終取得: 2日前")
        XCTAssertFalse(tiles[0].isFailed)
        XCTAssertNil(tiles[1].priceText)
        XCTAssertTrue(tiles[1].isFailed)
    }

    func testWithoutEstimatesTilesHaveNoPrice() {
        let tiles = SiteTile.make(rows: rows(), estimates: nil, now: now)
        XCTAssertTrue(tiles.allSatisfy { $0.priceText == nil && !$0.isBest && !$0.isDim && !$0.isFailed })
    }
}
