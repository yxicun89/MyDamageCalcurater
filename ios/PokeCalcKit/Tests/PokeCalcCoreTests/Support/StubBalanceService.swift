import Foundation
import XCTest

@testable import PokeCalcCore

/// `BalanceViewModel` のテスト用の `BalanceService`(ADR-0415)。`StubPokeCalcService` と同じ流儀:
/// `actor`、呼び出しをすべて記録し、`.manual` のときは応答を保留してテストが**任意の順序**で返す
/// (古い要求の応答が後から届く状況を再現する)。
actor StubBalanceService: BalanceService {
    enum Mode {
        /// 要求を受けたらすぐ既定の応答(要求の形を写したもの)を返す。
        case immediate
        /// 応答を保留する。テストが `resolveAnalyze(at:with:)` / `resolveCoverage(at:with:)` で返す。
        case manual
    }

    /// 要求の到着を待つ上限(1ms × 回数)。`StubPokeCalcService` と同じ考え方(無限に待たない)。
    private static let waitPollLimit = 10000

    private var analyzeMode: Mode = .immediate
    private var coverageMode: Mode = .immediate
    private var analyzeError: PokeCalcError?
    private var coverageError: PokeCalcError?

    private(set) var analyzeRequests: [[BalanceMemberInput]] = []
    private(set) var coverageRequests: [[BalanceMemberInput]] = []
    private var pendingAnalyze: [Int: CheckedContinuation<BalanceDefenseAnalysis, any Error>] = [:]
    private var pendingCoverage: [Int: CheckedContinuation<BalanceCoverageAnalysis, any Error>] = [:]
    /// Task cancel を受け取った要求番号(`analyzeRequests` / `coverageRequests` の添字)。
    private(set) var cancelledAnalyzeRequests: Set<Int> = []
    private(set) var cancelledCoverageRequests: Set<Int> = []

    // MARK: - テストからの操作

    func setAnalyzeMode(_ mode: Mode) { analyzeMode = mode }
    func setCoverageMode(_ mode: Mode) { coverageMode = mode }
    func setAnalyzeError(_ error: PokeCalcError?) { analyzeError = error }
    func setCoverageError(_ error: PokeCalcError?) { coverageError = error }

    func resolveAnalyze(at index: Int, with result: Result<BalanceDefenseAnalysis, PokeCalcError>) {
        guard let continuation = pendingAnalyze.removeValue(forKey: index) else {
            XCTFail("保留中の analyze が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func resolveCoverage(at index: Int, with result: Result<BalanceCoverageAnalysis, PokeCalcError>) {
        guard let continuation = pendingCoverage.removeValue(forKey: index) else {
            XCTFail("保留中の coverage が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// `analyzeRequests` が `count` 件になるまで待つ(届かなければ上限でテストを失敗させる)。
    func waitForAnalyzeRequests(_ count: Int) async {
        for _ in 0..<Self.waitPollLimit where analyzeRequests.count < count {
            try? await Task.sleep(for: .milliseconds(1))
        }
        if analyzeRequests.count < count { XCTFail("analyze の要求が \(count) 件に届かない: \(analyzeRequests.count)") }
    }

    func waitForCoverageRequests(_ count: Int) async {
        for _ in 0..<Self.waitPollLimit where coverageRequests.count < count {
            try? await Task.sleep(for: .milliseconds(1))
        }
        if coverageRequests.count < count { XCTFail("coverage の要求が \(count) 件に届かない: \(coverageRequests.count)") }
    }

    // MARK: - BalanceService

    func analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis {
        let index = analyzeRequests.count
        analyzeRequests.append(members)
        if let analyzeError { throw analyzeError }
        switch analyzeMode {
        case .immediate:
            return BalanceFixtures.defenseAnalysis(memberIds: members.map(\.pokemonId))
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { continuation in
                    pendingAnalyze[index] = continuation
                }
            } onCancel: {
                Task { await self.markAnalyzeCancelled(index) }
            }
        }
    }

    func coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis {
        let index = coverageRequests.count
        coverageRequests.append(members)
        if let coverageError { throw coverageError }
        switch coverageMode {
        case .immediate:
            return BalanceFixtures.coverageAnalysis(memberIds: members.map(\.pokemonId))
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { continuation in
                    pendingCoverage[index] = continuation
                }
            } onCancel: {
                Task { await self.markCoverageCancelled(index) }
            }
        }
    }

    // MARK: - 第3段(threats・recommendations・moveRange)。analyze/coverage と同じ流儀で、それぞれ独立に記録・保留する

    struct ThreatsRequest: Equatable {
        let members: [BalanceMemberInput]
        let threats: [BalanceMemberInput]
    }

    struct RecommendationsRequest: Equatable {
        let members: [BalanceMemberInput]
        let limit: Int?
    }

    private var threatsMode: Mode = .immediate
    private var recommendationsMode: Mode = .immediate
    private var moveRangeMode: Mode = .immediate
    private var threatsError: PokeCalcError?
    private var recommendationsError: PokeCalcError?
    private var moveRangeError: PokeCalcError?
    private(set) var threatsRequests: [ThreatsRequest] = []
    private(set) var recommendationsRequests: [RecommendationsRequest] = []
    private(set) var moveRangeRequests: [[String]] = []
    private var pendingThreats: [Int: CheckedContinuation<BalanceThreatsAnalysis, any Error>] = [:]
    private var pendingRecommendations: [Int: CheckedContinuation<BalanceRecommendations, any Error>] = [:]
    private var pendingMoveRange: [Int: CheckedContinuation<BalanceMoveRange, any Error>] = [:]
    private(set) var cancelledThreatsRequests: Set<Int> = []
    private(set) var cancelledRecommendationsRequests: Set<Int> = []
    private(set) var cancelledMoveRangeRequests: Set<Int> = []
    /// 即時応答の中身をテストが差し替える(nil なら要求の形を写した既定の応答)。
    private var recommendationsResult: BalanceRecommendations?
    private var moveRangeResult: BalanceMoveRange?

    func setThreatsMode(_ mode: Mode) { threatsMode = mode }
    func setRecommendationsMode(_ mode: Mode) { recommendationsMode = mode }
    func setMoveRangeMode(_ mode: Mode) { moveRangeMode = mode }
    func setThreatsError(_ error: PokeCalcError?) { threatsError = error }
    func setRecommendationsError(_ error: PokeCalcError?) { recommendationsError = error }
    func setMoveRangeError(_ error: PokeCalcError?) { moveRangeError = error }
    func setRecommendationsResult(_ result: BalanceRecommendations?) { recommendationsResult = result }
    func setMoveRangeResult(_ result: BalanceMoveRange?) { moveRangeResult = result }

    func resolveThreats(at index: Int, with result: Result<BalanceThreatsAnalysis, PokeCalcError>) {
        guard let continuation = pendingThreats.removeValue(forKey: index) else {
            XCTFail("保留中の threats が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func resolveRecommendations(at index: Int, with result: Result<BalanceRecommendations, PokeCalcError>) {
        guard let continuation = pendingRecommendations.removeValue(forKey: index) else {
            XCTFail("保留中の recommendations が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func resolveMoveRange(at index: Int, with result: Result<BalanceMoveRange, PokeCalcError>) {
        guard let continuation = pendingMoveRange.removeValue(forKey: index) else {
            XCTFail("保留中の moveRange が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func waitForThreatsRequests(_ count: Int) async {
        for _ in 0..<Self.waitPollLimit where threatsRequests.count < count {
            try? await Task.sleep(for: .milliseconds(1))
        }
        if threatsRequests.count < count { XCTFail("threats の要求が \(count) 件に届かない: \(threatsRequests.count)") }
    }

    func waitForRecommendationsRequests(_ count: Int) async {
        for _ in 0..<Self.waitPollLimit where recommendationsRequests.count < count {
            try? await Task.sleep(for: .milliseconds(1))
        }
        if recommendationsRequests.count < count {
            XCTFail("recommendations の要求が \(count) 件に届かない: \(recommendationsRequests.count)")
        }
    }

    func waitForMoveRangeRequests(_ count: Int) async {
        for _ in 0..<Self.waitPollLimit where moveRangeRequests.count < count {
            try? await Task.sleep(for: .milliseconds(1))
        }
        if moveRangeRequests.count < count { XCTFail("moveRange の要求が \(count) 件に届かない: \(moveRangeRequests.count)") }
    }

    func threats(members: [BalanceMemberInput], threats: [BalanceMemberInput]) async throws -> BalanceThreatsAnalysis {
        let index = threatsRequests.count
        threatsRequests.append(ThreatsRequest(members: members, threats: threats))
        if let threatsError { throw threatsError }
        switch threatsMode {
        case .immediate:
            return BalanceFixtures.threatsAnalysis(threatIds: threats.map(\.pokemonId), memberIds: members.map(\.pokemonId))
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { continuation in
                    pendingThreats[index] = continuation
                }
            } onCancel: {
                Task { await self.markThreatsCancelled(index) }
            }
        }
    }

    func recommendations(members: [BalanceMemberInput], limit: Int?) async throws -> BalanceRecommendations {
        let index = recommendationsRequests.count
        recommendationsRequests.append(RecommendationsRequest(members: members, limit: limit))
        if let recommendationsError { throw recommendationsError }
        switch recommendationsMode {
        case .immediate:
            return recommendationsResult ?? BalanceFixtures.recommendations(tag: members.count)
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { continuation in
                    pendingRecommendations[index] = continuation
                }
            } onCancel: {
                Task { await self.markRecommendationsCancelled(index) }
            }
        }
    }

    func moveRange(moveIds: [String]) async throws -> BalanceMoveRange {
        let index = moveRangeRequests.count
        moveRangeRequests.append(moveIds)
        if let moveRangeError { throw moveRangeError }
        switch moveRangeMode {
        case .immediate:
            return moveRangeResult ?? BalanceFixtures.moveRange()
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { continuation in
                    pendingMoveRange[index] = continuation
                }
            } onCancel: {
                Task { await self.markMoveRangeCancelled(index) }
            }
        }
    }

    private func markThreatsCancelled(_ index: Int) {
        cancelledThreatsRequests.insert(index)
        pendingThreats.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    private func markRecommendationsCancelled(_ index: Int) {
        cancelledRecommendationsRequests.insert(index)
        pendingRecommendations.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    private func markMoveRangeCancelled(_ index: Int) {
        cancelledMoveRangeRequests.insert(index)
        pendingMoveRange.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    private func markAnalyzeCancelled(_ index: Int) {
        cancelledAnalyzeRequests.insert(index)
        pendingAnalyze.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    private func markCoverageCancelled(_ index: Int) {
        cancelledCoverageRequests.insert(index)
        pendingCoverage.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }
}

/// balance の応答の架空フィクスチャ(タイプ順は契約の正準順 = `PokeType.allCases`)。倍率の値に意味は無い。
enum BalanceFixtures {
    static func defenseEntries(
        multiplier: String = "1", category: BalanceDefenseCategory = .neutral
    ) -> [BalanceDefenseEntry] {
        PokeType.allCases.map {
            BalanceDefenseEntry(attackType: $0, multiplier: multiplier, category: category, source: .type, effect: .none)
        }
    }

    /// 全メンバー・全18タイプを「等倍」にした応答(メンバー数だけが要求と一致する)。
    static func defenseAnalysis(memberIds: [String]) -> BalanceDefenseAnalysis {
        BalanceDefenseAnalysis(
            members: memberIds.map {
                BalanceMemberDefense(pokemonId: $0, abilityId: nil, types: [.normal], defense: defenseEntries())
            },
            teamSummary: PokeType.allCases.map {
                BalanceTeamSummaryEntry(attackType: $0, weak: 0, quadWeak: 0, resist: 0, immune: 0, neutral: memberIds.count)
            }
        )
    }

    static func coverageEntries(best: BalanceCoverageMultiplier?) -> [BalanceCoverageEntry] {
        PokeType.allCases.map {
            BalanceCoverageEntry(
                defenseType: $0, bestMultiplier: best,
                effective: best == .neutral || best == .double, superEffective: best == .double
            )
        }
    }

    static func coverageAnalysis(memberIds: [String]) -> BalanceCoverageAnalysis {
        BalanceCoverageAnalysis(
            members: memberIds.map {
                BalanceMemberCoverage(pokemonId: $0, moveIds: [], attackTypes: [], coverage: coverageEntries(best: nil))
            },
            teamCoverage: PokeType.allCases.map {
                BalanceTeamCoverageEntry(defenseType: $0, bestMultiplier: nil, effectiveMembers: 0, superEffectiveMembers: 0)
            }
        )
    }

    // MARK: - 第3段

    /// 仮想敵ごとに、全メンバーとの相性を1行ずつ持つ応答(倍率に意味は無い)。
    static func threatsAnalysis(threatIds: [String], memberIds: [String]) -> BalanceThreatsAnalysis {
        BalanceThreatsAnalysis(
            threats: threatIds.map { threatId in
                BalanceThreatResult(
                    pokemonId: threatId, abilityId: nil, attackTypes: [.fire],
                    matchups: memberIds.map {
                        BalanceThreatMatchup(pokemonId: $0, incoming: "1", outgoing: nil, safe: false, superEffective: false)
                    },
                    safeMembers: 0, superEffectiveMembers: 0)
            })
    }

    /// `tag`(メンバー数)ぶんだけ防御の穴の数が変わる、世代の区別用の応答。
    static func recommendations(tag: Int) -> BalanceRecommendations {
        BalanceRecommendations(
            defenseHoles: Array(PokeType.allCases.prefix(tag)), offenseHoles: [], candidates: [], abilityOptions: [])
    }

    static func moveRangeEntries(best: BalanceCoverageMultiplier = .neutral) -> [BalanceMoveRangeEntry] {
        PokeType.allCases.map {
            BalanceMoveRangeEntry(
                defenseType: $0, bestMultiplier: best, effective: best == .neutral || best == .double,
                superEffective: best == .double)
        }
    }

    static func moveRange(
        attackTypes: [PokeType] = [.normal], walledBy: [BalanceWalledByPokemon] = [],
        walledByAbility: [BalanceWalledByAbilityPokemon] = []
    ) -> BalanceMoveRange {
        BalanceMoveRange(
            attackTypes: attackTypes, typeChart: moveRangeEntries(), walledBy: walledBy, walledByAbility: walledByAbility)
    }
}
