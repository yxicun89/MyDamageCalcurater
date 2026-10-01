import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// P6-7: `APIPokeCalcService` の `DeviceDataService`(2本の DELETE)の写像(ADR-0501「P6-7」7章)。
final class APIDeviceDataServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static func recordJSON(_ status: String) -> String {
        #"{"status":"\#(status)","purgedAt":"2026-10-01T00:00:00Z","deleted":{"calcEvents":1,"aggregates":2,"favorites":3}}"#
    }

    private static func teamJSON(_ status: String) -> String {
        #"{"status":"\#(status)","purgedAt":"2026-10-01T00:00:00Z","deleted":{"teams":1,"teamMembers":2}}"#
    }

    private typealias Call = @Sendable (APIPokeCalcService) async throws -> DeletionProgress

    private var targets: [(name: String, path: String, json: @Sendable (String) -> String, call: Call)] {
        [
            ("record", "/api/record/device-data", { Self.recordJSON($0) }, { try await $0.deleteRecordDeviceData() }),
            ("team", "/api/team/device-data", { Self.teamJSON($0) }, { try await $0.deleteTeamDeviceData() }),
        ]
    }

    func testSendsDeleteWithPathAndIdentityHeaders() async throws {
        for target in targets {
            let transport = RecordingTransport(json: target.json("completed"))
            _ = try await target.call(try makeService(transport: transport))
            let sent = try XCTUnwrap(transport.requests.first, target.name)
            XCTAssertEqual(transport.requests.count, 1, target.name)
            XCTAssertEqual(sent.request.method.rawValue, "DELETE", target.name)
            XCTAssertEqual(sent.pathWithoutQuery, target.path, target.name)
            XCTAssertEqual(sent.header("X-Device-Id"), deviceID, target.name)
            XCTAssertEqual(sent.header("X-Session-Id"), sessionID, target.name)
        }
    }

    func testStatusMapsToDeletionProgress() async throws {
        for target in targets {
            for (status, expected) in [("completed", DeletionProgress.completed), ("partial", .partial)] {
                let service = try makeService(transport: RecordingTransport(json: target.json(status)))
                let progress = try await target.call(service)
                XCTAssertEqual(progress, expected, "\(target.name) \(status)")
            }
        }
    }

    func testServiceUnavailableMapsToPokeCalcErrorKeepingCode() async throws {
        for target in targets {
            for code in ["store_unavailable", "upstream_unavailable"] {
                let json = #"{"code":"\#(code)","message":"テスト用のエラー"}"#
                let service = try makeService(transport: RecordingTransport(status: 503, json: json))
                let error = await assertThrowsPokeCalcError("\(target.name) \(code)") { try await target.call(service) }
                XCTAssertEqual(error?.code, code, target.name)
            }
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        for target in targets {
            let service = try makeService(transport: FailingTransport())
            let error = await assertThrowsPokeCalcError(target.name) { try await target.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.transport, target.name)
        }
    }
}
