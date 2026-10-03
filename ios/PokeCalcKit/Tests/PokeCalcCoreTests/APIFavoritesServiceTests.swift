import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// お気に入り API(`listFavorites` / `createFavorite` / `deleteFavorite`。ADR-0227・ADR-0509)の写像。
/// パス・ヘッダー・本文・ステータス(200/201/204/400/404/503)・順序保持・通信失敗を契約どおりに確かめる。
final class APIFavoritesServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static func favoriteJSON(
        id: String, label: String?, key: String = "9002-000", updatedAt: String = "2026-10-02T00:00:00Z"
    ) -> String {
        let labelJSON = label.map { "\"\($0)\"" } ?? "null"
        return """
            {"id":"\(id)","label":\(labelJSON),"individual":{"speciesKey":"\(key)","level":50,
             "natureId":"test-nature-neutral","abilityId":"test-ability-1",
             "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0},
             "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
             "createdAt":"2026-10-01T00:00:00Z","updatedAt":"\(updatedAt)"}
            """
    }

    private static let individual = Individual(
        speciesKey: "9002-000", natureId: "test-nature-neutral",
        sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0), abilityId: "test-ability-1", itemId: "test-item-1")

    // MARK: - 一覧

    func testListSendsGetWithoutQueryAndWithIdentityHeaders() async throws {
        let transport = RecordingTransport(json: "[]")
        _ = try await makeService(transport: transport).favorites()
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/record/favorites")
        XCTAssertTrue(sent.queryItems.isEmpty, "ページングは無い")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
    }

    func testListMapsAllFieldsKeepingServerOrder() async throws {
        let json = """
            [\(Self.favoriteJSON(id: "42", label: "HB特化", updatedAt: "2026-10-03T00:00:00Z")),
             \(Self.favoriteJSON(id: "7", label: nil, key: "9003-000", updatedAt: "2026-10-02T00:00:00Z")),
             \(Self.favoriteJSON(id: "100", label: nil, key: "9001-000", updatedAt: "2026-10-02T00:00:00Z"))]
            """
        let list = try await makeService(transport: RecordingTransport(json: json)).favorites()
        XCTAssertEqual(list.map(\.id), ["42", "7", "100"], "サーバーの順(updatedAt 降順・同時刻は id 降順)を並べ替えない")
        XCTAssertEqual(list.map(\.label), ["HB特化", nil, nil])
        guard list.count == 3 else { return XCTFail("3件のはず: \(list.count)") }
        XCTAssertEqual(list[0].individual.speciesKey, "9002-000")
        XCTAssertEqual(list[0].individual.natureId, "test-nature-neutral")
        XCTAssertEqual(list[0].individual.abilityId, "test-ability-1")
        XCTAssertNil(list[0].individual.itemId)
        XCTAssertNil(list[0].individual.moveId, "契約の Individual に moveId は無い")
        XCTAssertEqual(list[0].individual.level, 50)
        XCTAssertEqual(list[0].individual.sp, StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0))
        XCTAssertEqual(list[0].createdAt, ISO8601DateFormatter().date(from: "2026-10-01T00:00:00Z"))
        XCTAssertEqual(list[0].updatedAt, ISO8601DateFormatter().date(from: "2026-10-03T00:00:00Z"))
    }

    func testListEmptyArrayIsSuccessNotNotFound() async throws {
        let list = try await makeService(transport: RecordingTransport(json: "[]")).favorites()
        XCTAssertEqual(list, [])
    }

    func testListServiceUnavailableMapsToPokeCalcErrorKeepingCode() async throws {
        for code in ["store_unavailable", "upstream_unavailable"] {
            let json = #"{"code":"\#(code)","message":"test error"}"#
            let service = try makeService(transport: RecordingTransport(status: 503, json: json))
            let error = await assertThrowsPokeCalcError(code) { try await service.favorites() }
            XCTAssertEqual(error?.code, code)
        }
    }

    func testListBadRequestAndTransportFailure() async throws {
        let bad = try makeService(
            transport: RecordingTransport(status: 400, json: #"{"code":"invalid_input","message":"x"}"#))
        let badError = await assertThrowsPokeCalcError("400") { try await bad.favorites() }
        XCTAssertEqual(badError?.code, "invalid_input")

        let failing = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") { try await failing.favorites() }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    // MARK: - 作成

    func testCreateSendsPostWithIndividualBodyAndIdentityHeaders() async throws {
        let transport = RecordingTransport(status: 201, json: Self.favoriteJSON(id: "42", label: "HB特化"))
        _ = try await makeService(transport: transport).addFavorite(label: "HB特化", individual: Self.individual)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/record/favorites")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        let body = try sent.jsonBody()
        XCTAssertEqual(Set(body.keys), ["label", "individual"], "id / createdAt / updatedAt はサーバーが決める(送ると 400 unknown_field)")
        XCTAssertEqual(body["label"] as? String, "HB特化")
        let individual = try XCTUnwrap(body["individual"] as? [String: Any])
        XCTAssertEqual(individual["speciesKey"] as? String, "9002-000")
        XCTAssertEqual(individual["natureId"] as? String, "test-nature-neutral")
        XCTAssertEqual(individual["abilityId"] as? String, "test-ability-1")
        XCTAssertEqual(individual["itemId"] as? String, "test-item-1")
        XCTAssertEqual(individual["level"] as? Int, 50)
        XCTAssertNil(individual["moveId"], "契約の Individual に moveId は無い(unknown_field になる)")
        let sp = try XCTUnwrap(individual["sp"] as? [String: Int])
        XCTAssertEqual(sp, ["hp": 32, "atk": 0, "def": 32, "spa": 0, "spd": 2, "spe": 0])
    }

    func testCreateNormalizesLabelBeforeSending() async throws {
        let long = String(repeating: "あ", count: RequestLimits.maxFavoriteLabelLength + 5)
        let cases: [(input: String?, sent: String?)] = [
            (nil, nil), ("", nil), ("  \n ", nil), ("  HB特化 ", "HB特化"),
            (long, String(repeating: "あ", count: RequestLimits.maxFavoriteLabelLength)),
        ]
        for testCase in cases {
            let transport = RecordingTransport(status: 201, json: Self.favoriteJSON(id: "1", label: nil))
            _ = try await makeService(transport: transport).addFavorite(label: testCase.input, individual: Self.individual)
            let body = try XCTUnwrap(transport.requests.first).jsonBody()
            let value = body["label"]
            if let expected = testCase.sent {
                XCTAssertEqual(value as? String, expected, "入力 \(String(describing: testCase.input))")
            } else {
                XCTAssertTrue(value == nil || value is NSNull, "未設定は送らない(または null): \(String(describing: value))")
            }
        }
    }

    func testCreate201MapsToCreatedAnd200ToAlreadyPinned() async throws {
        let created = try await makeService(
            transport: RecordingTransport(status: 201, json: Self.favoriteJSON(id: "42", label: nil))
        ).addFavorite(label: nil, individual: Self.individual)
        guard case .created(let favorite) = created else { return XCTFail("201 は .created: \(created)") }
        XCTAssertEqual(favorite.id, "42")

        let existing = try await makeService(
            transport: RecordingTransport(status: 200, json: Self.favoriteJSON(id: "41", label: nil))
        ).addFavorite(label: nil, individual: Self.individual)
        guard case .alreadyPinned(let pinned) = existing else { return XCTFail("200 は .alreadyPinned: \(existing)") }
        XCTAssertEqual(pinned.id, "41")
    }

    func testCreateBadRequestKeepsCode() async throws {
        for code in ["invalid_input", "unknown_field"] {
            let service = try makeService(
                transport: RecordingTransport(status: 400, json: #"{"code":"\#(code)","message":"test error"}"#))
            let error = await assertThrowsPokeCalcError(code) {
                try await service.addFavorite(label: nil, individual: Self.individual)
            }
            XCTAssertEqual(error?.code, code, "上限到達も 400 invalid_input(クライアントで握りつぶさない)")
        }
    }

    func testCreateServiceUnavailableAndInternalErrorAndTransport() async throws {
        for code in ["store_unavailable", "upstream_unavailable"] {
            let service = try makeService(
                transport: RecordingTransport(status: 503, json: #"{"code":"\#(code)","message":"test error"}"#))
            let error = await assertThrowsPokeCalcError(code) {
                try await service.addFavorite(label: nil, individual: Self.individual)
            }
            XCTAssertEqual(error?.code, code)
        }
        let internalError = try makeService(
            transport: RecordingTransport(status: 500, json: #"{"code":"internal","message":"test error"}"#))
        let error500 = await assertThrowsPokeCalcError("500") {
            try await internalError.addFavorite(label: nil, individual: Self.individual)
        }
        XCTAssertEqual(error500?.code, "internal")

        let failing = try makeService(transport: FailingTransport())
        let transportError = await assertThrowsPokeCalcError("transport") {
            try await failing.addFavorite(label: nil, individual: Self.individual)
        }
        XCTAssertEqual(transportError?.code, PokeCalcError.Code.transport)
    }

    // MARK: - 削除

    func testDeleteSendsDeleteWithIdInPathAndIdentityHeaders() async throws {
        let transport = RecordingTransport(status: 204, json: nil)
        try await makeService(transport: transport).removeFavorite(id: "42")
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "DELETE")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/record/favorites/42")
        XCTAssertNil(sent.body, "本文なし")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        XCTAssertTrue(sent.queryItems.isEmpty, "端末 ID はパスにもクエリにも含めない(ヘッダだけが正)")
    }

    func testDelete204Succeeds() async throws {
        try await makeService(transport: RecordingTransport(status: 204, json: nil)).removeFavorite(id: "42")
    }

    func testDeleteNotFoundKeepsCode() async throws {
        let service = try makeService(
            transport: RecordingTransport(status: 404, json: #"{"code":"not_found","message":"test error"}"#))
        let error = await assertThrowsPokeCalcError("404") { try await service.removeFavorite(id: "42") }
        XCTAssertEqual(error?.code, "not_found")
    }

    func testDeleteBadRequestServiceUnavailableAndTransport() async throws {
        let bad = try makeService(
            transport: RecordingTransport(status: 400, json: #"{"code":"invalid_input","message":"x"}"#))
        let badError = await assertThrowsPokeCalcError("400") { try await bad.removeFavorite(id: "42") }
        XCTAssertEqual(badError?.code, "invalid_input")

        for code in ["store_unavailable", "upstream_unavailable"] {
            let service = try makeService(
                transport: RecordingTransport(status: 503, json: #"{"code":"\#(code)","message":"test error"}"#))
            let error = await assertThrowsPokeCalcError(code) { try await service.removeFavorite(id: "42") }
            XCTAssertEqual(error?.code, code)
        }

        let failing = try makeService(transport: FailingTransport())
        let transportError = await assertThrowsPokeCalcError("transport") { try await failing.removeFavorite(id: "42") }
        XCTAssertEqual(transportError?.code, PokeCalcError.Code.transport)
    }
}
