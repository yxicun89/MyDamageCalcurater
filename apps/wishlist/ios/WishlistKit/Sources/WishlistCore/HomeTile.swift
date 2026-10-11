import Foundation

// フェーズ5-3 ホームのタイル(docs/phase5-spec.md 5-2「ホーム」・5-3「決めたこと」)。

/// タイル右下の価格の札。`nil`(札なし)は目安が none / failed / 無しのとき、またはオフライン中。
public enum PriceTag: Equatable, Sendable {
    /// `¥880〜`(`PriceFormat.yen(low) + "〜"`)
    case price(String)
    /// 「出品なし」(summary.status が no_result)
    case noListing
}

/// ホームの 1 タイル。ジャンルの色・絵柄と価格の札を持つ(枠の色=ジャンル色)。
public struct HomeTile: Equatable, Sendable, Identifiable {
    public var item: Item
    /// `#rrggbb`。商品のジャンルが `genres` に無ければ `GenreIcon.defaultColor`
    public var genreColor: String
    /// 商品のジャンルが `genres` に無ければ `.box`
    public var genreIcon: GenreIcon
    public var priceTag: PriceTag?

    public var id: Int { item.id }

    public init(item: Item, genreColor: String, genreIcon: GenreIcon, priceTag: PriceTag?) {
        self.item = item
        self.genreColor = genreColor
        self.genreIcon = genreIcon
        self.priceTag = priceTag
    }

    /// `item.summary`: status ok で low あり → `.price("¥low〜")` / noResult → `.noListing` / それ以外・nil → nil。
    /// `isOffline` なら常に nil(オフライン中は価格の札を消す)。
    public static func make(item: Item, genres: [Genre], isOffline: Bool) -> HomeTile {
        let genre = genres.first { $0.id == item.genreID }
        return HomeTile(
            item: item, genreColor: genre?.color ?? GenreIcon.defaultColor, genreIcon: genre?.icon ?? .box,
            priceTag: isOffline ? nil : priceTag(for: item.summary))
    }

    private static func priceTag(for summary: ItemSummary?) -> PriceTag? {
        guard let summary else { return nil }
        switch summary.status {
        case .ok: return summary.low.map { .price(PriceFormat.yen($0) + "〜") }
        case .noResult: return .noListing
        case .failed, .none: return nil
        }
    }
}
