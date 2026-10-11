import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcSpeedAPI
import XCTest

@testable import PokeCalcCore

/// `APISpeedService.matchup` / `reach`(I-speed-4c。ADR-0610 §2。契約 0.7.0): 生成された `Client`(PokeCalcSpeedAPI)と
/// 偽の transport で、ドメイン ↔ HTTP(services/speed/api/openapi.yaml)の写像を検証する。
/// 既存の `APISpeedServiceTests`(pokemon・table・position)に倣う。フィクスチャは架空(名前は「テスト」で始める)。
///
/// 本文の省略規則(Web の speedClient と同じ。省略 = 既定値と同じ意味):
///  - matchup: `self.rank` は 0 以外のときだけ、`scarf` / `tailwind` / `paralysis` / `trickRoom` は true のときだけ送る。
///    `self.current` は値があるときだけ。`opponent.assumptions` は nil のときだけ省く(サーバーの既定 4 つ)。
///    想定は `{"preset": id}` か `{"custom": {sp, nature, rank?, scarf?}}`(custom の rank 0・scarf false は送らない)。
///  - reach: 本文は `position` と同じ(`generatedPositionRequest` を共有)。
final class APISpeedMatchupTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APISpeedService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APISpeedService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - 架空のフィクスチャ(契約の例と同じ形)

    private static let selfJSON = #"{"pokemonId":"9001-000","nameJa":"テストカソウドリ","types":["fire","flying"],"baseSpeed":100}"#
    private static let opponentJSON = #"{"pokemonId":"9004-000","nameJa":"テストデンキネズミ","types":["electric"],"baseSpeed":130}"#

    private static var matchupJSON: String {
        """
        {"self":\(selfJSON),"opponent":\(opponentJSON),"trickRoom":false,"selfSpeed":167,"lines":[
          {"assumption":{"preset":"max-scarf"},"opponentSpeed":300,"current":"opponent_first","natures":[
            {"nature":"minus"},{"nature":"neutral"},{"nature":"plus"}]},
          {"assumption":{"preset":"uninvested"},"opponentSpeed":150,"current":"self_first","natures":[
            {"nature":"minus"},
            {"nature":"neutral","movesFirst":{"min":31,"max":32}},
            {"nature":"plus","movesFirst":{"min":18,"max":32},"tie":{"min":17,"max":17}}]},
          {"assumption":{"custom":{"sp":20,"nature":"plus","rank":1,"scarf":true}},"opponentSpeed":167,"current":"speed_tie","natures":[
            {"nature":"minus"},{"nature":"neutral","tie":{"min":10,"max":12}},{"nature":"plus"}]}]}
        """
    }

    private static var reachJSON: String {
        let pokemon = selfJSON
        var lines: [String] = []
        for preset in ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"] {
            lines.append(#"{"preset":"\#(preset)","slowerCount":0,"tieCount":0,"fasterCount":0}"#)
        }
        // 先頭の行だけ全項目を持つ(省略可能な項目が 3 つの区間と 2 体の例)。
        lines[0] = """
            {"preset":"uninvested","slower":{"min":1,"max":146},"tie":{"min":147,"max":147},"faster":{"min":148,"max":255},
             "slowerCount":3,"tieCount":1,"fasterCount":2,"fastestSlower":\(pokemon),"slowestFaster":\(opponentJSON)}
            """
        return #"{"speed":167,"pokemon":\#(pokemon),"lines":[\#(lines.joined(separator: ","))]}"#
    }

    private func errorJSON(_ code: String) -> String { #"{"code":"\#(code)","message":"english message from server"}"# }

    private func assertIdentityHeaders(_ sent: RecordingTransport.Recorded, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID, file: file, line: line)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID, file: file, line: line)
    }

    // MARK: - matchup: 要求(経路・ヘッダー)

    private func sentMatchupBody(_ request: SpeedMatchupRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.matchupJSON)
        _ = try await makeService(transport: transport).matchup(request)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        return (sent, try sent.jsonBody())
    }

    func testMatchupSendsPostWithPathHeadersAndJSONContentType() async throws {
        let (sent, _) = try await sentMatchupBody(
            SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000")))
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/speed/v1/matchup")
        XCTAssertTrue(sent.queryItems.isEmpty)
        XCTAssertEqual(sent.header("Content-Type")?.hasPrefix("application/json"), true)
        assertIdentityHeaders(sent)
    }

    // MARK: - matchup: 要求の本文

    /// 既定の値(ランク 0・場の効果 false・今の振りなし・想定なし)は何も送らない。
    func testMatchupBodyOmitsEveryDefault() async throws {
        let (_, body) = try await sentMatchupBody(
            SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000")))
        XCTAssertEqual(Set(body.keys), ["self", "opponent"], "trickRoom=false は送らない")
        let own = try XCTUnwrap(body["self"] as? [String: Any])
        XCTAssertEqual(Set(own.keys), ["pokemonId"])
        XCTAssertEqual(own["pokemonId"] as? String, "9001-000")
        let opponent = try XCTUnwrap(body["opponent"] as? [String: Any])
        XCTAssertEqual(Set(opponent.keys), ["pokemonId"], "assumptions が nil なら省く(サーバーの既定の 4 つ)")
        XCTAssertEqual(opponent["pokemonId"] as? String, "9004-000")
    }

    func testMatchupBodyCarriesEveryNonDefaultField() async throws {
        let request = SpeedMatchupRequest(
            own: .init(
                pokemonId: "9001-000", rank: 2, scarf: true, tailwind: true, paralysis: true,
                current: SpeedInvestment(sp: 20, nature: .plus)),
            opponent: .init(
                pokemonId: "9004-000",
                assumptions: [
                    .preset(.max),
                    .custom(SpeedCustomAssumption(sp: 4, nature: .minus, rank: -1, scarf: true)),
                    .custom(SpeedCustomAssumption(sp: 32, nature: .neutral)),
                ],
                tailwind: true, paralysis: true),
            trickRoom: true)
        let (_, body) = try await sentMatchupBody(request)
        XCTAssertEqual(Set(body.keys), ["self", "opponent", "trickRoom"])
        XCTAssertEqual(body["trickRoom"] as? Bool, true)

        let own = try XCTUnwrap(body["self"] as? [String: Any])
        XCTAssertEqual(Set(own.keys), ["pokemonId", "rank", "scarf", "tailwind", "paralysis", "current"])
        XCTAssertEqual(own["rank"] as? Int, 2)
        XCTAssertEqual(own["scarf"] as? Bool, true)
        XCTAssertEqual(own["tailwind"] as? Bool, true)
        XCTAssertEqual(own["paralysis"] as? Bool, true)
        let current = try XCTUnwrap(own["current"] as? [String: Any])
        XCTAssertEqual(Set(current.keys), ["sp", "nature"])
        XCTAssertEqual(current["sp"] as? Int, 20)
        XCTAssertEqual(current["nature"] as? String, "plus")

        let opponent = try XCTUnwrap(body["opponent"] as? [String: Any])
        XCTAssertEqual(Set(opponent.keys), ["pokemonId", "assumptions", "tailwind", "paralysis"])
        XCTAssertEqual(opponent["tailwind"] as? Bool, true)
        XCTAssertEqual(opponent["paralysis"] as? Bool, true)
        let assumptions = try XCTUnwrap(opponent["assumptions"] as? [[String: Any]])
        XCTAssertEqual(assumptions.count, 3, "想定の順を保つ")
        XCTAssertEqual(Set(assumptions[0].keys), ["preset"], "preset と custom は同時に送らない")
        XCTAssertEqual(assumptions[0]["preset"] as? String, "max")
        XCTAssertEqual(Set(assumptions[1].keys), ["custom"])
        let custom = try XCTUnwrap(assumptions[1]["custom"] as? [String: Any])
        XCTAssertEqual(Set(custom.keys), ["sp", "nature", "rank", "scarf"])
        XCTAssertEqual(custom["sp"] as? Int, 4)
        XCTAssertEqual(custom["nature"] as? String, "minus")
        XCTAssertEqual(custom["rank"] as? Int, -1)
        XCTAssertEqual(custom["scarf"] as? Bool, true)
        let plain = try XCTUnwrap(assumptions[2]["custom"] as? [String: Any])
        XCTAssertEqual(Set(plain.keys), ["sp", "nature"], "custom の rank 0・scarf false は送らない")
        XCTAssertEqual(plain["sp"] as? Int, 32)
        XCTAssertEqual(plain["nature"] as? String, "neutral")
    }

    func testMatchupBodySendsPresetIDsAsTheContractWritesThem() async throws {
        let all = SpeedPresetID.allCases.map { SpeedAssumption.preset($0) }
        let (_, body) = try await sentMatchupBody(
            SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000", assumptions: all)))
        let opponent = try XCTUnwrap(body["opponent"] as? [String: Any])
        let assumptions = try XCTUnwrap(opponent["assumptions"] as? [[String: Any]])
        XCTAssertEqual(
            assumptions.compactMap { $0["preset"] as? String },
            ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"])
    }

    // MARK: - matchup: 応答

    func testMatchupMapsResponseKeepingTheServerOrder() async throws {
        let matchup = try await makeService(transport: RecordingTransport(json: Self.matchupJSON))
            .matchup(SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000")))
        XCTAssertEqual(matchup.selfPokemon.pokemonId, "9001-000")
        XCTAssertEqual(matchup.selfPokemon.types, ["fire", "flying"])
        XCTAssertEqual(matchup.opponent.pokemonId, "9004-000")
        XCTAssertEqual(matchup.opponent.baseSpeed, 130)
        XCTAssertFalse(matchup.trickRoom)
        XCTAssertEqual(matchup.selfSpeed, 167)
        XCTAssertEqual(matchup.lines.map(\.opponentSpeed), [300, 150, 167], "並べ替えない")
        XCTAssertEqual(
            matchup.lines.map(\.assumption),
            [.preset(.maxScarf), .preset(.uninvested), .custom(SpeedCustomAssumption(sp: 20, nature: .plus, rank: 1, scarf: true))])
        XCTAssertEqual(matchup.lines.map(\.current), [.opponentFirst, .selfFirst, .speedTie])
    }

    func testMatchupMapsNaturesAndAbsentRangesToNil() async throws {
        let matchup = try await makeService(transport: RecordingTransport(json: Self.matchupJSON))
            .matchup(SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000")))
        XCTAssertEqual(matchup.lines[0].natures.map(\.nature), [.minus, .neutral, .plus], "常に 3 件。下降・補正なし・上昇の順")
        XCTAssertTrue(matchup.lines[0].natures.allSatisfy { $0.movesFirst == nil && $0.tie == nil }, "欄ごと無い区間は nil")
        let uninvested = matchup.lines[1].natures
        XCTAssertNil(uninvested[0].movesFirst)
        XCTAssertEqual(uninvested[1].movesFirst, SpeedSPRange(min: 31, max: 32))
        XCTAssertNil(uninvested[1].tie)
        XCTAssertEqual(uninvested[2].movesFirst, SpeedSPRange(min: 18, max: 32))
        XCTAssertEqual(uninvested[2].tie, SpeedSPRange(min: 17, max: 17))
        XCTAssertEqual(matchup.lines[2].natures[1].tie, SpeedSPRange(min: 10, max: 12))
    }

    func testMatchupWithoutCurrentHasNoSelfSpeedAndNoTurnOrder() async throws {
        let json = """
            {"self":\(Self.selfJSON),"opponent":\(Self.opponentJSON),"trickRoom":true,"lines":[
              {"assumption":{"preset":"max"},"opponentSpeed":200,"natures":[{"nature":"minus"},{"nature":"neutral"},{"nature":"plus"}]}]}
            """
        let matchup = try await makeService(transport: RecordingTransport(json: json))
            .matchup(SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000"), trickRoom: true))
        XCTAssertNil(matchup.selfSpeed, "self.current が無ければ応答に selfSpeed は無い")
        XCTAssertNil(matchup.lines[0].current)
        XCTAssertTrue(matchup.trickRoom)
    }

    // MARK: - reach: 要求

    private func sentReach(_ request: SpeedPositionRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.reachJSON)
        _ = try await makeService(transport: transport).reach(request)
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(transport.requests.count, 1)
        return (sent, try sent.jsonBody())
    }

    func testReachSendsPostWithPathHeadersAndJSONContentType() async throws {
        let (sent, _) = try await sentReach(SpeedPositionRequest(input: .raw(value: 170, pokemonId: nil)))
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/speed/v1/reach")
        XCTAssertTrue(sent.queryItems.isEmpty)
        XCTAssertEqual(sent.header("Content-Type")?.hasPrefix("application/json"), true)
        assertIdentityHeaders(sent)
    }

    /// 本文は `position` と同じ(mode ごとの項目・`tableTailwind`)。今の振りから作る custom が主な使い方。
    func testReachBodyIsTheSameAsPosition() async throws {
        let (_, custom) = try await sentReach(
            SpeedPositionRequest(
                input: .custom(pokemonId: "9001-000", sp: 20, nature: .plus, rank: 1, scarf: true, tailwind: true, paralysis: false),
                tableTailwind: true))
        XCTAssertEqual(Set(custom.keys), ["mode", "pokemonId", "sp", "nature", "rank", "scarf", "tailwind", "tableTailwind"])
        XCTAssertEqual(custom["mode"] as? String, "custom")
        XCTAssertEqual(custom["sp"] as? Int, 20)
        XCTAssertEqual(custom["nature"] as? String, "plus")
        XCTAssertEqual(custom["tableTailwind"] as? Bool, true)

        let (_, raw) = try await sentReach(SpeedPositionRequest(input: .raw(value: 170, pokemonId: nil)))
        XCTAssertEqual(Set(raw.keys), ["mode", "value"])
        XCTAssertEqual(raw["value"] as? Int, 170)
    }

    // MARK: - reach: 応答

    func testReachMapsResponse() async throws {
        let reach = try await makeService(transport: RecordingTransport(json: Self.reachJSON))
            .reach(SpeedPositionRequest(input: .raw(value: 167, pokemonId: "9001-000")))
        XCTAssertEqual(reach.speed, 167)
        XCTAssertEqual(reach.pokemon?.pokemonId, "9001-000")
        XCTAssertEqual(reach.lines.map(\.preset), SpeedPresetID.allCases, "6 件・表のプリセットの順")
        let first = reach.lines[0]
        XCTAssertEqual(first.slower, SpeedBaseSpeedRange(min: 1, max: 146))
        XCTAssertEqual(first.tie, SpeedBaseSpeedRange(min: 147, max: 147))
        XCTAssertEqual(first.faster, SpeedBaseSpeedRange(min: 148, max: 255))
        XCTAssertEqual([first.slowerCount, first.tieCount, first.fasterCount], [3, 1, 2])
        XCTAssertEqual(first.fastestSlower?.pokemonId, "9001-000")
        XCTAssertEqual(first.slowestFaster?.pokemonId, "9004-000")
    }

    func testReachMapsAbsentRangesAndExamplesToNil() async throws {
        let reach = try await makeService(transport: RecordingTransport(json: Self.reachJSON))
            .reach(SpeedPositionRequest(input: .raw(value: 167, pokemonId: nil)))
        let last = reach.lines[5]
        XCTAssertNil(last.slower)
        XCTAssertNil(last.tie)
        XCTAssertNil(last.faster)
        XCTAssertNil(last.fastestSlower)
        XCTAssertNil(last.slowestFaster)
        XCTAssertEqual([last.slowerCount, last.tieCount, last.fasterCount], [0, 0, 0])
    }

    func testReachWithoutPokemonHasNoPokemon() async throws {
        var lines: [String] = []
        for preset in ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"] {
            lines.append(#"{"preset":"\#(preset)","slowerCount":0,"tieCount":0,"fasterCount":0}"#)
        }
        let json = #"{"speed":170,"lines":[\#(lines.joined(separator: ","))]}"#
        let reach = try await makeService(transport: RecordingTransport(json: json))
            .reach(SpeedPositionRequest(input: .raw(value: 170, pokemonId: nil)))
        XCTAssertNil(reach.pokemon, "pokemon は要求に pokemonId が無ければ応答に無い")
    }

    // MARK: - エラー(2 操作とも)

    private typealias Call = @Sendable (APISpeedService) async throws -> Void

    private var calls: [(name: String, call: Call)] {
        let matchup = SpeedMatchupRequest(own: .init(pokemonId: "9001-000"), opponent: .init(pokemonId: "9004-000"))
        let reach = SpeedPositionRequest(input: .raw(value: 1, pokemonId: nil))
        return [
            ("matchup", { _ = try await $0.matchup(matchup) }),
            ("reach", { _ = try await $0.reach(reach) }),
        ]
    }

    /// 契約の HTTP エラー(400・413・422・500・503)はすべて、`code` をそのまま運ぶ `PokeCalcError`。
    func testDocumentedErrorStatusesKeepTheServerCode() async throws {
        let codeByStatus = [
            400: "invalid_request", 413: "request_too_large", 422: "unknown_pokemon", 500: "internal_error",
            503: "master_unavailable",
        ]
        for entry in calls {
            for (status, code) in codeByStatus {
                let service = try makeService(transport: RecordingTransport(status: status, json: errorJSON(code)))
                let error = await assertThrowsPokeCalcError("\(entry.name) \(status)") { try await entry.call(service) }
                XCTAssertEqual(error?.code, code, "\(entry.name) \(status)")
            }
        }
    }

    /// gateway が speed に届かないときの 503 `upstream_unavailable`(契約外の code)も、画面では「接続できません」になる。
    /// code を運ぶか decode に分類されるかは実装が決めてよい(どちらも `SpeedLabels.unavailable`)。
    func testGatewayUpstreamUnavailableEndsUpAsTheUnavailableMessage() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 503, json: errorJSON("upstream_unavailable")))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            let code = try XCTUnwrap(error?.code, entry.name)
            XCTAssertEqual(SpeedLabels.errorMessage(forCode: code), SpeedLabels.unavailable, entry.name)
        }
    }

    func testTransportFailureMapsToTransportError() async throws {
        for entry in calls {
            let service = try makeService(transport: FailingTransport())
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.transport, entry.name)
        }
    }

    func testUnreadableSuccessBodyMapsToDecodeError() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 200, json: #"{"unexpected":true}"#))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.decode, entry.name)
        }
    }

    func testUndocumentedStatusWithErrorBodyKeepsItsCode() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 404, json: errorJSON("not_found")))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, "not_found", entry.name)
        }
    }

    func testUndocumentedStatusWithUnreadableBodyMapsToUnexpectedStatus() async throws {
        for entry in calls {
            let service = try makeService(transport: RecordingTransport(status: 502, json: "<html>Bad Gateway</html>"))
            let error = await assertThrowsPokeCalcError(entry.name) { try await entry.call(service) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.unexpectedStatus, entry.name)
        }
    }

    /// タスクのキャンセルは `PokeCalcError` に包まず `CancellationError` のまま。
    func testCancellationIsNotWrappedInPokeCalcError() async throws {
        struct CancellingTransport: ClientTransport {
            func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String) async throws
                -> (HTTPResponse, HTTPBody?)
            {
                throw CancellationError()
            }
        }
        let service = try makeService(transport: CancellingTransport())
        for entry in calls {
            do {
                try await entry.call(service)
                XCTFail("\(entry.name): エラーにならなかった")
            } catch is CancellationError {
                // 期待どおり
            } catch {
                XCTFail("\(entry.name): CancellationError ではない: \(error)")
            }
        }
    }

    // MARK: - 生成の同期(契約が変わったときに落ちる網。SpeedContractSyncTests と同じ運用)

    func testTurnOrdersMatchContract() {
        XCTAssertEqual(SpeedTurnOrder.allCases.map(\.rawValue), Components.Schemas.TurnOrder.allCases.map(\.rawValue))
    }
}
