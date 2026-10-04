import XCTest

@testable import WishlistCore

// AC-IOS-HIS-09: 価格の推移のモック用フィクスチャ(`WISHLIST_USE_FAKE=estimates`)。XCUITest が値に依存する。
final class WishlistFixturesPriceHistoryTests: XCTestCase {
    func testItem12HasThreeDaysOfMercari() throws {
        let value = try XCTUnwrap(WishlistFixtures.priceHistories[12])
        XCTAssertEqual(value.overall, [DayLow(day: "2026-10-01", low: 3200), DayLow(day: "2026-10-02", low: 3000), DayLow(day: "2026-10-03", low: 3000)])
        XCTAssertEqual(value.sites.map(\.siteID), [1])
        XCTAssertEqual(value.sites.first?.points.map(\.low), [3200, 3000, 3000])
        // 10/3 は estimates(メルカリ ok ¥3,000〜¥4,500)と同じ値
        XCTAssertEqual(value.sites.first?.points.last, PricePoint(day: "2026-10-03", low: 3000, mid: 4500))
        XCTAssertEqual(WishlistFixtures.estimates[12]?.sites.first { $0.siteID == 1 }?.low, 3000)
    }

    func testItem11HasNoHistory() throws {
        let value = try XCTUnwrap(WishlistFixtures.priceHistories[11])
        XCTAssertTrue(value.sites.isEmpty)
        XCTAssertTrue(value.overall.isEmpty)
    }

    func testMakeServiceWithEstimatesServesTheHistory() async throws {
        let service = WishlistFixtures.makeServiceWithEstimates()
        let got = try await service.priceHistory(itemID: 12, days: nil)
        XCTAssertEqual(got, WishlistFixtures.priceHistories[12])
        let siteIDs = Set(WishlistFixtures.sites.map(\.id))
        for history in WishlistFixtures.priceHistories.values {
            XCTAssertTrue(history.sites.allSatisfy { siteIDs.contains($0.siteID) }, "参照整合")
        }
    }
}
