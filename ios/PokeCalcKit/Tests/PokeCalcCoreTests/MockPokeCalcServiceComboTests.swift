import XCTest

@testable import PokeCalcCore

/// モックの連続技(AC-i12。ADR-0535)。計算はせず、架空の決め打ち(`Resources/combo-results.json`)を要求の形に合わせて返す。
/// 値はフィクスチャの架空の数値(UI テストの文言もこの値に依存する)。
final class MockPokeCalcServiceComboTests: XCTestCase {

    private let attackerKey = "9001-000"
    private let defenderKey = "9002-000"
    private let physical = "test-move-physical-a"
    private let special = "test-move-special-b"
    private let boost = "test-move-status-a"
    private let multiHit = "test-move-multi-hit"

    private func individual(_ key: String) -> Individual {
        Individual(speciesKey: key, natureId: "test-nature-neutral", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
    }

    private func request(
        steps: [String], opponentMoveId: String? = nil, opponentSpeed: ComboOpponentSpeed = .unknown
    ) -> ComboRequest {
        ComboRequest(
            format: .single, attacker: individual(attackerKey), defender: individual(defenderKey), field: FieldState(),
            steps: steps.map { ComboStepInput(moveId: $0) }, opponentMoveId: opponentMoveId, opponentSpeed: opponentSpeed)
    }

    private func code(of operation: () async throws -> ComboResult) async -> String? {
        do {
            _ = try await operation()
            return nil
        } catch {
            return (error as? PokeCalcError)?.code
        }
    }

    func testStatusMoveFixtureHasAGuaranteedRankEffect() async throws {
        let move = try await MockPokeCalcService().move(id: boost)
        XCTAssertEqual(move.effect, MoveRankEffect(target: .user, stages: RankBlock(atk: 1, spe: 1)), "架空の変化技は攻撃 +1・素早さ +1")
        let plain = try await MockPokeCalcService().move(id: physical)
        XCTAssertNil(plain.effect)
    }

    func testResultFollowsTheRequestShape() async throws {
        let mock = try MockPokeCalcService()
        let result = try await mock.calcCombo(request(steps: [physical, boost]))
        XCTAssertEqual(result.steps.map(\.moveId), [physical, boost])
        XCTAssertNotNil(result.steps[0].damage)
        XCTAssertEqual(result.steps[0].damage?.category, .physical)
        XCTAssertNil(result.steps[1].damage, "変化技はダメージなし")
        XCTAssertEqual(result.steps.map(\.selfSpeed), [120, 180])
        XCTAssertEqual(result.steps.map(\.order), [.dependsOnOpponentSpeed, .dependsOnOpponentSpeed], "相手の素早さが分からない")
        XCTAssertEqual(result.steps.last?.line.selfSpeed, 180)
        XCTAssertEqual(result.total.minPercent, 91.4)
        XCTAssertEqual(result.total.maxPercent, 108.6)
        XCTAssertEqual(result.total.ko.displayChancePercent, 51.6)
        XCTAssertEqual(result.total.koByStep.count, 2)
        XCTAssertEqual(result.total.koByStep.last, result.total.ko)
        XCTAssertEqual(result.speed, ComboSpeedSummary(firstSelfSpeed: 120, lastSelfSpeed: 180, opponentSpeed: 0, opponentSpeedKnown: false))
        XCTAssertNil(result.opponentAttack, "相手の技が無ければ null")
    }

    func testKnownOpponentSpeedAndOpponentAttack() async throws {
        let mock = try MockPokeCalcService()
        let result = try await mock.calcCombo(request(steps: [physical, boost, physical], opponentMoveId: special, opponentSpeed: .fromDefender))
        XCTAssertEqual(result.steps.map(\.order), [.opponentFirst, .selfFirst, .selfFirst])
        XCTAssertEqual(result.speed.opponentSpeed, 150)
        XCTAssertTrue(result.speed.opponentSpeedKnown)
        let attack = try XCTUnwrap(result.opponentAttack)
        XCTAssertEqual(attack.minActions, 1)
        XCTAssertEqual(attack.maxActions, 2)
        XCTAssertEqual(attack.fast.actions.count, 2, "fast は maxActions 回")
        XCTAssertEqual(attack.slow.actions.count, 1, "slow は minActions 回")
        XCTAssertEqual(attack.fast.surviveChancePercent, 87.5)
        XCTAssertEqual(attack.fast.actions.first?.damage.category, .special, "相手の技の分類")
    }

    func testMoveMarksAreCarriedAsCalcMarks() async throws {
        let result = try await MockPokeCalcService().calcCombo(request(steps: [multiHit, physical, multiHit]))
        XCTAssertEqual(result.unsupported, [.calc(UnsupportedMark(target: .move, reason: .multiHit, id: multiHit))], "重複は 1 つ")
    }

    func testInvalidRequestsUseTheServerVocabulary() async throws {
        let mock = try MockPokeCalcService()
        let invalid = PokeCalcError.Code.invalidInput
        let notFound = PokeCalcError.Code.notFound
        let none = await code { try await mock.calcCombo(self.request(steps: [])) }
        XCTAssertEqual(none, invalid, "手 0 件")
        let tooMany = await code { try await mock.calcCombo(self.request(steps: [self.physical, self.physical, self.physical, self.physical])) }
        XCTAssertEqual(tooMany, invalid, "手 4 件")
        let plainStatus = await code { try await mock.calcCombo(self.request(steps: [self.physical, "test-move-status-plain"])) }
        XCTAssertEqual(plainStatus, notFound, "知らない技")
        let statusOpponent = await code { try await mock.calcCombo(self.request(steps: [self.physical], opponentMoveId: self.boost)) }
        XCTAssertEqual(statusOpponent, invalid, "変化技の相手の技")
        let unknownOpponent = await code { try await mock.calcCombo(self.request(steps: [self.physical], opponentMoveId: "test-move-none")) }
        XCTAssertEqual(unknownOpponent, notFound)
        var badSpecies = request(steps: [physical, boost])
        badSpecies.defender.speciesKey = "9999-000"
        let species = await code { try await mock.calcCombo(badSpecies) }
        XCTAssertEqual(species, notFound)
    }

    func testFailureScenarios() async throws {
        XCTAssertEqual(MockComboScenario.environmentKey, "POKECALC_MOCK_COMBO")
        XCTAssertEqual(MockComboScenario(environmentValue: nil), .normal)
        XCTAssertEqual(MockComboScenario(environmentValue: "unknown"), .normal)
        XCTAssertEqual(MockComboScenario(environmentValue: "fail"), .failure)
        XCTAssertEqual(MockComboScenario(environmentValue: "fail-once"), .failOnce)

        let failing = try MockPokeCalcService(comboScenario: .failure)
        for _ in 0..<2 {
            let failed = await code { try await failing.calcCombo(self.request(steps: [self.physical, self.boost])) }
            XCTAssertEqual(failed, PokeCalcError.Code.transport, "常に接続できない")
        }

        let once = try MockPokeCalcService(comboScenario: .failOnce)
        let first = await code { try await once.calcCombo(self.request(steps: [self.physical, self.boost])) }
        XCTAssertEqual(first, PokeCalcError.Code.transport)
        let second = await code { try await once.calcCombo(self.request(steps: [self.physical, self.boost])) }
        XCTAssertNil(second, "2 回目からは成功する(「もう一度」の確認用)")

        let normal = try MockPokeCalcService()
        let ok = await code { try await normal.calcCombo(self.request(steps: [self.physical, self.boost])) }
        XCTAssertNil(ok, "既定の init() は .normal")
    }
}
