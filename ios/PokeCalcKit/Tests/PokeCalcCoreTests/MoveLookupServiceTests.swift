import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `PokeCalcService.move(id:)` の2つの実装(issue #68 の残り。ADR-0501「issue #68 の残り」2章)。
///
/// - `APIPokeCalcService`: openapi `getMove`(`GET /api/pokedex/moves/{key}`)に写す。404 は
///   `species(key:)` の 404 と同じく `not_found` の `PokeCalcError` にする。
/// - `MockPokeCalcService`: フィクスチャの技から引き、無ければ他の操作と同じ `PokeCalcError.Code.notFound`。
///
/// 既存の `APIPokeCalcServiceTests` / `MockPokeCalcServiceTests` は変えない(操作を足した分だけここに書く)。
final class MoveLookupServiceTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    /// openapi `Move`(架空。`priority` あり)。
    private static let moveJSON = """
    {"id":"test-move-hidden","nameJa":"テストかくれわざ","type":"water","category":"special","power":60,"priority":1}
    """

    // MARK: - APIPokeCalcService

    func testAPIMoveSendsGetMoveWithIdentityHeaders() async throws {
        let transport = RecordingTransport(json: Self.moveJSON)
        let service = try makeService(transport: transport)

        _ = try await service.move(id: "test-move-hidden")

        XCTAssertEqual(transport.requests.count, 1)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/pokedex/moves/test-move-hidden")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
    }

    func testAPIMoveResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.moveJSON))

        let move = try await service.move(id: "test-move-hidden")

        XCTAssertEqual(move, Move(id: "test-move-hidden", nameJa: "テストかくれわざ", type: .water, category: .special, power: 60, priority: 1))
    }

    func testAPIMoveErrorResponsesKeepTheServerCode() async throws {
        let cases: [(name: String, status: Int, code: String)] = [
            ("getMove 404", 404, "not_found"),
            ("getMove 503", 503, "upstream_unavailable"),
            ("getMove 503 master", 503, "master_unavailable"),
            ("getMove 500 (default)", 500, "internal"),
        ]
        for testCase in cases {
            let json = #"{"code":"\#(testCase.code)","message":"テスト用のエラー"}"#
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: json))
            let error = await assertThrowsPokeCalcError(testCase.name) { try await service.move(id: "test-move-missing") }
            XCTAssertEqual(error?.code, testCase.code, testCase.name)
            XCTAssertEqual(error?.message, "テスト用のエラー", testCase.name)
        }
    }

    func testAPIMoveNotFoundUsesTheSameCodeAsTheClientVocabulary() async throws {
        // 画面(ViewModel)は API/モックを問わず `PokeCalcError.Code.notFound` で 404 を見分ける。
        let json = #"{"code":"not_found","message":"テスト用のエラー"}"#
        let service = try makeService(transport: RecordingTransport(status: 404, json: json))
        let error = await assertThrowsPokeCalcError("getMove 404") { try await service.move(id: "test-move-missing") }
        XCTAssertEqual(error?.code, PokeCalcError.Code.notFound)
    }

    func testAPIMoveTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") { try await service.move(id: "test-move-hidden") }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    // MARK: - MockPokeCalcService

    func testMockMoveReturnsTheSameMoveAsTheSearch() async throws {
        let mock = try MockPokeCalcService()
        let moves = try await mock.searchMoves(query: "", limit: MasterSearch.pageLimit)
        XCTAssertFalse(moves.isEmpty)

        for expected in moves {
            let move = try await mock.move(id: expected.id)
            XCTAssertEqual(move, expected, expected.id)
        }
    }

    func testMockMoveUnknownIDIsNotFound() async throws {
        let mock = try MockPokeCalcService()
        let error = await assertThrowsPokeCalcError("未知の技") { try await mock.move(id: "test-unknown-move") }
        XCTAssertEqual(error?.code, PokeCalcError.Code.notFound)
    }
}
