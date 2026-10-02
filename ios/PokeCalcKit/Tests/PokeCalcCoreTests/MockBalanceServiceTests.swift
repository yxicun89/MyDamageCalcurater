import XCTest

@testable import PokeCalcCore

/// `MockBalanceService`(P6-26。ADR-0505 §8): XCUITest・オフライン用の架空データ。**固定の事実**(UI テストと ADR-0505 §8 が頼る)を単体で固定する。
/// 式は本物を写したものではない(タイプ相性の正しさは保証しない。相性表を iOS に持ち込まない)。
/// i = メンバーの位置(0 始まり)・t = タイプの位置(`PokeType.allCases` 順。normal = 0・fire = 1・water = 2・electric = 3)。
final class MockBalanceServiceTests: XCTestCase {
    private func analyzeRequest(_ count: Int, ability: Bool = false) -> BalanceAnalyzeRequest {
        BalanceAnalyzeRequest(
            members: (0..<count).map { BalanceAnalyzeMember(pokemonId: "900\($0)-000", abilityId: ability && $0 == 0 ? "stub-ability" : nil) })
    }

    private func coverageRequest(_ moveCounts: [Int]) -> BalanceCoverageRequest {
        BalanceCoverageRequest(
            members: moveCounts.enumerated().map { index, count in
                BalanceCoverageMember(pokemonId: "900\(index)-000", moveIds: (0..<count).map { "stub-move-\($0)" })
            })
    }

    // MARK: - 環境変数

    func testScenarioFromEnvironmentValue() {
        XCTAssertEqual(MockBalanceScenario(environmentValue: nil), .normal)
        XCTAssertEqual(MockBalanceScenario(environmentValue: "error"), .error)
        XCTAssertEqual(MockBalanceScenario(environmentValue: "coverage-error"), .coverageError)
        XCTAssertEqual(MockBalanceScenario(environmentValue: "something-else"), .normal)
        XCTAssertEqual(MockBalanceService.scenarioEnvironmentKey, "POKECALC_MOCK_BALANCE")
    }

    // MARK: - analyze

