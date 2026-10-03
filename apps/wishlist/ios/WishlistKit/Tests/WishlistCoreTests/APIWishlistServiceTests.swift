import HTTPTypes
import OpenAPIRuntime
import OpenAPIURLSession
import XCTest

@testable import WishlistCore

// AC-IOS-SVC-01/02: APIWishlistService(生成クライアント + Bearer ミドルウェア)の送信内容と応答の写像。
// 実ネットワークは使わず、送られたリクエストを RecordingTransport で検査する。URL の連結だけは本物の URLSessionTransport + URLProtocol。
final class APIWishlistServiceTests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!
    private let token = "secret-token"

    private func service(_ transport: RecordingTransport, baseURL: URL? = nil) -> APIWishlistService {
        APIWishlistService(baseURL: baseURL ?? base, token: token, transport: transport)
    }

    private func itemJSON(id: Int = 12, extra: String = "") -> String {
        """
        {"id":\(id),"genre_id":1,"name":"グリス","option_text":null,"query_override":null,
         "image_url":"images/00000000-0000-4000-8000-0000000000\(id).png","source_url":null,"min_price":null,
         "sort_order":0,"site_overrides":[{"site_id":2,"query":null,"enabled":false}],
         "created_at":"2026-10-03T00:00:00Z","updated_at":"2026-10-03T00:00:00Z"\(extra)}
        """
    }

    // MARK: - 認証・パス・写像

    func testListItemsSendsBearerTokenAndMapsFields() async throws {
        let transport = RecordingTransport(reply: .json(#"{"items":[\#(itemJSON())]}"#))
        let items = try await service(transport).listItems()

        let request = try XCTUnwrap(transport.last).request
        XCTAssertEqual(request.method, .get)
        XCTAssertEqual(request.path, "/api/items")
        XCTAssertEqual(request.headerFields[.authorization], "Bearer secret-token")

        let item = try XCTUnwrap(items.first)
        XCTAssertEqual(items.count, 1)
        XCTAssertEqual(item.id, 12)
        XCTAssertEqual(item.genreID, 1)
        XCTAssertEqual(item.name, "グリス")
        XCTAssertNil(item.optionText)
        XCTAssertNil(item.queryOverride)
        XCTAssertEqual(item.imageURLPath, "images/00000000-0000-4000-8000-000000000012.png")
        XCTAssertEqual(item.siteOverrides, [SiteOverride(siteID: 2, query: nil, enabled: false)])
        XCTAssertEqual(item.createdAt, Date(timeIntervalSince1970: 1_790_985_600))
    }

    /// 前置 `/wishlist` を残して連結する(末尾 / の有無によらない)。本物の URLSessionTransport で確かめる。
    func testBaseURLPrefixIsKeptWithAndWithoutTrailingSlash() async throws {
        StubURLProtocol.install { _ in (200, Data(#"{"items":[]}"#.utf8)) }
        let transport = URLSessionTransport(configuration: .init(session: StubURLProtocol.makeSession()))
        for baseString in ["https://wishlist.example/wishlist", "https://wishlist.example/wishlist/"] {
            let api = APIWishlistService(baseURL: try XCTUnwrap(URL(string: baseString)), token: token, transport: transport)
            _ = try await api.listItems()
        }
        XCTAssertEqual(
            StubURLProtocol.requestedURLs.map(\.absoluteString),
            ["https://wishlist.example/wishlist/api/items", "https://wishlist.example/wishlist/api/items"])
    }

    func testEstimatesMapsEmptySites() async throws {
        let transport = RecordingTransport(reply: .json(#"{"item_id":12,"sites":[],"refreshing":false}"#))
        let estimates = try await service(transport).estimates(itemID: 12)
        XCTAssertEqual(transport.last?.request.path, "/api/items/12/estimates")
        XCTAssertEqual(estimates.itemID, 12)
        XCTAssertEqual(estimates.sites, [])
        XCTAssertFalse(estimates.refreshing)
        XCTAssertNil(estimates.summaryLow)
    }

    func testListGenresKeepsSiteIDOrder() async throws {
        let transport = RecordingTransport(reply: .json(#"{"genres":[{"id":1,"name":"S.H.Figuarts","query_template":"S.H.Figuarts {name}","sort_order":1,"site_ids":[2,1,3]}]}"#))
        let genres = try await service(transport).listGenres()
        XCTAssertEqual(transport.last?.request.path, "/api/genres")
        XCTAssertEqual(genres, [Genre(id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", sortOrder: 1, siteIDs: [2, 1, 3])])
    }

    func testListSitesMapsFetchType() async throws {
        let transport = RecordingTransport(reply: .json(#"{"sites":[{"id":1,"name":"メルカリ","search_url_template":"https://jp.mercari.com/search?keyword={q}","fetch_type":"link_only","is_reference":false}]}"#))
        let sites = try await service(transport).listSites()
        XCTAssertEqual(sites, [Site(id: 1, name: "メルカリ", searchURLTemplate: "https://jp.mercari.com/search?keyword={q}", fetchType: .linkOnly, isReference: false)])
    }

    // MARK: - PATCH(省略 = 変えない / null = 消す)

    func testUpdateItemSendsOnlyChangedFields() async throws {
        let transport = RecordingTransport(reply: .json(itemJSON()))
        _ = try await service(transport).updateItem(id: 12, patch: ItemPatch(name: "グリス 改"))

        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .patch)
        XCTAssertEqual(sent.request.path, "/api/items/12")
        XCTAssertEqual(sent.request.headerFields[.authorization], "Bearer secret-token")
        XCTAssertEqual(sent.request.headerFields[.contentType], "application/json")
        XCTAssertEqual(sent.json, ["name": "グリス 改"] as NSDictionary)
    }

    /// 生成型 ItemUpdate の nullable は nil が省略されるため、そのままでは null を送れない。`.clear` は JSON の null で送る。
    func testUpdateItemClearSendsExplicitNull() async throws {
        let transport = RecordingTransport(reply: .json(itemJSON()))
        _ = try await service(transport).updateItem(id: 12, patch: ItemPatch(queryOverride: .clear))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["query_override": NSNull()] as NSDictionary)
    }

    func testUpdateItemAllNullableFieldsClearAndSet() async throws {
        let transport = RecordingTransport(reply: .json(itemJSON()))
        let api = service(transport)
        _ = try await api.updateItem(
            id: 12,
            patch: ItemPatch(optionText: .clear, queryOverride: .set("グリス 上書き"), sourceURL: .clear, minPrice: .clear))
        XCTAssertEqual(
            try XCTUnwrap(transport.last).json,
            ["option_text": NSNull(), "query_override": "グリス 上書き", "source_url": NSNull(), "min_price": NSNull()] as NSDictionary)

        _ = try await api.updateItem(id: 12, patch: ItemPatch(genreID: 2, minPrice: .set(1500), sortOrder: 3))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["genre_id": 2, "min_price": 1500, "sort_order": 3] as NSDictionary)
    }

    func testUpdateGenreSendsSiteIDsAsFullReplacement() async throws {
        let transport = RecordingTransport(reply: .json(#"{"id":2,"name":"S.H.Figuarts","query_template":"x","sort_order":0,"site_ids":[1,2]}"#))
        _ = try await service(transport).updateGenre(id: 2, patch: GenreUpdate(siteIDs: [1, 2]))
        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .patch)
        XCTAssertEqual(sent.request.path, "/api/genres/2")
        XCTAssertEqual(sent.json, ["site_ids": [1, 2]] as NSDictionary)
    }

    func testUpdateSiteSendsOnlyName() async throws {
        let transport = RecordingTransport(reply: .json(#"{"id":2,"name":"アマゾン","search_url_template":"https://a.example/s?k={q}","fetch_type":"link_only","is_reference":false}"#))
        _ = try await service(transport).updateSite(id: 2, patch: SiteUpdate(name: "アマゾン"))
        XCTAssertEqual(transport.last?.request.path, "/api/sites/2")
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["name": "アマゾン"] as NSDictionary)
    }

    // MARK: - 登録

    func testCreateItemFromImageURLSendsJSON() async throws {
        let transport = RecordingTransport(reply: .json(201, itemJSON()))
        let fields = ItemFields(genreID: 1, name: "直した名前", sourceURL: "https://shop.example/p/1")
        let item = try await service(transport).createItem(fields, imageURL: "https://img.example/og.png")

        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .post)
        XCTAssertEqual(sent.request.path, "/api/items")
        XCTAssertEqual(sent.request.headerFields[.contentType], "application/json")
        XCTAssertEqual(
            sent.json,
            ["genre_id": 1, "name": "直した名前", "image_url": "https://img.example/og.png", "source_url": "https://shop.example/p/1"] as NSDictionary)
        XCTAssertEqual(item.id, 12)
    }

    func testCreateItemWithPhotoSendsMultipart() async throws {
        let transport = RecordingTransport(reply: .json(201, itemJSON()))
        let image = ImageUpload(data: Data("PNGDATA".utf8), filename: "photo.png", contentType: "image/png")
        _ = try await service(transport).createItem(ItemFields(genreID: 2, name: "新商品"), image: image)

        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .post)
        XCTAssertEqual(sent.request.path, "/api/items")
        let contentType = try XCTUnwrap(sent.request.headerFields[.contentType])
        XCTAssertTrue(contentType.hasPrefix("multipart/form-data; boundary="), contentType)
        let text = sent.bodyText
        XCTAssertTrue(text.contains(#"name="genre_id""#), text)
        XCTAssertTrue(text.contains(#"name="name""#), text)
        XCTAssertTrue(text.contains("新商品"), text)
        XCTAssertTrue(text.contains(#"name="image""#), text)
        XCTAssertTrue(text.contains(#"filename="photo.png""#), text)
        XCTAssertTrue(text.contains("PNGDATA"), text)
        XCTAssertFalse(text.contains(#"name="option_text""#), "nil の項目は送らない")
    }

    func testReplaceItemImageSendsMultipartPut() async throws {
        let transport = RecordingTransport(reply: .json(itemJSON()))
        let image = ImageUpload(data: Data("NEW".utf8), filename: "new.png", contentType: "image/png")
        _ = try await service(transport).replaceItemImage(id: 12, image: image)
        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .put)
        XCTAssertEqual(sent.request.path, "/api/items/12/image")
        XCTAssertTrue(sent.bodyText.contains(#"filename="new.png""#), sent.bodyText)
    }

    func testDraftFromURLSendsURLAndGenre() async throws {
        let transport = RecordingTransport(reply: .json(#"{"name":"OGP のタイトル","image_url":null,"source_url":"https://shop.example/p/1","genre_id":1}"#))
        let draft = try await service(transport).draftFromURL("https://shop.example/p/1", genreID: 1)
        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .post)
        XCTAssertEqual(sent.request.path, "/api/items/from-url")
        XCTAssertEqual(sent.json, ["url": "https://shop.example/p/1", "genre_id": 1] as NSDictionary)
        XCTAssertEqual(draft, ItemDraft(name: "OGP のタイトル", imageURL: nil, sourceURL: "https://shop.example/p/1", genreID: 1))

        _ = try await service(transport).draftFromURL("https://shop.example/p/2", genreID: nil)
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["url": "https://shop.example/p/2"] as NSDictionary)
    }

    func testCreateSiteSendsExplicitFields() async throws {
        let transport = RecordingTransport(reply: .json(201, #"{"id":4,"name":"新サイト","search_url_template":"https://new.example/s?q={q}","fetch_type":"scrape","is_reference":false}"#))
        let site = try await service(transport).createSite(
            SiteCreate(name: "新サイト", searchURLTemplate: "https://new.example/s?q={q}", fetchType: .scrape, isReference: false))
        XCTAssertEqual(
            try XCTUnwrap(transport.last).json,
            ["name": "新サイト", "search_url_template": "https://new.example/s?q={q}", "fetch_type": "scrape", "is_reference": false] as NSDictionary)
        XCTAssertEqual(site.fetchType, .scrape)
    }

    func testCreateGenreOmitsNilFields() async throws {
        let transport = RecordingTransport(reply: .json(201, #"{"id":5,"name":"新ジャンル","query_template":"{name} {option}","sort_order":0,"site_ids":[]}"#))
        _ = try await service(transport).createGenre(GenreCreate(name: "新ジャンル"))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["name": "新ジャンル"] as NSDictionary)
    }

    func testDeleteItemAcceptsNoContent() async throws {
        let transport = RecordingTransport(reply: .noContent)
        try await service(transport).deleteItem(id: 12)
        XCTAssertEqual(transport.last?.request.method, .delete)
        XCTAssertEqual(transport.last?.request.path, "/api/items/12")
    }

    // MARK: - エラーの写像

    private func thrownError(_ transport: RecordingTransport) async -> WishlistError? {
        do {
            _ = try await service(transport).listItems()
            return nil
        } catch let error as WishlistError {
            return error
        } catch {
            XCTFail("WishlistError 以外が投げられた: \(error)")
            return nil
        }
    }

    func testErrorBodyMapsCodeStatusAndMessage() async {
        let cases: [(Int, String, WishlistError.Code)] = [
            (400, "bad_request", .badRequest), (401, "unauthorized", .unauthorized), (404, "not_found", .notFound),
            (422, "unprocessable", .unprocessable), (502, "bad_gateway", .badGateway), (500, "internal", .internal),
        ]
        for (status, code, want) in cases {
            let error = await thrownError(RecordingTransport(reply: .json(status, #"{"code":"\#(code)","message":"msg-\#(code)"}"#)))
            XCTAssertEqual(error, WishlistError(code: want, status: status, message: "msg-\(code)"), "status \(status)")
        }
    }

    /// JSON でない応答(プロキシのエラーページなど)はステータスから決める。
    func testNonJSONErrorMapsFromStatus() async {
        let unauthorized = await thrownError(RecordingTransport(reply: .text(401, "<html>nope</html>")))
        XCTAssertEqual(unauthorized?.code, .unauthorized)
        XCTAssertEqual(unauthorized?.status, 401)
        let gateway = await thrownError(RecordingTransport(reply: .text(502, "bad gateway")))
        XCTAssertEqual(gateway?.code, .badGateway)
        let server = await thrownError(RecordingTransport(reply: .text(500, "oops")))
        XCTAssertEqual(server?.code, .internal)
        let unknown = await thrownError(RecordingTransport(reply: .text(503, "unavailable")))
        XCTAssertEqual(unknown?.code, .internal)
        XCTAssertEqual(unknown?.status, 503)
    }

    /// 通信できなかった(オフライン・タイムアウトなど)は `.network`(status 0)。ViewModel が「オフライン」の判定に使う。
    func testTransportFailureMapsToNetworkError() async {
        let error = await thrownError(RecordingTransport { _ in throw URLError(.notConnectedToInternet) })
        XCTAssertEqual(error?.code, .network)
        XCTAssertEqual(error?.status, 0)
        XCTAssertEqual(error?.isNetwork, true)
    }

    /// キャンセルは通信失敗(.network)にしない。
    func testCancellationIsNotMappedToNetworkError() async {
        let api = service(RecordingTransport { _ in throw CancellationError() })
        do {
            _ = try await api.listItems()
            XCTFail("投げられるはず")
        } catch {
            XCTAssertNil(error as? WishlistError, "\(error)")
        }
        let urlCancelled = service(RecordingTransport { _ in throw URLError(.cancelled) })
        do {
            _ = try await urlCancelled.listItems()
            XCTFail("投げられるはず")
        } catch {
            XCTAssertNotEqual((error as? WishlistError)?.code, .network)
        }
    }
}
