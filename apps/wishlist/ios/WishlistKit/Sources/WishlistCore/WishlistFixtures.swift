import Foundation

/// 架空のデータ(モック起動・プレビュー・XCUITest 用。`FakeWishlistService` / `InMemoryWishlistCache` に入れる)。
/// 内容は `WishlistFixturesTests` が固定する(XCUITest がこの名前・並びに依存するため)。
public enum WishlistFixtures {
    public static var sites: [Site] {
        [
            Site(
                id: 1, name: "メルカリ", searchURLTemplate: "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc",
                fetchType: .headless),
            Site(id: 2, name: "Amazon", searchURLTemplate: "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank", fetchType: .linkOnly),
        ]
    }

    public static var genres: [Genre] {
        [
            Genre(id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", sortOrder: 1, siteIDs: [1, 2]),
            Genre(id: 2, name: "デュエマ", queryTemplate: "{name} {option}", sortOrder: 2, siteIDs: [2, 1]),
        ]
    }

    public static var items: [Item] {
        [
            Item(id: 12, genreID: 1, name: "グリス", imageURLPath: "images/00000000-0000-4000-8000-000000000012.png"),
            Item(
                id: 11, genreID: 2, name: "ボルシャック", optionText: "銀トレジャー",
                imageURLPath: "images/00000000-0000-4000-8000-000000000011.png"),
        ]
    }

    /// 目安価格のフィクスチャの取得時刻(JST 2026-10-03 9:30 → サマリは `10/3 時点`)
    public static let estimateFetchedAt = Date(timeIntervalSince1970: 1_790_987_400)

    /// 商品ごとの estimates(`WISHLIST_USE_FAKE=estimates` の XCUITest が依存する。架空の値)。
    /// - 12 グリス: メルカリ(1)= ok ¥3,000〜¥4,500・5 件・在庫あり・参考外 1 件 / Amazon(2)= no_result(架空。実際の Amazon は link_only で目安を持たない)
    /// - 11 ボルシャック: メルカリ(1)・Amazon(2)ともに no_result(→ サマリは「出品ないかも」)
    public static var estimates: [Int: ItemEstimates] {
        [
            12: ItemEstimates(
                itemID: 12, summaryLow: 3000, summaryMid: 4500, summaryFetchedAt: estimateFetchedAt,
                sites: [
                    SiteEstimate(
                        siteID: 1, low: 3000, mid: 4500, count: 5, suspiciousCount: 1, inStockCount: 4, status: .ok,
                        fetchedAt: estimateFetchedAt),
                    SiteEstimate(siteID: 2, count: 0, suspiciousCount: 0, status: .noResult, fetchedAt: estimateFetchedAt),
                ]),
            11: ItemEstimates(
                itemID: 11,
                sites: [
                    SiteEstimate(siteID: 1, count: 0, suspiciousCount: 0, status: .noResult, fetchedAt: estimateFetchedAt),
                    SiteEstimate(siteID: 2, count: 0, suspiciousCount: 0, status: .noResult, fetchedAt: estimateFetchedAt),
                ]),
        ]
    }

    /// 商品 12 の出品(id 101 は参考にしている出品、id 102 が参考外 = 参考外の折りたたみに出る)
    public static var listings: [Listing] {
        [
            Listing(
                id: 102, siteID: 1, title: "グリス 変身ベルト(ジャンク)", price: 300, url: "https://item.example.com/102",
                imageURL: "https://img.example.com/102.jpg", suspiciousReasons: [.titleMismatch, .tooCheap],
                fetchedAt: estimateFetchedAt),
            Listing(
                id: 101, siteID: 1, title: "S.H.Figuarts グリス", price: 3000, url: "https://item.example.com/101",
                fetchedAt: estimateFetchedAt),
        ]
    }

    /// 価格の推移(フェーズ4-2。`WISHLIST_USE_FAKE=estimates` の XCUITest が依存する。架空の値)。
    /// - 12 グリス: メルカリ(1)だけ 3 日分(10/1 ¥3,200・10/2 ¥3,000・10/3 ¥3,000。10/3 は estimates と同じ)
    ///   → 要約 `価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,200`
    /// - 11 ボルシャック: 推移なし(→ `推移はまだありません`)
    public static var priceHistories: [Int: PriceHistory] {
        [
            12: PriceHistory(
                itemID: 12,
                sites: [
                    SitePriceHistory(
                        siteID: 1,
                        points: [
                            PricePoint(day: "2026-10-01", low: 3200, mid: 3600),
                            PricePoint(day: "2026-10-02", low: 3000, mid: 4000),
                            PricePoint(day: "2026-10-03", low: 3000, mid: 4500),
                        ])
                ],
                overall: [
                    DayLow(day: "2026-10-01", low: 3200),
                    DayLow(day: "2026-10-02", low: 3000),
                    DayLow(day: "2026-10-03", low: 3000),
                ]),
            11: PriceHistory(itemID: 11),
        ]
    }

    /// 目安価格つきのモック(`WISHLIST_USE_FAKE=estimates`)。商品・ジャンル・サイトは `items` / `genres` / `sites` と同じ。
    public static func makeServiceWithEstimates() -> FakeWishlistService {
        let service = FakeWishlistService(items: items, genres: genres, sites: sites)
        for value in estimates.values { service.setEstimates(value) }
        service.setListings(listings)
        for value in priceHistories.values { service.setPriceHistory(value) }
        return service
    }
}
