import XCTest

@testable import PokeCalcCore

/// `MockJudgeService`(P6-25。ADR-0504 §8): XCUITest・オフラインが頼る固定の事実。式は本物を写したものではない(候補の位置から決まる決定的な値)。
/// 固定する事実(位置 i = 候補の index。0 始まり):
///  - 素早さ: 自分 150(自分の追い風で 300)。候補 i は 100 + 25 × i(相手の追い風で 2 倍)。同速は i = 2 の追い風なしのとき。
///  - 優先度: 自分 0。候補は 0、ただし i = 1 だけ 1(素早さで抜いていても先に動けない行を作る)。
///  - 自分の技の確定数 attackerKo: hits = i + 1。i が偶数なら確定(100.0)、奇数なら乱数(10.0 + 5.0 × i)。
///  - 候補の技の確定数 defenderKo: i = 0 は hits 0(倒せない・0.0)。それ以外は hits = 6 - i。i >= 4 なら確定(100.0)、それ以外は乱数(12.5 × i)。
///  - 印: 既定はすべて空。`marks` シナリオでは順方向は全行に [技 multi_hit(自分の技 ID)]、逆方向は i = 1 の行だけ [技 variable_power(その候補の技 ID)]。
final class MockJudgeServiceTests: XCTestCase {
    private static let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)

    private static func individual(_ key: String) -> JudgeIndividual {
        JudgeIndividual(speciesKey: key, natureId: "mock-nature", sp: sp)
    }

    private static func request(
        defenders count: Int, speedField: JudgeSpeedField? = nil, moveId: String = "mock-move-self"
    ) -> JudgeRequest {
        JudgeRequest(
            attacker: individual("9001-000"), moveId: moveId,
            defenders: (0..<count).map { JudgeDefender(individual: individual("9002-000"), moveId: "mock-move-\($0)") },
            speedField: speedField)
    }

    private func matchups(_ count: Int, speedField: JudgeSpeedField? = nil, scenario: MockJudgeScenario = .normal) async throws -> [JudgeMatchup] {
        try await MockJudgeService(scenario: scenario).outspeedAndKo(Self.request(defenders: count, speedField: speedField)).matchups
    }

    // MARK: - シナリオ

    func testScenarioFromEnvironmentValue() {
        XCTAssertEqual(MockJudgeScenario(environmentValue: nil), .normal)
        XCTAssertEqual(MockJudgeScenario(environmentValue: "error"), .error)
        XCTAssertEqual(MockJudgeScenario(environmentValue: "candidate-error"), .candidateError)
        XCTAssertEqual(MockJudgeScenario(environmentValue: "marks"), .marks)
        XCTAssertEqual(MockJudgeScenario(environmentValue: "something-else"), .normal)
    }

    func testEnvironmentKeyAndInitFromEnvironment() async throws {
        XCTAssertEqual(MockJudgeService.scenarioEnvironmentKey, "POKECALC_MOCK_JUDGE")
        let service = MockJudgeService(environment: [MockJudgeService.scenarioEnvironmentKey: "error"])
        let error = await assertThrowsPokeCalcError("error シナリオ") { try await service.outspeedAndKo(Self.request(defenders: 1)) }
        XCTAssertEqual(error?.code, "upstream_unavailable")
    }

    func testCandidateErrorScenarioFailsOnlyWithTwoOrMoreCandidatesAndNamesTheSecond() async throws {
        let service = MockJudgeService(scenario: .candidateError)
        let ok = try await service.outspeedAndKo(Self.request(defenders: 1))
        XCTAssertEqual(ok.matchups.count, 1)
        let error = await assertThrowsPokeCalcError("2件") { try await service.outspeedAndKo(Self.request(defenders: 2)) }
        XCTAssertEqual(error?.code, "invalid_request")
        XCTAssertTrue(error?.message.contains("defenders[1]") == true, "どの候補かを message で示す(ADR-0703 §3)")
    }

    // MARK: - 件数・順序

    func testMatchupsHaveTheSameCountAndOrderAsTheDefenders() async throws {
        for count in [1, 3, RequestLimits.maxJudgeDefenders] {
            let rows = try await matchups(count)
            XCTAssertEqual(rows.map(\.defenderIndex), Array(0..<count))
        }
    }

    func testDefenderCountOutOfRangeIsInvalidRequest() async throws {
        for count in [0, RequestLimits.maxJudgeDefenders + 1] {
            let error = await assertThrowsPokeCalcError("\(count) 件") {
                try await MockJudgeService().outspeedAndKo(Self.request(defenders: count))
            }
            XCTAssertEqual(error?.code, "invalid_request", "\(count) 件")
        }
    }

    // MARK: - 素早さ・優先度・行動順

    func testSpeedsDifferPerCandidate() async throws {
        let rows = try await matchups(4)
        XCTAssertEqual(rows.map(\.attackerSpeed), [150, 150, 150, 150])
        XCTAssertEqual(rows.map(\.defenderSpeed), [100, 125, 150, 175], "候補ごとに違う値(行の取り違えを検出する)")
    }

    func testTailwindsDoubleTheSpeeds() async throws {
        let own = try await matchups(2, speedField: JudgeSpeedField(attackerTailwind: true))
        XCTAssertEqual(own.map(\.attackerSpeed), [300, 300])
        XCTAssertEqual(own.map(\.defenderSpeed), [100, 125])
        let theirs = try await matchups(2, speedField: JudgeSpeedField(defenderTailwind: true))
        XCTAssertEqual(theirs.map(\.attackerSpeed), [150, 150])
        XCTAssertEqual(theirs.map(\.defenderSpeed), [200, 250])
    }

    func testOutspeedsTieAndTurnOrder() async throws {
        let rows = try await matchups(4)
        XCTAssertEqual(rows.map(\.outspeeds), [true, true, false, false])
        XCTAssertEqual(rows.map(\.speedTie), [false, false, true, false], "i = 2 は同速")
        XCTAssertFalse(rows[2].outspeeds && rows[2].speedTie, "outspeeds と speedTie は同時に true にならない")
        // 行動順: 優先度が違えば優先度が先(i = 1 は素早さで抜いていても相手が先)。優先度が同じなら素早さ。同速は決まらない。
        XCTAssertEqual(rows.map(\.defenderMovePriority), [0, 1, 0, 0])
        XCTAssertEqual(rows.map(\.attackerMovePriority), [0, 0, 0, 0])
        XCTAssertEqual(rows.map(\.attackerMovesFirst), [true, false, false, false])
        XCTAssertEqual(rows.map(\.turnOrderTie), [false, false, true, false])
    }

    func testTrickRoomFlipsOutspeedsButNotTheTieOrTheSpeeds() async throws {
        let rows = try await matchups(4, speedField: JudgeSpeedField(trickRoom: true))
        XCTAssertEqual(rows.map(\.defenderSpeed), [100, 125, 150, 175], "トリックルームは実数値を変えない")
        XCTAssertEqual(rows.map(\.outspeeds), [false, false, false, true], "遅い方が先に動く")
        XCTAssertEqual(rows.map(\.speedTie), [false, false, true, false], "同速は反転しない")
        XCTAssertEqual(rows.map(\.attackerMovesFirst), [false, false, false, true])
        XCTAssertEqual(rows.map(\.turnOrderTie), [false, false, true, false])
    }

    // MARK: - 確定数

    func testAttackerKoDiffersPerCandidate() async throws {
        let rows = try await matchups(4)
        XCTAssertEqual(rows[0].attackerKo, JudgeKOChance(hits: 1, guaranteed: true, displayChancePercent: 100.0))
        XCTAssertEqual(rows[1].attackerKo, JudgeKOChance(hits: 2, guaranteed: false, displayChancePercent: 15.0))
        XCTAssertEqual(rows[2].attackerKo, JudgeKOChance(hits: 3, guaranteed: true, displayChancePercent: 100.0))
        XCTAssertEqual(rows[3].attackerKo, JudgeKOChance(hits: 4, guaranteed: false, displayChancePercent: 25.0))
    }

    func testDefenderKoDiffersPerCandidate() async throws {
        let rows = try await matchups(6)
        XCTAssertEqual(rows[0].defenderKo, JudgeKOChance(hits: 0, guaranteed: false, displayChancePercent: 0.0), "倒せない")
        XCTAssertEqual(rows[1].defenderKo, JudgeKOChance(hits: 5, guaranteed: false, displayChancePercent: 12.5))
        XCTAssertEqual(rows[2].defenderKo, JudgeKOChance(hits: 4, guaranteed: false, displayChancePercent: 25.0))
        XCTAssertEqual(rows[3].defenderKo, JudgeKOChance(hits: 3, guaranteed: false, displayChancePercent: 37.5))
        XCTAssertEqual(rows[4].defenderKo, JudgeKOChance(hits: 2, guaranteed: true, displayChancePercent: 100.0))
        XCTAssertEqual(rows[5].defenderKo, JudgeKOChance(hits: 1, guaranteed: true, displayChancePercent: 100.0))
    }

    // MARK: - 未対応の印

    func testNormalScenarioHasNoMarks() async throws {
        let rows = try await matchups(3)
        XCTAssertTrue(rows.allSatisfy { $0.attackerKoUnsupported.isEmpty && $0.defenderKoUnsupported.isEmpty })
    }

    func testMarksScenarioPutsTheForwardMarkOnEveryRowAndTheReverseMarkOnTheSecondOnly() async throws {
        let request = Self.request(defenders: 3, moveId: "mock-move-self")
        let rows = try await MockJudgeService(scenario: .marks).outspeedAndKo(request).matchups
        let forward = [UnsupportedMark(target: .move, reason: .multiHit, id: "mock-move-self")]
        XCTAssertEqual(rows.map(\.attackerKoUnsupported), [forward, forward, forward], "順方向は全行に共通")
        XCTAssertEqual(
            rows.map(\.defenderKoUnsupported),
            [[], [UnsupportedMark(target: .move, reason: .variablePower, id: "mock-move-1")], []],
            "逆方向は 2 番目の候補だけ(その候補の技の ID)。順方向と混ぜない")
    }
}
