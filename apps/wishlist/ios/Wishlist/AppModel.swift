import Foundation
import Observation
import WishlistCore

/// アプリ全体の依存(サービス・キャッシュ・設定・画像キャッシュ・ホームの状態)を束ねる。View はこれを Environment から読むだけ。
///
/// 起動の取り決め(docs/phase2-ios-spec.md の XCUITest):
/// - `WISHLIST_USE_FAKE=1`: 通信しない。WishlistFixtures の FakeWishlistService + InMemoryWishlistCache、設定済みの接続設定。
/// - `WISHLIST_USE_FAKE=offline`: SwiftData のキャッシュに WishlistFixtures があり、Service は通信できない。
/// - 環境変数なし: 本物の設定(UserDefaults)と API。未設定なら設定画面から始まる。
@MainActor
@Observable
final class AppModel {
    private(set) var home: HomeViewModel
    private(set) var service: any WishlistService
    private(set) var settings: WishlistSettings
    private(set) var imageCache: ImageFileCache
    let settingsStore: any WishlistSettingsStore

    private let cache: any WishlistCache
    private let usesFixedBackend: Bool
    private let seed: (@Sendable () async -> Void)?
    private var isPrepared = false

    private static let fakeSettings = WishlistSettings(apiBaseURL: "https://fake.example/wishlist/", token: "fake")

    init(environment: [String: String] = ProcessInfo.processInfo.environment) {
        let mode = environment["WISHLIST_USE_FAKE"]
        let store: any WishlistSettingsStore
        let cache: any WishlistCache
        let service: any WishlistService
        let settings: WishlistSettings
        var seed: (@Sendable () async -> Void)?
        switch mode {
        case "1":
            store = UserDefaultsSettingsStore(defaults: UserDefaults(suiteName: "wishlist.fake") ?? .standard)
            cache = InMemoryWishlistCache(items: WishlistFixtures.items, genres: WishlistFixtures.genres, sites: WishlistFixtures.sites)
            service = FakeWishlistService(items: WishlistFixtures.items, genres: WishlistFixtures.genres, sites: WishlistFixtures.sites)
            settings = Self.fakeSettings
        case "offline":
            store = UserDefaultsSettingsStore(defaults: UserDefaults(suiteName: "wishlist.fake") ?? .standard)
            let persistent: any WishlistCache = (try? SwiftDataWishlistCache(inMemory: true)) ?? InMemoryWishlistCache()
            cache = persistent
            let fake = FakeWishlistService(items: WishlistFixtures.items, genres: WishlistFixtures.genres, sites: WishlistFixtures.sites)
            fake.offline = true
            service = fake
            settings = Self.fakeSettings
            seed = {
                await persistent.saveItems(WishlistFixtures.items)
                await persistent.saveGenres(WishlistFixtures.genres)
                await persistent.saveSites(WishlistFixtures.sites)
            }
        default:
            let live = UserDefaultsSettingsStore()
            store = live
            settings = live.load()
            cache = Self.makePersistentCache()
            service = Self.makeService(settings)
        }
        self.settingsStore = store
        self.cache = cache
        self.service = service
        self.settings = settings
        self.usesFixedBackend = mode != nil && (mode == "1" || mode == "offline")
        self.seed = seed
        self.imageCache = Self.makeImageCache(settings: settings, offline: usesFixedBackend)
        self.home = HomeViewModel(service: service, cache: cache, settings: settings)
    }

    /// 起動時の準備(モック起動のときだけキャッシュへフィクスチャを入れる)。何度呼んでもよい。
    func prepare() async {
        guard !isPrepared else { return }
        isPrepared = true
        await seed?()
    }

    /// 設定画面で接続設定を保存したあとに呼ぶ。API・画像キャッシュ・ホームの状態を作り直す。
    func reconnect() {
        guard !usesFixedBackend else { return }
        settings = settingsStore.load()
        service = Self.makeService(settings)
        imageCache = Self.makeImageCache(settings: settings, offline: false)
        home = HomeViewModel(service: service, cache: cache, settings: settings)
    }

    // MARK: - 組み立て

    private static func makeService(_ settings: WishlistSettings) -> any WishlistService {
        // 未設定のときは API を呼ばない(HomeViewModel・設定画面が止める)。ここは型を満たすための置き場。
        let url = settings.baseURL ?? URL(string: "http://localhost/")!
        return APIWishlistService(baseURL: url, token: settings.token)
    }

    private static func makePersistentCache() -> any WishlistCache {
        if let directory = try? FileManager.default.url(
            for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        {
            let url = directory.appendingPathComponent("wishlist-cache.store")
            if let cache = try? SwiftDataWishlistCache(storeURL: url) { return cache }
        }
        if let cache = try? SwiftDataWishlistCache(inMemory: true) { return cache }
        return InMemoryWishlistCache()
    }

    private static func makeImageCache(settings: WishlistSettings, offline: Bool) -> ImageFileCache {
        let directory = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first?
            .appendingPathComponent(offline ? "wishlist-images-fake" : "wishlist-images", isDirectory: true)
            ?? FileManager.default.temporaryDirectory.appendingPathComponent("wishlist-images", isDirectory: true)
        let token = settings.token
        return ImageFileCache(directory: directory) { url in
            if offline { throw URLError(.notConnectedToInternet) }
            var request = URLRequest(url: url)
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            let (data, response) = try await URLSession.shared.data(for: request)
            guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw URLError(.badServerResponse) }
            return data
        }
    }
}
