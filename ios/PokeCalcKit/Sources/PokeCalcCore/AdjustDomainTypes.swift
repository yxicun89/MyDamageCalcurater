// AdjustDomainTypes: 調整 API(`/api/calc/adjust/*`。ADR-0250)の要求・応答のドメインの型(AJ7。ADR-0502 §3)。
//
// 生成型(`Components.Schemas.Adjust*`)の写しで、画面(`AdjustViewModel`)は生成型を直接使わない(ADR-0500 §3)。
// 省略可の項目は Optional で持ち、nil は「キーを送らない」(契約の既定をクライアントが補わない。ADR-0250 §3)。
// 計算はしない(指数・探索・配分はすべて calc-svc の engine。絶対ルール2)。

/// HP 実数値が属するライン(openapi `HPLineKind`)。
public enum HPLineKind: String, CaseIterable, Sendable, Hashable {
    case none
    case line16n = "16n"
    case line16nMinus1 = "16n-1"
}

/// 1つの 16n / 16n-1 ライン(openapi `HPLinePoint`)。
public struct HPLinePoint: Equatable, Sendable {
    /// そのラインの HP 実数値。
    public var hp: Int
    /// その HP にする HP の SP。
    public var sp: Int
    /// 現在の SP からの差(次のラインは正、前のラインは負)。
    public var spDelta: Int

    public init(hp: Int, sp: Int, spDelta: Int) {
        self.hp = hp
        self.sp = sp
        self.spDelta = spDelta
    }
}

/// HP の 16n / 16n-1 ライン(openapi `HPLineReport`)。無いラインは nil。
public struct HPLineReport: Equatable, Sendable {
    public var hp: Int
    public var sp: Int
    public var current: HPLineKind
    public var next16n: HPLinePoint?
    public var prev16n: HPLinePoint?
    public var next16nMinus1: HPLinePoint?
    public var prev16nMinus1: HPLinePoint?

    public init(
        hp: Int, sp: Int, current: HPLineKind,
        next16n: HPLinePoint? = nil, prev16n: HPLinePoint? = nil,
        next16nMinus1: HPLinePoint? = nil, prev16nMinus1: HPLinePoint? = nil
    ) {
        self.hp = hp
        self.sp = sp
        self.current = current
        self.next16n = next16n
        self.prev16n = prev16n
        self.next16nMinus1 = next16nMinus1
        self.prev16nMinus1 = prev16nMinus1
    }
}

/// `POST /api/calc/adjust/indices` の要求(openapi `AdjustIndicesRequest`)。
public struct AdjustIndicesRequest: Equatable, Sendable {
    public var individual: Individual
    /// 火力指数に使う技。nil なら送らない(応答の `firepowerIndex` は nil)。
    public var moveId: String?
    /// 火力指数の補正(4096 基準)。nil なら送らない(契約の既定 4096)。
    public var modifier: Int?
    /// 耐久指数の被ダメージ補正(4096 基準)。nil なら送らない。
    public var damageModifier: Int?

    public init(individual: Individual, moveId: String? = nil, modifier: Int? = nil, damageModifier: Int? = nil) {
        self.individual = individual
        self.moveId = moveId
        self.modifier = modifier
        self.damageModifier = damageModifier
    }
}

/// `POST /api/calc/adjust/indices` の応答(openapi `AdjustIndicesResult`)。指数は int64。
public struct AdjustIndicesResult: Equatable, Sendable {
    /// 実数値(ランク補正なし)。
    public var stats: StatBlock
    /// 火力指数。技を送らなければ nil。
    public var firepowerIndex: Int64?
    public var physicalBulkIndex: Int64
    public var specialBulkIndex: Int64
    public var hpLines: HPLineReport

    public init(stats: StatBlock, firepowerIndex: Int64?, physicalBulkIndex: Int64, specialBulkIndex: Int64, hpLines: HPLineReport) {
        self.stats = stats
        self.firepowerIndex = firepowerIndex
        self.physicalBulkIndex = physicalBulkIndex
        self.specialBulkIndex = specialBulkIndex
        self.hpLines = hpLines
    }
}

