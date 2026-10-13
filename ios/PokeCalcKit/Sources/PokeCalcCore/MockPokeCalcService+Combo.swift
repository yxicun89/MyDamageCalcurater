import Foundation

// MockPokeCalcService+Combo: 連続技のモック(CB-8。ADR-0535・ADR-0501「連続技」AC-i12)。
//
// `MockPokeCalcService` と同じ方針: 計算はしない。`Resources/combo-results.json` の架空の決め打ちを、
// 要求の形(手の数・変化技・相手の技・相手の素早さ)に合わせて返すだけ。ランク・ダメージ・確率の式は書かない
// (engine の式を Swift に写さない。絶対ルール2・ADR-0500 §4)。

/// 連続技の失敗シナリオ(環境変数 `POKECALC_MOCK_COMBO`。XCUITest が失敗表示と「もう一度」を確かめる)。
public enum MockComboScenario: Equatable, Sendable {
    case normal
    /// 常に接続できない。
    case failure
    /// 最初の 1 回だけ接続できない(「もう一度」で成功する確認用)。
    case failOnce

    public static let environmentKey = "POKECALC_MOCK_COMBO"

    /// 環境変数の値から作る(未設定・知らない値は `.normal`)。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "fail": self = .failure
        case "fail-once": self = .failOnce
        default: self = .normal
        }
    }
}

/// `.failOnce` の「失敗済みか」。サービスのコピー間で共有する(参照型。`NSLock` で保護)。
final class MockComboFailureLatch: @unchecked Sendable {
    private let lock = NSLock()
    private var hasFailed = false

    /// 初めて呼ばれたときだけ true を返す。
    func tripOnce() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard !hasFailed else { return false }
        hasFailed = true
        return true
    }
}

extension MockPokeCalcService: ComboCalcService {
    private static let singleResultKey = "single"

    public func calcCombo(_ request: ComboRequest) async throws -> ComboResult {
        switch comboScenario {
        case .normal: break
        case .failure: throw Self.comboTransportError()
        case .failOnce: if comboFailureLatch.tripOnce() { throw Self.comboTransportError() }
        }
        guard (1...RequestLimits.maxComboSteps).contains(request.steps.count) else {
            throw PokeCalcError(
                code: PokeCalcError.Code.invalidInput, message: "steps は 1〜\(RequestLimits.maxComboSteps) 件: \(request.steps.count)")
        }
        let stepMoves = try request.steps.map { try comboMove(id: $0.moveId) }
        let opponentMove = try request.opponentMoveId.map { try comboMove(id: $0) }
        for key in [request.attacker.speciesKey, request.defender.speciesKey]
        where !fixtures.species.contains(where: { $0.key == key }) {
            throw Self.notFoundError("種族", key)
        }
        for move in stepMoves where !ComboMoveRules.canBeStep(move) {
            throw PokeCalcError(code: PokeCalcError.Code.invalidInput, message: "ランクを変えない変化技は手にできない: \(move.id)")
        }
        if let opponentMove, !ComboMoveRules.canBeOpponentMove(opponentMove) {
            throw PokeCalcError(code: PokeCalcError.Code.invalidInput, message: "変化技は相手の技にできない: \(opponentMove.id)")
        }

        let canned = fixtures.combo
        let known = request.opponentSpeed == .fromDefender
        var selfRanks = request.attacker.ranks
        var opponentRanks = request.defender.ranks
        var steps: [ComboStepResult] = []
        var marks: [UnsupportedMark] = []
        for (index, move) in stepMoves.enumerated() {
            let cannedStep = canned.steps[min(index, canned.steps.count - 1)]
            let startSelf = selfRanks
            let startOpponent = opponentRanks
            Self.apply(move.effect, selfRanks: &selfRanks, opponentRanks: &opponentRanks)
            var damage: CalcResult?
            if move.category != .status {
                var result = try cannedResult(forKey: Self.singleResultKey, category: move.category)
                result.unsupported = try Self.moveMarks(try comboEntry(id: move.id), category: move.category)
                marks += result.unsupported
                damage = result
            }
            steps.append(ComboStepResult(
                moveId: move.id, selfStartRanks: startSelf, selfEndRanks: selfRanks, opponentStartRanks: startOpponent,
                opponentEndRanks: opponentRanks, damage: damage, selfSpeed: cannedStep.selfSpeed,
                opponentSpeed: known ? canned.knownOpponentSpeed : 0, selfPriority: move.priority,
                opponentPriority: opponentMove?.priority ?? 0,
                order: known ? (ComboTurnOrder(rawValue: cannedStep.knownOrder) ?? .dependsOnOpponentSpeed) : .dependsOnOpponentSpeed,
                line: ComboSpeedLine(
                    selfSpeed: cannedStep.line.selfSpeed,
                    selfFirst: ComboSpeedRange(min: cannedStep.line.selfFirst.min, max: cannedStep.line.selfFirst.max),
                    opponentFirst: ComboSpeedRange(min: cannedStep.line.opponentFirst.min, max: cannedStep.line.opponentFirst.max))))
        }

        let cannedKO = canned.total.ko
        let ko = ComboKOChance(
            chancePercent: cannedKO.chancePercent, guaranteed: cannedKO.guaranteed, displayChancePercent: cannedKO.displayChancePercent)
        let notKO = ComboKOChance(chancePercent: 0, guaranteed: false, displayChancePercent: 0)
        let total = ComboTotal(
            minDamage: canned.total.minDamage, maxDamage: canned.total.maxDamage, minPercent: canned.total.minPercent,
            maxPercent: canned.total.maxPercent, defenderHP: canned.total.defenderHP, ko: ko,
            koByStep: Array(repeating: notKO, count: steps.count - 1) + [ko])
        let speed = ComboSpeedSummary(
            firstSelfSpeed: steps[0].selfSpeed, lastSelfSpeed: steps[steps.count - 1].selfSpeed,
            opponentSpeed: known ? canned.knownOpponentSpeed : 0, opponentSpeedKnown: known)
        var seen = Set<UnsupportedMark>()
        let unsupported = marks.filter { seen.insert($0).inserted }.map(ComboMark.calc)
        return ComboResult(
            steps: steps, total: total, speed: speed,
            opponentAttack: try opponentMove.map { try comboOpponentAttack(move: $0, canned: canned.opponentAttack) },
            unsupported: unsupported)
    }

