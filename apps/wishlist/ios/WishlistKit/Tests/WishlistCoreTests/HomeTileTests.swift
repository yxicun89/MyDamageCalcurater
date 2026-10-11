import XCTest

@testable import WishlistCore

// AC-IOS-5-3c: ホームのタイル(ジャンルの色・絵柄・価格の札)とオフラインの札。docs/phase5-spec.md 5-2「ホーム」・5-3。
@MainActor
final class HomeTileTests: XCTestCase {
    private let figuarts = Genre(id: 1, name: "S.H.Figuarts", sortOrder: 1, color: "#e0393e", icon: .figure)
    private let duema = Genre(id: 2, name: "デュエマ", sortOrder: 2, color: "#4f46e5", icon: .card)

    private func item(_ id: Int, genre: Int = 1, summary: ItemSummary?) -> Item {
        var item = T.item(id, genre: genre)
        item.summary = summary
        return item
    }

    private func tile(_ summary: ItemSummary?, isOffline: Bool = false, genre: Int = 1) -> HomeTile {
        HomeTile.make(item: item(5, genre: genre, summary: summary), genres: [figuarts, duema], isOffline: isOffline)
    }

    /// AC-IOS-5-3c-1: タイルはジャンルの色と絵柄を持つ(枠の色=ジャンル色)
    func testTileExposesGenreColorAndIcon() {
        XCTAssertEqual(tile(nil).genreColor, "#e0393e")
        XCTAssertEqual(tile(nil).genreIcon, .figure)
        XCTAssertEqual(tile(nil, genre: 2).genreColor, "#4f46e5")
        XCTAssertEqual(tile(nil, genre: 2).genreIcon, .card)
    }

    /// AC-IOS-5-3c-2: ジャンルが一覧に無いときは強調色と箱
    func testUnknownGenreFallsBackToAccentAndBox() {
        let unknown = tile(nil, genre: 99)
        XCTAssertEqual(unknown.genreColor, "#1d5fd1")
        XCTAssertEqual(unknown.genreIcon, .box)
    }

    /// AC-IOS-5-3c-3: 価格の札。ok → `¥A〜`(桁区切り)/ no_result → 出品なし / failed・none・summary なし → 札なし
    func testPriceTagFollowsTheSummaryStatus() {
        XCTAssertEqual(tile(ItemSummary(low: 880, status: .ok)).priceTag, .price("¥880〜"))
        XCTAssertEqual(tile(ItemSummary(low: 12_800, status: .ok)).priceTag, .price("¥12,800〜"))
        XCTAssertEqual(tile(ItemSummary(low: nil, status: .noResult)).priceTag, .noListing)
        XCTAssertNil(tile(ItemSummary(low: nil, status: .none)).priceTag)
        XCTAssertNil(tile(ItemSummary(low: nil, status: .failed)).priceTag)
        XCTAssertNil(tile(ItemSummary(low: nil, status: .ok)).priceTag, "ok でも low が無ければ札を出さない")
        XCTAssertNil(tile(nil).priceTag)
    }

    /// AC-IOS-5-3c-4: オフライン中は価格の札を消す(出品なしの札も)
    func testOfflineHidesPriceTags() {
        XCTAssertNil(tile(ItemSummary(low: 880, status: .ok), isOffline: true).priceTag)
        XCTAssertNil(tile(ItemSummary(low: nil, status: .noResult), isOffline: true).priceTag)
    }

    private func makeViewModel(settings: WishlistSettings = T.configured, service: FakeWishlistService? = nil)
        -> (HomeViewModel, FakeWishlistService)
    {
        let service = service ?? FakeWishlistService(
            items: [item(12, genre: 1, summary: ItemSummary(low: 880, status: .ok)), item(11, genre: 2, summary: ItemSummary(status: .noResult))],
            genres: [figuarts, duema], sites: T.sites)
        let cache = InMemoryWishlistCache(items: service.items, genres: service.genres, sites: service.sites)
        return (HomeViewModel(service: service, cache: cache, settings: settings), service)
    }

    /// AC-IOS-5-3c-5: ViewModel の tiles は visibleItems の並びで、ジャンルの絞り込みに従う
    func testTilesFollowVisibleItemsAndTheGenreFilter() async {
        let (viewModel, _) = makeViewModel()
        await viewModel.refresh()
        XCTAssertEqual(viewModel.tiles.map(\.item.id), viewModel.visibleItems.map(\.id))
        XCTAssertEqual(viewModel.tiles.count, 2)
        viewModel.selectedGenreID = 2
        XCTAssertEqual(viewModel.tiles.map(\.item.id), [11])
        XCTAssertEqual(viewModel.tiles.first?.priceTag, .noListing)
        XCTAssertEqual(viewModel.tiles.first?.genreColor, "#4f46e5")
    }

    /// AC-IOS-5-3c-6: 接続先が無くても、保存済み(キャッシュ)でタイルが出て、札は「オフライン」・価格の札は消え、API は呼ばない
    func testLocalOnlyStillShowsCachedTilesWithoutPriceTagsAndWithTheOfflineBadge() async {
        let (viewModel, service) = makeViewModel(settings: T.localOnly)
        await viewModel.loadCached()
        await viewModel.refresh()
        XCTAssertEqual(viewModel.tiles.count, 2)
        XCTAssertTrue(viewModel.isLocalOnly)
        XCTAssertTrue(viewModel.showsOfflineBadge)
        XCTAssertTrue(viewModel.tiles.allSatisfy { $0.priceTag == nil })
        XCTAssertEqual(service.calls, [])
    }

    /// AC-IOS-5-3c-7: 取得に失敗して保存済みを使ったときも「オフライン」の札を出し、価格の札を消す。つながれば札も価格も戻る
    func testStaleShowsTheOfflineBadgeAndRecovers() async {
        let (viewModel, service) = makeViewModel()
        await viewModel.loadCached()
        service.offline = true
        await viewModel.refresh()
        XCTAssertTrue(viewModel.isStale)
        XCTAssertTrue(viewModel.showsOfflineBadge)
        XCTAssertEqual(viewModel.tiles.count, 2, "保存済みで一覧は出る")
        XCTAssertTrue(viewModel.tiles.allSatisfy { $0.priceTag == nil })

        service.offline = false
        await viewModel.refresh()
        XCTAssertFalse(viewModel.showsOfflineBadge)
        XCTAssertEqual(viewModel.tiles.first { $0.item.id == 12 }?.priceTag, .price("¥880〜"))
    }

    /// AC-IOS-5-3c-8: つながっていて接続先があれば札は出さない
    func testNoBadgeWhenOnline() async {
        let (viewModel, _) = makeViewModel()
        await viewModel.refresh()
        XCTAssertFalse(viewModel.isLocalOnly)
        XCTAssertFalse(viewModel.showsOfflineBadge)
    }
}
