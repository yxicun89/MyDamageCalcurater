import Foundation

/// 詳細の「サイトのタイル」1 つ分(docs/phase5-spec.md 5-2「詳細」)。マーク・価格・最安の強調・出品なしの薄さ・取得失敗を持つ。
public struct SiteTile: Equatable, Sendable, Identifiable {
    public var row: SiteRow
    /// マークの 1 文字と地の色(`SiteMark`)
    public var mark: String
    public var colorHex: String
    /// `¥3,000`(low がある。取得失敗の前回値も)/ `なし`(no_result)/ nil(価格なし)
    public var priceText: String?
    /// no_result のタイルは薄く出す
    public var isDim: Bool
    /// 取得に失敗して前回値も無い
    public var isFailed: Bool
    /// low が最小のタイル(同額は全部)
    public var isBest: Bool
    /// 失敗した前回値に付ける `最終取得: 2日前`(同じ日は `最終取得: 今日`)
    public var ageNote: String?

    public var id: Int { row.id }
    public var url: URL { row.link.url }
    public var name: String { row.link.site.name }
    public var accessibilityLabel: String { row.accessibilityLabel }

    /// `rows`(ジャンルの表示順)と `estimates` から組む。
    public static func make(rows: [SiteRow], estimates: ItemEstimates?, now: Date) -> [SiteTile] {
        let band = PriceBand.make(sites: rows.map(\.link.site), estimates: estimates?.sites ?? [])
        return rows.map { row in
            let site = row.link.site
            let estimate = estimates?.sites.first { $0.siteID == site.id }
            var tile = SiteTile(
                row: row, mark: SiteMark.mark(for: site.name), colorHex: SiteMark.colorHex(for: site.name), priceText: nil,
                isDim: false, isFailed: false, isBest: band.bestSiteIDs.contains(site.id), ageNote: nil)
            guard let estimate else { return tile }
            switch estimate.status {
            case .noResult:
                tile.priceText = "なし"
                tile.isDim = true
            case .ok, .failed:
                if let low = estimate.low {
                    tile.priceText = PriceFormat.yen(low)
                    if estimate.status == .failed { tile.ageNote = "最終取得: " + PriceFormat.age(of: estimate.fetchedAt, now: now) }
                } else {
                    tile.isFailed = estimate.status == .failed
                }
            }
            return tile
        }
    }
}
