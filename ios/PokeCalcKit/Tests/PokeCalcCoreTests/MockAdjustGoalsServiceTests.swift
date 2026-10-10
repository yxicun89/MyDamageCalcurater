import XCTest

@testable import PokeCalcCore

/// F-11: `MockAdjustGoalsService`(ADR-0525)。架空データを要求の形に合わせて返すだけで、計算はしない。
final class MockAdjustGoalsServiceTests: XCTestCase {
    private static let own = Individual(speciesKey: "9001-000", natureId: "test-nature-neutral", sp: zeroSP)
    private static let opponent = Individual(speciesKey: "9002-000", natureId: "test-nature-neutral", sp: zeroSP)

    private static func request(_ kinds: [AdjustGoalKind], moveId: String? = "test-move-physical-a") -> AdjustGoalsRequest {
        AdjustGoalsRequest(
            format: .single, selfIndividual: own,
            goals: kinds.map { AdjustGoalInput(kind: $0, opponent: opponent, moveId: $0 == .outspeed ? nil : moveId, hits: $0 == .outspeed ? nil : 1) })
    }

    func testFeasibleReturnsOneOutcomePerGoalInOrder() async throws {
        let result = try await MockAdjustGoalsService().adjustGoals(Self.request([.ko, .outspeed, .survive]))
        XCTAssertTrue(result.feasible)
        XCTAssertEqual(result.goals.map(\.kind), [.ko, .outspeed, .survive])
        XCTAssertTrue(result.goals.allSatisfy(\.met))
        XCTAssertEqual(result.remaining, SPLimits.maxTotal - result.plan.totalSp)
        XCTAssertLessThanOrEqual(result.plan.totalSp, SPLimits.maxTotal)
        XCTAssertNil(result.goals[1].chancePercent, "素早さに確率は無い")
        XCTAssertNotNil(result.goals[1].selfSpeed)
        XCTAssertNotNil(result.goals[0].chancePercent)
    }

    func testBoostMoveRaisesRankInTheCannedAnswer() async throws {
        var request = Self.request([.outspeed])
        request.goals[0].moveId = "test-move-physical-a"
        let result = try await MockAdjustGoalsService().adjustGoals(request)
        XCTAssertEqual(result.goals[0].selfSpeedRank, 1)
        let plain = try await MockAdjustGoalsService().adjustGoals(Self.request([.outspeed]))
        XCTAssertEqual(plain.goals[0].selfSpeedRank, 0)
    }

    func testInfeasibleScenarioFailsTheFirstGoalOnly() async throws {
        let result = try await MockAdjustGoalsService(scenario: .infeasible).adjustGoals(Self.request([.outspeed, .ko]))
        XCTAssertFalse(result.feasible)
        XCTAssertEqual(result.goals.map(\.met), [false, true])
    }

    func testUnavailableAndFailureScenariosThrowTheirCodes() async throws {
        let unavailable = await assertThrowsPokeCalcError("unavailable") {
            try await MockAdjustGoalsService(scenario: .unavailable).adjustGoals(Self.request([.outspeed]))
        }
        XCTAssertEqual(unavailable?.code, "not_found")
        XCTAssertTrue(AdjustGoalsAvailability.isUnavailable(try XCTUnwrap(unavailable)))
        let failure = await assertThrowsPokeCalcError("fail") {
            try await MockAdjustGoalsService(scenario: .failure).adjustGoals(Self.request([.outspeed]))
        }
        XCTAssertEqual(failure?.code, "master_unavailable")
        XCTAssertFalse(AdjustGoalsAvailability.isUnavailable(try XCTUnwrap(failure)))
    }

    func testUnknownSpeciesIsNotFound() async throws {
        var request = Self.request([.outspeed])
        request.goals[0].opponent.speciesKey = "0000-000"
        let error = await assertThrowsPokeCalcError("species") { try await MockAdjustGoalsService().adjustGoals(request) }
        XCTAssertEqual(error?.code, "not_found")
    }

    func testMultiHitMoveCarriesUnsupportedMark() async throws {
        let result = try await MockAdjustGoalsService().adjustGoals(Self.request([.ko], moveId: "test-move-multi-hit"))
        XCTAssertEqual(result.unsupported, [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-multi-hit")])
    }

    func testScenarioFromEnvironment() {
        XCTAssertEqual(MockAdjustGoalsScenario(environmentValue: nil), .feasible)
        XCTAssertEqual(MockAdjustGoalsScenario(environmentValue: "infeasible"), .infeasible)
        XCTAssertEqual(MockAdjustGoalsScenario(environmentValue: "unavailable"), .unavailable)
        XCTAssertEqual(MockAdjustGoalsScenario(environmentValue: "fail"), .failure)
        XCTAssertEqual(MockAdjustGoalsScenario(environmentValue: "unknown"), .feasible)
    }

    func testFixtureHasASpeedUpNatureForTheFastestPreset() async throws {
        let natures = try await MockPokeCalcService().natures()
        XCTAssertNoThrow(try AdjustSpeedPreset.fastest.build(natures: natures))
        XCTAssertNoThrow(try AdjustSpeedPreset.neutralMax.build(natures: natures))
    }
}
