import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `PokeCalcService.moves(ids:)` の2つの実装(`getMove` の複数版・openapi `getMovesByIds`)。
/// ADR-0501「getMovesByIds による構築編集の技の一括解決」1〜2章。
///
/// - `APIPokeCalcService`: `client.getMovesByIds` に写す。`ids` を重複除去し、空なら通信しない。
///   `RequestLimits.maxMoveBatchIds` を超える集合は、この件数ずつに分割して複数回呼び、応答を
///   渡した順に連結する。404 は無い(マスタに無い ID は 200 の応答から黙って省かれる)。
/// - `MockPokeCalcService`: フィクスチャの技から `ids` の順(重複除去後)に引き、無ければ省く。
///
/// 既存の `MoveLookupServiceTests`(`move(id:)`)・`APIPokeCalcServiceTests`・`MockPokeCalcServiceTests` は
/// 変えない(操作を足した分だけここに書く)。
final class MoveBatchLookupServiceTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    /// `RecordingTransport.Recorded.queryItems` は同名クエリが複数あると最後の値しか残さない
    /// (`?ids=a&ids=b` を `["ids": "b"]` に潰す)ので、繰り返しクエリの検証にはこちらを使う。
    private func allQueryValues(_ recorded: RecordingTransport.Recorded, name: String) -> [String] {
        (URLComponents(string: recorded.request.path ?? "")?.queryItems ?? [])
            .filter { $0.name == name }
            .compactMap(\.value)
    }

    /// openapi `Move` の JSON を組み立てる(`priority` は省略可なので付けない = 0 になる)。
    private static func moveJSON(id: String) -> String {
        #"{"id":"\#(id)","nameJa":"テスト\#(id)","type":"normal","category":"physical","power":40}"#
    }

    // MARK: - APIPokeCalcService: 要求

    func testAPIMovesSendsGetMovesByIdsWithRepeatedQueryAndIdentityHeaders() async throws {
        let transport = RecordingTransport(json: "[\(Self.moveJSON(id: "stub-batch-a"))]")
        let service = try makeService(transport: transport)

        _ = try await service.moves(ids: ["stub-batch-a", "stub-batch-b", "stub-batch-c"])

        XCTAssertEqual(transport.requests.count, 1, "64件以下は1回の呼び出しにまとまる")
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/pokedex/moves/batch")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        XCTAssertEqual(allQueryValues(sent, name: "ids"), ["stub-batch-a", "stub-batch-b", "stub-batch-c"],
                       "繰り返しクエリ ids=a&ids=b&ids=c で渡す順序も保つ")
    }

    func testAPIMovesEmptyInputMakesNoCall() async throws {
        let transport = RecordingTransport(json: "[]")
        let service = try makeService(transport: transport)

        let result = try await service.moves(ids: [])

        XCTAssertEqual(transport.requests.count, 0, "空配列なら getMovesByIds を呼ばない")
        XCTAssertEqual(result, [])
    }

    func testAPIMovesDedupesIdsBeforeSending() async throws {
        let transport = RecordingTransport(json: "[\(Self.moveJSON(id: "stub-batch-a")),\(Self.moveJSON(id: "stub-batch-b"))]")
        let service = try makeService(transport: transport)

        _ = try await service.moves(ids: ["stub-batch-a", "stub-batch-a", "stub-batch-b"])

        XCTAssertEqual(transport.requests.count, 1)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(allQueryValues(sent, name: "ids"), ["stub-batch-a", "stub-batch-b"],
                       "同じ ID を2回渡しても、送るクエリは1回にまとめる")
    }

    /// 分割の境界(`RequestLimits.maxMoveBatchIds` = `max` の倍数 ± 1)を表駆動で確かめる:
    /// `max` 件ちょうどなら1回、`max + 1` 件で2回、`2 * max` 件でも2回、`2 * max + 1` 件で3回。
    /// 各回の `ids` が空にならないこと(`stride(from: 0, through:)` に変えると最後の回が空配列になる
    /// 退行の検出)・分割しても連結すれば渡した順のままであることも確かめる。
    func testAPIMovesChunkBoundaries() async throws {
        let max = RequestLimits.maxMoveBatchIds
        let cases: [(name: String, count: Int, expectedCalls: Int)] = [
            ("max", max, 1),
            ("max + 1", max + 1, 2),
            ("2 * max", 2 * max, 2),
            ("2 * max + 1", 2 * max + 1, 3),
        ]
        for testCase in cases {
            let ids = (0..<testCase.count).map { "stub-batch-id-\($0)" }
            let transport = RecordingTransport { request in
                let requestedIds = (URLComponents(string: request.path ?? "")?.queryItems ?? [])
                    .filter { $0.name == "ids" }
                    .compactMap(\.value)
                let json = requestedIds.map(Self.moveJSON(id:)).joined(separator: ",")
                return RecordingTransport.Stub(status: 200, json: "[\(json)]")
            }
            let service = try makeService(transport: transport)

            let result = try await service.moves(ids: ids)

            XCTAssertEqual(transport.requests.count, testCase.expectedCalls, testCase.name)
            var sentIds: [String] = []
            for sent in transport.requests {
                let requestedIds = allQueryValues(sent, name: "ids")
                XCTAssertFalse(requestedIds.isEmpty, "\(testCase.name): 各回の ids は空にならない")
                sentIds += requestedIds
            }
            XCTAssertEqual(sentIds, ids, "\(testCase.name): 分割しても渡した順のまま")
            XCTAssertEqual(result.map(\.id), ids, "\(testCase.name): 分割した応答は渡した順に連結する")
        }
    }

    // MARK: - APIPokeCalcService: 応答 → ドメイン

    func testAPIMovesResponseMapsToDomainInOrder() async throws {
        let json = "[\(Self.moveJSON(id: "stub-batch-a")),\(Self.moveJSON(id: "stub-batch-b"))]"
        let service = try makeService(transport: RecordingTransport(json: json))

        let result = try await service.moves(ids: ["stub-batch-a", "stub-batch-b"])

        XCTAssertEqual(result, [
            Move(id: "stub-batch-a", nameJa: "テストstub-batch-a", type: .normal, category: .physical, power: 40),
            Move(id: "stub-batch-b", nameJa: "テストstub-batch-b", type: .normal, category: .physical, power: 40),
        ])
    }

    // MARK: - APIPokeCalcService: 失敗

    func testAPIMovesErrorResponsesKeepTheServerCode() async throws {
        // getMovesByIds に 404 は無い(マスタに無い ID は 200 の応答から省かれるだけ)。
        let cases: [(name: String, status: Int, code: String)] = [
            ("getMovesByIds 503", 503, "upstream_unavailable"),
            ("getMovesByIds 503 master", 503, "master_unavailable"),
            ("getMovesByIds 500 (default)", 500, "internal"),
        ]
        for testCase in cases {
            let json = #"{"code":"\#(testCase.code)","message":"テスト用のエラー"}"#
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: json))
            let error = await assertThrowsPokeCalcError(testCase.name) { try await service.moves(ids: ["stub-batch-a"]) }
            XCTAssertEqual(error?.code, testCase.code, testCase.name)
            XCTAssertEqual(error?.message, "テスト用のエラー", testCase.name)
        }
    }

    func testAPIMovesTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") { try await service.moves(ids: ["stub-batch-a"]) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    // MARK: - MockPokeCalcService

    func testMockMovesReturnsKnownMovesOmittingUnknownInIdsOrder() async throws {
        let mock = try MockPokeCalcService()
        let moves = try await mock.searchMoves(query: "", limit: MasterSearch.pageLimit)
        let first = try XCTUnwrap(moves.first)
        let second = try XCTUnwrap(moves.dropFirst().first)

        let result = try await mock.moves(ids: [second.id, "test-unknown-move", first.id])

        XCTAssertEqual(result, [second, first], "見つからない ID は省き、渡した ids の順のまま返す")
    }

    func testMockMovesEmptyInputReturnsEmptyArray() async throws {
        let mock = try MockPokeCalcService()
        let result = try await mock.moves(ids: [])
        XCTAssertEqual(result, [])
    }

    func testMockMovesDedupesRepeatedIds() async throws {
        let mock = try MockPokeCalcService()
        let moves = try await mock.searchMoves(query: "", limit: MasterSearch.pageLimit)
        let move = try XCTUnwrap(moves.first)

        let result = try await mock.moves(ids: [move.id, move.id])

        XCTAssertEqual(result, [move], "同じ ID を複数渡しても1件だけ返す")
    }
}
