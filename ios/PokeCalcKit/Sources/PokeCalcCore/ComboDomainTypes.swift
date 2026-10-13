// ComboDomainTypes: 連続技(CB-8。ADR-0535・ADR-0501「連続技」)のドメインの型と、画面が依存する境界。
//
// 契約は api/openapi.yaml の `POST /api/calc/combo`(`ComboRequest` / `ComboResult`)。生成型には依存せず、
// 生成型 ↔ ドメインの写像は `APIPokeCalcService+Combo.swift` の1か所に閉じる(ADR-0500 §3)。
// ここの型は値を運ぶだけで、計算は API が行う(クライアントで計算し直さない)。

// MARK: - 要求

/// 連続技の1手(契約 `ComboStepInput`)。
public struct ComboStepInput: Equatable, Sendable {
    public var moveId: String
    /// 急所。送るのは true のときだけ。
    public var critical: Bool
    /// 多段技の回数(指定したときだけ送る)。
    public var hits: Int?

    public init(moveId: String, critical: Bool = false, hits: Int? = nil) {
        self.moveId = moveId
        self.critical = critical
        self.hits = hits
    }
}

/// 相手の素早さの扱い(契約 `ComboOpponentSpeed`)。
public enum ComboOpponentSpeed: String, Equatable, Sendable {
    /// 分からない(既定。送らない)。
    case unknown
    /// 防御側の個体(振り)から求める。
    case fromDefender = "from_defender"
}

/// 行動順に効く場(契約 `ComboSpeedField`)。全手で同じ。
public struct ComboSpeedField: Equatable, Sendable {
    public var trickRoom: Bool
    public var selfTailwind: Bool
    public var opponentTailwind: Bool

    public init(trickRoom: Bool = false, selfTailwind: Bool = false, opponentTailwind: Bool = false) {
        self.trickRoom = trickRoom
        self.selfTailwind = selfTailwind
        self.opponentTailwind = opponentTailwind
    }

    /// 1つでも true か(全部 false のときは要求に載せない)。
    public var isAnyOn: Bool { trickRoom || selfTailwind || opponentTailwind }
}

/// 連続技の要求(契約 `ComboRequest`)。
public struct ComboRequest: Equatable, Sendable {
    public var format: Format
    /// 1 手目の開始時のランクを持つ攻撃側。
    public var attacker: Individual
    public var defender: Individual
    public var field: FieldState
    public var steps: [ComboStepInput]
    public var opponentMoveId: String?
    public var opponentSpeed: ComboOpponentSpeed
    public var speedField: ComboSpeedField

    public init(
        format: Format, attacker: Individual, defender: Individual, field: FieldState, steps: [ComboStepInput],
        opponentMoveId: String? = nil, opponentSpeed: ComboOpponentSpeed = .unknown,
        speedField: ComboSpeedField = ComboSpeedField()
    ) {
        self.format = format
        self.attacker = attacker
        self.defender = defender
        self.field = field
        self.steps = steps
        self.opponentMoveId = opponentMoveId
        self.opponentSpeed = opponentSpeed
        self.speedField = speedField
    }
}

// MARK: - 応答

/// 行動順(契約 `ComboTurnOrder`)。
public enum ComboTurnOrder: String, Equatable, Sendable {
    case selfFirst = "self_first"
    case opponentFirst = "opponent_first"
    case speedTie = "speed_tie"
    /// 相手の素早さが分からない。
    case dependsOnOpponentSpeed = "depends_on_opponent_speed"
}

/// 素早さの範囲(両端を含む。契約 `ComboSpeedRange`)。`min > max` は空。
public struct ComboSpeedRange: Equatable, Sendable {
    public var min: Int
    public var max: Int

    public init(min: Int, max: Int) {
        self.min = min
        self.max = max
    }

    public var isEmpty: Bool { min > max }
}

/// 相手の素早さがどの範囲なら自分が先か(契約 `ComboSpeedLine`)。トリックルーム中は向きが逆。
public struct ComboSpeedLine: Equatable, Sendable {
    public var selfSpeed: Int
    public var selfFirst: ComboSpeedRange
    public var opponentFirst: ComboSpeedRange

    public init(selfSpeed: Int, selfFirst: ComboSpeedRange, opponentFirst: ComboSpeedRange) {
        self.selfSpeed = selfSpeed
        self.selfFirst = selfFirst
        self.opponentFirst = opponentFirst
    }
}

/// 1手の結果(契約 `ComboStepResult`)。
public struct ComboStepResult: Equatable, Sendable {
    public var moveId: String
    public var selfStartRanks: RankBlock
    public var selfEndRanks: RankBlock
    public var opponentStartRanks: RankBlock
    public var opponentEndRanks: RankBlock
    /// 変化技の手は nil。
    public var damage: CalcResult?
    public var selfSpeed: Int
    public var opponentSpeed: Int
    public var selfPriority: Int
    public var opponentPriority: Int
    public var order: ComboTurnOrder
    public var line: ComboSpeedLine

    public init(
        moveId: String, selfStartRanks: RankBlock, selfEndRanks: RankBlock, opponentStartRanks: RankBlock,
        opponentEndRanks: RankBlock, damage: CalcResult?, selfSpeed: Int, opponentSpeed: Int, selfPriority: Int,
        opponentPriority: Int, order: ComboTurnOrder, line: ComboSpeedLine
    ) {
        self.moveId = moveId
        self.selfStartRanks = selfStartRanks
        self.selfEndRanks = selfEndRanks
        self.opponentStartRanks = opponentStartRanks
        self.opponentEndRanks = opponentEndRanks
        self.damage = damage
        self.selfSpeed = selfSpeed
        self.opponentSpeed = opponentSpeed
        self.selfPriority = selfPriority
        self.opponentPriority = opponentPriority
        self.order = order
        self.line = line
    }
}

