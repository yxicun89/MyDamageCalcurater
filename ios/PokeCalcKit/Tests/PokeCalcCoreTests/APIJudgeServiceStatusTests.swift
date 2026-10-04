import Foundation
import OpenAPIRuntime
import PokeCalcJudgeAPI
import XCTest

@testable import PokeCalcCore

/// `APIJudgeService` の状態異常の送信(契約 v0.3.0。ADR-0512): `status` は `nil` なら欄ごと送らない(null も送らない)、
/// 非 nil なら契約の値(`badly_poison` はスネークケース)をそのまま送る。自分と候補を取り違えない。他の本文は変えない。
final class APIJudgeServiceStatusTests: XCTestCase {
    private static let sp = StatBlock(hp: 1, atk: 2, def: 3, spa: 4, spd: 5, spe: 6)
    private static let emptyRow = #"{"defenderIndex":0,"outspeeds":true,"speedTie":false,"attackerSpeed":1,"defenderSpeed":1,"attackerMovePriority":0,"defenderMovePriority":0,"attackerMovesFirst":true,"turnOrderTie":false,"attackerKo":{"hits":1,"guaranteed":true,"displayChancePercent":100.0},"defenderKo":{"hits":1,"guaranteed":true,"displayChancePercent":100.0},"attackerKoUnsupported":[],"defenderKoUnsupported":[],"attackerSpeedApplied":[],"defenderSpeedApplied":[],"attackerSpeedIgnored":[],"defenderSpeedIgnored":[]}"#

    private func body(for request: JudgeRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: #"{"matchups":[\#(Self.emptyRow)]}"#)
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let service = APIJudgeService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11", sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"))
        _ = try await service.outspeedAndKo(request)
        let recorded = try XCTUnwrap(transport.requests.first)
        return (recorded, try recorded.jsonBody())
    }

    private func request(attacker: JudgeStatus? = nil, defenders: [JudgeStatus?] = [nil]) -> JudgeRequest {
        JudgeRequest(
            attacker: JudgeIndividual(speciesKey: "9001-000", natureId: "n", sp: Self.sp, status: attacker), moveId: "m-self",
            defenders: defenders.enumerated().map { index, status in
                JudgeDefender(
                    individual: JudgeIndividual(speciesKey: "900\(index + 2)-000", natureId: "n", sp: Self.sp, status: status),
                    moveId: "m-\(index)")
            })
    }

    func testNilStatusIsOmittedAndNeverNull() async throws {
        let (recorded, json) = try await body(for: request())
        let attacker = try XCTUnwrap(json["attacker"] as? [String: Any])
        XCTAssertEqual(Set(attacker.keys), ["speciesKey", "natureId", "sp"], "status を足しても従来の本文は変わらない")
        let defenders = try XCTUnwrap(json["defenders"] as? [[String: Any]])
        XCTAssertEqual(Set(defenders[0].keys), ["speciesKey", "natureId", "sp", "moveId"])
        XCTAssertFalse(String(decoding: try XCTUnwrap(recorded.body), as: UTF8.self).contains("null"))
    }

    func testEveryStatusIsSentAsTheContractValueForTheAttacker() async throws {
        for status in JudgeStatus.allCases where status != .none {
            let (_, json) = try await body(for: request(attacker: status))
            let attacker = try XCTUnwrap(json["attacker"] as? [String: Any])
            XCTAssertEqual(attacker["status"] as? String, status.rawValue, status.rawValue)
            let defenders = try XCTUnwrap(json["defenders"] as? [[String: Any]])
            XCTAssertNil(defenders[0]["status"], "自分の状態異常が候補に載らない")
        }
    }

    func testEveryStatusIsSentAsTheContractValueForACandidate() async throws {
        for status in JudgeStatus.allCases where status != .none {
            let (_, json) = try await body(for: request(defenders: [status]))
            let defenders = try XCTUnwrap(json["defenders"] as? [[String: Any]])
            XCTAssertEqual(defenders[0]["status"] as? String, status.rawValue, status.rawValue)
            let attacker = try XCTUnwrap(json["attacker"] as? [String: Any])
            XCTAssertNil(attacker["status"], "候補の状態異常が自分に載らない")
        }
    }

    func testBadlyPoisonIsSnakeCase() async throws {
        let (_, json) = try await body(for: request(attacker: .badlyPoison))
        XCTAssertEqual((json["attacker"] as? [String: Any])?["status"] as? String, "badly_poison")
    }

    func testStatusIsKeptPerCandidateInOrder() async throws {
        let (_, json) = try await body(for: request(attacker: .burn, defenders: [.paralysis, nil, .sleep]))
        let defenders = try XCTUnwrap(json["defenders"] as? [[String: Any]])
        XCTAssertEqual(defenders.map { $0["status"] as? String }, ["paralysis", nil, "sleep"])
        XCTAssertEqual((json["attacker"] as? [String: Any])?["status"] as? String, "burn")
    }

    func testStatusSitsNextToTheOtherOptionalFields() async throws {
        let attacker = JudgeIndividual(
            speciesKey: "9001-000", natureId: "n", sp: Self.sp, ranks: RankBlock(spe: 1), abilityId: "a", itemId: "i", status: .paralysis)
        let (_, json) = try await body(
            for: JudgeRequest(attacker: attacker, moveId: "m", defenders: [JudgeDefender(individual: attacker, moveId: "m2")]))
        XCTAssertEqual(
            Set(try XCTUnwrap(json["attacker"] as? [String: Any]).keys),
            ["speciesKey", "natureId", "sp", "ranks", "abilityId", "itemId", "status"])
    }
}
