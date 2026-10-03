import Foundation
import HTTPTypes
import OpenAPIRuntime
import OpenAPIURLSession
import WishlistAPI

private typealias Schemas = Components.Schemas

/// `WishlistService` を生成クライアント(`WishlistAPI.Client`)で実装する。
///
/// - 全 `/api/*` に `Authorization: Bearer <token>` を付ける(ClientMiddleware)。
/// - ベース URL は末尾 `/` の有無によらず、前置(`/wishlist`)を残して `api/items` などを連結する。
/// - **PATCH の null**: 生成型 `ItemUpdate` の nullable 項目は `String?` で、nil は JSON から省略される(null を送れない)。
///   `ItemPatch` の `.clear` は、ミドルウェアが送信直前にボディを組み直して JSON の `null` にする(`PatchNulls`)。
/// - 失敗は `WishlistError`: Error スキーマの code を写す。JSON でない応答はステータスから決める(401 unauthorized・404 not_found・
///   400 bad_request・422 unprocessable・501 not_implemented・502 bad_gateway・その他 internal)。通信できなかったときは `.network`(status 0)。
public struct APIWishlistService: WishlistService {
    private let client: Client

    public init(baseURL: URL, token: String, transport: any ClientTransport = URLSessionTransport()) {
        // 前置パスを残すため、サーバー URL は末尾 `/` を除いて渡す(生成クライアントは `/api/...` を連結する)。
        var text = baseURL.absoluteString
        while text.hasSuffix("/") { text.removeLast() }
        client = Client(
            serverURL: URL(string: text) ?? baseURL, transport: transport,
            middlewares: [WishlistMiddleware(token: token, baseURL: baseURL)])
    }

    // MARK: - 共通

    /// 生成クライアントの呼び出しを包み、失敗を `WishlistError` にする。
    private func call<T>(_ body: () async throws -> T) async throws -> T {
        do {
            return try await body()
        } catch {
            if isCancellation(error) { throw error }
            throw Self.wishlistError(error)
        }
    }

    private static func wishlistError(_ error: any Error) -> WishlistError {
        if let error = error as? WishlistError { return error }
        if let error = error as? ClientError { return wishlistError(error.underlyingError) }
        if error is DecodingError {
            return WishlistError(code: .decode, message: "応答を読めませんでした")
        }
        if error is URLError || (!(error is CancellationError) && (error as NSError).domain == NSURLErrorDomain) {
            return WishlistError(code: .network, message: error.localizedDescription)
        }
        return WishlistError(code: .decode, message: error.localizedDescription)
    }

    private static func itemFieldsBody(_ f: ItemFields) -> Schemas.ItemFields {
        Schemas.ItemFields(
            genreId: Int64(f.genreID), name: f.name, optionText: f.optionText, queryOverride: f.queryOverride,
            sourceUrl: f.sourceURL, minPrice: f.minPrice, sortOrder: f.sortOrder)
    }

    private static func part(_ text: String) -> HTTPBody { HTTPBody(text) }

    // MARK: - 商品

    public func listItems() async throws -> [Item] {
        try await call {
            try await client.listItems().ok.body.json.items.map(Self.item)
        }
    }

    public func createItem(_ fields: ItemFields, image: ImageUpload) async throws -> Item {
        try await call {
            var parts: [Schemas.ItemCreateMultipart] = [
                .genreId(.init(payload: .init(body: Self.part(String(fields.genreID))))),
                .name(.init(payload: .init(body: Self.part(fields.name)))),
            ]
            if let v = fields.optionText { parts.append(.optionText(.init(payload: .init(body: Self.part(v))))) }
            if let v = fields.queryOverride { parts.append(.queryOverride(.init(payload: .init(body: Self.part(v))))) }
            if let v = fields.sourceURL { parts.append(.sourceUrl(.init(payload: .init(body: Self.part(v))))) }
            if let v = fields.minPrice { parts.append(.minPrice(.init(payload: .init(body: Self.part(String(v)))))) }
            if let v = fields.sortOrder { parts.append(.sortOrder(.init(payload: .init(body: Self.part(String(v)))))) }
            parts.append(.image(.init(payload: .init(body: HTTPBody(image.data)), filename: image.filename)))
            let output = try await client.createItem(body: .multipartForm(MultipartBody(parts)))
            return Self.item(try output.created.body.json)
        }
    }

