import Foundation
import Synchronization

/// メモリ上の `WishlistService`(ViewModel のテスト・SwiftUI プレビュー・XCUITest のモック起動用)。
/// spec-writer が置いた完成品。ViewModel のテストはこれを前提にするので、挙動を変えるときはテストも見直す。
///
/// - 呼び出しを `calls` に順に記録する(失敗した呼び出しも記録する)。
/// - `offline = true` なら全呼び出しが `.network` で失敗。`failure` を入れると全呼び出しがそのエラーで失敗(例: 401)。
/// - `failOnce(_:)` は次の 1 回だけ失敗させる。
/// - 変更系はメモリ上の一覧に反映し、API と同じ形(更新後の項目)を返す。
public final class FakeWishlistService: WishlistService {
    public enum Call: Equatable, Sendable {
        case listItems
        case listGenres
        case listSites
        case createItemMultipart(ItemFields, filename: String)
        case createItemFromURL(ItemFields, imageURL: String)
        case draftFromURL(String, genreID: Int?)
        case updateItem(id: Int, patch: ItemPatch)
        case deleteItem(id: Int)
        case replaceItemImage(id: Int, filename: String)
        case estimates(itemID: Int)
        case refreshEstimates(itemID: Int)
        case listings(itemID: Int, siteID: Int?)
        case priceHistory(itemID: Int, days: Int?)
        case createGenre(GenreCreate)
        case updateGenre(id: Int, patch: GenreUpdate)
        case createSite(SiteCreate)
        case updateSite(id: Int, patch: SiteUpdate)
    }

    private struct State {
        var items: [Item]
        var genres: [Genre]
        var sites: [Site]
        var calls: [Call] = []
        var offline = false
        var failure: WishlistError?
        var failOnce: WishlistError?
        var draft: ItemDraft?
        /// 商品ごとの GET estimates の応答の列。呼ぶたびに先頭を返し、残りが 1 つになったらそれを返し続ける
        var estimates: [Int: [ItemEstimates]] = [:]
        var refreshResponses: [Int: ItemEstimates] = [:]
        var listings: [Listing] = []
        var priceHistories: [Int: PriceHistory] = [:]
        var gate: (@Sendable (Call) async -> Void)?
        var nextItemID: Int
        var nextGenreID: Int
        var nextSiteID: Int
    }

    private let state: Mutex<State>

    public init(items: [Item] = [], genres: [Genre] = [], sites: [Site] = []) {
        state = Mutex(State(
            items: items, genres: genres, sites: sites,
            nextItemID: (items.map(\.id).max() ?? 0) + 1,
            nextGenreID: (genres.map(\.id).max() ?? 0) + 1,
            nextSiteID: (sites.map(\.id).max() ?? 0) + 1))
    }

    // MARK: - テストからの操作・検査

    public var calls: [Call] { state.withLock { $0.calls } }
    public var items: [Item] { state.withLock { $0.items } }
    public var genres: [Genre] { state.withLock { $0.genres } }
    public var sites: [Site] { state.withLock { $0.sites } }

    public var offline: Bool {
        get { state.withLock { $0.offline } }
        set { state.withLock { $0.offline = newValue } }
    }
    public var failure: WishlistError? {
        get { state.withLock { $0.failure } }
        set { state.withLock { $0.failure = newValue } }
    }
    /// `from-url` が返す下書き(nil なら 422 を投げる)
    public var draft: ItemDraft? {
        get { state.withLock { $0.draft } }
        set { state.withLock { $0.draft = newValue } }
    }
    public func failOnce(_ error: WishlistError) { state.withLock { $0.failOnce = error } }
    public func setEstimates(_ estimates: ItemEstimates) { state.withLock { $0.estimates[estimates.itemID] = [estimates] } }
    /// GET estimates の応答を順に返す(最後の 1 つは返し続ける)。ポーリングのテスト用。空なら何もしない。
    public func setEstimatesSequence(_ sequence: [ItemEstimates]) {
        guard let first = sequence.first else { return }
        state.withLock { $0.estimates[first.itemID] = sequence }
    }
    /// POST refresh の本文(未設定なら、GET の先頭の応答に `refreshing: true` を付けたもの)
    public func setRefreshResponse(_ estimates: ItemEstimates) { state.withLock { $0.refreshResponses[estimates.itemID] = estimates } }
    /// GET listings が返す出品(全商品分。`itemID` ごとに分けず、`siteID` だけで絞る。並びはそのまま)
    public func setListings(_ listings: [Listing]) { state.withLock { $0.listings = listings } }
    /// GET price-history の応答(商品ごと)。未設定の商品は推移なし(`days` は要求の値、nil なら 90)
    public func setPriceHistory(_ history: PriceHistory) { state.withLock { $0.priceHistories[history.itemID] = history } }
    /// estimates・refreshEstimates・listings・priceHistory の応答を返す直前に呼ばれる(応答は呼び出し時点で決まる)。
    /// 応答を保留して順序の入れ替わり(古い応答)をテストするために使う。
    public var gate: (@Sendable (Call) async -> Void)? {
        get { state.withLock { $0.gate } }
        set { state.withLock { $0.gate = newValue } }
    }
    public func replaceStoredItems(_ items: [Item]) { state.withLock { $0.items = items } }