/// 最小 SP の探索の要求(openapi `AdjustSearchRequest`。min-sp-to-ko / min-sp-to-survive で共有)。
/// どちらが自分かは操作で決まる(ko は attacker、survive は defender。ADR-0250 §2)。
/// 場・急所は AJ6 と同じく入力させないので持たない(ADR-0319 §2・ADR-0502 §3)。
public struct AdjustSearchRequest: Equatable, Sendable {
    public var format: Format
    public var attacker: Individual
    public var defender: Individual
    public var moveId: String
    /// 目標の発数(1...`RequestLimits.maxAdjustHits`)。
    public var hits: Int
    /// 満たすべき確率(%)。nil なら送らない(契約の既定 100 = 確定)。
    public var thresholdPercent: Double?

    public init(format: Format, attacker: Individual, defender: Individual, moveId: String, hits: Int, thresholdPercent: Double? = nil) {
        self.format = format
        self.attacker = attacker
        self.defender = defender
        self.moveId = moveId
        self.hits = hits
        self.thresholdPercent = thresholdPercent
    }
}

/// `POST /api/calc/adjust/min-sp-to-ko` の応答(openapi `AdjustKOResult`)。
public struct AdjustKOResult: Equatable, Sendable {
    /// 探索した能力(物理 → atk、特殊 → spa)。
    public var stat: StatKey
    public var searchLimit: Int
    public var feasible: Bool
    public var sp: Int
    /// engine の生値(%、丸めない。表示は `AdjustText.chancePercentText` で 0.1% 単位に切り捨てる)。
    public var chancePercent: Double
    public var unsupported: [UnsupportedMark]

    public init(stat: StatKey, searchLimit: Int, feasible: Bool, sp: Int, chancePercent: Double, unsupported: [UnsupportedMark] = []) {
        self.stat = stat
        self.searchLimit = searchLimit
        self.feasible = feasible
        self.sp = sp
        self.chancePercent = chancePercent
        self.unsupported = unsupported
    }
}

/// `POST /api/calc/adjust/min-sp-to-survive` の応答(openapi `AdjustSurviveResult`)。
public struct AdjustSurviveResult: Equatable, Sendable {
    /// H と組にして探索した能力(物理 → def、特殊 → spd)。
    public var stat: StatKey
    public var searchLimit: Int
    public var feasible: Bool
    public var hpSp: Int
    public var statSp: Int
    public var totalSp: Int
    public var bulkIndex: Int64
    public var chancePercent: Double
    public var unsupported: [UnsupportedMark]

    public init(
        stat: StatKey, searchLimit: Int, feasible: Bool, hpSp: Int, statSp: Int, totalSp: Int,
        bulkIndex: Int64, chancePercent: Double, unsupported: [UnsupportedMark] = []
    ) {
        self.stat = stat
        self.searchLimit = searchLimit
        self.feasible = feasible
        self.hpSp = hpSp
        self.statSp = statSp
        self.totalSp = totalSp
        self.bulkIndex = bulkIndex
        self.chancePercent = chancePercent
        self.unsupported = unsupported
    }
}

/// 残り SP を回す側(openapi `AllocMode`)。
public enum AllocMode: String, CaseIterable, Sendable, Hashable {
    case bulk
    case offense
}

/// 耐久側の指数最大の基準(openapi `BulkFocus`)。
public enum BulkFocus: String, CaseIterable, Sendable, Hashable {
    case physical
    case special
    case both
}

/// 回す能力の SP の上限(openapi `AdjustCeiling`)。nil の能力は送らない(契約の既定 32)。
public struct AdjustCeiling: Equatable, Sendable {
    public var hp: Int?
    public var atk: Int?
    public var def: Int?
    public var spa: Int?
    public var spd: Int?
    public var spe: Int?

