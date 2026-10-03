import XCTest

@testable import WishlistCore

// AC-IOS-LIB-04: サイト行(web/src/lib/links.test.ts と同じ条件)。
final class SiteLinksTests: XCTestCase {
    private let mercari = Site(id: 1, name: "メルカリ", searchURLTemplate: "https://jp.mercari.com/search?keyword={q}")
    private let amazon = Site(id: 2, name: "Amazon", searchURLTemplate: "https://www.amazon.co.jp/s?k={q}")
    private let other = Site(id: 3, name: "その他", searchURLTemplate: "https://site3.example.com/s?q={q}")
    private var sites: [Site] { [mercari, amazon, other] }
    private let genre = Genre(id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", siteIDs: [2, 1])

    func testItemQueryFromTemplate() {
        XCTAssertEqual(SiteLinks.itemQuery(item: T.item(1, name: "グリス"), genre: genre), "S.H.Figuarts グリス")
    }

    func testItemQueryOverrideWins() {
        XCTAssertEqual(SiteLinks.itemQuery(item: T.item(1, name: "グリス", override: "グリス 上書き"), genre: genre), "グリス 上書き")
    }

    func testItemQueryIgnoresSiteOverrides() {
        let item = T.item(1, name: "グリス", siteOverrides: [SiteOverride(siteID: 1, query: "メルカリ用")])
        XCTAssertEqual(SiteLinks.itemQuery(item: item, genre: genre), "S.H.Figuarts グリス")
    }

    func testItemQueryWithoutGenreJoinsNameAndOption() {
        XCTAssertEqual(SiteLinks.itemQuery(item: T.item(1, name: "A", option: "B"), genre: nil), "A B")
    }

    func testLinksFollowGenreSiteOrderAndBuildURLs() {
        let links = SiteLinks.resolve(item: T.item(1, name: "グリス"), genre: genre, sites: sites)
        XCTAssertEqual(links.map(\.site.name), ["Amazon", "メルカリ"])
        XCTAssertEqual(
            links.dropFirst().first?.url.absoluteString,
            "https://jp.mercari.com/search?keyword=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9")
        XCTAssertEqual(links.dropFirst().first?.query, "S.H.Figuarts グリス")
    }

    func testSiteWithNonHTTPSearchURLIsDropped() {
        let bad = Site(id: 9, name: "悪い", searchURLTemplate: "javascript:alert('{q}')")
        var g = genre
        g.siteIDs = [9] + genre.siteIDs
        let links = SiteLinks.resolve(item: T.item(1, name: "グリス"), genre: g, sites: sites + [bad])
        XCTAssertFalse(links.map(\.site.id).contains(9))
        XCTAssertEqual(links.count, genre.siteIDs.count)
    }

    func testDisabledSiteIsDropped() {
        let item = T.item(1, name: "グリス", siteOverrides: [SiteOverride(siteID: 2, enabled: false)])
        XCTAssertEqual(SiteLinks.resolve(item: item, genre: genre, sites: sites).map(\.site.id), [1])
    }

    func testQueryPrecedenceSiteQueryThenOverrideThenTemplate() {
        let item = T.item(
            1, name: "グリス", override: "上書き",
            siteOverrides: [SiteOverride(siteID: 1, query: "メルカリ用", enabled: true)])
        let links = SiteLinks.resolve(item: item, genre: genre, sites: sites)
        XCTAssertEqual(links.first { $0.site.id == 1 }?.query, "メルカリ用")
        XCTAssertEqual(links.first { $0.site.id == 2 }?.query, "上書き")
    }

    func testBlankSiteQueryFallsBackToOverride() {
        let item = T.item(1, name: "グリス", override: "上書き", siteOverrides: [SiteOverride(siteID: 1, query: "  ", enabled: true)])
        XCTAssertEqual(SiteLinks.resolve(item: item, genre: genre, sites: sites).first { $0.site.id == 1 }?.query, "上書き")
    }

    func testUnknownSiteIDsAreSkipped() {
        var g = genre
        g.siteIDs = [99, 1]
        XCTAssertEqual(SiteLinks.resolve(item: T.item(1), genre: g, sites: sites).map(\.site.id), [1])
    }

    func testNoGenreGivesNoLinks() {
        XCTAssertEqual(SiteLinks.resolve(item: T.item(1), genre: nil, sites: sites), [])
    }

    /// 空の option は空白を詰めてから URL にする(仕様 §4)。
    func testOptionlessItemHasNoTrailingSpaceInQuery() {
        let duema = Genre(id: 2, name: "デュエマ", queryTemplate: "{name} {option}", siteIDs: [1])
        let links = SiteLinks.resolve(item: T.item(1, genre: 2, name: "ボルシャック"), genre: duema, sites: sites)
        XCTAssertEqual(links.first?.query, "ボルシャック")
        XCTAssertEqual(links.first?.url.absoluteString, "https://jp.mercari.com/search?keyword=%E3%83%9C%E3%83%AB%E3%82%B7%E3%83%A3%E3%83%83%E3%82%AF")
    }
}
