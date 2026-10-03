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

    func listGenres() async throws -> [Genre]
    func createGenre(_ body: GenreCreate) async throws -> Genre
    func updateGenre(id: Int, patch: GenreUpdate) async throws -> Genre
    func listSites() async throws -> [Site]
    func createSite(_ body: SiteCreate) async throws -> Site
    func updateSite(id: Int, patch: SiteUpdate) async throws -> Site
}
