// MockJudgeService: `JudgeService` のモック(P6-25。XCUITest・オフライン用。ADR-0504 §8)。
// 架空データ(名前は入力から引くのでモック自身は名前を持たない)。挙動は起動時の環境変数 `POKECALC_MOCK_JUDGE` で切り替える
// (`POKECALC_MOCK_SPEED` と同じ流儀)。固定する事実は MockJudgeServiceTests と ADR-0504 §8 にある。
//
// 式は本物を写したものではない(候補の位置から決まる決定的な値)。モックは判定の正しさを保証しない。
// **候補ごとに違う値を返す**(行の取り違えを画面・テストが検出できるように。ADR-0705 §8)。

public enum MockJudgeScenario: Equatable, Sendable {
    /// すべて成功・未対応の印なし(環境変数なし・未知の値の既定)。
    case normal
    /// すべて失敗(`upstream_unavailable`)。
    case error
    /// 候補が2件以上のときだけ失敗(`invalid_request`。message は `defenders[1]` を示す)。1件なら成功。候補の番号を示す表示の確認用。
    case candidateError
    /// 未対応の印を返す(順方向は全行に共通、逆方向は 2 番目の候補だけ)。方向ごとの出し分けの確認用。
    case marks

    /// 環境変数の値: `error` / `candidate-error` / `marks`。nil・未知の値は `.normal`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "error": self = .error
        case "candidate-error": self = .candidateError
        case "marks": self = .marks
        default: self = .normal
        }
    }
}

public struct MockJudgeService: JudgeService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_JUDGE"

    private let scenario: MockJudgeScenario

    public init(scenario: MockJudgeScenario = .normal) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockJudgeScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    public func outspeedAndKo(_ request: JudgeRequest) async throws -> JudgeResponse {
        let count = request.defenders.count
        guard count >= RequestLimits.minJudgeDefenders, count <= RequestLimits.maxJudgeDefenders else {
            throw PokeCalcError(code: "invalid_request", message: "defenders: 候補数が範囲外です")
        }
        switch scenario {
        case .error:
            throw PokeCalcError(code: "upstream_unavailable", message: "mock: upstream unavailable")
        case .candidateError where count >= 2:
            throw PokeCalcError(code: "invalid_request", message: "defenders[1]: mock candidate error")
        default:
            break
        }
        let field = request.speedField ?? JudgeSpeedField()
        let matchups = request.defenders.indices.map { index in
            matchup(index, request: request, field: field)
        }
        return JudgeResponse(matchups: matchups)
    }

    // MARK: - 位置 i から決まる決定的な値(ADR-0504 §8)

    private static let baseAttackerSpeed = 150
    private static let baseDefenderSpeed = 100
    private static let defenderSpeedStep = 25
    private static let tailwindMultiplier = 2
    private static let defenderPriorityIndex = 1

    private func matchup(_ index: Int, request: JudgeRequest, field: JudgeSpeedField) -> JudgeMatchup {
        let attackerSpeed = Self.baseAttackerSpeed * (field.attackerTailwind ? Self.tailwindMultiplier : 1)
        let defenderSpeed =
            (Self.baseDefenderSpeed + Self.defenderSpeedStep * index) * (field.defenderTailwind ? Self.tailwindMultiplier : 1)
        let tie = attackerSpeed == defenderSpeed
        let outspeeds = !tie && (field.trickRoom ? attackerSpeed < defenderSpeed : attackerSpeed > defenderSpeed)
        let defenderPriority = index == Self.defenderPriorityIndex ? 1 : 0
        let turnOrderTie = defenderPriority == 0 && tie
        let attackerMovesFirst = defenderPriority == 0 ? outspeeds : false
        var attackerMarks: [UnsupportedMark] = []
        var defenderMarks: [UnsupportedMark] = []
        if scenario == .marks {
            attackerMarks = [UnsupportedMark(target: .move, reason: .multiHit, id: request.moveId)]
            if index == 1 {
                defenderMarks = [UnsupportedMark(target: .move, reason: .variablePower, id: request.defenders[index].moveId)]
            }
        }
        return JudgeMatchup(
            defenderIndex: index, outspeeds: outspeeds, speedTie: tie, attackerSpeed: attackerSpeed,
            defenderSpeed: defenderSpeed, attackerMovePriority: 0, defenderMovePriority: defenderPriority,
            attackerMovesFirst: attackerMovesFirst, turnOrderTie: turnOrderTie, attackerKo: attackerKo(index),
            defenderKo: defenderKo(index), attackerKoUnsupported: attackerMarks, defenderKoUnsupported: defenderMarks)
    }

    private func attackerKo(_ index: Int) -> JudgeKOChance {
        let guaranteed = index % 2 == 0
        return JudgeKOChance(
            hits: index + 1, guaranteed: guaranteed, displayChancePercent: guaranteed ? 100.0 : 10.0 + 5.0 * Double(index))
    }

    private func defenderKo(_ index: Int) -> JudgeKOChance {
        guard index > 0 else { return JudgeKOChance(hits: 0, guaranteed: false, displayChancePercent: 0.0) }
        let guaranteed = index >= 4
        return JudgeKOChance(
            hits: 6 - index, guaranteed: guaranteed, displayChancePercent: guaranteed ? 100.0 : 12.5 * Double(index))
    }
}
