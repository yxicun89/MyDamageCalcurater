import Foundation

// ドメインの型(api/openapi.yaml の schema を、生成型から切り離して Swift らしい名前にしたもの)。
// 生成型 ↔ ドメインの写像は APIWishlistService に閉じる(PokeCalcKit と同じ方針)。
// このファイルは spec-writer が置いた完成品(データの入れ物だけ。ロジックなし)。

public enum FetchType: String, Codable, Sendable, CaseIterable, Equatable {
    case api
    case scrape
    case headless
    case linkOnly = "link_only"
}

public struct Genre: Codable, Sendable, Equatable, Hashable, Identifiable {
    public var id: Int
    public var name: String
    /// `{name}` と `{option}` を埋め込む
    public var queryTemplate: String
    public var sortOrder: Int
    /// 表示するサイト(表示順)
    public var siteIDs: [Int]
    /// 表記揺れの辞書(フェーズ4-1)。1 グループ = 同じものを指す語(2 語以上)。無ければ空。
    public var aliases: [[String]]

    public init(
        id: Int, name: String, queryTemplate: String = "{name} {option}", sortOrder: Int = 0, siteIDs: [Int] = [],
        aliases: [[String]] = []
    ) {
        self.id = id
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
        self.aliases = aliases
    }

    private enum CodingKeys: String, CodingKey {
        case id, name, queryTemplate, sortOrder, siteIDs, aliases
    }

    /// 保存済みのキャッシュ(aliases を持たない古い JSON)も読めるよう、aliases が無ければ空にする。
    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        self.init(
            id: try c.decode(Int.self, forKey: .id), name: try c.decode(String.self, forKey: .name),
            queryTemplate: try c.decode(String.self, forKey: .queryTemplate), sortOrder: try c.decode(Int.self, forKey: .sortOrder),
            siteIDs: try c.decode([Int].self, forKey: .siteIDs), aliases: try c.decodeIfPresent([[String]].self, forKey: .aliases) ?? [])
    }
}

public struct Site: Codable, Sendable, Equatable, Hashable, Identifiable {
    public var id: Int
    public var name: String
    /// `{q}` を encodeURIComponent(検索ワード) で置き換える
    public var searchURLTemplate: String
    public var fetchType: FetchType
    public var isReference: Bool

    public init(id: Int, name: String, searchURLTemplate: String, fetchType: FetchType = .linkOnly, isReference: Bool = false) {
        self.id = id
        self.name = name
        self.searchURLTemplate = searchURLTemplate
        self.fetchType = fetchType
        self.isReference = isReference
    }
}

public struct SiteOverride: Codable, Sendable, Equatable, Hashable {
    public var siteID: Int
    /// このサイトだけで使う検索ワード(テンプレート・queryOverride より優先)
    public var query: String?
    /// false ならこの商品ではこのサイトを出さない
    public var enabled: Bool

    public init(siteID: Int, query: String? = nil, enabled: Bool = true) {
        self.siteID = siteID
        self.query = query
        self.enabled = enabled
    }
}

public struct Item: Codable, Sendable, Equatable, Hashable, Identifiable {
    public var id: Int
    public var genreID: Int
    public var name: String
    public var optionText: String?
    public var queryOverride: String?
    /// 画像のパス `images/<name>`(先頭に / を付けない)。`WishlistURLs.imageURL(path:baseURL:)` でベース URL 基準に解決する
    public var imageURLPath: String
    public var sourceURL: String?
    public var minPrice: Int?
    public var sortOrder: Int
    public var siteOverrides: [SiteOverride]
    public var createdAt: Date
    public var updatedAt: Date
    /// 公式ページ(sourceURL)の販売状況を夜間に監視するか(フェーズ4-3)。応答に無ければ false
    public var watchOfficial: Bool
    /// 公式ページの販売状況(まだ確かめていなければ nil)
    public var officialStatus: OfficialStatus?

    public init(
        id: Int, genreID: Int, name: String, optionText: String? = nil, queryOverride: String? = nil,
        imageURLPath: String, sourceURL: String? = nil, minPrice: Int? = nil, sortOrder: Int = 0,
        siteOverrides: [SiteOverride] = [], createdAt: Date = Date(timeIntervalSince1970: 1_790_000_000),
        updatedAt: Date = Date(timeIntervalSince1970: 1_790_000_000), watchOfficial: Bool = false,
        officialStatus: OfficialStatus? = nil
    ) {
        self.id = id
        self.genreID = genreID
        self.name = name
        self.optionText = optionText
        self.queryOverride = queryOverride
        self.imageURLPath = imageURLPath
        self.sourceURL = sourceURL
        self.minPrice = minPrice
        self.sortOrder = sortOrder
        self.siteOverrides = siteOverrides
        self.createdAt = createdAt
        self.updatedAt = updatedAt
        self.watchOfficial = watchOfficial
        self.officialStatus = officialStatus
    }

