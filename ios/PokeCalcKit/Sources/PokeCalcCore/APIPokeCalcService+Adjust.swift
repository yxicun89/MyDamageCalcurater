import PokeCalcAPI

// 調整 API(AJ7。ADR-0502 §3)の `APIPokeCalcService` 実装。
//
// `DeviceDataService` と同じく `PokeCalcService` とは別のプロトコルに準拠させる。写像は生成型 ↔ ドメインの
// 1か所の原則(ADR-0500 §3)に従い、この拡張の中に閉じる。ドメインの nil は「キーを送らない」(契約の既定を
// クライアントが補わない。ADR-0250 §3)。`minSpeed` だけは生成型では任意だが、Web と同じく常に送る。
// 通信・デコード・取り消しの扱いは `APIPokeCalcService.send` と同じ。

extension APIPokeCalcService: AdjustService {
    public func adjustIndices(_ request: AdjustIndicesRequest) async throws -> AdjustIndicesResult {
        let output = try await send {
            try await client.adjustIndices(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedIndicesRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainIndicesResult(try ok.body.json)
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

    public func adjustMinSpToKo(_ request: AdjustSearchRequest) async throws -> AdjustKOResult {
        let output = try await send {
            try await client.adjustMinSpToKo(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedSearchRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainKOResult(try ok.body.json)
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

    public func adjustMinSpToSurvive(_ request: AdjustSearchRequest) async throws -> AdjustSurviveResult {
        let output = try await send {
            try await client.adjustMinSpToSurvive(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedSearchRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainSurviveResult(try ok.body.json)
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

    public func adjustAllocation(_ request: AdjustAllocationRequest) async throws -> AdjustAllocationResult {
        let output = try await send {
            try await client.adjustAllocation(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedAllocationRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return Self.domainAllocationResult(try ok.body.json)
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

    public func moveLearners(moveId: String, limit: Int, offset: Int) async throws -> [SpeciesSummary] {
        let output = try await send {
            try await client.listMoveLearners(.init(
                path: .init(key: moveId),
                query: .init(limit: limit, offset: offset),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map(Self.domainSpeciesSummary)
        case .notFound(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - ドメイン → 要求

    private static func generatedIndicesRequest(_ request: AdjustIndicesRequest) -> Components.Schemas.AdjustIndicesRequest {
        .init(
            individual: .init(value1: generatedIndividual(request.individual)),
            moveId: request.moveId,
            modifier: request.modifier.map { .init(value1: $0) },
            damageModifier: request.damageModifier.map { .init(value1: $0) }
        )
    }

    private static func generatedSearchRequest(_ request: AdjustSearchRequest) -> Components.Schemas.AdjustSearchRequest {
        .init(
            format: generatedFormat(request.format),
            attacker: generatedIndividual(request.attacker),
            defender: generatedIndividual(request.defender),
            moveId: request.moveId,
            hits: request.hits,
            thresholdPercent: request.thresholdPercent
        )
    }

    private static func generatedAllocationRequest(_ request: AdjustAllocationRequest) -> Components.Schemas.AdjustAllocationRequest {
        .init(
            _self: .init(value1: generatedIndividual(request.selfIndividual)),
            ceiling: generatedCeiling(request.ceiling),
            mode: generatedAllocMode(request.mode),
            focus: request.focus.map { .init(value1: generatedBulkFocus($0)) },
            offenseCategory: request.offenseCategory.map { .init(value1: generatedMoveCategory($0)) },
            minSpeed: request.minSpeed,
            goal: request.goal.map(generatedGoal)
        )
    }

    private static func generatedGoal(_ goal: AdjustAllocGoal) -> Components.Schemas.AdjustAllocGoal {
        .init(
            format: generatedFormat(goal.format),
            opponent: generatedIndividual(goal.opponent),
            moveId: goal.moveId,
            hits: goal.hits,
            thresholdPercent: goal.thresholdPercent
        )
    }

    private static func generatedCeiling(_ ceiling: AdjustCeiling) -> Components.Schemas.AdjustCeiling {
        .init(hp: ceiling.hp, atk: ceiling.atk, def: ceiling.def, spa: ceiling.spa, spd: ceiling.spd, spe: ceiling.spe)
    }

    private static func generatedAllocMode(_ mode: AllocMode) -> Components.Schemas.AllocMode {
        switch mode {
        case .bulk: return .bulk
        case .offense: return .offense
        }
    }

    private static func generatedBulkFocus(_ focus: BulkFocus) -> Components.Schemas.BulkFocus {
        switch focus {
        case .physical: return .physical
        case .special: return .special
        case .both: return .both
        }
    }

    private static func generatedMoveCategory(_ category: MoveCategory) -> Components.Schemas.MoveCategory {
        switch category {
        case .physical: return .physical
        case .special: return .special
        case .status: return .status
        }
    }

    // MARK: - 応答 → ドメイン

    private static func domainIndicesResult(_ result: Components.Schemas.AdjustIndicesResult) -> AdjustIndicesResult {
        AdjustIndicesResult(
            stats: domainStatBlock(result.stats.value1),
            firepowerIndex: result.firepowerIndex,
            physicalBulkIndex: result.physicalBulkIndex,
            specialBulkIndex: result.specialBulkIndex,
            hpLines: domainHPLines(result.hpLines)
        )
    }

    private static func domainHPLines(_ report: Components.Schemas.HPLineReport) -> HPLineReport {
        HPLineReport(
            hp: report.hp, sp: report.sp, current: domainHPLineKind(report.current),
            next16n: report.next16n.map { domainHPLinePoint($0.value1) },
            prev16n: report.prev16n.map { domainHPLinePoint($0.value1) },
            next16nMinus1: report.next16nMinus1.map { domainHPLinePoint($0.value1) },
            prev16nMinus1: report.prev16nMinus1.map { domainHPLinePoint($0.value1) }
        )
    }

    private static func domainHPLinePoint(_ point: Components.Schemas.HPLinePoint) -> HPLinePoint {
        HPLinePoint(hp: point.hp, sp: point.sp, spDelta: point.spDelta)
    }

    private static func domainHPLineKind(_ kind: Components.Schemas.HPLineKind) -> HPLineKind {
        switch kind {
        case .none: return .none
        case ._16n: return .line16n
        case ._16n1: return .line16nMinus1
        }
    }

    private static func domainKOResult(_ result: Components.Schemas.AdjustKOResult) -> AdjustKOResult {
        AdjustKOResult(
            stat: domainStatKey(result.stat.value1), searchLimit: result.searchLimit, feasible: result.feasible,
            sp: result.sp, chancePercent: result.chancePercent,
            unsupported: result.unsupported.map(domainUnsupportedMark)
        )
    }

    private static func domainSurviveResult(_ result: Components.Schemas.AdjustSurviveResult) -> AdjustSurviveResult {
        AdjustSurviveResult(
            stat: domainStatKey(result.stat.value1), searchLimit: result.searchLimit, feasible: result.feasible,
            hpSp: result.hpSp, statSp: result.statSp, totalSp: result.totalSp, bulkIndex: result.bulkIndex,
            chancePercent: result.chancePercent, unsupported: result.unsupported.map(domainUnsupportedMark)
        )
    }

    private static func domainAllocationResult(_ result: Components.Schemas.AdjustAllocationResult) -> AdjustAllocationResult {
        AdjustAllocationResult(
            remaining: result.remaining, maxIndex: domainPlan(result.maxIndex),
            minSp: result.minSp.map { domainPlan($0.value1) },
            unsupported: result.unsupported.map(domainUnsupportedMark)
        )
    }

    private static func domainPlan(_ plan: Components.Schemas.AdjustAllocPlan) -> AdjustAllocPlan {
        AdjustAllocPlan(
            sp: domainStatBlock(plan.sp.value1), totalSp: plan.totalSp, stats: domainStatBlock(plan.stats.value1),
            physicalBulk: plan.physicalBulk, specialBulk: plan.specialBulk, speedMet: plan.speedMet,
            goalMet: plan.goalMet, chancePercent: plan.chancePercent
        )
    }
}
