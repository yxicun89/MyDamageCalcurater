import Foundation
import Observation

/// API の接続先とトークン(端末ごと。本体と Share Extension は別々に持つ。仕様 §9.5)。
/// 保存先は UserDefaults(Keychain は将来)。
public struct WishlistSettings: Equatable, Sendable {
    /// API のベース URL の文字列(例 `https://<ホスト>/wishlist/`)。未設定は nil。
    public var apiBaseURL: String?
    public var token: String

    public init(apiBaseURL: String? = nil, token: String = "") {
        self.apiBaseURL = apiBaseURL
        self.token = token
    }

    /// http(s) で解釈できるときだけ URL。それ以外は nil。
    public var baseURL: URL? {
        guard let text = apiBaseURL, Deeplink.isHTTPURL(text) else { return nil }
        return URL(string: text)
    }

    /// URL が使えて、トークンが(空白だけでなく)入っているとき true。false なら API を呼ばず設定画面から始める。
    public var isConfigured: Bool {
        baseURL != nil && !token.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}

/// 設定の保存。
public protocol WishlistSettingsStore: Sendable {
    /// 保存が無い・壊れている・http(s) 以外の URL は `WishlistSettings()` 相当(URL だけ無効な場合はトークンは残す)。
    func load() -> WishlistSettings
    /// 保存できたら true。例外は投げない。
    func save(_ settings: WishlistSettings) -> Bool
}

/// UserDefaults の `wishlist.settings` に JSON `{"apiBaseUrl": string|null, "token": string}` で保存する(PWA の localStorage と同じ形)。
public struct UserDefaultsSettingsStore: WishlistSettingsStore {
    public static let key = "wishlist.settings"

    // UserDefaults はスレッドセーフ(SDK に Sendable の注釈が無いため unsafe)
    private nonisolated(unsafe) let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    private struct Stored: Codable {
        var apiBaseUrl: String?
        var token: String

        func encode(to encoder: any Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            try c.encode(apiBaseUrl, forKey: .apiBaseUrl)  // nil は JSON の null(PWA と同じ形)
            try c.encode(token, forKey: .token)
        }
    }

    public func load() -> WishlistSettings {
        guard let data = defaults.data(forKey: Self.key), let stored = try? JSONDecoder().decode(Stored.self, from: data) else {
            return WishlistSettings()
        }
        let url = stored.apiBaseUrl.flatMap { Deeplink.isHTTPURL($0) ? $0 : nil }
        return WishlistSettings(apiBaseURL: url, token: stored.token)
    }

    public func save(_ settings: WishlistSettings) -> Bool {
        guard let data = try? JSONEncoder().encode(Stored(apiBaseUrl: settings.apiBaseURL, token: settings.token)) else { return false }
        defaults.set(data, forKey: Self.key)
        return defaults.data(forKey: Self.key) == data
    }
}

/// 設定画面(本体・Share Extension 共通)の入力と検証。
@MainActor
@Observable
public final class ConnectionSettingsViewModel {
    public var apiBaseURLText: String
    public var token: String
    public private(set) var errorMessage: String?

    private let store: any WishlistSettingsStore

    public init(store: any WishlistSettingsStore) {
        self.store = store
        let current = store.load()
        apiBaseURLText = current.apiBaseURL ?? ""
        token = current.token
    }

    /// 前後の空白を除いて保存する。URL が空でなく http(s) でないときは保存せず `errorMessage` を立てて false。
    /// URL が空なら `apiBaseURL = nil` で保存する(未設定)。成功したら `errorMessage = nil` で true。
    public func save() -> Bool {
        let url = apiBaseURLText.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedToken = token.trimmingCharacters(in: .whitespacesAndNewlines)
        if !url.isEmpty && !Deeplink.isHTTPURL(url) {
            errorMessage = "URL は http:// か https:// で始めてください"
            return false
        }
        let settings = WishlistSettings(apiBaseURL: url.isEmpty ? nil : url, token: trimmedToken)
        guard store.save(settings) else {
            errorMessage = "設定を保存できませんでした"
            return false
        }
        errorMessage = nil
        return true
    }
}
