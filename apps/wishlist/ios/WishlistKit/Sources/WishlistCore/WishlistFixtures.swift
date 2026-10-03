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
}