    public func createItem(_ fields: ItemFields, imageURL: String) async throws -> Item {
        try await call {
            let body = Schemas.ItemCreateJSON(
                value1: Self.itemFieldsBody(fields), value2: .init(imageUrl: imageURL))
            return Self.item(try await client.createItem(body: .json(body)).created.body.json)
        }
    }

    public func draftFromURL(_ url: String, genreID: Int?) async throws -> ItemDraft {
        try await call {
            let body = Schemas.FromURLRequest(url: url, genreId: genreID.map(Int64.init))
            let draft = try await client.draftItemFromURL(body: .json(body)).ok.body.json
            return ItemDraft(
                name: draft.name, imageURL: draft.imageUrl, sourceURL: draft.sourceUrl, genreID: draft.genreId.map(Int.init))
        }
    }

    public func updateItem(id: Int, patch: ItemPatch) async throws -> Item {
        try await call {
            var body = Schemas.ItemUpdate(
                genreId: patch.genreID.map(Int64.init), name: patch.name, sortOrder: patch.sortOrder,
                siteOverrides: patch.siteOverrides?.map(Self.siteOverride))
            var cleared: Set<String> = []
            switch patch.optionText {
            case .keep: break
            case .set(let v): body.optionText = v
            case .clear: cleared.insert("option_text")
            }
            switch patch.queryOverride {
            case .keep: break
            case .set(let v): body.queryOverride = v
            case .clear: cleared.insert("query_override")
            }
            switch patch.sourceURL {
            case .keep: break
            case .set(let v): body.sourceUrl = v
            case .clear: cleared.insert("source_url")
            }
            switch patch.minPrice {
            case .keep: break
            case .set(let v): body.minPrice = v
            case .clear: cleared.insert("min_price")
            }
            let output = try await PatchNulls.$keys.withValue(cleared) {
                try await client.updateItem(path: .init(id: Int64(id)), body: .json(body))
            }
            return Self.item(try output.ok.body.json)
        }
    }

    public func deleteItem(id: Int) async throws {
        try await call {
            _ = try await client.deleteItem(path: .init(id: Int64(id))).noContent
        }
    }

    public func replaceItemImage(id: Int, image: ImageUpload) async throws -> Item {
        try await call {
            let part: Operations.ReplaceItemImage.Input.Body.MultipartFormPayload = .image(
                .init(payload: .init(body: HTTPBody(image.data)), filename: image.filename))
            let output = try await client.replaceItemImage(
                path: .init(id: Int64(id)), body: .multipartForm(MultipartBody([part])))
            return Self.item(try output.ok.body.json)
        }
    }

    public func estimates(itemID: Int) async throws -> ItemEstimates {
        try await call {
            Self.estimates(try await client.getItemEstimates(path: .init(id: Int64(itemID))).ok.body.json)
        }
    }

    public func refreshEstimates(itemID: Int) async throws -> ItemEstimates {
        try await call {
            Self.estimates(try await client.refreshItemEstimates(path: .init(id: Int64(itemID))).accepted.body.json)
        }
    }

    public func listings(itemID: Int, siteID: Int?) async throws -> [Listing] {
        try await call {
            let output = try await client.listItemListings(
                path: .init(id: Int64(itemID)), query: .init(siteId: siteID.map(Int64.init)))
            return try output.ok.body.json.listings.map { l in
                Listing(
                    id: Int(l.id), siteID: Int(l.siteId), title: l.title, price: l.price, url: l.url, imageURL: l.imageUrl,
                    inStock: l.inStock, suspiciousReasons: l.suspiciousReasons.compactMap { SuspiciousReason(rawValue: $0.rawValue) },
                    fetchedAt: l.fetchedAt)
            }
        }
    }