    private enum CodingKeys: String, CodingKey {
        case id, genreID, name, optionText, queryOverride, imageURLPath, sourceURL, minPrice, sortOrder, siteOverrides, createdAt, updatedAt
        case watchOfficial, officialStatus
    }

    /// 保存済みのキャッシュ(watchOfficial・officialStatus を持たない古い JSON)も読めるよう、無ければ false・nil にする。
    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        self.init(
            id: try c.decode(Int.self, forKey: .id), genreID: try c.decode(Int.self, forKey: .genreID),
            name: try c.decode(String.self, forKey: .name), optionText: try c.decodeIfPresent(String.self, forKey: .optionText),
            queryOverride: try c.decodeIfPresent(String.self, forKey: .queryOverride),
            imageURLPath: try c.decode(String.self, forKey: .imageURLPath), sourceURL: try c.decodeIfPresent(String.self, forKey: .sourceURL),
            minPrice: try c.decodeIfPresent(Int.self, forKey: .minPrice), sortOrder: try c.decode(Int.self, forKey: .sortOrder),
            siteOverrides: try c.decode([SiteOverride].self, forKey: .siteOverrides),
            createdAt: try c.decode(Date.self, forKey: .createdAt), updatedAt: try c.decode(Date.self, forKey: .updatedAt),
            watchOfficial: try c.decodeIfPresent(Bool.self, forKey: .watchOfficial) ?? false,
            officialStatus: try c.decodeIfPresent(OfficialStatus.self, forKey: .officialStatus))
    }
}

// MARK: - フェーズ4-3 公式サイトの販売状況(docs/phase4-spec.md 4-3)

/// 公式ページの販売状況(api/openapi.yaml の OfficialState)
public enum OfficialState: String, Codable, Sendable, Equatable, Hashable, CaseIterable {
    case available
    case preorder
    case soldout
    case ended
    case unknown
    case ambiguous
    case blocked
    case failed
}

/// 保存された販売状況。`status` は最後に判定できた状態(最後の試行が failed・blocked でも上書きしない)。
/// `lastResult`・`lastAttemptAt` は最後の試行の結果と時刻。
public struct OfficialStatus: Codable, Sendable, Equatable, Hashable {
    public var status: OfficialState
    /// 判定の根拠にした語(最大 3。ページに現れた順)
    public var evidence: [String]
    /// `status` を確かめた時刻
    public var checkedAt: Date
    /// `status` が前回の判定から変わった時刻(無ければ nil)
    public var changedAt: Date?
    public var previousStatus: OfficialState?
    public var lastResult: OfficialState
    public var lastAttemptAt: Date

    public init(
        status: OfficialState, evidence: [String] = [], checkedAt: Date, changedAt: Date? = nil, previousStatus: OfficialState? = nil,
        lastResult: OfficialState? = nil, lastAttemptAt: Date? = nil
    ) {
        self.status = status
        self.evidence = evidence
        self.checkedAt = checkedAt
        self.changedAt = changedAt
        self.previousStatus = previousStatus
        self.lastResult = lastResult ?? status
        self.lastAttemptAt = lastAttemptAt ?? checkedAt
    }
}

/// URL の OGP から作った下書き(保存はしない)
public struct ItemDraft: Codable, Sendable, Equatable {
    public var name: String
    /// og:image の絶対 URL(無ければ nil)
    public var imageURL: String?
    public var sourceURL: String
    public var genreID: Int?

    public init(name: String, imageURL: String?, sourceURL: String, genreID: Int? = nil) {
        self.name = name
        self.imageURL = imageURL
        self.sourceURL = sourceURL
        self.genreID = genreID
    }
}

/// 登録時に送る商品の項目(画像を除く)
public struct ItemFields: Sendable, Equatable {
    public var genreID: Int
    public var name: String
    public var optionText: String?
    public var queryOverride: String?
    public var sourceURL: String?
    public var minPrice: Int?
    public var sortOrder: Int?

    public init(
        genreID: Int, name: String, optionText: String? = nil, queryOverride: String? = nil,
        sourceURL: String? = nil, minPrice: Int? = nil, sortOrder: Int? = nil
    ) {
        self.genreID = genreID
        self.name = name
        self.optionText = optionText
        self.queryOverride = queryOverride
        self.sourceURL = sourceURL
        self.minPrice = minPrice
        self.sortOrder = sortOrder
    }
}

