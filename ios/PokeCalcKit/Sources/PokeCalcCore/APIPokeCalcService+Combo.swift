import Foundation
import OpenAPIRuntime
import PokeCalcAPI

// APIPokeCalcService+Combo: 連続技(`POST /api/calc/combo`。ADR-0253・ADR-0535)と、公開 `Move.effect` の写像。
// `PokeCalcService` とは別のプロトコル(`ComboCalcService`)。生成型 ↔ ドメインの写像はこのファイルに閉じる。

extension APIPokeCalcService: ComboCalcService {
    public func calcCombo(_ request: ComboRequest) async throws -> ComboResult {
        let output = try await send {
            try await client.calcCombo(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedComboRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainComboResult(try ok.body.json)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .internalServerError(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - ドメイン → 要求(省略できる項目はキーを作らない)

    private static func generatedComboRequest(_ request: ComboRequest) -> Components.Schemas.ComboRequest {
        .init(
            format: generatedFormat(request.format),
            attacker: generatedIndividual(request.attacker),
            defender: generatedIndividual(request.defender),
            field: request.field == FieldState() ? nil : generatedFieldState(request.field),
            steps: request.steps.map { step in
                .init(moveId: step.moveId, critical: step.critical ? true : nil, hits: step.hits)
            },
            opponentMoveId: request.opponentMoveId,
            opponentSpeed: request.opponentSpeed == .unknown ? nil : .init(value1: .fromDefender),
            speedField: request.speedField.isAnyOn
                ? .init(
                    trickRoom: request.speedField.trickRoom, selfTailwind: request.speedField.selfTailwind,
                    opponentTailwind: request.speedField.opponentTailwind)
                : nil
        )
    }

    // MARK: - 応答 → ドメイン

    private static func domainComboResult(_ result: Components.Schemas.ComboResult) -> ComboResult {
        ComboResult(
            steps: result.steps.map(domainComboStep),
            total: domainComboTotal(result.total),
            speed: ComboSpeedSummary(
                firstSelfSpeed: result.speed.firstSelfSpeed, lastSelfSpeed: result.speed.lastSelfSpeed,
                opponentSpeed: result.speed.opponentSpeed, opponentSpeedKnown: result.speed.opponentSpeedKnown),
            opponentAttack: result.opponentAttack.map { domainComboOpponentAttack($0.value1) },
            unsupported: result.unsupported.map(domainComboMark))
    }

    private static func domainComboStep(_ step: Components.Schemas.ComboStepResult) -> ComboStepResult {
        ComboStepResult(
            moveId: step.moveId,
            selfStartRanks: domainComboRanks(step.selfStartRanks), selfEndRanks: domainComboRanks(step.selfEndRanks),
            opponentStartRanks: domainComboRanks(step.opponentStartRanks),
            opponentEndRanks: domainComboRanks(step.opponentEndRanks),
            damage: step.damage.map { domainCalcResult($0.value1) },
            selfSpeed: step.selfSpeed, opponentSpeed: step.opponentSpeed,
            selfPriority: step.selfPriority, opponentPriority: step.opponentPriority,
            order: domainComboTurnOrder(step.order),
            line: ComboSpeedLine(
                selfSpeed: step.line.selfSpeed,
                selfFirst: ComboSpeedRange(min: step.line.selfFirst.min, max: step.line.selfFirst.max),
                opponentFirst: ComboSpeedRange(min: step.line.opponentFirst.min, max: step.line.opponentFirst.max)))
    }

    private static func domainComboRanks(_ ranks: Components.Schemas.ComboRanks) -> RankBlock {
        RankBlock(atk: ranks.atk, def: ranks.def, spa: ranks.spa, spd: ranks.spd, spe: ranks.spe)
    }

    private static func domainComboTurnOrder(_ order: Components.Schemas.ComboTurnOrder) -> ComboTurnOrder {
        switch order {
        case .selfFirst: return .selfFirst
        case .opponentFirst: return .opponentFirst
        case .speedTie: return .speedTie
        case .dependsOnOpponentSpeed: return .dependsOnOpponentSpeed
        }
    }

    private static func domainComboKO(_ ko: Components.Schemas.ComboKOChance) -> ComboKOChance {
        ComboKOChance(
            chancePercent: ko.chancePercent, guaranteed: ko.guaranteed, displayChancePercent: ko.displayChancePercent)
    }

    private static func domainComboTotal(_ total: Components.Schemas.ComboTotal) -> ComboTotal {
        ComboTotal(
            minDamage: total.minDamage, maxDamage: total.maxDamage, minPercent: total.minPercent,
            maxPercent: total.maxPercent, defenderHP: total.defenderHP, ko: domainComboKO(total.ko),
            koByStep: total.koByStep.map(domainComboKO))
    }

    private static func domainComboOpponentAttack(_ attack: Components.Schemas.ComboOpponentAttack) -> ComboOpponentAttack {
        ComboOpponentAttack(
            selfHp: attack.selfHp, minActions: attack.minActions, maxActions: attack.maxActions,
            fast: domainComboScenario(attack.fast), slow: domainComboScenario(attack.slow))
    }

    private static func domainComboScenario(_ scenario: Components.Schemas.ComboOpponentScenario) -> ComboOpponentScenario {
        ComboOpponentScenario(
            actions: scenario.actions.map { action in
                ComboOpponentAction(
                    step: action.step,
                    timing: action.timing == .beforeSelf ? .beforeSelf : .afterSelf,
                    damage: domainCalcResult(action.damage.value1), surviveChancePercent: action.surviveChancePercent)
            },
            surviveChancePercent: scenario.surviveChancePercent)
    }

    /// 連続技だけの理由は `.combo`、それ以外は単発と同じ印(`.calc`)。
    private static func domainComboMark(_ mark: Components.Schemas.UnsupportedMark) -> ComboMark {
        guard let reason = ComboMarkReason(rawValue: mark.reason) else { return .calc(domainUnsupportedMark(mark)) }
        return .combo(target: UnsupportedTarget(contractValue: mark.target), reason: reason, id: mark.id)
    }

    // MARK: - 公開 Move.effect → MoveRankEffect

    private static let effectChanceKey = "chance"
    private static let effectTargetKey = "target"
    private static let effectStagesKey = "stages"
    private static let guaranteedChance = 100

    /// 契約 `Move.effect`(`MasterEffect` の自由形式の JSON。PascalCase の `Chance`・`Target`・`Stages`)から、
    /// 確率 100% でランクを変える効果を読む。キーの大小文字は区別しない。確率が 100 以外・対象が語彙外・
    /// stages のキーが atk/def/spa/spd/spe 以外なら nil(「ランクが変わらない」と同じ扱い。combo-calc.md §7 Q4 (b))。
    static func domainMoveRankEffect(_ raw: [String: (any Sendable)?]?) -> MoveRankEffect? {
        guard let raw else { return nil }
        let fields = Dictionary(raw.map { ($0.key.lowercased(), $0.value) }, uniquingKeysWith: { first, _ in first })
        guard let chance = integer(fields[effectChanceKey] ?? nil), chance == guaranteedChance,
            let targetText = (fields[effectTargetKey] ?? nil) as? String, let target = MoveEffectTarget(rawValue: targetText),
            let stagesRaw = (fields[effectStagesKey] ?? nil) as? [String: (any Sendable)?]
        else { return nil }
        var stages = RankBlock()
        for (key, value) in stagesRaw {
            guard let stage = integer(value) else { return nil }
            switch key.lowercased() {
            case "atk": stages.atk = stage
            case "def": stages.def = stage
            case "spa": stages.spa = stage
            case "spd": stages.spd = stage
            case "spe": stages.spe = stage
            default: return nil
            }
        }
        return MoveRankEffect(target: target, stages: stages)
    }

    private static func integer(_ value: (any Sendable)?) -> Int? {
        if let int = value as? Int { return int }
        if let double = value as? Double, double == double.rounded() { return Int(double) }
        return nil
    }
}