/// 倒せる確率(契約 `ComboKOChance`)。画面に出すのは `displayChancePercent`。
public struct ComboKOChance: Equatable, Sendable {
    public var chancePercent: Double
    public var guaranteed: Bool
    public var displayChancePercent: Double

    public init(chancePercent: Double, guaranteed: Bool, displayChancePercent: Double) {
        self.chancePercent = chancePercent
        self.guaranteed = guaranteed
        self.displayChancePercent = displayChancePercent
    }
}

/// 全手の合計(契約 `ComboTotal`)。
public struct ComboTotal: Equatable, Sendable {
    public var minDamage: Int
    public var maxDamage: Int
    public var minPercent: Double
    public var maxPercent: Double
    public var defenderHP: Int
    public var ko: ComboKOChance
    /// i 手目までで倒せる確率(累積。長さ = 手の数)。
    public var koByStep: [ComboKOChance]

    public init(
        minDamage: Int, maxDamage: Int, minPercent: Double, maxPercent: Double, defenderHP: Int, ko: ComboKOChance,
        koByStep: [ComboKOChance]
    ) {
        self.minDamage = minDamage
        self.maxDamage = maxDamage
        self.minPercent = minPercent
        self.maxPercent = maxPercent
        self.defenderHP = defenderHP
        self.ko = ko
        self.koByStep = koByStep
    }
}

/// 素早さの要約(契約 `ComboSpeed`)。
public struct ComboSpeedSummary: Equatable, Sendable {
    public var firstSelfSpeed: Int
    public var lastSelfSpeed: Int
    /// `opponentSpeedKnown` が false なら 0。
    public var opponentSpeed: Int
    public var opponentSpeedKnown: Bool

    public init(firstSelfSpeed: Int, lastSelfSpeed: Int, opponentSpeed: Int, opponentSpeedKnown: Bool) {
        self.firstSelfSpeed = firstSelfSpeed
        self.lastSelfSpeed = lastSelfSpeed
        self.opponentSpeed = opponentSpeed
        self.opponentSpeedKnown = opponentSpeedKnown
    }
}

/// 相手の行動が自分の手の前か後か(契約 `ComboOpponentTiming`)。
public enum ComboOpponentTiming: String, Equatable, Sendable {
    case beforeSelf = "before_self"
    case afterSelf = "after_self"
}

/// 相手の1回の行動(契約 `ComboOpponentAction`)。
public struct ComboOpponentAction: Equatable, Sendable {
    public var step: Int
    public var timing: ComboOpponentTiming
    public var damage: CalcResult
    /// ここまでの全行動を耐える確率(%。engine の生値)。
    public var surviveChancePercent: Double

    public init(step: Int, timing: ComboOpponentTiming, damage: CalcResult, surviveChancePercent: Double) {
        self.step = step
        self.timing = timing
        self.damage = damage
        self.surviveChancePercent = surviveChancePercent
    }
}

/// 相手の行動の筋書き(契約 `ComboOpponentScenario`)。
public struct ComboOpponentScenario: Equatable, Sendable {
    public var actions: [ComboOpponentAction]
    public var surviveChancePercent: Double

    public init(actions: [ComboOpponentAction], surviveChancePercent: Double) {
        self.actions = actions
        self.surviveChancePercent = surviveChancePercent
    }
}

/// 連続技を撃ち終わるまでに受ける相手の攻撃(契約 `ComboOpponentAttack`)。`fast` を主に出す。
public struct ComboOpponentAttack: Equatable, Sendable {
    public var selfHp: Int
    public var minActions: Int
    public var maxActions: Int
    public var fast: ComboOpponentScenario
    public var slow: ComboOpponentScenario

    public init(selfHp: Int, minActions: Int, maxActions: Int, fast: ComboOpponentScenario, slow: ComboOpponentScenario) {
        self.selfHp = selfHp
        self.minActions = minActions
        self.maxActions = maxActions
        self.fast = fast
        self.slow = slow
    }
}

/// 連続技だけが付ける印の理由(契約 `UnsupportedMark.reason` の `choice_locked` など)。
public enum ComboMarkReason: String, CaseIterable, Equatable, Sendable {
    case choiceLocked = "choice_locked"
    case defenderFullHPAssumed = "defender_full_hp_assumed"
    case speedUndetermined = "speed_undetermined"
}

/// 連続技の応答の印。連続技だけの理由は `.combo`、それ以外は単発と同じ `.calc`。
public enum ComboMark: Equatable, Sendable {
    case calc(UnsupportedMark)
    case combo(target: UnsupportedTarget, reason: ComboMarkReason, id: String)
}

/// 連続技の結果(契約 `ComboResult`)。
public struct ComboResult: Equatable, Sendable {
    public var steps: [ComboStepResult]
    public var total: ComboTotal
    public var speed: ComboSpeedSummary
    /// 相手の技を選んだときだけ。
    public var opponentAttack: ComboOpponentAttack?
    public var unsupported: [ComboMark]

    public init(
        steps: [ComboStepResult], total: ComboTotal, speed: ComboSpeedSummary, opponentAttack: ComboOpponentAttack?,
        unsupported: [ComboMark]
    ) {
        self.steps = steps
        self.total = total
        self.speed = speed
        self.opponentAttack = opponentAttack
        self.unsupported = unsupported
    }
}

// MARK: - 境界

/// 連続技の計算(`POST /api/calc/combo`)。`PokeCalcService` とは別のプロトコル
/// (連続技の失敗が単発の計算に影響しないよう混ぜない。絶対ルール5)。失敗は `PokeCalcError`。
public protocol ComboCalcService: Sendable {
    func calcCombo(_ request: ComboRequest) async throws -> ComboResult
}
