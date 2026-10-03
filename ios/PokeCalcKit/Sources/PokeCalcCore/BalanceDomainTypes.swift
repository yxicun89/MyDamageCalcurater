import Foundation

// BalanceDomainTypes: タイプバランス(balance-svc)のドメインの型(ADR-0415)。
//
// 倍率は balance の応答の文字列(分数。`DefenseMultiplier`)をそのまま運び、iOS で計算し直さない
// (ADR-0415 §4)。生成型(`PokeCalcBalanceAPI`)は `APIBalanceService` だけが知る。

/// balance へ送るメンバー1体(`analyze` は `pokemonId`・`abilityId`、`coverage` は `pokemonId`・`moveIds` だけを使う)。
public struct BalanceMemberInput: Equatable, Sendable {
    public var pokemonId: String
    public var abilityId: String?
    public var moveIds: [String]

    public init(pokemonId: String, abilityId: String? = nil, moveIds: [String] = []) {
        self.pokemonId = pokemonId
        self.abilityId = abilityId
        self.moveIds = moveIds
    }
}

// MARK: - 防御相性(analyze)

/// openapi `DefenseCategory`。語(弱点/耐性/無効)はこの値から選び、倍率の値から判定し直さない。
public enum BalanceDefenseCategory: String, CaseIterable, Sendable, Hashable {
    case quadWeak = "quad_weak"
    case weak
    case neutral
    case resist
    case quadResist = "quad_resist"
    case immune
}

/// openapi `EffectSource`。倍率の出どころ(タイプ相性だけ / 特性が変えた)。
public enum BalanceEffectSource: String, CaseIterable, Sendable, Hashable {
    case type
    case ability
}

/// openapi `DefenseEffect`。特性の効果の種類。
public enum BalanceEffectKind: String, CaseIterable, Sendable, Hashable {
    case none
    case immune
    case absorb
    case multiplier
}

public struct BalanceDefenseEntry: Equatable, Sendable {
    public var attackType: PokeType
    /// 分数の文字列(`"4"`・`"1/2"`・`"3/2"` など)。応答のまま。
    public var multiplier: String
    public var category: BalanceDefenseCategory
    public var source: BalanceEffectSource
    public var effect: BalanceEffectKind

    public init(
        attackType: PokeType, multiplier: String, category: BalanceDefenseCategory,
        source: BalanceEffectSource, effect: BalanceEffectKind
    ) {
        self.attackType = attackType
        self.multiplier = multiplier
        self.category = category
        self.source = source
        self.effect = effect
    }
}

public struct BalanceMemberDefense: Equatable, Sendable {
    public var pokemonId: String
    public var abilityId: String?
    public var types: [PokeType]
    /// 18 タイプ分(契約の正準順)。
    public var defense: [BalanceDefenseEntry]

    public init(pokemonId: String, abilityId: String?, types: [PokeType], defense: [BalanceDefenseEntry]) {
        self.pokemonId = pokemonId
        self.abilityId = abilityId
        self.types = types
        self.defense = defense
    }
}

public struct BalanceTeamSummaryEntry: Equatable, Sendable {
    public var attackType: PokeType
    public var weak: Int
    public var quadWeak: Int
    public var resist: Int
    public var immune: Int
    public var neutral: Int

    public init(attackType: PokeType, weak: Int, quadWeak: Int, resist: Int, immune: Int, neutral: Int) {
        self.attackType = attackType
        self.weak = weak
        self.quadWeak = quadWeak
        self.resist = resist
        self.immune = immune
        self.neutral = neutral
    }
}

public struct BalanceDefenseAnalysis: Equatable, Sendable {
    /// 要求順(同じポケモンの重複も保つ)。
    public var members: [BalanceMemberDefense]
    public var teamSummary: [BalanceTeamSummaryEntry]

    public init(members: [BalanceMemberDefense], teamSummary: [BalanceTeamSummaryEntry]) {
        self.members = members
        self.teamSummary = teamSummary
    }
}

// MARK: - 攻撃範囲(coverage)

/// openapi `CoverageMultiplier`(攻撃側の単タイプ倍率の表示形)。攻撃技が無いときは nil で運ぶ。
public enum BalanceCoverageMultiplier: String, CaseIterable, Sendable, Hashable {
    case zero = "0"
    case half = "1/2"
    case neutral = "1"
    case double = "2"
}

public struct BalanceCoverageEntry: Equatable, Sendable {
    public var defenseType: PokeType
    public var bestMultiplier: BalanceCoverageMultiplier?
    public var effective: Bool
    public var superEffective: Bool

    public init(
        defenseType: PokeType, bestMultiplier: BalanceCoverageMultiplier?, effective: Bool, superEffective: Bool
    ) {
        self.defenseType = defenseType
        self.bestMultiplier = bestMultiplier
        self.effective = effective
        self.superEffective = superEffective
    }
}

public struct BalanceMemberCoverage: Equatable, Sendable {
    public var pokemonId: String
    public var moveIds: [String]
    public var attackTypes: [PokeType]
    public var coverage: [BalanceCoverageEntry]

    public init(pokemonId: String, moveIds: [String], attackTypes: [PokeType], coverage: [BalanceCoverageEntry]) {
        self.pokemonId = pokemonId
        self.moveIds = moveIds
        self.attackTypes = attackTypes
        self.coverage = coverage
    }
}

public struct BalanceTeamCoverageEntry: Equatable, Sendable {
    public var defenseType: PokeType
    public var bestMultiplier: BalanceCoverageMultiplier?
    public var effectiveMembers: Int
    public var superEffectiveMembers: Int

    public init(
        defenseType: PokeType, bestMultiplier: BalanceCoverageMultiplier?, effectiveMembers: Int,
        superEffectiveMembers: Int
    ) {
        self.defenseType = defenseType
        self.bestMultiplier = bestMultiplier
        self.effectiveMembers = effectiveMembers
        self.superEffectiveMembers = superEffectiveMembers
    }
}

public struct BalanceCoverageAnalysis: Equatable, Sendable {
    public var members: [BalanceMemberCoverage]
    public var teamCoverage: [BalanceTeamCoverageEntry]

    public init(members: [BalanceMemberCoverage], teamCoverage: [BalanceTeamCoverageEntry]) {
        self.members = members
        self.teamCoverage = teamCoverage
    }
}