    /// TODO(implementer): docs/phase4-spec.md AC-IOS-HIS-API-01(生成クライアントの `getItemPriceHistory`。days が nil ならクエリなし)
    public func priceHistory(itemID: Int, days: Int?) async throws -> PriceHistory {
        _ = (itemID, days)
        throw WishlistError.stub
    }

    private static func estimates(_ e: Schemas.ItemEstimates) -> ItemEstimates {
        ItemEstimates(
            itemID: Int(e.itemId), summaryLow: e.summaryLow, summaryMid: e.summaryMid, summaryFetchedAt: e.summaryFetchedAt,
            sites: e.sites.map { s in
                SiteEstimate(
                    siteID: Int(s.siteId), low: s.low, mid: s.mid, count: s.count, suspiciousCount: s.suspiciousCount,
                    inStockCount: s.inStockCount, status: EstimateStatus(rawValue: s.status.rawValue) ?? .failed,
                    fetchedAt: s.fetchedAt)
            },
            refreshing: e.refreshing)
    }

    // MARK: - ジャンル・サイト

    public func listGenres() async throws -> [Genre] {
        try await call { try await client.listGenres().ok.body.json.genres.map(Self.genre) }
    }

    public func createGenre(_ body: GenreCreate) async throws -> Genre {
        try await call {
            let request = Schemas.GenreCreate(
                name: body.name, queryTemplate: body.queryTemplate, sortOrder: body.sortOrder, siteIds: body.siteIDs?.map(Int64.init))
            return Self.genre(try await client.createGenre(body: .json(request)).created.body.json)
        }
    }

    public func updateGenre(id: Int, patch: GenreUpdate) async throws -> Genre {
        try await call {
            let request = Schemas.GenreUpdate(
                name: patch.name, queryTemplate: patch.queryTemplate, sortOrder: patch.sortOrder,
                siteIds: patch.siteIDs?.map(Int64.init))
            return Self.genre(try await client.updateGenre(path: .init(id: Int64(id)), body: .json(request)).ok.body.json)
        }
    }

    public func listSites() async throws -> [Site] {
        try await call { try await client.listSites().ok.body.json.sites.map(Self.site) }
    }

    public func createSite(_ body: SiteCreate) async throws -> Site {
        try await call {
            let request = Schemas.SiteCreate(
                name: body.name, searchUrlTemplate: body.searchURLTemplate,
                fetchType: body.fetchType.flatMap { Schemas.FetchType(rawValue: $0.rawValue) }.map { .init(value1: $0) },
                isReference: body.isReference)
            return Self.site(try await client.createSite(body: .json(request)).created.body.json)
        }
    }

    public func updateSite(id: Int, patch: SiteUpdate) async throws -> Site {
        try await call {
            let request = Schemas.SiteUpdate(
                name: patch.name, searchUrlTemplate: patch.searchURLTemplate,
                fetchType: patch.fetchType.flatMap { Schemas.FetchType(rawValue: $0.rawValue) }, isReference: patch.isReference)
            return Self.site(try await client.updateSite(path: .init(id: Int64(id)), body: .json(request)).ok.body.json)
        }
    }

    // MARK: - 写像(生成型 → ドメイン)

    private static func siteOverride(_ o: SiteOverride) -> Schemas.SiteOverride {
        Schemas.SiteOverride(siteId: Int64(o.siteID), query: o.query, enabled: o.enabled)
    }

    private static func item(_ i: Schemas.Item) -> Item {
        Item(
            id: Int(i.id), genreID: Int(i.genreId), name: i.name, optionText: i.optionText, queryOverride: i.queryOverride,
            imageURLPath: i.imageUrl, sourceURL: i.sourceUrl, minPrice: i.minPrice, sortOrder: i.sortOrder,
            siteOverrides: i.siteOverrides.map { SiteOverride(siteID: Int($0.siteId), query: $0.query, enabled: $0.enabled) },
            createdAt: i.createdAt, updatedAt: i.updatedAt)
    }

