// MockBalanceService: 構築の中の「タイプバランス」を XCUITest で確かめるための架空の応答(ADR-0532)。
//
// モック構成の既定は `UnavailableBalanceService`(架空の相性表を利用者に見せない。ADR-0415 §4)のまま。
// 起動時の環境変数 `POKECALC_MOCK_BALANCE=fixture` を渡したときだけこれに替える(UI テスト専用。`POKECALC_MOCK_SPEED` と同じ流儀)。
// 相性の値に意味は無い(本物の相性表を写していない)。名前はすべて「テスト」で始める(ADR-0002)。
// 固定する事実は MockBalanceServiceTests にある。

public struct MockBalanceService: BalanceService {
    public static let environmentKey = "POKECALC_MOCK_BALANCE"
    /// この値のときだけ架空の応答を返す。
    public static let fixtureValue = "fixture"

    public init() {}

    /// モック構成で使う `BalanceService`。`fixture` のときだけ架空の応答、それ以外は接続先なし(`balance_unavailable`)。
    public static func make(environment: [String: String]) -> any BalanceService {
        environment[environmentKey] == fixtureValue ? MockBalanceService() : UnavailableBalanceService()
    }

    // MARK: - 架空の値

    /// 弱点・耐性・無効の見本にする攻撃タイプ(ほかは等倍)。
    private static let weakType = PokeType.fire
    private static let resistType = PokeType.water
    private static let immuneType = PokeType.ground

    /// おすすめの候補の該当ポケモン。先頭 4 体はモックのマスタ(`Resources/species.json`)にいる種族(タップで枠に入る)。
    /// 残りは「もっと見る」を出すための架空の名前だけの行。
    private static let pickablePokemon: [BalanceCandidatePokemon] =
        [
            ("9002-000", "テストモンに", [PokeType.water, .flying]),
            ("9003-000", "テストモンさん", [.electric]),
            ("9004-000", "テストモンよん", [.normal]),
            ("9001-000", "テストモンいち", [.fire]),
        ].map { BalanceCandidatePokemon(pokemonId: $0.0, nameJa: $0.1, types: $0.2, exactMatch: true) }
        + (1...6).map {
            BalanceCandidatePokemon(pokemonId: "91\(String(format: "%02d", $0))-000", nameJa: "テストモンかそう\($0)", types: [.water], exactMatch: false)
        }

    // MARK: - BalanceService

    public func analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis {
        let defense = PokeType.allCases.map { type -> BalanceDefenseEntry in
            switch type {
            case Self.weakType:
                return BalanceDefenseEntry(attackType: type, multiplier: "2", category: .weak, source: .type, effect: .none)
            case Self.resistType:
                return BalanceDefenseEntry(attackType: type, multiplier: "1/2", category: .resist, source: .type, effect: .none)
            case Self.immuneType:
                return BalanceDefenseEntry(attackType: type, multiplier: "0", category: .immune, source: .type, effect: .none)
            default:
                return BalanceDefenseEntry(attackType: type, multiplier: "1", category: .neutral, source: .type, effect: .none)
            }
        }
        let count = members.count
        return BalanceDefenseAnalysis(
            members: members.map {
                BalanceMemberDefense(pokemonId: $0.pokemonId, abilityId: $0.abilityId, types: [.normal], defense: defense)
            },
            teamSummary: PokeType.allCases.map { type in
                BalanceTeamSummaryEntry(
                    attackType: type, weak: type == Self.weakType ? count : 0, quadWeak: 0,
                    resist: type == Self.resistType ? count : 0, immune: type == Self.immuneType ? count : 0,
                    neutral: [Self.weakType, Self.resistType, Self.immuneType].contains(type) ? 0 : count)
            })
    }

    public func coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis {
        let anyAttack = members.contains { !$0.moveIds.isEmpty }
        return BalanceCoverageAnalysis(
            members: members.map { member in
                let best: BalanceCoverageMultiplier? = member.moveIds.isEmpty ? nil : .neutral
                return BalanceMemberCoverage(
                    pokemonId: member.pokemonId, moveIds: member.moveIds,
                    attackTypes: member.moveIds.isEmpty ? [] : [.normal],
                    coverage: PokeType.allCases.map {
                        BalanceCoverageEntry(
                            defenseType: $0, bestMultiplier: best, effective: best != nil, superEffective: false)
                    })
            },
            teamCoverage: PokeType.allCases.map {
                BalanceTeamCoverageEntry(
                    defenseType: $0, bestMultiplier: anyAttack ? .neutral : nil,
                    effectiveMembers: members.filter { !$0.moveIds.isEmpty }.count, superEffectiveMembers: 0)
            })
    }

    public func threats(members: [BalanceMemberInput], threats: [BalanceMemberInput]) async throws -> BalanceThreatsAnalysis {
        BalanceThreatsAnalysis(
            threats: threats.map { threat in
                BalanceThreatResult(
                    pokemonId: threat.pokemonId, abilityId: threat.abilityId, attackTypes: [Self.weakType],
                    matchups: members.map {
                        BalanceThreatMatchup(pokemonId: $0.pokemonId, incoming: "2", outgoing: nil, safe: false, superEffective: false)
                    },
                    safeMembers: 0, superEffectiveMembers: 0)
            })
    }

    public func recommendations(members: [BalanceMemberInput], limit: Int?) async throws -> BalanceRecommendations {
        let candidates = [
            BalanceTypeCandidate(
                types: [Self.resistType], defenseCovered: [Self.weakType], offenseCovered: [], weaknesses: 2,
                pokemon: Self.pickablePokemon),
            BalanceTypeCandidate(
                types: [.grass, .poison], defenseCovered: [Self.resistType], offenseCovered: [], weaknesses: 4,
                pokemon: [Self.pickablePokemon[2]]),
        ]
        return BalanceRecommendations(
            defenseHoles: [Self.weakType], offenseHoles: [],
            candidates: Array(candidates.prefix(limit ?? candidates.count)), abilityOptions: [],
            skewedAttackTypes: [Self.weakType])
    }

    public func moveRange(moveIds: [String]) async throws -> BalanceMoveRange {
        BalanceMoveRange(
            attackTypes: [.normal],
            typeChart: PokeType.allCases.map {
                BalanceMoveRangeEntry(defenseType: $0, bestMultiplier: .neutral, effective: true, superEffective: false)
            },
            walledBy: [], walledByAbility: [])
    }
}
