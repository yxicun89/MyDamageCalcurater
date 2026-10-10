// AdjustGoalsDomainTypes: 調整の「目標」方式(`POST /api/calc/adjust/goals`。F-11・ADR-0331・ADR-0525)の境界と型。
//
// `AdjustService` には混ぜない(`FavoritesService` と同じく別プロトコル。既存の準拠型を変えずに足す)。
// 生成型の写しで、画面は生成型を直接使わない(ADR-0500 §3)。計算はしない(絶対ルール2)。省略可の項目は
// Optional で持ち、nil は「キーを送らない」(契約の既定をクライアントが補わない)。

/// 目標の種類(openapi `AdjustGoalKind`)。`CaseIterable` の順が画面の選択肢の並び(Web の `ADJUST_GOAL_KINDS`)。
public enum AdjustGoalKind: String, CaseIterable, Sendable, Hashable {
    /// 相手の素早さを上回る。
    case outspeed
    /// 相手の技を耐える。
    case survive
    /// 自分の技で倒す。
    case ko
}

/// 1つの目標(openapi `AdjustGoal`)。種類ごとに使う項目が違う。
public struct AdjustGoalInput: Equatable, Sendable {
    public var kind: AdjustGoalKind
    public var opponent: Individual
    /// outspeed = 先に使う技(任意)、survive = 相手の技、ko = 自分の技。
    public var moveId: String?
    /// survive・ko で送る。outspeed は nil。
    public var hits: Int?
    /// survive・ko で 100 以外のときだけ送る(nil は契約の既定 100)。
    public var thresholdPercent: Double?

    public init(
        kind: AdjustGoalKind, opponent: Individual, moveId: String? = nil, hits: Int? = nil, thresholdPercent: Double? = nil
    ) {
        self.kind = kind
        self.opponent = opponent
        self.moveId = moveId
        self.hits = hits
        self.thresholdPercent = thresholdPercent
    }
}

/// `POST /api/calc/adjust/goals` の要求(openapi `AdjustGoalsRequest`)。
public struct AdjustGoalsRequest: Equatable, Sendable {
    public var format: Format
    /// 自分の個体。`sp` は各能力の下限(固定する能力ポイント)。
    public var selfIndividual: Individual
    /// nil なら送らない(各能力 32)。
    public var ceiling: AdjustCeiling?
    /// 1...`RequestLimits.maxAdjustGoals` 件。順序に意味がある(応答の `goals` と同じ順)。
    public var goals: [AdjustGoalInput]

    public init(format: Format, selfIndividual: Individual, ceiling: AdjustCeiling? = nil, goals: [AdjustGoalInput]) {
        self.format = format
        self.selfIndividual = selfIndividual
        self.ceiling = ceiling
        self.goals = goals
    }
}

/// 目標をすべて満たす(満たせなければ最も近い)振り方(openapi `AdjustGoalsPlan`)。
public struct AdjustGoalsPlan: Equatable, Sendable {
    public var sp: StatBlock
    public var totalSp: Int
    /// 実数値(ランク補正なし)。
    public var stats: StatBlock

    public init(sp: StatBlock, totalSp: Int, stats: StatBlock) {
        self.sp = sp
        self.totalSp = totalSp
        self.stats = stats
    }
}

/// 目標1件の結果(openapi `AdjustGoalOutcome`。要求の `goals` と同じ順)。
public struct AdjustGoalOutcome: Equatable, Sendable {
    public var kind: AdjustGoalKind
    public var met: Bool
    /// survive・ko の確率(%。engine の生値)。outspeed は nil。
    public var chancePercent: Double?
    /// outspeed で比べた自分・相手の素早さ。それ以外は nil。
    public var selfSpeed: Int?
    public var opponentSpeed: Int?
    /// outspeed で自分に掛けたランク(-6...6)。それ以外は nil。
    public var selfSpeedRank: Int?

    public init(
        kind: AdjustGoalKind, met: Bool, chancePercent: Double? = nil, selfSpeed: Int? = nil,
        opponentSpeed: Int? = nil, selfSpeedRank: Int? = nil
    ) {
        self.kind = kind
        self.met = met
        self.chancePercent = chancePercent
        self.selfSpeed = selfSpeed
        self.opponentSpeed = opponentSpeed
        self.selfSpeedRank = selfSpeedRank
    }
}

/// `POST /api/calc/adjust/goals` の応答(openapi `AdjustGoalsResult`)。
public struct AdjustGoalsResult: Equatable, Sendable {
    /// すべての目標を満たすか。
    public var feasible: Bool
    /// 66 - plan.totalSp。
    public var remaining: Int
    public var plan: AdjustGoalsPlan
    public var goals: [AdjustGoalOutcome]
    public var unsupported: [UnsupportedMark]

    public init(feasible: Bool, remaining: Int, plan: AdjustGoalsPlan, goals: [AdjustGoalOutcome], unsupported: [UnsupportedMark] = []) {
        self.feasible = feasible
        self.remaining = remaining
        self.plan = plan
        self.goals = goals
        self.unsupported = unsupported
    }
}

/// 調整の「目標」API(openapi `adjustGoals`)。実装は `APIPokeCalcService`(拡張。HTTP)と `MockAdjustGoalsService`。
/// 失敗は `PokeCalcError`、取り消しは `CancellationError`(包まない)。
public protocol AdjustGoalsService: Sendable {
    func adjustGoals(_ request: AdjustGoalsRequest) async throws -> AdjustGoalsResult
}

/// サーバーが目標の操作を提供していない(`ADJUST_GOALS_ENABLED` が無効・ルートなし)ことを示すエラーか。
/// 契約の 200/400/500/503 以外(`default`)で返る 404 `not_found` が対象。通信失敗・503 は「一時的に使えない」であって
/// 「機能がない」ではないので含めない(次の送信で直るかもしれないため、目標方式は引っ込めない)。
public enum AdjustGoalsAvailability {
    public static let unavailableCodes: Set<String> = ["not_found"]

    public static func isUnavailable(_ error: any Error) -> Bool {
        guard let error = error as? PokeCalcError else { return false }
        return unavailableCodes.contains(error.code)
    }
}
