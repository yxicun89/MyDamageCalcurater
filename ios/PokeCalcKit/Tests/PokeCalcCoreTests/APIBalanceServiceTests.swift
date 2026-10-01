import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcBalanceAPI
import XCTest

@testable import PokeCalcCore

// APIBalanceService(ADR-0415): 生成クライアント(PokeCalcBalanceAPI.Client)と偽の transport で、
// ドメイン(PokeCalcCore の Balance* 型)↔ HTTP(services/balance/api/openapi.yaml)の写像を検証する。
// 期待値は balance の openapi.yaml のパス・ヘッダー・スキーマから書く。フィクスチャは架空(ADR-0002)。
//
// 決めた形:
// - `protocol BalanceService: Sendable { analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis;
//    coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis }`。エラーは `PokeCalcError` に統一。
// - `struct BalanceMemberInput: Equatable, Sendable { pokemonId: String; abilityId: String?; moveIds: [String] }`。
// - `APIBalanceService(client: PokeCalcBalanceAPI.Client, identity: ClientIdentity)` と `init(baseURL: URL, identity:)`。
//   baseURL は gateway(`/api/balance/*` 経由。ADR-0414)。契約のパスは `/api/balance/...` なので baseURL に付け足さない。
// - analyze は `pokemonId`・(あれば)`abilityId` だけを送る(`moveIds` は送らない)。coverage は `pokemonId`・`moveIds` だけを送る
//   (`abilityId` は送らない)。ヘッダーは全操作に `X-Device-Id` / `X-Session-Id`。
// - 倍率は応答の文字列のまま運ぶ(`BalanceDefenseEntry.multiplier: String`)。iOS で分数を計算し直さない。
// - エラー: 400/413/422/503/500 → `PokeCalcError(code: <応答の code>, message: <応答の message>)`、
//   通信失敗 → `PokeCalcError.Code.transport`、200 の本文が壊れている → `PokeCalcError.Code.decode`、
//   Task のキャンセル → `CancellationError`(`PokeCalcError` に包まない。`APIPokeCalcService` と同じ)。
final class APIBalanceServiceTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIBalanceService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIBalanceService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - フィクスチャ(契約の正準タイプ順)

    private static let typeIds = [
        "normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground",
        "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy",
    ]

    /// 18 タイプぶんの防御エントリ JSON。`fire` だけ上書きできる。
    private static func defenseJSON(fire: String = #""multiplier":"1","category":"neutral","source":"type","effect":"none""#) -> String {
        let entries = typeIds.map { type -> String in
            let body = type == "fire" ? fire : #""multiplier":"1","category":"neutral","source":"type","effect":"none""#
            return #"{"attackType":"\#(type)",\#(body)}"#
        }
        return "[" + entries.joined(separator: ",") + "]"
    }

    private static func summaryJSON(fireWeak: Int) -> String {
        let entries = typeIds.map { type -> String in
            let weak = type == "fire" ? fireWeak : 0
            return #"{"attackType":"\#(type)","weak":\#(weak),"quadWeak":0,"resist":0,"immune":0,"neutral":\#(2 - weak)}"#
        }
        return "[" + entries.joined(separator: ",") + "]"
    }

    /// メンバー2体の analyze 応答。1体目は特性(ability-9001)付きで fire に ×3/2 の特性補正。
    private static let analyzeJSON = """
    {"members":[
      {"pokemonId":"9001-000","abilityId":"ability-9001","types":["grass","poison"],
       "defense":\(defenseJSON(fire: #""multiplier":"3/2","category":"weak","source":"ability","effect":"multiplier""#))},
      {"pokemonId":"9002-000","types":["water"],"defense":\(defenseJSON())}
    ],"teamSummary":\(summaryJSON(fireWeak: 1))}
    """

    private static func coverageEntriesJSON(fire: String) -> String {
        let entries = typeIds.map { type -> String in
            type == "fire"
                ? #"{"defenseType":"fire",\#(fire)}"#
                : #"{"defenseType":"\#(type)","bestMultiplier":null,"effective":false,"superEffective":false}"#
        }
        return "[" + entries.joined(separator: ",") + "]"
    }

    private static let coverageJSON = """
    {"members":[
      {"pokemonId":"9001-000","moveIds":["move-9001"],"attackTypes":["water"],
       "coverage":\(coverageEntriesJSON(fire: #""bestMultiplier":"2","effective":true,"superEffective":true"#))},
      {"pokemonId":"9002-000","moveIds":[],"attackTypes":[],
       "coverage":\(coverageEntriesJSON(fire: #""bestMultiplier":null,"effective":false,"superEffective":false"#))}
    ],"teamCoverage":[
      \(typeIds.map { type -> String in
        type == "fire"
            ? #"{"defenseType":"fire","bestMultiplier":"2","effectiveMembers":1,"superEffectiveMembers":1}"#
            : #"{"defenseType":"\#(type)","bestMultiplier":null,"effectiveMembers":0,"superEffectiveMembers":0}"#
      }.joined(separator: ","))
    ]}
    """

    private static let errorJSON: (String, String) -> String = { code, message in
        #"{"code":"\#(code)","message":"\#(message)"}"#
    }

    private let members = [
        BalanceMemberInput(pokemonId: "9001-000", abilityId: "ability-9001", moveIds: ["move-9001"]),
        BalanceMemberInput(pokemonId: "9002-000", abilityId: nil, moveIds: []),
    ]

    // MARK: - analyze: リクエスト

    func testAnalyzeSendsPostWithHeadersAndOnlyPokemonAndAbility() async throws {
        let transport = RecordingTransport(json: Self.analyzeJSON)
        let service = try makeService(transport: transport)

        _ = try await service.analyze(members: members)

        let request = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(request.request.method, .post)
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/analyze")
        XCTAssertEqual(request.header("X-Device-Id"), deviceID)
        XCTAssertEqual(request.header("X-Session-Id"), sessionID)

        let sent = try request.jsonBody()
        let sentMembers = try XCTUnwrap(sent["members"] as? [[String: Any]])
        XCTAssertEqual(sentMembers.count, 2)
        XCTAssertEqual(sentMembers[0]["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(sentMembers[0]["abilityId"] as? String, "ability-9001")
        XCTAssertNil(sentMembers[0]["moveIds"], "analyze には技を送らない(契約の AnalyzeRequestMember に無い)")
        XCTAssertEqual(sentMembers[1]["pokemonId"] as? String, "9002-000")
        XCTAssertNil(sentMembers[1]["abilityId"], "特性なしのメンバーは abilityId のキー自体を送らない")
    }

    // MARK: - analyze: 応答の写像

    func testAnalyzeMapsMembersAndTeamSummary() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.analyzeJSON))

        let analysis = try await service.analyze(members: members)

        XCTAssertEqual(analysis.members.map(\.pokemonId), ["9001-000", "9002-000"], "要求順")
        let first = analysis.members[0]
        XCTAssertEqual(first.abilityId, "ability-9001")
        XCTAssertEqual(first.types, [.grass, .poison])
        XCTAssertEqual(first.defense.map(\.attackType), PokeType.allCases, "契約の正準順(normal ... fairy)のまま")
        let fire = try XCTUnwrap(first.defense.first { $0.attackType == .fire })
        XCTAssertEqual(fire.multiplier, "3/2", "倍率は応答の文字列のまま(計算し直さない)")
        XCTAssertEqual(fire.category, .weak)
        XCTAssertEqual(fire.source, .ability)
        XCTAssertEqual(fire.effect, .multiplier)
        let water = try XCTUnwrap(first.defense.first { $0.attackType == .water })
        XCTAssertEqual(water.source, .type)
        XCTAssertEqual(water.effect, BalanceEffectKind.none)

        XCTAssertNil(analysis.members[1].abilityId)
        XCTAssertEqual(analysis.teamSummary.count, 18)
        let summaryFire = try XCTUnwrap(analysis.teamSummary.first { $0.attackType == .fire })
        XCTAssertEqual(summaryFire.weak, 1)
        XCTAssertEqual(summaryFire.neutral, 1)
        XCTAssertEqual(summaryFire.quadWeak, 0)
        XCTAssertEqual(summaryFire.resist, 0)
        XCTAssertEqual(summaryFire.immune, 0)
    }

    // MARK: - coverage

    func testCoverageSendsPokemonAndMoveIdsOnly() async throws {
        let transport = RecordingTransport(json: Self.coverageJSON)
        let service = try makeService(transport: transport)

        _ = try await service.coverage(members: members)

        let request = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(request.request.method, .post)
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/coverage")
        XCTAssertEqual(request.header("X-Device-Id"), deviceID)
        XCTAssertEqual(request.header("X-Session-Id"), sessionID)
        let sentMembers = try XCTUnwrap(try request.jsonBody()["members"] as? [[String: Any]])
        XCTAssertEqual(sentMembers[0]["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(sentMembers[0]["moveIds"] as? [String], ["move-9001"])
        XCTAssertNil(sentMembers[0]["abilityId"], "coverage には特性を送らない(契約の CoverageRequestMember に無い)")
        XCTAssertEqual(sentMembers[1]["moveIds"] as? [String], [], "技なしのメンバーは空配列で送る(moveIds は必須)")
    }

    func testCoverageMapsMembersAndTeamCoverageWithNullAsNil() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.coverageJSON))

        let coverage = try await service.coverage(members: members)

        XCTAssertEqual(coverage.members.map(\.pokemonId), ["9001-000", "9002-000"])
        XCTAssertEqual(coverage.members[0].moveIds, ["move-9001"])
        XCTAssertEqual(coverage.members[0].attackTypes, [.water])
        let fire = try XCTUnwrap(coverage.members[0].coverage.first { $0.defenseType == .fire })
        XCTAssertEqual(fire.bestMultiplier, .double)
        XCTAssertTrue(fire.effective)
        XCTAssertTrue(fire.superEffective)
        let noMoveFire = try XCTUnwrap(coverage.members[1].coverage.first { $0.defenseType == .fire })
        XCTAssertNil(noMoveFire.bestMultiplier, "攻撃技なしの null は nil で運ぶ")
        XCTAssertFalse(noMoveFire.effective)

        XCTAssertEqual(coverage.teamCoverage.map(\.defenseType), PokeType.allCases)
        let teamFire = try XCTUnwrap(coverage.teamCoverage.first { $0.defenseType == .fire })
        XCTAssertEqual(teamFire.bestMultiplier, .double)
        XCTAssertEqual(teamFire.effectiveMembers, 1)
        XCTAssertEqual(teamFire.superEffectiveMembers, 1)
        let teamWater = try XCTUnwrap(coverage.teamCoverage.first { $0.defenseType == .water })
        XCTAssertNil(teamWater.bestMultiplier)
    }

    // MARK: - エラー(契約の 400/413/422/503/500)

    func testErrorStatusesMapToPokeCalcErrorWithServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_request"),
            (413, "request_too_large"),
            (422, "unknown_pokemon"),
            (422, "unknown_move"),
            (503, "master_unavailable"),
            (500, "internal_error"),
        ]
        for testCase in cases {
            let body = Self.errorJSON(testCase.code, "internal english message")
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: body))

            let analyzeError = await assertThrowsPokeCalcError("analyze \(testCase.status)") {
                try await service.analyze(members: members)
            }
            XCTAssertEqual(analyzeError?.code, testCase.code, "analyze \(testCase.status)")
            XCTAssertEqual(analyzeError?.message, "internal english message")

            let coverageError = await assertThrowsPokeCalcError("coverage \(testCase.status)") {
                try await service.coverage(members: members)
            }
            XCTAssertEqual(coverageError?.code, testCase.code, "coverage \(testCase.status)")
        }
    }

    func testTransportFailureIsTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") { try await service.analyze(members: members) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    func testMalformedSuccessBodyIsDecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(json: #"{"members":"not-an-array"}"#))
        let error = await assertThrowsPokeCalcError("decode") { try await service.analyze(members: members) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    // MARK: - 接続できない構成(モック)ではデータを偽らない

    /// モック構成(接続先なし)で balance を呼ぶと `balance_unavailable` で失敗する。架空の相性表を返さない
    /// (Web の ADR-0411 と同じ。ADR-0415 §4)。
    func testUnavailableBalanceServiceFailsWithoutFakeData() async {
        let service = UnavailableBalanceService()
        let analyzeError = await assertThrowsPokeCalcError("unavailable analyze") { try await service.analyze(members: members) }
        XCTAssertEqual(analyzeError?.code, "balance_unavailable")
        let coverageError = await assertThrowsPokeCalcError("unavailable coverage") { try await service.coverage(members: members) }
        XCTAssertEqual(coverageError?.code, "balance_unavailable")
        XCTAssertEqual(BalanceScreenError(analyzeError ?? PokeCalcError(code: "", message: "")).message, "タイプバランスの API に接続できません")
    }
}