/// アップロードする画像(multipart の `image`)
public struct ImageUpload: Sendable, Equatable {
    public var data: Data
    public var filename: String
    public var contentType: String

    public init(data: Data, filename: String, contentType: String) {
        self.data = data
        self.filename = filename
        self.contentType = contentType
    }
}

/// PATCH の 1 項目。API は「項目を省略 = 変えない」と「null = 消す」を区別する(docs/design.md W-08)。
public enum FieldUpdate<Value: Sendable & Equatable>: Sendable, Equatable {
    case keep
    case set(Value)
    case clear
}

/// `PATCH /api/items/{id}`。変えた項目だけを入れる。nullable な 4 項目は `FieldUpdate`(null を送れる)。
public struct ItemPatch: Sendable, Equatable {
    public var genreID: Int?
    public var name: String?
    public var optionText: FieldUpdate<String> = .keep
    public var queryOverride: FieldUpdate<String> = .keep
    public var sourceURL: FieldUpdate<String> = .keep
    public var minPrice: FieldUpdate<Int> = .keep
    public var sortOrder: Int?
    public var siteOverrides: [SiteOverride]?
    /// 公式ページの監視(フェーズ4-3)。nil なら送らない
    public var watchOfficial: Bool?

    public init(
        genreID: Int? = nil, name: String? = nil, optionText: FieldUpdate<String> = .keep,
        queryOverride: FieldUpdate<String> = .keep, sourceURL: FieldUpdate<String> = .keep,
        minPrice: FieldUpdate<Int> = .keep, sortOrder: Int? = nil, siteOverrides: [SiteOverride]? = nil,
        watchOfficial: Bool? = nil
    ) {
        self.genreID = genreID
        self.name = name
        self.optionText = optionText
        self.queryOverride = queryOverride
        self.sourceURL = sourceURL
        self.minPrice = minPrice
        self.sortOrder = sortOrder
        self.siteOverrides = siteOverrides
        self.watchOfficial = watchOfficial
    }

    /// 何も変えない(送る項目が無い)なら true
    public var isEmpty: Bool { self == ItemPatch() }
}

public struct GenreCreate: Sendable, Equatable {
    public var name: String
    /// nil ならサーバーの既定(`{name} {option}`)
    public var queryTemplate: String?
    public var sortOrder: Int?
    public var siteIDs: [Int]?
    /// 表記揺れの辞書(フェーズ4-1)。nil なら送らない
    public var aliases: [[String]]?

    public init(name: String, queryTemplate: String? = nil, sortOrder: Int? = nil, siteIDs: [Int]? = nil, aliases: [[String]]? = nil) {
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
        self.aliases = aliases
    }
}

public struct GenreUpdate: Sendable, Equatable {
    public var name: String?
    public var queryTemplate: String?
    public var sortOrder: Int?
    /// 渡すと表示するサイトと順序を全件置き換える
    public var siteIDs: [Int]?
    /// 渡すと表記揺れの辞書を全件置き換える(空なら全部消す。フェーズ4-1)
    public var aliases: [[String]]?

    public init(name: String? = nil, queryTemplate: String? = nil, sortOrder: Int? = nil, siteIDs: [Int]? = nil, aliases: [[String]]? = nil) {
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
        self.aliases = aliases
    }
}

public struct SiteCreate: Sendable, Equatable {
    public var name: String
    public var searchURLTemplate: String
    /// nil ならサーバーの既定(link_only)
    public var fetchType: FetchType?
    public var isReference: Bool?

    public init(name: String, searchURLTemplate: String, fetchType: FetchType? = nil, isReference: Bool? = nil) {
        self.name = name
        self.searchURLTemplate = searchURLTemplate
        self.fetchType = fetchType
        self.isReference = isReference
    }
}

public struct SiteUpdate: Sendable, Equatable {
    public var name: String?
    public var searchURLTemplate: String?
    public var fetchType: FetchType?
    public var isReference: Bool?

    public init(name: String? = nil, searchURLTemplate: String? = nil, fetchType: FetchType? = nil, isReference: Bool? = nil) {
        self.name = name
        self.searchURLTemplate = searchURLTemplate
        self.fetchType = fetchType
        self.isReference = isReference
    }
}

public enum EstimateStatus: String, Codable, Sendable, Equatable {
    case ok
    case failed
    case noResult = "no_result"
}

