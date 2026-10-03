import CryptoKit
import Foundation
import SwiftData
import Synchronization

/// 一覧(items・genres・sites)のオフライン用の保存先。「Mac が落ちていても一覧とディープリンクが使える」ための層。
/// 実装は SwiftDataWishlistCache(本番)と InMemoryWishlistCache(テスト・プレビュー)。
public protocol WishlistCache: Sendable {
    /// 保存が無いときは nil(空配列を保存したときは空配列。区別する)
    func loadItems() async -> [Item]?
    func loadGenres() async -> [Genre]?
    func loadSites() async -> [Site]?
    /// 全件置き換え。保存できなくても例外は投げない(キャッシュの失敗で画面を止めない)。
    func saveItems(_ items: [Item]) async
    func saveGenres(_ genres: [Genre]) async
    func saveSites(_ sites: [Site]) async
}

/// メモリ上のキャッシュ(spec-writer が置いた完成品)。
public final class InMemoryWishlistCache: WishlistCache {
    private struct State {
        var items: [Item]?
        var genres: [Genre]?
        var sites: [Site]?
        var saveCount = 0
    }
    private let state: Mutex<State>

    public init(items: [Item]? = nil, genres: [Genre]? = nil, sites: [Site]? = nil) {
        state = Mutex(State(items: items, genres: genres, sites: sites))
    }

    public var saveCount: Int { state.withLock { $0.saveCount } }
    public func loadItems() async -> [Item]? { state.withLock { $0.items } }
    public func loadGenres() async -> [Genre]? { state.withLock { $0.genres } }
    public func loadSites() async -> [Site]? { state.withLock { $0.sites } }
    public func saveItems(_ items: [Item]) async { state.withLock { $0.items = items; $0.saveCount += 1 } }
    public func saveGenres(_ genres: [Genre]) async { state.withLock { $0.genres = genres; $0.saveCount += 1 } }
    public func saveSites(_ sites: [Site]) async { state.withLock { $0.sites = sites; $0.saveCount += 1 } }
}

/// 種類(items・genres・sites)ごとに 1 行、JSON にして保存する SwiftData のモデル。
@Model
final class CacheRecord {
    @Attribute(.unique) var kind: String
    var payload: Data

    init(kind: String, payload: Data) {
        self.kind = kind
        self.payload = payload
    }
}

/// SwiftData に保存するキャッシュ(本番)。
/// - `init(inMemory:)` はインメモリのコンテナ(テスト用)。`init(storeURL:)` は指定ファイルのストア(同じ URL で作り直しても残る)。
/// - 保存は種類ごとの全件置き換え。items を保存しても genres・sites には触れない。
/// - 保存が無いときは nil(空配列の保存とは区別する)。
public actor SwiftDataWishlistCache: WishlistCache {
    private let context: ModelContext

    public init(inMemory: Bool) throws {
        let configuration = ModelConfiguration(isStoredInMemoryOnly: inMemory)
        context = ModelContext(try ModelContainer(for: CacheRecord.self, configurations: configuration))
    }

    public init(storeURL: URL) throws {
        let configuration = ModelConfiguration(url: storeURL)
        context = ModelContext(try ModelContainer(for: CacheRecord.self, configurations: configuration))
    }

    public func loadItems() async -> [Item]? { load("items") }
    public func loadGenres() async -> [Genre]? { load("genres") }
    public func loadSites() async -> [Site]? { load("sites") }
    public func saveItems(_ items: [Item]) async { save(items, kind: "items") }
    public func saveGenres(_ genres: [Genre]) async { save(genres, kind: "genres") }
    public func saveSites(_ sites: [Site]) async { save(sites, kind: "sites") }

    private func record(_ kind: String) -> CacheRecord? {
        var descriptor = FetchDescriptor<CacheRecord>(predicate: #Predicate { $0.kind == kind })
        descriptor.fetchLimit = 1
        return try? context.fetch(descriptor).first
    }

    private func load<Value: Decodable>(_ kind: String) -> Value? {
        guard let record = record(kind) else { return nil }
        return try? JSONDecoder().decode(Value.self, from: record.payload)
    }

    /// 保存できなくても例外は投げない(キャッシュの失敗で画面を止めない)。
    private func save<Value: Encodable>(_ value: Value, kind: String) {
        guard let payload = try? JSONEncoder().encode(value) else { return }
        if let existing = record(kind) {
            existing.payload = payload
        } else {
            context.insert(CacheRecord(kind: kind, payload: payload))
        }
        try? context.save()
    }
}

public struct Cached<Value: Sendable>: Sendable {
    public var data: Value
    /// true なら取得に失敗して、保存済みの値を返した
    public var stale: Bool

    public init(data: Value, stale: Bool) {
        self.data = data
        self.stale = stale
    }
}

/// サービスとキャッシュをつなぐ。取得に成功したら保存して `stale == false`。
/// 失敗(WishlistError を含むどんな例外も)したら、保存済みを `stale == true` で返す。保存済みも無いときは元の例外を投げる。
/// 失敗でキャッシュを上書きしない。
public struct WishlistRepository: Sendable {
    private let service: any WishlistService
    private let cache: any WishlistCache

    public init(service: any WishlistService, cache: any WishlistCache) {
        self.service = service
        self.cache = cache
    }

    public func items() async throws -> Cached<[Item]> {
        try await fetch(service.listItems, load: cache.loadItems, save: cache.saveItems)
    }

    public func genres() async throws -> Cached<[Genre]> {
        try await fetch(service.listGenres, load: cache.loadGenres, save: cache.saveGenres)
    }

    public func sites() async throws -> Cached<[Site]> {
        try await fetch(service.listSites, load: cache.loadSites, save: cache.saveSites)
    }

    private func fetch<Value: Sendable>(
        _ remote: () async throws -> Value, load: () async -> Value?, save: (Value) async -> Void
    ) async throws -> Cached<Value> {
        do {
            let fresh = try await remote()
            await save(fresh)
            return Cached(data: fresh, stale: false)
        } catch {
            if let saved = await load() { return Cached(data: saved, stale: true) }
            throw error
        }
    }
}

/// 画像のファイルキャッシュ。一度取れた画像はファイルに置き、オフラインでも返す。
/// - `data(for:)`: ファイルがあればそれを返し、`loader` は呼ばない。無ければ `loader` で取って保存して返す。
/// - `loader` が失敗して保存済みも無ければ、その例外を投げる(途中のファイルを残さない)。
/// - 同じ `directory` で作り直しても保存済みの画像を返す(プロセスをまたぐ)。URL ごとに別ファイル。
public final class ImageFileCache: Sendable {
    private let directory: URL
    private let loader: @Sendable (URL) async throws -> Data

    public init(directory: URL, loader: @escaping @Sendable (URL) async throws -> Data) {
        self.directory = directory
        self.loader = loader
    }

    public func data(for url: URL) async throws -> Data {
        if let saved = cachedData(for: url) { return saved }
        let data = try await loader(url)
        let file = fileURL(for: url)
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try? data.write(to: file, options: .atomic)
        return data
    }

    /// 保存済みならそのデータ。無ければ nil(通信しない)。
    public func cachedData(for url: URL) -> Data? {
        try? Data(contentsOf: fileURL(for: url))
    }

    private func fileURL(for url: URL) -> URL {
        let digest = SHA256.hash(data: Data(url.absoluteString.utf8))
        return directory.appendingPathComponent(digest.map { String(format: "%02x", $0) }.joined())
    }
}
