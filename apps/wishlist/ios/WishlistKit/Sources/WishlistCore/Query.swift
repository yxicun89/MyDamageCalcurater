import Foundation

/// 検索ワードの入力(Go の query.Build・TS の buildQuery と同じ規則。testdata/query-cases.json の build)。
public struct QueryInput: Sendable, Equatable {
    public var template: String
    public var name: String
    public var option: String?
    public var queryOverride: String?
    public var siteQuery: String?

    public init(template: String, name: String, option: String? = nil, queryOverride: String? = nil, siteQuery: String? = nil) {
        self.template = template
        self.name = name
        self.option = option
        self.queryOverride = queryOverride
        self.siteQuery = siteQuery
    }
}

public enum SearchQuery {
    /// 連続する空白(全角・タブ・改行を含む)を半角 1 つにして前後を落とす。
    static func squeeze(_ s: String) -> String {
        s.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
    }

    /// 優先順位 siteQuery > queryOverride > テンプレート。空白(全角・タブ含む)のみの override は「無い」。
    /// 結果は連続空白を半角 1 つに詰めて前後を除く。置換は 1 回の走査(値の中の `{option}` を再置換しない)。
    public static func build(_ input: QueryInput) -> String {
        let site = squeeze(input.siteQuery ?? "")
        if !site.isEmpty { return site }
        let override = squeeze(input.queryOverride ?? "")
        if !override.isEmpty { return override }
        // 1 回の走査で置換する(値の中の {option} などを再置換しない)。
        var filled = ""
        var rest = Substring(input.template)
        while let open = rest.firstIndex(of: "{") {
            filled += rest[rest.startIndex..<open]
            let tail = rest[open...]
            if tail.hasPrefix("{name}") {
                filled += input.name
                rest = tail.dropFirst("{name}".count)
            } else if tail.hasPrefix("{option}") {
                filled += input.option ?? ""
                rest = tail.dropFirst("{option}".count)
            } else {
                filled += "{"
                rest = tail.dropFirst()
            }
        }
        filled += rest
        return squeeze(filled)
    }
}
