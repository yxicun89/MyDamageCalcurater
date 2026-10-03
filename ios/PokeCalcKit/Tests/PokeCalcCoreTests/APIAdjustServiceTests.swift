import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// AJ7: `APIPokeCalcService` の `AdjustService`(調整 API 4本と技の逆引き)の写像(ADR-0502 §3・AC1)。
///
/// 固定すること: 契約どおりのパス・メソッド・ヘッダー・本文(省略可の欄は nil なら送らない)、応答の写像
/// (int64・null・HPLineKind・未対応の印)、エラーの code をそのまま運ぶこと、通信失敗・デコード失敗の区別。
/// 数値はすべて架空(engine の計算の写しではない)。
final class APIAdjustServiceTests: XCTestCase {
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
        sp: StatBlock(hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
    )
    private static let opponent = Individual(
        speciesKey: "9002-000", natureId: "test-nature-def-up",
        sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0)
    )

    // MARK: - 応答の JSON(架空)

    private static let indicesJSON = """
    {"stats":{"hp":155,"atk":100,"def":90,"spa":80,"spd":85,"spe":120},
     "firepowerIndex":3000000000,"physicalBulkIndex":13950,"specialBulkIndex":13175,
     "hpLines":{"hp":155,"sp":0,"current":"16n-1",
       "next16n":{"hp":160,"sp":5,"spDelta":5},"prev16n":null,
       "next16nMinus1":{"hp":175,"sp":20,"spDelta":20},"prev16nMinus1":null}}
    """
    private static let indicesNullFirepowerJSON = """
    {"stats":{"hp":155,"atk":100,"def":90,"spa":80,"spd":85,"spe":120},
     "firepowerIndex":null,"physicalBulkIndex":13950,"specialBulkIndex":13175,
     "hpLines":{"hp":160,"sp":5,"current":"16n",
       "next16n":null,"prev16n":{"hp":144,"sp":0,"spDelta":-5},
       "next16nMinus1":{"hp":175,"sp":20,"spDelta":15},"prev16nMinus1":{"hp":159,"sp":4,"spDelta":-1}}}
    """
    private static let koJSON = """
    {"stat":"spa","searchLimit":32,"feasible":false,"sp":32,"chancePercent":37.5,
     "unsupported":[{"target":"move","reason":"multi_hit","id":"test-move-multi-hit"}]}
    """
    private static let surviveJSON = """
    {"stat":"spd","searchLimit":62,"feasible":true,"hpSp":20,"statSp":8,"totalSp":28,
     "bulkIndex":4000000000,"chancePercent":100,"unsupported":[]}
    """
    private static let planJSON = """
    {"sp":{"hp":32,"atk":0,"def":17,"spa":0,"spd":17,"spe":0},"totalSp":66,
     "stats":{"hp":187,"atk":100,"def":107,"spa":80,"spd":102,"spe":120},
     "physicalBulk":20009,"specialBulk":19074,"speedMet":true,"goalMet":false,"chancePercent":87.5}
    """
    private static var allocationJSON: String {
        #"{"remaining":62,"maxIndex":\#(planJSON),"minSp":\#(planJSON),"unsupported":[{"target":"defender_item","reason":"unsupported_effect","id":"test-item-a"}]}"#
    }
    private static var allocationNoGoalJSON: String {
        #"{"remaining":62,"maxIndex":\#(planJSON),"minSp":null,"unsupported":[]}"#
    }
    private static let learnersJSON = """
    [{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストモンいち","types":["fire"]},
     {"key":"9002-000","dexNo":9002,"form":0,"nameJa":"テストモンに","types":["water","flying"]}]
    """

    // MARK: - (1) indices

    func testIndicesSendsPostWithHeadersAndBody() async throws {
        let transport = RecordingTransport(json: Self.indicesJSON)
        _ = try await makeService(transport: transport).adjustIndices(
            AdjustIndicesRequest(individual: Self.own, moveId: "test-move-special-a", modifier: 6144))

        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/adjust/indices")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        let body = try sent.jsonBody()
        let individual = try XCTUnwrap(body["individual"] as? [String: Any])
        XCTAssertEqual(individual["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(individual["natureId"] as? String, "test-nature-neutral")
        XCTAssertEqual(individual["level"] as? Int, 50)
        XCTAssertEqual(individual["sp"] as? [String: Int], ["hp": 4, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0])
        XCTAssertEqual(body["moveId"] as? String, "test-move-special-a")
        XCTAssertEqual(body["modifier"] as? Int, 6144)
        XCTAssertNil(body["damageModifier"], "nil の補正は送らない(契約の既定 4096)")
    }

    func testIndicesOmitsOptionalKeysWhenNil() async throws {
        let transport = RecordingTransport(json: Self.indicesNullFirepowerJSON)
        _ = try await makeService(transport: transport).adjustIndices(AdjustIndicesRequest(individual: Self.own))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()
        XCTAssertNil(body["moveId"])
        XCTAssertNil(body["modifier"])
        XCTAssertNil(body["damageModifier"])
    }

    func testIndicesResponseMapsToDomain() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.indicesJSON))
            .adjustIndices(AdjustIndicesRequest(individual: Self.own, moveId: "test-move-special-a"))
        XCTAssertEqual(result.stats, StatBlock(hp: 155, atk: 100, def: 90, spa: 80, spd: 85, spe: 120))
        XCTAssertEqual(result.firepowerIndex, 3_000_000_000, "int64 の値を落とさない")
        XCTAssertEqual(result.physicalBulkIndex, 13950)
        XCTAssertEqual(result.specialBulkIndex, 13175)
        XCTAssertEqual(result.hpLines, HPLineReport(
            hp: 155, sp: 0, current: .line16nMinus1,
            next16n: HPLinePoint(hp: 160, sp: 5, spDelta: 5), prev16n: nil,
            next16nMinus1: HPLinePoint(hp: 175, sp: 20, spDelta: 20), prev16nMinus1: nil
        ))
    }

    func testIndicesResponseMapsNullFirepowerAndAllLines() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.indicesNullFirepowerJSON))
            .adjustIndices(AdjustIndicesRequest(individual: Self.own))
        XCTAssertNil(result.firepowerIndex)
        XCTAssertEqual(result.hpLines.current, .line16n)
        XCTAssertNil(result.hpLines.next16n)
        XCTAssertEqual(result.hpLines.prev16n, HPLinePoint(hp: 144, sp: 0, spDelta: -5))
        XCTAssertEqual(result.hpLines.prev16nMinus1, HPLinePoint(hp: 159, sp: 4, spDelta: -1))
    }

    func testHPLineKindNoneMapsToNone() async throws {
        let json = Self.indicesJSON.replacingOccurrences(of: #""current":"16n-1""#, with: #""current":"none""#)
        let result = try await makeService(transport: RecordingTransport(json: json))
            .adjustIndices(AdjustIndicesRequest(individual: Self.own))
        XCTAssertEqual(result.hpLines.current, HPLineKind.none)
    }

    // MARK: - (2) min-sp-to-ko / min-sp-to-survive

    func testMinSpToKoSendsSearchRequest() async throws {
        let transport = RecordingTransport(json: Self.koJSON)
        _ = try await makeService(transport: transport).adjustMinSpToKo(AdjustSearchRequest(
            format: .single, attacker: Self.own, defender: Self.opponent, moveId: "test-move-special-a",
            hits: 2, thresholdPercent: 90))

        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/adjust/min-sp-to-ko")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        let body = try sent.jsonBody()
        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertEqual(body["moveId"] as? String, "test-move-special-a")
        XCTAssertEqual(body["hits"] as? Int, 2)
        XCTAssertEqual(body["thresholdPercent"] as? Double, 90)
        XCTAssertEqual((body["attacker"] as? [String: Any])?["speciesKey"] as? String, "9001-000")
        let defender = try XCTUnwrap(body["defender"] as? [String: Any])
        XCTAssertEqual(defender["speciesKey"] as? String, "9002-000")
        XCTAssertEqual(defender["natureId"] as? String, "test-nature-def-up")
        XCTAssertEqual(defender["sp"] as? [String: Int], ["hp": 32, "atk": 0, "def": 32, "spa": 0, "spd": 0, "spe": 0])
    }

    func testSearchOmitsThresholdWhenNil() async throws {
        let transport = RecordingTransport(json: Self.surviveJSON)
        _ = try await makeService(transport: transport).adjustMinSpToSurvive(AdjustSearchRequest(
            format: .single, attacker: Self.opponent, defender: Self.own, moveId: "test-move-physical-a", hits: 1))
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/adjust/min-sp-to-survive")
        let body = try sent.jsonBody()
        XCTAssertNil(body["thresholdPercent"], "100(確定)は契約の既定なので送らない")
        XCTAssertNil(body["field"], "場は AJ7 では送らない")
        XCTAssertEqual(body["hits"] as? Int, 1)
    }

    func testMinSpToKoResponseMapsToDomain() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.koJSON)).adjustMinSpToKo(
            AdjustSearchRequest(format: .single, attacker: Self.own, defender: Self.opponent, moveId: "m", hits: 2))
        XCTAssertEqual(result, AdjustKOResult(
            stat: .spa, searchLimit: 32, feasible: false, sp: 32, chancePercent: 37.5,
            unsupported: [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-multi-hit")]
        ))
    }

    func testMinSpToSurviveResponseMapsToDomain() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.surviveJSON)).adjustMinSpToSurvive(
            AdjustSearchRequest(format: .single, attacker: Self.opponent, defender: Self.own, moveId: "m", hits: 1))
        XCTAssertEqual(result, AdjustSurviveResult(
            stat: .spd, searchLimit: 62, feasible: true, hpSp: 20, statSp: 8, totalSp: 28,
            bulkIndex: 4_000_000_000, chancePercent: 100, unsupported: []
        ))
    }

    // MARK: - (3) allocation

    func testAllocationBulkSendsSelfCeilingModeFocusAndMinSpeed() async throws {
        let transport = RecordingTransport(json: Self.allocationNoGoalJSON)
        _ = try await makeService(transport: transport).adjustAllocation(AdjustAllocationRequest(
            selfIndividual: Self.own, ceiling: AdjustCeiling(hp: 32, def: 20, spd: 32), mode: .bulk, focus: .both))

        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/adjust/allocation")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        let body = try sent.jsonBody()
        XCTAssertEqual((body["self"] as? [String: Any])?["speciesKey"] as? String, "9001-000", "契約のキーは self")
        XCTAssertEqual(body["ceiling"] as? [String: Int], ["hp": 32, "def": 20, "spd": 32], "nil の能力は送らない")
        XCTAssertEqual(body["mode"] as? String, "bulk")
        XCTAssertEqual(body["focus"] as? String, "both")
        XCTAssertNil(body["offenseCategory"])
        XCTAssertEqual(body["minSpeed"] as? Int, 0, "生成型では任意だが Web と同じく常に送る")
        XCTAssertNil(body["goal"])
    }

    func testAllocationOffenseSendsCategoryMinSpeedAndGoal() async throws {
        let transport = RecordingTransport(json: Self.allocationJSON)
        _ = try await makeService(transport: transport).adjustAllocation(AdjustAllocationRequest(
            selfIndividual: Self.own, ceiling: AdjustCeiling(spa: 32, spe: 20), mode: .offense,
            offenseCategory: .special, minSpeed: 120,
            goal: AdjustAllocGoal(format: .single, opponent: Self.opponent, moveId: "test-move-special-a", hits: 2, thresholdPercent: 75)))

        let body = try XCTUnwrap(transport.requests.first).jsonBody()
        XCTAssertEqual(body["mode"] as? String, "offense")
        XCTAssertNil(body["focus"])
        XCTAssertEqual(body["offenseCategory"] as? String, "special")
        XCTAssertEqual(body["minSpeed"] as? Int, 120)
        XCTAssertEqual(body["ceiling"] as? [String: Int], ["spa": 32, "spe": 20])
        let goal = try XCTUnwrap(body["goal"] as? [String: Any])
        XCTAssertEqual(goal["format"] as? String, "single")
        XCTAssertEqual(goal["moveId"] as? String, "test-move-special-a")
        XCTAssertEqual(goal["hits"] as? Int, 2)
        XCTAssertEqual(goal["thresholdPercent"] as? Double, 75)
        XCTAssertEqual((goal["opponent"] as? [String: Any])?["natureId"] as? String, "test-nature-def-up")
    }

    func testAllocationResponseMapsToDomain() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.allocationJSON)).adjustAllocation(
            AdjustAllocationRequest(selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .bulk, focus: .physical))
        let plan = AdjustAllocPlan(
            sp: StatBlock(hp: 32, atk: 0, def: 17, spa: 0, spd: 17, spe: 0), totalSp: 66,
            stats: StatBlock(hp: 187, atk: 100, def: 107, spa: 80, spd: 102, spe: 120),
            physicalBulk: 20009, specialBulk: 19074, speedMet: true, goalMet: false, chancePercent: 87.5
        )
        XCTAssertEqual(result, AdjustAllocationResult(
            remaining: 62, maxIndex: plan, minSp: plan,
            unsupported: [UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: "test-item-a")]
        ))
    }

    func testAllocationNullMinSpMapsToNil() async throws {
        let result = try await makeService(transport: RecordingTransport(json: Self.allocationNoGoalJSON)).adjustAllocation(
            AdjustAllocationRequest(selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .bulk, focus: .both))
        XCTAssertNil(result.minSp)
        XCTAssertEqual(result.unsupported, [])
    }

    // MARK: - (4) 技の逆引き

    func testMoveLearnersSendsGetWithPathQueryAndHeaders() async throws {
        let transport = RecordingTransport(json: Self.learnersJSON)
        let species = try await makeService(transport: transport).moveLearners(moveId: "test-move-physical-a", limit: 50, offset: 100)

        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(sent.request.method.rawValue, "GET")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/pokedex/moves/test-move-physical-a/learners")
        XCTAssertEqual(sent.queryItems["limit"], "50")
        XCTAssertEqual(sent.queryItems["offset"], "100")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
        XCTAssertEqual(species, [
            SpeciesSummary(key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストモンいち", types: [.fire]),
            SpeciesSummary(key: "9002-000", dexNo: 9002, form: 0, nameJa: "テストモンに", types: [.water, .flying]),
        ])
    }

    func testMoveLearnersEmptyArrayMapsToEmpty() async throws {
        let species = try await makeService(transport: RecordingTransport(json: "[]")).moveLearners(moveId: "m", limit: 50, offset: 0)
        XCTAssertEqual(species, [])
    }

    func testMoveLearnersNotFoundKeepsCode() async throws {
        let service = try makeService(transport: RecordingTransport(status: 404, json: #"{"code":"not_found","message":"move not found"}"#))
        let error = await assertThrowsPokeCalcError("404") { try await service.moveLearners(moveId: "m", limit: 50, offset: 0) }
        XCTAssertEqual(error?.code, "not_found")
    }

    // MARK: - (5) エラーの写像(4本の POST と GET で同じ)

    private typealias Call = @Sendable (APIPokeCalcService) async throws -> Void

    private var allOperations: [(name: String, call: Call)] {
        let search = AdjustSearchRequest(format: .single, attacker: Self.own, defender: Self.opponent, moveId: "m", hits: 1)
        return [
            ("indices", { _ = try await $0.adjustIndices(AdjustIndicesRequest(individual: Self.own)) }),
            ("ko", { _ = try await $0.adjustMinSpToKo(search) }),
            ("survive", { _ = try await $0.adjustMinSpToSurvive(search) }),
            ("allocation", {
                _ = try await $0.adjustAllocation(AdjustAllocationRequest(
                    selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .bulk, focus: .both))
            }),
            ("learners", { _ = try await $0.moveLearners(moveId: "m", limit: 50, offset: 0) }),
        ]
    }

    func testErrorResponsesKeepServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_input"), (500, "type_chart_missing"), (503, "master_unavailable"), (503, "upstream_unavailable"),
        ]
        for operation in allOperations {
            for item in cases {
                let json = #"{"code":"\#(item.code)","message":"internal detail"}"#
                let service = try makeService(transport: RecordingTransport(status: item.status, json: json))
                let error = await assertThrowsPokeCalcError("\(operation.name) \(item.status)") { try await operation.call(service) }
                XCTAssertEqual(error?.code, item.code, "\(operation.name) \(item.status)")
            }
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        for operation in allOperations {
            let service = try makeService(transport: FailingTransport())
            let error = await assertThrowsPokeCalcError(operation.name) { try await operation.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.transport, operation.name)
        }
    }

    func testMalformedResponseMapsToDecodeError() async throws {
        for operation in allOperations {
            let service = try makeService(transport: RecordingTransport(json: #"{"unexpected":true}"#))
            let error = await assertThrowsPokeCalcError(operation.name) { try await operation.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.decode, operation.name)
        }
    }

    /// 取り消しは `PokeCalcError` に包まず `CancellationError` のまま投げる(ViewModel がエラー表示しないため)。
    func testCancellationIsRethrownAsCancellationError() async throws {
        let transport = RecordingTransport(responder: { _ in throw CancellationError() })
        let service = try makeService(transport: transport)
        for operation in allOperations {
            do {
                try await operation.call(service)
                XCTFail("\(operation.name): エラーにならなかった")
            } catch is CancellationError {
                // 期待どおり
            } catch {
                XCTFail("\(operation.name): CancellationError ではない \(error)")
            }
        }
    }
}
