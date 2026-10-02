import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcJudgeAPI
import XCTest

@testable import PokeCalcCore

/// `APIJudgeService`(P6-25。ADR-0504 §3): 生成された `Client`(PokeCalcJudgeAPI)と偽の transport で、
/// ドメイン ↔ HTTP(services/judge/api/openapi.yaml)の写像を検証する。期待値は契約のパス・スキーマから書く。
/// フィクスチャは架空(種族キー・技 ID は 9xxx / stub-)。
final class APIJudgeServiceTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIJudgeService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIJudgeService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - 架空のフィクスチャ

    private static let sp = StatBlock(hp: 1, atk: 2, def: 3, spa: 4, spd: 5, spe: 6)

    private static let minimalAttacker = JudgeIndividual(speciesKey: "9001-000", natureId: "stub-nature-neutral", sp: sp)
    private static let minimalDefender = JudgeDefender(
        individual: JudgeIndividual(speciesKey: "9002-000", natureId: "stub-nature-neutral", sp: sp), moveId: "stub-move-defender")
    private static let minimalRequest = JudgeRequest(
        attacker: minimalAttacker, moveId: "stub-move-attacker", defenders: [minimalDefender])

    private static func koJSON(_ hits: Int, _ guaranteed: Bool, _ percent: String) -> String {
        #"{"hits":\#(hits),"guaranteed":\#(guaranteed),"displayChancePercent":\#(percent)}"#
    }

    private static func markJSON(_ target: String, _ reason: String, _ id: String) -> String {
        #"{"target":"\#(target)","reason":"\#(reason)","id":"\#(id)"}"#
    }

    /// 候補ごとに違う値を持つ応答の1行(行の取り違えを検出する)。
    private static func matchupJSON(
        index: Int, defenderSpeed: Int, attackerKo: String, defenderKo: String, attackerMarks: String = "[]", defenderMarks: String = "[]",
        outspeeds: Bool = true, speedTie: Bool = false, movesFirst: Bool = true, turnOrderTie: Bool = false, defenderPriority: Int = 0
    ) -> String {
        """
        {"defenderIndex":\(index),"outspeeds":\(outspeeds),"speedTie":\(speedTie),"attackerSpeed":200,"defenderSpeed":\(defenderSpeed),
         "attackerMovePriority":0,"defenderMovePriority":\(defenderPriority),"attackerMovesFirst":\(movesFirst),"turnOrderTie":\(turnOrderTie),
         "attackerKo":\(attackerKo),"defenderKo":\(defenderKo),
         "attackerKoUnsupported":\(attackerMarks),"defenderKoUnsupported":\(defenderMarks)}
        """
    }

    private static var responseJSON: String {
        let rows = [
            matchupJSON(
                index: 0, defenderSpeed: 150, attackerKo: koJSON(1, true, "100.0"), defenderKo: koJSON(0, false, "0.0")),
            matchupJSON(
                index: 1, defenderSpeed: 200, attackerKo: koJSON(3, false, "37.5"), defenderKo: koJSON(2, true, "100.0"),
                outspeeds: false, speedTie: true, movesFirst: false, turnOrderTie: true, defenderPriority: 1),
        ]
        return #"{"matchups":[\#(rows.joined(separator: ","))]}"#
    }

    private func errorJSON(_ code: String) -> String { #"{"code":"\#(code)","message":"english message from server defenders[1]"}"# }

    private func assertIdentityHeaders(_ sent: RecordingTransport.Recorded, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID, file: file, line: line)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID, file: file, line: line)
    }

    private func sent(_ request: JudgeRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.responseJSON)
        _ = try await makeService(transport: transport).outspeedAndKo(request)
        XCTAssertEqual(transport.requests.count, 1)
        let first = try XCTUnwrap(transport.requests.first)
        return (first, try first.jsonBody())
    }

    // MARK: - パス・ヘッダー

    func testSendsPostWithPathHeadersAndJSONContentType() async throws {
        let (request, _) = try await sent(Self.minimalRequest)
        XCTAssertEqual(request.request.method.rawValue, "POST")
        XCTAssertEqual(request.pathWithoutQuery, "/api/judge/v1/outspeed-and-ko")
        XCTAssertTrue(request.queryItems.isEmpty)
        XCTAssertEqual(request.header("Content-Type")?.hasPrefix("application/json"), true)
        assertIdentityHeaders(request)
    }

    // MARK: - ボディ(省略の規則。最小の要求を送る。ADR-0705 §6)

    func testMinimalBodyHasOnlyTheRequiredFields() async throws {
        let (_, body) = try await sent(Self.minimalRequest)
        XCTAssertEqual(Set(body.keys), ["format", "attacker", "defenders", "moveId"], "ranks・speedField・field は載せない")
        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertEqual(body["moveId"] as? String, "stub-move-attacker")

        let attacker = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(Set(attacker.keys), ["speciesKey", "natureId", "sp"], "ranks・abilityId・itemId は nil なら欄ごと送らない(null も送らない)")
        XCTAssertEqual(attacker["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(attacker["natureId"] as? String, "stub-nature-neutral")
        let sp = try XCTUnwrap(attacker["sp"] as? [String: Int])
        XCTAssertEqual(sp, ["hp": 1, "atk": 2, "def": 3, "spa": 4, "spd": 5, "spe": 6], "能力ポイントは6項目とも常に送る")

        let defenders = try XCTUnwrap(body["defenders"] as? [[String: Any]])
        XCTAssertEqual(defenders.count, 1)
        XCTAssertEqual(Set(defenders[0].keys), ["speciesKey", "natureId", "sp", "moveId"], "候補は技 moveId を必ず持つ")
        XCTAssertEqual(defenders[0]["moveId"] as? String, "stub-move-defender")
    }

    func testBodyNeverContainsNullOrTheFieldObject() async throws {
        let (request, body) = try await sent(Self.minimalRequest)
        let text = String(decoding: try XCTUnwrap(request.body), as: UTF8.self)
        XCTAssertFalse(text.contains("null"), "null を明示的に送らない")
        XCTAssertNil(body["field"], "field(天候・地形・壁)は iOS では送らない(ADR-0504 §3)")
    }

    func testFullBodyCarriesRanksAbilityItemAndSpeedField() async throws {
        let attacker = JudgeIndividual(
            speciesKey: "9001-000", natureId: "stub-nature-atk-up", sp: Self.sp,
            ranks: RankBlock(atk: 1, def: -2, spa: 3, spd: 0, spe: 6), abilityId: "stub-ability", itemId: "stub-item-a")
        let first = JudgeDefender(
            individual: JudgeIndividual(speciesKey: "9002-000", natureId: "n1", sp: Self.sp, abilityId: "stub-ability-x"), moveId: "move-one")
        let second = JudgeDefender(
            individual: JudgeIndividual(speciesKey: "9003-000", natureId: "n2", sp: Self.sp, ranks: RankBlock(spe: -1), itemId: "stub-item-b"),
            moveId: "move-two")
        let request = JudgeRequest(
            attacker: attacker, moveId: "move-self", defenders: [first, second],
            speedField: JudgeSpeedField(trickRoom: true, attackerTailwind: false, defenderTailwind: true))
        let (_, body) = try await sent(request)

        let sentAttacker = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(Set(sentAttacker.keys), ["speciesKey", "natureId", "sp", "ranks", "abilityId", "itemId"])
        let ranks = try XCTUnwrap(sentAttacker["ranks"] as? [String: Int])
        XCTAssertEqual(ranks, ["atk": 1, "def": -2, "spa": 3, "spd": 0, "spe": 6], "ranks は非 nil なら5項目すべて送る(0 も含む)")
        XCTAssertEqual(sentAttacker["abilityId"] as? String, "stub-ability")
        XCTAssertEqual(sentAttacker["itemId"] as? String, "stub-item-a")

        let speedField = try XCTUnwrap(body["speedField"] as? [String: Bool])
        XCTAssertEqual(speedField, ["trickRoom": true, "attackerTailwind": false, "defenderTailwind": true], "speedField は非 nil なら3項目すべて送る")

        let defenders = try XCTUnwrap(body["defenders"] as? [[String: Any]])
        XCTAssertEqual(defenders.map { $0["speciesKey"] as? String }, ["9002-000", "9003-000"], "候補の順を保つ")
        XCTAssertEqual(defenders.map { $0["moveId"] as? String }, ["move-one", "move-two"], "技は候補ごとに別")
        XCTAssertEqual(Set(defenders[0].keys), ["speciesKey", "natureId", "sp", "moveId", "abilityId"])
        XCTAssertEqual(Set(defenders[1].keys), ["speciesKey", "natureId", "sp", "moveId", "ranks", "itemId"])
    }

    func testFormatIsSentAsTheContractValue() async throws {
        var request = Self.minimalRequest
        request.format = .double
        let (_, body) = try await sent(request)
        XCTAssertEqual(body["format"] as? String, "double", "iOS の画面は single 固定だが、型は契約の値を運ぶ")
    }

    // MARK: - 応答の写像

    func testMapsMatchupsKeepingOrderAndIndexAndEveryField() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.responseJSON))
        let response = try await service.outspeedAndKo(Self.minimalRequest)
        XCTAssertEqual(response.matchups.map(\.defenderIndex), [0, 1], "並びと defenderIndex は応答のまま")

        let first = response.matchups[0]
        XCTAssertEqual(first.attackerSpeed, 200)
        XCTAssertEqual(first.defenderSpeed, 150)
        XCTAssertTrue(first.outspeeds)
        XCTAssertFalse(first.speedTie)
        XCTAssertEqual(first.attackerMovePriority, 0)
        XCTAssertEqual(first.defenderMovePriority, 0)
        XCTAssertTrue(first.attackerMovesFirst)
        XCTAssertFalse(first.turnOrderTie)
        XCTAssertEqual(first.attackerKo, JudgeKOChance(hits: 1, guaranteed: true, displayChancePercent: 100.0))
        XCTAssertEqual(first.defenderKo, JudgeKOChance(hits: 0, guaranteed: false, displayChancePercent: 0.0))

        let second = response.matchups[1]
        XCTAssertEqual(second.defenderSpeed, 200, "候補ごとに違う値を取り違えない")
        XCTAssertFalse(second.outspeeds)
        XCTAssertTrue(second.speedTie)
        XCTAssertEqual(second.defenderMovePriority, 1)
        XCTAssertFalse(second.attackerMovesFirst)
        XCTAssertTrue(second.turnOrderTie)
        XCTAssertEqual(second.attackerKo, JudgeKOChance(hits: 3, guaranteed: false, displayChancePercent: 37.5))
        XCTAssertEqual(second.defenderKo, JudgeKOChance(hits: 2, guaranteed: true, displayChancePercent: 100.0))
    }

    func testRowsAreNotReorderedByDefenderIndex() async throws {
        let rows = [
            Self.matchupJSON(index: 1, defenderSpeed: 120, attackerKo: Self.koJSON(1, true, "100.0"), defenderKo: Self.koJSON(1, true, "100.0")),
            Self.matchupJSON(index: 0, defenderSpeed: 110, attackerKo: Self.koJSON(1, true, "100.0"), defenderKo: Self.koJSON(1, true, "100.0")),
        ]
        let json = #"{"matchups":[\#(rows.joined(separator: ","))]}"#
        let response = try await makeService(transport: RecordingTransport(json: json)).outspeedAndKo(Self.minimalRequest)
        XCTAssertEqual(response.matchups.map(\.defenderIndex), [1, 0], "サービス層は並べ替えない(並べ替えは表示の整形が defenderIndex で行う)")
    }

    func testUnsupportedMarksAreMappedPerDirectionKeepingOrder() async throws {
        let attackerMarks = "[\(Self.markJSON("move", "multi_hit", "stub-move-attacker")),\(Self.markJSON("attacker_item", "unsupported_effect", "stub-item-a"))]"
        let defenderMarks = "[\(Self.markJSON("attacker_ability", "unsupported_effect", "stub-ability-x"))]"
        let row = Self.matchupJSON(
            index: 0, defenderSpeed: 150, attackerKo: Self.koJSON(2, true, "100.0"), defenderKo: Self.koJSON(2, true, "100.0"),
            attackerMarks: attackerMarks, defenderMarks: defenderMarks)
        let json = #"{"matchups":[\#(row)]}"#
        let matchup = try await makeService(transport: RecordingTransport(json: json)).outspeedAndKo(Self.minimalRequest).matchups[0]
        XCTAssertEqual(
            matchup.attackerKoUnsupported,
            [UnsupportedMark(target: .move, reason: .multiHit, id: "stub-move-attacker"),
             UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: "stub-item-a")],
            "順方向の印は応答の順のまま")
        XCTAssertEqual(
            matchup.defenderKoUnsupported, [UnsupportedMark(target: .attackerAbility, reason: .unsupportedEffect, id: "stub-ability-x")],
            "逆方向の印は順方向と混ぜない(役割が入れ替わる。ADR-0708 §5)")
    }

    func testUnknownMarkTargetAndReasonAreKeptAsUnknown() async throws {
        let marks = "[\(Self.markJSON("future_target", "future_reason", "stub-x"))]"
        let row = Self.matchupJSON(
            index: 0, defenderSpeed: 150, attackerKo: Self.koJSON(2, true, "100.0"), defenderKo: Self.koJSON(2, true, "100.0"),
            attackerMarks: marks)
        let matchup = try await makeService(transport: RecordingTransport(json: #"{"matchups":[\#(row)]}"#))
            .outspeedAndKo(Self.minimalRequest).matchups[0]
        XCTAssertEqual(matchup.attackerKoUnsupported, [UnsupportedMark(target: .unknown, reason: .unknown, id: "stub-x")], "知らない値で応答全体を捨てない")
    }

    func testEmptyMarkArraysMapToEmpty() async throws {
        let matchups = try await makeService(transport: RecordingTransport(json: Self.responseJSON)).outspeedAndKo(Self.minimalRequest).matchups
        XCTAssertTrue(matchups.allSatisfy { $0.attackerKoUnsupported.isEmpty && $0.defenderKoUnsupported.isEmpty })
    }

    /// 印の欄は必須(null にも欠落にもしない。ADR-0708 §3)。欠けた応答は「印なし」と読まず、読めない応答(decode)にする。
    func testMissingUnsupportedFieldIsADecodeError() async throws {
        let row = """
            {"defenderIndex":0,"outspeeds":true,"speedTie":false,"attackerSpeed":200,"defenderSpeed":150,"attackerMovePriority":0,
             "defenderMovePriority":0,"attackerMovesFirst":true,"turnOrderTie":false,
             "attackerKo":\(Self.koJSON(1, true, "100.0")),"defenderKo":\(Self.koJSON(1, true, "100.0"))}
            """
        let service = try makeService(transport: RecordingTransport(json: #"{"matchups":[\#(row)]}"#))
        let error = await assertThrowsPokeCalcError("印の欄が無い応答") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    // MARK: - エラー

    /// 契約の HTTP エラーはすべて、`code` と `message` をそのまま運ぶ(英語の message を画面に出さないのは画面側)。
    func testDocumentedErrorStatusesKeepTheServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_request"), (413, "request_too_large"), (422, "unknown_species"), (422, "unknown_move"),
            (422, "unknown_nature"), (500, "internal_error"), (503, "upstream_unavailable"),
        ]
        for entry in cases {
            let service = try makeService(transport: RecordingTransport(status: entry.status, json: errorJSON(entry.code)))
            let error = await assertThrowsPokeCalcError("\(entry.status) \(entry.code)") { try await service.outspeedAndKo(Self.minimalRequest) }
            XCTAssertEqual(error?.code, entry.code, "\(entry.status)")
            XCTAssertEqual(error?.message, "english message from server defenders[1]", "どの候補で失敗したかを読むため message も運ぶ")
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("通信不能") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    func testUnreadableSuccessBodyMapsToDecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(status: 200, json: #"{"unexpected":true}"#))
        let error = await assertThrowsPokeCalcError("読めない 200") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    /// 契約に無いステータス(`default` 応答が無いので生成クライアントは undocumented で返す)。
    /// 本文が `{code,message}` の JSON ならその code(404/405 の `not_found`。ADR-0802)。読めなければ専用のコード。
    func testUndocumentedStatusWithErrorBodyKeepsItsCode() async throws {
        let service = try makeService(transport: RecordingTransport(status: 404, json: errorJSON("not_found")))
        let error = await assertThrowsPokeCalcError("404") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, "not_found")
    }

    func testUndocumentedStatusWithUnreadableBodyMapsToUnexpectedStatus() async throws {
        let service = try makeService(transport: RecordingTransport(status: 502, json: "<html>Bad Gateway</html>"))
        let error = await assertThrowsPokeCalcError("502") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.unexpectedStatus)
    }

    /// 契約に無い `code` の 4xx/5xx(ErrorCode の enum に無い値)は、契約の型では読めない(decode)。
    /// 契約に `ErrorCode` が増えたときの古いアプリの挙動を固定する(DECISIONS.md の連絡のとおり、増やす前に相談する)。
    func testUnknownErrorCodeOnADocumentedStatusIsADecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(status: 503, json: errorJSON("some_future_code")))
        let error = await assertThrowsPokeCalcError("未知の code") { try await service.outspeedAndKo(Self.minimalRequest) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    /// タスクのキャンセルは `PokeCalcError` に包まず `CancellationError` のまま(`APIPokeCalcService` と同じ)。
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
            _ = try await service.outspeedAndKo(Self.minimalRequest)
            XCTFail("エラーにならなかった")
        } catch is CancellationError {
            // 期待どおり
        } catch {
            XCTFail("CancellationError ではない: \(error)")
        }
    }
}
