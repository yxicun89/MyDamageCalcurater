import Foundation

// MockAdjustGoalsService: `AdjustGoalsService` のモック(F-11。ADR-0525。XCUITest と接続先なしの起動で使う)。
//
// `MockAdjustService` と同じ方針: 架空データ(9001〜9004・`test-move-*`)を要求の形に合わせて返すだけで、
// **探索・素早さの比較を計算しない**(engine の式を Swift に写さない。絶対ルール2)。
// 挙動は起動時の環境変数 `POKECALC_MOCK_ADJUST_GOALS` で切り替える(`POKECALC_MOCK_CALC_HISTORY` と同じ流儀)。

public enum MockAdjustGoalsScenario: Equatable, Sendable {
    /// すべての目標を満たす(環境変数なし・未知の値の既定)。
    case feasible
    /// 先頭の目標だけ満たせない(`feasible: false`)。
    case infeasible
    /// 常に 404 `not_found`(サーバーが目標の操作を提供していない)。
    case unavailable
    /// 常に 503 `master_unavailable`(一時的な失敗。機能なしとは区別する)。
    case failure

    /// 環境変数の値: `infeasible` / `unavailable` / `fail`。nil・未知の値は `.feasible`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "infeasible": self = .infeasible
        case "unavailable": self = .unavailable
        case "fail": self = .failure
        default: self = .feasible
        }
    }
}

public struct MockAdjustGoalsService: AdjustGoalsService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_ADJUST_GOALS"

    /// 決め打ちの案(計算ではない)。S と H・B に置いた合計 40。
    private static let plan = AdjustGoalsPlan(
        sp: StatBlock(hp: 12, atk: 0, def: 8, spa: 0, spd: 0, spe: 20), totalSp: 40,
        stats: StatBlock(hp: 155, atk: 100, def: 98, spa: 80, spd: 85, spe: 130))
    private static let selfSpeed = 130
    private static let opponentSpeed = 120
    /// 満たせなかった素早さの目標で見せる相手の素早さ(決め打ち)。
    private static let fasterOpponentSpeed = 140

    private let fixtures: MockFixtures
    private let scenario: MockAdjustGoalsScenario

    public init(scenario: MockAdjustGoalsScenario = .feasible) throws {
        fixtures = try MockFixtures.load()
        self.scenario = scenario
    }

    public init(environment: [String: String]) throws {
        try self.init(scenario: MockAdjustGoalsScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    public func adjustGoals(_ request: AdjustGoalsRequest) async throws -> AdjustGoalsResult {
        switch scenario {
        case .unavailable:
            throw PokeCalcError(code: "not_found", message: "モック: 目標の操作が無い")
        case .failure:
            throw PokeCalcError(code: "master_unavailable", message: "モック: 一時的な失敗")
        case .feasible, .infeasible:
            break
        }
        try requireSpecies(request.selfIndividual.speciesKey)
        var marks: [UnsupportedMark] = []
        for goal in request.goals {
            try requireSpecies(goal.opponent.speciesKey)
            if let moveId = goal.moveId {
                let entry = try move(moveId)
                if goal.kind != .outspeed {
                    marks += try MockPokeCalcService.moveMarks(entry, category: MockPokeCalcService.domainMoveCategory(entry.category))
                }
            }
        }
        let outcomes = request.goals.enumerated().map { index, goal in
            outcome(for: goal, met: !(scenario == .infeasible && index == 0))
        }
        return AdjustGoalsResult(
            feasible: outcomes.allSatisfy(\.met), remaining: SPLimits.maxTotal - Self.plan.totalSp, plan: Self.plan,
            goals: outcomes, unsupported: marks)
    }

    private func outcome(for goal: AdjustGoalInput, met: Bool) -> AdjustGoalOutcome {
        switch goal.kind {
        case .outspeed:
            // 先に使う技を選んだときは +1 段階の例(効果の有無はサーバーが決める。モックは技の有無だけ見る)。
            return AdjustGoalOutcome(
                kind: .outspeed, met: met, selfSpeed: Self.selfSpeed,
                opponentSpeed: met ? Self.opponentSpeed : Self.fasterOpponentSpeed,
                selfSpeedRank: goal.moveId == nil ? 0 : 1)
        case .survive, .ko:
            return AdjustGoalOutcome(kind: goal.kind, met: met, chancePercent: met ? 100 : 37.5)
        }
    }

    private func requireSpecies(_ key: String) throws {
        guard fixtures.species.contains(where: { $0.key == key }) else {
            throw MockPokeCalcService.notFoundError("種族", key)
        }
    }

    private func move(_ id: String) throws -> MockFixtures.MoveEntry {
        guard let entry = fixtures.moves.first(where: { $0.id == id }) else {
            throw MockPokeCalcService.notFoundError("技", id)
        }
        return entry
    }
}
