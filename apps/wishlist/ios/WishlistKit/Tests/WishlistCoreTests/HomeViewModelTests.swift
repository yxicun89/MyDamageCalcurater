import XCTest

@testable import WishlistCore

// AC-IOS-VM-HOME-01〜09: ホームの状態(ジャンル絞り込み・並び・オフライン・削除)。fake の Service と in-memory のキャッシュでテストする。
@MainActor
final class HomeViewModelTests: XCTestCase {
    private func makeViewModel(
        service: FakeWishlistService? = nil, cache: InMemoryWishlistCache? = nil, settings: WishlistSettings = T.configured
    ) -> (HomeViewModel, FakeWishlistService, InMemoryWishlistCache) {
        let service = service ?? T.fake()
        let cache = cache ?? InMemoryWishlistCache()
        return (HomeViewModel(service: service, cache: cache, settings: settings), service, cache)
    }

    // MARK: - 取得とキャッシュ

    func testRefreshLoadsAllThreeListsAndSavesThem() async {
        let (viewModel, _, cache) = makeViewModel()
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, T.items)
        XCTAssertEqual(viewModel.genres, T.genres)
        XCTAssertEqual(viewModel.sites, T.sites)
        XCTAssertFalse(viewModel.isStale)
        XCTAssertNil(viewModel.loadError)
        let savedItems = await cache.loadItems()
        let savedGenres = await cache.loadGenres()
        let savedSites = await cache.loadSites()
        XCTAssertEqual(savedItems, T.items)
        XCTAssertEqual(savedGenres, T.genres)
        XCTAssertEqual(savedSites, T.sites)
    }

    /// 起動時はキャッシュを即出す(通信しない)。
    func testLoadCachedFillsFromCacheWithoutCallingTheService() async {
        let (viewModel, service, _) = makeViewModel(cache: InMemoryWishlistCache(items: T.items, genres: T.genres, sites: T.sites))
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.items, T.items)
        XCTAssertEqual(viewModel.genres, T.genres)
        XCTAssertEqual(viewModel.sites, T.sites)
        XCTAssertEqual(service.calls, [])
    }

    func testLoadCachedWithEmptyCacheLeavesEverythingEmpty() async {
        let (viewModel, _, _) = makeViewModel()
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.items, [])
        XCTAssertEqual(viewModel.genres, [])
        XCTAssertEqual(viewModel.sites, [])
    }

    /// AC-OFF-01/03: API が落ちていても(401 でも)保存済みの一覧が出る。stale を立て、エラー画面にはしない。
    func testRefreshFailureKeepsCachedDataAndMarksStale() async {
        for failure in [WishlistError(code: .network, message: "offline"), T.unauthorized()] {
            let service = T.fake()
            service.failure = failure
            let (viewModel, _, _) = makeViewModel(
                service: service, cache: InMemoryWishlistCache(items: T.items, genres: T.genres, sites: T.sites))
            await viewModel.loadCached()
            await viewModel.refresh()
            XCTAssertEqual(viewModel.items, T.items, "\(failure.code)")
            XCTAssertEqual(viewModel.genres, T.genres)
            XCTAssertTrue(viewModel.isStale)
            XCTAssertNil(viewModel.loadError)
        }
    }

    /// AC-OFF-04: 保存済みも無ければ、白画面にせず理由を出す。
    func testRefreshFailureWithoutCacheSetsLoadError() async {
        let (viewModel, _, _) = makeViewModel(service: T.fake(offline: true))
        await viewModel.refresh()
        XCTAssertNotNil(viewModel.loadError)
        XCTAssertEqual(viewModel.items, [])
    }

    /// AC-OFF-05: オンラインに戻ると最新に置き換わり、stale が消える。
    func testBackOnlineReplacesCachedDataAndClearsStale() async {
        let service = T.fake(offline: true)
        let (viewModel, _, cache) = makeViewModel(service: service, cache: InMemoryWishlistCache(items: [T.item(1, name: "古い")], genres: T.genres, sites: T.sites))
        await viewModel.loadCached()
        await viewModel.refresh()
        XCTAssertTrue(viewModel.isStale)

        service.offline = false
        await viewModel.refresh()
        XCTAssertEqual(viewModel.items, T.items)
        XCTAssertFalse(viewModel.isStale)
        XCTAssertNil(viewModel.loadError)
        let saved = await cache.loadItems()
        XCTAssertEqual(saved, T.items)
    }

    // MARK: - 設定が無いとき

    func testNeedsSettingsWhenTokenOrURLIsMissing() {
        XCTAssertTrue(makeViewModel(settings: WishlistSettings(apiBaseURL: T.baseURLString, token: "")).0.needsSettings)
        XCTAssertTrue(makeViewModel(settings: WishlistSettings(apiBaseURL: nil, token: "t")).0.needsSettings)
        XCTAssertFalse(makeViewModel().0.needsSettings)
    }

    /// AC-SET-01: 設定が無いときは API を呼ばない(キャッシュは出せる)。
    func testRefreshDoesNotCallTheServiceWhenNotConfigured() async {
        let (viewModel, service, _) = makeViewModel(
            cache: InMemoryWishlistCache(items: T.items, genres: T.genres, sites: T.sites),
            settings: WishlistSettings(apiBaseURL: nil, token: ""))
        await viewModel.loadCached()
        await viewModel.refresh()
        XCTAssertEqual(service.calls, [])
        XCTAssertEqual(viewModel.items, T.items)
    }

    // MARK: - 絞り込みと並び

    func testVisibleItemsFilterByGenre() async {
        let (viewModel, _, _) = makeViewModel()
        await viewModel.refresh()
        XCTAssertNil(viewModel.selectedGenreID)
        XCTAssertEqual(Set(viewModel.visibleItems.map(\.id)), [11, 12], "すべて")
        viewModel.selectedGenreID = 2
        XCTAssertEqual(viewModel.visibleItems.map(\.id), [11])
        viewModel.selectedGenreID = 1
        XCTAssertEqual(viewModel.visibleItems.map(\.id), [12])
        viewModel.selectedGenreID = nil
        XCTAssertEqual(viewModel.visibleItems.count, 2)
    }

    /// sortOrder 昇順、同順は id 降順(API の並びと同じ。クライアントでも保つ)。
    func testVisibleItemsAreSortedBySortOrderThenIDDescending() async {
        let items = [T.item(1, sort: 5), T.item(2, sort: 1), T.item(3, sort: 1)]
        let (viewModel, _, _) = makeViewModel(service: T.fake(), cache: InMemoryWishlistCache(items: items, genres: T.genres, sites: T.sites))
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.visibleItems.map(\.id), [3, 2, 1])
    }

    func testChipGenresAreSortedBySortOrderThenIDAscending() async {
        let genres = [
            Genre(id: 3, name: "C", sortOrder: 2), Genre(id: 1, name: "A", sortOrder: 2), Genre(id: 2, name: "B", sortOrder: 1),
        ]
        let (viewModel, _, _) = makeViewModel(cache: InMemoryWishlistCache(items: [], genres: genres, sites: []))
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.chipGenres.map(\.id), [2, 1, 3])
    }

    /// 0 件でも落ちない。
    func testEmptyListsAreFine() async {
        let (viewModel, _, _) = makeViewModel(service: FakeWishlistService())
        await viewModel.refresh()
        XCTAssertEqual(viewModel.visibleItems, [])
        XCTAssertEqual(viewModel.chipGenres, [])
        XCTAssertFalse(viewModel.isStale)
    }

    // MARK: - 反映

    func testUpsertReplacesExistingAndAddsNewAndSavesCache() async {
        let (viewModel, _, cache) = makeViewModel()
        await viewModel.refresh()
        var renamed = T.gris
        renamed.name = "グリス 改"
        await viewModel.upsert(renamed)
        XCTAssertEqual(viewModel.items.first { $0.id == 12 }?.name, "グリス 改")
        XCTAssertEqual(viewModel.items.count, 2)

        await viewModel.upsert(T.item(20, genre: 1, name: "新商品"))
        XCTAssertEqual(viewModel.items.count, 3)
        let saved = await cache.loadItems()
        XCTAssertEqual(Set(saved?.map(\.id) ?? []), [11, 12, 20])
        XCTAssertEqual(saved?.first { $0.id == 12 }?.name, "グリス 改")
    }

    func testSetGenresAndSitesUpdateAndSaveCache() async {
        let (viewModel, _, cache) = makeViewModel()
        await viewModel.refresh()
        let newGenres = [T.figuarts]
        await viewModel.setGenres(newGenres)
        await viewModel.setSites([T.mercari])
        XCTAssertEqual(viewModel.genres, newGenres)
        XCTAssertEqual(viewModel.sites, [T.mercari])
        let savedGenres = await cache.loadGenres()
        let savedSites = await cache.loadSites()
        XCTAssertEqual(savedGenres, newGenres)
        XCTAssertEqual(savedSites, [T.mercari])
    }

    /// 選んでいたジャンルが無くなったら「すべて」に戻す(空のグリッドに取り残さない)。
    func testSelectedGenreResetsWhenItNoLongerExists() async {
        let (viewModel, _, _) = makeViewModel()
        await viewModel.refresh()
        viewModel.selectedGenreID = 2
        await viewModel.setGenres([T.figuarts])
        XCTAssertNil(viewModel.selectedGenreID)
    }

    // MARK: - 削除

    func testDeleteRemovesFromListAndCache() async {
        let (viewModel, service, cache) = makeViewModel()
        await viewModel.refresh()
        let deleted = await viewModel.delete(T.gris)
        XCTAssertTrue(deleted)
        XCTAssertEqual(viewModel.items.map(\.id), [11])
        XCTAssertTrue(service.calls.contains(.deleteItem(id: 12)))
        let saved = await cache.loadItems()
        XCTAssertEqual(saved?.map(\.id), [11])
        XCTAssertNil(viewModel.errorMessage)
    }

    /// 失敗したらグリッドは変えず、理由を出す。
    func testDeleteFailureKeepsTheListAndSetsErrorMessage() async {
        let (viewModel, service, cache) = makeViewModel()
        await viewModel.refresh()
        service.offline = true
        let deleted = await viewModel.delete(T.gris)
        XCTAssertFalse(deleted)
        XCTAssertEqual(Set(viewModel.items.map(\.id)), [11, 12])
        XCTAssertNotNil(viewModel.errorMessage)
        let saved = await cache.loadItems()
        XCTAssertEqual(Set(saved?.map(\.id) ?? []), [11, 12])
    }
}