public struct SiteEstimate: Codable, Sendable, Equatable {
    public var siteID: Int
    public var low: Int?
    public var mid: Int?
    public var count: Int
    public var suspiciousCount: Int
    /// 参考外を除き、在庫ありの件数(0 なら「在庫なし」)
    public var inStockCount: Int
    public var status: EstimateStatus
    /// `status` が failed のときは前回取得した時刻
    public var fetchedAt: Date

    public init(
        siteID: Int, low: Int? = nil, mid: Int? = nil, count: Int, suspiciousCount: Int, inStockCount: Int = 0,
        status: EstimateStatus, fetchedAt: Date
    ) {
        self.siteID = siteID
        self.low = low
        self.mid = mid
        self.count = count
        self.suspiciousCount = suspiciousCount
        self.inStockCount = inStockCount
        self.status = status
        self.fetchedAt = fetchedAt
    }
}

public struct ItemEstimates: Codable, Sendable, Equatable {
    public var itemID: Int
    public var summaryLow: Int?
    public var summaryMid: Int?
    public var summaryFetchedAt: Date?
    public var sites: [SiteEstimate]
    public var refreshing: Bool

    public init(itemID: Int, summaryLow: Int? = nil, summaryMid: Int? = nil, summaryFetchedAt: Date? = nil, sites: [SiteEstimate] = [], refreshing: Bool = false) {
        self.itemID = itemID
        self.summaryLow = summaryLow
        self.summaryMid = summaryMid
        self.summaryFetchedAt = summaryFetchedAt
        self.sites = sites
        self.refreshing = refreshing
    }
}

/// 参考外と判定した理由(api/openapi.yaml の SuspiciousReason)
public enum SuspiciousReason: String, Codable, Sendable, Equatable, CaseIterable {
    case titleMismatch = "title_mismatch"
    case tooCheap = "too_cheap"
    case belowMin = "below_min"
}

/// 取得した出品 1 件(`GET /api/items/{id}/listings`)。`suspiciousReasons` が空なら参考にしている出品。
public struct Listing: Codable, Sendable, Equatable, Identifiable {
    public var id: Int
    public var siteID: Int
    public var title: String
    /// 円(送料は含めない)
    public var price: Int
    public var url: String
    public var imageURL: String?
    public var inStock: Bool
    public var suspiciousReasons: [SuspiciousReason]
    public var fetchedAt: Date

    public init(
        id: Int, siteID: Int, title: String, price: Int, url: String, imageURL: String? = nil, inStock: Bool = true,
        suspiciousReasons: [SuspiciousReason] = [], fetchedAt: Date = Date(timeIntervalSince1970: 1_790_985_600)
    ) {
        self.id = id
        self.siteID = siteID
        self.title = title
        self.price = price
        self.url = url
        self.imageURL = imageURL
        self.inStock = inStock
        self.suspiciousReasons = suspiciousReasons
        self.fetchedAt = fetchedAt
    }
}

// MARK: - フェーズ4-2 価格の推移(docs/phase4-spec.md 4-2)

/// 1 サイトの 1 日(JST)の目安(`GET /api/items/{id}/price-history`)。`day` は API の `YYYY-MM-DD` のまま持つ。
public struct PricePoint: Codable, Sendable, Equatable {
    public var day: String
    public var low: Int
    /// 件数 3 未満の日は nil
    public var mid: Int?

    public init(day: String, low: Int, mid: Int? = nil) {
        self.day = day
        self.low = low
        self.mid = mid
    }
}

/// 1 サイトの推移(day 昇順。点のある日だけ)
public struct SitePriceHistory: Codable, Sendable, Equatable {
    public var siteID: Int
    public var points: [PricePoint]

    public init(siteID: Int, points: [PricePoint]) {
        self.siteID = siteID
        self.points = points
    }
}

/// その日の全サイトの low の最小
public struct DayLow: Codable, Sendable, Equatable {
    public var day: String
    public var low: Int

    public init(day: String, low: Int) {
        self.day = day
        self.low = low
    }
}

/// 価格の推移。`sites` は点のあるサイトだけ(ジャンルの表示順)、`overall` は day 昇順
public struct PriceHistory: Codable, Sendable, Equatable {
    public var itemID: Int
    public var days: Int
    public var sites: [SitePriceHistory]
    public var overall: [DayLow]

    public init(itemID: Int, days: Int = 90, sites: [SitePriceHistory] = [], overall: [DayLow] = []) {
        self.itemID = itemID
        self.days = days
        self.sites = sites
        self.overall = overall
    }
}
