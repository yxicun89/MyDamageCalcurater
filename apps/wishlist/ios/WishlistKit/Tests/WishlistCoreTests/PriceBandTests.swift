import XCTest

@testable import WishlistCore

// AC-IOS-5-3d: 価格の帯(web/src/lib/band.ts の移植)。docs/phase5-spec.md 5-2「詳細」。
final class PriceBandTests: XCTestCase {
    private func estimate(_ siteID: Int, low: Int?, status: EstimateStatus = .ok) -> SiteEstimate {
        SiteEstimate(siteID: siteID, low: low, mid: low, count: low == nil ? 0 : 5, suspiciousCount: 0, status: status, fetchedAt: Date(timeIntervalSince1970: 1_790_985_600))
    }

    /// AC-IOS-5-3d-1: 最安を 0、最高を 100 とした整数の位置。描画位置は position / 100
    func testPositionsSpanZeroToHundred() {
        let band = PriceBand.make(sites: T.sites, estimates: [estimate(1, low: 1000), estimate(2, low: 3000), estimate(3, low: 2000)])
        XCTAssertEqual(band.dots.map(\.siteID), [1, 2, 3], "sites の順")
        XCTAssertEqual(band.dots.map(\.position), [0, 100, 50])
        XCTAssertEqual(band.dots.map(\.fraction), [0.0, 1.0, 0.5])
        XCTAssertEqual(band.dots.map(\.low), [1000, 3000, 2000])
        XCTAssertEqual(band.rangeBar, 0.0...1.0, "安い〜中央の太線は帯の全幅")
    }

    /// AC-IOS-5-3d-2: 位置は四捨五入(1000, 1333, 3000 → 17)
    func testPositionIsRounded() {
        let band = PriceBand.make(sites: T.sites, estimates: [estimate(1, low: 1000), estimate(2, low: 1333), estimate(3, low: 3000)])
        XCTAssertEqual(band.dots.map(\.position), [0, 17, 100])
    }

    /// AC-IOS-5-3d-3: low が無いサイト(出品なし・link_only)は点にしない。取得失敗の前回値は点にする。sites に無いサイトは無視
    func testOnlySitesWithLowBecomeDots() {
        let band = PriceBand.make(
            sites: [T.mercari, T.amazon, T.other],
            estimates: [estimate(1, low: nil, status: .noResult), estimate(2, low: 4000, status: .failed), estimate(9, low: 1)])
        XCTAssertEqual(band.dots.map(\.siteID), [2])
    }

    /// AC-IOS-5-3d-4: 最安のサイト(同額は全部)
    func testBestSites() {
        let band = PriceBand.make(sites: T.sites, estimates: [estimate(1, low: 2000), estimate(2, low: 1500), estimate(3, low: 1500)])
        XCTAssertEqual(band.bestSiteIDs, [2, 3])
        let none = PriceBand.make(sites: T.sites, estimates: [])
        XCTAssertEqual(none.bestSiteIDs, [])
        XCTAssertEqual(none.dots, [])
        XCTAssertNil(none.rangeBar)
    }

    /// AC-IOS-5-3d-5: 全部同額(min == max)は position 0(Web と同じ契約)だが、描画は中央に置き、伸ばした太線は描かない
    func testEqualPricesCenterTheDotsAndDrawNoStretchedBar() {
        let band = PriceBand.make(sites: T.sites, estimates: [estimate(1, low: 3000), estimate(2, low: 3000)])
        XCTAssertEqual(band.dots.map(\.position), [0, 0])
        XCTAssertEqual(band.dots.map(\.fraction), [0.5, 0.5])
        XCTAssertNil(band.rangeBar)
        XCTAssertEqual(band.bestSiteIDs, [1, 2])
    }

    /// AC-IOS-5-3d-6: 点が 1 つだけでも同じ(中央・太線なし)
    func testSingleDotIsCenteredWithNoBar() {
        let band = PriceBand.make(sites: T.sites, estimates: [estimate(2, low: 4800)])
        XCTAssertEqual(band.dots.count, 1)
        XCTAssertEqual(band.dots.first?.position, 0)
        XCTAssertEqual(band.dots.first?.fraction, 0.5)
        XCTAssertNil(band.rangeBar)
    }
}