    // MARK: - 補助

    private static func comboTransportError() -> PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.transport, message: "モックの連続技は接続できない設定です")
    }

    private func comboEntry(id: String) throws -> MockFixtures.MoveEntry {
        guard let entry = fixtures.moves.first(where: { $0.id == id }) else { throw Self.notFoundError("技", id) }
        return entry
    }

    private func comboMove(id: String) throws -> Move {
        try Self.domainMove(try comboEntry(id: id))
    }

    /// 技の効果をランクの足し算で反映する(フィクスチャの値をそのまま足すだけ。-6..+6 に収める)。
    private static func apply(_ effect: MoveRankEffect?, selfRanks: inout RankBlock, opponentRanks: inout RankBlock) {
        guard let effect else { return }
        switch effect.target {
        case .user: selfRanks = adding(effect.stages, to: selfRanks)
        case .opponent: opponentRanks = adding(effect.stages, to: opponentRanks)
        }
    }

    private static func adding(_ delta: RankBlock, to ranks: RankBlock) -> RankBlock {
        func clamped(_ value: Int) -> Int { min(RankLimits.max, max(RankLimits.min, value)) }
        return RankBlock(
            atk: clamped(ranks.atk + delta.atk), def: clamped(ranks.def + delta.def), spa: clamped(ranks.spa + delta.spa),
            spd: clamped(ranks.spd + delta.spd), spe: clamped(ranks.spe + delta.spe))
    }

    /// 相手の技があるときだけ。`fast` は maxActions 回・`slow` は minActions 回(契約の説明どおり)。
    private func comboOpponentAttack(move: Move, canned: MockFixtures.ComboFixture.OpponentAttack) throws -> ComboOpponentAttack {
        let damage = try cannedResult(forKey: Self.singleResultKey, category: move.category)
        func scenario(count: Int, timing: ComboOpponentTiming, survive: Double) -> ComboOpponentScenario {
            ComboOpponentScenario(
                actions: (0..<count).map { ComboOpponentAction(step: $0, timing: timing, damage: damage, surviveChancePercent: survive) },
                surviveChancePercent: survive)
        }
        return ComboOpponentAttack(
            selfHp: canned.selfHp, minActions: canned.minActions, maxActions: canned.maxActions,
            fast: scenario(count: canned.maxActions, timing: .beforeSelf, survive: canned.fastSurviveChancePercent),
            slow: scenario(count: canned.minActions, timing: .afterSelf, survive: canned.slowSurviveChancePercent))
    }
}
