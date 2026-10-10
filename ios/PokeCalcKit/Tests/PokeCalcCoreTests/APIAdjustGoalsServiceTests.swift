import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// F-11(ADR-0525): `APIPokeCalcService` の `AdjustGoalsService`(`POST /api/calc/adjust/goals`)の写像。
/// 契約どおりのパス・ヘッダー・本文(省略可の欄は nil なら送らない)、応答の写像(null・ランク・未対応の印)、
/// エラーの code をそのまま運ぶこと(404/501 は画面が機能なしと判断する材料)。数値はすべて架空。
final class APIAdjustGoalsServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static let own = Individual(
        speciesKey: "9001-000", natureId: "test-nature-neutral",
        sp: StatBlock(hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
    private static let opponent = Individual(
        speciesKey: "9002-000", natureId: "test-nature-spe-up",
        sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 32), itemId: "test-item-stone")

    private static let resultJSON = """
    {"feasible":false,"remaining":26,
     "plan":{"sp":{"hp":12,"atk":0,"def":8,"spa":0,"spd":0,"spe":20},"totalSp":40,
             "stats":{"hp":155,"atk":100,"def":98,"spa":80,"spd":85,"spe":130}},
     "goals":[
       {"kind":"outspeed","met":true,"chancePercent":null,"selfSpeed":195,"opponentSpeed":120,"selfSpeedRank":1},
       {"kind":"survive","met":false,"chancePercent":37.5,"selfSpeed":null,"opponentSpeed":null,"selfSpeedRank":null},
       {"kind":"ko","met":true,"chancePercent":100,"selfSpeed":null,"opponentSpeed":null,"selfSpeedRank":null}],
     "unsupported":[{"target":"move","reason":"multi_hit","id":"test-move-multi-hit"}]}
    """

    private static let request = AdjustGoalsRequest(
        format: .single, selfIndividual: own,
        goals: [
            AdjustGoalInput(kind: .outspeed, opponent: opponent, moveId: "test-move-boost"),
            AdjustGoalInput(kind: .survive, opponent: opponent, moveId: "test-move-special-a", hits: 2, thresholdPercent: 90),
            AdjustGoalInput(kind: .ko, opponent: opponent, moveId: "test-move-physical-a", hits: 1),
        ])

    func testSendsPostWithHeadersAndBody() async throws {
        let transport = RecordingTransport(json: Self.resultJSON)
        _ = try await makeService(transport: transport).adjustGoals(Self.request)

        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/adjust/goals")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        let body = try sent.jsonBody()
        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertNil(body["ceiling"], "nil の ceiling は送らない(各能力 32)")
        let own = try XCTUnwrap(body["self"] as? [String: Any], "契約のキーは self")
        XCTAssertEqual(own["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(own["sp"] as? [String: Int], ["hp": 4, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0])
        let goals = try XCTUnwrap(body["goals"] as? [[String: Any]])
        XCTAssertEqual(goals.map { $0["kind"] as? String }, ["outspeed", "survive", "ko"], "目標の順を保つ")

        let outspeed = goals[0]
        XCTAssertEqual(outspeed["moveId"] as? String, "test-move-boost")
        XCTAssertNil(outspeed["hits"])
        XCTAssertNil(outspeed["thresholdPercent"])
        let opponent = try XCTUnwrap(outspeed["opponent"] as? [String: Any])
        XCTAssertEqual(opponent["speciesKey"] as? String, "9002-000")
        XCTAssertEqual(opponent["natureId"] as? String, "test-nature-spe-up")
        XCTAssertEqual(opponent["itemId"] as? String, "test-item-stone")
        XCTAssertEqual(goals[1]["hits"] as? Int, 2)
        XCTAssertEqual(goals[1]["thresholdPercent"] as? Double, 90)
        XCTAssertEqual(goals[2]["hits"] as? Int, 1)
        XCTAssertNil(goals[2]["thresholdPercent"], "nil のしきい値は送らない(契約の既定 100)")
    }

    func testSendsCeilingWhenGiven() async throws {
        let transport = RecordingTransport(json: Self.resultJSON)
        var request = Self.request
        request.ceiling = AdjustCeiling(hp: 20, spe: 10)
        _ = try await makeService(transport: transport).adjustGoals(request)
        let body = try XCTUnwrap(transport.requests.first).jsonBody()
        let ceiling = try XCTUnwrap(body["ceiling"] as? [String: Int])
        XCTAssertEqual(ceiling["hp"], 20)
        XCTAssertEqual(ceiling["spe"], 10)
        XCTAssertNil(ceiling["atk"])
    }

    func testResponseMapsToDomain() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.resultJSON)).adjustGoals(Self.request)
        XCTAssertFalse(result.feasible)
        XCTAssertEqual(result.remaining, 26)
        XCTAssertEqual(result.plan, AdjustGoalsPlan(
            sp: StatBlock(hp: 12, atk: 0, def: 8, spa: 0, spd: 0, spe: 20), totalSp: 40,
            stats: StatBlock(hp: 155, atk: 100, def: 98, spa: 80, spd: 85, spe: 130)))
        XCTAssertEqual(result.goals, [
            AdjustGoalOutcome(kind: .outspeed, met: true, chancePercent: nil, selfSpeed: 195, opponentSpeed: 120, selfSpeedRank: 1),
            AdjustGoalOutcome(kind: .survive, met: false, chancePercent: 37.5),
            AdjustGoalOutcome(kind: .ko, met: true, chancePercent: 100),
        ])
        XCTAssertEqual(result.unsupported, [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-multi-hit")])
    }

    func testErrorResponsesKeepServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_input"), (500, "type_chart_missing"), (503, "master_unavailable"), (503, "upstream_unavailable"),
            (404, "not_found"),
        ]
        for item in cases {
            let json = #"{"code":"\#(item.code)","message":"internal detail"}"#
            let service = try makeService(transport: RecordingTransport(status: item.status, json: json))
            let error = await assertThrowsPokeCalcError("\(item.status)") { try await service.adjustGoals(Self.request) }
            XCTAssertEqual(error?.code, item.code, "\(item.status)")
        }
    }

    func testTransportAndDecodeFailuresAreDistinguished() async throws {
        let failing = try makeService(transport: FailingTransport())
        let transportError = await assertThrowsPokeCalcError("transport") { try await failing.adjustGoals(Self.request) }
        XCTAssertEqual(transportError?.code, PokeCalcError.Code.transport)
        let malformed = try makeService(transport: RecordingTransport(json: #"{"unexpected":true}"#))
        let decodeError = await assertThrowsPokeCalcError("decode") { try await malformed.adjustGoals(Self.request) }
        XCTAssertEqual(decodeError?.code, PokeCalcError.Code.decode)
    }
}
