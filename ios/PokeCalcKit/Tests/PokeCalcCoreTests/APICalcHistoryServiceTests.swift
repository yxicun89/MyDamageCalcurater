import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// 計算履歴 API(`listCalcHistory`。ADR-0230・ADR-0519)の写像。パス・クエリ・ヘッダー・ステータス(200/400/503)・
/// 順序保持・`nextCursor`・既定値の補完・通信失敗を契約どおりに確かめる。
final class APICalcHistoryServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static let fullEntry = """
        {"occurredAt":"2026-10-09T01:00:00Z",
         "calc":{"format":"double",
           "attacker":{"speciesKey":"9002-000","level":50,"natureId":"test-nature-spa-up","abilityId":"test-ability-beta",
             "itemId":"test-item-berry","sp":{"hp":0,"atk":0,"def":0,"spa":32,"spd":2,"spe":32},
             "ranks":{"atk":0,"def":0,"spa":2,"spd":0,"spe":0},"status":"burn"},
           "defender":{"speciesKey":"9003-000","level":50,"natureId":"test-nature-neutral",
             "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0}},
           "moveId":"test-move-special-b",
           "field":{"weather":"rain","terrain":"grassy",
             "attackerScreens":{"reflect":true,"lightScreen":false,"auroraVeil":false},
             "defenderScreens":{"reflect":false,"lightScreen":true,"auroraVeil":true}},
           "options":{"critical":true}},
         "result":{"minPercent":41.2,"maxPercent":137.0}}
        """

    private static let minimalEntry = """
        {"occurredAt":"2026-10-08T01:00:00Z",
         "calc":{"format":"single",
           "attacker":{"speciesKey":"9003-000","level":50,"natureId":"test-nature-neutral",
             "sp":{"hp":0,"atk":0,"def":0,"spa":32,"spd":2,"spe":32}},
           "defender":{"speciesKey":"9002-000","level":50,"natureId":"test-nature-neutral",
             "sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":32,"spe":0}},
           "moveId":"test-move-physical-a"},
         "result":{"minPercent":10.0,"maxPercent":12.5}}
        """

    func testSendsGetWithLimitAndHeadersAndNoCursorForFirstPage() async throws {
        let transport = RecordingTransport(json: #"{"items":[],"nextCursor":null}"#)
        _ = try await makeService(transport: transport).calcHistory(limit: 20, cursor: nil)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/record/calc-history")
        XCTAssertEqual(sent.queryItems["limit"], "20")
        XCTAssertNil(sent.queryItems["cursor"], "先頭ページに cursor を付けない")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
    }

    func testPassesCursorUnchanged() async throws {
        let cursor = "MTcyODQzNTYwMDAwMDAwMDAwMHxj_-"
        let transport = RecordingTransport(json: #"{"items":[],"nextCursor":null}"#)
        _ = try await makeService(transport: transport).calcHistory(limit: 20, cursor: cursor)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.queryItems["cursor"], cursor)
        XCTAssertEqual(sent.queryItems["limit"], "20")
    }

    func testMapsEveryFieldKeepingServerOrderAndNextCursor() async throws {
        let json = #"{"items":[\#(Self.fullEntry),\#(Self.minimalEntry)],"nextCursor":"abc_-1"}"#
        let page = try await makeService(transport: RecordingTransport(json: json)).calcHistory(limit: 20, cursor: nil)
        XCTAssertEqual(page.nextCursor, "abc_-1")
        XCTAssertEqual(page.items.count, 2)
        let full = page.items[0]
        XCTAssertEqual(full.occurredAt, ISO8601DateFormatter().date(from: "2026-10-09T01:00:00Z"))
        XCTAssertEqual(full.calc.format, .double)
        XCTAssertEqual(full.calc.moveId, "test-move-special-b")
        XCTAssertEqual(full.calc.attacker.speciesKey, "9002-000")
        XCTAssertEqual(full.calc.attacker.natureId, "test-nature-spa-up")
        XCTAssertEqual(full.calc.attacker.abilityId, "test-ability-beta")
        XCTAssertEqual(full.calc.attacker.itemId, "test-item-berry")
        XCTAssertEqual(full.calc.attacker.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 32, spd: 2, spe: 32))
        XCTAssertEqual(full.calc.attacker.ranks, RankBlock(spa: 2))
        XCTAssertEqual(full.calc.attacker.status, .burn)
        XCTAssertEqual(full.calc.defender.speciesKey, "9003-000")
        XCTAssertEqual(full.calc.field.weather, .rain)
        XCTAssertEqual(full.calc.field.terrain, .grassy)
        XCTAssertEqual(full.calc.field.attackerScreens, Screens(reflect: true))
        XCTAssertEqual(full.calc.field.defenderScreens, Screens(lightScreen: true, auroraVeil: true))
        XCTAssertTrue(full.calc.critical)
        XCTAssertEqual(full.minPercent, 41.2)
        XCTAssertEqual(full.maxPercent, 137.0, "100% を超える値も切らない")
        XCTAssertGreaterThan(full.occurredAt, page.items[1].occurredAt, "サーバーの順(新しい順)を並べ替えない")
    }

    func testOmittedFieldAndOptionsBecomeDefaults() async throws {
        let json = #"{"items":[\#(Self.minimalEntry)],"nextCursor":null}"#
        let page = try await makeService(transport: RecordingTransport(json: json)).calcHistory(limit: 20, cursor: nil)
        let calc = try XCTUnwrap(page.items.first).calc
        XCTAssertEqual(calc.field, FieldState())
        XCTAssertFalse(calc.critical)
        XCTAssertEqual(calc.attacker.status, .none)
        XCTAssertEqual(calc.attacker.ranks, RankBlock())
        XCTAssertNil(calc.attacker.abilityId)
        XCTAssertNil(page.nextCursor)
    }

    func testEmptyPageIsSuccess() async throws {
        let page = try await makeService(transport: RecordingTransport(json: #"{"items":[],"nextCursor":null}"#))
            .calcHistory(limit: 20, cursor: nil)
        XCTAssertEqual(page, CalcHistoryPage(items: [], nextCursor: nil))
    }

    func testServiceUnavailableMapsToPokeCalcErrorKeepingCode() async throws {
        for code in ["store_unavailable", "upstream_unavailable"] {
            let json = #"{"code":"\#(code)","message":"test error"}"#
            let service = try makeService(transport: RecordingTransport(status: 503, json: json))
            let error = await assertThrowsPokeCalcError(code) { try await service.calcHistory(limit: 20, cursor: nil) }
            XCTAssertEqual(error?.code, code)
            XCTAssertEqual(error.map(RecordScreenError.init), .storeUnavailable)
        }
    }

    func testBadRequestMapsToInvalidInput() async throws {
        let service = try makeService(
            transport: RecordingTransport(status: 400, json: #"{"code":"invalid_input","message":"x"}"#))
        let error = await assertThrowsPokeCalcError("400") { try await service.calcHistory(limit: 20, cursor: "bad") }
        XCTAssertEqual(error?.code, "invalid_input")
    }

    func testTransportFailureMapsToTransportError() async throws {
        let transport = RecordingTransport { _ in throw URLError(.notConnectedToInternet) }
        let service = try makeService(transport: transport)
        let error = await assertThrowsPokeCalcError("transport") { try await service.calcHistory(limit: 20, cursor: nil) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }
}
