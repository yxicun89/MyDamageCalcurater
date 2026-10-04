import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcJudgeAPI
import XCTest

@testable import PokeCalcCore

/// 応答の素早さの欄(`*SpeedApplied`・`*SpeedIgnored`)に契約の既知の値以外が来ても、decode が失敗せず文字列のまま運ぶ(ADR-0512)。
/// 生成型の `SpeedFactor`・`SpeedIgnoredInput` は閉じた enum(`@frozen`)なので、契約に値が増えると古いアプリが応答ごと読めなくなる(判定レーンからの依頼)。
/// 生成物は手編集しない。設計(実装者への注意に詳細): `APIJudgeService.init(serverURL:transport:identity:)` が組む `Client` に ClientMiddleware を付け、
/// 成功応答の本文から4つの配列を取り出して呼び出しごとの入れ物(TaskLocal)へ退避し、生成型には空配列として渡す。サービスは退避した生の文字列で `JudgeMatchup` を作る。
/// 厳格さは保つ: 欄が無い・配列でない・文字列でない要素を含む応答は、従来どおり decode エラー(必須欄の欠落を「空」と読まない)。
final class APIJudgeServiceUnknownSpeedValuesTests: XCTestCase {
    private static let ko = #"{"hits":1,"guaranteed":true,"displayChancePercent":100.0}"#

    /// 素早さの4欄を差し替えられる応答の1行。`speedExtras` が nil なら4欄とも空配列。
    private static func row(index: Int = 0, defenderSpeed: Int = 150, speedExtras: String? = nil) -> String {
        let extras = speedExtras ?? #""attackerSpeedApplied":[],"defenderSpeedApplied":[],"attackerSpeedIgnored":[],"defenderSpeedIgnored":[]"#
        return """
            {"defenderIndex":\(index),"outspeeds":true,"speedTie":false,"attackerSpeed":200,"defenderSpeed":\(defenderSpeed),
             "attackerMovePriority":0,"defenderMovePriority":0,"attackerMovesFirst":true,"turnOrderTie":false,
             "attackerKo":\(ko),"defenderKo":\(ko),"attackerKoUnsupported":[],"defenderKoUnsupported":[],\(extras)}
            """
    }

    private static func extras(_ attackerApplied: String, _ defenderApplied: String, _ attackerIgnored: String, _ defenderIgnored: String) -> String {
        #""attackerSpeedApplied":\#(attackerApplied),"defenderSpeedApplied":\#(defenderApplied),"attackerSpeedIgnored":\#(attackerIgnored),"defenderSpeedIgnored":\#(defenderIgnored)"#
    }

    private static let request = JudgeRequest(
        attacker: JudgeIndividual(speciesKey: "9001-000", natureId: "n", sp: zeroSP), moveId: "stub-move-self",
        defenders: [JudgeDefender(individual: JudgeIndividual(speciesKey: "9002-000", natureId: "n", sp: zeroSP), moveId: "stub-move")])

    private func makeService(transport: any ClientTransport) throws -> APIJudgeService {
        APIJudgeService(
            serverURL: try XCTUnwrap(URL(string: "https://pokecalc.example.invalid")), transport: transport,
            identity: ClientIdentity(deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11", sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"))
    }

    private func matchups(_ json: String) async throws -> [JudgeMatchup] {
        try await makeService(transport: RecordingTransport(json: json)).outspeedAndKo(Self.request).matchups
    }

    // MARK: - 未知の値でも落ちない

    func testUnknownSpeedFactorDoesNotFailTheDecodeAndIsCarriedAsAString() async throws {
        let json = #"{"matchups":[\#(Self.row(speedExtras: Self.extras(#"["rank","futureFactor","paralysis"]"#, #"["brandNew"]"#, "[]", "[]")))]}"#
        let result = try await matchups(json)
        XCTAssertEqual(result[0].attackerSpeedApplied, ["rank", "futureFactor", "paralysis"], "未知の値も並べ替えず運ぶ")
        XCTAssertEqual(result[0].defenderSpeedApplied, ["brandNew"])
    }

