import XCTest

@testable import WishlistCore

// AC-IOS-OFF-01〜03: 仕様 §9.5 / §12 フェーズ2の完了条件「機内モードでも一覧が表示され、検索ボタン(リンク)が動く」。
// SwiftData のキャッシュ + 通信できない Service で、ホーム → 詳細シートを通しで確かめる。
@MainActor
final class OfflineScenarioTests: XCTestCase {
    private func seededCache() async throws -> SwiftDataWishlistCache {
        let cache = try SwiftDataWishlistCache(inMemory: true)
        await cache.saveItems(T.items)
        await cache.saveGenres(T.genres)
        await cache.saveSites(T.sites)
        return cache
    }

    func testOfflineLaunchShowsTheSavedListAndWorkingLinks() async throws {
        let service = T.fake(offline: true)
        let home = HomeViewModel(service: service, cache: try await seededCache(), settings: T.configured)
        await home.loadCached()
        await home.refresh()

        XCTAssertEqual(home.visibleItems.map(\.id), [12, 11], "同じ sortOrder なら id 降順")
        XCTAssertTrue(home.isStale)
        XCTAssertNil(home.loadError)
        XCTAssertEqual(home.chipGenres.map(\.name), ["S.H.Figuarts", "デュエマ"])

        let gris = try XCTUnwrap(home.visibleItems.first { $0.id == 12 })
        let detail = ItemDetailViewModel(
            item: gris, genre: home.genres.first { $0.id == gris.genreID }, sites: home.sites, service: service)
        XCTAssertEqual(detail.links.map(\.site.name), ["Amazon", "メルカリ", "その他"])
        XCTAssertTrue(detail.links.allSatisfy { Deeplink.isHTTPURL($0.url.absoluteString) })
        await detail.loadEstimates()
        XCTAssertEqual(detail.summaryText, "オフライン")
        XCTAssertEqual(detail.links.count, 3, "オフラインでもリンクは残る")
    }

    func testSavedImageIsServedWhenTheNetworkIsDown() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let base = try XCTUnwrap(URL(string: T.baseURLString))
        let imageURL = try XCTUnwrap(WishlistURLs.imageURL(path: T.gris.imageURLPath, baseURL: base))

        let online = ImageFileCache(directory: directory) { _ in Data("GRIS".utf8) }
        _ = try await online.data(for: imageURL)
        let offline = ImageFileCache(directory: directory) { _ in throw URLError(.notConnectedToInternet) }
        let data = try await offline.data(for: imageURL)
        XCTAssertEqual(data, Data("GRIS".utf8))
    }

    /// 取得に成功したらキャッシュが最新に置き換わる(次の起動で、機内モードでも新しい一覧が出る)。
    func testOnlineRefreshUpdatesTheSavedListForTheNextOfflineLaunch() async throws {
        let cache = try await seededCache()
        let service = FakeWishlistService(items: [T.gris, T.item(30, genre: 1, name: "新着")], genres: T.genres, sites: T.sites)
        let first = HomeViewModel(service: service, cache: cache, settings: T.configured)
        await first.refresh()

        let second = HomeViewModel(service: T.fake(offline: true), cache: cache, settings: T.configured)
        await second.loadCached()
        XCTAssertEqual(Set(second.items.map(\.id)), [12, 30])
    }
}
