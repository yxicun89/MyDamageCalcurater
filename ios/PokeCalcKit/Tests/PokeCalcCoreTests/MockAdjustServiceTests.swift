import XCTest

@testable import PokeCalcCore

/// AJ7: `MockAdjustService`(ADR-0502 §4)。架空データを要求の形に合わせて返すだけで、計算はしない。
/// 数値そのもの(決め打ちの値)は固定しない(フィクスチャを変えてもこのテストは壊れない)。
final class MockAdjustServiceTests: XCTestCase {
    private static let own = Individual(
        speciesKey: "9001-000", natureId: "test-nature-neutral", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
    )
    private static let opponent = Individual(
        speciesKey: "9002-000", natureId: "test-nature-neutral", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
    )

    func testIndicesFirepowerFollowsMovePresence() async throws {
        let mock = try MockAdjustService()
        let withMove = try await mock.adjustIndices(AdjustIndicesRequest(individual: Self.own, moveId: "test-move-physical-a"))
        XCTAssertNotNil(withMove.firepowerIndex)
        let withoutMove = try await mock.adjustIndices(AdjustIndicesRequest(individual: Self.own))
        XCTAssertNil(withoutMove.firepowerIndex, "技を送らなければ火力指数は null(契約と同じ)")
    }

    func testSearchResultsUseTheSearchedStat() async throws {
        let mock = try MockAdjustService()
        let physical = try await mock.adjustMinSpToKo(AdjustSearchRequest(
            format: .single, attacker: Self.own, defender: Self.opponent, moveId: "test-move-physical-a", hits: 1))
        XCTAssertEqual(physical.stat, .atk)
        let special = try await mock.adjustMinSpToKo(AdjustSearchRequest(
            format: .single, attacker: Self.own, defender: Self.opponent, moveId: "test-move-special-a", hits: 1))
        XCTAssertEqual(special.stat, .spa)
        let survive = try await mock.adjustMinSpToSurvive(AdjustSearchRequest(
            format: .single, attacker: Self.opponent, defender: Self.own, moveId: "test-move-special-a", hits: 1))
        XCTAssertEqual(survive.stat, .spd)
    }

    func testMultiHitMoveCarriesUnsupportedMark() async throws {
        let mock = try MockAdjustService()
        let result = try await mock.adjustMinSpToKo(AdjustSearchRequest(
            format: .single, attacker: Self.own, defender: Self.opponent, moveId: "test-move-multi-hit", hits: 1))
        XCTAssertEqual(result.unsupported, [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-multi-hit")],
                       "計算画面のモックと同じく、技の機構から未対応の印を付ける(XCUITest で印の表示を確かめるため)")
    }

    func testAllocationMinSpFollowsGoalPresence() async throws {
        let mock = try MockAdjustService()
        let noGoal = try await mock.adjustAllocation(AdjustAllocationRequest(
            selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .bulk, focus: .both))
        XCTAssertNil(noGoal.minSp)
        let withGoal = try await mock.adjustAllocation(AdjustAllocationRequest(
            selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .offense, offenseCategory: .physical,
            goal: AdjustAllocGoal(format: .single, opponent: Self.opponent, moveId: "test-move-physical-a", hits: 1)))
        XCTAssertNotNil(withGoal.minSp)
        for plan in [noGoal.maxIndex, withGoal.maxIndex] {
            XCTAssertLessThanOrEqual(plan.totalSp, SPLimits.maxTotal)
        }
    }

    /// 攻撃側の案は H・B・D に SP を置かず、分類の攻撃と S だけに置く(耐久側の案を流用しない)。
    func testOffensePlanPutsSPOnlyOnAttackAndSpeed() async throws {
        let mock = try MockAdjustService()
        for category in [MoveCategory.physical, .special] {
            let result = try await mock.adjustAllocation(AdjustAllocationRequest(
                selfIndividual: Self.own, ceiling: AdjustCeiling(), mode: .offense, offenseCategory: category))
            let sp = result.maxIndex.sp
            XCTAssertEqual([sp.hp, sp.def, sp.spd], [0, 0, 0])
            XCTAssertEqual(category == .physical ? sp.spa : sp.atk, 0)
            XCTAssertGreaterThan(category == .physical ? sp.atk : sp.spa, 0)
        }
    }

    /// 逆引きはフィクスチャの learnset から引く(`MockPokeCalcService` の種族と一致する)。図鑑番号・フォルム番号の昇順。
    func testMoveLearnersReverseLookupMatchesFixtureLearnsets() async throws {
        let mock = try MockAdjustService()
        let master = try MockPokeCalcService()
        let all = try await master.searchSpecies(query: "", limit: MasterSearch.pageLimit)
        var expected: [SpeciesSummary] = []
        for summary in all {
            let detail = try await master.species(key: summary.key)
            if detail.learnset.contains("test-move-special-b") { expected.append(summary) }
        }
        expected.sort { ($0.dexNo, $0.form) < ($1.dexNo, $1.form) }
        XCTAssertFalse(expected.isEmpty, "フィクスチャにこの技を覚える種族がある前提")

        let learners = try await mock.moveLearners(moveId: "test-move-special-b", limit: 50, offset: 0)
        XCTAssertEqual(learners, expected)
    }

    func testMoveLearnersPaging() async throws {
        let mock = try MockAdjustService()
        let all = try await mock.moveLearners(moveId: "test-move-physical-a", limit: 50, offset: 0)
        XCTAssertGreaterThanOrEqual(all.count, 2)
        let first = try await mock.moveLearners(moveId: "test-move-physical-a", limit: 1, offset: 0)
        let second = try await mock.moveLearners(moveId: "test-move-physical-a", limit: 1, offset: 1)
        XCTAssertEqual(first + second, Array(all.prefix(2)))
        let beyond = try await mock.moveLearners(moveId: "test-move-physical-a", limit: 50, offset: all.count)
        XCTAssertEqual(beyond, [])
    }

    func testMoveLearnersUnknownMoveIsNotFound() async throws {
        let mock = try MockAdjustService()
        let error = await assertThrowsPokeCalcError("unknown move") {
            try await mock.moveLearners(moveId: "test-move-does-not-exist", limit: 50, offset: 0)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.notFound)
    }
}