    /// 呼び出しを記録し、失敗の設定があれば投げる。
    private func enter(_ call: Call) throws {
        let error: WishlistError? = state.withLock { s in
            s.calls.append(call)
            if s.offline { return WishlistError(code: .network, message: "offline (fake)") }
            if let once = s.failOnce {
                s.failOnce = nil
                return once
            }
            return s.failure
        }
        if let error { throw error }
    }

    private static func notFound() -> WishlistError { WishlistError(code: .notFound, status: 404, message: "not found (fake)") }

    // MARK: - WishlistService

    public func listItems() async throws -> [Item] {
        try enter(.listItems)
        return state.withLock { $0.items }
    }

    public func listGenres() async throws -> [Genre] {
        try enter(.listGenres)
        return state.withLock { $0.genres }
    }

    public func listSites() async throws -> [Site] {
        try enter(.listSites)
        return state.withLock { $0.sites }
    }

    private func append(_ fields: ItemFields, imagePath: String) -> Item {
        state.withLock { s in
            let id = s.nextItemID
            s.nextItemID += 1
            let item = Item(
                id: id, genreID: fields.genreID, name: fields.name, optionText: fields.optionText,
                queryOverride: fields.queryOverride, imageURLPath: imagePath, sourceURL: fields.sourceURL,
                minPrice: fields.minPrice, sortOrder: fields.sortOrder ?? 0)
            s.items.append(item)
            return item
        }
    }

    public func createItem(_ fields: ItemFields, image: ImageUpload) async throws -> Item {
        try enter(.createItemMultipart(fields, filename: image.filename))
        return append(fields, imagePath: "images/00000000-0000-4000-8000-\(String(format: "%012d", state.withLock { $0.nextItemID })).png")
    }

    public func createItem(_ fields: ItemFields, imageURL: String) async throws -> Item {
        try enter(.createItemFromURL(fields, imageURL: imageURL))
        return append(fields, imagePath: "images/00000000-0000-4000-8000-\(String(format: "%012d", state.withLock { $0.nextItemID })).png")
    }

    public func draftFromURL(_ url: String, genreID: Int?) async throws -> ItemDraft {
        try enter(.draftFromURL(url, genreID: genreID))
        guard let draft = state.withLock({ $0.draft }) else {
            throw WishlistError(code: .unprocessable, status: 422, message: "no draft (fake)")
        }
        return draft
    }

    public func updateItem(id: Int, patch: ItemPatch) async throws -> Item {
        try enter(.updateItem(id: id, patch: patch))
        return try state.withLock { s in
            guard let index = s.items.firstIndex(where: { $0.id == id }) else { throw Self.notFound() }
            var item = s.items[index]
            if let v = patch.genreID { item.genreID = v }
            if let v = patch.name { item.name = v }
            Self.apply(patch.optionText, to: &item.optionText)
            Self.apply(patch.queryOverride, to: &item.queryOverride)
            Self.apply(patch.sourceURL, to: &item.sourceURL)
            Self.apply(patch.minPrice, to: &item.minPrice)
            if let v = patch.sortOrder { item.sortOrder = v }
            if let v = patch.siteOverrides { item.siteOverrides = v }
            if let v = patch.watchOfficial { item.watchOfficial = v }
            s.items[index] = item
            return item
        }
    }

    private static func apply<V>(_ update: FieldUpdate<V>, to field: inout V?) {
        switch update {
        case .keep: break
        case .set(let v): field = v
        case .clear: field = nil
        }
    }

    public func deleteItem(id: Int) async throws {
        try enter(.deleteItem(id: id))
        try state.withLock { s in
            guard s.items.contains(where: { $0.id == id }) else { throw Self.notFound() }
            s.items.removeAll { $0.id == id }
        }
    }

