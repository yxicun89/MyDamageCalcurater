// BalanceDomainTypes: タイプバランス画面(P6-26。ADR-0505)のドメインの型と、画面が依存する境界 `BalanceService`。
//
// 契約は services/balance/api/openapi.yaml(生成物は PokeCalcBalanceAPI)。ここの型は生成型に依存しない
// (`PokeCalcService`・`SpeedService`・`JudgeService` のドメイン型と同じ方針。ADR-0500 §3)。生成型 ↔ ドメインの写像は `APIBalanceService` に閉じる。
// 倍率・分類・集計・攻撃範囲はすべて balance-svc が決める。この層は応答を運ぶだけで、相性の計算も弱点・耐性の判定し直しもしない
// (タイプ相性表はコードに持たない。絶対ルール〈ドメイン規約〉・ADR-0013・type-balance-design.md §4)。
// 倍率は契約の文字列のまま(`"1/2"`・`"3/4"` のような既約分数。float にしない。ADR-0017 §3)。
//
// 範囲(ADR-0505 §1): analyze(防御相性)と coverage(攻撃範囲)の 2 本。threats・recommendations・move-range は後続(別タスク)。
//
// spec-writer の足場: 型の形(名前・引数・case 名・rawValue)は Balance*Tests が固定する。

import Foundation

// MARK: - 要求

/// analyze の1体(openapi `AnalyzeRequestMember`)。`pokemonId` は構築の `speciesKey` をそのまま使う(Web も同じ。タイプは balance が read model から引く。ADR-0014 §1)。
public struct BalanceAnalyzeMember: Equatable, Sendable {
    public var pokemonId: String
    /// nil なら欄ごと送らない(`null` を送らない)。
    public var abilityId: String?

    public init(pokemonId: String, abilityId: String? = nil) {
        self.pokemonId = pokemonId
        self.abilityId = abilityId
    }
}

/// `POST /api/balance/v1/team-balance/analyze` の要求。メンバーは 1〜`RequestLimits.maxBalanceMembers` 体(順を保つ)。
public struct BalanceAnalyzeRequest: Equatable, Sendable {
    public var members: [BalanceAnalyzeMember]

    public init(members: [BalanceAnalyzeMember]) {
        self.members = members
    }
}

/// coverage の1体(openapi `CoverageRequestMember`)。`moveIds` は 0〜`RequestLimits.maxBalanceMovesPerMember` 件(重複なし・順を保つ)。常に載せる(空配列も送る)。
public struct BalanceCoverageMember: Equatable, Sendable {
    public var pokemonId: String
    public var moveIds: [String]

    public init(pokemonId: String, moveIds: [String]) {
        self.pokemonId = pokemonId
        self.moveIds = moveIds
    }
}

/// `POST /api/balance/v1/team-balance/coverage` の要求。
public struct BalanceCoverageRequest: Equatable, Sendable {
    public var members: [BalanceCoverageMember]

    public init(members: [BalanceCoverageMember]) {
        self.members = members
    }
}

// MARK: - 応答(analyze)

/// 防御の分類(openapi `DefenseCategory`)。rawValue は契約の値(`BalanceContractSyncTests` が固定する)。
public enum BalanceDefenseCategory: String, CaseIterable, Sendable, Hashable {
    case quadWeak = "quad_weak"
    case weak
    case neutral
    case resist
    case quadResist = "quad_resist"
    case immune
}

/// 倍率の出どころ(openapi `EffectSource`)。タイプ由来か、特性が変えたか。
public enum BalanceEffectSource: String, CaseIterable, Sendable, Hashable {
    case type
    case ability
}

/// 特性の効果の種類(openapi `DefenseEffect`)。
public enum BalanceDefenseEffect: String, CaseIterable, Sendable, Hashable {
    case none
    case immune
    case absorb
    case multiplier
}

/// 1 攻撃タイプに対する1体の防御(openapi `DefenseEntry`)。
public struct BalanceDefenseEntry: Equatable, Sendable {
    public var attackType: PokeType
    /// 倍率。契約の文字列のまま(`"0"`・`"1/4"`・`"3/4"`・`"2"` など。float にしない)。
    public var multiplier: String
    public var category: BalanceDefenseCategory
    public var source: BalanceEffectSource
    public var effect: BalanceDefenseEffect

    public init(
        attackType: PokeType, multiplier: String, category: BalanceDefenseCategory, source: BalanceEffectSource = .type,
        effect: BalanceDefenseEffect = .none
    ) {
        self.attackType = attackType
        self.multiplier = multiplier
        self.category = category
        self.source = source
        self.effect = effect
    }
}

/// メンバー1体の防御(openapi `MemberDefense`)。`defense` は 18 攻撃タイプ(契約の正準順 normal … fairy)。
public struct BalanceMemberDefense: Equatable, Sendable {
    public var pokemonId: String
    public var abilityId: String?
    public var types: [PokeType]
    public var defense: [BalanceDefenseEntry]

