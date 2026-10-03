import Foundation

// BalanceStage3Types: タイプバランス第3段(仮想敵・おすすめタイプ・技範囲チェッカー)のドメインの型(ADR-0415 §8)。
//
// 倍率は balance の応答の文字列(分数)をそのまま運び、iOS で計算し直さない。生成型(`PokeCalcBalanceAPI`)は
// `APIBalanceService` だけが知る。`nameJa` は応答にあるときだけ入る(無ければ nil。画面は ID を出す)。

// MARK: - 仮想敵(threats)

public struct BalanceThreatMatchup: Equatable, Sendable {
    public var pokemonId: String
    /// メンバーが仮想敵の攻撃技から受ける最大倍率(分数の文字列)。仮想敵に攻撃技が無ければ nil。
    public var incoming: String?
    /// メンバーの攻撃技が仮想敵へ与える最大倍率。メンバーに攻撃技が無ければ nil。
    public var outgoing: String?
    public var safe: Bool
    public var superEffective: Bool

    public init(pokemonId: String, incoming: String?, outgoing: String?, safe: Bool, superEffective: Bool) {
        self.pokemonId = pokemonId
        self.incoming = incoming
        self.outgoing = outgoing
        self.safe = safe
        self.superEffective = superEffective
    }
}

public struct BalanceThreatResult: Equatable, Sendable {
    public var pokemonId: String
    public var abilityId: String?
    public var attackTypes: [PokeType]
    /// 要求したメンバーの順(同じポケモンの重複も保つ)。
    public var matchups: [BalanceThreatMatchup]
    public var safeMembers: Int
    public var superEffectiveMembers: Int

    public init(
        pokemonId: String, abilityId: String?, attackTypes: [PokeType], matchups: [BalanceThreatMatchup],
        safeMembers: Int, superEffectiveMembers: Int
    ) {
        self.pokemonId = pokemonId
        self.abilityId = abilityId
        self.attackTypes = attackTypes
        self.matchups = matchups
        self.safeMembers = safeMembers
        self.superEffectiveMembers = superEffectiveMembers
    }
}

public struct BalanceThreatsAnalysis: Equatable, Sendable {
    /// 要求した仮想敵の順。
    public var threats: [BalanceThreatResult]

    public init(threats: [BalanceThreatResult]) {
        self.threats = threats
    }
}

// MARK: - おすすめタイプ(recommendations)

public struct BalanceCandidatePokemon: Equatable, Sendable {
    public var pokemonId: String
    public var nameJa: String?
    public var types: [PokeType]
    public var exactMatch: Bool

    public init(pokemonId: String, nameJa: String?, types: [PokeType], exactMatch: Bool) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.types = types
        self.exactMatch = exactMatch
    }
}

public struct BalanceTypeCandidate: Equatable, Sendable {
    public var types: [PokeType]
    public var defenseCovered: [PokeType]
    public var offenseCovered: [PokeType]
    public var weaknesses: Int
    public var pokemon: [BalanceCandidatePokemon]

    public init(
        types: [PokeType], defenseCovered: [PokeType], offenseCovered: [PokeType], weaknesses: Int,
        pokemon: [BalanceCandidatePokemon]
    ) {
        self.types = types
        self.defenseCovered = defenseCovered
        self.offenseCovered = offenseCovered
        self.weaknesses = weaknesses
        self.pokemon = pokemon
    }
}

public struct BalanceAbilityOptionPokemon: Equatable, Sendable {
    public var pokemonId: String
    public var nameJa: String?
    public var abilityId: String
    /// 応答のまま(分数の文字列)。
    public var multiplier: String

    public init(pokemonId: String, nameJa: String?, abilityId: String, multiplier: String) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.abilityId = abilityId
        self.multiplier = multiplier
    }
}

public struct BalanceAbilityOption: Equatable, Sendable {
    public var attackType: PokeType
    public var pokemon: [BalanceAbilityOptionPokemon]

    public init(attackType: PokeType, pokemon: [BalanceAbilityOptionPokemon]) {
        self.attackType = attackType
        self.pokemon = pokemon
    }
}

public struct BalanceRecommendations: Equatable, Sendable {
    public var defenseHoles: [PokeType]
    public var offenseHoles: [PokeType]
    /// 応答の順(ふさげる穴の数の降順 → 弱点数の昇順 → 正準順)。
    public var candidates: [BalanceTypeCandidate]
    public var abilityOptions: [BalanceAbilityOption]

    public init(
        defenseHoles: [PokeType], offenseHoles: [PokeType], candidates: [BalanceTypeCandidate],
        abilityOptions: [BalanceAbilityOption]
    ) {
        self.defenseHoles = defenseHoles
        self.offenseHoles = offenseHoles
        self.candidates = candidates
        self.abilityOptions = abilityOptions
    }
}

// MARK: - 技範囲チェッカー(move-range)

public struct BalanceMoveRangeEntry: Equatable, Sendable {
    public var defenseType: PokeType
    /// 攻撃技が1つ以上ある要求だけが通るので、常に値がある(`BalanceCoverageMultiplier` の nil は来ない)。
    public var bestMultiplier: BalanceCoverageMultiplier
    public var effective: Bool
    public var superEffective: Bool

    public init(defenseType: PokeType, bestMultiplier: BalanceCoverageMultiplier, effective: Bool, superEffective: Bool) {
        self.defenseType = defenseType
        self.bestMultiplier = bestMultiplier
        self.effective = effective
        self.superEffective = superEffective
    }
}

public struct BalanceWalledByPokemon: Equatable, Sendable {
    public var pokemonId: String
    public var nameJa: String?
    public var types: [PokeType]
    public var bestMultiplier: String

    public init(pokemonId: String, nameJa: String?, types: [PokeType], bestMultiplier: String) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.types = types
        self.bestMultiplier = bestMultiplier
    }
}

public struct BalanceWalledByAbilityPokemon: Equatable, Sendable {
    public var pokemonId: String
    public var nameJa: String?
    public var abilityId: String
    public var bestMultiplier: String

    public init(pokemonId: String, nameJa: String?, abilityId: String, bestMultiplier: String) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.abilityId = abilityId
        self.bestMultiplier = bestMultiplier
    }
}

public struct BalanceMoveRange: Equatable, Sendable {
    public var attackTypes: [PokeType]
    /// 18 タイプ分(契約の正準順)。
    public var typeChart: [BalanceMoveRangeEntry]
    public var walledBy: [BalanceWalledByPokemon]
    public var walledByAbility: [BalanceWalledByAbilityPokemon]

    public init(
        attackTypes: [PokeType], typeChart: [BalanceMoveRangeEntry], walledBy: [BalanceWalledByPokemon],
        walledByAbility: [BalanceWalledByAbilityPokemon]
    ) {
        self.attackTypes = attackTypes
        self.typeChart = typeChart
        self.walledBy = walledBy
        self.walledByAbility = walledByAbility
    }
}
