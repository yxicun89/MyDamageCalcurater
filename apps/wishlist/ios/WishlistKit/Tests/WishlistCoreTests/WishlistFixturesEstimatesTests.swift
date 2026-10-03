import XCTest

@testable import WishlistCore

// AC-IOS-EST-10: 目安価格つきのモック起動(`WISHLIST_USE_FAKE=estimates`)用フィクスチャ。XCUITest が値に依存する。
final class WishlistFixturesEstimatesTests: XCTestCase {
    func testItem12HasAnOkSiteAndANoResultSite() throws {
        let value = try XCTUnwrap(WishlistFixtures.estimates[12])
        XCTAssertEqual(value.summaryLow, 3000)
        XCTAssertEqual(value.summaryMid, 4500)
        XCTAssertEqual(value.summaryFetchedAt, Date(timeIntervalSince1970: 1_790_987_400), "JST 10/3 9:30")
        XCTAssertFalse(value.refreshing)
        XCTAssertEqual(
            value.sites.map(\.siteID), [1, 2])
        let mercari = try XCTUnwrap(value.sites.first { $0.siteID == 1 })
        XCTAssertEqual([mercari.low, mercari.mid], [3000, 4500])
        XCTAssertEqual([mercari.count, mercari.suspiciousCount, mercari.inStockCount], [5, 1, 4])
        XCTAssertEqual(mercari.status, .ok)
        let amazon = try XCTUnwrap(value.sites.first { $0.siteID == 2 })
        XCTAssertEqual(amazon.status, .noResult)
        XCTAssertNil(amazon.low)
    }

    func testItem11IsAllNoResult() throws {
        let value = try XCTUnwrap(WishlistFixtures.estimates[11])
        XCTAssertNil(value.summaryLow)
        XCTAssertEqual(value.sites.map(\.status), [.noResult, .noResult])
    }

    func testListingsHaveOneSuspiciousListingForItem12() {
        let suspicious = WishlistFixtures.listings.filter { !$0.suspiciousReasons.isEmpty }
        XCTAssertEqual(suspicious.map(\.id), [102])
        XCTAssertEqual(suspicious.first?.suspiciousReasons, [.titleMismatch, .tooCheap])
        XCTAssertEqual(suspicious.first?.siteID, 1)
        XCTAssertEqual(WishlistFixtures.listings.count, 2)
    }

    /// 参照整合: estimates のサイトは sites に、listings のサイトも sites にある。
    func testReferentialIntegrity() {
        let siteIDs = Set(WishlistFixtures.sites.map(\.id))
        for value in WishlistFixtures.estimates.values {
            XCTAssertTrue(value.sites.allSatisfy { siteIDs.contains($0.siteID) })
        }
        XCTAssertTrue(WishlistFixtures.listings.allSatisfy { siteIDs.contains($0.siteID) })
    }

    func testMakeServiceWithEstimatesServesThem() async throws {
        let service = WishlistFixtures.makeServiceWithEstimates()
        let value = try await service.estimates(itemID: 12)
        XCTAssertEqual(value, WishlistFixtures.estimates[12])
        let listings = try await service.listings(itemID: 12, siteID: nil)
        XCTAssertEqual(listings.map(\.id), [102, 101])
        let items = try await service.listItems()
        XCTAssertEqual(items, WishlistFixtures.items)
    }
}
