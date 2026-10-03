import Foundation
import OpenAPIRuntime
import PokeCalcBalanceAPI
import XCTest

@testable import PokeCalcCore

// APIBalanceService 第3段(ADR-0415 §8): threats・recommendations・move-range の HTTP ↔ ドメインの写像。
// 期待値は services/balance/api/openapi.yaml のパス・ヘッダー・スキーマから書く。フィクスチャは架空(ADR-0002)。
//
// 決めた形:
// - `threats(members:threats:)`: POST /api/balance/v1/team-balance/threats。members・threats とも `pokemonId`・`moveIds`(常に送る)・
//   `abilityId`(あれば)。
// - `recommendations(members:limit:)`: POST .../recommendations。members は同じ形。`limit` は nil なら送らない(既定 10 は契約側)。
// - `moveRange(moveIds:)`: POST /api/balance/v1/move-range/analyze。`moveIds` だけ(ポケモンは送らない)。
// - 倍率は応答の文字列のまま。`incoming`/`outgoing` の null は nil。エラーは `PokeCalcError(code:)`、通信失敗は transport、本文不正は decode。
final class APIBalanceServiceStage3Tests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIBalanceService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIBalanceService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private let members = [
        BalanceMemberInput(pokemonId: "9001-000", abilityId: "ability-9001", moveIds: ["move-9001"]),
        BalanceMemberInput(pokemonId: "9002-000", abilityId: nil, moveIds: []),
    ]
    private let threats = [BalanceMemberInput(pokemonId: "9003-000", abilityId: nil, moveIds: ["move-9002"])]

    private static let typeIds = [
        "normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground",
        "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy",
    ]

    // MARK: - threats

    private static let threatsJSON = """
    {"threats":[{"pokemonId":"9003-000","abilityId":"ability-9003","attackTypes":["fire","water"],
      "matchups":[
        {"pokemonId":"9001-000","incoming":"2","outgoing":"1/2","safe":false,"superEffective":false},
        {"pokemonId":"9002-000","incoming":null,"outgoing":"2","safe":false,"superEffective":true}
      ],"safeMembers":0,"superEffectiveMembers":1}]}
    """

    func testThreatsSendsMembersAndThreatsWithMoveIdsAndOptionalAbility() async throws {
        let transport = RecordingTransport(json: Self.threatsJSON)
        let service = try makeService(transport: transport)

        _ = try await service.threats(members: members, threats: threats)

        let request = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(request.request.method, .post)
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/threats")
        XCTAssertEqual(request.header("X-Device-Id"), deviceID)
        XCTAssertEqual(request.header("X-Session-Id"), sessionID)
        let sent = try request.jsonBody()
        let sentMembers = try XCTUnwrap(sent["members"] as? [[String: Any]])
        XCTAssertEqual(sentMembers.count, 2)
        XCTAssertEqual(sentMembers[0]["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(sentMembers[0]["abilityId"] as? String, "ability-9001")
        XCTAssertEqual(sentMembers[0]["moveIds"] as? [String], ["move-9001"])
        XCTAssertNil(sentMembers[1]["abilityId"])
        XCTAssertEqual(sentMembers[1]["moveIds"] as? [String], [], "moveIds は必須なので空でも送る")
        let sentThreats = try XCTUnwrap(sent["threats"] as? [[String: Any]])
        XCTAssertEqual(sentThreats.count, 1)
        XCTAssertEqual(sentThreats[0]["pokemonId"] as? String, "9003-000")
        XCTAssertEqual(sentThreats[0]["moveIds"] as? [String], ["move-9002"])
    }

    func testThreatsMapsMatchupsKeepingFractionsAndNulls() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.threatsJSON))

        let result = try await service.threats(members: members, threats: threats)

        XCTAssertEqual(result.threats.count, 1)
        let threat = result.threats[0]
        XCTAssertEqual(threat.pokemonId, "9003-000")
        XCTAssertEqual(threat.abilityId, "ability-9003")
        XCTAssertEqual(threat.attackTypes, [.fire, .water])
        XCTAssertEqual(threat.safeMembers, 0)
        XCTAssertEqual(threat.superEffectiveMembers, 1)
        XCTAssertEqual(threat.matchups.count, 2)
        XCTAssertEqual(threat.matchups[0].incoming, "2")
        XCTAssertEqual(threat.matchups[0].outgoing, "1/2")
        XCTAssertFalse(threat.matchups[0].safe)
        XCTAssertNil(threat.matchups[1].incoming, "攻撃技が無ければ null → nil")
        XCTAssertEqual(threat.matchups[1].outgoing, "2")
        XCTAssertTrue(threat.matchups[1].superEffective)
    }

    // MARK: - recommendations

    private static let recommendationsJSON = """
    {"defenseHoles":["fire","water"],"offenseHoles":["ghost"],
     "candidates":[{"types":["grass","poison"],"defenseCovered":["water"],"offenseCovered":["ghost"],"weaknesses":4,
       "pokemon":[{"pokemonId":"9001-000","nameJa":"テストアルファ","types":["grass","poison"],"exactMatch":true},
                  {"pokemonId":"9004-000","types":["grass"],"exactMatch":false}]}],
     "abilityOptions":[{"attackType":"fire","pokemon":[
        {"pokemonId":"9002-000","nameJa":"テストベータ","abilityId":"ability-9002","multiplier":"1/2"}]},
        {"attackType":"water","pokemon":[]}]}
    """

    func testRecommendationsSendsMembersAndOmitsLimitWhenNil() async throws {
        let transport = RecordingTransport(json: Self.recommendationsJSON)
        let service = try makeService(transport: transport)

        _ = try await service.recommendations(members: members, limit: nil)

        let request = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/recommendations")
        XCTAssertEqual(request.header("X-Device-Id"), deviceID)
        XCTAssertEqual(request.header("X-Session-Id"), sessionID)
        let sent = try request.jsonBody()
        let sentMembers = try XCTUnwrap(sent["members"] as? [[String: Any]])
        XCTAssertEqual(sentMembers.count, 2)
        XCTAssertEqual(sentMembers[0]["abilityId"] as? String, "ability-9001")
        XCTAssertEqual(sentMembers[0]["moveIds"] as? [String], ["move-9001"])
        XCTAssertNil(sent["limit"], "limit が nil なら送らない(契約の既定 10)")
    }

    func testRecommendationsSendsLimitWhenGiven() async throws {
        let transport = RecordingTransport(json: Self.recommendationsJSON)
        let service = try makeService(transport: transport)

        _ = try await service.recommendations(members: members, limit: 5)

        let sent = try XCTUnwrap(transport.requests.first).jsonBody()
        XCTAssertEqual(sent["limit"] as? Int, 5)
    }

    func testRecommendationsMapsHolesCandidatesAndAbilityOptions() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.recommendationsJSON))

        let result = try await service.recommendations(members: members, limit: nil)

        XCTAssertEqual(result.defenseHoles, [.fire, .water])
        XCTAssertEqual(result.offenseHoles, [.ghost])
        XCTAssertEqual(result.candidates.count, 1)
        let candidate = result.candidates[0]
        XCTAssertEqual(candidate.types, [.grass, .poison])
        XCTAssertEqual(candidate.defenseCovered, [.water])
        XCTAssertEqual(candidate.offenseCovered, [.ghost])
        XCTAssertEqual(candidate.weaknesses, 4)
        XCTAssertEqual(candidate.pokemon.count, 2)
        XCTAssertEqual(candidate.pokemon[0].pokemonId, "9001-000")
        XCTAssertEqual(candidate.pokemon[0].nameJa, "テストアルファ")
        XCTAssertTrue(candidate.pokemon[0].exactMatch)
        XCTAssertNil(candidate.pokemon[1].nameJa, "nameJa が無ければ nil(画面は ID を出す)")
        XCTAssertFalse(candidate.pokemon[1].exactMatch)
        XCTAssertEqual(result.abilityOptions.count, 2)
        XCTAssertEqual(result.abilityOptions[0].attackType, .fire)
        XCTAssertEqual(result.abilityOptions[0].pokemon.first?.abilityId, "ability-9002")
        XCTAssertEqual(result.abilityOptions[0].pokemon.first?.multiplier, "1/2")
        XCTAssertEqual(result.abilityOptions[0].pokemon.first?.nameJa, "テストベータ")
        XCTAssertEqual(result.abilityOptions[1].pokemon.count, 0)
    }

    // MARK: - move-range

    private static func typeChartJSON(fire: String) -> String {
        let entries = typeIds.map { type -> String in
            type == "fire"
                ? #"{"defenseType":"fire",\#(fire)}"#
                : #"{"defenseType":"\#(type)","bestMultiplier":"1","effective":true,"superEffective":false}"#
        }
        return "[" + entries.joined(separator: ",") + "]"
    }

    private static let moveRangeJSON = """
    {"attackTypes":["water"],
     "typeChart":\(typeChartJSON(fire: #""bestMultiplier":"2","effective":true,"superEffective":true"#)),
     "walledBy":[{"pokemonId":"9001-000","nameJa":"テストアルファ","types":["water","grass"],"bestMultiplier":"1/4"}],
     "walledByAbility":[{"pokemonId":"9002-000","abilityId":"ability-9002","bestMultiplier":"0"}]}
    """

    func testMoveRangeSendsOnlyMoveIds() async throws {
        let transport = RecordingTransport(json: Self.moveRangeJSON)
        let service = try makeService(transport: transport)

        _ = try await service.moveRange(moveIds: ["move-9001", "move-9002"])

        let request = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/move-range/analyze")
        XCTAssertEqual(request.header("X-Device-Id"), deviceID)
        XCTAssertEqual(request.header("X-Session-Id"), sessionID)
        let sent = try request.jsonBody()
        XCTAssertEqual(sent["moveIds"] as? [String], ["move-9001", "move-9002"])
        XCTAssertEqual(Set(sent.keys), ["moveIds"], "ポケモンは送らない(技構成だけで決まる。ADR-0404 §1)")
    }

    func testMoveRangeMapsTypeChartAndWalls() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.moveRangeJSON))

        let result = try await service.moveRange(moveIds: ["move-9001"])

        XCTAssertEqual(result.attackTypes, [.water])
        XCTAssertEqual(result.typeChart.map(\.defenseType), PokeType.allCases, "契約の正準順 18 タイプ")
        let fire = try XCTUnwrap(result.typeChart.first { $0.defenseType == .fire })
        XCTAssertEqual(fire.bestMultiplier, .double)
        XCTAssertTrue(fire.effective)
        XCTAssertTrue(fire.superEffective)
        XCTAssertEqual(result.walledBy.count, 1)
        XCTAssertEqual(result.walledBy[0].pokemonId, "9001-000")
        XCTAssertEqual(result.walledBy[0].nameJa, "テストアルファ")
        XCTAssertEqual(result.walledBy[0].types, [.water, .grass])
        XCTAssertEqual(result.walledBy[0].bestMultiplier, "1/4")
        XCTAssertEqual(result.walledByAbility.count, 1)
        XCTAssertEqual(result.walledByAbility[0].abilityId, "ability-9002")
        XCTAssertNil(result.walledByAbility[0].nameJa)
        XCTAssertEqual(result.walledByAbility[0].bestMultiplier, "0")
    }

    // MARK: - エラー・通信失敗・キャンセル(3 操作とも)

    func testErrorStatusesMapToPokeCalcErrorWithServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_request"), (413, "request_too_large"), (422, "unknown_move"),
            (503, "overloaded"), (503, "master_unavailable"), (500, "internal_error"),
        ]
        for testCase in cases {
            let body = #"{"code":"\#(testCase.code)","message":"internal english message"}"#
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: body))
            let tag = "\(testCase.status) \(testCase.code)"

            let threatsError = await assertThrowsPokeCalcError("threats \(tag)") {
                try await service.threats(members: members, threats: threats)
            }
            XCTAssertEqual(threatsError?.code, testCase.code, "threats \(tag)")
            XCTAssertEqual(threatsError?.message, "internal english message")
            let recommendationsError = await assertThrowsPokeCalcError("recommendations \(tag)") {
                try await service.recommendations(members: members, limit: nil)
            }
            XCTAssertEqual(recommendationsError?.code, testCase.code, "recommendations \(tag)")
            let moveRangeError = await assertThrowsPokeCalcError("moveRange \(tag)") {
                try await service.moveRange(moveIds: ["move-9001"])
            }
            XCTAssertEqual(moveRangeError?.code, testCase.code, "moveRange \(tag)")
        }
    }

    func testTransportFailureIsTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let threatsError = await assertThrowsPokeCalcError("threats") {
            try await service.threats(members: members, threats: threats)
        }
        XCTAssertEqual(threatsError?.code, PokeCalcError.Code.transport)
        let recommendationsError = await assertThrowsPokeCalcError("recommendations") {
            try await service.recommendations(members: members, limit: nil)
        }
        XCTAssertEqual(recommendationsError?.code, PokeCalcError.Code.transport)
        let moveRangeError = await assertThrowsPokeCalcError("moveRange") {
            try await service.moveRange(moveIds: ["move-9001"])
        }
        XCTAssertEqual(moveRangeError?.code, PokeCalcError.Code.transport)
    }

    func testMalformedSuccessBodyIsDecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(json: #"{"threats":"x"}"#))
        let error = await assertThrowsPokeCalcError("decode") {
            try await service.threats(members: members, threats: threats)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    // MARK: - 接続できない構成(モック)ではデータを偽らない

    func testUnavailableBalanceServiceFailsForStage3Operations() async {
        let service = UnavailableBalanceService()
        let threatsError = await assertThrowsPokeCalcError("threats") {
            try await service.threats(members: members, threats: threats)
        }
        XCTAssertEqual(threatsError?.code, "balance_unavailable")
        let recommendationsError = await assertThrowsPokeCalcError("recommendations") {
            try await service.recommendations(members: members, limit: nil)
        }
        XCTAssertEqual(recommendationsError?.code, "balance_unavailable")
        let moveRangeError = await assertThrowsPokeCalcError("moveRange") {
            try await service.moveRange(moveIds: ["move-9001"])
        }
        XCTAssertEqual(moveRangeError?.code, "balance_unavailable")
    }
}
