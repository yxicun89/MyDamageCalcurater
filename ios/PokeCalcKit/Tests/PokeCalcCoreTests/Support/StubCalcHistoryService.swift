import Foundation
import XCTest

@testable import PokeCalcCore

/// `CalcHistoryViewModel` のテスト用 `CalcHistoryService`。呼び出しごとに台本を1つ消費し(尽きたら空ページ)、
/// `limit` と `cursor` を記録する。`.manual` は応答を保留し、テストが `resolve(at:with:)` で**任意の順序**で返す
/// (古い応答が後から届く状況の再現)。保留中に Task cancel を受けたら `CancellationError` で終える。
actor StubCalcHistoryService: CalcHistoryService {
    enum Step: Sendable {
        case page(CalcHistoryPage)
        case failure(PokeCalcError)
        case manual
    }

    struct Call: Equatable, Sendable {
        let limit: Int
        let cursor: String?
    }

    private(set) var calls: [Call] = []
    private var script: [Step]
    private var pending: [Int: CheckedContinuation<CalcHistoryPage, any Error>] = [:]

    init(_ script: [Step]) {
        self.script = script
    }

    static func entry(
        _ index: Int, attacker: String = "9001-000", defender: String = "9002-000", move: String = "test-move-a"
    ) -> CalcHistoryEntry {
        CalcHistoryEntry(
            occurredAt: Date(timeIntervalSince1970: 1_790_000_000 - TimeInterval(index) * 60),
            calc: CalcHistoryCalc(
                format: .single,
                attacker: Individual(
                    speciesKey: attacker, natureId: "test-nature-neutral",
                    sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32)),
                defender: Individual(
                    speciesKey: defender, natureId: "test-nature-neutral",
                    sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0)),
                moveId: move),
            minPercent: 10 + Double(index), maxPercent: 12 + Double(index))
    }

    static func page(_ indexes: Range<Int>, next: String?) -> CalcHistoryPage {
        CalcHistoryPage(items: indexes.map { entry($0) }, nextCursor: next)
    }

    func calcHistory(limit: Int, cursor: String?) async throws -> CalcHistoryPage {
        let index = calls.count
        calls.append(Call(limit: limit, cursor: cursor))
        let step = script.isEmpty ? Step.page(CalcHistoryPage(items: [], nextCursor: nil)) : script.removeFirst()
        switch step {
        case .page(let page): return page
        case .failure(let error): throw error
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<CalcHistoryPage, any Error>) in
                    pending[index] = continuation
                }
            } onCancel: {
                Task { await self.cancel(at: index) }
            }
        }
    }

    func resolve(at index: Int, with result: Result<CalcHistoryPage, PokeCalcError>) {
        guard let continuation = pending.removeValue(forKey: index) else {
            XCTFail("保留中の calcHistory が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    private func cancel(at index: Int) {
        pending.removeValue(forKey: index)?.resume(throwing: CancellationError())
    }

    /// `count` 回以上呼ばれるまで待つ(1ms ポーリング)。
    func waitForCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<5000 {
            if calls.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("calcHistory が \(count) 回呼ばれなかった(\(calls.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "calcHistory の待ち合わせがタイムアウト")
    }
}
