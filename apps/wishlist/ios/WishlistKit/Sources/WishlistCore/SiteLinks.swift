import Foundation

/// 詳細シートのサイト行 1 つ分。
public struct SiteLink: Sendable, Equatable {
    public var site: Site
    /// そのサイトで使う検索ワード(site_overrides.query > queryOverride > テンプレート)
    public var query: String
    public var url: URL

    public init(site: Site, query: String, url: URL) {
        self.site = site
        self.query = query
        self.url = url
    }
}

public enum SiteLinks {
    /// ジャンルの siteIDs 順。enabled=false のサイトと、未知のサイト ID は除く。genre が nil なら空。
    /// 組み立てた URL が http(s) でないサイトは除く(API から来た値なので描画前にも検査する)。
    /// ジャンルのテンプレートが無いとき(genre が nil)は空配列。
    public static func resolve(item: Item, genre: Genre?, sites: [Site]) -> [SiteLink] {
        guard let genre else { return [] }
        var links: [SiteLink] = []
        for id in genre.siteIDs {
            guard let site = sites.first(where: { $0.id == id }) else { continue }
            let override = item.siteOverrides.first { $0.siteID == id }
            if let override, !override.enabled { continue }
            let query = queryFor(item: item, genre: genre, siteQuery: override?.query)
            let urlString = Deeplink.build(template: site.searchURLTemplate, query: query)
            guard Deeplink.isHTTPURL(urlString), let url = URL(string: urlString) else { continue }
            links.append(SiteLink(site: site, query: query, url: url))
        }
        return links
    }

    /// 商品の検索ワード(サイト別の上書きを除く。シートの検索ワード欄に出す値)。genre が nil なら `{name} {option}`。
    public static func itemQuery(item: Item, genre: Genre?) -> String {
        queryFor(item: item, genre: genre, siteQuery: nil)
    }

    private static func queryFor(item: Item, genre: Genre?, siteQuery: String?) -> String {
        SearchQuery.build(QueryInput(
            template: genre?.queryTemplate ?? "{name} {option}", name: item.name, option: item.optionText,
            queryOverride: item.queryOverride, siteQuery: siteQuery))
    }
}