    public init(hp: Int? = nil, atk: Int? = nil, def: Int? = nil, spa: Int? = nil, spd: Int? = nil, spe: Int? = nil) {
        self.hp = hp
        self.atk = atk
        self.def = def
        self.spa = spa
        self.spd = spd
        self.spe = spe
    }
}

/// 配分の目標(openapi `AdjustAllocGoal`)。bulk は「opponent の技を hits 発耐える」、offense は
/// 「自分の技で opponent を hits 発で倒す」。
public struct AdjustAllocGoal: Equatable, Sendable {
    public var format: Format
    public var opponent: Individual
    public var moveId: String
    public var hits: Int
    /// nil なら送らない(既定 100)。
    public var thresholdPercent: Double?

    public init(format: Format, opponent: Individual, moveId: String, hits: Int, thresholdPercent: Double? = nil) {
        self.format = format
        self.opponent = opponent
        self.moveId = moveId
        self.hits = hits
        self.thresholdPercent = thresholdPercent
    }
}

/// `POST /api/calc/adjust/allocation` の要求(openapi `AdjustAllocationRequest`)。
/// 契約のキー `self` は Swift では紛らわしいので `selfIndividual` と呼ぶ(写像で `self` に戻す)。
public struct AdjustAllocationRequest: Equatable, Sendable {
    /// 自分の個体。`sp` は各能力の下限(固定する能力ポイント)。
    public var selfIndividual: Individual
    public var ceiling: AdjustCeiling
    public var mode: AllocMode
    /// bulk で必須。offense では nil(送らない)。
    public var focus: BulkFocus?
    /// offense で必須(physical / special)。bulk では nil(送らない)。
    public var offenseCategory: MoveCategory?
    /// 素早さの目標(実数値)。生成型では任意だが Web と同じく常に送る(0 = 目標なし。ADR-0319 §5)。
    public var minSpeed: Int
    /// 「目標を指定する」のときだけ。nil なら送らない(応答の `minSp` は nil)。
    public var goal: AdjustAllocGoal?

    public init(
        selfIndividual: Individual, ceiling: AdjustCeiling, mode: AllocMode, focus: BulkFocus? = nil,
        offenseCategory: MoveCategory? = nil, minSpeed: Int = 0, goal: AdjustAllocGoal? = nil
    ) {
        self.selfIndividual = selfIndividual
        self.ceiling = ceiling
        self.mode = mode
        self.focus = focus
        self.offenseCategory = offenseCategory
        self.minSpeed = minSpeed
        self.goal = goal
    }
}

/// 配分の1案(openapi `AdjustAllocPlan`)。
public struct AdjustAllocPlan: Equatable, Sendable {
    public var sp: StatBlock
    public var totalSp: Int
    public var stats: StatBlock
    public var physicalBulk: Int64
    public var specialBulk: Int64
    public var speedMet: Bool
    public var goalMet: Bool
    public var chancePercent: Double

    public init(
        sp: StatBlock, totalSp: Int, stats: StatBlock, physicalBulk: Int64, specialBulk: Int64,
        speedMet: Bool, goalMet: Bool, chancePercent: Double
    ) {
        self.sp = sp
        self.totalSp = totalSp
        self.stats = stats
        self.physicalBulk = physicalBulk
        self.specialBulk = specialBulk
        self.speedMet = speedMet
        self.goalMet = goalMet
        self.chancePercent = chancePercent
    }
}

/// `POST /api/calc/adjust/allocation` の応答(openapi `AdjustAllocationResult`)。
public struct AdjustAllocationResult: Equatable, Sendable {
    public var remaining: Int
    public var maxIndex: AdjustAllocPlan
    /// goal を送らなければ nil。
    public var minSp: AdjustAllocPlan?
    public var unsupported: [UnsupportedMark]

    public init(remaining: Int, maxIndex: AdjustAllocPlan, minSp: AdjustAllocPlan?, unsupported: [UnsupportedMark] = []) {
        self.remaining = remaining
        self.maxIndex = maxIndex
        self.minSp = minSp
        self.unsupported = unsupported
    }
}