    func testAnalyzeReturnsOneRowPerMemberInOrderWith18TypesInCanonicalOrder() async throws {
        let response = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(3))
        XCTAssertEqual(response.members.map(\.pokemonId), ["9000-000", "9001-000", "9002-000"])
        for member in response.members {
            XCTAssertEqual(member.defense.map(\.attackType), PokeType.allCases)
            XCTAssertEqual(member.types.count, 1)
        }
        XCTAssertEqual(response.teamSummary.map(\.attackType), PokeType.allCases)
        XCTAssertEqual(response.members.map { $0.types[0] }, [.normal, .water, .grass], "メンバー i のタイプは PokeType.allCases[(2 × i) % 18]")
    }

    /// 分類は (i + t) % 6 = 0 → ×4 quad_weak / 1 → ×2 weak / 2 → ×1 neutral / 3 → ×1/2 resist / 4 → ×1/4 quad_resist / 5 → ×0 immune。
    /// メンバーごとに違う値を返す(行の取り違えを画面・テストが検出できる)。
    func testAnalyzeDefenseIsAFunctionOfMemberAndTypePosition() async throws {
        let response = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(2))
        let expected: [(multiplier: String, category: BalanceDefenseCategory)] = [
            ("4", .quadWeak), ("2", .weak), ("1", .neutral), ("1/2", .resist), ("1/4", .quadResist), ("0", .immune),
        ]
        for (memberIndex, member) in response.members.enumerated() {
            for (typeIndex, entry) in member.defense.enumerated() {
                let want = expected[(memberIndex + typeIndex) % 6]
                XCTAssertEqual(entry.multiplier, want.multiplier, "i=\(memberIndex) t=\(typeIndex)")
                XCTAssertEqual(entry.category, want.category, "i=\(memberIndex) t=\(typeIndex)")
                XCTAssertEqual(entry.source, .type)
                XCTAssertEqual(entry.effect, .none)
            }
        }
        XCTAssertEqual(response.members[0].defense[0].multiplier, "4", "メンバー 0 の normal は ×4")
        XCTAssertEqual(response.members[1].defense[0].multiplier, "2", "メンバー 1 の normal は ×2(メンバーごとに違う)")
    }

    /// 特性を送ったメンバーだけ、normal(t = 0)が ×3/4・resist・source = ability・effect = multiplier になる。応答は abilityId を返す。
    func testAnalyzeAbilityMemberHasAnAbilityChangedEntry() async throws {
        let response = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(2, ability: true))
        XCTAssertEqual(response.members[0].abilityId, "stub-ability")
        XCTAssertNil(response.members[1].abilityId)
        XCTAssertEqual(
            response.members[0].defense[0],
            BalanceDefenseEntry(attackType: .normal, multiplier: "3/4", category: .resist, source: .ability, effect: .multiplier))
        XCTAssertEqual(response.members[1].defense[0].source, .type, "特性を送っていないメンバーは変わらない")
    }

    /// 集計は応答のメンバーから数えた値(不変条件: weak + resist + immune + neutral = メンバー数・quadWeak は weak の内数)。
    func testAnalyzeTeamSummaryIsConsistentWithTheMembers() async throws {
        let response = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(4))
        for entry in response.teamSummary {
            XCTAssertEqual(entry.weak + entry.resist + entry.immune + entry.neutral, 4, "\(entry.attackType)")
            XCTAssertLessThanOrEqual(entry.quadWeak, entry.weak)
        }
        // メンバー 0〜3 の normal(t = 0)は (i + 0) % 6 = 0,1,2,3 → quad_weak, weak, neutral, resist。
        XCTAssertEqual(
            response.teamSummary[0], BalanceTeamSummaryEntry(attackType: .normal, weak: 2, quadWeak: 1, resist: 1, immune: 0, neutral: 1))
    }

    func testAnalyzeRejectsOutOfRangeMemberCounts() async {
        for count in [0, RequestLimits.maxBalanceMembers + 1] {
            let error = await assertThrowsPokeCalcError("members \(count)") { try await MockBalanceService().analyzeTeamBalance(self.analyzeRequest(count)) }
            XCTAssertEqual(error?.code, "invalid_request", "\(count)")
        }
    }

    func testAnalyzeAcceptsTheLimitCounts() async throws {
        let one = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(RequestLimits.minBalanceMembers))
        XCTAssertEqual(one.members.count, 1)
        let six = try await MockBalanceService().analyzeTeamBalance(analyzeRequest(RequestLimits.maxBalanceMembers))
        XCTAssertEqual(six.members.count, 6)
    }

    // MARK: - coverage

    /// 技が 0 件のメンバーは attackTypes が空・全タイプ nil。技を持つメンバー i の attackTypes は、技 k の攻撃タイプ PokeType.allCases[(i + 5 × k) % 18] の重複なし・正準順。
    func testCoverageAttackTypesFollowTheMovePositions() async throws {
        let response = try await MockBalanceService().analyzeTeamCoverage(coverageRequest([2, 0, 3]))
        XCTAssertEqual(response.members.map(\.pokemonId), ["9000-000", "9001-000", "9002-000"])
        XCTAssertEqual(response.members[0].moveIds, ["stub-move-0", "stub-move-1"])
        XCTAssertEqual(response.members[0].attackTypes, [.normal, .ice], "i=0: k=0 → allCases[0] = normal・k=1 → allCases[5] = ice(正準順・重複なし)")
        XCTAssertEqual(response.members[1].attackTypes, [])
        XCTAssertTrue(response.members[1].coverage.allSatisfy { $0.bestMultiplier == nil && !$0.effective && !$0.superEffective })
    }

    /// 倍率は [0, 1/2, 1, 2][(i + t) % 4]。有効 = 等倍以上・抜群 = ×2。
    func testCoverageMultiplierIsAFunctionOfMemberAndTypePosition() async throws {
        let response = try await MockBalanceService().analyzeTeamCoverage(coverageRequest([1, 1]))
        let sequence: [BalanceCoverageMultiplier] = [.zero, .half, .neutral, .double]
        for (memberIndex, member) in response.members.enumerated() {
            XCTAssertEqual(member.coverage.map(\.defenseType), PokeType.allCases)
            for (typeIndex, entry) in member.coverage.enumerated() {
                let want = sequence[(memberIndex + typeIndex) % 4]
                XCTAssertEqual(entry.bestMultiplier, want, "i=\(memberIndex) t=\(typeIndex)")
                XCTAssertEqual(entry.effective, want == .neutral || want == .double)
                XCTAssertEqual(entry.superEffective, want == .double)
            }
        }
        XCTAssertEqual(response.members[0].coverage[0].bestMultiplier, .zero, "メンバー 0 の normal は ×0")
        XCTAssertEqual(response.members[1].coverage[0].bestMultiplier, .half, "メンバー 1 の normal は ×1/2(メンバーごとに違う)")
    }

    /// チームの行は、技を持つメンバーの最大倍率(順序 0 < 1/2 < 1 < 2)・有効/抜群の人数。誰も攻撃技を持たなければ nil。
    func testCoverageTeamRowsAreDerivedFromTheMembers() async throws {
        let response = try await MockBalanceService().analyzeTeamCoverage(coverageRequest([1, 0, 1]))
        // normal(t = 0): i=0 → (0+0)%4 → 0、i=2 → (2+0)%4 → 1(中立)。メンバー 1 は技なし(数えない)。
        XCTAssertEqual(
            response.teamCoverage[0], BalanceTeamCoverageEntry(defenseType: .normal, bestMultiplier: .neutral, effectiveMembers: 1, superEffectiveMembers: 0))
        let none = try await MockBalanceService().analyzeTeamCoverage(coverageRequest([0, 0]))
        XCTAssertTrue(none.teamCoverage.allSatisfy { $0.bestMultiplier == nil && $0.effectiveMembers == 0 && $0.superEffectiveMembers == 0 })
    }

    func testCoverageRejectsOutOfRangeMemberCounts() async {
        for count in [0, RequestLimits.maxBalanceMembers + 1] {
            let request = coverageRequest(Array(repeating: 1, count: count))
            let error = await assertThrowsPokeCalcError("members \(count)") { try await MockBalanceService().analyzeTeamCoverage(request) }
            XCTAssertEqual(error?.code, "invalid_request", "\(count)")
        }
    }

    // MARK: - シナリオ

    func testErrorScenarioFailsBothOperations() async {
        let service = MockBalanceService(scenario: .error)
        let analyze = await assertThrowsPokeCalcError("analyze") { try await service.analyzeTeamBalance(self.analyzeRequest(1)) }
        XCTAssertEqual(analyze?.code, "master_unavailable")
        let coverage = await assertThrowsPokeCalcError("coverage") { try await service.analyzeTeamCoverage(self.coverageRequest([1])) }
        XCTAssertEqual(coverage?.code, "master_unavailable")
    }

    /// coverage だけ失敗(`internal_error`)。analyze は成功する(片方の失敗が他方の表示を消さないことの確認用)。
    func testCoverageErrorScenarioFailsOnlyCoverage() async throws {
        let service = MockBalanceService(scenario: .coverageError)
        let analyze = try await service.analyzeTeamBalance(analyzeRequest(1))
        XCTAssertEqual(analyze.members.count, 1)
        let coverage = await assertThrowsPokeCalcError("coverage") { try await service.analyzeTeamCoverage(self.coverageRequest([1])) }
        XCTAssertEqual(coverage?.code, "internal_error")
    }

    func testInitFromEnvironmentUsesTheScenarioKey() async {
        let service = MockBalanceService(environment: ["POKECALC_MOCK_BALANCE": "error"])
        let error = await assertThrowsPokeCalcError("env") { try await service.analyzeTeamBalance(self.analyzeRequest(1)) }
        XCTAssertEqual(error?.code, "master_unavailable")
    }
}
