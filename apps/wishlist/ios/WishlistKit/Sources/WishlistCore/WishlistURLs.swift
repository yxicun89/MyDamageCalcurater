import Foundation

public enum WishlistURLs {
    /// 末尾に `/` を補う(`https://h/wishlist` → `https://h/wishlist/`)。
    public static func normalizeBaseURL(_ url: URL) -> URL {
        let s = url.absoluteString
        return s.hasSuffix("/") ? url : (URL(string: s + "/") ?? url)
    }

    /// `Item.imageURLPath`(`images/<name>`。先頭 `/` なし)をベース URL 基準で解決する。
    /// ベース URL の末尾 `/` の有無によらず、パスの前置(`/wishlist`)が残る。解決できなければ nil。
    public static func imageURL(path: String, baseURL: URL) -> URL? {
        if Deeplink.isHTTPURL(path) { return URL(string: path) }
        return URL(string: path, relativeTo: normalizeBaseURL(baseURL))?.absoluteURL
    }
}

/// UI の定数(spec で決めた値。View と XCUITest が同じ値を使う)。
public enum HomeLayout {
    /// ホームの画像グリッドの列数(仕様 §3: 2〜3列。iPhone は 3)
    public static let gridColumnCount = 3
    /// 長押しで編集・削除メニューを出すまでの秒数(PWA と同じ 0.5 秒)
    public static let longPressSeconds = 0.5
}

/// 画面に出す固定文言(PWA と同じ)。
public enum WishlistText {
    /// 詳細シートのサマリ(フェーズ2は価格を取得しないので常にこれ。通信できないときは `offline`)
    public static let noPriceInfo = "まだ価格情報はありません"
    public static let offline = "オフライン"
}
