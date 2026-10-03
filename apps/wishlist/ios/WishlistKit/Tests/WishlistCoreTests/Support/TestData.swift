import Foundation
import Synchronization

@testable import WishlistCore

// テスト共通のデータ・ヘルパ。WishlistFixtures(実装が埋める)には依存しない(ViewModel のテストは自前のデータで書く)。

enum T {
    static let baseURLString = "https://wishlist.example/wishlist/"
    static var configured: WishlistSettings { WishlistSettings(apiBaseURL: baseURLString, token: "test-token") }

    static func uuid(_ n: Int) -> String { "00000000-0000-4000-8000-" + String(format: "%012d", n) }

    static func item(
        _ id: Int, genre: Int = 1, name: String? = nil, option: String? = nil, override: String? = nil,
        sort: Int = 0, siteOverrides: [SiteOverride] = [], minPrice: Int? = nil, sourceURL: String? = nil
    ) -> Item {
        Item(
            id: id, genreID: genre, name: name ?? "商品\(id)", optionText: option, queryOverride: override,
            imageURLPath: "images/\(uuid(id)).png", sourceURL: sourceURL, minPrice: minPrice, sortOrder: sort,
            siteOverrides: siteOverrides)
    }

    static let mercari = Site(
        id: 1, name: "メルカリ", searchURLTemplate: "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc",
        fetchType: .headless)
    static let amazon = Site(
        id: 2, name: "Amazon", searchURLTemplate: "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank")
    static let other = Site(id: 3, name: "その他", searchURLTemplate: "https://site3.example.com/s?q={q}")
    static var sites: [Site] { [mercari, amazon, other] }

    /// S.H.Figuarts(id 1。サイトは Amazon → メルカリ → その他の順)と デュエマ(id 2)
    static let figuarts = Genre(id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", sortOrder: 1, siteIDs: [2, 1, 3])
    static let duema = Genre(id: 2, name: "デュエマ", queryTemplate: "{name} {option}", sortOrder: 2, siteIDs: [1, 2])
    static var genres: [Genre] { [figuarts, duema] }

    /// グリス(id 12・ジャンル 1)とボルシャック(id 11・ジャンル 2・option あり)
    static let gris = item(12, genre: 1, name: "グリス")
    static let borushack = item(11, genre: 2, name: "ボルシャック", option: "銀トレジャー")
    static var items: [Item] { [gris, borushack] }

    static func fake(offline: Bool = false) -> FakeWishlistService {
        let service = FakeWishlistService(items: items, genres: genres, sites: sites)
        service.offline = offline
        return service
    }

    static func unauthorized() -> WishlistError {
        WishlistError(code: .unauthorized, status: 401, message: "unauthorized")
    }
}

/// テスト用の設定ストア(メモリ)。
final class InMemorySettingsStore: WishlistSettingsStore {
    private let state: Mutex<(settings: WishlistSettings, saves: Int, canSave: Bool)>

    init(_ settings: WishlistSettings = WishlistSettings(), canSave: Bool = true) {
        state = Mutex((settings, 0, canSave))
    }

    var saves: Int { state.withLock { $0.saves } }
    var stored: WishlistSettings { state.withLock { $0.settings } }

    func load() -> WishlistSettings { state.withLock { $0.settings } }
    func save(_ settings: WishlistSettings) -> Bool {
        state.withLock { s in
            guard s.canSave else { return false }
            s.settings = settings
            s.saves += 1
            return true
        }
    }
}

func makeTempDirectory(_ name: String = #function) throws -> URL {
    let url = FileManager.default.temporaryDirectory
        .appendingPathComponent("wishlist-tests-\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
    return url
}

/// JSON のボディをオブジェクトにする(キーの順序に依存せず比べるため)。
func jsonObject(_ data: Data, file: StaticString = #filePath, line: UInt = #line) -> NSDictionary {
    guard let object = try? JSONSerialization.jsonObject(with: data) as? NSDictionary else {
        preconditionFailure("JSON のオブジェクトではない: \(String(decoding: data, as: UTF8.self))", file: file, line: line)
    }
    return object
}
