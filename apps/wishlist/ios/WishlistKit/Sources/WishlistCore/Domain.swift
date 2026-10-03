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

    public init(id: Int, name: String, queryTemplate: String = "{name} {option}", sortOrder: Int = 0, siteIDs: [Int] = []) {
        self.id = id
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
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

    public init(
        id: Int, genreID: Int, name: String, optionText: String? = nil, queryOverride: String? = nil,
        imageURLPath: String, sourceURL: String? = nil, minPrice: Int? = nil, sortOrder: Int = 0,
        siteOverrides: [SiteOverride] = [], createdAt: Date = Date(timeIntervalSince1970: 1_790_000_000),
        updatedAt: Date = Date(timeIntervalSince1970: 1_790_000_000)
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

    public init(
        genreID: Int? = nil, name: String? = nil, optionText: FieldUpdate<String> = .keep,
        queryOverride: FieldUpdate<String> = .keep, sourceURL: FieldUpdate<String> = .keep,
        minPrice: FieldUpdate<Int> = .keep, sortOrder: Int? = nil, siteOverrides: [SiteOverride]? = nil
    ) {
        self.genreID = genreID
        self.name = name
        self.optionText = optionText
        self.queryOverride = queryOverride
        self.sourceURL = sourceURL
        self.minPrice = minPrice
        self.sortOrder = sortOrder
        self.siteOverrides = siteOverrides
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

    public init(name: String, queryTemplate: String? = nil, sortOrder: Int? = nil, siteIDs: [Int]? = nil) {
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
    }
}

public struct GenreUpdate: Sendable, Equatable {
    public var name: String?
    public var queryTemplate: String?
    public var sortOrder: Int?
    /// 渡すと表示するサイトと順序を全件置き換える
    public var siteIDs: [Int]?

    public init(name: String? = nil, queryTemplate: String? = nil, sortOrder: Int? = nil, siteIDs: [Int]? = nil) {
        self.name = name
        self.queryTemplate = queryTemplate
        self.sortOrder = sortOrder
        self.siteIDs = siteIDs
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
    public var status: EstimateStatus
    public var fetchedAt: Date

    public init(siteID: Int, low: Int? = nil, mid: Int? = nil, count: Int, suspiciousCount: Int, status: EstimateStatus, fetchedAt: Date) {
        self.siteID = siteID
        self.low = low
        self.mid = mid
        self.count = count
        self.suspiciousCount = suspiciousCount
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
