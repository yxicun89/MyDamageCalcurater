import HTTPTypes
import OpenAPIRuntime
import XCTest

@testable import WishlistCore

// AC-IOS-OFF-API-01・AC-IOS-ALI-API-01: APIWishlistService の公式の販売状況・監視の切り替え・表記揺れの辞書(docs/phase4-spec.md)。
final class APIWishlistOfficialTests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!

    private func service(_ transport: RecordingTransport) -> APIWishlistService {
        APIWishlistService(baseURL: base, token: "secret-token", transport: transport)
    }

    private func itemJSON(_ extra: String) -> String {
        """
        {"id":12,"genre_id":1,"name":"グリス","option_text":null,"query_override":null,
         "image_url":"images/00000000-0000-4000-8000-000000000012.png","source_url":"https://tamashii.example/item/1/","min_price":null,
         "sort_order":0,"site_overrides":[],"created_at":"2026-10-03T00:00:00Z","updated_at":"2026-10-03T00:00:00Z"\(extra)}
        """
    }

    func testItemMapsWatchOfficialAndOfficialStatus() async throws {
        let status = #"""
            ,"watch_official":true,"official_status":{"status":"ended","evidence":["予約受付終了"],
             "checked_at":"2026-10-03T19:00:00Z","changed_at":"2026-10-03T19:00:00Z","previous_status":"preorder",
             "last_result":"failed","last_attempt_at":"2026-10-04T18:00:00Z"}
            """#
        let transport = RecordingTransport(reply: .json(#"{"items":[\#(itemJSON(status))]}"#))
        let items = try await service(transport).listItems()
        let item = try XCTUnwrap(items.first)
        XCTAssertTrue(item.watchOfficial)
        XCTAssertEqual(
            item.officialStatus,
            OfficialStatus(
                status: .ended, evidence: ["予約受付終了"], checkedAt: iso("2026-10-03T19:00:00Z"), changedAt: iso("2026-10-03T19:00:00Z"),
                previousStatus: .preorder, lastResult: .failed, lastAttemptAt: iso("2026-10-04T18:00:00Z")))
    }

    func testItemWithNullOrMissingOfficialFields() async throws {
        let transport = RecordingTransport(reply: .json(#"{"items":[\#(itemJSON(#","watch_official":false,"official_status":null"#)),\#(itemJSON(""))]}"#))
        let items = try await service(transport).listItems()
        XCTAssertEqual(items.count, 2)
        for item in items {
            XCTAssertFalse(item.watchOfficial)
            XCTAssertNil(item.officialStatus)
        }
    }

    func testUpdateItemSendsWatchOfficialOnlyWhenSet() async throws {
        let transport = RecordingTransport(reply: .json(itemJSON(#","watch_official":false,"official_status":null"#)))
        _ = try await service(transport).updateItem(id: 12, patch: ItemPatch(watchOfficial: false))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["watch_official": false] as NSDictionary)
        _ = try await service(transport).updateItem(id: 12, patch: ItemPatch(name: "グリス改"))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["name": "グリス改"] as NSDictionary)
    }

    func testGenreAliasesAreMappedAndMissingMeansEmpty() async throws {
        let transport = RecordingTransport(
            reply: .json(
                #"{"genres":[{"id":1,"name":"S.H.Figuarts","query_template":"S.H.Figuarts {name}","sort_order":1,"site_ids":[2,1],"aliases":[["S.H.Figuarts","SHフィギュアーツ"]]},{"id":2,"name":"デュエマ","query_template":"{name} {option}","sort_order":2,"site_ids":[1]}]}"#
            ))
        let genres = try await service(transport).listGenres()
        XCTAssertEqual(genres.map(\.aliases), [[["S.H.Figuarts", "SHフィギュアーツ"]], []])
    }

    func testCreateAndUpdateGenreSendAliases() async throws {
        let reply = #"{"id":3,"name":"ガンプラ","query_template":"{name}","sort_order":0,"site_ids":[],"aliases":[["HG","ハイグレード"]]}"#
        let transport = RecordingTransport { sent in sent.request.method == .post ? .json(201, reply) : .json(reply) }
        let created = try await service(transport).createGenre(GenreCreate(name: "ガンプラ", queryTemplate: "{name}", aliases: [["HG", "ハイグレード"]]))
        XCTAssertEqual(created.aliases, [["HG", "ハイグレード"]])
        let sent = try XCTUnwrap(transport.last).json
        XCTAssertEqual(sent["aliases"] as? [[String]], [["HG", "ハイグレード"]])

        _ = try await service(transport).updateGenre(id: 3, patch: GenreUpdate(aliases: []))
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["aliases": []] as NSDictionary, "空なら [] を送る(全部消す)")
        _ = try await service(transport).updateGenre(id: 3, patch: GenreUpdate(name: "ガンプラ2"))
        XCTAssertNil(try XCTUnwrap(transport.last).json["aliases"], "nil なら送らない")
    }
}
