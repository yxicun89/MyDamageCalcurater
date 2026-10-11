import Foundation

/// API の接続先(端末ごと。本体と Share Extension は別々に持つ)。
/// 接続先はビルド設定 `WISHLIST_API_BASE_URL`(git に入れない xcconfig)を Info.plist 経由で読む。
/// 認証の情報は持たない(docs/phase5-spec.md 5-3)。
public struct WishlistSettings: Equatable, Sendable {
    /// API のベース URL の文字列(例 `https://<ホスト>/wishlist/`)。未設定は nil。
    public var apiBaseURL: String?

    public init(apiBaseURL: String? = nil) {
        self.apiBaseURL = apiBaseURL
    }

    /// Info.plist のキー(`WISHLIST_API_BASE_URL` ビルド設定を通す)。
    public static let infoKey = "WishlistAPIBaseURL"

    /// Info.plist(`Bundle.main.infoDictionary`)から接続先を読む。
    /// 値が無い・文字列でない・空白だけ・展開されていない `$(...)`・http(s) でない → `apiBaseURL = nil`(保存済みのキャッシュだけで動く)。
    public static func fromInfo(_ info: [String: Any]?) -> WishlistSettings {
        guard let raw = info?[infoKey] as? String else { return WishlistSettings() }
        let text = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, !text.hasPrefix("$("), Deeplink.isHTTPURL(text) else { return WishlistSettings() }
        return WishlistSettings(apiBaseURL: text)
    }

    /// http(s) で解釈できるときだけ URL。それ以外は nil。
    public var baseURL: URL? {
        guard let text = apiBaseURL, Deeplink.isHTTPURL(text) else { return nil }
        return URL(string: text)
    }

    /// 使える URL があるとき true。false なら API を呼ばず、保存済みのキャッシュだけで動く。
    public var isConfigured: Bool {
        baseURL != nil
    }
}
