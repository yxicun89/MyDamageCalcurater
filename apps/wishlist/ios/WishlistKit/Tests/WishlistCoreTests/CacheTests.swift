import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-CACHE-01〜05: オフラインキャッシュ。「Mac が落ちていても一覧とディープリンクが使える」。

/// SwiftData のキャッシュ(インメモリのコンテナ。macOS の `swift test` で動くことを確認済み)。
final class SwiftDataWishlistCacheTests: XCTestCase {
    private func makeCache() throws -> SwiftDataWishlistCache { try SwiftDataWishlistCache(inMemory: true) }

    private var sampleItems: [Item] {
        [
            T.item(12, genre: 1, name: "グリス", siteOverrides: [SiteOverride(siteID: 1, query: "メルカリ用", enabled: true), SiteOverride(siteID: 2, enabled: false)]),
            T.item(11, genre: 2, name: "ボルシャック", option: "銀トレジャー", override: "ボルシャック 銀", sort: 3, minPrice: 1500, sourceURL: "https://shop.example/p/1"),
        ]
    }

    func testEmptyCacheReturnsNilForEveryKind() async throws {
        let cache = try makeCache()
        let items = await cache.loadItems()
        let genres = await cache.loadGenres()
        let sites = await cache.loadSites()
        XCTAssertNil(items)
        XCTAssertNil(genres)
        XCTAssertNil(sites)
    }

    func testItemsRoundTripKeepsEveryFieldAndOrder() async throws {
        let cache = try makeCache()
        await cache.saveItems(sampleItems)
        let loaded = await cache.loadItems()
        XCTAssertEqual(loaded, sampleItems)
    }

    func testGenresRoundTripKeepsSiteIDOrder() async throws {
        let cache = try makeCache()
        await cache.saveGenres(T.genres)
        let loaded = await cache.loadGenres()
        XCTAssertEqual(loaded, T.genres)
        XCTAssertEqual(loaded?.first?.siteIDs, [2, 1, 3])
    }

    func testSitesRoundTrip() async throws {
        let cache = try makeCache()
        await cache.saveSites(T.sites)
        let loaded = await cache.loadSites()
        XCTAssertEqual(loaded, T.sites)
    }

    func testSaveReplacesTheWholeKind() async throws {
        let cache = try makeCache()
        await cache.saveItems(sampleItems)
        await cache.saveItems([sampleItems[1]])
        let loaded = await cache.loadItems()
        XCTAssertEqual(loaded?.map(\.id), [11])
    }

    /// 空配列を保存したときは、保存が無い(nil)とは区別する(「0 件」をオフラインでも出せるように)。
    func testSavingEmptyArrayIsNotNil() async throws {
        let cache = try makeCache()
        await cache.saveItems([])
        let loaded = await cache.loadItems()
        XCTAssertEqual(loaded, [])
    }

    func testKindsAreIndependent() async throws {
        let cache = try makeCache()
        await cache.saveItems(sampleItems)
        let genres = await cache.loadGenres()
        XCTAssertNil(genres)
        await cache.saveGenres(T.genres)
        await cache.saveSites(T.sites)
        await cache.saveItems([])
        let loadedGenres = await cache.loadGenres()
        XCTAssertEqual(loadedGenres, T.genres)
    }

    func testFileStoreSurvivesANewInstance() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let storeURL = directory.appendingPathComponent("wishlist.store")
        do {
            let cache = try SwiftDataWishlistCache(storeURL: storeURL)
            await cache.saveItems(sampleItems)
            await cache.saveGenres(T.genres)
            await cache.saveSites(T.sites)
        }
        let reopened = try SwiftDataWishlistCache(storeURL: storeURL)
        let items = await reopened.loadItems()
        let genres = await reopened.loadGenres()
        let sites = await reopened.loadSites()
        XCTAssertEqual(items, sampleItems)
        XCTAssertEqual(genres, T.genres)
        XCTAssertEqual(sites, T.sites)
    }
}

/// 取得に成功したら保存して stale=false、失敗したら保存済みを stale=true で返す(web の fetchWithCache と同じ)。
final class WishlistRepositoryTests: XCTestCase {
    func testSuccessReturnsFreshDataAndSavesIt() async throws {
        let service = T.fake()
        let cache = InMemoryWishlistCache()
        let repository = WishlistRepository(service: service, cache: cache)

        let items = try await repository.items()
        XCTAssertEqual(items.data, T.items)
        XCTAssertFalse(items.stale)
        let genres = try await repository.genres()
        XCTAssertEqual(genres.data, T.genres)
        XCTAssertFalse(genres.stale)
        let sites = try await repository.sites()
        XCTAssertEqual(sites.data, T.sites)
        XCTAssertFalse(sites.stale)

        let savedItems = await cache.loadItems()
        let savedGenres = await cache.loadGenres()
        let savedSites = await cache.loadSites()
        XCTAssertEqual(savedItems, T.items)
        XCTAssertEqual(savedGenres, T.genres)
        XCTAssertEqual(savedSites, T.sites)
    }

    func testNetworkFailureReturnsSavedDataAsStale() async throws {
        let cache = InMemoryWishlistCache(items: [T.gris], genres: [T.figuarts], sites: [T.mercari])
        let repository = WishlistRepository(service: T.fake(offline: true), cache: cache)
        let items = try await repository.items()
        XCTAssertEqual(items.data, [T.gris])
        XCTAssertTrue(items.stale)
        let genres = try await repository.genres()
        XCTAssertEqual(genres.data, [T.figuarts])
        XCTAssertTrue(genres.stale)
        let sites = try await repository.sites()
        XCTAssertEqual(sites.data, [T.mercari])
        XCTAssertTrue(sites.stale)
    }

