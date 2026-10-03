import Foundation

// MockAdjustService: `AdjustService` のモック(AJ7。ADR-0502 §4。XCUITest と接続先なしの起動で使う)。
//
// `MockPokeCalcService` と同じ方針: 架空データ(`Resources/*.json`。名前は「テスト」で始める)を返すだけで、
// **指数・探索・配分を計算しない**(engine の式を Swift に写さない。絶対ルール2・ADR-0500 §4)。
// - 指数・最小 SP・配分は `Resources/adjust-results.json` の決め打ちの値を、要求の形(技の有無・発数・
//   目標の有無・回す側)に合わせて返す。
// - 技を覚えるポケモンは `species.json` の learnset を逆に引き、図鑑番号・フォルム番号の昇順で
//   `limit` / `offset` を効かせる(引くだけで計算ではない)。マスタに無い技は `not_found`。

public struct MockAdjustService: AdjustService {
    /// `adjust-results.json` の形(アプリ内だけの固定フィクスチャ。openapi の契約とは無関係)。
    private struct Canned: Decodable {
        struct Stats: Decodable {
            let hp: Int, atk: Int, def: Int, spa: Int, spd: Int, spe: Int
            var block: StatBlock { StatBlock(hp: hp, atk: atk, def: def, spa: spa, spd: spd, spe: spe) }
        }
        struct Point: Decodable {
            let hp: Int, sp: Int, spDelta: Int
            var point: HPLinePoint { HPLinePoint(hp: hp, sp: sp, spDelta: spDelta) }
        }
        struct Lines: Decodable {
            let hp: Int, sp: Int
            let current: String
            let next16n: Point?
            let prev16n: Point?
            let next16nMinus1: Point?
            let prev16nMinus1: Point?
        }
        struct Indices: Decodable {
            let stats: Stats
            let firepowerIndex: Int64
            let physicalBulkIndex: Int64
            let specialBulkIndex: Int64
            let hpLines: Lines
        }
        struct KO: Decodable {
            let searchLimit: Int
            let feasible: Bool
            let sp: Int
            let chancePercent: Double
        }
        struct Survive: Decodable {
            let searchLimit: Int
            let feasible: Bool
            let hpSp: Int, statSp: Int, totalSp: Int
            let bulkIndex: Int64
            let chancePercent: Double
        }
        struct Plan: Decodable {
            let sp: Stats
            let totalSp: Int
            let stats: Stats
            let physicalBulk: Int64
            let specialBulk: Int64
            let speedMet: Bool
            let goalMet: Bool
            let chancePercent: Double
            var plan: AdjustAllocPlan {
                AdjustAllocPlan(
                    sp: sp.block, totalSp: totalSp, stats: stats.block, physicalBulk: physicalBulk,
                    specialBulk: specialBulk, speedMet: speedMet, goalMet: goalMet, chancePercent: chancePercent)
            }
        }
        struct Allocation: Decodable {
            let remaining: Int
            let plan: Plan
        }
        struct OffenseAllocation: Decodable {
            let remaining: Int
            let physicalPlan: Plan
            let specialPlan: Plan
        }
        let indices: Indices
        let offenseAllocation: OffenseAllocation
        let ko: KO
        let survive: Survive
        let allocation: Allocation
    }

    private let fixtures: MockFixtures
    private let canned: Canned