    private static func genre(_ g: Schemas.Genre) -> Genre {
        Genre(id: Int(g.id), name: g.name, queryTemplate: g.queryTemplate, sortOrder: g.sortOrder, siteIDs: g.siteIds.map(Int.init))
    }

    private static func site(_ s: Schemas.Site) -> Site {
        Site(
            id: Int(s.id), name: s.name, searchURLTemplate: s.searchUrlTemplate,
            fetchType: FetchType(rawValue: s.fetchType.rawValue) ?? .linkOnly, isReference: s.isReference)
    }
}

/// PATCH で null を送りたい項目の JSON キー(`ItemPatch` の `.clear`)を、ミドルウェアへ渡す。
enum PatchNulls {
    @TaskLocal static var keys: Set<String> = []
}

/// Bearer トークンの付与、PATCH の null の組み立て、エラー応答・通信失敗の `WishlistError` への変換。
struct WishlistMiddleware: ClientMiddleware {
    let token: String
    /// 設定したベース URL(トークンを付ける範囲の判定に使う)
    let baseURL: URL

    func intercept(
        _ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String,
        next: @Sendable (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var request = request
        var body = body
        if let full = URL(string: baseURL.absoluteString.trimmingSuffix("/") + (request.path ?? "")),
            AuthPolicy.shouldAttachToken(url: full, baseURL: self.baseURL)
        {
            request.headerFields[.authorization] = "Bearer \(token)"
        }

        // JSON は charset 無しの `application/json` で送る(契約の media type と一致させる)。
        if let type = request.headerFields[.contentType], type.hasPrefix("application/json") {
            request.headerFields[.contentType] = "application/json"
        }

        let nulls = PatchNulls.keys
        if request.method == .patch, !nulls.isEmpty {
            var object: [String: Any] = [:]
            if let body {
                let data = try await Data(collecting: body, upTo: 10_000_000)
                object = (try? JSONSerialization.jsonObject(with: data) as? [String: Any]) ?? [:]
            }
            for key in nulls { object[key] = NSNull() }
            body = HTTPBody(try JSONSerialization.data(withJSONObject: object))
        }

        let response: HTTPResponse
        let responseBody: HTTPBody?
        do {
            (response, responseBody) = try await next(request, body, baseURL)
        } catch {
            // キャンセルは通信失敗にしない(そのまま投げる)
            if isCancellation(error) { throw error }
            throw WishlistError(code: .network, message: error.localizedDescription)
        }
        guard response.status.code >= 400 else { return (response, responseBody) }
        throw await Self.error(response, body: responseBody)
    }

    private struct ErrorBody: Decodable {
        var code: String
        var message: String
    }

    private static func error(_ response: HTTPResponse, body: HTTPBody?) async -> WishlistError {
        let status = response.status.code
        if let body, let data = try? await Data(collecting: body, upTo: 1_000_000),
            let decoded = try? JSONDecoder().decode(ErrorBody.self, from: data),
            let code = WishlistError.Code(rawValue: decoded.code)
        {
            return WishlistError(code: code, status: status, message: decoded.message)
        }
        let code: WishlistError.Code =
            switch status {
            case 400: .badRequest
            case 401: .unauthorized
            case 404: .notFound
            case 422: .unprocessable
            case 501: .notImplemented
            case 502: .badGateway
            default: .internal
            }
        return WishlistError(code: code, status: status, message: "HTTP \(status)")
    }
}

private extension String {
    func trimmingSuffix(_ suffix: Character) -> String {
        var text = self
        while text.last == suffix { text.removeLast() }
        return text
    }
}

/// キャンセル(Task のキャンセル・URLError.cancelled)か。生成クライアントが包んだ `ClientError` の中も見る。
func isCancellation(_ error: any Error) -> Bool {
    if error is CancellationError { return true }
    if (error as? URLError)?.code == .cancelled { return true }
    if let error = error as? ClientError { return isCancellation(error.underlyingError) }
    return false
}