    /// 401(トークン違い)でも保存済みを使う(白画面にしない)。
    func testUnauthorizedAlsoFallsBackToSavedData() async throws {
        let service = T.fake()
        service.failure = T.unauthorized()
        let repository = WishlistRepository(service: service, cache: InMemoryWishlistCache(items: [T.gris]))
        let items = try await repository.items()
        XCTAssertEqual(items.data, [T.gris])
        XCTAssertTrue(items.stale)
    }

    func testFailureWithoutSavedDataRethrowsTheOriginalError() async {
        let service = T.fake()
        service.failure = T.unauthorized()
        let repository = WishlistRepository(service: service, cache: InMemoryWishlistCache())
        do {
            _ = try await repository.items()
            XCTFail("保存済みが無いので投げるはず")
        } catch {
            XCTAssertEqual(error as? WishlistError, T.unauthorized())
        }
    }

    func testFailureDoesNotOverwriteTheCache() async throws {
        let cache = InMemoryWishlistCache(items: [T.gris])
        let service = T.fake(offline: true)
        let repository = WishlistRepository(service: service, cache: cache)
        _ = try await repository.items()
        let saved = await cache.loadItems()
        XCTAssertEqual(saved, [T.gris])
        XCTAssertEqual(cache.saveCount, 0)
    }

    /// 空配列を保存済みなら、失敗しても空配列(stale)を返す(nil とは違う)。
    func testSavedEmptyArrayIsReturnedAsStaleNotRethrown() async throws {
        let repository = WishlistRepository(service: T.fake(offline: true), cache: InMemoryWishlistCache(items: []))
        let items = try await repository.items()
        XCTAssertEqual(items.data, [])
        XCTAssertTrue(items.stale)
    }

    func testBackOnlineReplacesTheSavedData() async throws {
        let service = T.fake()
        let cache = InMemoryWishlistCache(items: [T.item(1, name: "古い")])
        let repository = WishlistRepository(service: service, cache: cache)
        let items = try await repository.items()
        XCTAssertEqual(items.data, T.items)
        XCTAssertFalse(items.stale)
        let saved = await cache.loadItems()
        XCTAssertEqual(saved, T.items)
    }
}

/// 画像のファイルキャッシュ。一度取れた画像はオフラインでも返す。
final class ImageFileCacheTests: XCTestCase {
    private let url = URL(string: "https://wishlist.example/wishlist/images/00000000-0000-4000-8000-000000000012.png")!
    private let other = URL(string: "https://wishlist.example/wishlist/images/00000000-0000-4000-8000-000000000011.png")!

    private final class Counter: @unchecked Sendable {
        private let lock = NSLock()
        private var value = 0
        func increment() { lock.withLock { value += 1 } }
        var count: Int { lock.withLock { value } }
    }

    func testSecondRequestIsServedFromFileWithoutLoading() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let counter = Counter()
        let cache = ImageFileCache(directory: directory) { _ in
            counter.increment()
            return Data("IMG12".utf8)
        }
        let first = try await cache.data(for: url)
        let second = try await cache.data(for: url)
        XCTAssertEqual(first, Data("IMG12".utf8))
        XCTAssertEqual(second, first)
        XCTAssertEqual(counter.count, 1)
    }

    func testCachedDataIsNilUntilLoaded() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = ImageFileCache(directory: directory) { _ in Data("IMG".utf8) }
        XCTAssertNil(cache.cachedData(for: url))
        _ = try await cache.data(for: url)
        XCTAssertEqual(cache.cachedData(for: url), Data("IMG".utf8))
    }

    /// プロセスをまたぐ: 同じディレクトリで作り直しても、通信せずに返す(Mac が落ちていても画像が出る)。
    func testNewInstanceOnSameDirectoryServesSavedImageOffline() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let online = ImageFileCache(directory: directory) { _ in Data("IMG12".utf8) }
        _ = try await online.data(for: url)

        let offline = ImageFileCache(directory: directory) { _ in throw URLError(.notConnectedToInternet) }
        let data = try await offline.data(for: url)
        XCTAssertEqual(data, Data("IMG12".utf8))
    }

    func testLoaderFailureWithoutSavedImageThrowsAndLeavesNoFile() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = ImageFileCache(directory: directory) { _ in throw URLError(.notConnectedToInternet) }
        do {
            _ = try await cache.data(for: url)
            XCTFail("保存済みが無いので投げるはず")
        } catch {
            XCTAssertNotNil(error as? URLError)
        }
        XCTAssertNil(cache.cachedData(for: url))
        let files = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
        XCTAssertEqual(files, [], "途中のファイルを残さない")
    }

    func testDifferentURLsAreStoredSeparately() async throws {
        let directory = try makeTempDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = ImageFileCache(directory: directory) { requested in Data(requested.lastPathComponent.utf8) }
        let a = try await cache.data(for: url)
        let b = try await cache.data(for: other)
        XCTAssertNotEqual(a, b)
        XCTAssertEqual(cache.cachedData(for: url), a)
        XCTAssertEqual(cache.cachedData(for: other), b)
    }
}
