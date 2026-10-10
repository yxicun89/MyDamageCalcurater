import Foundation
import XCTest

@testable import PokeCalcCore

/// `AdjustViewModel` の目標方式(F-11。ADR-0525)のテスト用 `AdjustGoalsService`。
///
/// - 呼び出しをすべて `calls` に記録する。
/// - `.immediate` は `result`(既定は `StubAdjustGoalsFixtures.result(for:)`)か `error` を返す。
/// - `.manual` は応答を保留し、テストが `resolve(at:)` で任意の順序で返す(古い応答の上書きを確かめる)。
actor StubAdjustGoalsService: AdjustGoalsService {
    enum Mode {
        case immediate
        case manual
    }

    private var mode: Mode = .immediate
    private var error: PokeCalcError?
    private var override: AdjustGoalsResult?
    private var pending: [Int: CheckedContinuation<AdjustGoalsResult, any Error>] = [:]
    private(set) var calls: [AdjustGoalsRequest] = []

    func setMode(_ mode: Mode) { self.mode = mode }
    func setError(_ error: PokeCalcError?) { self.error = error }
    func setResult(_ result: AdjustGoalsResult?) { override = result }

    func resolve(at index: Int, with result: AdjustGoalsResult? = nil) {
        guard let continuation = pending.removeValue(forKey: index) else {
            XCTFail("保留中の呼び出しが無い: index \(index)")
            return
        }
        continuation.resume(returning: result ?? StubAdjustGoalsFixtures.result(for: calls[index]))
    }

    func waitForCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<10000 {
            if calls.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("AdjustGoalsService が \(count) 回呼ばれなかった(\(calls.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func adjustGoals(_ request: AdjustGoalsRequest) async throws -> AdjustGoalsResult {
        let index = calls.count
        calls.append(request)
        if let error { throw error }
        switch mode {
        case .immediate:
            return override ?? StubAdjustGoalsFixtures.result(for: request)
        case .manual:
            return try await withCheckedThrowingContinuation { pending[index] = $0 }
        }
    }
}

/// 架空の応答(数値に意味は無い。engine の計算の写しではない)。
enum StubAdjustGoalsFixtures {
    static let plan = AdjustGoalsPlan(
        sp: StatBlock(hp: 4, atk: 0, def: 0, spa: 0, spd: 0, spe: 20), totalSp: 24,
        stats: StatBlock(hp: 129, atk: 70, def: 70, spa: 70, spd: 70, spe: 90))

    /// 要求の目標と同じ数・同じ順の、すべて満たす結果。
    static func result(for request: AdjustGoalsRequest) -> AdjustGoalsResult {
        AdjustGoalsResult(
            feasible: true, remaining: 42, plan: plan,
            goals: request.goals.map { goal in
                switch goal.kind {
                case .outspeed:
                    return AdjustGoalOutcome(kind: .outspeed, met: true, selfSpeed: 90, opponentSpeed: 80, selfSpeedRank: 0)
                case .survive, .ko:
                    return AdjustGoalOutcome(kind: goal.kind, met: true, chancePercent: 100)
                }
            })
    }
}