    func testUnknownSpeedIgnoredInputDoesNotFailTheDecodeAndIsCarriedAsAString() async throws {
        let json = #"{"matchups":[\#(Self.row(speedExtras: Self.extras("[]", "[]", #"["abilityId","futureInput"]"#, #"["terrain","itemId"]"#)))]}"#
        let result = try await matchups(json)
        XCTAssertEqual(result[0].attackerSpeedIgnored, ["abilityId", "futureInput"])
        XCTAssertEqual(result[0].defenderSpeedIgnored, ["terrain", "itemId"])
    }

    func testOtherFieldsOfTheRowStillDecodeNormallyAlongsideUnknownValues() async throws {
        let json = #"{"matchups":[\#(Self.row(index: 0, defenderSpeed: 321, speedExtras: Self.extras(#"["x"]"#, #"["y"]"#, #"["z"]"#, #"["w"]"#)))]}"#
        let matchup = try await matchups(json)[0]
        XCTAssertEqual(matchup.defenderIndex, 0)
        XCTAssertEqual(matchup.attackerSpeed, 200)
        XCTAssertEqual(matchup.defenderSpeed, 321)
        XCTAssertTrue(matchup.outspeeds)
        XCTAssertEqual(matchup.attackerKo, JudgeKOChance(hits: 1, guaranteed: true, displayChancePercent: 100.0))
        XCTAssertEqual(matchup.attackerKoUnsupported, [])
    }

