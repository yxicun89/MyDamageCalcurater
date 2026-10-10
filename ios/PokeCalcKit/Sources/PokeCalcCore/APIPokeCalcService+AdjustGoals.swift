import PokeCalcAPI

// 調整の「目標」API(F-11。ADR-0525)の `APIPokeCalcService` 実装。
//
// `AdjustService` とは別のプロトコルに準拠させる(既存の準拠型を変えない)。写像は生成型 ↔ ドメインの1か所の
// 原則(ADR-0500 §3)に従い、この拡張の中に閉じる。ドメインの nil は「キーを送らない」(契約の既定をクライアントが補わない)。
// 通信・デコード・取り消しの扱いは `APIPokeCalcService.send` と同じ。契約に無い 404/501 は `default` として
// `domainError` に流れ、code(`not_found` 等)がそのまま運ばれる(画面は `AdjustGoalsAvailability` で機能なしと判断する)。

extension APIPokeCalcService: AdjustGoalsService {
    public func adjustGoals(_ request: AdjustGoalsRequest) async throws -> AdjustGoalsResult {
        let output = try await send {
            try await client.adjustGoals(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedGoalsRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainGoalsResult(try ok.body.json)
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

    // MARK: - ドメイン → 要求

    private static func generatedGoalsRequest(_ request: AdjustGoalsRequest) -> Components.Schemas.AdjustGoalsRequest {
        .init(
            format: generatedFormat(request.format),
            _self: .init(value1: generatedIndividual(request.selfIndividual)),
            ceiling: request.ceiling.map(generatedCeiling),
            goals: request.goals.map(generatedGoalInput)
        )
    }

    private static func generatedGoalInput(_ goal: AdjustGoalInput) -> Components.Schemas.AdjustGoal {
        .init(
            kind: generatedGoalKind(goal.kind),
            opponent: .init(value1: generatedIndividual(goal.opponent)),
            moveId: goal.moveId,
            hits: goal.hits,
            thresholdPercent: goal.thresholdPercent
        )
    }

    private static func generatedGoalKind(_ kind: AdjustGoalKind) -> Components.Schemas.AdjustGoalKind {
        switch kind {
        case .outspeed: return .outspeed
        case .survive: return .survive
        case .ko: return .ko
        }
    }

    // MARK: - 応答 → ドメイン

    private static func domainGoalsResult(_ result: Components.Schemas.AdjustGoalsResult) -> AdjustGoalsResult {
        AdjustGoalsResult(
            feasible: result.feasible, remaining: result.remaining,
            plan: AdjustGoalsPlan(
                sp: domainStatBlock(result.plan.sp.value1), totalSp: result.plan.totalSp,
                stats: domainStatBlock(result.plan.stats.value1)),
            goals: result.goals.map(domainGoalOutcome),
            unsupported: result.unsupported.map(domainUnsupportedMark)
        )
    }

    private static func domainGoalOutcome(_ outcome: Components.Schemas.AdjustGoalOutcome) -> AdjustGoalOutcome {
        AdjustGoalOutcome(
            kind: domainGoalKind(outcome.kind), met: outcome.met, chancePercent: outcome.chancePercent,
            selfSpeed: outcome.selfSpeed, opponentSpeed: outcome.opponentSpeed, selfSpeedRank: outcome.selfSpeedRank)
    }

    private static func domainGoalKind(_ kind: Components.Schemas.AdjustGoalKind) -> AdjustGoalKind {
        switch kind {
        case .outspeed: return .outspeed
        case .survive: return .survive
        case .ko: return .ko
        }
    }
}
