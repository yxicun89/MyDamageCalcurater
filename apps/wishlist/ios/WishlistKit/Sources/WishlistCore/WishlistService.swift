import Foundation

/// API(api/openapi.yaml)の操作。ViewModel はこのプロトコルだけに依存する。
/// 実装は APIWishlistService(生成クライアント)と FakeWishlistService(メモリ上。テスト・プレビュー・XCUITest 用)。
/// 失敗は `WishlistError` で投げる。
public protocol WishlistService: Sendable {
    func listItems() async throws -> [Item]
    /// multipart(`image` にファイル)で登録
    func createItem(_ fields: ItemFields, image: ImageUpload) async throws -> Item
    /// JSON(`image_url` をサーバーが取得して保存)で登録
    func createItem(_ fields: ItemFields, imageURL: String) async throws -> Item
    func draftFromURL(_ url: String, genreID: Int?) async throws -> ItemDraft
    func updateItem(id: Int, patch: ItemPatch) async throws -> Item
    func deleteItem(id: Int) async throws
    func replaceItemImage(id: Int, image: ImageUpload) async throws -> Item
    func estimates(itemID: Int) async throws -> ItemEstimates
    /// `POST /api/items/{id}/estimates/refresh`(202。本文は GET と同じ形で、取得できる対象があれば `refreshing: true`)
    func refreshEstimates(itemID: Int) async throws -> ItemEstimates
    /// `GET /api/items/{id}/listings`(参考外も含む。`siteID` を渡すとそのサイトだけ)
    func listings(itemID: Int, siteID: Int?) async throws -> [Listing]
    /// `GET /api/items/{id}/price-history`(フェーズ4-2)。`days` が nil ならクエリを付けない(サーバーの既定 90 日)
    func priceHistory(itemID: Int, days: Int?) async throws -> PriceHistory

    func listGenres() async throws -> [Genre]
    func createGenre(_ body: GenreCreate) async throws -> Genre
    func updateGenre(id: Int, patch: GenreUpdate) async throws -> Genre
    func listSites() async throws -> [Site]
    func createSite(_ body: SiteCreate) async throws -> Site
    func updateSite(id: Int, patch: SiteUpdate) async throws -> Site
}
