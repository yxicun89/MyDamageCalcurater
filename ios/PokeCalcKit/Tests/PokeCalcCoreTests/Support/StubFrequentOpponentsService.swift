import Foundation
import XCTest

@testable import PokeCalcCore

/// `FrequentOpponentsViewModel` のテスト用 `FrequentOpponentsService`(P6-23)。
/// 呼び出しごとに台本を1つ消費し(尽きたら空配列)、`limit` を記録する。
/// `.manual` は応答を保留し、テストが `resolve(at:with:)` で**任意の順序**で返す(古い応答が後から届く状況の再現)。
/// 保留中に Task cancel を受けたら `CancellationError` で終える。
actor StubFrequentOpponentsService: FrequentOpponentsService {
    enum Step: Sendable {
        case result([FrequentOpponent])
        case failure(PokeCalcError)
        case manual
    }

    private(set) var limits: [Int] = []
    private(set) var cancelledCalls: Set<Int> = []
    private var script: [Step]
    private var pending: [Int: CheckedContinuation<[FrequentOpponent], any Error>] = [:]

    init(_ script: [Step]) {
        self.script = script
    }

    init(keys: [String]) {
        script = [.result(keys.map { Self.opponent($0) })]
    }

    static func opponent(_ key: String, score: Double = 1) -> FrequentOpponent {
        FrequentOpponent(speciesKey: key, score: score, count: 1, lastCalculatedAt: Date(timeIntervalSince1970: 1_790_000_000))
    }

    func frequentOpponents(limit: Int) async throws -> [FrequentOpponent] {
        let index = limits.count
        limits.append(limit)
        let step = script.isEmpty ? Step.result([]) : script.removeFirst()
        switch step {
        case .result(let list): return list
        case .failure(let error): throw error
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<[FrequentOpponent], any Error>) in
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

    func resolve(at index: Int, with result: Result<[FrequentOpponent], PokeCalcError>) {
        guard let continuation = pending.removeValue(forKey: index) else {
            XCTFail("保留中の frequentOpponents が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    private func cancel(at index: Int) {
        cancelledCalls.insert(index)
        pending.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    /// `count` 回以上呼ばれるまで待つ(1ms ポーリング。壁時計の固定待ちをしない)。
    func waitForCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<5000 {
            if limits.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("frequentOpponents が \(count) 回呼ばれなかった(\(limits.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "frequentOpponents の待ち合わせがタイムアウト")
    }

    func waitForCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<5000 {
            if cancelledCalls.contains(index) { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("frequentOpponents の呼び出し \(index) が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "frequentOpponents のキャンセル待ちがタイムアウト")
    }
}
