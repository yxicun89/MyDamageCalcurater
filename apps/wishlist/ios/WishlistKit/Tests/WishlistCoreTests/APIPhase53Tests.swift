import HTTPTypes
import OpenAPIRuntime
import XCTest

@testable import WishlistCore

// AC-IOS-5-3h: API の写像(5-1 の追加項目)と from-url の送信内容。実ネットワークは使わず RecordingTransport で見る。
final class APIPhase53Tests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!

    private func service(_ transport: RecordingTransport) -> APIWishlistService {
        APIWishlistService(baseURL: base, transport: transport)
    }

    /// AC-IOS-5-3h-1: from-url に url・title・image_url を送る。genre_id は送らない。Authorization は付けない
    func testDraftFromPageSendsURLTitleAndImageURL() async throws {
        let transport = RecordingTransport(reply: .json(
            #"{"name":"グリス","image_url":"https://img.example/og.png","source_url":"https://shop.example/p/1","genre_id":1,"genre_guessed":true}"#))
        let draft = try await service(transport).draftFromPage(
            url: "https://shop.example/p/1", title: "グリス | ショップ", imageURL: "https://img.example/og.png")
        let sent = try XCTUnwrap(transport.last)
        XCTAssertEqual(sent.request.method, .post)
        XCTAssertEqual(sent.request.path, "/api/items/from-url")
        XCTAssertNil(sent.request.headerFields[.authorization])
        XCTAssertEqual(
            sent.json,
            ["url": "https://shop.example/p/1", "title": "グリス | ショップ", "image_url": "https://img.example/og.png"] as NSDictionary)
        XCTAssertEqual(
            draft,
            ItemDraft(
                name: "グリス", imageURL: "https://img.example/og.png", sourceURL: "https://shop.example/p/1", genreID: 1, genreGuessed: true))
    }

    /// AC-IOS-5-3h-2: title・image_url が無ければ送らない(url だけ)
    func testDraftFromPageOmitsMissingFields() async throws {
        let transport = RecordingTransport(reply: .json(#"{"name":"x","source_url":"https://shop.example/p/1","genre_id":null,"genre_guessed":false}"#))
        let draft = try await service(transport).draftFromPage(url: "https://shop.example/p/1", title: nil, imageURL: nil)
        XCTAssertEqual(try XCTUnwrap(transport.last).json, ["url": "https://shop.example/p/1"] as NSDictionary)
        XCTAssertNil(draft.genreID)
        XCTAssertFalse(draft.genreGuessed)
    }

    /// AC-IOS-5-3h-3: from-url（旧）の応答の genre_guessed も写す
    func testDraftFromURLMapsGenreGuessed() async throws {
        let transport = RecordingTransport(reply: .json(#"{"name":"x","image_url":null,"source_url":"https://shop.example/p/1","genre_id":2,"genre_guessed":true}"#))
        let draft = try await service(transport).draftFromURL("https://shop.example/p/1", genreID: nil)
        XCTAssertEqual(draft.genreID, 2)
        XCTAssertTrue(draft.genreGuessed)
    }

    /// AC-IOS-5-3h-4: ジャンルの color・icon・match_hosts・match_keywords を写す
    func testListGenresMapsColorIconAndMatchClues() async throws {
        let transport = RecordingTransport(reply: .json(
            ##"{"genres":[{"id":1,"name":"デュエマ","query_template":"{name} {option}","sort_order":1,"site_ids":[1],"color":"#4f46e5","icon":"card","match_hosts":["dm.takaratomy.co.jp"],"match_keywords":["デュエマ","DM"]}]}"##))
        let genres = try await service(transport).listGenres()
        let genre = try XCTUnwrap(genres.first)
        XCTAssertEqual(genre.color, "#4f46e5")
        XCTAssertEqual(genre.icon, .card)
        XCTAssertEqual(genre.matchHosts, ["dm.takaratomy.co.jp"])
        XCTAssertEqual(genre.matchKeywords, ["デュエマ", "DM"])
    }

    /// AC-IOS-5-3h-5: 商品の summary（ホームの価格の札）を写す。無ければ nil
    func testListItemsMapsSummary() async throws {
        let item = { (id: Int, summary: String) in
            """
            {"id":\(id),"genre_id":1,"name":"グリス","image_url":"images/a.png","sort_order":0,"site_overrides":[],
             "created_at":"2026-10-03T00:00:00Z","updated_at":"2026-10-03T00:00:00Z"\(summary)}
            """
        }
        let transport = RecordingTransport(reply: .json(
            #"{"items":[\#(item(1, #","summary":{"low":880,"status":"ok","fetched_at":"2026-10-03T00:00:00Z"}"#)),\#(item(2, #","summary":{"low":null,"status":"no_result","fetched_at":null}"#)),\#(item(3, ""))]}"#))
        let items = try await service(transport).listItems()
        XCTAssertEqual(items[0].summary, ItemSummary(low: 880, status: .ok, fetchedAt: Date(timeIntervalSince1970: 1_790_985_600)))
        XCTAssertEqual(items[1].summary, ItemSummary(low: nil, status: .noResult, fetchedAt: nil))
        XCTAssertNil(items[2].summary)
    }

    /// AC-IOS-5-3h-6: ジャンルの追加は color・icon・match_hosts・match_keywords を送る
    func testCreateGenreSendsColorIconAndClues() async throws {
        let transport = RecordingTransport(reply: .json(
            201, ##"{"id":5,"name":"ガンプラ","query_template":"{name}","sort_order":0,"site_ids":[],"color":"#16a34a","icon":"robot","match_hosts":["a.example"],"match_keywords":["HG"]}"##))
        _ = try await service(transport).createGenre(
            GenreCreate(name: "ガンプラ", color: "#16a34a", icon: .robot, matchHosts: ["a.example"], matchKeywords: ["HG"]))
        XCTAssertEqual(
            try XCTUnwrap(transport.last).json,
            ["name": "ガンプラ", "color": "#16a34a", "icon": "robot", "match_hosts": ["a.example"], "match_keywords": ["HG"]] as NSDictionary)
    }

    /// AC-IOS-5-3h-7: ジャンルの更新は変えた項目だけ。手がかりを空にするときは [] を送る
    func testUpdateGenreSendsOnlyChangedClues() async throws {
        let transport = RecordingTransport(reply: .json(
            ##"{"id":1,"name":"x","query_template":"x","sort_order":0,"site_ids":[],"color":"#e0393e","icon":"star","match_hosts":[],"match_keywords":[]}"##))
        _ = try await service(transport).updateGenre(id: 1, patch: GenreUpdate(color: "#e0393e", icon: .star, matchHosts: []))
        XCTAssertEqual(
            try XCTUnwrap(transport.last).json, ["color": "#e0393e", "icon": "star", "match_hosts": [] as [String]] as NSDictionary)
    }
}