    public init() throws {
        fixtures = try MockFixtures.load()
        guard let url = Bundle.module.url(forResource: "adjust-results", withExtension: "json") else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "adjust-results.json がバンドルに無い")
        }
        canned = try JSONDecoder().decode(Canned.self, from: Data(contentsOf: url))
    }

    // MARK: - 調整 API

    public func adjustIndices(_ request: AdjustIndicesRequest) async throws -> AdjustIndicesResult {
        try requireSpecies(request.individual.speciesKey)
        if let moveId = request.moveId { _ = try move(moveId) }
        let indices = canned.indices
        let lines = indices.hpLines
        guard let current = HPLineKind(rawValue: lines.current) else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureInvalid, message: "未知の HPLineKind: \(lines.current)")
        }
        return AdjustIndicesResult(
            stats: indices.stats.block,
            firepowerIndex: request.moveId == nil ? nil : indices.firepowerIndex,
            physicalBulkIndex: indices.physicalBulkIndex,
            specialBulkIndex: indices.specialBulkIndex,
            hpLines: HPLineReport(
                hp: lines.hp, sp: lines.sp, current: current,
                next16n: lines.next16n?.point, prev16n: lines.prev16n?.point,
                next16nMinus1: lines.next16nMinus1?.point, prev16nMinus1: lines.prev16nMinus1?.point)
        )
    }

    public func adjustMinSpToKo(_ request: AdjustSearchRequest) async throws -> AdjustKOResult {
        try requireSpecies(request.attacker.speciesKey)
        try requireSpecies(request.defender.speciesKey)
        let (entry, category) = try moveAndCategory(request.moveId)
        let ko = canned.ko
        return AdjustKOResult(
            stat: AttackerPreset.relevantStat(for: category), searchLimit: ko.searchLimit, feasible: ko.feasible,
            sp: ko.sp, chancePercent: ko.chancePercent,
            unsupported: try MockPokeCalcService.moveMarks(entry, category: category))
    }

    public func adjustMinSpToSurvive(_ request: AdjustSearchRequest) async throws -> AdjustSurviveResult {
        try requireSpecies(request.attacker.speciesKey)
        try requireSpecies(request.defender.speciesKey)
        let (entry, category) = try moveAndCategory(request.moveId)
        let survive = canned.survive
        return AdjustSurviveResult(
            stat: KnownDefenderPreset.relevantStat(for: category), searchLimit: survive.searchLimit,
            feasible: survive.feasible, hpSp: survive.hpSp, statSp: survive.statSp, totalSp: survive.totalSp,
            bulkIndex: survive.bulkIndex, chancePercent: survive.chancePercent,
            unsupported: try MockPokeCalcService.moveMarks(entry, category: category))
    }

    public func adjustAllocation(_ request: AdjustAllocationRequest) async throws -> AdjustAllocationResult {
        try requireSpecies(request.selfIndividual.speciesKey)
        var marks: [UnsupportedMark] = []
        if let goal = request.goal {
            try requireSpecies(goal.opponent.speciesKey)
            let (entry, category) = try moveAndCategory(goal.moveId)
            marks = try MockPokeCalcService.moveMarks(entry, category: category)
        }
        let (remaining, plan) = cannedPlan(for: request)
        return AdjustAllocationResult(
            remaining: remaining, maxIndex: plan, minSp: request.goal == nil ? nil : plan, unsupported: marks)
    }

    /// 耐久側は H・B・D、攻撃側は攻撃(物理は A・特殊は C)と S だけに SP を置いた決め打ちの案(計算ではない)。
    private func cannedPlan(for request: AdjustAllocationRequest) -> (remaining: Int, plan: AdjustAllocPlan) {
        guard request.mode == .offense else {
            return (canned.allocation.remaining, canned.allocation.plan.plan)
        }
        let offense = canned.offenseAllocation
        let plan = request.offenseCategory == .special ? offense.specialPlan : offense.physicalPlan
        return (offense.remaining, plan.plan)
    }

    // MARK: - 技を覚えるポケモン(逆引き)

    public func moveLearners(moveId: String, limit: Int, offset: Int) async throws -> [SpeciesSummary] {
        _ = try move(moveId)
        let matching = fixtures.species
            .filter { $0.learnset.contains(moveId) }
            .sorted { ($0.dexNo, $0.form) < ($1.dexNo, $1.form) }
        guard offset < matching.count else { return [] }
        return try matching[offset..<min(matching.count, offset + limit)].map(MockPokeCalcService.domainSpeciesSummary)
    }

    // MARK: - 引くだけ

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

    private func moveAndCategory(_ id: String) throws -> (MockFixtures.MoveEntry, MoveCategory) {
        let entry = try move(id)
        return (entry, try MockPokeCalcService.domainMoveCategory(entry.category))
    }
}
