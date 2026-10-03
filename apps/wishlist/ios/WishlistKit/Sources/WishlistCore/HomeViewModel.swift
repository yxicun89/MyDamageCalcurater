import Foundation
import Observation

/// ホーム(画像グリッド + ジャンルのチップ)の状態。
@MainActor
@Observable
public final class HomeViewModel {
    public private(set) var items: [Item] = []
    public private(set) var genres: [Genre] = []
    public private(set) var sites: [Site] = []
    /// nil は「すべて」
    public var selectedGenreID: Int?
    /// 取得に失敗して、保存済み(キャッシュ)を出しているとき true
    public private(set) var isStale = false
    /// 取得に失敗し、保存済みも無いときのメッセージ(白画面にしない)
    public private(set) var loadError: String?
    /// 削除などの操作の失敗(グリッドは変えない)
    public private(set) var errorMessage: String?

    private let service: any WishlistService
    private let cache: any WishlistCache
    private let settings: WishlistSettings

    public init(service: any WishlistService, cache: any WishlistCache, settings: WishlistSettings) {
        self.service = service
        self.cache = cache
        self.settings = settings
    }

    /// トークン・URL が未設定なら true(設定画面から始め、API を呼ばない)
    public var needsSettings: Bool { !settings.isConfigured }

    /// 選択中のジャンルで絞った商品。sortOrder 昇順、同順は id 降順(API の並びと同じ)。「すべて」は全件。
    public var visibleItems: [Item] {
        items
            .filter { selectedGenreID == nil || $0.genreID == selectedGenreID }
            .sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id > $1.id }
    }

    /// チップに並べるジャンル。sortOrder 昇順、同順は id 昇順。
    public var chipGenres: [Genre] {
        genres.sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }
    }

    /// キャッシュから即座に items・genres・sites を埋める(通信しない)。
    public func loadCached() async {
        // 3 つそろってから反映する(商品だけ先に出ると、詳細シートがサイト無しで開いてしまう)
        let savedItems = await cache.loadItems()
        let savedGenres = await cache.loadGenres()
        let savedSites = await cache.loadSites()
        if let savedItems { items = savedItems }
        if let savedGenres { genres = savedGenres }
        if let savedSites { sites = savedSites }
    }

    /// 一覧・ジャンル・サイトを取得する(3 つは独立。`WishlistRepository` で、取れたものだけ置き換え、取れなかったものはキャッシュを使う)。
    /// 成功した分はキャッシュへ保存する。`needsSettings` のときは何もしない(API を呼ばない)。
    /// 1 つでもキャッシュで代用したら `isStale = true`。取れず保存済みも無い種類があれば `loadError` を立てる。
    public func refresh() async {
        guard !needsSettings else { return }
        let repository = WishlistRepository(service: service, cache: cache)
        var stale = false
        var failure: String?
        var newItems: [Item]?
        var newGenres: [Genre]?
        var newSites: [Site]?
        do {
            let result = try await repository.items()
            newItems = result.data
            stale = stale || result.stale
        } catch { failure = errorText(error) }
        do {
            let result = try await repository.genres()
            newGenres = result.data
            stale = stale || result.stale
        } catch { failure = errorText(error) }
        do {
            let result = try await repository.sites()
            newSites = result.data
            stale = stale || result.stale
        } catch { failure = errorText(error) }
        // 3 つそろってから反映する
        if let newItems { items = newItems }
        if let newGenres { genres = newGenres }
        if let newSites { sites = newSites }
        isStale = stale
        loadError = failure
    }

    /// 登録・編集の結果を反映する(同じ id なら置き換え、無ければ追加)。キャッシュにも保存する。
    public func upsert(_ item: Item) async {
        if let index = items.firstIndex(where: { $0.id == item.id }) {
            items[index] = item
        } else {
            items.append(item)
        }
        await cache.saveItems(items)
    }

    /// 設定画面でジャンル・サイトを変えたあとに反映する(キャッシュにも保存する)。
    /// `setGenres` は、選択中のジャンルが無くなったら `selectedGenreID` を nil(すべて)に戻す。
    public func setGenres(_ genres: [Genre]) async {
        self.genres = genres
        if let id = selectedGenreID, !genres.contains(where: { $0.id == id }) { selectedGenreID = nil }
        await cache.saveGenres(genres)
    }
    public func setSites(_ sites: [Site]) async {
        self.sites = sites
        await cache.saveSites(sites)
    }

    /// 削除する。成功したら一覧とキャッシュから消して true。失敗したら `errorMessage` を立て、一覧は変えず false。
    public func delete(_ item: Item) async -> Bool {
        do {
            try await service.deleteItem(id: item.id)
        } catch {
            errorMessage = errorText(error)
            return false
        }
        errorMessage = nil
        items.removeAll { $0.id == item.id }
        await cache.saveItems(items)
        return true
    }
}