    public init(pokemonId: String, abilityId: String? = nil, types: [PokeType], defense: [BalanceDefenseEntry]) {
        self.pokemonId = pokemonId
        self.abilityId = abilityId
        self.types = types
        self.defense = defense
    }
}

/// 攻撃タイプごとのチーム集計(openapi `TeamSummaryEntry`)。`quadWeak` は `weak` の内数。
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

/// analyze の応答。`members` は要求と同じ順(重複した pokemonId も残る)・`teamSummary` は 18 攻撃タイプの正準順。
public struct BalanceAnalyzeResponse: Equatable, Sendable {
    public var members: [BalanceMemberDefense]
    public var teamSummary: [BalanceTeamSummaryEntry]

    public init(members: [BalanceMemberDefense], teamSummary: [BalanceTeamSummaryEntry]) {
        self.members = members
        self.teamSummary = teamSummary
    }
}

// MARK: - 応答(coverage)

/// 攻撃範囲の倍率(openapi `CoverageMultiplier`。`null`〈攻撃技なし〉は `Optional` で表す)。
public enum BalanceCoverageMultiplier: String, CaseIterable, Sendable, Hashable {
    case zero = "0"
    case half = "1/2"
    case neutral = "1"
    case double = "2"
}

/// 1 防御タイプに対する1体の攻撃範囲(openapi `DefenseCoverageEntry`)。`bestMultiplier == nil` は攻撃技が無い(そのとき `effective`・`superEffective` は false)。
public struct BalanceDefenseCoverageEntry: Equatable, Sendable {
    public var defenseType: PokeType
    public var bestMultiplier: BalanceCoverageMultiplier?
    public var effective: Bool
    public var superEffective: Bool

    public init(defenseType: PokeType, bestMultiplier: BalanceCoverageMultiplier?, effective: Bool, superEffective: Bool) {
        self.defenseType = defenseType
        self.bestMultiplier = bestMultiplier
        self.effective = effective
        self.superEffective = superEffective
    }
}

/// メンバー1体の攻撃範囲(openapi `MemberCoverage`)。`attackTypes` は変化技を除く技のタイプ(重複なし・正準順。攻撃技が無ければ空)。
public struct BalanceMemberCoverage: Equatable, Sendable {
    public var pokemonId: String
    public var moveIds: [String]
    public var attackTypes: [PokeType]
    public var coverage: [BalanceDefenseCoverageEntry]

    public init(pokemonId: String, moveIds: [String], attackTypes: [PokeType], coverage: [BalanceDefenseCoverageEntry]) {
        self.pokemonId = pokemonId
        self.moveIds = moveIds
        self.attackTypes = attackTypes
        self.coverage = coverage
    }
}

/// 防御タイプごとのチーム集計(openapi `TeamCoverageEntry`)。`superEffectiveMembers` は `effectiveMembers` の内数。
public struct BalanceTeamCoverageEntry: Equatable, Sendable {
    public var defenseType: PokeType
    public var bestMultiplier: BalanceCoverageMultiplier?
    public var effectiveMembers: Int
    public var superEffectiveMembers: Int

    public init(defenseType: PokeType, bestMultiplier: BalanceCoverageMultiplier?, effectiveMembers: Int, superEffectiveMembers: Int) {
        self.defenseType = defenseType
        self.bestMultiplier = bestMultiplier
        self.effectiveMembers = effectiveMembers
        self.superEffectiveMembers = superEffectiveMembers
    }
}

/// coverage の応答。`members` は要求と同じ順・`teamCoverage` は 18 防御タイプの正準順。
public struct BalanceCoverageResponse: Equatable, Sendable {
    public var members: [BalanceMemberCoverage]
    public var teamCoverage: [BalanceTeamCoverageEntry]

    public init(members: [BalanceMemberCoverage], teamCoverage: [BalanceTeamCoverageEntry]) {
        self.members = members
        self.teamCoverage = teamCoverage
    }
}

// MARK: - 境界

/// タイプバランス API の境界。画面(`BalanceViewModel`)はこのプロトコルだけに依存する。
/// `PokeCalcService`・`SpeedService`・`JudgeService` には混ぜない(絶対ルール 5: balance の失敗が計算・構築に影響しない。ADR-0505 §4)。
/// 失敗は `PokeCalcError`(code は契約の `ErrorCode`・`transport`・`decode`・`client_unexpected_status`)。タスクのキャンセルは `CancellationError`。
public protocol BalanceService: Sendable {
    /// `POST /api/balance/v1/team-balance/analyze`(`analyzeTeamBalance`)。
    func analyzeTeamBalance(_ request: BalanceAnalyzeRequest) async throws -> BalanceAnalyzeResponse
    /// `POST /api/balance/v1/team-balance/coverage`(`analyzeTeamCoverage`)。
    func analyzeTeamCoverage(_ request: BalanceCoverageRequest) async throws -> BalanceCoverageResponse
}
