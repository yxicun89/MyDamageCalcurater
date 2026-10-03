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
}
