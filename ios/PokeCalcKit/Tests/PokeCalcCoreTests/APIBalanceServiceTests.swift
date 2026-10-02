import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcBalanceAPI
import XCTest

@testable import PokeCalcCore

/// `APIBalanceService`(P6-26。ADR-0505 §3): 生成された `Client`(PokeCalcBalanceAPI)と偽の transport で、
/// ドメイン ↔ HTTP(services/balance/api/openapi.yaml)の写像を検証する。期待値は契約のパス・スキーマから書く。
/// フィクスチャは架空(pokemonId は 9xxx-xxx / 技・特性 ID は stub-)。
final class APIBalanceServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIBalanceService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIBalanceService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - 架空のフィクスチャ

    private static let types = ["normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground",
                                "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"]

    private static let analyzeRequest = BalanceAnalyzeRequest(
        members: [BalanceAnalyzeMember(pokemonId: "9001-000"), BalanceAnalyzeMember(pokemonId: "9002-000", abilityId: "stub-ability")])
    private static let coverageRequest = BalanceCoverageRequest(
        members: [BalanceCoverageMember(pokemonId: "9001-000", moveIds: ["stub-move-a", "stub-move-b"]),
                  BalanceCoverageMember(pokemonId: "9002-000", moveIds: [])])

    /// 18 攻撃タイプの防御。`overrides` の攻撃タイプだけ値を差し替える(それ以外は ×1 等倍・type)。
    private static func defenseJSON(overrides: [String: String] = [:]) -> String {
        let rows = types.map { type in
            overrides[type]
                ?? #"{"attackType":"\#(type)","multiplier":"1","category":"neutral","source":"type","effect":"none"}"#
        }
        return "[" + rows.joined(separator: ",") + "]"
    }

    private static func summaryJSON(weak: Int = 0) -> String {
        let rows = types.map { type in
            #"{"attackType":"\#(type)","weak":\#(weak),"quadWeak":0,"resist":0,"immune":0,"neutral":2}"#
        }
        return "[" + rows.joined(separator: ",") + "]"
    }

    /// 2 体。メンバー 0 は normal が ×4(quad_weak)・メンバー 1 は特性ありで fire を特性が無効(ability・immune)・water が ×3/4(ability・multiplier)。
    private static var analyzeResponseJSON: String {
        let first = #"""
            {"pokemonId":"9001-000","types":["normal"],"defense":\#(defenseJSON(overrides: [
                "normal": #"{"attackType":"normal","multiplier":"4","category":"quad_weak","source":"type","effect":"none"}"#]))}
            """#
        let second = #"""
            {"pokemonId":"9002-000","abilityId":"stub-ability","types":["fire","water"],"defense":\#(defenseJSON(overrides: [
                "fire": #"{"attackType":"fire","multiplier":"0","category":"immune","source":"ability","effect":"immune"}"#,
                "water": #"{"attackType":"water","multiplier":"3/4","category":"resist","source":"ability","effect":"multiplier"}"#]))}
            """#
        return #"{"members":[\#(first),\#(second)],"teamSummary":\#(summaryJSON(weak: 1))}"#
    }

    private static func coverageEntries(_ best: String?, effective: Bool, superEffective: Bool) -> String {
        let value = best.map { #""\#($0)""# } ?? "null"
        let rows = types.map { type in
            #"{"defenseType":"\#(type)","bestMultiplier":\#(value),"effective":\#(effective),"superEffective":\#(superEffective)}"#
        }
        return "[" + rows.joined(separator: ",") + "]"
    }

    /// メンバー 0 は ×2(抜群)・メンバー 1 は攻撃技なし(bestMultiplier が null)。
    private static var coverageResponseJSON: String {
        let team = types.map { type in
            #"{"defenseType":"\#(type)","bestMultiplier":"2","effectiveMembers":1,"superEffectiveMembers":1}"#
        }
        return #"""
            {"members":[
              {"pokemonId":"9001-000","moveIds":["stub-move-a","stub-move-b"],"attackTypes":["fire","water"],"coverage":\#(coverageEntries("2", effective: true, superEffective: true))},
              {"pokemonId":"9002-000","moveIds":[],"attackTypes":[],"coverage":\#(coverageEntries(nil, effective: false, superEffective: false))}],
             "teamCoverage":[\#(team.joined(separator: ","))]}
            """#
    }

    private func errorJSON(_ code: String) -> String { #"{"code":"\#(code)","message":"english message from server"}"# }

    private func assertIdentityHeaders(_ sent: RecordingTransport.Recorded, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID, file: file, line: line)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID, file: file, line: line)
    }

    private func sentAnalyze(_ request: BalanceAnalyzeRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.analyzeResponseJSON)
        _ = try await makeService(transport: transport).analyzeTeamBalance(request)
        XCTAssertEqual(transport.requests.count, 1)
        let first = try XCTUnwrap(transport.requests.first)
        return (first, try first.jsonBody())
    }

    private func sentCoverage(_ request: BalanceCoverageRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.coverageResponseJSON)
        _ = try await makeService(transport: transport).analyzeTeamCoverage(request)
        XCTAssertEqual(transport.requests.count, 1)
        let first = try XCTUnwrap(transport.requests.first)
        return (first, try first.jsonBody())
    }

    // MARK: - パス・ヘッダー・ボディ

    func testAnalyzeSendsPostWithPathAndHeaders() async throws {
        let (request, _) = try await sentAnalyze(Self.analyzeRequest)
        XCTAssertEqual(request.request.method.rawValue, "POST")
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/analyze")
        XCTAssertTrue(request.queryItems.isEmpty)
        XCTAssertEqual(request.header("Content-Type")?.hasPrefix("application/json"), true)
        assertIdentityHeaders(request)
    }

    func testCoverageSendsPostWithPathAndHeaders() async throws {
        let (request, _) = try await sentCoverage(Self.coverageRequest)
        XCTAssertEqual(request.request.method.rawValue, "POST")
        XCTAssertEqual(request.pathWithoutQuery, "/api/balance/v1/team-balance/coverage")
        XCTAssertTrue(request.queryItems.isEmpty)
        assertIdentityHeaders(request)
    }

    /// `abilityId` は nil なら欄ごと送らない(`null` を送らない)。メンバーの順は保つ。
    func testAnalyzeBodyOmitsAbsentAbilityAndKeepsMemberOrder() async throws {
        let (request, body) = try await sentAnalyze(Self.analyzeRequest)
        XCTAssertEqual(Set(body.keys), ["members"])
        let members = try XCTUnwrap(body["members"] as? [[String: Any]])
        XCTAssertEqual(members.count, 2)
        XCTAssertEqual(Set(members[0].keys), ["pokemonId"], "特性なしは abilityId の欄ごと送らない")
        XCTAssertEqual(members[0]["pokemonId"] as? String, "9001-000")
        XCTAssertEqual(Set(members[1].keys), ["pokemonId", "abilityId"])
        XCTAssertEqual(members[1]["abilityId"] as? String, "stub-ability")
        XCTAssertFalse(String(decoding: try XCTUnwrap(request.body), as: UTF8.self).contains("null"), "null を明示的に送らない")
    }

    /// coverage の `moveIds` は必須(技が無いメンバーも空配列で載せる)。順を保つ。
    func testCoverageBodyAlwaysCarriesMoveIdsEvenWhenEmpty() async throws {
        let (_, body) = try await sentCoverage(Self.coverageRequest)
        XCTAssertEqual(Set(body.keys), ["members"])
        let members = try XCTUnwrap(body["members"] as? [[String: Any]])
        XCTAssertEqual(members.count, 2)
        XCTAssertEqual(Set(members[0].keys), ["pokemonId", "moveIds"])
        XCTAssertEqual(members[0]["moveIds"] as? [String], ["stub-move-a", "stub-move-b"], "技の順を保つ")
        XCTAssertEqual(members[1]["moveIds"] as? [String], [], "技が無いメンバーも moveIds を載せる(欄を省かない)")
    }

    // MARK: - 応答の写像(analyze)

    func testAnalyzeResponseKeepsOrderAndValuesAsTheServerSent() async throws {
        let response = try await makeService(transport: RecordingTransport(json: Self.analyzeResponseJSON)).analyzeTeamBalance(Self.analyzeRequest)
        XCTAssertEqual(response.members.map(\.pokemonId), ["9001-000", "9002-000"], "メンバーは要求の順のまま")
        XCTAssertNil(response.members[0].abilityId)
        XCTAssertEqual(response.members[1].abilityId, "stub-ability")
        XCTAssertEqual(response.members[0].types, [.normal])
        XCTAssertEqual(response.members[1].types, [.fire, .water], "タイプは応答の順のまま")
        XCTAssertEqual(response.members[0].defense.map(\.attackType), PokeType.allCases, "防御は 18 攻撃タイプの正準順のまま")
        XCTAssertEqual(response.teamSummary.map(\.attackType), PokeType.allCases)
    }

    /// 倍率は文字列のまま(`"4"`・`"3/4"` を数値に変換しない)・分類・出どころ・効果をそのまま写す。
    func testAnalyzeEntriesKeepMultiplierStringCategorySourceAndEffect() async throws {
        let response = try await makeService(transport: RecordingTransport(json: Self.analyzeResponseJSON)).analyzeTeamBalance(Self.analyzeRequest)
        XCTAssertEqual(
            response.members[0].defense[0],
            BalanceDefenseEntry(attackType: .normal, multiplier: "4", category: .quadWeak, source: .type, effect: .none))
        XCTAssertEqual(
            response.members[1].defense[1],
            BalanceDefenseEntry(attackType: .fire, multiplier: "0", category: .immune, source: .ability, effect: .immune))
        XCTAssertEqual(
            response.members[1].defense[2],
            BalanceDefenseEntry(attackType: .water, multiplier: "3/4", category: .resist, source: .ability, effect: .multiplier),
            "既約分数の文字列をそのまま運ぶ")
    }

    func testAnalyzeTeamSummaryKeepsTheCounts() async throws {
        let response = try await makeService(transport: RecordingTransport(json: Self.analyzeResponseJSON)).analyzeTeamBalance(Self.analyzeRequest)
        XCTAssertEqual(
            response.teamSummary[0], BalanceTeamSummaryEntry(attackType: .normal, weak: 1, quadWeak: 0, resist: 0, immune: 0, neutral: 2))
    }

    // MARK: - 応答の写像(coverage)

    func testCoverageResponseKeepsOrderAndValues() async throws {
        let response = try await makeService(transport: RecordingTransport(json: Self.coverageResponseJSON)).analyzeTeamCoverage(Self.coverageRequest)
        XCTAssertEqual(response.members.map(\.pokemonId), ["9001-000", "9002-000"])
        XCTAssertEqual(response.members[0].moveIds, ["stub-move-a", "stub-move-b"])
        XCTAssertEqual(response.members[0].attackTypes, [.fire, .water], "攻撃タイプは応答の順(正準順)のまま")
        XCTAssertEqual(response.members[0].coverage.map(\.defenseType), PokeType.allCases)
        XCTAssertEqual(
            response.members[0].coverage[0],
            BalanceDefenseCoverageEntry(defenseType: .normal, bestMultiplier: .double, effective: true, superEffective: true))
        XCTAssertEqual(
            response.teamCoverage[0],
            BalanceTeamCoverageEntry(defenseType: .normal, bestMultiplier: .double, effectiveMembers: 1, superEffectiveMembers: 1))
    }

    /// 攻撃技が無いメンバーは `bestMultiplier` が null(契約上 nullable)。`nil` のまま写す(0 や等倍に丸めない)。
    func testCoverageNullBestMultiplierStaysNil() async throws {
        let response = try await makeService(transport: RecordingTransport(json: Self.coverageResponseJSON)).analyzeTeamCoverage(Self.coverageRequest)
        XCTAssertEqual(response.members[1].attackTypes, [])
        XCTAssertTrue(response.members[1].coverage.allSatisfy { $0.bestMultiplier == nil && !$0.effective && !$0.superEffective })
    }

    func testEveryCoverageMultiplierIsMapped() async throws {
        for (raw, expected) in [("0", BalanceCoverageMultiplier.zero), ("1/2", .half), ("1", .neutral), ("2", .double)] {
            let json = Self.coverageResponseJSON.replacingOccurrences(of: #""bestMultiplier":"2","effectiveMembers""#, with: #""bestMultiplier":"\#(raw)","effectiveMembers""#)
            let response = try await makeService(transport: RecordingTransport(json: json)).analyzeTeamCoverage(Self.coverageRequest)
            XCTAssertEqual(response.teamCoverage[0].bestMultiplier, expected, raw)
        }
    }

    // MARK: - エラー(2 つの操作とも同じ分類)

    /// 契約の HTTP エラーはすべて、`code` と `message` をそのまま運ぶ(英語の message を画面に出さないのは画面側)。
    func testDocumentedErrorStatusesKeepTheServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_request"), (400, "missing_request_context"), (413, "request_too_large"), (422, "unknown_pokemon"),
            (422, "unknown_move"), (422, "unknown_ability"), (500, "internal_error"), (503, "master_unavailable"), (503, "overloaded"),
        ]
        for entry in cases {
            let transport = RecordingTransport(status: entry.status, json: errorJSON(entry.code))
            let service = try makeService(transport: transport)
            let analyzeError = await assertThrowsPokeCalcError("analyze \(entry.status) \(entry.code)") {
                try await service.analyzeTeamBalance(Self.analyzeRequest)
            }
            XCTAssertEqual(analyzeError?.code, entry.code, "analyze \(entry.status)")
            XCTAssertEqual(analyzeError?.message, "english message from server")
            let coverageError = await assertThrowsPokeCalcError("coverage \(entry.status) \(entry.code)") {
                try await service.analyzeTeamCoverage(Self.coverageRequest)
            }
            XCTAssertEqual(coverageError?.code, entry.code, "coverage \(entry.status)")
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let analyzeError = await assertThrowsPokeCalcError("通信不能") { try await service.analyzeTeamBalance(Self.analyzeRequest) }
        XCTAssertEqual(analyzeError?.code, PokeCalcError.Code.transport)
        let coverageError = await assertThrowsPokeCalcError("通信不能") { try await service.analyzeTeamCoverage(Self.coverageRequest) }
        XCTAssertEqual(coverageError?.code, PokeCalcError.Code.transport)
    }

    func testUnreadableSuccessBodyMapsToDecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(status: 200, json: #"{"unexpected":true}"#))
        let analyzeError = await assertThrowsPokeCalcError("読めない 200") { try await service.analyzeTeamBalance(Self.analyzeRequest) }
        XCTAssertEqual(analyzeError?.code, PokeCalcError.Code.decode)
        let coverageError = await assertThrowsPokeCalcError("読めない 200") { try await service.analyzeTeamCoverage(Self.coverageRequest) }
        XCTAssertEqual(coverageError?.code, PokeCalcError.Code.decode)
    }

    /// 契約に無いステータス(`default` 応答が無いので生成クライアントは undocumented で返す)。
    /// 本文が `{code,message}` の JSON ならその code(404/405 の `not_found`。ADR-0802)。読めなければ専用のコード。
    func testUndocumentedStatusWithErrorBodyKeepsItsCode() async throws {
        let service = try makeService(transport: RecordingTransport(status: 404, json: errorJSON("not_found")))
        let error = await assertThrowsPokeCalcError("404") { try await service.analyzeTeamBalance(Self.analyzeRequest) }
        XCTAssertEqual(error?.code, "not_found")
    }

    func testUndocumentedStatusWithUnreadableBodyMapsToUnexpectedStatus() async throws {
        let service = try makeService(transport: RecordingTransport(status: 502, json: "<html>Bad Gateway</html>"))
        let error = await assertThrowsPokeCalcError("502") { try await service.analyzeTeamCoverage(Self.coverageRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.unexpectedStatus)
    }

    /// 契約の `ErrorCode` に無い code は、契約の型では読めない(decode)。契約に `ErrorCode` が増えたときの古いアプリの挙動を固定する
    /// (DECISIONS.md の連絡のとおり、増やす前に相談する)。
    func testUnknownErrorCodeOnADocumentedStatusIsADecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(status: 503, json: errorJSON("some_future_code")))
        let error = await assertThrowsPokeCalcError("未知の code") { try await service.analyzeTeamBalance(Self.analyzeRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    /// タスクのキャンセルは `PokeCalcError` に包まず `CancellationError` のまま(`APIJudgeService` と同じ)。
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
            _ = try await service.analyzeTeamBalance(Self.analyzeRequest)
            XCTFail("エラーにならなかった")
        } catch is CancellationError {
            // 期待どおり
        } catch {
            XCTFail("CancellationError ではない: \(error)")
        }
    }
}
