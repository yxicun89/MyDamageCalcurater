import Foundation

public enum Deeplink {
    /// `{q}` をすべて `encodeURIComponent(query)` で置き換える(testdata/query-cases.json の deeplink)。
    /// エスケープしないのは `A-Za-z0-9 - _ . ! ~ * ' ( )` だけ。空白は `%20`。置換後の値の中の `{q}` は再置換しない。
    public static func build(template: String, query: String) -> String {
        let encoded = query.addingPercentEncoding(withAllowedCharacters: unreserved) ?? query
        return template.replacingOccurrences(of: "{q}", with: encoded)
    }

    /// http(s) の URL で `{q}` を含むときだけ true(サイト設定の入力検証)。
    public static func isValidSearchTemplate(_ template: String) -> Bool {
        template.contains("{q}") && isHTTPURL(template)
    }

    /// http(s) の絶対 URL なら true。`UIApplication.shared.open` に渡す前の最後の確認(javascript: などを出さない)。
    public static func isHTTPURL(_ string: String) -> Bool {
        guard let url = URL(string: string), let scheme = url.scheme?.lowercased(), scheme == "http" || scheme == "https" else {
            return false
        }
        return url.host?.isEmpty == false
    }

    /// encodeURIComponent がエスケープしない文字
    private static let unreserved = CharacterSet(
        charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()")
}
