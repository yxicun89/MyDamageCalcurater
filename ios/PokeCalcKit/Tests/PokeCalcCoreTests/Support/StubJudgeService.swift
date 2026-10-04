import Foundation
import XCTest

@testable import PokeCalcCore

/// 判定のテスト用フィクスチャ(架空。数値に意味は無い。ADR-0002)。
enum StubJudge {
    static let none = JudgeKOChance(hits: 0, guaranteed: false, displayChancePercent: 0)

    /// 候補の位置ごとに値が違う行(行の取り違えを検出する。ADR-0705 §8)。
    /// defenderSpeed = 100 + 10 × index、attackerKo.hits = index + 1(確定)、defenderKo.hits = 5 - index(乱数 37.5%)。
    static func matchup(_ index: Int) -> JudgeMatchup {
        JudgeMatchup(
            defenderIndex: index, outspeeds: true, speedTie: false, attackerSpeed: 200, defenderSpeed: 100 + 10 * index,
            attackerMovePriority: 0, defenderMovePriority: 0, attackerMovesFirst: true, turnOrderTie: false,
            attackerKo: JudgeKOChance(hits: index + 1, guaranteed: true, displayChancePercent: 100),
            defenderKo: JudgeKOChance(hits: 5 - index, guaranteed: false, displayChancePercent: 37.5))
    }

    /// 要求の候補と同じ数・同じ順の応答(既定の応答)。
    static func echo(for request: JudgeRequest) -> JudgeResponse {
        JudgeResponse(matchups: request.defenders.indices.map { matchup($0) })
    }

    static let failure = PokeCalcError(code: "upstream_unavailable", message: "english message from server")

    /// 全項目を入れた自分・候補の個体(省略の規則の確認用)。
    static let sp = StatBlock(hp: 1, atk: 2, def: 3, spa: 4, spd: 5, spe: 6)
}

/// `JudgeViewModel` のテスト用 `JudgeService`。呼び出しを記録し、応答はクロージャで決める。
/// `hold` すると、`release` されるまで全ての呼び出しが返らない(古い応答・cancel の検証用)。
/// 既定の応答は `StubJudge.echo(for:)`。
actor StubJudgeService: JudgeService {
    private(set) var requests: [JudgeRequest] = []
    /// cancel された呼び出しの番号(0 始まり)。
    private(set) var cancelled: [Int] = []

    private var responder: @Sendable (JudgeRequest, Int) -> Result<JudgeResponse, PokeCalcError> = { request, _ in
        .success(StubJudge.echo(for: request))
    }
    private var holding = false
    private var released: Set<Int> = []
    /// true なら hold 中に cancel されても返らず、release で(古い要求でも)応答を返す。
    private var ignoresCancellation = false

    private static let pollInterval: Duration = .milliseconds(1)
    private static let pollLimit = 3000

    func setResponder(_ responder: @escaping @Sendable (JudgeRequest, Int) -> Result<JudgeResponse, PokeCalcError>) {
        self.responder = responder
    }

    func hold(ignoringCancellation: Bool = false) {
        holding = true
        ignoresCancellation = ignoringCancellation
    }

    func release(at index: Int) {
        released.insert(index)
    }

    func waitForCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if requests.count >= count { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("outspeedAndKo が \(count) 回呼ばれなかった(\(requests.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func waitForCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if cancelled.contains(index) { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("\(index) 番目の呼び出しが cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func outspeedAndKo(_ request: JudgeRequest) async throws -> JudgeResponse {
        let index = requests.count
        requests.append(request)
        if holding {
            while !released.contains(index) {
                if ignoresCancellation {
                    await Task.yield()
                    try? await Task.sleep(for: Self.pollInterval)
                    continue
                }
                do {
                    try await Task.sleep(for: Self.pollInterval)
                } catch {
                    cancelled.append(index)
                    throw CancellationError()
                }
            }
            if !ignoresCancellation, Task.isCancelled {
                cancelled.append(index)
                throw CancellationError()
            }
        }
        return try responder(request, index).get()
    }
}
