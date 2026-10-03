import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// P6-23: `APIPokeCalcService` の `FrequentOpponentsService`(`listFrequentOpponents`)の写像(ADR-0501「P6-23」)。
final class APIFrequentOpponentsServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static let listJSON = """
        [
          {"speciesKey":"9003-000","score":4.5,"count":6,"lastCalculatedAt":"2026-10-01T09:30:00Z"},
          {"speciesKey":"9001-000","score":4.5,"count":3,"lastCalculatedAt":"2026-09-30T00:00:00Z"},
          {"speciesKey":"9002-000","score":0.25,"count":1,"lastCalculatedAt":"2026-08-01T12:00:00Z"}
        ]
        """

    func testSendsGetWithPathLimitAndIdentityHeaders() async throws {
        let transport = RecordingTransport(json: Self.listJSON)
        _ = try await makeService(transport: transport).frequentOpponents(limit: 7)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/record/frequent-opponents")
        XCTAssertEqual(sent.queryItems["limit"], "7", "limit はそのままクエリに載せる")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
    }

    func testOutOfRangeLimitIsNotClampedByTheClient() async throws {
        for limit in [0, 51] {
            let transport = RecordingTransport(json: Self.listJSON)
            _ = try? await makeService(transport: transport).frequentOpponents(limit: limit)
            XCTAssertEqual(transport.requests.first?.queryItems["limit"], "\(limit)", "範囲外の判定はサーバー(400)に任せる")
        }
    }

    func testOKMapsAllFieldsKeepingServerOrder() async throws {
        let list = try await makeService(transport: RecordingTransport(json: Self.listJSON)).frequentOpponents(limit: 10)
        XCTAssertEqual(list.map(\.speciesKey), ["9003-000", "9001-000", "9002-000"], "サーバーの順(スコア降順・同点は key 昇順)を保つ")
        XCTAssertEqual(list.map(\.score), [4.5, 4.5, 0.25])
        XCTAssertEqual(list.map(\.count), [6, 3, 1])
        XCTAssertEqual(list.first?.lastCalculatedAt, ISO8601DateFormatter().date(from: "2026-10-01T09:30:00Z"))
    }

    func testEmptyArrayIsSuccessNotNotFound() async throws {
        let list = try await makeService(transport: RecordingTransport(json: "[]")).frequentOpponents(limit: 10)
        XCTAssertEqual(list, [])
    }

    func testBadRequestMapsToInvalidInput() async throws {
        let json = #"{"code":"invalid_input","message":"limit は 1〜50"}"#
        let service = try makeService(transport: RecordingTransport(status: 400, json: json))
        let error = await assertThrowsPokeCalcError("400") { try await service.frequentOpponents(limit: 0) }
        XCTAssertEqual(error?.code, "invalid_input")
    }

    func testServiceUnavailableMapsToPokeCalcErrorKeepingCode() async throws {
        for code in ["store_unavailable", "upstream_unavailable"] {
            let json = #"{"code":"\#(code)","message":"テスト用のエラー"}"#
            let service = try makeService(transport: RecordingTransport(status: 503, json: json))
            let error = await assertThrowsPokeCalcError(code) { try await service.frequentOpponents(limit: 10) }
            XCTAssertEqual(error?.code, code)
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") { try await service.frequentOpponents(limit: 10) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }
}
