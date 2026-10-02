import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcSpeedAPI
import XCTest

@testable import PokeCalcCore

/// `APISpeedService`(P6-24。ADR-0503 §3): 生成された `Client`(PokeCalcSpeedAPI)と偽の transport で、
/// ドメイン ↔ HTTP(services/speed/api/openapi.yaml)の写像を検証する。期待値は契約のパス・クエリ名・スキーマから書く。
/// フィクスチャは架空(名前は「テスト」で始める)。
final class APISpeedServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APISpeedService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APISpeedService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - 架空のフィクスチャ(契約の例と同じ形)

    private static let pokemonJSON = #"{"pokemonId":"9001-000","nameJa":"テストカソウドリ","types":["fire","flying"],"baseSpeed":100}"#
    private static func entryJSON(_ pokemonID: String, _ preset: String) -> String {
        #"{"pokemonId":"\#(pokemonID)","nameJa":"テストカソウドリ","types":["fire"],"baseSpeed":100,"preset":"\#(preset)"}"#
    }

    private static let pokemonListJSON = #"{"regulationId":"example","pokemon":[\#(pokemonJSON)]}"#

    private static var tableJSON: String {
        """
        {"regulationId":"example","presets":["max","max-scarf"],"tiers":[
          {"speed":300,"entries":[\(entryJSON("9001-000", "max-scarf")),\(entryJSON("9003-000", "max"))]},
          {"speed":200,"entries":[\(entryJSON("9002-000", "max"))]}]}
        """
    }

    private static var positionJSON: String {
        """
        {"speed":301,"pokemon":\(pokemonJSON),"faster":12,"slower":30,"tie":[\(entryJSON("9003-000", "max-scarf"))]}
        """
    }

    private static let errorCodes = [
        "invalid_request", "missing_header", "invalid_header", "unknown_pokemon", "request_too_large",
        "master_unavailable", "internal_error",
    ]

    private func errorJSON(_ code: String) -> String { #"{"code":"\#(code)","message":"english message from server"}"# }

    private func assertIdentityHeaders(_ sent: RecordingTransport.Recorded, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID, file: file, line: line)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID, file: file, line: line)
    }

    // MARK: - ポケモン一覧

    func testPokemonSendsGetWithPathAndIdentityHeadersAndNoQuery() async throws {
        let transport = RecordingTransport(json: Self.pokemonListJSON)
        _ = try await makeService(transport: transport).pokemon()
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/speed/v1/pokemon")
        XCTAssertTrue(sent.queryItems.isEmpty)
        assertIdentityHeaders(sent)
    }

    func testPokemonMapsResponse() async throws {
        let list = try await makeService(transport: RecordingTransport(json: Self.pokemonListJSON)).pokemon()
        XCTAssertEqual(list.regulationId, "example")
        XCTAssertEqual(
            list.pokemon,
            [SpeedPokemon(pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100)])
    }

    // MARK: - 表

    func testTableWithoutArgumentsSendsNoQueryAtAll() async throws {
        let transport = RecordingTransport(json: Self.tableJSON)
        _ = try await makeService(transport: transport).table(presets: nil, field: SpeedTableField())
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/speed/v1/table")
        XCTAssertTrue(sent.queryItems.isEmpty, "presets 省略(全6行)・場の状態が off のときは何も付けない(省略 = false と同じ応答)")
        assertIdentityHeaders(sent)
    }

    func testTablePresetsAreSentCommaSeparatedInOneParameter() async throws {
        let transport = RecordingTransport(json: Self.tableJSON)
        _ = try await makeService(transport: transport).table(presets: [.max, .maxScarf], field: SpeedTableField())
        let sent = try XCTUnwrap(transport.requests.first)
        // 契約: style=form, explode=false。順序は結果に影響しない(契約)ので集合で比べる。
        let value = try XCTUnwrap(sent.queryItems["presets"])
        XCTAssertEqual(Set(value.split(separator: ",").map(String.init)), ["max", "max-scarf"])
        XCTAssertEqual(value.split(separator: ",").count, 2, "同名のパラメータを繰り返さず1つにまとめる")
        XCTAssertNil(sent.queryItems["tailwind"])
        XCTAssertNil(sent.queryItems["trickRoom"])
    }

    func testTableSendsFieldFlagsOnlyWhenTrue() async throws {
        let transport = RecordingTransport(json: Self.tableJSON)
        let service = try makeService(transport: transport)
        _ = try await service.table(presets: nil, field: SpeedTableField(tailwind: true, trickRoom: false))
        _ = try await service.table(presets: nil, field: SpeedTableField(tailwind: false, trickRoom: true))
        _ = try await service.table(presets: nil, field: SpeedTableField(tailwind: true, trickRoom: true))
        let queries = transport.requests.map(\.queryItems)
        XCTAssertEqual(queries[0], ["tailwind": "true"])
        XCTAssertEqual(queries[1], ["trickRoom": "true"])
        XCTAssertEqual(queries[2], ["tailwind": "true", "trickRoom": "true"])
    }

    func testTableMapsTiersAndEntriesKeepingTheServerOrder() async throws {
        let table = try await makeService(transport: RecordingTransport(json: Self.tableJSON))
            .table(presets: nil, field: SpeedTableField())
        XCTAssertEqual(table.regulationId, "example")
        XCTAssertEqual(table.presets, [.max, .maxScarf])
        XCTAssertEqual(table.tiers.map(\.speed), [300, 200], "並び(速い順/トリックルームでは遅い順)は応答のまま。並べ替えない")
        XCTAssertEqual(table.tiers[0].entries.map(\.preset), [.maxScarf, .max])
        XCTAssertEqual(table.tiers[0].entries.map(\.pokemonId), ["9001-000", "9003-000"])
        XCTAssertEqual(
            table.tiers[1].entries,
            [SpeedTableEntry(pokemonId: "9002-000", nameJa: "テストカソウドリ", types: ["fire"], baseSpeed: 100, preset: .max)])
    }

    // MARK: - 位置(本文)

    private func sentBody(_ request: SpeedPositionRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.positionJSON)
        _ = try await makeService(transport: transport).position(request)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        return (sent, try sent.jsonBody())
    }

    func testPositionSendsPostWithPathHeadersAndJSONContentType() async throws {
        let (sent, _) = try await sentBody(
            SpeedPositionRequest(input: .preset(pokemonId: "9001-000", preset: .max, scarf: false, tailwind: false, paralysis: false)))
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/speed/v1/position")
        XCTAssertTrue(sent.queryItems.isEmpty)
        XCTAssertEqual(sent.header("Content-Type")?.hasPrefix("application/json"), true)
        assertIdentityHeaders(sent)
    }

    /// mode に要らない項目は送らない(契約上 400 になる)。false の場の効果も送らない(省略 = false と同じ応答。Web と同じ本文)。
    func testPresetBodyHasOnlyTheFieldsOfPresetMode() async throws {
        let (_, body) = try await sentBody(
            SpeedPositionRequest(input: .preset(pokemonId: "9001-000", preset: .neutralMax, scarf: true, tailwind: false, paralysis: false)))
        XCTAssertEqual(Set(body.keys), ["mode", "pokemonId", "preset", "scarf"])
        XCTAssertEqual(body["mode"] as? String, "preset")
        XCTAssertEqual(body["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(body["preset"] as? String, "neutral-max")
        XCTAssertEqual(body["scarf"] as? Bool, true)
    }

    func testPresetBodyAddsFieldEffectsOnlyWhenTrue() async throws {
        let (_, body) = try await sentBody(
            SpeedPositionRequest(
                input: .preset(pokemonId: "9001-000", preset: .max, scarf: false, tailwind: true, paralysis: true),
                tableTailwind: true))
        XCTAssertEqual(Set(body.keys), ["mode", "pokemonId", "preset", "scarf", "tailwind", "paralysis", "tableTailwind"])
        XCTAssertEqual(body["scarf"] as? Bool, false, "scarf は preset・custom で必須なので false でも送る")
        XCTAssertEqual(body["tailwind"] as? Bool, true)
        XCTAssertEqual(body["paralysis"] as? Bool, true)
        XCTAssertEqual(body["tableTailwind"] as? Bool, true)
    }

    func testCustomBodyHasOnlyTheFieldsOfCustomMode() async throws {
        let (_, body) = try await sentBody(
            SpeedPositionRequest(
                input: .custom(pokemonId: "9002-000", sp: 32, nature: .plus, rank: -2, scarf: true, tailwind: false, paralysis: false)))
        XCTAssertEqual(Set(body.keys), ["mode", "pokemonId", "sp", "nature", "rank", "scarf"])
        XCTAssertEqual(body["mode"] as? String, "custom")
        XCTAssertEqual(body["sp"] as? Int, 32)
        XCTAssertEqual(body["nature"] as? String, "plus")
        XCTAssertEqual(body["rank"] as? Int, -2)
        XCTAssertEqual(body["scarf"] as? Bool, true)
    }

    func testCustomBodyAddsFieldEffectsOnlyWhenTrue() async throws {
        let (_, body) = try await sentBody(
            SpeedPositionRequest(
                input: .custom(pokemonId: "9002-000", sp: 0, nature: .minus, rank: 0, scarf: false, tailwind: true, paralysis: false)))
        XCTAssertEqual(Set(body.keys), ["mode", "pokemonId", "sp", "nature", "rank", "scarf", "tailwind"])
        XCTAssertEqual(body["nature"] as? String, "minus")
    }

    func testRawBodyWithoutPokemonHasOnlyModeAndValue() async throws {
        let (_, body) = try await sentBody(SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil)))
        XCTAssertEqual(Set(body.keys), ["mode", "value"])
        XCTAssertEqual(body["mode"] as? String, "raw")
        XCTAssertEqual(body["value"] as? Int, 301)
    }

    func testRawBodyWithPokemonAndTableTailwind() async throws {
        let (_, body) = try await sentBody(
            SpeedPositionRequest(input: .raw(value: 301, pokemonId: "9001-000"), tableTailwind: true))
        XCTAssertEqual(Set(body.keys), ["mode", "value", "pokemonId", "tableTailwind"], "raw は tailwind・paralysis を持たない")
        XCTAssertEqual(body["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(body["tableTailwind"] as? Bool, true)
    }

    // MARK: - 位置(応答)

    func testPositionMapsResponse() async throws {
        let position = try await makeService(transport: RecordingTransport(json: Self.positionJSON))
            .position(SpeedPositionRequest(input: .raw(value: 301, pokemonId: "9001-000")))
        XCTAssertEqual(position.speed, 301)
        XCTAssertEqual(position.pokemon?.pokemonId, "9001-000")
        XCTAssertEqual(position.pokemon?.types, ["fire", "flying"])
        XCTAssertEqual(position.faster, 12)
        XCTAssertEqual(position.slower, 30)
        XCTAssertEqual(position.tie.map(\.preset), [.maxScarf])
        XCTAssertEqual(position.tie.map(\.pokemonId), ["9003-000"])
    }

    func testPositionWithoutPokemonAndWithoutTie() async throws {
        let json = #"{"speed":301,"faster":0,"slower":0,"tie":[]}"#
        let position = try await makeService(transport: RecordingTransport(json: json))
            .position(SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil)))
        XCTAssertNil(position.pokemon, "pokemon は要求に pokemonId が無ければ応答に無い")
        XCTAssertEqual(position.tie, [])
    }

    // MARK: - エラー(3操作とも)

    private typealias Call = @Sendable (APISpeedService) async throws -> Void

    private var calls: [(name: String, statuses: [Int], call: Call)] {
        let position = SpeedPositionRequest(input: .raw(value: 1, pokemonId: nil))
        return [
            ("pokemon", [400, 500, 503], { _ = try await $0.pokemon() }),
            ("table", [400, 500, 503], { _ = try await $0.table(presets: nil, field: SpeedTableField()) }),
            ("position", [400, 413, 422, 500, 503], { _ = try await $0.position(position) }),
        ]
    }

    /// 契約の HTTP エラーはすべて、`code` をそのまま運ぶ `PokeCalcError`(英語の message は画面に出さない側で捨てる)。
    func testDocumentedErrorStatusesKeepTheServerCode() async throws {
        let codeByStatus = [
            400: "invalid_request", 413: "request_too_large", 422: "unknown_pokemon", 500: "internal_error",
            503: "master_unavailable",
        ]
        for entry in calls {
            for status in entry.statuses {
                let code = try XCTUnwrap(codeByStatus[status])
                let service = try makeService(transport: RecordingTransport(status: status, json: errorJSON(code)))
                let error = await assertThrowsPokeCalcError("\(entry.name) \(status)") { try await entry.call(service) }
                XCTAssertEqual(error?.code, code, "\(entry.name) \(status)")
            }
        }
    }

    /// 契約の ErrorCode の語彙(ヘッダー起因の 400 の2種類を含む)がそのまま届く。
    func testEveryContractCodeSurvivesOnBadRequest() async throws {
        for code in ["invalid_request", "missing_header", "invalid_header"] {
            let service = try makeService(transport: RecordingTransport(status: 400, json: errorJSON(code)))
            let error = await assertThrowsPokeCalcError(code) { try await service.pokemon() }
            XCTAssertEqual(error?.code, code)
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        for entry in calls {
            let service = try makeService(transport: FailingTransport())
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.transport, entry.name)
        }
    }

    func testUnreadableSuccessBodyMapsToDecodeError() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 200, json: #"{"unexpected":true}"#))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.decode, entry.name)
        }
    }

    /// 契約に無いステータス(`default` 応答が無いので生成クライアントは undocumented で返す)。
    /// 本文が `{code,message}` の JSON ならその code(gateway の 404/405 など。ADR-0802)。読めなければ専用のコード。
    func testUndocumentedStatusWithErrorBodyKeepsItsCode() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 404, json: errorJSON("not_found")))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, "not_found", entry.name)
        }
    }

    func testUndocumentedStatusWithUnreadableBodyMapsToUnexpectedStatus() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 502, json: "<html>Bad Gateway</html>"))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.unexpectedStatus, entry.name)
        }
    }

    /// タスクのキャンセルは `PokeCalcError` に包まず `CancellationError` のまま(`APIPokeCalcService` と同じ)。
    func testCancellationIsNotWrappedInPokeCalcError() async throws {
        struct CancellingTransport: ClientTransport {
            func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String) async throws
                -> (HTTPResponse, HTTPBody?)
            {
                throw CancellationError()
            }
        }
        let service = try makeService(transport: CancellingTransport())
        do {
            _ = try await service.pokemon()
            XCTFail("エラーにならなかった")
        } catch is CancellationError {
            // 期待どおり
        } catch {
            XCTFail("CancellationError ではない: \(error)")
        }
    }
}
