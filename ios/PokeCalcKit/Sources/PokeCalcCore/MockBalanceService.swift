// MockBalanceService: `BalanceService` のモック(P6-26。XCUITest・オフライン用。ADR-0505 §8)。
// 架空データ。挙動は起動時の環境変数 `POKECALC_MOCK_BALANCE` で切り替える(`POKECALC_MOCK_JUDGE` と同じ流儀)。固定する事実は MockBalanceServiceTests と ADR-0505 §8 にある。
//
// 式は本物を写したものではない(メンバーの位置・技の位置から決まる決定的な値)。モックはタイプ相性の正しさを保証しない(相性表を iOS に持ち込まない)。
// **メンバーごとに違う値を返す**(行の取り違えを画面・テストが検出できるように)。

public enum MockBalanceScenario: Equatable, Sendable {
    /// すべて成功(環境変数なし・未知の値の既定)。
    case normal
    /// analyze も coverage も失敗(`master_unavailable`)。
    case error
    /// coverage だけ失敗(`internal_error`)。analyze は成功する。「片方の失敗が他方の表示を消さない」の確認用。
    case coverageError

    /// 環境変数の値: `error` / `coverage-error`。nil・未知の値は `.normal`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "error": self = .error
        case "coverage-error": self = .coverageError
        default: self = .normal
        }
    }
}

public struct MockBalanceService: BalanceService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_BALANCE"

    private let scenario: MockBalanceScenario

    public init(scenario: MockBalanceScenario = .normal) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockBalanceScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    public func analyzeTeamBalance(_ request: BalanceAnalyzeRequest) async throws -> BalanceAnalyzeResponse {
        if scenario == .error { throw Self.masterUnavailable }
        try Self.validateMemberCount(request.members.count)
        let members = request.members.enumerated().map { index, member in
            Self.memberDefense(index: index, member: member)
        }
        return BalanceAnalyzeResponse(members: members, teamSummary: Self.summary(of: members))
    }

    public func analyzeTeamCoverage(_ request: BalanceCoverageRequest) async throws -> BalanceCoverageResponse {
        switch scenario {
        case .error: throw Self.masterUnavailable
        case .coverageError: throw PokeCalcError(code: "internal_error", message: "mock coverage failure")
        case .normal: break
        }
        try Self.validateMemberCount(request.members.count)
        let members = request.members.enumerated().map { index, member in
            Self.memberCoverage(index: index, member: member)
        }
        return BalanceCoverageResponse(members: members, teamCoverage: Self.teamCoverage(of: members))
    }

    // MARK: - 内部(決定的な架空値。タイプ相性の正しさは保証しない)

    private static let masterUnavailable = PokeCalcError(code: "master_unavailable", message: "mock master unavailable")

    private static func validateMemberCount(_ count: Int) throws {
        guard (RequestLimits.minBalanceMembers...RequestLimits.maxBalanceMembers).contains(count) else {
            throw PokeCalcError(code: "invalid_request", message: "members out of range")
        }
    }

    private static func memberDefense(index: Int, member: BalanceAnalyzeMember) -> BalanceMemberDefense {
        let types = PokeType.allCases
        let hasAbility = member.abilityId != nil
        let defense = types.enumerated().map { typeIndex, type -> BalanceDefenseEntry in
            if hasAbility && typeIndex == 0 {
                return BalanceDefenseEntry(attackType: type, multiplier: "3/4", category: .resist, source: .ability, effect: .multiplier)
            }
            switch (index + typeIndex) % 6 {
            case 0: return BalanceDefenseEntry(attackType: type, multiplier: "4", category: .quadWeak)
            case 1: return BalanceDefenseEntry(attackType: type, multiplier: "2", category: .weak)
            case 2: return BalanceDefenseEntry(attackType: type, multiplier: "1", category: .neutral)
            case 3: return BalanceDefenseEntry(attackType: type, multiplier: "1/2", category: .resist)
            case 4: return BalanceDefenseEntry(attackType: type, multiplier: "1/4", category: .quadResist)
            default: return BalanceDefenseEntry(attackType: type, multiplier: "0", category: .immune)
            }
        }
        return BalanceMemberDefense(
            pokemonId: member.pokemonId, abilityId: member.abilityId, types: [types[(2 * index) % types.count]], defense: defense)
    }

    private static func summary(of members: [BalanceMemberDefense]) -> [BalanceTeamSummaryEntry] {
        PokeType.allCases.enumerated().map { typeIndex, type in
            let categories = members.map { $0.defense[typeIndex].category }
            return BalanceTeamSummaryEntry(
                attackType: type,
                weak: categories.filter { $0 == .weak || $0 == .quadWeak }.count,
                quadWeak: categories.filter { $0 == .quadWeak }.count,
                resist: categories.filter { $0 == .resist || $0 == .quadResist }.count,
                immune: categories.filter { $0 == .immune }.count,
                neutral: categories.filter { $0 == .neutral }.count)
        }
    }

    private static let coverageSequence: [BalanceCoverageMultiplier] = [.zero, .half, .neutral, .double]

    private static func memberCoverage(index: Int, member: BalanceCoverageMember) -> BalanceMemberCoverage {
        let types = PokeType.allCases
        let attackSet = Set(member.moveIds.indices.map { types[(index + 5 * $0) % types.count] })
        let attackTypes = types.filter { attackSet.contains($0) }
        let hasMoves = !member.moveIds.isEmpty
        let coverage = types.enumerated().map { typeIndex, type -> BalanceDefenseCoverageEntry in
            guard hasMoves else {
                return BalanceDefenseCoverageEntry(defenseType: type, bestMultiplier: nil, effective: false, superEffective: false)
            }
            let multiplier = coverageSequence[(index + typeIndex) % coverageSequence.count]
            return BalanceDefenseCoverageEntry(
                defenseType: type, bestMultiplier: multiplier, effective: multiplier == .neutral || multiplier == .double,
                superEffective: multiplier == .double)
        }
        return BalanceMemberCoverage(pokemonId: member.pokemonId, moveIds: member.moveIds, attackTypes: attackTypes, coverage: coverage)
    }

    private static func teamCoverage(of members: [BalanceMemberCoverage]) -> [BalanceTeamCoverageEntry] {
        PokeType.allCases.enumerated().map { typeIndex, type in
            let entries = members.compactMap { member -> BalanceDefenseCoverageEntry? in
                let entry = member.coverage[typeIndex]
                return entry.bestMultiplier == nil ? nil : entry
            }
            let best = entries.compactMap(\.bestMultiplier).max { rank($0) < rank($1) }
            return BalanceTeamCoverageEntry(
                defenseType: type, bestMultiplier: best, effectiveMembers: entries.filter(\.effective).count,
                superEffectiveMembers: entries.filter(\.superEffective).count)
        }
    }

    private static func rank(_ multiplier: BalanceCoverageMultiplier) -> Int {
        coverageSequence.firstIndex(of: multiplier) ?? 0
    }
}
