import Foundation
import XCTest

@testable import PokeCalcCore

/// `AdjustViewModel` のテスト用の `AdjustService`(AJ7。ADR-0502)。
///
/// - 呼び出しをすべて `calls` に記録する(呼ばれた順。添字が要求番号)。
/// - `.immediate` は既定の架空の応答(`StubAdjustFixtures`)か `setError` のエラーを返す。
/// - `.manual` は応答を保留し、テストが `resolve(at:with:)` で**任意の順序**で返す。保留中に Task cancel を
///   受けたら `CancellationError` で終え、`cancelledCalls` に記録する(`StubPokeCalcService` と同じ形)。
actor StubAdjustService: AdjustService {
    enum Mode {
        case immediate
        case manual
    }

    /// 1回の呼び出しの記録。
    enum Call: Equatable, Sendable {
        case indices(AdjustIndicesRequest)
        case ko(AdjustSearchRequest)
        case survive(AdjustSearchRequest)
        case allocation(AdjustAllocationRequest)
        case learners(moveId: String, limit: Int, offset: Int)

        var kind: Kind {
            switch self {
            case .indices: return .indices
            case .ko: return .ko
            case .survive: return .survive
            case .allocation: return .allocation
            case .learners: return .learners
            }
        }
    }

    enum Kind: Hashable, Sendable {
        case indices, ko, survive, allocation, learners
    }

    /// `.manual` でテストが返す応答。
    enum Response: Sendable {
        case indices(AdjustIndicesResult)
        case ko(AdjustKOResult)
        case survive(AdjustSurviveResult)
        case allocation(AdjustAllocationResult)
        case learners([SpeciesSummary])
    }

    private static let waitPollLimit = 10000

    private var mode: Mode = .immediate
    private var errors: [Kind: PokeCalcError] = [:]
    /// `.immediate` の learners の応答(既定は空)。
    private var learnersResponder: @Sendable (String, Int, Int) -> [SpeciesSummary] = { _, _, _ in [] }
    /// `.immediate` の ko / survive / allocation の応答の上書き(既定は `StubAdjustFixtures`)。
    private var koResult = StubAdjustFixtures.koResult
    private var surviveResult = StubAdjustFixtures.surviveResult
    private var allocationResult = StubAdjustFixtures.allocationResult

    private(set) var calls: [Call] = []
    private(set) var cancelledCalls: Set<Int> = []
    private var pending: [Int: CheckedContinuation<Response, any Error>] = [:]

    // MARK: - テストからの操作

    func setMode(_ mode: Mode) { self.mode = mode }
    func setError(_ error: PokeCalcError?, for kind: Kind) { errors[kind] = error }
    func setLearnersResponder(_ responder: @escaping @Sendable (String, Int, Int) -> [SpeciesSummary]) {
        learnersResponder = responder
    }
    func setKOResult(_ result: AdjustKOResult) { koResult = result }
    func setSurviveResult(_ result: AdjustSurviveResult) { surviveResult = result }
    func setAllocationResult(_ result: AdjustAllocationResult) { allocationResult = result }

    func calls(of kind: Kind) -> [Call] { calls.filter { $0.kind == kind } }

    /// 保留中の `index` 番目(`calls` の添字)に応答する。
    func resolve(at index: Int, with result: Result<Response, PokeCalcError>) {
        guard let continuation = pending.removeValue(forKey: index) else {
            XCTFail("保留中の呼び出しが無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// 保留中の `index` 番目に、その操作の既定の応答を返す。
    func resolveWithDefault(at index: Int) {
        guard index < calls.count else {
            XCTFail("呼び出しが無い: index \(index)")
            return
        }
        resolve(at: index, with: .success(defaultResponse(for: calls[index])))
    }

    func waitForCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if calls.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("AdjustService が \(count) 回呼ばれなかった(\(calls.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "AdjustService の待ち合わせがタイムアウト")
    }

    func waitForCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if cancelledCalls.contains(index) { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("呼び出し \(index) が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "AdjustService のキャンセル待ちがタイムアウト")
    }

    // MARK: - AdjustService

    func adjustIndices(_ request: AdjustIndicesRequest) async throws -> AdjustIndicesResult {
        guard case .indices(let result) = try await handle(.indices(request)) else { throw Self.mismatch }
        return result
    }

    func adjustMinSpToKo(_ request: AdjustSearchRequest) async throws -> AdjustKOResult {
        guard case .ko(let result) = try await handle(.ko(request)) else { throw Self.mismatch }
        return result
    }

    func adjustMinSpToSurvive(_ request: AdjustSearchRequest) async throws -> AdjustSurviveResult {
        guard case .survive(let result) = try await handle(.survive(request)) else { throw Self.mismatch }
        return result
    }

    func adjustAllocation(_ request: AdjustAllocationRequest) async throws -> AdjustAllocationResult {
        guard case .allocation(let result) = try await handle(.allocation(request)) else { throw Self.mismatch }
        return result
    }

    func moveLearners(moveId: String, limit: Int, offset: Int) async throws -> [SpeciesSummary] {
        guard case .learners(let result) = try await handle(.learners(moveId: moveId, limit: limit, offset: offset)) else {
            throw Self.mismatch
        }
        return result
    }

    // MARK: - 内部

    private static let mismatch = PokeCalcError(code: "test_response_mismatch", message: "テスト: 応答の種類が呼び出しと違う")

    private func defaultResponse(for call: Call) -> Response {
        switch call {
        case .indices(let request):
            return .indices(StubAdjustFixtures.indicesResult(firepower: request.moveId != nil))
        case .ko: return .ko(koResult)
        case .survive: return .survive(surviveResult)
        case .allocation(let request):
            var result = allocationResult
            if request.goal == nil { result.minSp = nil }
            return .allocation(result)
        case .learners(let moveId, let limit, let offset):
            return .learners(learnersResponder(moveId, limit, offset))
        }
    }

    private func cancel(at index: Int) {
        cancelledCalls.insert(index)
        if let continuation = pending.removeValue(forKey: index) {
            continuation.resume(throwing: CancellationError())
        }
    }

    private func handle(_ call: Call) async throws -> Response {
        let index = calls.count
        calls.append(call)
        if let error = errors[call.kind] { throw error }
        switch mode {
        case .immediate:
            return defaultResponse(for: call)
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Response, any Error>) in
                    if cancelledCalls.contains(index) {
                        continuation.resume(throwing: CancellationError())
                    } else {
                        pending[index] = continuation
                    }
                }
            } onCancel: {
                Task { await self.cancel(at: index) }
            }
        }
    }
}

/// 架空の応答(数値に意味は無い。engine の計算の写しではない)。
enum StubAdjustFixtures {
    static let stats = StatBlock(hp: 125, atk: 70, def: 70, spa: 70, spd: 70, spe: 70)

    static func indicesResult(firepower: Bool) -> AdjustIndicesResult {
        AdjustIndicesResult(
            stats: stats, firepowerIndex: firepower ? 4200 : nil, physicalBulkIndex: 8750, specialBulkIndex: 8750,
            hpLines: HPLineReport(
                hp: 125, sp: 0, current: .none,
                next16n: HPLinePoint(hp: 128, sp: 3, spDelta: 3), prev16n: nil,
                next16nMinus1: HPLinePoint(hp: 127, sp: 2, spDelta: 2), prev16nMinus1: nil
            )
        )
    }

    static let koResult = AdjustKOResult(stat: .atk, searchLimit: 32, feasible: true, sp: 12, chancePercent: 100)
    static let surviveResult = AdjustSurviveResult(
        stat: .def, searchLimit: 66, feasible: true, hpSp: 20, statSp: 8, totalSp: 28, bulkIndex: 9000, chancePercent: 100
    )
    static let plan = AdjustAllocPlan(
        sp: StatBlock(hp: 32, atk: 0, def: 17, spa: 0, spd: 17, spe: 0), totalSp: 66, stats: stats,
        physicalBulk: 10000, specialBulk: 10000, speedMet: true, goalMet: true, chancePercent: 100
    )
    static let allocationResult = AdjustAllocationResult(remaining: 66, maxIndex: plan, minSp: plan)
}