    public func replaceItemImage(id: Int, image: ImageUpload) async throws -> Item {
        try enter(.replaceItemImage(id: id, filename: image.filename))
        return try state.withLock { s in
            guard let index = s.items.firstIndex(where: { $0.id == id }) else { throw Self.notFound() }
            s.items[index].imageURLPath = "images/11111111-1111-4111-8111-111111111111.png"
            return s.items[index]
        }
    }

    public func estimates(itemID: Int) async throws -> ItemEstimates {
        try enter(.estimates(itemID: itemID))
        let response = state.withLock { s -> ItemEstimates in
            guard var sequence = s.estimates[itemID], let head = sequence.first else { return ItemEstimates(itemID: itemID) }
            if sequence.count > 1 {
                sequence.removeFirst()
                s.estimates[itemID] = sequence
            }
            return head
        }
        await state.withLock { $0.gate }?(.estimates(itemID: itemID))
        return response
    }

    public func refreshEstimates(itemID: Int) async throws -> ItemEstimates {
        try enter(.refreshEstimates(itemID: itemID))
        let response = state.withLock { s -> ItemEstimates in
            if let set = s.refreshResponses[itemID] { return set }
            var base = s.estimates[itemID]?.first ?? ItemEstimates(itemID: itemID)
            base.refreshing = true
            return base
        }
        await state.withLock { $0.gate }?(.refreshEstimates(itemID: itemID))
        return response
    }

    public func listings(itemID: Int, siteID: Int?) async throws -> [Listing] {
        try enter(.listings(itemID: itemID, siteID: siteID))
        let response = state.withLock { s in s.listings.filter { siteID == nil || $0.siteID == siteID } }
        await state.withLock { $0.gate }?(.listings(itemID: itemID, siteID: siteID))
        return response
    }

    public func priceHistory(itemID: Int, days: Int?) async throws -> PriceHistory {
        try enter(.priceHistory(itemID: itemID, days: days))
        let response = state.withLock { s in s.priceHistories[itemID] ?? PriceHistory(itemID: itemID, days: days ?? 90) }
        await state.withLock { $0.gate }?(.priceHistory(itemID: itemID, days: days))
        return response
    }

    public func createGenre(_ body: GenreCreate) async throws -> Genre {
        try enter(.createGenre(body))
        return state.withLock { s in
            let genre = Genre(
                id: s.nextGenreID, name: body.name, queryTemplate: body.queryTemplate ?? "{name} {option}",
                sortOrder: body.sortOrder ?? 0, siteIDs: body.siteIDs ?? [], aliases: body.aliases ?? [])
            s.nextGenreID += 1
            s.genres.append(genre)
            return genre
        }
    }

    public func updateGenre(id: Int, patch: GenreUpdate) async throws -> Genre {
        try enter(.updateGenre(id: id, patch: patch))
        return try state.withLock { s in
            guard let index = s.genres.firstIndex(where: { $0.id == id }) else { throw Self.notFound() }
            if let v = patch.name { s.genres[index].name = v }
            if let v = patch.queryTemplate { s.genres[index].queryTemplate = v }
            if let v = patch.sortOrder { s.genres[index].sortOrder = v }
            if let v = patch.siteIDs { s.genres[index].siteIDs = v }
            if let v = patch.aliases { s.genres[index].aliases = v }
            return s.genres[index]
        }
    }

    public func createSite(_ body: SiteCreate) async throws -> Site {
        try enter(.createSite(body))
        return state.withLock { s in
            let site = Site(
                id: s.nextSiteID, name: body.name, searchURLTemplate: body.searchURLTemplate,
                fetchType: body.fetchType ?? .linkOnly, isReference: body.isReference ?? false)
            s.nextSiteID += 1
            s.sites.append(site)
            return site
        }
    }

    public func updateSite(id: Int, patch: SiteUpdate) async throws -> Site {
        try enter(.updateSite(id: id, patch: patch))
        return try state.withLock { s in
            guard let index = s.sites.firstIndex(where: { $0.id == id }) else { throw Self.notFound() }
            if let v = patch.name { s.sites[index].name = v }
            if let v = patch.searchURLTemplate { s.sites[index].searchURLTemplate = v }
            if let v = patch.fetchType { s.sites[index].fetchType = v }
            if let v = patch.isReference { s.sites[index].isReference = v }
            return s.sites[index]
        }
    }
}
