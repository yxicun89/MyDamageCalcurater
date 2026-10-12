import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService` の連続技(`POST /api/calc/combo`。ADR-0253・ADR-0535)と公開 `Move.effect` の写像(AC-i11)。
/// 期待値は api/openapi.yaml のプロパティ名から書く。フィクスチャは架空(ADR-0002)。
final class APIPokeCalcServiceComboTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport), identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private static let calcResultJSON = """
    {"rolls":[40,40,41,41,42,42,43,43,44,44,45,45,46,46,47,48],"minDamage":40,"maxDamage":48,
     "minPercent":40.2,"maxPercent":47.8,"defenderHP":100,"effectiveness":1,"stab":false,"category":"physical",
     "ko":{"hits":3,"guaranteed":true,"chancePercent":100,"displayChancePercent":100},
     "unsupported":[{"target":"move","reason":"multi_hit","id":"test-move-a"}]}
    """

    private static let ranksJSON = #"{"atk":0,"def":0,"spa":0,"spd":0,"spe":0}"#
    private static let boostedJSON = #"{"atk":1,"def":0,"spa":0,"spd":0,"spe":1}"#

    /// 2 手(攻撃 → 変化技)・相手の技あり・連続技の印あり。
    private static let comboResultJSON = """
    {"steps":[
      {"moveId":"test-move-a","selfStartRanks":\(ranksJSON),"selfEndRanks":\(ranksJSON),
       "opponentStartRanks":\(ranksJSON),"opponentEndRanks":\(ranksJSON),"damage":\(calcResultJSON),
       "selfSpeed":132,"opponentSpeed":0,"selfPriority":0,"opponentPriority":0,"order":"depends_on_opponent_speed",
       "line":{"selfSpeed":132,"selfFirst":{"min":0,"max":131},"opponentFirst":{"min":133,"max":999}}},
      {"moveId":"test-move-boost","selfStartRanks":\(ranksJSON),"selfEndRanks":\(boostedJSON),
       "opponentStartRanks":\(ranksJSON),"opponentEndRanks":\(ranksJSON),"damage":null,
       "selfSpeed":198,"opponentSpeed":0,"selfPriority":1,"opponentPriority":0,"order":"self_first",
       "line":{"selfSpeed":198,"selfFirst":{"min":0,"max":197},"opponentFirst":{"min":199,"max":999}}}
     ],
     "total":{"minDamage":91,"maxDamage":109,"minPercent":91.4,"maxPercent":108.6,"defenderHP":100,
       "ko":{"chancePercent":51.5625,"guaranteed":false,"displayChancePercent":51.6},
       "koByStep":[{"chancePercent":0,"guaranteed":false,"displayChancePercent":0},
                   {"chancePercent":51.5625,"guaranteed":false,"displayChancePercent":51.6}]},
     "speed":{"firstSelfSpeed":132,"lastSelfSpeed":198,"opponentSpeed":0,"opponentSpeedKnown":false},
     "opponentAttack":{"selfHp":132,"minActions":1,"maxActions":2,
       "fast":{"actions":[{"step":0,"timing":"before_self","damage":\(calcResultJSON),"surviveChancePercent":87.5}],
               "surviveChancePercent":87.5},
       "slow":{"actions":[],"surviveChancePercent":100}},
     "unsupported":[{"target":"move","reason":"multi_hit","id":"test-move-a"},
                    {"target":"attacker_item","reason":"choice_locked","id":"test-item-choice"},
                    {"target":"defender_ability","reason":"defender_full_hp_assumed","id":"test-ability-full"},
                    {"target":"move","reason":"speed_undetermined","id":"test-move-boost"}]}
    """

    private static let minimalResultJSON = """
    {"steps":[{"moveId":"test-move-a","selfStartRanks":\(ranksJSON),"selfEndRanks":\(ranksJSON),
       "opponentStartRanks":\(ranksJSON),"opponentEndRanks":\(ranksJSON),"damage":\(calcResultJSON),
       "selfSpeed":100,"opponentSpeed":0,"selfPriority":0,"opponentPriority":0,"order":"speed_tie",
       "line":{"selfSpeed":100,"selfFirst":{"min":0,"max":99},"opponentFirst":{"min":101,"max":999}}}],
     "total":{"minDamage":40,"maxDamage":48,"minPercent":40.2,"maxPercent":47.8,"defenderHP":100,
       "ko":{"chancePercent":0,"guaranteed":false,"displayChancePercent":0},
       "koByStep":[{"chancePercent":0,"guaranteed":false,"displayChancePercent":0}]},
     "speed":{"firstSelfSpeed":100,"lastSelfSpeed":100,"opponentSpeed":0,"opponentSpeedKnown":false},
     "opponentAttack":null,"unsupported":[]}
    """

    private let attacker = Individual(
        speciesKey: "9001-000", natureId: "test-nature-atk", sp: StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32),
        abilityId: "test-ability-a", itemId: "test-item-a", ranks: RankBlock(atk: 1), status: .burn)
    private let defender = Individual(
        speciesKey: "9002-000", natureId: "test-nature-def", sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0),
        abilityId: "test-ability-b", itemId: "test-item-b", ranks: RankBlock(def: 1))

    private func request(
        steps: [ComboStepInput] = [ComboStepInput(moveId: "test-move-a"), ComboStepInput(moveId: "test-move-boost")],
        opponentMoveId: String? = nil, opponentSpeed: ComboOpponentSpeed = .unknown, speedField: ComboSpeedField = ComboSpeedField()
    ) -> ComboRequest {
        ComboRequest(
            format: .single, attacker: attacker, defender: defender, field: FieldState(weather: .rain), steps: steps,
            opponentMoveId: opponentMoveId, opponentSpeed: opponentSpeed, speedField: speedField)
    }

    private func sentBody(_ request: ComboRequest) async throws -> (RecordingTransport.Recorded, [String: Any]) {
        let transport = RecordingTransport(json: Self.minimalResultJSON)
        _ = try await makeService(transport: transport).calcCombo(request)
        let sent = try XCTUnwrap(transport.requests.first)
        return (sent, try sent.jsonBody())
    }

    // MARK: - 要求

    func testRequestGoesToTheComboPathWithClientHeaders() async throws {
        let (sent, _) = try await sentBody(request())
        XCTAssertEqual(sent.request.method.rawValue, "POST")
        XCTAssertEqual(sent.pathWithoutQuery, "/api/calc/combo")
        XCTAssertEqual(sent.operationID, "calcCombo")
        XCTAssertEqual(sent.header("X-Device-Id"), deviceID)
        XCTAssertEqual(sent.header("X-Session-Id"), sessionID)
    }

    func testDefaultRequestOmitsOptionalKeys() async throws {
        let (_, body) = try await sentBody(request())
        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertNil(body["opponentMoveId"], "相手の技を選んでいなければ送らない")
        XCTAssertNil(body["opponentSpeed"], "unknown は既定なので送らない(Web と同じ)")
        XCTAssertNil(body["speedField"], "全部 false なら送らない")
        let steps = try XCTUnwrap(body["steps"] as? [[String: Any]])
        XCTAssertEqual(steps.count, 2)
        XCTAssertEqual(steps[0]["moveId"] as? String, "test-move-a")
        XCTAssertNil(steps[0]["critical"], "急所は true のときだけ")
        XCTAssertNil(steps[0]["hits"], "多段の回数は指定したときだけ")
        let attackerJSON = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(attackerJSON["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(attackerJSON["itemId"] as? String, "test-item-a")
        XCTAssertEqual(attackerJSON["status"] as? String, "burn")
        let defenderJSON = try XCTUnwrap(body["defender"] as? [String: Any])
        XCTAssertEqual(defenderJSON["natureId"] as? String, "test-nature-def")
        XCTAssertEqual((defenderJSON["ranks"] as? [String: Any])?["def"] as? Int, 1)
        let field = try XCTUnwrap(body["field"] as? [String: Any])
        XCTAssertEqual(field["weather"] as? String, "rain")
    }

    func testOptionalKeysAreSentWhenSet() async throws {
        let (_, body) = try await sentBody(request(
            steps: [ComboStepInput(moveId: "test-move-a", critical: true, hits: 3), ComboStepInput(moveId: "test-move-boost")],
            opponentMoveId: "test-move-opp", opponentSpeed: .fromDefender,
            speedField: ComboSpeedField(trickRoom: true, selfTailwind: false, opponentTailwind: true)))
        XCTAssertEqual(body["opponentMoveId"] as? String, "test-move-opp")
        XCTAssertEqual(body["opponentSpeed"] as? String, "from_defender")
        let speedField = try XCTUnwrap(body["speedField"] as? [String: Any])
        XCTAssertEqual(speedField["trickRoom"] as? Bool, true)
        XCTAssertEqual(speedField["opponentTailwind"] as? Bool, true)
        let steps = try XCTUnwrap(body["steps"] as? [[String: Any]])
        XCTAssertEqual(steps[0]["critical"] as? Bool, true)
        XCTAssertEqual(steps[0]["hits"] as? Int, 3)
        XCTAssertNil(steps[1]["critical"])
    }

    // MARK: - 応答

    func testResponseMapsEveryField() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.comboResultJSON))
        let result = try await service.calcCombo(request(opponentMoveId: "test-move-opp"))

        XCTAssertEqual(result.steps.count, 2)
        let first = result.steps[0]
        XCTAssertEqual(first.moveId, "test-move-a")
        XCTAssertEqual(first.damage?.minPercent, 40.2)
        XCTAssertEqual(first.damage?.unsupported, [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-a")])
        XCTAssertEqual(first.order, .dependsOnOpponentSpeed)
        XCTAssertEqual(first.line, ComboSpeedLine(
            selfSpeed: 132, selfFirst: ComboSpeedRange(min: 0, max: 131), opponentFirst: ComboSpeedRange(min: 133, max: 999)))
        let second = result.steps[1]
        XCTAssertNil(second.damage, "変化技の手は null")
        XCTAssertEqual(second.selfEndRanks, RankBlock(atk: 1, spe: 1))
        XCTAssertEqual(second.selfPriority, 1)
        XCTAssertEqual(second.order, .selfFirst)

        XCTAssertEqual(result.total.minPercent, 91.4)
        XCTAssertEqual(result.total.maxPercent, 108.6)
        XCTAssertEqual(result.total.ko, ComboKOChance(chancePercent: 51.5625, guaranteed: false, displayChancePercent: 51.6))
        XCTAssertEqual(result.total.koByStep.count, 2)
        XCTAssertEqual(result.speed, ComboSpeedSummary(firstSelfSpeed: 132, lastSelfSpeed: 198, opponentSpeed: 0, opponentSpeedKnown: false))

        let attack = try XCTUnwrap(result.opponentAttack)
        XCTAssertEqual(attack.selfHp, 132)
        XCTAssertEqual(attack.minActions, 1)
        XCTAssertEqual(attack.maxActions, 2)
        XCTAssertEqual(attack.fast.surviveChancePercent, 87.5)
        XCTAssertEqual(attack.fast.actions.first?.timing, .beforeSelf)
        XCTAssertEqual(attack.fast.actions.first?.damage.maxPercent, 47.8)
        XCTAssertEqual(attack.slow.actions, [])

        XCTAssertEqual(result.unsupported, [
            .calc(UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-a")),
            .combo(target: .attackerItem, reason: .choiceLocked, id: "test-item-choice"),
            .combo(target: .defenderAbility, reason: .defenderFullHPAssumed, id: "test-ability-full"),
            .combo(target: .move, reason: .speedUndetermined, id: "test-move-boost"),
        ], "連続技だけの理由は ComboMark.combo。それ以外は単発と同じ印")
    }

    func testNullOpponentAttackAndSpeedTie() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.minimalResultJSON))
        let result = try await service.calcCombo(request(steps: [ComboStepInput(moveId: "test-move-a")]))
        XCTAssertNil(result.opponentAttack)
        XCTAssertEqual(result.steps.first?.order, .speedTie)
        XCTAssertEqual(result.unsupported, [])
    }

    // MARK: - 失敗(AC-i10・AC-i11)

    func testErrorResponsesCarryTheServerCode() async throws {
        let cases: [(status: Int, code: String)] = [
            (400, "invalid_input"), (400, "unknown_move"), (500, "internal"), (503, "master_unavailable"),
        ]
        for testCase in cases {
            let transport = RecordingTransport(
                status: testCase.status, json: #"{"code":"\#(testCase.code)","message":"テストのエラー"}"#)
            do {
                _ = try await makeService(transport: transport).calcCombo(request())
                XCTFail("失敗するはず: \(testCase)")
            } catch let error as PokeCalcError {
                XCTAssertEqual(error.code, testCase.code, "\(testCase)")
            }
        }
    }

    func testTransportAndDecodeFailures() async throws {
        do {
            _ = try await makeService(transport: FailingTransport()).calcCombo(request())
            XCTFail("失敗するはず")
        } catch let error as PokeCalcError {
            XCTAssertEqual(error.code, PokeCalcError.Code.transport)
        }
        do {
            _ = try await makeService(transport: RecordingTransport(json: #"{"steps":"broken"}"#)).calcCombo(request())
            XCTFail("失敗するはず")
        } catch let error as PokeCalcError {
            XCTAssertEqual(error.code, PokeCalcError.Code.decode)
        }
    }

    // MARK: - 公開 Move.effect(AC-i11)

    func testMoveEffectIsMappedFromPascalCaseAndInvalidEffectsAreIgnored() async throws {
        let json = """
        [
          {"id":"test-move-boost","nameJa":"テストまいわざ","type":"dragon","category":"status","power":0,
           "effect":{"Chance":100,"Target":"self","Stages":{"atk":1,"spe":1}}},
          {"id":"test-move-slow","nameJa":"テストいわわざ","type":"rock","category":"physical","power":60,
           "effect":{"Chance":100,"Target":"target","Stages":{"spe":-1}}},
          {"id":"test-move-chance","nameJa":"テストかくりつわざ","type":"steel","category":"physical","power":50,
           "effect":{"Chance":10,"Target":"self","Stages":{"atk":1}}},
          {"id":"test-move-odd","nameJa":"テストへんなこうか","type":"normal","category":"status","power":0,
           "effect":{"Chance":100,"Target":"ally","Stages":{"atk":1}}},
          {"id":"test-move-hp","nameJa":"テストHPこうか","type":"normal","category":"status","power":0,
           "effect":{"Chance":100,"Target":"self","Stages":{"hp":1}}},
          {"id":"test-move-plain","nameJa":"テストふつう","type":"normal","category":"physical","power":40}
        ]
        """
        let moves = try await makeService(transport: RecordingTransport(json: json)).searchMoves(query: "テスト", limit: 10)
        XCTAssertEqual(moves.count, 6, "効果が読めなくても技そのものは落とさない")
        XCTAssertEqual(moves[0].effect, MoveRankEffect(target: .user, stages: RankBlock(atk: 1, spe: 1)))
        XCTAssertEqual(moves[1].effect, MoveRankEffect(target: .opponent, stages: RankBlock(spe: -1)))
        XCTAssertNil(moves[2].effect, "確率 100 未満は扱わない(combo-calc.md §7 Q4 (b))")
        XCTAssertNil(moves[3].effect, "target の語彙外は「ランクが変わらない」")
        XCTAssertNil(moves[4].effect, "stages のキーの語彙外は「ランクが変わらない」")
        XCTAssertNil(moves[5].effect)
    }
}