    func testEachRowKeepsItsOwnArraysAndSides() async throws {
        let rows = [
            Self.row(index: 0, speedExtras: Self.extras(#"["rank"]"#, "[]", "[]", #"["fieldWeather"]"#)),
            Self.row(index: 1, speedExtras: Self.extras("[]", #"["newFactor","choiceScarf"]"#, #"["newInput"]"#, "[]")),
            Self.row(index: 2),
        ]
        let result = try await matchups(#"{"matchups":[\#(rows.joined(separator: ","))]}"#)
        XCTAssertEqual(result.map(\.defenderIndex), [0, 1, 2], "並べ替えない")
        XCTAssertEqual(result.map(\.attackerSpeedApplied), [["rank"], [], []])
        XCTAssertEqual(result.map(\.defenderSpeedApplied), [[], ["newFactor", "choiceScarf"], []])
        XCTAssertEqual(result.map(\.attackerSpeedIgnored), [[], ["newInput"], []])
        XCTAssertEqual(result.map(\.defenderSpeedIgnored), [["fieldWeather"], [], []])
    }

    func testKnownValuesStillWorkThroughTheSamePath() async throws {
        let json = #"{"matchups":[\#(Self.row(speedExtras: Self.extras(#"["rank","tailwind","ability","choiceScarf","item","paralysis"]"#, "[]", #"["abilityId","itemId","fieldWeather"]"#, "[]")))]}"#
        let matchup = try await matchups(json)[0]
        XCTAssertEqual(matchup.attackerSpeedApplied, ["rank", "tailwind", "ability", "choiceScarf", "item", "paralysis"])
        XCTAssertEqual(matchup.attackerSpeedIgnored, ["abilityId", "itemId", "fieldWeather"])
    }

    func testEmptyArraysStillMapToEmpty() async throws {
        let matchup = try await matchups(#"{"matchups":[\#(Self.row())]}"#)[0]
        XCTAssertEqual(matchup.attackerSpeedApplied, [])
        XCTAssertEqual(matchup.defenderSpeedApplied, [])
        XCTAssertEqual(matchup.attackerSpeedIgnored, [])
        XCTAssertEqual(matchup.defenderSpeedIgnored, [])
    }

    // MARK: - 厳格さは保つ(必須欄の欠落・形の違いは decode エラー)

    func testMissingSpeedFieldIsStillADecodeError() async throws {
        let extras = #""attackerSpeedApplied":[],"defenderSpeedApplied":[],"attackerSpeedIgnored":[]"#  // defenderSpeedIgnored が無い
        let service = try makeService(transport: RecordingTransport(json: #"{"matchups":[\#(Self.row(speedExtras: extras))]}"#))
        let error = await assertThrowsPokeCalcError("欄が無い") { try await service.outspeedAndKo(Self.request) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode, "必須欄の欠落を「空」と読まない")
    }

    func testNullOrNonArraySpeedFieldIsStillADecodeError() async throws {
        for bad in ["null", #""rank""#, "{}", "1"] {
            let service = try makeService(
                transport: RecordingTransport(json: #"{"matchups":[\#(Self.row(speedExtras: Self.extras(bad, "[]", "[]", "[]")))]}"#))
            let error = await assertThrowsPokeCalcError("配列でない: \(bad)") { try await service.outspeedAndKo(Self.request) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.decode, bad)
        }
    }

    func testNonStringElementIsStillADecodeError() async throws {
        for bad in ["[1]", "[null]", #"["rank",{}]"#, "[[]]"] {
            let service = try makeService(
                transport: RecordingTransport(json: #"{"matchups":[\#(Self.row(speedExtras: Self.extras("[]", "[]", bad, "[]")))]}"#))
            let error = await assertThrowsPokeCalcError("文字列でない要素: \(bad)") { try await service.outspeedAndKo(Self.request) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.decode, bad)
        }
    }

    func testUnreadableBodyIsStillADecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(json: "not json"))
        let error = await assertThrowsPokeCalcError("読めない本文") { try await service.outspeedAndKo(Self.request) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    // MARK: - エラー応答・要求は変えない

    func testUnknownErrorCodeStaysADecodeErrorAndKnownCodesKeepTheirCode() async throws {
        let unknown = try makeService(
            transport: RecordingTransport(status: 503, json: #"{"code":"some_future_code","message":"m"}"#))
        let error = await assertThrowsPokeCalcError("未知の ErrorCode") { try await unknown.outspeedAndKo(Self.request) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode, "ErrorCode の未知値は従来どおり(増えたときの扱いは JudgeLabels の既存の流儀)")
        let known = try makeService(transport: RecordingTransport(status: 503, json: #"{"code":"upstream_unavailable","message":"m"}"#))
        let knownError = await assertThrowsPokeCalcError("既知の ErrorCode") { try await known.outspeedAndKo(Self.request) }
        XCTAssertEqual(knownError?.code, "upstream_unavailable")
    }

    func testTheRequestIsSentUnchangedThroughTheSamePath() async throws {
        let transport = RecordingTransport(json: #"{"matchups":[\#(Self.row())]}"#)
        _ = try await makeService(transport: transport).outspeedAndKo(Self.request)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.pathWithoutQuery, "/api/judge/v1/outspeed-and-ko")
        XCTAssertEqual(sent.header("X-Device-Id"), "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11")
        XCTAssertEqual(Set(try sent.jsonBody().keys), ["format", "attacker", "defenders", "moveId"])
    }

    // MARK: - 呼び出しをまたいで混ざらない

    /// 本文の `moveId` を未知の値に埋めて返す transport。並行に呼んでも、各呼び出しは自分の本文から作られた値だけを受け取る。
    private struct EchoingTransport: ClientTransport {
        func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String) async throws -> (HTTPResponse, HTTPBody?) {
            let data = try await Data(collecting: try XCTUnwrap(body), upTo: 1 << 20)
            let json = try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any])
            let marker = try XCTUnwrap(json["moveId"] as? String)
            try await Task.sleep(nanoseconds: UInt64.random(in: 1_000_000...20_000_000))
            let response = #"{"matchups":[\#(APIJudgeServiceUnknownSpeedValuesTests.row(speedExtras: APIJudgeServiceUnknownSpeedValuesTests.extras(#"["future-\#(marker)"]"#, "[]", "[]", #"["input-\#(marker)"]"#)))]}"#
            var fields = HTTPFields()
            fields[.contentType] = "application/json"
            return (HTTPResponse(status: .ok, headerFields: fields), HTTPBody(response))
        }
    }

    func testConcurrentCallsDoNotMixUpUnknownValues() async throws {
        let service = try makeService(transport: EchoingTransport())
        try await withThrowingTaskGroup(of: (String, JudgeMatchup).self) { group in
            for number in 0..<12 {
                let marker = "m\(number)"
                group.addTask {
                    var request = Self.request
                    request.moveId = marker
                    return (marker, try await service.outspeedAndKo(request).matchups[0])
                }
            }
            for try await (marker, matchup) in group {
                XCTAssertEqual(matchup.attackerSpeedApplied, ["future-\(marker)"], "別の呼び出しの値が混ざらない")
                XCTAssertEqual(matchup.defenderSpeedIgnored, ["input-\(marker)"])
            }
        }
    }
}
